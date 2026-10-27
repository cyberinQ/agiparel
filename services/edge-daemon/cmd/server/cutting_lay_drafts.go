// Batch 08A: append-only cutting lay plans and classic part x marker x ply
// draft genealogy. No material issue, physical cut proof, or RFID bind occurs.
package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var cutDraftUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
var errCutInput = errors.New("CUT_INPUT_INVALID")

type cutOrderOption struct {
	IntakeID  string `json:"intake_id"`
	PONumber  string `json:"po_number"`
	StyleCode string `json:"style_code"`
	Quantity  int64  `json:"ordered_qty"`
	Status    string `json:"status"`
}

func (s *EdgeServer) handleCuttingLayDrafts(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/v1/cutting/lay-drafts" {
		http.NotFound(w, r)
		return
	}
	tenant, e := intakeTenant()
	if e != nil || s.db == nil {
		http.Error(w, "Cutting tenant/database unavailable", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 18*time.Second)
	defer cancel()
	switch r.Method {
	case http.MethodGet:
		if sourceID := r.URL.Query().Get("intake_id"); sourceID != "" {
			if !passportID.MatchString(sourceID) || len(sourceID) > 40 {
				docJSON(w, 400, map[string]string{"error": "invalid buyer intake identifier"})
				return
			}
			source, err := s.readCutOrderSource(ctx, tenant, sourceID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					docJSON(w, 404, map[string]string{"error": "Buyer order not found"})
				} else {
					http.Error(w, "Source SKU matrix unavailable", 503)
				}
				return
			}
			docJSON(w, 200, source)
			return
		}
		if id := r.URL.Query().Get("draft_id"); id != "" && !cutDraftUUID.MatchString(id) {
			docJSON(w, 400, map[string]string{"error": "invalid cutting draft id"})
			return
		}
		orders, drafts, e := s.readCutDrafts(ctx, tenant, r.URL.Query().Get("draft_id"))
		if e != nil {
			http.Error(w, "Unable to load cutting draft ledger", 503)
			return
		}
		docJSON(w, 200, map[string]any{"orders": orders, "drafts": drafts, "authority": "DRAFT_ONLY", "rfid": "NO_PHYSICAL_BIND"})
	case http.MethodPost:
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "Expected application/json", 415)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var v cutDraftInput
		if dec.Decode(&v) != nil {
			http.Error(w, "Invalid cutting draft JSON", 400)
			return
		}
		var extra any
		if dec.Decode(&extra) != io.EOF {
			http.Error(w, "Expected one JSON object", 400)
			return
		}
		if e = validateCutDraft(v); e != nil {
			docJSON(w, 400, map[string]string{"error": e.Error()})
			return
		}
		result, replayed, e := s.saveCutDraft(ctx, tenant, v)
		if e != nil {
			switch e.Error() {
			case "INTAKE_NOT_FOUND", "FABRIC_NOT_IN_BOM", "SKU_MATRIX_MISSING", "SKU_NOT_IN_BUYER_ORDER":
				docJSON(w, 422, map[string]string{"error": e.Error()})
			case "REQUEST_KEY_REUSE", "REVISION_CONFLICT", "OVERPLANNED_BUYER_SKU", "OVERPLANNED_BUYER_ORDER", "CUT_REVISION_FROZEN_BY_SPREAD_EVIDENCE":
				docJSON(w, 409, map[string]string{"error": e.Error()})
			default:
				http.Error(w, "Unable to save cutting draft", 503)
			}
			return
		}
		status := 201
		if replayed {
			status = 200
		}
		docJSON(w, status, map[string]any{"draft": result, "replayed": replayed, "authority": "DRAFT_ONLY"})
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", 405)
	}
}

// Source facts displayed beside the worksheet; not an approval or release.
type cutSourceSKU struct {
	ColorCode string `json:"color_code"`
	SizeCode  string `json:"size_code"`
	Quantity  int64  `json:"ordered_qty"`
}

func (s *EdgeServer) readCutOrderSource(ctx context.Context, tenant, id string) (map[string]any, error) {
	tx, e := s.docTransaction(ctx, tenant)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	var o cutOrderOption
	e = tx.QueryRowContext(ctx, `SELECT id,po_number,style_code,ordered_qty,status FROM order_intakes_v5 WHERE tenant_id=$1 AND id=$2`, tenant, id).Scan(&o.IntakeID, &o.PONumber, &o.StyleCode, &o.Quantity, &o.Status)
	if e != nil {
		return nil, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT color_code,size_code,ordered_qty FROM order_intake_skus_v5 WHERE tenant_id=$1 AND intake_id=$2 ORDER BY color_code,size_code LIMIT 501`, tenant, id)
	if e != nil {
		return nil, e
	}
	skus := []cutSourceSKU{}
	for rows.Next() {
		var v cutSourceSKU
		if e = rows.Scan(&v.ColorCode, &v.SizeCode, &v.Quantity); e != nil {
			break
		}
		skus = append(skus, v)
	}
	if x := rows.Err(); e == nil {
		e = x
	}
	rows.Close()
	if e != nil {
		return nil, e
	}
	if len(skus) > 500 {
		return nil, errors.New("SOURCE_SKU_MATRIX_TOO_LARGE")
	}
	rows, e = tx.QueryContext(ctx, `SELECT DISTINCT material_sku FROM order_intake_bom_v5 WHERE tenant_id=$1 AND intake_id=$2 AND category='FABRIC' ORDER BY material_sku LIMIT 101`, tenant, id)
	if e != nil {
		return nil, e
	}
	fabrics := []string{}
	for rows.Next() {
		var v string
		if e = rows.Scan(&v); e != nil {
			break
		}
		fabrics = append(fabrics, v)
	}
	if x := rows.Err(); e == nil {
		e = x
	}
	rows.Close()
	if e != nil {
		return nil, e
	}
	if len(fabrics) > 100 {
		return nil, errors.New("SOURCE_FABRIC_BOM_TOO_LARGE")
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return map[string]any{"order": o, "skus": skus, "fabric_skus": fabrics, "authority": "SOURCE_FACTS_ONLY"}, nil
}

func cutSnapshotHash(v cutDraftReport) string {
	v.SnapshotSHA256 = ""
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// Read the immutable snapshot and check it has not been changed or corrupted.
func decodeCutSnapshot(raw []byte, storedHash string) (cutDraftReport, error) {
	var v cutDraftReport
	if e := json.Unmarshal(raw, &v); e != nil {
		return v, e
	}
	if cutSnapshotHash(v) != storedHash {
		return cutDraftReport{}, errors.New("ERR_CUT_SNAPSHOT_INTEGRITY")
	}
	return v, nil
}

func (s *EdgeServer) readCutDrafts(ctx context.Context, tenant, id string) ([]cutOrderOption, []cutDraftReport, error) {
	if id != "" && !cutDraftUUID.MatchString(id) {
		return nil, nil, errCutInput
	}
	tx, e := s.docTransaction(ctx, tenant)
	if e != nil {
		return nil, nil, e
	}
	defer tx.Rollback()
	orders := []cutOrderOption{}
	if id == "" {
		rows, e := tx.QueryContext(ctx, `SELECT id,po_number,style_code,ordered_qty,status FROM order_intakes_v5 WHERE tenant_id=$1 ORDER BY captured_at DESC,id DESC LIMIT 100`, tenant)
		if e != nil {
			return nil, nil, e
		}
		for rows.Next() {
			var o cutOrderOption
			if e = rows.Scan(&o.IntakeID, &o.PONumber, &o.StyleCode, &o.Quantity, &o.Status); e != nil {
				break
			}
			orders = append(orders, o)
		}
		if x := rows.Err(); e == nil {
			e = x
		}
		rows.Close()
		if e != nil {
			return nil, nil, e
		}
	}
	var rows *sql.Rows
	if id != "" {
		rows, e = tx.QueryContext(ctx, `SELECT snap.snapshot,snap.snapshot_sha256 FROM cut_lay_snapshots_v8 snap WHERE snap.tenant_id=$1 AND snap.lay_draft_id=$2`, tenant, id)
	} else {
		rows, e = tx.QueryContext(ctx, `SELECT snap.snapshot,snap.snapshot_sha256 FROM cut_lay_drafts_v8 d
   JOIN cut_lay_snapshots_v8 snap ON snap.tenant_id=d.tenant_id AND snap.lay_draft_id=d.id
   WHERE d.tenant_id=$1 AND d.revision=(SELECT MAX(x.revision) FROM cut_lay_drafts_v8 x
    WHERE x.tenant_id=d.tenant_id AND x.intake_id=d.intake_id AND x.cut_no=d.cut_no)
   ORDER BY d.created_at DESC,d.id DESC LIMIT 60`, tenant)
	}
	if e != nil {
		return nil, nil, e
	}
	out := []cutDraftReport{}
	for rows.Next() {
		var raw []byte
		var sha string
		if e = rows.Scan(&raw, &sha); e != nil {
			break
		}
		var report cutDraftReport
		report, e = decodeCutSnapshot(raw, sha)
		if e != nil {
			break
		}
		if id == "" {
			report.Bundles = nil
		}
		out = append(out, report)
	}
	if x := rows.Err(); e == nil {
		e = x
	}
	rows.Close()
	if e != nil {
		return nil, nil, e
	}
	if e = tx.Commit(); e != nil {
		return nil, nil, e
	}
	return orders, out, nil
}

func (s *EdgeServer) saveCutDraft(ctx context.Context, tenant string, v cutDraftInput) (cutDraftReport, bool, error) {
	var empty cutDraftReport
	if e := validateCutDraft(v); e != nil {
		return empty, false, e
	}
	// Exactly serialize revisions and quantity limits across all cut numbers of
	// one buyer intake. This lock covers only draft planning, never stock.
	tx, e := s.docTransaction(ctx, tenant)
	if e != nil {
		return empty, false, e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1),hashtext($2))`, tenant, "CUT08A:"+v.IntakeID); e != nil {
		return empty, false, e
	}
	inputHash := cutInputHash(v)
	var raw []byte
	var savedSHA, oldHash string
	e = tx.QueryRowContext(ctx, `SELECT snap.snapshot,snap.snapshot_sha256,d.input_sha256 FROM cut_lay_drafts_v8 d JOIN cut_lay_snapshots_v8 snap ON snap.tenant_id=d.tenant_id AND snap.lay_draft_id=d.id WHERE d.tenant_id=$1 AND d.request_key=$2`, tenant, v.RequestKey).Scan(&raw, &savedSHA, &oldHash)
	if e == nil {
		if inputHash != oldHash {
			return empty, false, errors.New("REQUEST_KEY_REUSE")
		}
		result, x := decodeCutSnapshot(raw, savedSHA)
		if x != nil {
			return empty, false, x
		}
		if x = tx.Commit(); x != nil {
			return empty, false, x
		}
		return result, true, nil
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return empty, false, e
	}
	var po, orderHash, status string
	var totalQty int64
	e = tx.QueryRowContext(ctx, `SELECT po_number,source_sha256,status,ordered_qty FROM order_intakes_v5 WHERE tenant_id=$1 AND id=$2`, tenant, v.IntakeID).Scan(&po, &orderHash, &status, &totalQty)
	if errors.Is(e, sql.ErrNoRows) {
		return empty, false, errors.New("INTAKE_NOT_FOUND")
	}
	if e != nil {
		return empty, false, e
	}
	if status != "PROPOSED" || totalQty <= 0 {
		return empty, false, errors.New("INTAKE_NOT_FOUND")
	}
	// The cut's material is an actual FABRIC component in this buyer-order BOM.
	var fabricCount int
	e = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM order_intake_bom_v5 WHERE tenant_id=$1 AND intake_id=$2 AND material_sku=$3 AND category='FABRIC'`, tenant, v.IntakeID, v.FabricSKU).Scan(&fabricCount)
	if e != nil {
		return empty, false, e
	}
	if fabricCount == 0 {
		return empty, false, errors.New("FABRIC_NOT_IN_BOM")
	}
	var lastRev int
	e = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(revision),0) FROM cut_lay_drafts_v8 WHERE tenant_id=$1 AND intake_id=$2 AND cut_no=$3`, tenant, v.IntakeID, v.CutNo).Scan(&lastRev)
	if e != nil {
		return empty, false, e
	}
	if lastRev != v.ExpectedRevision {
		return empty, false, errors.New("REVISION_CONFLICT")
	}
	// Once source-linked physical spread evidence exists, freeze this cut's
	// marker genealogy. Corrections require a supervised future workflow.
	if lastRev > 0 {
		var reported int
		e = tx.QueryRowContext(ctx, `SELECT count(*) FROM cut_spread_reports_v8 s JOIN cut_lay_drafts_v8 d ON d.tenant_id=s.tenant_id AND d.id=s.lay_draft_id WHERE d.tenant_id=$1 AND d.intake_id=$2 AND d.cut_no=$3`, tenant, v.IntakeID, v.CutNo).Scan(&reported)
		if e != nil {
			return empty, false, e
		}
		if reported > 0 {
			return empty, false, errors.New("CUT_REVISION_FROZEN_BY_SPREAD_EVIDENCE")
		}
	}
	nextRev := lastRev + 1
	bundles, e := buildCutBundles(tenant, v, nextRev)
	if e != nil {
		return empty, false, e
	}
	skuAllowed := make(map[string]int64)
	skuRows, e := tx.QueryContext(ctx, `SELECT color_code,size_code,ordered_qty FROM order_intake_skus_v5 WHERE tenant_id=$1 AND intake_id=$2`, tenant, v.IntakeID)
	if e != nil {
		return empty, false, e
	}
	for skuRows.Next() {
		var c, z string
		var qty int64
		if e = skuRows.Scan(&c, &z, &qty); e != nil {
			break
		}
		skuAllowed[c+"\x00"+z] = qty
	}
	if x := skuRows.Err(); e == nil {
		e = x
	}
	skuRows.Close()
	if e != nil {
		return empty, false, e
	}
	// Latest revision of every OTHER cut; updating this cut does not count its
	// old revision a second time. No truncate/partial count is permitted.
	previous := make(map[string]int64)
	rows, e := tx.QueryContext(ctx, `SELECT p.color_code,p.size_code,COALESCE(SUM(d.reported_plies),0)::bigint
 FROM cut_lay_positions_v8 p JOIN cut_lay_drafts_v8 d ON d.tenant_id=p.tenant_id AND d.id=p.lay_draft_id
 WHERE d.tenant_id=$1 AND d.intake_id=$2 AND d.cut_no<>$3 AND
 d.revision=(SELECT MAX(x.revision) FROM cut_lay_drafts_v8 x WHERE x.tenant_id=d.tenant_id AND x.intake_id=d.intake_id AND x.cut_no=d.cut_no)
 GROUP BY p.color_code,p.size_code`, tenant, v.IntakeID, v.CutNo)
	if e != nil {
		return empty, false, e
	}
	for rows.Next() {
		var c, z string
		var qty int64
		if e = rows.Scan(&c, &z, &qty); e != nil {
			break
		}
		previous[c+"\x00"+z] = qty
	}
	if x := rows.Err(); e == nil {
		e = x
	}
	rows.Close()
	if e != nil {
		return empty, false, e
	}
	proposed := cutCounts(v.Positions, v.ReportedPlies)
	if e = checkCutQuantities(skuAllowed, previous, proposed); e != nil {
		return empty, false, e
	}
	if e = checkCutTotal(totalQty, previous, proposed); e != nil {
		return empty, false, e
	}
	// Signed issue evidence is only a read-only fact, not cutting proof.
	var verifiedIssues int
	var issuedMM int64
	e = tx.QueryRowContext(ctx, `SELECT COUNT(*)::int,COALESCE(SUM(mi.length_mm),0)::bigint FROM material_issues_v5 mi
 JOIN store_rolls sr ON sr.tenant_id=mi.tenant_id AND sr.id=mi.roll_id
 WHERE mi.tenant_id=$1 AND mi.order_id IN ($2,$3) AND mi.cut_no=$4
 AND sr.factory_sku=$5 AND sr.shade_band=$6`, tenant, v.IntakeID, po, v.CutNo, v.FabricSKU, v.ShadeBand).Scan(&verifiedIssues, &issuedMM)
	if e != nil {
		return empty, false, e
	}
	report := cutDraftReport{
		IntakeID: v.IntakeID, PONumber: po, CutNo: v.CutNo, Revision: nextRev, MarkerCode: v.MarkerCode,
		FabricSKU: v.FabricSKU, ShadeBand: v.ShadeBand, MarkerLengthMM: v.MarkerLengthMM,
		UsableWidthMM: v.UsableWidthMM, ReportedPlies: v.ReportedPlies, BundlePlyGroupSize: v.BundlePlyGroupSize,
		PlannedGarments: len(v.Positions) * v.ReportedPlies, PositionCount: len(v.Positions),
		BundleCount: len(bundles), SourceOrderSHA256: orderHash,
		State: "PROPOSED_HOLD", Authority: "NO_CUTTING_RELEASE",
		IssuedLengthMM: issuedMM, VerifiedIssueCount: verifiedIssues,
		Warning: "Operator-entered ply counts and printable bundle labels are DRAFT only. No machine vision, physical RFID scans, consumption confirmation, or signed cutting release is implied.",
		Note:    v.Note, Bundles: bundles,
	}
	for _, b := range bundles {
		report.PlannedParts += b.PieceCount
	}
	var id string
	var createdAt time.Time
	// Insert first to obtain a genuine database UUID; report is then immutable.
	e = tx.QueryRowContext(ctx, `INSERT INTO cut_lay_drafts_v8
 (tenant_id,intake_id,cut_no,revision,request_key,input_sha256,source_order_sha256,
 marker_code,fabric_sku,shade_band,marker_length_mm,usable_width_mm,reported_plies,bundle_ply_group_size,planned_garments,review_note)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
 RETURNING id::text,created_at`, tenant, v.IntakeID, v.CutNo, nextRev, v.RequestKey, inputHash, orderHash,
		v.MarkerCode, v.FabricSKU, v.ShadeBand, v.MarkerLengthMM, v.UsableWidthMM, v.ReportedPlies, v.BundlePlyGroupSize, report.PlannedGarments, v.Note).Scan(&id, &createdAt)
	if e != nil {
		return empty, false, e
	}
	// Record the finalized report with the database-assigned UUID in a separate
	// immutable snapshot row. No UPDATE privilege or trigger exception needed.
	report.DraftID = id
	report.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	report.SnapshotSHA256 = cutSnapshotHash(report)
	serialized, e := json.Marshal(report)
	if e != nil {
		return empty, false, e
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO cut_lay_snapshots_v8(tenant_id,lay_draft_id,snapshot,snapshot_sha256)
 VALUES($1,$2,$3::jsonb,$4)`, tenant, id, string(serialized), report.SnapshotSHA256)
	if e != nil {
		return empty, false, e
	}
	for _, p := range v.Positions {
		_, e = tx.ExecContext(ctx, `INSERT INTO cut_lay_positions_v8(tenant_id,lay_draft_id,marker_position,color_code,size_code)
 VALUES($1,$2,$3,$4,$5)`, tenant, id, p.Position, p.ColorCode, p.SizeCode)
		if e != nil {
			return empty, false, e
		}
	}
	for _, b := range bundles {
		_, e = tx.ExecContext(ctx, `INSERT INTO cut_bundle_drafts_v8(tenant_id,lay_draft_id,label_code,marker_position,color_code,size_code,part_code,pieces_per_garment,ply_group,ply_start,ply_end,piece_count)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, tenant, id, b.LabelCode, b.Position, b.ColorCode, b.SizeCode, b.PartCode, b.PiecesPerGarment, b.PlyGroup, b.PlyStart, b.PlyEnd, b.PieceCount)
		if e != nil {
			return empty, false, e
		}
	}
	if e = tx.Commit(); e != nil {
		return empty, false, e
	}
	return report, false, nil
}
