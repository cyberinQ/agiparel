// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// F3.10: signed exceptional-WIP reporting and independent, scan-backed review.
// Closing a case clears an administrative hold only. It NEVER changes physical
// custody, binds a tag, fabricates a scan, or grants sewing/QC/shipping credit.
package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/attestation"
)

const exceptionAuthority = "SIGNED_EXCEPTION_REVIEW_NO_CUSTODY_CHANGE"

type wipExceptionCommand struct {
	CommandID string `json:"command_id"`
	PlantID   string `json:"plant_id"`
	PartID    string `json:"part_id"`
	StationID string `json:"station_id"`
	Action    string `json:"action"`
	IssueType string `json:"issue_type"`
	CaseID    string `json:"case_id"`
	ScanID    string `json:"scan_id"`
	Note      string `json:"note"`
}
type wipExceptionRequest struct {
	Command wipExceptionCommand `json:"command"`
}
type wipExceptionAPI struct {
	wip     *wipAPI
	access  *chapterAccessAPI
	enabled bool
}

func newWIPExceptionAPI(s *EdgeServer) *wipExceptionAPI {
	p := &wipExceptionAPI{}
	if s == nil || s.db == nil || os.Getenv("AGIPAREL_WIP_EXCEPTIONS_ENABLED") != "true" ||
		(os.Getenv("ENV") != "production" && os.Getenv("ENV") != "staging") {
		return p
	}
	p.wip = newWIPAPI(s)
	if !p.wip.enabled {
		return p
	}
	p.access = newChapterAccessAPI(s)
	p.enabled = p.access.enabled
	return p
}
func (p *wipExceptionAPI) ready(w http.ResponseWriter) bool {
	if p == nil || !p.enabled {
		docJSON(w, 503, map[string]string{"error": "ERR_F310_DISABLED"})
		return false
	}
	return true
}
func exceptionValid(c wipExceptionCommand) bool {
	if !sewCommandID.MatchString(c.CommandID) || !sewFields(c.PlantID, c.StationID) || !sewUUID(c.PartID) ||
		len(strings.TrimSpace(c.Note)) < 12 || len(c.Note) > 500 || strings.ContainsRune(c.Note, 0) {
		return false
	}
	switch c.Action {
	case "OPEN":
		if c.CaseID != "" || c.ScanID != "" {
			return false
		}
		switch c.IssueType {
		case "LOST_CARRIER", "UNREADABLE_CARRIER", "MISSING_WIP", "FAILED_SCAN", "MISROUTED_PART", "REJECTED_HANDOFF":
			return true
		}
	case "CLOSE":
		return c.IssueType == "" && sewUUID(c.CaseID) && sewUUID(c.ScanID)
	}
	return false
}
func exceptionPermission(action string) (string, string) {
	if action == "OPEN" {
		return "sewing.exception.report", "sewing_operator"
	}
	return "sewing.exception.resolve", "sewing_supervisor"
}
func exceptionChallenge(c wipExceptionCommand) string { return "WIP_EXCEPTION_V1:" + sewDigest(c) }
func (p *wipExceptionAPI) challenge(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var req wipExceptionRequest
	if !sewProdStrictJSON(w, r, &req) {
		return
	}
	if !exceptionValid(req.Command) {
		docJSON(w, 422, map[string]string{"error": "ERR_F310_FIELDS"})
		return
	}
	permission, role := exceptionPermission(req.Command.Action)
	docJSON(w, 200, map[string]any{"challenge": exceptionChallenge(req.Command), "required_action": permission, "required_role": role,
		"required_auth": "webauthn_or_fido2", "authority": exceptionAuthority, "custody_changed": false, "production_credit": false})
}
func (p *wipExceptionAPI) event(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var req wipExceptionRequest
	if !sewProdStrictJSON(w, r, &req) {
		return
	}
	c := req.Command
	if !exceptionValid(c) {
		docJSON(w, 422, map[string]string{"error": "ERR_F310_FIELDS"})
		return
	}
	token, err := bearer(r)
	if err != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_F310_SIGNED_BEARER_REQUIRED"})
		return
	}
	permission, role := exceptionPermission(c.Action)
	a, err := p.wip.gen.sewing.verifier.Verify(token, permission, exceptionChallenge(c), role, true)
	if err != nil || a.PlantID != c.PlantID || !validChapterActor(a) || len(a.JWTID) == 0 || len(a.JWTID) > 255 {
		docJSON(w, 401, map[string]string{"error": "ERR_F310_SIGNED_ASSERTION"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	id, replay, err := p.persist(ctx, c, a, token, permission)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	status := 201
	if replay {
		status = 200
	}
	docJSON(w, status, map[string]any{"id": id, "replayed": replay, "action": c.Action, "authority": exceptionAuthority,
		"custody_changed": false, "production_credit": false, "quality_release": false, "shipping_release": false})
}
func (p *wipExceptionAPI) persist(ctx context.Context, c wipExceptionCommand, a attestation.Claims, token, permission string) (string, bool, error) {
	tx, err := p.wip.gen.sewing.server.docTransaction(ctx, a.TenantID)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback()
	if err = workforceChapterRequire(ctx, tx, a, permission); err != nil {
		return "", false, err
	}
	if err = sewTxLock(ctx, tx, a.TenantID, "WIP:PART:"+c.PartID); err != nil {
		return "", false, err
	}
	// Reject proof replay independently of command-id replay, including any prior
	// case or closure. Permission revocation was already checked above.
	hash := sewDigest(c)
	var id, priorHash, priorActor string
	if c.Action == "OPEN" {
		err = tx.QueryRowContext(ctx, `SELECT id::text,command_sha256,reporter_subject FROM wip_exception_cases_v23 WHERE tenant_id=$1 AND command_id=$2`, a.TenantID, c.CommandID).Scan(&id, &priorHash, &priorActor)
	} else {
		err = tx.QueryRowContext(ctx, `SELECT id::text,command_sha256,supervisor_subject FROM wip_exception_closures_v23 WHERE tenant_id=$1 AND command_id=$2`, a.TenantID, c.CommandID).Scan(&id, &priorHash, &priorActor)
	}
	if err == nil {
		if hash != priorHash || a.Subject != priorActor {
			return "", false, sewFail(409, "ERR_F310_COMMAND_REPLAY_CONFLICT")
		}
		return id, true, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, err
	}
	// The case/part scope is checked again by SQL triggers under the same lock.
	if c.Action == "OPEN" {
		err = tx.QueryRowContext(ctx, `INSERT INTO wip_exception_cases_v23
   (tenant_id,plant_id,part_id,station_id,issue_type,note,command_id,command_sha256,reporter_subject,reporter_proof_id,assertion_sha256)
   VALUES($1,$2,$3::uuid,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id::text`,
			a.TenantID, c.PlantID, c.PartID, c.StationID, c.IssueType, c.Note, c.CommandID, hash, a.Subject, a.JWTID, sewProdHashString(token)).Scan(&id)
	} else {
		err = tx.QueryRowContext(ctx, `INSERT INTO wip_exception_closures_v23
   (tenant_id,plant_id,part_id,station_id,case_id,scan_id,note,command_id,command_sha256,supervisor_subject,supervisor_proof_id,assertion_sha256)
   VALUES($1,$2,$3::uuid,$4,$5::uuid,$6::uuid,$7,$8,$9,$10,$11,$12) RETURNING id::text`,
			a.TenantID, c.PlantID, c.PartID, c.StationID, c.CaseID, c.ScanID, c.Note, c.CommandID, hash, a.Subject, a.JWTID, sewProdHashString(token)).Scan(&id)
	}
	if err != nil {
		return "", false, sewFail(409, "ERR_F310_CASE_OR_PROOF_CONFLICT")
	}
	return id, false, tx.Commit()
}
func exceptionQuery(q url.Values) (tenant, plant, part string, limit int, err error) {
	for k := range q {
		if k != "tenant_id" && k != "plant_id" && k != "part_id" && k != "limit" {
			return "", "", "", 0, errors.New("unknown field")
		}
	}
	if len(q["tenant_id"]) != 1 || len(q["plant_id"]) != 1 || len(q["part_id"]) != 1 ||
		!sewFields(q.Get("tenant_id"), q.Get("plant_id")) || !sewUUID(q.Get("part_id")) {
		return "", "", "", 0, errors.New("invalid scope")
	}
	if len(q["limit"]) > 1 {
		return "", "", "", 0, errors.New("invalid limit")
	}
	limit = 25
	if len(q["limit"]) == 1 {
		limit, err = strconv.Atoi(q.Get("limit"))
		if err != nil || limit < 1 || limit > 50 {
			return "", "", "", 0, errors.New("invalid limit")
		}
	}
	return q.Get("tenant_id"), q.Get("plant_id"), q.Get("part_id"), limit, nil
}
func (p *wipExceptionAPI) cases(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	if !p.ready(w) {
		return
	}
	tenant, plant, part, limit, err := exceptionQuery(r.URL.Query())
	if err != nil {
		docJSON(w, 422, map[string]string{"error": "ERR_F310_QUERY"})
		return
	}
	actor, err := (&commissioningAPI{access: p.access}).identify(r, tenant, plant)
	if err != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_F310_AUTH"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, err := p.wip.gen.sewing.server.docTransaction(ctx, tenant)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	defer tx.Rollback()
	readAllowed, err := workforceChapterPermit(ctx, tx, actor, "sewing.read")
	if err != nil {
		sewProdReply(w, err)
		return
	}
	reportAllowed, err := workforceChapterPermit(ctx, tx, actor, "sewing.exception.report")
	if err != nil {
		sewProdReply(w, err)
		return
	}
	resolveAllowed, err := workforceChapterPermit(ctx, tx, actor, "sewing.exception.resolve")
	if err != nil {
		sewProdReply(w, err)
		return
	}
	if !readAllowed && !reportAllowed && !resolveAllowed {
		docJSON(w, 403, map[string]string{"error": "ERR_F310_GRANT_REQUIRED"})
		return
	}
	var assigned, exists bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sew_prod_worker_stations_v10 WHERE tenant_id=$1 AND plant_id=$2 AND worker_subject=$3 AND enabled)`, tenant, plant, actor.Subject).Scan(&assigned)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	if !assigned {
		docJSON(w, 403, map[string]string{"error": "ERR_F310_WORKER_NOT_ASSIGNED"})
		return
	}
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM gen_cut_parts_v18 p JOIN gen_garments_v18 g ON g.tenant_id=p.tenant_id AND g.id=p.garment_id WHERE p.tenant_id=$1 AND p.id=$2::uuid AND g.plant_id=$3)`, tenant, part, plant).Scan(&exists)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	if !exists {
		docJSON(w, 404, map[string]string{"error": "ERR_F310_PART_NOT_FOUND"})
		return
	}
	rows, err := tx.QueryContext(ctx, `SELECT c.id::text,c.issue_type,c.station_id,c.note,c.opened_at,x.closed_at IS NOT NULL
 FROM wip_exception_cases_v23 c LEFT JOIN wip_exception_closures_v23 x ON x.tenant_id=c.tenant_id AND x.case_id=c.id
 WHERE c.tenant_id=$1 AND c.plant_id=$2 AND c.part_id=$3::uuid
 ORDER BY c.opened_at DESC,c.id DESC LIMIT $4`, tenant, plant, part, limit)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	type entry struct {
		ID        string    `json:"id"`
		IssueType string    `json:"issue_type"`
		StationID string    `json:"station_id"`
		Note      string    `json:"note"`
		OpenedAt  time.Time `json:"opened_at"`
		Closed    bool      `json:"closed"`
	}
	items := make([]entry, 0)
	for rows.Next() {
		var v entry
		if err = rows.Scan(&v.ID, &v.IssueType, &v.StationID, &v.Note, &v.OpenedAt, &v.Closed); err != nil {
			break
		}
		items = append(items, v)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		sewProdReply(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		sewProdReply(w, err)
		return
	}
	docJSON(w, 200, map[string]any{"tenant_id": tenant, "plant_id": plant, "part_id": part, "cases": items, "limit": limit,
		"authority": exceptionAuthority, "custody_changed": false, "physical_recovery_certified": false})
}
func registerWIPExceptionRoutes(mux *http.ServeMux, s *EdgeServer) {
	p := newWIPExceptionAPI(s)
	mux.HandleFunc("/api/v1/wip/exceptions/challenge", p.challenge)
	mux.HandleFunc("/api/v1/wip/exceptions/events", p.event)
	mux.HandleFunc("/api/v1/wip/exceptions/cases", p.cases)
}
