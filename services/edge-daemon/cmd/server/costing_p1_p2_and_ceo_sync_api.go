// Author: Sunil Jain, CyberinQ LLC (Generated 2026-11-24)
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/costing"
)

func registerCostingP1P2AndCEOSyncRoutesV53(mux *http.ServeMux, s *EdgeServer) {
	mux.HandleFunc("/api/v1/costing/cut-tag-preview", s.handleCostingCutTagPreviewV53)
	mux.HandleFunc("/api/v1/costing/carton-preview", s.handleCostingCartonPreviewV53)
	mux.HandleFunc("/api/v1/costing/scenarios", s.handleCostingScenariosV53)
	mux.HandleFunc("/api/v1/costing/sampling-tna", s.handleCostingSamplingTNAV53)
	mux.HandleFunc("/api/v1/costing/ocr-variance", s.handleCostingOCRVarianceV53)

	mux.HandleFunc("/api/v1/sync/factory-push", s.handleFactoryToCloudSyncV53)
	mux.HandleFunc("/api/v1/ceo/six-stage-board", s.handleCEOSixStageDashboardV53)
}

func (s *EdgeServer) handleCostingCutTagPreviewV53(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tenant, err := intakeTenant()
	if err != nil || s.db == nil {
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}
	var req struct {
		VersionID          string  `json:"version_id"`
		AllowancePct       float64 `json:"allowance_pct"`
		MinExtraPcs        int     `json:"min_extra_pcs"`
		MaxExtraPcs        int     `json:"max_extra_pcs"`
		MaxPlies           int     `json:"max_plies"`
		BundleSize         int     `json:"bundle_size"`
		CutPartsPerGarment int     `json:"cut_parts_per_garment"`
		MinEconomicCutPcs  int     `json:"min_economic_cut_pcs"`
		RFIDTagUnitCost    float64 `json:"rfid_tag_unit_cost"`
	}
	if !candidateJSONBody(w, r, 32768, &req) || strings.TrimSpace(req.VersionID) == "" {
		http.Error(w, "version_id required", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, err := s.docTransaction(ctx, tenant)
	if err != nil {
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		SELECT colour, size, upc, qty
		FROM costing_order_matrix_lines_v52
		WHERE tenant_id=$1 AND version_id=$2::uuid`, tenant, req.VersionID)
	if err != nil {
		http.Error(w, "Failed to load order matrix", http.StatusServiceUnavailable)
		return
	}
	matrix := []costing.OrderMatrixLine{}
	for rows.Next() {
		var m costing.OrderMatrixLine
		if rows.Scan(&m.Colour, &m.Size, &m.UPC, &m.Qty) == nil {
			matrix = append(matrix, m)
		}
	}
	rows.Close()

	rule := costing.CutPreviewRule{
		AllowancePct:       nonZeroFloat(req.AllowancePct, 0.02),
		MinExtraPcs:        nonZeroInt(req.MinExtraPcs, 1),
		MaxExtraPcs:        nonZeroInt(req.MaxExtraPcs, 15),
		MaxPlies:           nonZeroInt(req.MaxPlies, 210),
		BundleSize:         nonZeroInt(req.BundleSize, 20),
		CutPartsPerGarment: nonZeroInt(req.CutPartsPerGarment, 16),
		MinEconomicCutPcs:  nonZeroInt(req.MinEconomicCutPcs, 10),
		RFIDTagUnitCost:    req.RFIDTagUnitCost,
	}
	res, err := costing.ComputeCutAndTagPreview(matrix, rule)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	bWarn, _ := json.Marshal(res.Warnings)
	bBrk, _ := json.Marshal(res.Breakdown)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO costing_cut_previews_v53 (
			tenant_id, version_id, allowance_pct, min_extra_pcs, max_extra_pcs, max_plies,
			bundle_size, cut_parts_per_garment, total_order_qty, total_cut_qty,
			estimated_cuts, estimated_bundles, garment_rfid_tags, bundle_rfid_tags, total_rfid_tags,
			warnings, breakdown, computed_at
		) VALUES ($1,$2::uuid,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16::jsonb,$17::jsonb,clock_timestamp())
		ON CONFLICT (tenant_id, version_id) DO UPDATE SET
			allowance_pct=EXCLUDED.allowance_pct, min_extra_pcs=EXCLUDED.min_extra_pcs,
			max_extra_pcs=EXCLUDED.max_extra_pcs, total_cut_qty=EXCLUDED.total_cut_qty,
			estimated_cuts=EXCLUDED.estimated_cuts, estimated_bundles=EXCLUDED.estimated_bundles,
			garment_rfid_tags=EXCLUDED.garment_rfid_tags, bundle_rfid_tags=EXCLUDED.bundle_rfid_tags,
			total_rfid_tags=EXCLUDED.total_rfid_tags, warnings=EXCLUDED.warnings,
			breakdown=EXCLUDED.breakdown, computed_at=clock_timestamp()`,
		tenant, req.VersionID, rule.AllowancePct, rule.MinExtraPcs, rule.MaxExtraPcs, rule.MaxPlies,
		rule.BundleSize, rule.CutPartsPerGarment, res.TotalOrderQty, res.TotalCutQty,
		res.EstimatedCuts, res.EstimatedBundles, res.GarmentRFIDTags, res.BundleRFIDTags,
		res.TotalRFIDTags, string(bWarn), string(bBrk))
	if err != nil {
		http.Error(w, "Failed to save cut/tag preview: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	_ = tx.Commit()
	docJSON(w, http.StatusOK, res)
}

func (s *EdgeServer) handleCostingCartonPreviewV53(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tenant, err := intakeTenant()
	if err != nil || s.db == nil {
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}
	var req struct {
		VersionID       string                   `json:"version_id"`
		DeclaredCartons int                      `json:"declared_cartons"`
		Plans           []costing.CartonPlanLine `json:"plans"`
	}
	if !candidateJSONBody(w, r, 131072, &req) || strings.TrimSpace(req.VersionID) == "" {
		http.Error(w, "version_id and plans required", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, err := s.docTransaction(ctx, tenant)
	if err != nil {
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		SELECT colour, size, upc, qty
		FROM costing_order_matrix_lines_v52
		WHERE tenant_id=$1 AND version_id=$2::uuid`, tenant, req.VersionID)
	if err != nil {
		http.Error(w, "Failed to load order matrix", http.StatusServiceUnavailable)
		return
	}
	matrix := []costing.OrderMatrixLine{}
	for rows.Next() {
		var m costing.OrderMatrixLine
		if rows.Scan(&m.Colour, &m.Size, &m.UPC, &m.Qty) == nil {
			matrix = append(matrix, m)
		}
	}
	rows.Close()

	res, err := costing.ComputeCartonPlanPreview(matrix, req.DeclaredCartons, req.Plans)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	bGaps, _ := json.Marshal(res.Gaps)
	bLines, _ := json.Marshal(res.Lines)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO costing_carton_plans_v53 (
			tenant_id, version_id, assortment_mode, total_order_qty, packed_qty,
			declared_cartons, computed_cartons, total_cbm, total_gross_weight_kg,
			total_carton_cost, is_complete, gaps, lines, computed_at
		) VALUES ($1,$2::uuid,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb,$13::jsonb,clock_timestamp())
		ON CONFLICT (tenant_id, version_id) DO UPDATE SET
			assortment_mode=EXCLUDED.assortment_mode, packed_qty=EXCLUDED.packed_qty,
			declared_cartons=EXCLUDED.declared_cartons, computed_cartons=EXCLUDED.computed_cartons,
			total_cbm=EXCLUDED.total_cbm, total_gross_weight_kg=EXCLUDED.total_gross_weight_kg,
			total_carton_cost=EXCLUDED.total_carton_cost, is_complete=EXCLUDED.is_complete,
			gaps=EXCLUDED.gaps, lines=EXCLUDED.lines, computed_at=clock_timestamp()`,
		tenant, req.VersionID, res.AssortmentMode, res.TotalOrderQty, res.PackedQty,
		res.DeclaredCartons, res.ComputedCartons, res.TotalCBM, res.TotalGrossWeightKG,
		res.TotalCartonCost, res.IsComplete, string(bGaps), string(bLines))
	if err != nil {
		http.Error(w, "Failed to save carton plan: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	_ = tx.Commit()
	docJSON(w, http.StatusOK, res)
}

func (s *EdgeServer) handleCostingScenariosV53(w http.ResponseWriter, r *http.Request) {
	tenant, err := intakeTenant()
	if err != nil || s.db == nil {
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, err := s.docTransaction(ctx, tenant)
	if err != nil {
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}
	defer tx.Rollback()

	if r.Method == http.MethodGet {
		sheetID := strings.TrimSpace(r.URL.Query().Get("sheet_id"))
		rows, err := tx.QueryContext(ctx, `
			SELECT id::text, version_no, scenario_slot, name, parameters, is_quoted, created_at
			FROM costing_scenarios_v52
			WHERE tenant_id=$1 AND sheet_id=$2::uuid
			ORDER BY version_no DESC, scenario_slot ASC`, tenant, sheetID)
		if err != nil {
			http.Error(w, "Failed to query scenarios", http.StatusServiceUnavailable)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id, name string
			var verNo, slot int
			var rawParams []byte
			var quoted bool
			var created time.Time
			if rows.Scan(&id, &verNo, &slot, &name, &rawParams, &quoted, &created) == nil {
				var params any
				_ = json.Unmarshal(rawParams, &params)
				out = append(out, map[string]any{
					"id": id, "version_no": verNo, "scenario_slot": slot,
					"name": name, "parameters": params, "is_quoted": quoted, "created_at": created,
				})
			}
		}
		_ = tx.Commit()
		docJSON(w, http.StatusOK, map[string]any{"scenarios": out})
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		SheetID      string `json:"sheet_id"`
		VersionNo    int    `json:"version_no"`
		ScenarioSlot int    `json:"scenario_slot"`
		Name         string `json:"name"`
		Parameters   any    `json:"parameters"`
		IsQuoted     bool   `json:"is_quoted"`
		ActorID      string `json:"actor_id"`
	}
	if !candidateJSONBody(w, r, 65536, &req) {
		return
	}
	if req.ScenarioSlot < 1 || req.ScenarioSlot > 3 {
		http.Error(w, "ERR_FR11_MAX_THREE_SCENARIOS_PER_SHEET", http.StatusUnprocessableEntity)
		return
	}
	if req.IsQuoted {
		_, _ = tx.ExecContext(ctx, `
			UPDATE costing_scenarios_v52 SET is_quoted=false
			WHERE tenant_id=$1 AND sheet_id=$2::uuid AND version_no=$3`,
			tenant, req.SheetID, nonZeroInt(req.VersionNo, 1))
	}
	bParams, _ := json.Marshal(req.Parameters)
	var id string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO costing_scenarios_v52 (
			tenant_id, sheet_id, version_no, scenario_slot, name, parameters, is_quoted, created_by
		) VALUES ($1,$2::uuid,$3,$4,$5,$6::jsonb,$7,$8)
		ON CONFLICT (tenant_id, sheet_id, version_no, scenario_slot) DO UPDATE SET
			name=EXCLUDED.name, parameters=EXCLUDED.parameters, is_quoted=EXCLUDED.is_quoted
		RETURNING id::text`,
		tenant, req.SheetID, nonZeroInt(req.VersionNo, 1), req.ScenarioSlot,
		nonEmptyOr(req.Name, "Scenario"), string(bParams), req.IsQuoted,
		nonEmptyOr(req.ActorID, "merchandiser")).Scan(&id)
	if err != nil {
		http.Error(w, "Failed to save scenario: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}
	_ = tx.Commit()
	docJSON(w, http.StatusCreated, map[string]any{"scenario_id": id, "scenario_slot": req.ScenarioSlot, "is_quoted": req.IsQuoted})
}

func (s *EdgeServer) handleCostingSamplingTNAV53(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tenant, err := intakeTenant()
	if err != nil || s.db == nil {
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}
	var req struct {
		VersionID   string         `json:"version_id"`
		OffsetsDays map[string]int `json:"offsets_days,omitempty"`
	}
	if !candidateJSONBody(w, r, 16384, &req) || strings.TrimSpace(req.VersionID) == "" {
		http.Error(w, "version_id required", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, err := s.docTransaction(ctx, tenant)
	if err != nil {
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}
	defer tx.Rollback()

	var ordDate, cutDate time.Time
	err = tx.QueryRowContext(ctx, `
		SELECT cs.order_date, pr.latest_plan_cut_date
		FROM costing_versions_v52 cv
		JOIN costing_sheets_v52 cs ON cs.tenant_id=cv.tenant_id AND cs.id=cv.sheet_id
		JOIN LATERAL (
			SELECT latest_plan_cut_date FROM costing_preview_runs_v52
			WHERE tenant_id=cv.tenant_id AND version_id=cv.id ORDER BY run_at DESC LIMIT 1
		) pr ON true
		WHERE cv.tenant_id=$1 AND cv.id=$2::uuid`, tenant, req.VersionID).Scan(&ordDate, &cutDate)
	if err != nil {
		http.Error(w, "Run planning preview before generating sampling TNA", http.StatusNotFound)
		return
	}

	tna := costing.ComputeSamplingTNA(ordDate, cutDate, time.Now().UTC(), req.OffsetsDays)
	bMiles, _ := json.Marshal(tna.Milestones)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO costing_sampling_tna_v53 (
			tenant_id, version_id, order_date, plan_cut_date, milestones, has_past_or_pre_order_flag, computed_at
		) VALUES ($1,$2::uuid,$3,$4,$5::jsonb,$6,clock_timestamp())
		ON CONFLICT (tenant_id, version_id) DO UPDATE SET
			order_date=EXCLUDED.order_date, plan_cut_date=EXCLUDED.plan_cut_date,
			milestones=EXCLUDED.milestones, has_past_or_pre_order_flag=EXCLUDED.has_past_or_pre_order_flag,
			computed_at=clock_timestamp()`,
		tenant, req.VersionID, ordDate, cutDate, string(bMiles), tna.HasPastOrPreOrderFlags)
	if err != nil {
		http.Error(w, "Failed to store sampling TNA: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	_ = tx.Commit()
	docJSON(w, http.StatusOK, tna)
}

func (s *EdgeServer) handleCostingOCRVarianceV53(w http.ResponseWriter, r *http.Request) {
	tenant, err := intakeTenant()
	if err != nil || s.db == nil {
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, err := s.docTransaction(ctx, tenant)
	if err != nil {
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}
	defer tx.Rollback()

	if r.Method == http.MethodGet {
		rows, err := tx.QueryContext(ctx, `
			SELECT po_id, costed_qty, actual_shipped_qty, costed_sam::double precision,
			       actual_avg_sam::double precision, cm_variance_pct::double precision,
			       material_variance_pct::double precision, schedule_variance_days, shipped_on_time, recorded_at
			FROM costing_ocr_actuals_v53
			WHERE tenant_id=$1 ORDER BY recorded_at DESC LIMIT 100`, tenant)
		if err != nil {
			http.Error(w, "Failed to query OCR variances", http.StatusServiceUnavailable)
			return
		}
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var po string
			var cQty, aQty, schedDays int
			var cSAM, aSAM, cmVar, matVar float64
			var onTime bool
			var recAt time.Time
			if rows.Scan(&po, &cQty, &aQty, &cSAM, &aSAM, &cmVar, &matVar, &schedDays, &onTime, &recAt) == nil {
				items = append(items, map[string]any{
					"po_id": po, "costed_qty": cQty, "actual_shipped_qty": aQty,
					"costed_sam": cSAM, "actual_avg_sam": aSAM, "cm_variance_pct": cmVar,
					"material_variance_pct": matVar, "schedule_variance_days": schedDays,
					"shipped_on_time": onTime, "recorded_at": recAt,
				})
			}
		}
		_ = tx.Commit()
		docJSON(w, http.StatusOK, map[string]any{"ocr_variances": items})
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		POID                string  `json:"po_id"`
		ActualShippedQty    int     `json:"actual_shipped_qty"`
		ActualAvgSAM        float64 `json:"actual_avg_sam"`
		ActualCMTotal       float64 `json:"actual_cm_total"`
		ActualMaterialTotal float64 `json:"actual_material_total"`
		ActualExFactory     string  `json:"actual_ex_factory"`
	}
	if !candidateJSONBody(w, r, 32768, &req) {
		return
	}
	actExf, err := time.Parse("2006-01-02", req.ActualExFactory)
	if err != nil {
		http.Error(w, "Invalid actual_ex_factory ISO date", http.StatusBadRequest)
		return
	}

	var baseID string
	var cQty int
	var cSAM, cCMPc, cMatPc float64
	var qShip time.Time
	err = tx.QueryRowContext(ctx, `
		SELECT pb.id::text, ps.total_qty, ob.total_sam::double precision,
		       ps.cm_pc::double precision, ps.material_pc::double precision, cs.requested_ship_date
		FROM planning_baselines_v52 pb
		JOIN costing_sheets_v52 cs ON cs.tenant_id=pb.tenant_id AND cs.id=pb.sheet_id
		JOIN costing_price_summaries_v52 ps ON ps.tenant_id=pb.tenant_id AND ps.version_id=pb.version_id
		JOIN costing_ob_pins_v52 ob ON ob.tenant_id=pb.tenant_id AND ob.version_id=pb.version_id
		WHERE pb.tenant_id=$1 AND pb.po_id=$2`, tenant, req.POID).Scan(&baseID, &cQty, &cSAM, &cCMPc, &cMatPc, &qShip)
	if err != nil {
		http.Error(w, "Confirmed PlanningBaseline not found for PO", http.StatusNotFound)
		return
	}

	v, err := costing.ComputeOCRCostedVsActual(costing.OCRActualInput{
		POID: req.POID, CostedQty: cQty, ActualShippedQty: req.ActualShippedQty,
		CostedSAM: cSAM, ActualAvgSAM: req.ActualAvgSAM,
		CostedCMTotal: cCMPc * float64(cQty), ActualCMTotal: req.ActualCMTotal,
		CostedMaterialTotal: cMatPc * float64(cQty), ActualMaterialTotal: req.ActualMaterialTotal,
		QuotedExFactory: qShip.AddDate(0, 0, -2), ActualExFactory: actExf,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO costing_ocr_actuals_v53 (
			tenant_id, po_id, baseline_id, costed_qty, actual_shipped_qty, costed_sam, actual_avg_sam,
			costed_cm_total, actual_cm_total, costed_material_total, actual_material_total,
			quoted_ex_factory, actual_ex_factory, cm_variance_pct, material_variance_pct,
			schedule_variance_days, shipped_on_time
		) VALUES ($1,$2,$3::uuid,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		ON CONFLICT (tenant_id, po_id) DO UPDATE SET
			actual_shipped_qty=EXCLUDED.actual_shipped_qty, actual_avg_sam=EXCLUDED.actual_avg_sam,
			actual_cm_total=EXCLUDED.actual_cm_total, actual_material_total=EXCLUDED.actual_material_total,
			actual_ex_factory=EXCLUDED.actual_ex_factory, cm_variance_pct=EXCLUDED.cm_variance_pct,
			material_variance_pct=EXCLUDED.material_variance_pct, schedule_variance_days=EXCLUDED.schedule_variance_days,
			shipped_on_time=EXCLUDED.shipped_on_time, recorded_at=clock_timestamp()`,
		tenant, req.POID, baseID, v.CostedQty, v.ActualShippedQty, v.CostedSAM, v.ActualAvgSAM,
		v.CostedCMTotal, v.ActualCMTotal, v.CostedMaterialTotal, v.ActualMaterialTotal,
		v.QuotedExFactory, v.ActualExFactory, v.CMVariancePct, v.MaterialVariancePct,
		v.ScheduleVarianceDay, v.ShippedOnTime)
	if err != nil {
		http.Error(w, "Failed to record OCR variance: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	_ = tx.Commit()
	docJSON(w, http.StatusOK, v)
}

func (s *EdgeServer) handleFactoryToCloudSyncV53(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tenant, err := intakeTenant()
	if err != nil || s.db == nil {
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}
	var req struct {
		FactoryID   string `json:"factory_id"`
		FactoryName string `json:"factory_name"`
		Events      []struct {
			EventID     string `json:"event_id"`
			StreamName  string `json:"stream_name"`
			AggregateID string `json:"aggregate_id"`
			Payload     any    `json:"payload"`
			OccurredAt  string `json:"occurred_at"`
		} `json:"events"`
	}
	if !candidateJSONBody(w, r, 524288, &req) || strings.TrimSpace(req.FactoryID) == "" {
		http.Error(w, "factory_id required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	tx, err := s.docTransaction(ctx, tenant)
	if err != nil {
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}
	defer tx.Rollback()

	accepted := 0
	duplicates := 0
	for _, ev := range req.Events {
		occ, err := time.Parse(time.RFC3339, ev.OccurredAt)
		if err != nil {
			occ = time.Now().UTC()
		}
		bPay, _ := json.Marshal(ev.Payload)
		res, err := tx.ExecContext(ctx, `
			INSERT INTO cloud_sync_outbox_v53 (
				event_id, tenant_id, factory_id, stream_name, aggregate_id, payload, occurred_at, synced_at
			) VALUES ($1::uuid,$2,$3,$4,$5,$6::jsonb,$7,clock_timestamp())
			ON CONFLICT (event_id) DO NOTHING`,
			ev.EventID, tenant, req.FactoryID, nonEmptyOr(ev.StreamName, "SHOPFLOOR"),
			ev.AggregateID, string(bPay), occ)
		if err == nil {
			if n, _ := res.RowsAffected(); n > 0 {
				accepted++
			} else {
				duplicates++
			}
		}
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO cloud_sync_factory_status_v53 (
			tenant_id, factory_id, factory_name, last_sync_at, pending_events, last_batch_count, sync_status
		) VALUES ($1,$2,$3,clock_timestamp(),0,$4,'HEALTHY')
		ON CONFLICT (tenant_id, factory_id) DO UPDATE SET
			factory_name=EXCLUDED.factory_name, last_sync_at=clock_timestamp(),
			pending_events=0, last_batch_count=EXCLUDED.last_batch_count, sync_status='HEALTHY'`,
		tenant, req.FactoryID, nonEmptyOr(req.FactoryName, req.FactoryID), accepted)
	if err != nil {
		http.Error(w, "Failed to update factory sync status: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	_ = tx.Commit()
	docJSON(w, http.StatusOK, map[string]any{
		"factory_id":         req.FactoryID,
		"accepted_events":    accepted,
		"deduplicated_count": duplicates,
		"sync_cadence_min":   10,
		"synced_at":          time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *EdgeServer) handleCEOSixStageDashboardV53(w http.ResponseWriter, r *http.Request) {
	tenant, err := intakeTenant()
	if err != nil || s.db == nil {
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	tx, err := s.docTransaction(ctx, tenant)
	if err != nil {
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}
	defer tx.Rollback()

	if r.Method == http.MethodPost {
		var req struct {
			FactoryID           string `json:"factory_id"`
			FactoryName         string `json:"factory_name"`
			POID                string `json:"po_id"`
			BuyerID             string `json:"buyer_id"`
			StyleID             string `json:"style_id"`
			PlanDate            string `json:"plan_date"`
			Stage               string `json:"stage"`
			PlannedQty          int    `json:"planned_qty"`
			ActualQty           int    `json:"actual_qty"`
			ShortfallReasonCode string `json:"shortfall_reason_code"`
			ShortfallNote       string `json:"shortfall_note"`
			RecordedBy          string `json:"recorded_by"`
		}
		if !candidateJSONBody(w, r, 32768, &req) {
			return
		}
		if req.ActualQty < req.PlannedQty && strings.TrimSpace(req.ShortfallReasonCode) == "" {
			http.Error(w, "ERR_CEO_BOARD_SHORTFALL_REASON_REQUIRED_WHEN_ACTUAL_BELOW_PLAN", http.StatusUnprocessableEntity)
			return
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO factory_stage_daily_plan_v53 (
				tenant_id, factory_id, factory_name, po_id, buyer_id, style_id, plan_date,
				stage, planned_qty, actual_qty, shortfall_reason_code, shortfall_note, shortfall_recorded_by, updated_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7::date,$8,$9,$10,$11,$12,$13,clock_timestamp())
			ON CONFLICT (tenant_id, factory_id, po_id, plan_date, stage) DO UPDATE SET
				planned_qty=EXCLUDED.planned_qty, actual_qty=EXCLUDED.actual_qty,
				shortfall_reason_code=EXCLUDED.shortfall_reason_code,
				shortfall_note=EXCLUDED.shortfall_note,
				shortfall_recorded_by=EXCLUDED.shortfall_recorded_by,
				updated_at=clock_timestamp()`,
			tenant, req.FactoryID, nonEmptyOr(req.FactoryName, req.FactoryID), req.POID,
			req.BuyerID, req.StyleID, req.PlanDate, strings.ToUpper(req.Stage),
			req.PlannedQty, req.ActualQty, strings.ToUpper(req.ShortfallReasonCode),
			req.ShortfallNote, req.RecordedBy)
		if err != nil {
			http.Error(w, "Failed to upsert 6-stage plan/actual row: "+err.Error(), http.StatusUnprocessableEntity)
			return
		}
		_ = tx.Commit()
		docJSON(w, http.StatusOK, map[string]any{"status": "RECORDED"})
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fRows, _ := tx.QueryContext(ctx, `
		SELECT factory_id, factory_name, last_sync_at, pending_events, sync_status
		FROM cloud_sync_factory_status_v53
		WHERE tenant_id=$1 ORDER BY factory_id ASC`, tenant)
	freshness := []map[string]any{}
	if fRows != nil {
		for fRows.Next() {
			var fID, fName, status string
			var lastSync time.Time
			var pending int
			if fRows.Scan(&fID, &fName, &lastSync, &pending, &status) == nil {
				stale := time.Since(lastSync) > 15*time.Minute
				freshness = append(freshness, map[string]any{
					"factory_id": fID, "factory_name": fName,
					"last_sync_at": lastSync, "pending_events": pending,
					"sync_status": status, "is_stale_over_15m": stale,
				})
			}
		}
		fRows.Close()
	}

	sRows, err := tx.QueryContext(ctx, `
		SELECT factory_id, factory_name, po_id, buyer_id, style_id, plan_date::text,
		       stage, planned_qty, actual_qty, (actual_qty - planned_qty) AS variance_qty,
		       shortfall_reason_code, shortfall_note, shortfall_recorded_by
		FROM factory_stage_daily_plan_v53
		WHERE tenant_id=$1
		ORDER BY plan_date DESC, factory_id ASC, po_id ASC,
		         CASE stage
		           WHEN 'RAW_MATERIAL_ARRIVAL' THEN 1
		           WHEN 'CUTTING' THEN 2
		           WHEN 'SEWING' THEN 3
		           WHEN 'FINISHING' THEN 4
		           WHEN 'PACKING' THEN 5
		           WHEN 'SHIPMENT' THEN 6
		           ELSE 7
		         END`, tenant)
	if err != nil {
		http.Error(w, "Failed to query CEO 6-stage board", http.StatusServiceUnavailable)
		return
	}
	defer sRows.Close()
	stages := []map[string]any{}
	for sRows.Next() {
		var fID, fName, po, buyer, style, pDate, stg, rCode, rNote, rBy string
		var pQty, aQty, vQty int
		if sRows.Scan(&fID, &fName, &po, &buyer, &style, &pDate, &stg, &pQty, &aQty, &vQty, &rCode, &rNote, &rBy) == nil {
			stages = append(stages, map[string]any{
				"factory_id": fID, "factory_name": fName, "po_id": po, "buyer_id": buyer,
				"style_id": style, "plan_date": pDate, "stage": stg,
				"planned_qty": pQty, "actual_qty": aQty, "variance_qty": vQty,
				"shortfall_reason_code": rCode, "shortfall_note": rNote, "shortfall_recorded_by": rBy,
			})
		}
	}
	_ = tx.Commit()
	docJSON(w, http.StatusOK, map[string]any{
		"sync_interval_minutes": 10,
		"factory_freshness":     freshness,
		"six_stage_board":       stages,
	})
}
