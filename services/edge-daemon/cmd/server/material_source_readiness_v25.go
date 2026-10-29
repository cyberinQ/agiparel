// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// F3.15B: report scoped PROPOSED buyer BOM alongside observed, signed cutting
// release consumption. This is evidence comparison, never a procurement gate.
package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"
)

type sourceMaterialLine struct {
	SKU                           string `json:"material_sku"`
	Category                      string `json:"category"`
	UOM                           string `json:"uom"`
	QtyPerGarmentMilli            int64  `json:"qty_per_garment_milli"`
	DeclaredOrderRequirementMilli int64  `json:"declared_order_requirement_milli"`
}
type sourceMaterialEvidence struct {
	Authority                     string               `json:"authority"`
	ProductionRelease             bool                 `json:"production_release"`
	WorkOrderID                   string               `json:"work_order_id"`
	PlantID                       string               `json:"plant_id"`
	PONumber                      string               `json:"po_number"`
	SourceIntakeID                string               `json:"source_intake_id,omitempty"`
	SourceSHA256                  string               `json:"source_sha256,omitempty"`
	SourceState                   string               `json:"source_state"`
	SourceOrderedQty              int64                `json:"source_ordered_qty,omitempty"`
	Lines                         []sourceMaterialLine `json:"bom_lines"`
	SignedReleaseCount            int64                `json:"signed_cutting_releases"`
	SignedAttributedConsumptionMM int64                `json:"signed_attributed_consumption_mm"`
	MeasurementStatus             string               `json:"measurement_status"`
}

func (a *materialAPI) materialSourceReadiness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", 405)
		return
	}
	if !a.ready(w) {
		return
	}
	actor, err := a.authorize(r, "material.source.read", "stores_manager", true)
	if err != nil {
		materialError(w, err)
		return
	}
	id := r.URL.Query().Get("work_order_id")
	if id == "" || len(id) > 64 {
		respondMaterial(w, 400, map[string]string{"error": "ERR_F315B_ORDER_ID"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	tx, err := a.repository.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		materialError(w, err)
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "SELECT set_config('app.current_tenant_id',$1,true)", actor.TenantID); err != nil {
		materialError(w, err)
		return
	}
	out, err := loadSourceMaterialEvidence(ctx, tx, actor.TenantID, actor.PlantID, id)
	if errors.Is(err, sql.ErrNoRows) {
		respondMaterial(w, 404, map[string]string{"error": "ERR_F315B_ORDER_NOT_FOUND"})
		return
	}
	if err != nil {
		materialError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		materialError(w, err)
		return
	}
	respondMaterial(w, 200, out)
}
func loadSourceMaterialEvidence(ctx context.Context, tx *sql.Tx, tenant, plant, orderID string) (sourceMaterialEvidence, error) {
	out := sourceMaterialEvidence{Authority: "PROPOSED_BOM_VS_SIGNED_CUT_RELEASE_OBSERVATIONS_NOT_BUYER_APPROVAL", ProductionRelease: false,
		WorkOrderID: orderID, PlantID: plant, SourceState: "NO_SOURCE_INTAKE", Lines: []sourceMaterialLine{}, MeasurementStatus: "NOT_MEASURED"}
	var style string
	var qty int64
	err := tx.QueryRowContext(ctx, `SELECT w.po_number,s.style_code,w.ordered_qty
 FROM dynamic_work_orders w JOIN styles_master s ON s.id=w.style_id AND s.tenant_id=w.tenant_id
 WHERE w.tenant_id=$1 AND w.plant_id=$2 AND w.id=$3`, tenant, plant, orderID).Scan(&out.PONumber, &style, &qty)
	if err != nil {
		return out, err
	}
	var sourceCount int64
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM order_intakes_v5 WHERE tenant_id=$1 AND po_number=$2 AND style_code=$3`, tenant, out.PONumber, style).Scan(&sourceCount); err != nil {
		return out, err
	}
	if sourceCount > 1 {
		out.SourceState = "AMBIGUOUS_PROPOSED_INTAKES"
	}
	if sourceCount == 1 {
		err = tx.QueryRowContext(ctx, `SELECT id,source_sha256,status,ordered_qty FROM order_intakes_v5
      WHERE tenant_id=$1 AND po_number=$2 AND style_code=$3`, tenant, out.PONumber, style).
			Scan(&out.SourceIntakeID, &out.SourceSHA256, &out.SourceState, &out.SourceOrderedQty)
		if err != nil {
			return out, err
		}
		// Source intake is *always* PROPOSED in migration 0007, not buyer-approved.
		out.SourceState = "PROPOSED_BUYER_BOM_NOT_APPROVED"
		if out.SourceOrderedQty != qty {
			out.SourceState = "PROPOSED_BOM_ORDER_QUANTITY_MISMATCH"
		}
		rows, err := tx.QueryContext(ctx, `SELECT material_sku,category,uom,
      ROUND(qty_per_garment*1000)::bigint,
      ROUND(qty_per_garment*$3::bigint*1000)::bigint
    FROM order_intake_bom_v5 WHERE tenant_id=$1 AND intake_id=$2 ORDER BY category,material_sku,uom`, tenant, out.SourceIntakeID, qty)
		if err != nil {
			return out, err
		}
		for rows.Next() {
			var l sourceMaterialLine
			if err = rows.Scan(&l.SKU, &l.Category, &l.UOM, &l.QtyPerGarmentMilli, &l.DeclaredOrderRequirementMilli); err != nil {
				rows.Close()
				return out, err
			}
			out.Lines = append(out.Lines, l)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return out, err
		}
	}
	// Only signed releases, never merely proposed spreads, are considered observed.
	err = tx.QueryRowContext(ctx, `SELECT count(*)::bigint,COALESCE(sum(s.reported_consumption_mm),0)::bigint
 FROM cut_sewing_releases_v8 r JOIN cut_spread_reports_v8 s ON s.tenant_id=r.tenant_id AND s.id=r.spread_report_id
 JOIN cut_lay_drafts_v8 d ON d.tenant_id=r.tenant_id AND d.id=r.lay_draft_id
 JOIN order_intakes_v5 i ON i.tenant_id=d.tenant_id AND i.id=d.intake_id
 WHERE r.tenant_id=$1 AND r.plant_id=$2 AND i.po_number=$3 AND i.style_code=$4`, tenant, plant, out.PONumber, style).
		Scan(&out.SignedReleaseCount, &out.SignedAttributedConsumptionMM)
	if err != nil {
		return out, err
	}
	if out.SignedReleaseCount > 0 {
		out.MeasurementStatus = "SIGNED_CUT_RELEASE_REPORTED_LENGTH_NOT_INDEPENDENT_CALIBRATION"
	}
	return out, nil
}
