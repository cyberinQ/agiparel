// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// F3.17A: signed outward/return job-work custody of permanent cut-part IDs.
// No independent vendor acknowledgement, manufacturing, QC, payroll or shipment credit.
package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/attestation"
)

const jobworkAuthority = "SIGNED_INTERNAL_JOBWORK_CUSTODY_CLAIM_NOT_VENDOR_CERTIFICATION"

type jobworkCommand struct {
	CommandID    string `json:"command_id"`
	PlantID      string `json:"plant_id"`
	StationID    string `json:"station_id"`
	PartID       string `json:"part_id"`
	ScanID       string `json:"scan_id"`
	VendorID     string `json:"vendor_id,omitempty"`
	JobReference string `json:"job_reference,omitempty"`
	Process      string `json:"process,omitempty"`
	OutwardID    string `json:"outward_id,omitempty"`
	Note         string `json:"note"`
}
type jobworkRequest struct {
	Command jobworkCommand `json:"command"`
}
type jobworkAPI struct {
	holds   *pieceHoldAPI
	enabled bool
}

func newJobworkAPI(s *EdgeServer) *jobworkAPI {
	p := &jobworkAPI{}
	if s == nil || s.db == nil || os.Getenv("AGIPAREL_JOBWORK_CUSTODY_ENABLED") != "true" ||
		(os.Getenv("ENV") != "staging" && os.Getenv("ENV") != "production") {
		return p
	}
	p.holds = newPieceHoldAPI(s)
	p.enabled = p.holds.enabled
	return p
}
func (p *jobworkAPI) ready(w http.ResponseWriter) bool {
	if p == nil || !p.enabled {
		docJSON(w, 503, map[string]string{"error": "ERR_F317_DISABLED"})
		return false
	}
	return true
}
func jobworkValid(c jobworkCommand, outward bool) bool {
	if !sewCommandID.MatchString(c.CommandID) || !sewFields(c.PlantID, c.StationID) ||
		!sewUUID(c.PartID) || !sewUUID(c.ScanID) || len(strings.TrimSpace(c.Note)) < 12 ||
		len(c.Note) > 500 || strings.ContainsRune(c.Note, 0) {
		return false
	}
	if outward {
		if !sewFields(c.VendorID, c.JobReference) || c.OutwardID != "" {
			return false
		}
		switch c.Process {
		case "EMBROIDERY", "PRINTING", "WASHING", "DYEING", "OTHER_FINISHING":
			return true
		}
		return false
	}
	return sewUUID(c.OutwardID) && c.VendorID == "" && c.JobReference == "" && c.Process == ""
}
func jobworkAction(outward bool) (string, string, string) {
	if outward {
		return "jobwork.outward", "sewing_operator", "JOBWORK_OUT_V28:"
	}
	return "jobwork.inward", "sewing_supervisor", "JOBWORK_IN_V28:"
}
func jobworkChallenge(c jobworkCommand, outward bool) string {
	_, _, prefix := jobworkAction(outward)
	return prefix + sewDigest(c)
}
func (p *jobworkAPI) challenge(w http.ResponseWriter, r *http.Request, outward bool) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var req jobworkRequest
	if !sewProdStrictJSON(w, r, &req) {
		return
	}
	if !jobworkValid(req.Command, outward) {
		docJSON(w, 422, map[string]string{"error": "ERR_F317_FIELDS"})
		return
	}
	permission, role, _ := jobworkAction(outward)
	docJSON(w, 200, map[string]any{"challenge": jobworkChallenge(req.Command, outward),
		"required_action": permission, "required_role": role, "required_auth": "webauthn_or_fido2",
		"authority": jobworkAuthority, "vendor_confirmed": false, "operation_credit": false, "qc_release": false, "shipment": false})
}
func (p *jobworkAPI) event(w http.ResponseWriter, r *http.Request, outward bool) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var req jobworkRequest
	if !sewProdStrictJSON(w, r, &req) {
		return
	}
	c := req.Command
	if !jobworkValid(c, outward) {
		docJSON(w, 422, map[string]string{"error": "ERR_F317_FIELDS"})
		return
	}
	token, err := bearer(r)
	if err != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_F317_SIGNED_BEARER_REQUIRED"})
		return
	}
	permission, role, _ := jobworkAction(outward)
	a, err := p.holds.wip.gen.sewing.verifier.Verify(token, permission, jobworkChallenge(c, outward), role, true)
	if err != nil || !validChapterActor(a) || a.PlantID != c.PlantID || a.JWTID == "" || len(a.JWTID) > 255 {
		docJSON(w, 401, map[string]string{"error": "ERR_F317_SIGNED_ASSERTION"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	id, replay, err := p.persist(ctx, c, a, token, permission, outward)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	code := 201
	if replay {
		code = 200
	}
	docJSON(w, code, map[string]any{"id": id, "replayed": replay, "action": permission,
		"part_id": c.PartID, "authority": jobworkAuthority, "vendor_confirmed": false,
		"operation_credit": false, "quality_release": false, "shipment": false})
}
func (p *jobworkAPI) persist(ctx context.Context, c jobworkCommand, a attestation.Claims, token, permission string, outward bool) (string, bool, error) {
	tx, err := p.holds.wip.gen.sewing.server.docTransaction(ctx, a.TenantID)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SET LOCAL ROLE agiparel_app`); err != nil {
		return "", false, err
	}
	if err = workforceChapterRequire(ctx, tx, a, permission); err != nil {
		return "", false, err
	}
	if err = sewTxLock(ctx, tx, a.TenantID, "F317:COMMAND:"+c.CommandID); err != nil {
		return "", false, err
	}
	hash := sewDigest(c)
	table := "jobwork_outwards_v28"
	subjectColumn := "sender_subject"
	if !outward {
		table = "jobwork_inwards_v28"
		subjectColumn = "receiver_subject"
	}
	// Table and column are selected exclusively from constants, never HTTP input.
	var id, oldHash, oldSubject string
	err = tx.QueryRowContext(ctx, "SELECT id::text,command_sha256,"+subjectColumn+" FROM "+table+" WHERE tenant_id=$1 AND command_id=$2", a.TenantID, c.CommandID).Scan(&id, &oldHash, &oldSubject)
	if err == nil {
		if oldHash != hash || oldSubject != a.Subject {
			return "", false, sewFail(409, "ERR_F317_COMMAND_CONFLICT")
		}
		return id, true, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, err
	}
	if outward {
		err = tx.QueryRowContext(ctx, `INSERT INTO jobwork_outwards_v28
          (tenant_id,plant_id,station_id,part_id,scan_id,vendor_id,job_reference,process_code,note,
           command_id,command_sha256,sender_subject,sender_proof_id,assertion_sha256)
          VALUES ($1,$2,$3,$4::uuid,$5::uuid,$6,$7,$8,$9,$10,$11,$12,$13,$14) RETURNING id::text`,
			a.TenantID, c.PlantID, c.StationID, c.PartID, c.ScanID, c.VendorID, c.JobReference, c.Process, c.Note,
			c.CommandID, hash, a.Subject, a.JWTID, sewProdHashString(token)).Scan(&id)
	} else {
		err = tx.QueryRowContext(ctx, `INSERT INTO jobwork_inwards_v28
          (tenant_id,plant_id,station_id,part_id,scan_id,outward_id,note,
           command_id,command_sha256,receiver_subject,receiver_proof_id,assertion_sha256)
          VALUES ($1,$2,$3,$4::uuid,$5::uuid,$6::uuid,$7,$8,$9,$10,$11,$12) RETURNING id::text`,
			a.TenantID, c.PlantID, c.StationID, c.PartID, c.ScanID, c.OutwardID, c.Note,
			c.CommandID, hash, a.Subject, a.JWTID, sewProdHashString(token)).Scan(&id)
	}
	if err != nil {
		return "", false, jobworkAdmissionError(err)
	}
	return id, false, tx.Commit()
}
func jobworkAdmissionError(err error) error {
	var st interface{ SQLState() string }
	if errors.As(err, &st) {
		switch st.SQLState() {
		case "23505", "23503", "23514", "P0001":
			return sewFail(409, "ERR_F317_CUSTODY_OR_PROOF_CONFLICT")
		}
	}
	return err
}
func (p *jobworkAPI) status(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	if !p.ready(w) {
		return
	}
	q := r.URL.Query()
	if len(q) != 3 || len(q["tenant_id"]) != 1 || len(q["plant_id"]) != 1 || len(q["part_id"]) != 1 ||
		!sewFields(q.Get("tenant_id"), q.Get("plant_id")) || !sewUUID(q.Get("part_id")) {
		docJSON(w, 422, map[string]string{"error": "ERR_F317_QUERY"})
		return
	}
	for k := range q {
		if k != "tenant_id" && k != "plant_id" && k != "part_id" {
			docJSON(w, 422, map[string]string{"error": "ERR_F317_QUERY"})
			return
		}
	}
	// Exact tenant/plant identity and signed-read permission are checked by existing OIDC browser gate.
	access := p.holds.access
	if access == nil {
		docJSON(w, 503, map[string]string{"error": "ERR_F317_READ_DISABLED"})
		return
	}
	actor, err := (&commissioningAPI{access: access}).identify(r, q.Get("tenant_id"), q.Get("plant_id"))
	if err != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_F317_AUTH"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, err := commissionTx(ctx, p.holds.wip.gen.sewing.server.db, q.Get("tenant_id"))
	if err != nil {
		sewProdReply(w, err)
		return
	}
	defer tx.Rollback()
	permitted, err := workforceChapterPermit(ctx, tx, actor, "sewing.read")
	if err != nil {
		sewProdReply(w, err)
		return
	}
	if !permitted {
		docJSON(w, 403, map[string]string{"error": "ERR_F317_READ_GRANT_REQUIRED"})
		return
	}
	var outwardID, vendor, process, job, station string
	var opened time.Time
	var returned sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT o.id::text,o.vendor_id,o.process_code,o.job_reference,o.station_id,o.recorded_at,
       (SELECT min(i.recorded_at) FROM jobwork_inwards_v28 i WHERE i.tenant_id=o.tenant_id AND i.outward_id=o.id)
       FROM jobwork_outwards_v28 o WHERE o.tenant_id=$1 AND o.plant_id=$2 AND o.part_id=$3::uuid
       ORDER BY o.recorded_at DESC,o.id DESC LIMIT 1`, q.Get("tenant_id"), q.Get("plant_id"), q.Get("part_id")).Scan(
		&outwardID, &vendor, &process, &job, &station, &opened, &returned)
	if errors.Is(err, sql.ErrNoRows) {
		docJSON(w, 200, map[string]any{"part_id": q.Get("part_id"), "status": "NO_JOBWORK_RECORD", "physical_status_verified": false})
		return
	}
	if err != nil {
		sewProdReply(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		sewProdReply(w, err)
		return
	}
	state := "OUTWARD_RECORDED_NOT_VENDOR_CERTIFIED"
	if returned.Valid {
		state = "RETURN_ATTESTED_PENDING_INTERNAL_QUALITY"
	}
	var returnedAt any
	if returned.Valid {
		returnedAt = returned.Time.UTC()
	}
	docJSON(w, 200, map[string]any{"part_id": q.Get("part_id"), "outward_id": outwardID, "vendor_id": vendor,
		"process": process, "job_reference": job, "station_id": station, "outward_recorded_at": opened,
		"return_recorded_at": returnedAt, "status": state, "physical_status_verified": false,
		"quality_release": false, "operation_credit": false})
}
func registerJobworkRoutes(mux *http.ServeMux, s *EdgeServer) {
	p := newJobworkAPI(s)
	mux.HandleFunc("/api/v1/jobwork/parts/outward/challenge", func(w http.ResponseWriter, r *http.Request) { p.challenge(w, r, true) })
	mux.HandleFunc("/api/v1/jobwork/parts/outward", func(w http.ResponseWriter, r *http.Request) { p.event(w, r, true) })
	mux.HandleFunc("/api/v1/jobwork/parts/inward/challenge", func(w http.ResponseWriter, r *http.Request) { p.challenge(w, r, false) })
	mux.HandleFunc("/api/v1/jobwork/parts/inward", func(w http.ResponseWriter, r *http.Request) { p.event(w, r, false) })
	mux.HandleFunc("/api/v1/jobwork/parts/status", p.status)
}
