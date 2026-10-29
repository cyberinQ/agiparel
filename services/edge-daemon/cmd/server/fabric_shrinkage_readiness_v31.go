// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// F3.18B. A read-only comparison of reported measurements with a versioned,
// provisioner-owned customer specification. Not a cutting release or calibrated QC.
package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"
)

const shrinkageReadinessAuthorityV31 = "PROVISIONED_BUYER_SPEC_VS_HUMAN_REPORTED_SAMPLES_NOT_CUT_RELEASE"

func validShrinkageReadinessV31(q map[string][]string) (string, string, string, string, bool) {
	for k, v := range q {
		if len(v) != 1 {
			return "", "", "", "", false
		}
		switch k {
		case "tenant_id", "plant_id", "roll_id", "treatment_code":
		default:
			return "", "", "", "", false
		}
	}
	t, p, r, tr := q["tenant_id"], q["plant_id"], q["roll_id"], q["treatment_code"]
	if len(t) != 1 || len(p) != 1 || len(r) != 1 || len(tr) != 1 || !sewFields(t[0], p[0], r[0]) {
		return "", "", "", "", false
	}
	switch tr[0] {
	case "WASH", "STEAM", "RELAXATION", "OTHER":
		return t[0], p[0], r[0], tr[0], true
	}
	return "", "", "", "", false
}

type shrinkageReadinessEvidenceV31 struct {
	Revision         int    `json:"spec_revision"`
	MinimumSamples   int    `json:"minimum_samples"`
	TotalSamples     int64  `json:"recorded_samples"`
	OutsideSamples   int64  `json:"outside_spec_samples"`
	WarpMinBP        int    `json:"warp_min_bp"`
	WarpMaxBP        int    `json:"warp_max_bp"`
	WeftMinBP        int    `json:"weft_min_bp"`
	WeftMaxBP        int    `json:"weft_max_bp"`
	SourceDocumentID string `json:"spec_source_document_id"`
	SourceSHA256     string `json:"spec_source_sha256"`
}

func evaluateShrinkageReadinessV31(e shrinkageReadinessEvidenceV31) string {
	if e.Revision <= 0 || e.MinimumSamples <= 0 {
		return "NO_CURRENT_APPROVED_SPEC"
	}
	if e.TotalSamples < int64(e.MinimumSamples) {
		return "INSUFFICIENT_REPORTED_SAMPLES"
	}
	if e.OutsideSamples > 0 {
		return "OUTSIDE_SPEC_HOLD_FOR_REVIEW"
	}
	return "WITHIN_LIMITS_REPORTED_UNCALIBRATED"
}
func (p *fabricShrinkageAPIv30) readinessV31(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	if !p.ready(w) {
		return
	}
	tenant, plant, roll, treatment, ok := validShrinkageReadinessV31(r.URL.Query())
	if !ok {
		docJSON(w, 422, map[string]string{"error": "ERR_F318B_QUERY"})
		return
	}
	if p.server == nil {
		docJSON(w, 503, map[string]string{"error": "ERR_F318B_READ_IDENTITY_DISABLED"})
		return
	}
	access := newChapterAccessAPI(p.server)
	if !access.enabled {
		docJSON(w, 503, map[string]string{"error": "ERR_F318B_READ_IDENTITY_DISABLED"})
		return
	}
	actor, err := (&commissioningAPI{access: access}).identify(r, tenant, plant)
	if err != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_F318B_AUTH"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, err := commissionTx(ctx, p.server.db, tenant)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	defer tx.Rollback()
	permitted, err := workforceChapterPermit(ctx, tx, actor, "stores.read")
	if err != nil {
		sewProdReply(w, err)
		return
	}
	if !permitted {
		docJSON(w, 403, map[string]string{"error": "ERR_F318B_READ_GRANT_REQUIRED"})
		return
	}
	var sku, inspection string
	err = tx.QueryRowContext(ctx, `SELECT factory_sku,inspection_status FROM store_rolls
      WHERE tenant_id=$1 AND plant_id=$2 AND id=$3`, tenant, plant, roll).Scan(&sku, &inspection)
	if errors.Is(err, sql.ErrNoRows) {
		docJSON(w, 404, map[string]string{"error": "ERR_F318B_ROLL_NOT_FOUND"})
		return
	}
	if err != nil {
		sewProdReply(w, err)
		return
	}
	e := shrinkageReadinessEvidenceV31{}
	var specEnabled, sourceCurrent bool
	err = tx.QueryRowContext(ctx, `SELECT s.revision,s.minimum_samples,s.warp_min_bp,s.warp_max_bp,
        s.weft_min_bp,s.weft_max_bp,s.source_document_id,s.source_sha256,s.enabled,
         (d.status='APPROVED' AND upper(d.issuer)='CUSTOMER' AND d.file_checksum=s.source_sha256)
       FROM fabric_shrinkage_specs_v31 s JOIN documents d ON d.tenant_id=s.tenant_id AND d.id=s.source_document_id
       WHERE s.tenant_id=$1 AND s.plant_id=$2 AND s.material_sku=$3 AND s.treatment_code=$4
         ORDER BY s.revision DESC LIMIT 1`, tenant, plant, sku, treatment).Scan(&e.Revision, &e.MinimumSamples, &e.WarpMinBP, &e.WarpMaxBP,
		&e.WeftMinBP, &e.WeftMaxBP, &e.SourceDocumentID, &e.SourceSHA256, &specEnabled, &sourceCurrent)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		sewProdReply(w, err)
		return
	}
	if err == nil && specEnabled && sourceCurrent {
		err = tx.QueryRowContext(ctx, `SELECT count(*)::bigint,
        count(*) FILTER(WHERE warp_change_bp<$5 OR warp_change_bp>$6 OR weft_change_bp<$7 OR weft_change_bp>$8)::bigint
        FROM fabric_shrinkage_observations_v30
        WHERE tenant_id=$1 AND plant_id=$2 AND roll_id=$3 AND treatment_code=$4
          AND material_sku=$9`, tenant, plant, roll, treatment, e.WarpMinBP, e.WarpMaxBP, e.WeftMinBP, e.WeftMaxBP, sku).
			Scan(&e.TotalSamples, &e.OutsideSamples)
		if err != nil {
			sewProdReply(w, err)
			return
		}
	}
	if !specEnabled || !sourceCurrent {
		e = shrinkageReadinessEvidenceV31{}
	}
	if err = tx.Commit(); err != nil {
		sewProdReply(w, err)
		return
	}
	state := evaluateShrinkageReadinessV31(e)
	if inspection != "PASS" {
		state = "ROLL_INSPECTION_NOT_PASS"
	}
	docJSON(w, 200, map[string]any{"tenant_id": tenant, "plant_id": plant, "roll_id": roll,
		"material_sku": sku, "treatment_code": treatment, "roll_inspection_status": inspection,
		"readiness": state, "evidence": e, "authority": shrinkageReadinessAuthorityV31,
		"instrument_calibration_verified": false, "cut_release": false, "buyer_quality_release": false,
		"measurement_source": "HUMAN_ATTESTED_DIMENSIONS"})
}
func registerFabricShrinkageReadinessRoutesV31(mux *http.ServeMux, s *EdgeServer) {
	p := newFabricShrinkageAPIv30(s)
	mux.HandleFunc("/api/v1/fabric/shrinkage/readiness", p.readinessV31)
}
