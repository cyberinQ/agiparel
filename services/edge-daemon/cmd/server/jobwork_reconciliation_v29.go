// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// F3.17B: signed discrepancy reports and source-backed live return reconciliation.
// No vendor approval, accounting settlement, shipping, QC or production credit.
package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/attestation"
)

const jobworkDiscrepancyAuthority = "SIGNED_INTERNAL_REPORT_NOT_VERIFIED_VENDOR_LOSS_OR_QC_RESULT"

var f317bHash = regexp.MustCompile(`^[0-9a-f]{64}$`)

type jobworkDiscrepancyCommand struct {
	CommandID      string `json:"command_id"`
	PlantID        string `json:"plant_id"`
	OutwardID      string `json:"outward_id"`
	PartID         string `json:"part_id"`
	Category       string `json:"category"`
	Note           string `json:"note"`
	EvidenceRef    string `json:"evidence_ref"`
	EvidenceSHA256 string `json:"evidence_sha256"`
}
type jobworkDiscrepancyRequest struct {
	Command jobworkDiscrepancyCommand `json:"command"`
}

func validJobworkDiscrepancy(c jobworkDiscrepancyCommand) bool {
	if !sewCommandID.MatchString(c.CommandID) || !sewFields(c.PlantID) ||
		!sewUUID(c.OutwardID) || !sewUUID(c.PartID) || !f317bHash.MatchString(c.EvidenceSHA256) ||
		len(strings.TrimSpace(c.Note)) < 20 || len(c.Note) > 1000 || strings.ContainsRune(c.Note, 0) ||
		len(strings.TrimSpace(c.EvidenceRef)) < 8 || len(c.EvidenceRef) > 180 || strings.ContainsAny(c.EvidenceRef, "\n\r\x00") {
		return false
	}
	return c.Category == "MISSING_AT_RECONCILIATION" || c.Category == "DAMAGED_ON_RETURN"
}
func jobworkDiscrepancyChallenge(c jobworkDiscrepancyCommand) string {
	return "JOBWORK_DISCREPANCY_V29:" + sewDigest(c)
}
func (p *jobworkAPI) discrepancyChallenge(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var req jobworkDiscrepancyRequest
	if !sewProdStrictJSON(w, r, &req) {
		return
	}
	if !validJobworkDiscrepancy(req.Command) {
		docJSON(w, 422, map[string]string{"error": "ERR_F317B_FIELDS"})
		return
	}
	docJSON(w, 200, map[string]any{"challenge": jobworkDiscrepancyChallenge(req.Command),
		"required_action": "jobwork.discrepancy.report", "required_role": "sewing_supervisor",
		"required_auth": "webauthn_or_fido2", "authority": jobworkDiscrepancyAuthority,
		"evidence_fingerprint_is_not_verification": true, "physical_loss_confirmed": false,
		"quality_release": false, "financial_settlement": false})
}
func (p *jobworkAPI) discrepancyEvent(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var req jobworkDiscrepancyRequest
	if !sewProdStrictJSON(w, r, &req) {
		return
	}
	c := req.Command
	if !validJobworkDiscrepancy(c) {
		docJSON(w, 422, map[string]string{"error": "ERR_F317B_FIELDS"})
		return
	}
	token, err := bearer(r)
	if err != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_F317B_SIGNED_BEARER_REQUIRED"})
		return
	}
	a, err := p.holds.wip.gen.sewing.verifier.Verify(token, "jobwork.discrepancy.report", jobworkDiscrepancyChallenge(c), "sewing_supervisor", true)
	if err != nil || !validChapterActor(a) || a.PlantID != c.PlantID || a.JWTID == "" || len(a.JWTID) > 255 {
		docJSON(w, 401, map[string]string{"error": "ERR_F317B_SIGNED_ASSERTION"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	id, replayed, err := p.persistDiscrepancy(ctx, c, a, token)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	code := 201
	if replayed {
		code = 200
	}
	docJSON(w, code, map[string]any{"id": id, "replayed": replayed, "outward_id": c.OutwardID,
		"part_id": c.PartID, "category": c.Category, "authority": jobworkDiscrepancyAuthority,
		"physical_loss_confirmed": false, "vendor_acknowledged": false, "released": false})
}
func (p *jobworkAPI) persistDiscrepancy(ctx context.Context, c jobworkDiscrepancyCommand, a attestation.Claims, token string) (string, bool, error) {
	tx, err := p.holds.wip.gen.sewing.server.docTransaction(ctx, a.TenantID)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SET LOCAL ROLE agiparel_app`); err != nil {
		return "", false, err
	}
	if err = workforceChapterRequire(ctx, tx, a, "jobwork.discrepancy.report"); err != nil {
		return "", false, err
	}
	if err = sewTxLock(ctx, tx, a.TenantID, "F317B:COMMAND:"+c.CommandID); err != nil {
		return "", false, err
	}
	hash := sewDigest(c)
	var id, oldHash, oldSubject string
	err = tx.QueryRowContext(ctx, `SELECT id::text,command_sha256,reporter_subject
   FROM jobwork_discrepancies_v29 WHERE tenant_id=$1 AND command_id=$2`, a.TenantID, c.CommandID).Scan(&id, &oldHash, &oldSubject)
	if err == nil {
		if oldHash != hash || oldSubject != a.Subject {
			return "", false, sewFail(409, "ERR_F317B_COMMAND_CONFLICT")
		}
		return id, true, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, err
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO jobwork_discrepancies_v29
  (tenant_id,plant_id,outward_id,part_id,category,note,evidence_ref,evidence_sha256,
   command_id,command_sha256,reporter_subject,reporter_proof_id,assertion_sha256)
  VALUES ($1,$2,$3::uuid,$4::uuid,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING id::text`,
		a.TenantID, c.PlantID, c.OutwardID, c.PartID, c.Category, c.Note, c.EvidenceRef, c.EvidenceSHA256,
		c.CommandID, hash, a.Subject, a.JWTID, sewProdHashString(token)).Scan(&id)
	if err != nil {
		return "", false, jobworkAdmissionError(err)
	}
	return id, false, tx.Commit()
}

func validJobworkRegister(q map[string][]string) (string, string, string, string, string, int, bool) {
	for k, v := range q {
		if len(v) != 1 {
			return "", "", "", "", "", 0, false
		}
		switch k {
		case "tenant_id", "plant_id", "vendor_id", "job_reference", "process", "after", "limit":
		default:
			return "", "", "", "", "", 0, false
		}
	}
	t, p, v, j, process := q["tenant_id"], q["plant_id"], q["vendor_id"], q["job_reference"], q["process"]
	if len(t) != 1 || len(p) != 1 || len(v) != 1 || len(j) != 1 || len(process) != 1 ||
		!sewFields(t[0], p[0], v[0], j[0]) {
		return "", "", "", "", "", 0, false
	}
	switch process[0] {
	case "EMBROIDERY", "PRINTING", "WASHING", "DYEING", "OTHER_FINISHING":
	default:
		return "", "", "", "", "", 0, false
	}
	after := ""
	if a, ok := q["after"]; ok {
		after = a[0]
		if !sewUUID(after) {
			return "", "", "", "", "", 0, false
		}
	}
	limit := 50
	if s, ok := q["limit"]; ok {
		n, err := strconv.Atoi(s[0])
		if err != nil || n < 1 || n > 100 {
			return "", "", "", "", "", 0, false
		}
		limit = n
	}
	return t[0], p[0], v[0], j[0], process[0], limit, true
}

type jobworkRegisterRow struct {
	OutwardID         string    `json:"outward_id"`
	PartID            string    `json:"part_id"`
	OutwardAt         time.Time `json:"outward_at"`
	InwardID          *string   `json:"inward_id"`
	Discrepancy       *string   `json:"discrepancy"`
	VendorAckRecorded bool      `json:"vendor_ack_recorded"`
	Status            string    `json:"status"`
}

func (p *jobworkAPI) register(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	if !p.ready(w) {
		return
	}
	q := r.URL.Query()
	tenant, plant, vendor, job, process, limit, ok := validJobworkRegister(q)
	if !ok {
		docJSON(w, 422, map[string]string{"error": "ERR_F317B_QUERY"})
		return
	}
	if p.holds.access == nil {
		docJSON(w, 503, map[string]string{"error": "ERR_F317B_READ_DISABLED"})
		return
	}
	actor, err := (&commissioningAPI{access: p.holds.access}).identify(r, tenant, plant)
	if err != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_F317B_AUTH"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, err := commissionTx(ctx, p.holds.wip.gen.sewing.server.db, tenant)
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
		docJSON(w, 403, map[string]string{"error": "ERR_F317B_READ_GRANT_REQUIRED"})
		return
	}
	var outward, returned, disputed, unreturned, damaged, uniqueParts int64
	err = tx.QueryRowContext(ctx, `SELECT COUNT(*)::bigint,COUNT(i.id)::bigint,COUNT(d.id)::bigint,
  COUNT(*) FILTER(WHERE i.id IS NULL)::bigint,
  COUNT(*) FILTER(WHERE d.category='DAMAGED_ON_RETURN')::bigint,
  COUNT(DISTINCT o.part_id)::bigint
  FROM jobwork_outwards_v28 o
  LEFT JOIN jobwork_inwards_v28 i ON i.tenant_id=o.tenant_id AND i.outward_id=o.id
  LEFT JOIN jobwork_discrepancies_v29 d ON d.tenant_id=o.tenant_id AND d.outward_id=o.id
  WHERE o.tenant_id=$1 AND o.plant_id=$2 AND o.vendor_id=$3 AND o.job_reference=$4 AND o.process_code=$5`,
		tenant, plant, vendor, job, process).Scan(&outward, &returned, &disputed, &unreturned, &damaged, &uniqueParts)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	// Vendor-signed acknowledgements are a separate, non-credit evidence layer.
	var vendorAcks int64
	err = tx.QueryRowContext(ctx, `SELECT count(*)::bigint FROM jobwork_vendor_acks_v30 a
     JOIN jobwork_outwards_v28 o ON o.tenant_id=a.tenant_id AND o.id=a.outward_id
     WHERE o.tenant_id=$1 AND o.plant_id=$2 AND o.vendor_id=$3 AND o.job_reference=$4 AND o.process_code=$5`,
		tenant, plant, vendor, job, process).Scan(&vendorAcks)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	after := ""
	if vv := q.Get("after"); vv != "" {
		after = vv
	}
	rows, err := tx.QueryContext(ctx, `SELECT o.id::text,o.part_id::text,o.recorded_at,i.id::text,d.category,(a.id IS NOT NULL)
  FROM jobwork_outwards_v28 o
  LEFT JOIN jobwork_inwards_v28 i ON i.tenant_id=o.tenant_id AND i.outward_id=o.id
  LEFT JOIN jobwork_discrepancies_v29 d ON d.tenant_id=o.tenant_id AND d.outward_id=o.id
  LEFT JOIN jobwork_vendor_acks_v30 a ON a.tenant_id=o.tenant_id AND a.outward_id=o.id
  WHERE o.tenant_id=$1 AND o.plant_id=$2 AND o.vendor_id=$3 AND o.job_reference=$4 AND o.process_code=$5
   AND (NULLIF($6,'') IS NULL OR o.id>NULLIF($6,'')::uuid)
  ORDER BY o.id LIMIT $7`, tenant, plant, vendor, job, process, after, limit+1)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	items := make([]jobworkRegisterRow, 0, limit+1)
	for rows.Next() {
		var r jobworkRegisterRow
		var inward, category sql.NullString
		if err = rows.Scan(&r.OutwardID, &r.PartID, &r.OutwardAt, &inward, &category, &r.VendorAckRecorded); err != nil {
			break
		}
		if inward.Valid {
			v := inward.String
			r.InwardID = &v
		}
		if category.Valid {
			v := category.String
			r.Discrepancy = &v
			r.Status = "DISCREPANCY_REPORTED_UNRESOLVED"
		} else if inward.Valid {
			r.Status = "RETURN_ATTESTED_PENDING_INTERNAL_QC"
		} else {
			r.Status = "OUTWARD_OPEN_NOT_VENDOR_RECEIPT"
		}
		items = append(items, r)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		sewProdReply(w, err)
		return
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	next := ""
	if more && len(items) > 0 {
		next = items[len(items)-1].OutwardID
	}
	if err = tx.Commit(); err != nil {
		sewProdReply(w, err)
		return
	}
	docJSON(w, 200, map[string]any{"tenant_id": tenant, "plant_id": plant, "vendor_id": vendor,
		"job_reference": job, "process": process, "authority": "LIVE_LEDGER_RECONCILIATION_NOT_SIGNED_GATE_PASS_OR_VENDOR_ACK",
		"outward_count": outward, "distinct_part_count": uniqueParts, "repeated_part_outward_count": outward - uniqueParts, "inward_count": returned, "not_returned_count": unreturned,
		"disputed_count": disputed, "damaged_return_count": damaged, "vendor_ack_evidence_count": vendorAcks,
		"items": items, "next_after": next, "has_more": more,
		"snapshot_scope": "single_repeatable_read_database_transaction", "vendor_ack_is_not_physical_receipt_or_qc": true,
		"financial_settlement": false, "quality_release": false})
}
func registerJobworkReconciliationRoutes(mux *http.ServeMux, s *EdgeServer) {
	p := newJobworkAPI(s)
	mux.HandleFunc("/api/v1/jobwork/reconciliation", p.register)
	mux.HandleFunc("/api/v1/jobwork/discrepancies/challenge", p.discrepancyChallenge)
	mux.HandleFunc("/api/v1/jobwork/discrepancies", p.discrepancyEvent)
}
