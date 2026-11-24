// Author: Sunil Jain, CyberinQ LLC (Generated 2026-11-24)
// HTTP handlers for:
//   - Costing with Planning Preview (Costing PRD v0.5, S1-S5, FR-1..FR-13, D1..D15, V1..V14)
//   - Assigned OB Lifecycle & Absenteeism Shuffles (Costing PRD §4.3, D13-D15, V13-V14)
//   - Shop-Floor Checker Back-Credit, Lost-Time Stoppages, Operator Skill Matrix & DHU,
//     Outside-Process Challans & Billing Verification, and Reusable Roll RFID Intervals (Shop-Floor PRD §3-§8)
package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/costing"
)

func registerCostingAndShopfloorRoutesV52(mux *http.ServeMux, s *EdgeServer) {
	// Costing Screens S1-S5 & APIs
	mux.HandleFunc("/api/v1/costing/sheets", s.handleCostingSheetsV52)
	mux.HandleFunc("/api/v1/costing/preview", s.handleCostingPreviewV52)
	mux.HandleFunc("/api/v1/costing/replan", s.handleCostingReplanV52)
	mux.HandleFunc("/api/v1/costing/submit", s.handleCostingSubmitV52)
	mux.HandleFunc("/api/v1/costing/decide", s.handleCostingDecideV52)
	mux.HandleFunc("/api/v1/costing/confirm", s.handleCostingConfirmV52)

	// Assigned OB Lifecycle & Absenteeism Shuffle (D13-D15, V13-V14)
	mux.HandleFunc("/api/v1/planning/assigned-ob", s.handleAssignedOBV52)
	mux.HandleFunc("/api/v1/planning/assigned-ob/piece-scanout", s.handleAssignedOBPieceScanoutV52)

	// Shop-Floor Tracking Gaps Reconciled (Shop-Floor PRD §4.8, §4.9, §4.11, §5.5-§5.8)
	mux.HandleFunc("/api/v1/shopfloor/checker-pass", s.handleCheckerStagePassV52)
	mux.HandleFunc("/api/v1/shopfloor/stoppages", s.handleWorkstationStoppageV52)
	mux.HandleFunc("/api/v1/shopfloor/skill-matrix", s.handleOperatorSkillMatrixV52)
	mux.HandleFunc("/api/v1/jobwork/challans", s.handleOutsideProcessChallanV52)
	mux.HandleFunc("/api/v1/stores/reusable-roll-tag", s.handleReusableRollTagIntervalV52)
}

func (s *EdgeServer) appendCostingAudit(ctx context.Context, tx *sql.Tx, tenant, entity, entityID, actor, role, action string, beforeObj, afterObj any, reason string) error {
	bBefore, _ := json.Marshal(beforeObj)
	bAfter, _ := json.Marshal(afterObj)
	_, err := tx.ExecContext(ctx, `
		INSERT INTO costing_audit_events_v52 (
			tenant_id, entity, entity_id, actor_id, role, action, before_state, after_state, reason
		) VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8::jsonb,$9)`,
		tenant, entity, entityID, actor, role, action, string(bBefore), string(bAfter), reason)
	return err
}

// handleCostingSheetsV52 (S1 Register & S2 Draft Save with V1-V5, V9 blocking validation)
func (s *EdgeServer) handleCostingSheetsV52(w http.ResponseWriter, r *http.Request) {
	tenant, err := intakeTenant()
	if err != nil || s.db == nil {
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
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

	if r.Method == http.MethodGet {
		rows, err := tx.QueryContext(ctx, `
			SELECT cs.id::text, cs.buyer_id, cs.style_id, cs.po_ref, cs.currency,
			       cs.requested_ship_date::text, cs.tentative_confirmation_date::text,
			       cs.quote_valid_until::text, cs.state, cs.current_version_no,
			       COALESCE(ps.fob_pc, 0), COALESCE(ps.margin_pct, 0), COALESCE(pr.verdict, 'PENDING')
			FROM costing_sheets_v52 cs
			LEFT JOIN costing_versions_v52 cv
			       ON cv.tenant_id=cs.tenant_id AND cv.sheet_id=cs.id AND cv.version_no=cs.current_version_no
			LEFT JOIN costing_price_summaries_v52 ps
			       ON ps.tenant_id=cv.tenant_id AND ps.version_id=cv.id
			LEFT JOIN LATERAL (
				SELECT verdict FROM costing_preview_runs_v52
				WHERE tenant_id=cv.tenant_id AND version_id=cv.id
				ORDER BY run_at DESC LIMIT 1
			) pr ON true
			WHERE cs.tenant_id=$1
			ORDER BY cs.updated_at DESC LIMIT 200`, tenant)
		if err != nil {
			http.Error(w, "Failed to query costing register", http.StatusServiceUnavailable)
			return
		}
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var id, buyer, style, po, curr, ship, tent, valid, state, verdict string
			var ver int
			var fob, margin float64
			if rows.Scan(&id, &buyer, &style, &po, &curr, &ship, &tent, &valid, &state, &ver, &fob, &margin, &verdict) == nil {
				items = append(items, map[string]any{
					"id": id, "buyer_id": buyer, "style_id": style, "po_ref": po, "currency": curr,
					"requested_ship_date": ship, "tentative_confirmation_date": tent,
					"quote_valid_until": valid, "state": state, "version_no": ver,
					"fob_pc": fob, "margin_pct": margin, "verdict": verdict,
				})
			}
		}
		_ = tx.Commit()
		docJSON(w, http.StatusOK, map[string]any{"sheets": items})
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UnitID                    string                    `json:"unit_id"`
		BuyerID                   string                    `json:"buyer_id"`
		StyleID                   string                    `json:"style_id"`
		PORef                     string                    `json:"po_ref"`
		Currency                  string                    `json:"currency"`
		OrderDate                 string                    `json:"order_date"`
		RequestedShipDate         string                    `json:"requested_ship_date"`
		TentativeConfirmationDate string                    `json:"tentative_confirmation_date"`
		QuoteValidUntil           string                    `json:"quote_valid_until"`
		StatedGrandTotal          *int                      `json:"stated_grand_total,omitempty"`
		StatedColourTotals        map[string]int            `json:"stated_colour_totals,omitempty"`
		Lines                     []costing.OrderMatrixLine `json:"lines"`
		BOMLines                  []costing.BOMLineInput    `json:"bom_lines"`
		OBID                      string                    `json:"ob_id"`
		OBVersion                 int                       `json:"ob_version"`
		OBState                   string                    `json:"ob_state"`
		OBOperations              []costing.OBOperation     `json:"ob_operations"`
		CostLines                 []costing.CostLine        `json:"cost_lines"`
		MarginPct                 float64                   `json:"margin_pct"`
		FXRateRef                 string                    `json:"fx_rate_ref"`
		FXRate                    float64                   `json:"fx_rate"`
		ActorID                   string                    `json:"actor_id"`
	}
	if !candidateJSONBody(w, r, 262144, &req) {
		return
	}
	ordDate, err1 := time.Parse("2006-01-02", req.OrderDate)
	shipDate, err2 := time.Parse("2006-01-02", req.RequestedShipDate)
	tentDate, err3 := time.Parse("2006-01-02", req.TentativeConfirmationDate)
	validDate, err4 := time.Parse("2006-01-02", req.QuoteValidUntil)
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		http.Error(w, "ERR_V5_INVALID_ISO_DATES", http.StatusBadRequest)
		return
	}

	// V3: Load existing buyer open UPCs from DB
	openUPCs := map[string]string{}
	upcRows, err := tx.QueryContext(ctx, `
		SELECT m.upc, cs.po_ref
		FROM costing_order_matrix_lines_v52 m
		JOIN costing_sheets_v52 cs ON cs.tenant_id=m.tenant_id AND cs.id=m.sheet_id
		WHERE m.tenant_id=$1 AND m.buyer_id=$2 AND m.sheet_active=true`, tenant, req.BuyerID)
	if err == nil {
		for upcRows.Next() {
			var u, p string
			if upcRows.Scan(&u, &p) == nil {
				openUPCs[u] = p
			}
		}
		upcRows.Close()
	}

	matrixSummary, err := costing.ValidateOrderMatrix(costing.OrderMatrixInput{
		BuyerID: req.BuyerID, StyleID: req.StyleID, PORef: req.PORef, Currency: req.Currency,
		OrderDate: ordDate, RequestedShipDate: shipDate,
		TentativeConfirmationDate: tentDate, QuoteValidUntil: validDate,
		StatedGrandTotal: req.StatedGrandTotal, StatedColourTotals: req.StatedColourTotals,
		Lines: req.Lines, ExistingBuyerOpenUPCs: openUPCs,
	}, time.Now().UTC())
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	// Read active CPM (D3/D3a locked: total_factory_expense / total_paid_work_minutes)
	var cpm float64
	var cpmPeriod string
	err = tx.QueryRowContext(ctx, `
		SELECT cpm::double precision, period_label
		FROM cost_rates_v52
		WHERE tenant_id=$1 AND unit_id=$2 AND active=true
		ORDER BY period_label DESC LIMIT 1`, tenant, req.UnitID).Scan(&cpm, &cpmPeriod)
	if err != nil {
		http.Error(w, "ERR_D3_ACTIVE_PAID_MINUTES_CPM_NOT_FOUND", http.StatusUnprocessableEntity)
		return
	}

	computedBOM := make([]costing.BOMLineComputed, 0, len(req.BOMLines))
	for i, bl := range req.BOMLines {
		bl.LineNo = i + 1
		cb, err := costing.ComputeBOMLine(bl, matrixSummary.TotalQty)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		computedBOM = append(computedBOM, cb)
	}

	if strings.ToUpper(strings.TrimSpace(req.OBState)) != "RELEASED" {
		http.Error(w, "ERR_V7_PINNED_OB_MUST_BE_RELEASED", http.StatusUnprocessableEntity)
		return
	}
	priceSummary, totalSAM, err := costing.BuildPriceSummary(
		matrixSummary.TotalQty, computedBOM, req.OBOperations, cpm, req.CostLines, req.MarginPct, req.FXRate,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	actor := nonEmptyOr(strings.TrimSpace(req.ActorID), "merchandiser")
	var sheetID string
	var verNo int
	err = tx.QueryRowContext(ctx, `
		INSERT INTO costing_sheets_v52 (
			tenant_id, unit_id, buyer_id, style_id, po_ref, currency,
			order_date, requested_ship_date, tentative_confirmation_date, quote_valid_until,
			state, current_version_no, created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'DRAFT',1,$11)
		ON CONFLICT (tenant_id, buyer_id, po_ref, style_id) DO UPDATE SET
			updated_at = clock_timestamp()
		RETURNING id::text, current_version_no`,
		tenant, req.UnitID, req.BuyerID, req.StyleID, req.PORef, nonEmptyOr(req.Currency, "USD"),
		ordDate, shipDate, tentDate, validDate, actor).Scan(&sheetID, &verNo)
	if err != nil {
		http.Error(w, "Failed to upsert costing sheet: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	// V9: If current version is already frozen/submitted, mint a new version
	var verID string
	var isFrozen bool
	err = tx.QueryRowContext(ctx, `
		SELECT id::text, frozen FROM costing_versions_v52
		WHERE tenant_id=$1 AND sheet_id=$2::uuid AND version_no=$3`,
		tenant, sheetID, verNo).Scan(&verID, &isFrozen)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `
			INSERT INTO costing_versions_v52 (tenant_id, sheet_id, version_no, created_by)
			VALUES ($1,$2::uuid,$3,$4) RETURNING id::text`,
			tenant, sheetID, verNo, actor).Scan(&verID)
	} else if err == nil && isFrozen {
		nextVer := verNo + 1
		err = tx.QueryRowContext(ctx, `
			INSERT INTO costing_versions_v52 (tenant_id, sheet_id, version_no, parent_version_no, change_reason, created_by)
			VALUES ($1,$2::uuid,$3,$4,'Revision after submission',$5) RETURNING id::text`,
			tenant, sheetID, nextVer, verNo, actor).Scan(&verID)
		if err == nil {
			_, err = tx.ExecContext(ctx, `
				UPDATE costing_sheets_v52 SET current_version_no=$3, state='DRAFT', updated_at=clock_timestamp()
				WHERE tenant_id=$1 AND id=$2::uuid`, tenant, sheetID, nextVer)
			verNo = nextVer
		}
	}
	if err != nil {
		http.Error(w, "Failed to prepare costing version: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	// Replace draft lines & mark any prior preview stale (S2 rule)
	_, _ = tx.ExecContext(ctx, `DELETE FROM costing_order_matrix_lines_v52 WHERE tenant_id=$1 AND version_id=$2::uuid`, tenant, verID)
	for _, l := range req.Lines {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO costing_order_matrix_lines_v52 (
				tenant_id, buyer_id, sheet_id, version_id, colour, size, upc, qty, created_by
			) VALUES ($1,$2,$3::uuid,$4::uuid,$5,$6,$7,$8,$9)`,
			tenant, req.BuyerID, sheetID, verID, strings.ToUpper(strings.TrimSpace(l.Colour)),
			strings.ToUpper(strings.TrimSpace(l.Size)), strings.TrimSpace(l.UPC), l.Qty, actor)
		if err != nil {
			http.Error(w, "ERR_V3_MATRIX_LINE_CONFLICT: "+err.Error(), http.StatusUnprocessableEntity)
			return
		}
	}

	_, _ = tx.ExecContext(ctx, `DELETE FROM costing_bom_lines_v52 WHERE tenant_id=$1 AND version_id=$2::uuid`, tenant, verID)
	for _, b := range computedBOM {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO costing_bom_lines_v52 (
				tenant_id, version_id, line_no, item_id, category, colour, supplier_id,
				consumption, uom, purchase_unit_multiple, wastage_pct, unit_price, price_currency,
				gross_qty, stock_snapshot_qty, stock_snapshot_at, net_qty, line_cost_per_pc, created_by
			) VALUES ($1,$2::uuid,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`,
			tenant, verID, b.LineNo, b.ItemID, strings.ToUpper(b.Category), b.Colour, b.SupplierID,
			b.Consumption, b.UOM, nonZeroFloat(b.PurchaseUnitMultiple, 1), b.WastagePct, b.UnitPrice,
			nonEmptyOr(b.PriceCurrency, "USD"), b.GrossQty, b.AvailUnreservedStock, b.StockSnapshotAt,
			b.NetQty, b.LineCostPerPc, actor)
		if err != nil {
			http.Error(w, "Failed to save BOM line: "+err.Error(), http.StatusUnprocessableEntity)
			return
		}
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO costing_ob_pins_v52 (
			tenant_id, version_id, ob_id, ob_version, ob_state, total_sam, cpm_used, cpm_period, created_by
		) VALUES ($1,$2::uuid,$3,$4,'RELEASED',$5,$6,$7,$8)
		ON CONFLICT (tenant_id, version_id) DO UPDATE SET
			ob_id=EXCLUDED.ob_id, ob_version=EXCLUDED.ob_version, total_sam=EXCLUDED.total_sam,
			cpm_used=EXCLUDED.cpm_used, cpm_period=EXCLUDED.cpm_period`,
		tenant, verID, req.OBID, req.OBVersion, totalSAM, cpm, cpmPeriod, actor)
	if err != nil {
		http.Error(w, "Failed to pin OB version: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO costing_price_summaries_v52 (
			version_id, tenant_id, total_qty, material_pc, cm_pc, other_pc, overhead_pc,
			margin_pct, fob_pc, order_value, base_fob_pc, base_order_value, fx_rate_ref, fx_rate
		) VALUES ($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (version_id) DO UPDATE SET
			total_qty=EXCLUDED.total_qty, material_pc=EXCLUDED.material_pc, cm_pc=EXCLUDED.cm_pc,
			other_pc=EXCLUDED.other_pc, overhead_pc=EXCLUDED.overhead_pc, margin_pct=EXCLUDED.margin_pct,
			fob_pc=EXCLUDED.fob_pc, order_value=EXCLUDED.order_value, base_fob_pc=EXCLUDED.base_fob_pc,
			base_order_value=EXCLUDED.base_order_value, computed_at=clock_timestamp()`,
		verID, tenant, priceSummary.TotalQty, priceSummary.MaterialPerPc, priceSummary.CMPerPc,
		priceSummary.OtherPerPc, priceSummary.OverheadPerPc, priceSummary.MarginPct,
		priceSummary.FOBPerPc, priceSummary.OrderValue, priceSummary.BaseFOBPerPc,
		priceSummary.BaseOrderValue, nonEmptyOr(req.FXRateRef, "SPOT"), priceSummary.FXRate)
	if err != nil {
		http.Error(w, "Failed to save price summary: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	// Mark prior preview runs stale on any input edit
	_, _ = tx.ExecContext(ctx, `
		UPDATE costing_preview_runs_v52 SET is_stale=true, stale_reason='Inputs modified after preview'
		WHERE tenant_id=$1 AND version_id=$2::uuid AND is_stale=false`, tenant, verID)

	_ = s.appendCostingAudit(ctx, tx, tenant, "COSTING_VERSION", verID, actor, "MERCHANDISER", "SAVE_DRAFT", nil, priceSummary, "Saved costing draft")
	if err = tx.Commit(); err != nil {
		http.Error(w, "Commit failed", http.StatusServiceUnavailable)
		return
	}
	docJSON(w, http.StatusCreated, map[string]any{
		"sheet_id":      sheetID,
		"version_id":    verID,
		"version_no":    verNo,
		"matrix":        matrixSummary,
		"price_summary": priceSummary,
		"cpm_used":      cpm,
		"cpm_period":    cpmPeriod,
		"preview_stale": true,
	})
}

// handleCostingPreviewV52 computes FR-5..FR-8 feasibility and persists an immutable PreviewRun snapshot.
func (s *EdgeServer) handleCostingPreviewV52(w http.ResponseWriter, r *http.Request) {
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
		VersionID                string                `json:"version_id"`
		PeakTargetPcsPerDay      int                   `json:"peak_target_pcs_per_day"`
		MinutesPerOperatorPerDay int                   `json:"minutes_per_operator_per_day"`
		DailyRampPcs             []int                 `json:"daily_ramp_pcs"`
		OBOperations             []costing.OBOperation `json:"ob_operations"`
		OwnedMachinesByType      map[string]int        `json:"owned_machines_by_type"`
		PoolOperatorsByGrade     map[string]int        `json:"pool_operators_by_grade"`
		DailyAvailableMinutes    int                   `json:"daily_available_minutes"`
		DailyCommittedMinutes    int                   `json:"daily_committed_minutes"`
	}
	if !candidateJSONBody(w, r, 131072, &req) {
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

	var sheetID, unitID string
	var ordDate, shipDate time.Time
	var totalQty int
	err = tx.QueryRowContext(ctx, `
		SELECT cs.id::text, cs.unit_id, cs.order_date, cs.requested_ship_date, ps.total_qty
		FROM costing_versions_v52 cv
		JOIN costing_sheets_v52 cs ON cs.tenant_id=cv.tenant_id AND cs.id=cv.sheet_id
		JOIN costing_price_summaries_v52 ps ON ps.tenant_id=cv.tenant_id AND ps.version_id=cv.id
		WHERE cv.tenant_id=$1 AND cv.id=$2::uuid`, tenant, req.VersionID).Scan(&sheetID, &unitID, &ordDate, &shipDate, &totalQty)
	if err != nil {
		http.Error(w, "Costing version or price summary not found", http.StatusNotFound)
		return
	}

	// Subtract active CapacityHold minutes from other quotes (D2 / FR-8)
	var heldMinutes int
	_ = tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(held_minutes), 0)
		FROM costing_capacity_holds_v52
		WHERE tenant_id=$1 AND unit_id=$2 AND sheet_id <> $3::uuid
		  AND status='HELD' AND hold_until >= CURRENT_DATE`, tenant, unitID, sheetID).Scan(&heldMinutes)

	// Load active Assigned OB blocks across other POs (§4.3)
	blockedMach := map[string]int{}
	blockedOps := map[string]int{}
	_ = tx.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT machine_no)
		FROM ob_assignments_v52
		WHERE tenant_id=$1 AND unit_id=$2 AND status='ASSIGNED'`, tenant, unitID).Scan(new(int))

	bomRows, err := tx.QueryContext(ctx, `
		SELECT line_no, item_id, category, supplier_id, consumption::double precision, uom,
		       wastage_pct::double precision, unit_price::double precision, gross_qty::double precision,
		       stock_snapshot_qty::double precision, stock_snapshot_at, net_qty::double precision,
		       line_cost_per_pc::double precision
		FROM costing_bom_lines_v52 WHERE tenant_id=$1 AND version_id=$2::uuid ORDER BY line_no`, tenant, req.VersionID)
	if err != nil {
		http.Error(w, "Failed to load BOM lines", http.StatusServiceUnavailable)
		return
	}
	bomList := []costing.BOMLineComputed{}
	for bomRows.Next() {
		var b costing.BOMLineComputed
		if bomRows.Scan(&b.LineNo, &b.ItemID, &b.Category, &b.SupplierID, &b.Consumption, &b.UOM,
			&b.WastagePct, &b.UnitPrice, &b.GrossQty, &b.AvailUnreservedStock, &b.StockSnapshotAt,
			&b.NetQty, &b.LineCostPerPc) == nil {
			b.LeadTimeDays = 21
			bomList = append(bomList, b)
		}
	}
	bomRows.Close()

	prevIn := costing.PreviewInput{
		OrderDate:                ordDate,
		RequestedShipDate:        shipDate,
		TransitHandoverDays:      2,
		AuditLoadBufferDays:      2,
		FinishingLagDays:         1,
		MaterialBufferDays:       3,
		AmberSlackThresholdDays:  7,
		MinutesPerOperatorPerDay: nonZeroInt(req.MinutesPerOperatorPerDay, 480),
		PeakTargetPcsPerDay:      nonZeroInt(req.PeakTargetPcsPerDay, 1000),
		MachineSparePct:          0.05,
		OrderQty:                 totalQty,
		BOMLines:                 bomList,
		OBOperations:             req.OBOperations,
		OwnedMachinesByType:      req.OwnedMachinesByType,
		BlockedMachinesByType:    blockedMach,
		PoolOperatorsByGrade:     req.PoolOperatorsByGrade,
		BlockedOperatorsByGrade:  blockedOps,
		DailyRampPcs:             req.DailyRampPcs,
		DailyAvailableMinutes:    req.DailyAvailableMinutes,
		DailyCommittedMinutes:    req.DailyCommittedMinutes,
		DailyHeldMinutes:         heldMinutes,
	}
	prevRes, err := costing.RunPlanningPreview(prevIn)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	snapJSON, _ := json.Marshal(prevIn)
	reasonsJSON, _ := json.Marshal(prevRes.Reasons)
	var runID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO costing_preview_runs_v52 (
			tenant_id, version_id, input_snapshot_hash, input_snapshot, verdict, reasons,
			latest_plan_cut_date, sewing_start_date, sewing_end_date, finishing_end_date,
			earliest_feasible_ex_factory, slack_days, is_stale
		) VALUES ($1,$2::uuid,$3,$4::jsonb,$5,$6::jsonb,$7,$8,$9,$10,$11,$12,false)
		RETURNING id::text`,
		tenant, req.VersionID, prevRes.InputSnapshotHash, string(snapJSON), string(prevRes.Verdict),
		string(reasonsJSON), prevRes.LatestPlanCutDate, prevRes.SewingStartDate, prevRes.SewingEndDate,
		prevRes.FinishingEndDate, prevRes.EarliestFeasibleExFactory, prevRes.SlackDays).Scan(&runID)
	if err != nil {
		http.Error(w, "Failed to persist preview run: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	_, _ = tx.ExecContext(ctx, `
		UPDATE costing_sheets_v52 SET state='PREVIEWED', updated_at=clock_timestamp()
		WHERE tenant_id=$1 AND id=$2::uuid AND state='DRAFT'`, tenant, sheetID)
	if err = tx.Commit(); err != nil {
		http.Error(w, "Commit failed", http.StatusServiceUnavailable)
		return
	}
	docJSON(w, http.StatusOK, map[string]any{"run_id": runID, "preview": prevRes})
}

// handleCostingReplanV52 records mandatory joint re-plans for Amber/Red verdicts (D5, V8).
func (s *EdgeServer) handleCostingReplanV52(w http.ResponseWriter, r *http.Request) {
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
		SheetID      string   `json:"sheet_id"`
		TriggerKind  string   `json:"trigger_kind"`
		Participants []string `json:"participants"`
		Changes      any      `json:"changes"`
		Reason       string   `json:"reason"`
	}
	if !candidateJSONBody(w, r, 65536, &req) {
		return
	}
	if len(strings.TrimSpace(req.Reason)) < 20 || len(req.Participants) < 2 {
		http.Error(w, "ERR_D5_JOINT_REPLAN_REQUIRES_PARTICIPANTS_AND_20_CHAR_REASON", http.StatusUnprocessableEntity)
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

	bPart, _ := json.Marshal(req.Participants)
	bChg, _ := json.Marshal(req.Changes)
	var id string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO costing_replan_records_v52 (tenant_id, sheet_id, trigger_kind, participants, changes, reason)
		VALUES ($1,$2::uuid,$3,$4::jsonb,$5::jsonb,$6) RETURNING id::text`,
		tenant, req.SheetID, nonEmptyOr(req.TriggerKind, "AMBER"), string(bPart), string(bChg), strings.TrimSpace(req.Reason)).Scan(&id)
	if err != nil {
		http.Error(w, "Failed to record joint replan: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}
	_ = s.appendCostingAudit(ctx, tx, tenant, "COSTING_SHEET", req.SheetID, strings.Join(req.Participants, ","), "JOINT_TEAM", "REPLAN_RECORDED", nil, req.Changes, req.Reason)
	_ = tx.Commit()
	docJSON(w, http.StatusCreated, map[string]any{"replan_id": id})
}

// handleCostingSubmitV52 enforces V7, V9, and D5 (no Amber/Red override) before freezing version for approval.
func (s *EdgeServer) handleCostingSubmitV52(w http.ResponseWriter, r *http.Request) {
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
		VersionID string `json:"version_id"`
		ActorID   string `json:"actor_id"`
	}
	if !candidateJSONBody(w, r, 16384, &req) {
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

	var sheetID, obState, verdict string
	var isStale bool
	err = tx.QueryRowContext(ctx, `
		SELECT cv.sheet_id::text, ob.ob_state, pr.verdict, pr.is_stale
		FROM costing_versions_v52 cv
		JOIN costing_ob_pins_v52 ob ON ob.tenant_id=cv.tenant_id AND ob.version_id=cv.id
		JOIN LATERAL (
			SELECT verdict, is_stale FROM costing_preview_runs_v52
			WHERE tenant_id=cv.tenant_id AND version_id=cv.id
			ORDER BY run_at DESC LIMIT 1
		) pr ON true
		WHERE cv.tenant_id=$1 AND cv.id=$2::uuid`, tenant, req.VersionID).Scan(&sheetID, &obState, &verdict, &isStale)
	if err != nil {
		http.Error(w, "ERR_V7_PREVIEW_OR_OB_PIN_MISSING", http.StatusUnprocessableEntity)
		return
	}

	var replanReason string
	_ = tx.QueryRowContext(ctx, `
		SELECT reason FROM costing_replan_records_v52
		WHERE tenant_id=$1 AND sheet_id=$2::uuid
		ORDER BY recorded_at DESC LIMIT 1`, tenant, sheetID).Scan(&replanReason)

	if err := costing.ValidateSubmitAndConfirmGate(isStale, obState, costing.Verdict(verdict), replanReason != "", replanReason); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	// Freeze version (V9) & move sheet to SUBMITTED
	_, err = tx.ExecContext(ctx, `UPDATE costing_versions_v52 SET frozen=true WHERE tenant_id=$1 AND id=$2::uuid`, tenant, req.VersionID)
	if err != nil {
		http.Error(w, "Failed to freeze version", http.StatusServiceUnavailable)
		return
	}
	_, _ = tx.ExecContext(ctx, `UPDATE costing_sheets_v52 SET state='SUBMITTED', updated_at=clock_timestamp() WHERE tenant_id=$1 AND id=$2::uuid`, tenant, sheetID)
	_ = s.appendCostingAudit(ctx, tx, tenant, "COSTING_VERSION", req.VersionID, nonEmptyOr(req.ActorID, "merchandiser"), "MERCHANDISER", "SUBMIT_FOR_APPROVAL", nil, map[string]any{"verdict": verdict}, "Submitted with fresh preview")
	_ = tx.Commit()
	docJSON(w, http.StatusOK, map[string]any{"version_id": req.VersionID, "state": "SUBMITTED", "frozen": true})
}

// handleCostingDecideV52 (S4 Mobile/Desktop Approval) creates CapacityHold on approval (D2) or requires 20-char reason on reject (V8).
func (s *EdgeServer) handleCostingDecideV52(w http.ResponseWriter, r *http.Request) {
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
		VersionID  string `json:"version_id"`
		ApproverID string `json:"approver_id"`
		Decision   string `json:"decision"` // APPROVED or REJECTED
		Reason     string `json:"reason"`
		Device     string `json:"device"`
	}
	if !candidateJSONBody(w, r, 32768, &req) {
		return
	}
	decision := strings.ToUpper(strings.TrimSpace(req.Decision))
	if decision != "APPROVED" && decision != "REJECTED" {
		http.Error(w, "decision must be APPROVED or REJECTED", http.StatusBadRequest)
		return
	}
	if decision == "REJECTED" && len(strings.TrimSpace(req.Reason)) < 20 {
		http.Error(w, "ERR_V8_REJECTION_REASON_MIN_20_CHARS", http.StatusUnprocessableEntity)
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

	var sheetID, unitID string
	var verNo, totalQty int
	var tentDate, sewStart, sewEnd time.Time
	var totalSAM float64
	err = tx.QueryRowContext(ctx, `
		SELECT cs.id::text, cs.unit_id, cv.version_no, cs.tentative_confirmation_date,
		       ps.total_qty, ob.total_sam, pr.sewing_start_date, pr.sewing_end_date
		FROM costing_versions_v52 cv
		JOIN costing_sheets_v52 cs ON cs.tenant_id=cv.tenant_id AND cs.id=cv.sheet_id
		JOIN costing_price_summaries_v52 ps ON ps.tenant_id=cv.tenant_id AND ps.version_id=cv.id
		JOIN costing_ob_pins_v52 ob ON ob.tenant_id=cv.tenant_id AND ob.version_id=cv.id
		JOIN LATERAL (
			SELECT sewing_start_date, sewing_end_date FROM costing_preview_runs_v52
			WHERE tenant_id=cv.tenant_id AND version_id=cv.id ORDER BY run_at DESC LIMIT 1
		) pr ON true
		WHERE cv.tenant_id=$1 AND cv.id=$2::uuid`, tenant, req.VersionID).Scan(&sheetID, &unitID, &verNo, &tentDate, &totalQty, &totalSAM, &sewStart, &sewEnd)
	if err != nil {
		http.Error(w, "Submitted costing version not found", http.StatusNotFound)
		return
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO costing_approvals_v52 (tenant_id, version_id, approval_level, approver_id, decision, reason, device)
		VALUES ($1,$2::uuid,1,$3,$4,$5,$6)`,
		tenant, req.VersionID, nonEmptyOr(req.ApproverID, "factory_head"), decision, strings.TrimSpace(req.Reason), nonEmptyOr(req.Device, "DESKTOP"))
	if err != nil {
		http.Error(w, "Failed to record approval decision", http.StatusServiceUnavailable)
		return
	}

	newState := "REJECTED"
	if decision == "APPROVED" {
		newState = "QUOTED"
		// D2: Place CapacityHold until tentative_confirmation_date
		days := int(sewEnd.Sub(sewStart).Hours()/24) + 1
		if days < 1 {
			days = 1
		}
		dailyMin := int((float64(totalQty) * totalSAM) / float64(days))
		for d := 0; d < days; d++ {
			_, _ = tx.ExecContext(ctx, `
				INSERT INTO costing_capacity_holds_v52 (
					tenant_id, sheet_id, version_no, unit_id, hold_date, held_minutes, hold_until, status
				) VALUES ($1,$2::uuid,$3,$4,$5,$6,$7,'HELD')`,
				tenant, sheetID, verNo, unitID, sewStart.AddDate(0, 0, d), dailyMin, tentDate)
		}
	}
	_, _ = tx.ExecContext(ctx, `UPDATE costing_sheets_v52 SET state=$3, updated_at=clock_timestamp() WHERE tenant_id=$1 AND id=$2::uuid`, tenant, sheetID, newState)
	_ = s.appendCostingAudit(ctx, tx, tenant, "COSTING_VERSION", req.VersionID, nonEmptyOr(req.ApproverID, "factory_head"), "APPROVER", decision, nil, map[string]any{"state": newState}, req.Reason)
	_ = tx.Commit()
	docJSON(w, http.StatusOK, map[string]any{"version_id": req.VersionID, "state": newState})
}

// handleCostingConfirmV52 (S5 / FR-13) freezes PlanningBaseline & converts CapacityHolds in one transaction.
func (s *EdgeServer) handleCostingConfirmV52(w http.ResponseWriter, r *http.Request) {
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
		VersionID string `json:"version_id"`
		ActorID   string `json:"actor_id"`
	}
	if !candidateJSONBody(w, r, 16384, &req) {
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

	var sheetID, poRef, styleID, buyerID, runID, verdict string
	var isStale bool
	var fobPc, orderVal, matPc, cmPc float64
	var totalQty int
	err = tx.QueryRowContext(ctx, `
		SELECT cs.id::text, cs.po_ref, cs.style_id, cs.buyer_id, pr.id::text, pr.verdict, pr.is_stale,
		       ps.fob_pc::double precision, ps.order_value::double precision,
		       ps.material_pc::double precision, ps.cm_pc::double precision, ps.total_qty
		FROM costing_versions_v52 cv
		JOIN costing_sheets_v52 cs ON cs.tenant_id=cv.tenant_id AND cs.id=cv.sheet_id
		JOIN costing_price_summaries_v52 ps ON ps.tenant_id=cv.tenant_id AND ps.version_id=cv.id
		JOIN LATERAL (
			SELECT id, verdict, is_stale FROM costing_preview_runs_v52
			WHERE tenant_id=cv.tenant_id AND version_id=cv.id ORDER BY run_at DESC LIMIT 1
		) pr ON true
		WHERE cv.tenant_id=$1 AND cv.id=$2::uuid AND cs.state='QUOTED'`, tenant, req.VersionID).Scan(
		&sheetID, &poRef, &styleID, &buyerID, &runID, &verdict, &isStale, &fobPc, &orderVal, &matPc, &cmPc, &totalQty)
	if err != nil {
		http.Error(w, "Quoted costing version not found", http.StatusNotFound)
		return
	}
	if isStale {
		http.Error(w, "ERR_FR13_PREVIEW_STALE_RERUN_BEFORE_CONFIRM", http.StatusConflict)
		return
	}

	payload, _ := json.Marshal(map[string]any{
		"po_ref": poRef, "style_id": styleID, "buyer_id": buyerID, "total_qty": totalQty,
		"costed_fob_pc": fobPc, "costed_order_value": orderVal, "costed_material_pc": matPc,
		"costed_cm_pc": cmPc, "verdict": verdict,
	})
	var baselineID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO planning_baselines_v52 (tenant_id, po_id, sheet_id, version_id, run_id, payload)
		VALUES ($1,$2,$3::uuid,$4::uuid,$5::uuid,$6::jsonb)
		RETURNING id::text`, tenant, poRef, sheetID, req.VersionID, runID, string(payload)).Scan(&baselineID)
	if err != nil {
		http.Error(w, "Failed to freeze PlanningBaseline: "+err.Error(), http.StatusConflict)
		return
	}

	// Convert CapacityHolds (D2) and mark matrix UPCs inactive on open-quote index
	_, _ = tx.ExecContext(ctx, `UPDATE costing_capacity_holds_v52 SET status='CONVERTED', updated_at=clock_timestamp() WHERE tenant_id=$1 AND sheet_id=$2::uuid AND status='HELD'`, tenant, sheetID)
	_, _ = tx.ExecContext(ctx, `UPDATE costing_order_matrix_lines_v52 SET sheet_active=false WHERE tenant_id=$1 AND sheet_id=$2::uuid`, tenant, sheetID)
	_, _ = tx.ExecContext(ctx, `UPDATE costing_sheets_v52 SET state='CONFIRMED', updated_at=clock_timestamp() WHERE tenant_id=$1 AND id=$2::uuid`, tenant, sheetID)

	_ = s.appendCostingAudit(ctx, tx, tenant, "PLANNING_BASELINE", baselineID, nonEmptyOr(req.ActorID, "merchandiser"), "MERCHANDISER", "CONFIRM_ORDER_HANDOFF", nil, json.RawMessage(payload), "Confirmed order and froze baseline")
	_ = tx.Commit()
	docJSON(w, http.StatusOK, map[string]any{"baseline_id": baselineID, "po_id": poRef, "state": "CONFIRMED"})
}

// handleAssignedOBV52 implements §4.3, D14, D15, V13, V14 (versioned operator/machine OB blocks & absenteeism shuffles).
func (s *EdgeServer) handleAssignedOBV52(w http.ResponseWriter, r *http.Request) {
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

	if r.Method == http.MethodGet {
		poID := strings.TrimSpace(r.URL.Query().Get("po_id"))
		rows, err := tx.QueryContext(ctx, `
			SELECT id::text, unit_id, po_id, ob_id, ob_version, plan_version, operation_id, operation_seq,
			       sam::double precision, target_qty, completed_qty, operator_id, operator_name,
			       machine_id, machine_no, machine_name, block_from::text, block_to::text,
			       status, changed_by, changer_role, change_reason, changed_at
			FROM ob_assignments_v52
			WHERE tenant_id=$1 AND ($2='' OR po_id=$2) AND status='ASSIGNED'
			ORDER BY po_id ASC, operation_seq ASC`, tenant, poID)
		if err != nil {
			http.Error(w, "Failed to query assigned OB", http.StatusServiceUnavailable)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id, unit, po, obID, opID, empID, empName, mID, mNo, mName, bFrom, bTo, status, chgBy, role, reason string
			var obVer, planVer, opSeq, tgt, comp int
			var sam float64
			var chgAt time.Time
			if rows.Scan(&id, &unit, &po, &obID, &obVer, &planVer, &opID, &opSeq, &sam, &tgt, &comp,
				&empID, &empName, &mID, &mNo, &mName, &bFrom, &bTo, &status, &chgBy, &role, &reason, &chgAt) == nil {
				out = append(out, map[string]any{
					"id": id, "unit_id": unit, "po_id": po, "ob_id": obID, "ob_version": obVer,
					"plan_version": planVer, "operation_id": opID, "operation_seq": opSeq,
					"sam": sam, "target_qty": tgt, "completed_qty": comp,
					"operator_id": empID, "operator_name": empName,
					"machine_id": mID, "machine_no": mNo, "machine_name": mName,
					"block_from": bFrom, "block_to": bTo, "status": status,
					"changed_by": chgBy, "changer_role": role, "change_reason": reason, "changed_at": chgAt,
				})
			}
		}
		_ = tx.Commit()
		docJSON(w, http.StatusOK, map[string]any{"assigned_ob": out})
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UnitID       string `json:"unit_id"`
		POID         string `json:"po_id"`
		OBID         string `json:"ob_id"`
		OBVersion    int    `json:"ob_version"`
		ChangedBy    string `json:"changed_by"`
		ChangerRole  string `json:"changer_role"` // V14: IE, FACTORY_MANAGER, SUPERVISOR, PLANNING_MANAGER
		ChangeReason string `json:"change_reason"`
		Operations   []struct {
			OperationID  string  `json:"operation_id"`
			OperationSeq int     `json:"operation_seq"`
			SAM          float64 `json:"sam"`
			TargetQty    int     `json:"target_qty"`
			OperatorID   string  `json:"operator_id"`
			OperatorName string  `json:"operator_name"`
			MachineID    string  `json:"machine_id"`
			MachineNo    string  `json:"machine_no"`
			MachineName  string  `json:"machine_name"`
			BlockFrom    string  `json:"block_from"`
			BlockTo      string  `json:"block_to"`
		} `json:"operations"`
	}
	if !candidateJSONBody(w, r, 131072, &req) {
		return
	}
	role := strings.ToUpper(strings.TrimSpace(req.ChangerRole))
	if role != "PLANNING_MANAGER" && role != "IE" && role != "FACTORY_MANAGER" && role != "SUPERVISOR" {
		http.Error(w, "ERR_V14_UNAUTHORIZED_ROLE_FOR_OB_ASSIGNMENT_OR_SHUFFLE", http.StatusForbidden)
		return
	}
	if strings.TrimSpace(req.ChangedBy) == "" || len(req.Operations) == 0 {
		http.Error(w, "ERR_V13_CHANGER_ID_AND_OPERATIONS_REQUIRED", http.StatusBadRequest)
		return
	}

	var curPlanVer int
	_ = tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(plan_version), 0) FROM ob_assignments_v52
		WHERE tenant_id=$1 AND po_id=$2`, tenant, req.POID).Scan(&curPlanVer)
	nextPlanVer := curPlanVer + 1

	// Supersede prior active version (read-only history preserved)
	_, _ = tx.ExecContext(ctx, `
		UPDATE ob_assignments_v52 SET status='SUPERSEDED'
		WHERE tenant_id=$1 AND po_id=$2 AND status='ASSIGNED'`, tenant, req.POID)

	// D15: No fixed skill-grade blocking rule on reassignment
	for _, op := range req.Operations {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO ob_assignments_v52 (
				tenant_id, unit_id, po_id, ob_id, ob_version, plan_version,
				operation_id, operation_seq, sam, target_qty,
				operator_id, operator_name, machine_id, machine_no, machine_name,
				block_from, block_to, status, changed_by, changer_role, change_reason
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16::date,$17::date,'ASSIGNED',$18,$19,$20)`,
			tenant, req.UnitID, req.POID, req.OBID, req.OBVersion, nextPlanVer,
			op.OperationID, op.OperationSeq, op.SAM, op.TargetQty,
			op.OperatorID, op.OperatorName, op.MachineID, op.MachineNo, op.MachineName,
			op.BlockFrom, op.BlockTo, req.ChangedBy, role, req.ChangeReason)
		if err != nil {
			http.Error(w, "Failed to save assigned OB operation: "+err.Error(), http.StatusUnprocessableEntity)
			return
		}
	}
	_ = s.appendCostingAudit(ctx, tx, tenant, "ASSIGNED_OB", req.POID, req.ChangedBy, role, "PUBLISH_PLAN_VERSION", nil, map[string]any{"plan_version": nextPlanVer}, req.ChangeReason)
	_ = tx.Commit()
	docJSON(w, http.StatusCreated, map[string]any{"po_id": req.POID, "plan_version": nextPlanVer, "status": "ASSIGNED"})
}

// handleAssignedOBPieceScanoutV52 increments completed_qty; DB trigger auto-releases block on last piece (D13).
func (s *EdgeServer) handleAssignedOBPieceScanoutV52(w http.ResponseWriter, r *http.Request) {
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
		POID        string `json:"po_id"`
		OperationID string `json:"operation_id"`
		PiecesDelta int    `json:"pieces_delta"`
	}
	if !candidateJSONBody(w, r, 8192, &req) || req.PiecesDelta <= 0 {
		http.Error(w, "Invalid scanout payload", http.StatusBadRequest)
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

	var status string
	var comp, tgt int
	err = tx.QueryRowContext(ctx, `
		UPDATE ob_assignments_v52
		SET completed_qty = completed_qty + $4
		WHERE tenant_id=$1 AND po_id=$2 AND operation_id=$3 AND status='ASSIGNED'
		RETURNING status, completed_qty, target_qty`,
		tenant, req.POID, req.OperationID, req.PiecesDelta).Scan(&status, &comp, &tgt)
	if err != nil {
		http.Error(w, "Active OB assignment not found", http.StatusNotFound)
		return
	}
	_ = tx.Commit()
	docJSON(w, http.StatusOK, map[string]any{
		"po_id": req.POID, "operation_id": req.OperationID,
		"completed_qty": comp, "target_qty": tgt,
		"status": status, "auto_released": status == "RELEASED",
	})
}

// handleCheckerStagePassV52 implements Shop-Floor PRD §5.5 (checker PASS confirms behind-stage ops & logs missed reads).
func (s *EdgeServer) handleCheckerStagePassV52(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		GarmentID   string            `json:"garment_id"`
		CheckerID   string            `json:"checker_id"`
		StageOps    []string          `json:"stage_operations"`
		RecordedOps []string          `json:"recorded_operations"`
		StationByOp map[string]string `json:"station_by_op"`
	}
	if !candidateJSONBody(w, r, 32768, &req) {
		return
	}
	recSet := map[string]bool{}
	for _, op := range req.RecordedOps {
		recSet[op] = true
	}
	backCredited := []string{}
	missedReadStations := []string{}
	for _, op := range req.StageOps {
		if !recSet[op] {
			backCredited = append(backCredited, op)
			if stn, ok := req.StationByOp[op]; ok {
				missedReadStations = append(missedReadStations, stn)
			}
		}
	}
	docJSON(w, http.StatusOK, map[string]any{
		"garment_id":             req.GarmentID,
		"checker_id":             req.CheckerID,
		"back_credited_ops":      backCredited,
		"missed_read_stations":   missedReadStations,
		"authoritative_stage_ok": true,
	})
}

// handleWorkstationStoppageV52 records automated lost-time stoppages & escalations (Shop-Floor PRD §5.7).
func (s *EdgeServer) handleWorkstationStoppageV52(w http.ResponseWriter, r *http.Request) {
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
		PlantID     string `json:"plant_id"`
		LineID      string `json:"line_id"`
		StationID   string `json:"station_id"`
		OperatorID  string `json:"operator_id"`
		OperationID string `json:"operation_id"`
		Category    string `json:"category"` // MACHINE_BREAKDOWN, NO_WIP, OPERATOR_OWN_ISSUE
	}
	if !candidateJSONBody(w, r, 16384, &req) {
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

	var id string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO workstation_stoppages_v52 (
			tenant_id, plant_id, line_id, station_id, operator_id, operation_id,
			category, started_at, supervisor_alerted_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,clock_timestamp(),clock_timestamp())
		RETURNING id::text`,
		tenant, nonEmptyOr(req.PlantID, "PLANT_DEFAULT"), req.LineID, req.StationID,
		req.OperatorID, req.OperationID, strings.ToUpper(req.Category)).Scan(&id)
	if err != nil {
		http.Error(w, "Failed to log stoppage: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}
	_ = tx.Commit()
	docJSON(w, http.StatusCreated, map[string]any{"stoppage_id": id, "supervisor_alerted": true})
}

// handleOperatorSkillMatrixV52 computes Speed Against Standard & DHU per (operator, operation) (§5.8).
func (s *EdgeServer) handleOperatorSkillMatrixV52(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
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

	rows, err := tx.QueryContext(ctx, `
		SELECT a.operator_id, a.operator_name, a.operation_id,
		       SUM(a.completed_qty)::int AS pieces,
		       SUM(a.completed_qty * a.sam)::double precision AS earned_min,
		       COALESCE((
		           SELECT COUNT(*) FROM operator_defect_ledger_v52 d
		           WHERE d.tenant_id=a.tenant_id AND d.responsible_operator_id=a.operator_id
		             AND d.operation_id=a.operation_id
		       ), 0)::int AS defects,
		       COALESCE((
		           SELECT COUNT(*) FROM operator_defect_ledger_v52 d
		           WHERE d.tenant_id=a.tenant_id AND d.responsible_operator_id=a.operator_id
		             AND d.operation_id=a.operation_id AND d.status='OPEN'
		       ), 0)::int AS open_alterations
		FROM ob_assignments_v52 a
		WHERE a.tenant_id=$1
		GROUP BY a.tenant_id, a.operator_id, a.operator_name, a.operation_id
		ORDER BY a.operator_id, a.operation_id`, tenant)
	if err != nil {
		http.Error(w, "Failed to query skill matrix", http.StatusServiceUnavailable)
		return
	}
	defer rows.Close()
	matrix := []map[string]any{}
	for rows.Next() {
		var opID, opName, operation string
		var pcs, defects, openAlt int
		var earnedMin float64
		if rows.Scan(&opID, &opName, &operation, &pcs, &earnedMin, &defects, &openAlt) == nil {
			dhu := 0.0
			if pcs > 0 {
				dhu = (float64(defects) / float64(pcs)) * 100.0
			}
			matrix = append(matrix, map[string]any{
				"operator_id": opID, "operator_name": opName, "operation_id": operation,
				"pieces_completed": pcs, "earned_sam_minutes": earnedMin,
				"dhu": dhu, "open_alterations": openAlt,
			})
		}
	}
	_ = tx.Commit()
	docJSON(w, http.StatusOK, map[string]any{"skill_matrix": matrix})
}

// handleOutsideProcessChallanV52 tracks vendor challans, turnaround SLA, and vendor bill verification (§4.9).
func (s *EdgeServer) handleOutsideProcessChallanV52(w http.ResponseWriter, r *http.Request) {
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
		PlantID     string `json:"plant_id"`
		LotNumber   string `json:"lot_number"`
		VendorID    string `json:"vendor_id"`
		ProcessCode string `json:"process_code"`
		POID        string `json:"po_id"`
		StyleID     string `json:"style_id"`
		ColorCode   string `json:"color_code"`
		SentQty     int    `json:"sent_qty"`
		TagsSent    int    `json:"tags_sent"`
		SLAHours    int    `json:"agreed_turnaround_hours"`
	}
	if !candidateJSONBody(w, r, 16384, &req) {
		return
	}
	h := sha256.Sum256([]byte(tenant + "|" + req.LotNumber))
	barcode := "CHLN-" + strings.ToUpper(hex.EncodeToString(h[:8]))

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, err := s.docTransaction(ctx, tenant)
	if err != nil {
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}
	defer tx.Rollback()

	var id string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO outside_process_challans_v52 (
			tenant_id, plant_id, lot_number, challan_barcode, vendor_id, process_code,
			po_id, style_id, color_code, sent_qty, tags_sent_count, agreed_turnaround_hours
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id::text`,
		tenant, nonEmptyOr(req.PlantID, "PLANT_DEFAULT"), req.LotNumber, barcode, req.VendorID,
		req.ProcessCode, req.POID, req.StyleID, req.ColorCode, req.SentQty, req.TagsSent,
		nonZeroInt(req.SLAHours, 48)).Scan(&id)
	if err != nil {
		http.Error(w, "Failed to issue vendor challan: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}
	_ = tx.Commit()
	docJSON(w, http.StatusCreated, map[string]any{"challan_id": id, "challan_barcode": barcode})
}

// handleReusableRollTagIntervalV52 maps & releases reusable Active/NFC/UHF tags via interval history (§3.7, §6.2).
func (s *EdgeServer) handleReusableRollTagIntervalV52(w http.ResponseWriter, r *http.Request) {
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
		Action        string `json:"action"` // MAP or RELEASE
		PlantID       string `json:"plant_id"`
		EPCHex        string `json:"epc_hex"`
		TagType       string `json:"tag_type"`
		TargetType    string `json:"target_type"`
		TargetID      string `json:"target_id"`
		ActorID       string `json:"actor_id"`
		ReleaseReason string `json:"release_reason"`
	}
	if !candidateJSONBody(w, r, 16384, &req) {
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

	epc := strings.ToUpper(strings.TrimSpace(req.EPCHex))
	if strings.ToUpper(req.Action) == "RELEASE" {
		_, err = tx.ExecContext(ctx, `
			UPDATE roll_tag_mappings_v52
			SET released_at=clock_timestamp(), release_reason=$3
			WHERE tenant_id=$1 AND epc_hex=$2 AND released_at IS NULL`,
			tenant, epc, nonEmptyOr(req.ReleaseReason, "COMPLETED_AND_RETURNED_TO_POOL"))
		if err != nil {
			http.Error(w, "Failed to release tag mapping", http.StatusServiceUnavailable)
			return
		}
		_ = tx.Commit()
		docJSON(w, http.StatusOK, map[string]any{"epc_hex": epc, "status": "UNUSED_IN_POOL"})
		return
	}

	var id string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO roll_tag_mappings_v52 (
			tenant_id, plant_id, epc_hex, tag_type, target_type, target_id, mapped_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id::text`,
		tenant, nonEmptyOr(req.PlantID, "PLANT_DEFAULT"), epc,
		nonEmptyOr(strings.ToUpper(req.TagType), "ACTIVE"),
		nonEmptyOr(strings.ToUpper(req.TargetType), "ROLL"),
		req.TargetID, nonEmptyOr(req.ActorID, "store_operator")).Scan(&id)
	if err != nil {
		http.Error(w, "ERR_TAG_ALREADY_MAPPED_OR_INVALID: "+err.Error(), http.StatusConflict)
		return
	}
	_ = tx.Commit()
	docJSON(w, http.StatusCreated, map[string]any{"mapping_id": id, "epc_hex": epc, "status": "MAPPED"})
}

func nonZeroFloat(v, def float64) float64 {
	if v > 0 {
		return v
	}
	return def
}

func nonZeroInt(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}
