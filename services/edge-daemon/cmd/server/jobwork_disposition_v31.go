// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// F3.17D. An independently signed administrative disposition never creates
// a vendor loss, payment, production quantity or QC approval.
package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/attestation"
)

const jobworkDispositionAuthorityV31 = "INDEPENDENT_DISCREPANCY_DISPOSITION_NOT_QC_OR_SETTLEMENT"

type jobworkDispositionCommandV31 struct {
	CommandID      string `json:"command_id"`
	PlantID        string `json:"plant_id"`
	DiscrepancyID  string `json:"discrepancy_id"`
	OutwardID      string `json:"outward_id"`
	PartID         string `json:"part_id"`
	ResolutionKind string `json:"resolution_kind"`
	EvidenceRef    string `json:"evidence_ref"`
	EvidenceSHA256 string `json:"evidence_sha256"`
	Note           string `json:"note"`
}
type jobworkDispositionRequestV31 struct {
	Command jobworkDispositionCommandV31 `json:"command"`
}

func validJobworkDispositionV31(c jobworkDispositionCommandV31) bool {
	if !sewCommandID.MatchString(c.CommandID) || !sewFields(c.PlantID) || !sewUUID(c.DiscrepancyID) ||
		!sewUUID(c.OutwardID) || !sewUUID(c.PartID) || !f317bHash.MatchString(c.EvidenceSHA256) ||
		len(strings.TrimSpace(c.EvidenceRef)) < 8 || len(c.EvidenceRef) > 180 ||
		strings.ContainsAny(c.EvidenceRef, "\r\n\x00") || len(strings.TrimSpace(c.Note)) < 20 ||
		len(c.Note) > 1000 || strings.ContainsRune(c.Note, 0) {
		return false
	}
	switch c.ResolutionKind {
	case "RETURNED_TO_PIECE_HOLD", "SHORTAGE_REPORTED_REMAINS_BLOCKED", "DAMAGE_REPORTED_REMAINS_BLOCKED":
		return true
	}
	return false
}
func jobworkDispositionChallengeV31(c jobworkDispositionCommandV31) string {
	return "JOBWORK_DISPOSITION_V31:" + sewDigest(c)
}
func (p *jobworkAPI) dispositionChallengeV31(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var req jobworkDispositionRequestV31
	if !sewProdStrictJSON(w, r, &req) {
		return
	}
	if !validJobworkDispositionV31(req.Command) {
		docJSON(w, 422, map[string]string{"error": "ERR_F317D_FIELDS"})
		return
	}
	docJSON(w, 200, map[string]any{"challenge": jobworkDispositionChallengeV31(req.Command),
		"required_action": "jobwork.discrepancy.resolve", "required_role": "sewing_supervisor",
		"required_auth": "webauthn_or_fido2", "authority": jobworkDispositionAuthorityV31,
		"physical_recovery_requires_inward_and_active_piece_hold": true,
		"quality_release": false, "financial_settlement": false, "shipment": false})
}
func (p *jobworkAPI) dispositionEventV31(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var req jobworkDispositionRequestV31
	if !sewProdStrictJSON(w, r, &req) {
		return
	}
	c := req.Command
	if !validJobworkDispositionV31(c) {
		docJSON(w, 422, map[string]string{"error": "ERR_F317D_FIELDS"})
		return
	}
	token, err := bearer(r)
	if err != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_F317D_AUTH"})
		return
	}
	a, err := p.holds.wip.gen.sewing.verifier.Verify(token, "jobwork.discrepancy.resolve", jobworkDispositionChallengeV31(c), "sewing_supervisor", true)
	if err != nil || !validChapterActor(a) || a.PlantID != c.PlantID || a.JWTID == "" || len(a.JWTID) > 255 {
		docJSON(w, 401, map[string]string{"error": "ERR_F317D_ASSERTION"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	id, replayed, err := p.persistDispositionV31(ctx, c, a, token)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	status := 201
	if replayed {
		status = 200
	}
	docJSON(w, status, map[string]any{"id": id, "replayed": replayed, "discrepancy_id": c.DiscrepancyID,
		"resolution_kind": c.ResolutionKind, "authority": jobworkDispositionAuthorityV31,
		"physical_recovery_attributed_only_to_inward_evidence": c.ResolutionKind == "RETURNED_TO_PIECE_HOLD",
		"qc_release": false, "production_credit": false, "settlement": false})
}
func (p *jobworkAPI) persistDispositionV31(ctx context.Context, c jobworkDispositionCommandV31, a attestation.Claims, token string) (string, bool, error) {
	tx, err := p.holds.wip.gen.sewing.server.docTransaction(ctx, a.TenantID)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SET LOCAL ROLE agiparel_app`); err != nil {
		return "", false, err
	}
	if err = workforceChapterRequire(ctx, tx, a, "jobwork.discrepancy.resolve"); err != nil {
		return "", false, err
	}
	if err = sewTxLock(ctx, tx, a.TenantID, "F317D:COMMAND:"+c.CommandID); err != nil {
		return "", false, err
	}
	hash := sewDigest(c)
	var id, oldHash, oldSubject string
	err = tx.QueryRowContext(ctx, `SELECT id::text,command_sha256,supervisor_subject FROM jobwork_discrepancy_resolutions_v31
     WHERE tenant_id=$1 AND command_id=$2`, a.TenantID, c.CommandID).Scan(&id, &oldHash, &oldSubject)
	if err == nil {
		if oldHash != hash || oldSubject != a.Subject {
			return "", false, sewFail(409, "ERR_F317D_COMMAND_CONFLICT")
		}
		return id, true, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, err
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO jobwork_discrepancy_resolutions_v31
      (tenant_id,plant_id,discrepancy_id,outward_id,part_id,resolution_kind,evidence_ref,evidence_sha256,note,
       command_id,command_sha256,supervisor_subject,supervisor_proof_id,assertion_sha256)
       VALUES($1,$2,$3::uuid,$4::uuid,$5::uuid,$6,$7,$8,$9,$10,$11,$12,$13,$14) RETURNING id::text`,
		a.TenantID, c.PlantID, c.DiscrepancyID, c.OutwardID, c.PartID, c.ResolutionKind, c.EvidenceRef, c.EvidenceSHA256, c.Note,
		c.CommandID, hash, a.Subject, a.JWTID, sewProdHashString(token)).Scan(&id)
	if err != nil {
		return "", false, jobworkAdmissionError(err)
	}
	return id, false, tx.Commit()
}
func registerJobworkDispositionRoutesV31(mux *http.ServeMux, s *EdgeServer) {
	p := newJobworkAPI(s)
	mux.HandleFunc("/api/v1/jobwork/dispositions/challenge", p.dispositionChallengeV31)
	mux.HandleFunc("/api/v1/jobwork/dispositions", p.dispositionEventV31)
}
