// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// F3.18A: independently signed human-reported measured sample dimensions.
// No lab calibration, buyer-spec approval, cut release, or inferred fabric yield.
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

const shrinkageAuthorityV30 = "SIGNED_HUMAN_REPORTED_SHRINKAGE_NOT_CALIBRATED_OR_BUYER_APPROVED"

type fabricShrinkageCommandV30 struct {
	CommandID        string `json:"command_id"`
	PlantID          string `json:"plant_id"`
	RollID           string `json:"roll_id"`
	MaterialSKU      string `json:"material_sku"`
	SampleID         string `json:"sample_id"`
	TreatmentCode    string `json:"treatment_code"`
	WarpBeforeMM     int    `json:"warp_before_mm"`
	WarpAfterMM      int    `json:"warp_after_mm"`
	WeftBeforeMM     int    `json:"weft_before_mm"`
	WeftAfterMM      int    `json:"weft_after_mm"`
	SourceDocumentID string `json:"source_document_id"`
	SourceSHA256     string `json:"source_sha256"`
	MethodNote       string `json:"method_note"`
}
type fabricShrinkageRequestV30 struct {
	Command fabricShrinkageCommandV30 `json:"command"`
}

func shrinkageValidV30(c fabricShrinkageCommandV30) bool {
	if !sewCommandID.MatchString(c.CommandID) || !sewFields(c.PlantID, c.RollID, c.MaterialSKU, c.SampleID) ||
		len(c.SampleID) < 4 || len(c.SampleID) > 80 || len(c.SourceDocumentID) < 1 || len(c.SourceDocumentID) > 36 ||
		!f317bHash.MatchString(c.SourceSHA256) || len(strings.TrimSpace(c.MethodNote)) < 20 || len(c.MethodNote) > 800 ||
		strings.ContainsRune(c.MethodNote, 0) {
		return false
	}
	switch c.TreatmentCode {
	case "WASH", "STEAM", "RELAXATION", "OTHER":
	default:
		return false
	}
	for _, v := range []int{c.WarpBeforeMM, c.WeftBeforeMM} {
		if v < 50 || v > 10000 {
			return false
		}
	}
	for _, v := range []int{c.WarpAfterMM, c.WeftAfterMM} {
		if v < 25 || v > 15000 {
			return false
		}
	}
	return absShrinkageV30(c.WarpBeforeMM-c.WarpAfterMM)*100 <= c.WarpBeforeMM*30 &&
		absShrinkageV30(c.WeftBeforeMM-c.WeftAfterMM)*100 <= c.WeftBeforeMM*30
}
func absShrinkageV30(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
func shrinkageChallengeV30(c fabricShrinkageCommandV30) string {
	return "FABRIC_SHRINKAGE_V30:" + sewDigest(c)
}

type fabricShrinkageAPIv30 struct {
	server   *EdgeServer
	material *materialAPI
	enabled  bool
}

func newFabricShrinkageAPIv30(s *EdgeServer) *fabricShrinkageAPIv30 {
	p := &fabricShrinkageAPIv30{server: s}
	if s == nil || s.db == nil || os.Getenv("AGIPAREL_FABRIC_SHRINKAGE_ENABLED") != "true" ||
		(os.Getenv("ENV") != "staging" && os.Getenv("ENV") != "production") {
		return p
	}
	p.material = newMaterialAPI(s.db)
	p.enabled = p.material.initErr == nil && p.material.verifier != nil
	return p
}
func (p *fabricShrinkageAPIv30) ready(w http.ResponseWriter) bool {
	if p == nil || !p.enabled {
		docJSON(w, 503, map[string]string{"error": "ERR_F318A_DISABLED"})
		return false
	}
	return true
}
func (p *fabricShrinkageAPIv30) challenge(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var q fabricShrinkageRequestV30
	if !sewProdStrictJSON(w, r, &q) {
		return
	}
	if !shrinkageValidV30(q.Command) {
		docJSON(w, 422, map[string]string{"error": "ERR_F318A_FIELDS"})
		return
	}
	docJSON(w, 200, map[string]any{"challenge": shrinkageChallengeV30(q.Command),
		"required_action": "stores.fabric.shrinkage.record", "required_role": "stores_inspector",
		"authority": shrinkageAuthorityV30, "buyer_spec_conformance": false,
		"calibrated_instrument_verified": false, "cut_release": false})
}
func (p *fabricShrinkageAPIv30) event(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var q fabricShrinkageRequestV30
	if !sewProdStrictJSON(w, r, &q) {
		return
	}
	c := q.Command
	if !shrinkageValidV30(c) {
		docJSON(w, 422, map[string]string{"error": "ERR_F318A_FIELDS"})
		return
	}
	token, err := bearer(r)
	if err != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_F318A_AUTH"})
		return
	}
	a, err := p.material.verifier.Verify(token, "stores.fabric.shrinkage.record", shrinkageChallengeV30(c), "stores_inspector", true)
	if err != nil || !validChapterActor(a) || a.PlantID != c.PlantID || a.JWTID == "" || len(a.JWTID) > 255 {
		docJSON(w, 401, map[string]string{"error": "ERR_F318A_ASSERTION"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	id, replay, err := p.persist(ctx, c, a, token)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	status := 201
	if replay {
		status = 200
	}
	docJSON(w, status, map[string]any{"id": id, "replayed": replay, "roll_id": c.RollID,
		"warp_change_bp": shrinkageBasisPointsV30(c.WarpBeforeMM, c.WarpAfterMM),
		"weft_change_bp": shrinkageBasisPointsV30(c.WeftBeforeMM, c.WeftAfterMM),
		"authority":      shrinkageAuthorityV30, "cut_release": false, "buyer_approved": false})
}
func shrinkageBasisPointsV30(before, after int) int {
	d := (before - after) * 10000
	if d >= 0 {
		return (d + before/2) / before
	}
	return -((-d + before/2) / before)
}
func (p *fabricShrinkageAPIv30) persist(ctx context.Context, c fabricShrinkageCommandV30, a attestation.Claims, token string) (string, bool, error) {
	tx, err := p.server.docTransaction(ctx, a.TenantID)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SET LOCAL ROLE agiparel_app`); err != nil {
		return "", false, err
	}
	if err = workforceChapterRequire(ctx, tx, a, "stores.fabric.shrinkage.record"); err != nil {
		return "", false, err
	}
	if err = sewTxLock(ctx, tx, a.TenantID, "F318A:COMMAND:"+c.CommandID); err != nil {
		return "", false, err
	}
	hash := sewDigest(c)
	var id, oldHash, oldSubject string
	err = tx.QueryRowContext(ctx, `SELECT id::text,command_sha256,tester_subject FROM fabric_shrinkage_observations_v30
 WHERE tenant_id=$1 AND command_id=$2`, a.TenantID, c.CommandID).Scan(&id, &oldHash, &oldSubject)
	if err == nil {
		if oldHash != hash || oldSubject != a.Subject {
			return "", false, sewFail(409, "ERR_F318A_COMMAND_CONFLICT")
		}
		return id, true, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, err
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO fabric_shrinkage_observations_v30
 (tenant_id,plant_id,roll_id,material_sku,sample_id,treatment_code,warp_before_mm,
  warp_after_mm,weft_before_mm,weft_after_mm,source_document_id,source_sha256,method_note,
  command_id,command_sha256,tester_subject,tester_proof_id,assertion_sha256)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18) RETURNING id::text`,
		a.TenantID, c.PlantID, c.RollID, c.MaterialSKU, c.SampleID, c.TreatmentCode, c.WarpBeforeMM,
		c.WarpAfterMM, c.WeftBeforeMM, c.WeftAfterMM, c.SourceDocumentID, c.SourceSHA256, c.MethodNote,
		c.CommandID, hash, a.Subject, a.JWTID, sewProdHashString(token)).Scan(&id)
	if err != nil {
		return "", false, jobworkAdmissionError(err)
	}
	return id, false, tx.Commit()
}
func registerFabricShrinkageRoutesV30(mux *http.ServeMux, s *EdgeServer) {
	p := newFabricShrinkageAPIv30(s)
	mux.HandleFunc("/api/v1/fabric/shrinkage/challenge", p.challenge)
	mux.HandleFunc("/api/v1/fabric/shrinkage/observations", p.event)
}
