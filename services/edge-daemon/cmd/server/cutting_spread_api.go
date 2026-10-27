// Batch 08B: a signed-issued-material backed cutting spread report.
// A development operator can capture *reported* evidence; it never releases
// material, mints an RFID scan, or asserts cutting completed.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

const cutSpreadHold = "REPORTED_PENDING_INDEPENDENT_VERIFICATION"

func (s *EdgeServer) handleCuttingSpreads(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/v1/cutting/spreads" {
		http.NotFound(w, r)
		return
	}
	tenant, e := intakeTenant()
	if e != nil || s.db == nil {
		http.Error(w, "Cutting operator/database unavailable", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	switch r.Method {
	case http.MethodGet:
		draftID := r.URL.Query().Get("draft_id")
		if draftID != "" && !cutDraftUUID.MatchString(draftID) {
			docJSON(w, 400, map[string]string{"error": "ERR_CUT_DRAFT_ID"})
			return
		}
		result, e := s.readCuttingSpreads(ctx, tenant, draftID)
		if e != nil {
			http.Error(w, "Cutting evidence unavailable", 503)
			return
		}
		docJSON(w, 200, result)
	case http.MethodPost:
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "Expected application/json", 415)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var v cutSpreadInput
		if e = dec.Decode(&v); e != nil {
			docJSON(w, 400, map[string]string{"error": "ERR_CUT_SPREAD_JSON"})
			return
		}
		var extra any
		if dec.Decode(&extra) != io.EOF {
			docJSON(w, 400, map[string]string{"error": "ERR_CUT_SPREAD_EXTRA_JSON"})
			return
		}
		if e = validateCutSpread(v); e != nil {
			docJSON(w, 400, map[string]string{"error": e.Error()})
			return
		}
		var receipt cutSpreadReceipt
		var replay bool
		receipt, replay, e = s.saveCuttingSpread(ctx, tenant, v)
		if e != nil {
			code := e.Error()
			status := 409
			if code == "ERR_CUT_DRAFT_NOT_FOUND" {
				status = 404
			}
			if !strings.HasPrefix(code, "ERR_CUT_") {
				code = "ERR_CUT_SPREAD_DATABASE"
				status = 503
			}
			docJSON(w, status, map[string]string{"error": code})
			return
		}
		status := 201
		if replay {
			status = 200
		}
		docJSON(w, status, map[string]any{"spread": receipt, "replayed": replay, "authority": "EVIDENCE_ONLY_NOT_RELEASED"})
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", 405)
	}
}

func loadCutDraftForSpread(ctx context.Context, tx *sql.Tx, tenant, draftID string) (cutDraftReport, error) {
	var raw []byte
	var sha string
	e := tx.QueryRowContext(ctx, `SELECT s.snapshot,s.snapshot_sha256 FROM cut_lay_snapshots_v8 s WHERE s.tenant_id=$1 AND s.lay_draft_id=$2::uuid`, tenant, draftID).Scan(&raw, &sha)
	if e != nil {
		return cutDraftReport{}, e
	}
	return decodeCutSnapshot(raw, sha)
}
func (s *EdgeServer) saveCuttingSpread(ctx context.Context, tenant string, v cutSpreadInput) (cutSpreadReceipt, bool, error) {
	var zero cutSpreadReceipt
	if e := validateCutSpread(v); e != nil {
		return zero, false, e
	}
	tx, e := s.docTransaction(ctx, tenant)
	if e != nil {
		return zero, false, e
	}
	defer tx.Rollback()
	// Same per-order lock as the 08A draft writer. The lock prevents revisions
	// racing with the attribution transaction.
	var intakeID string
	var cutNo, latest int
	var po, fabric, shade string
	e = tx.QueryRowContext(ctx, `SELECT d.intake_id,d.cut_no,o.po_number,d.fabric_sku,d.shade_band FROM cut_lay_drafts_v8 d JOIN order_intakes_v5 o ON o.tenant_id=d.tenant_id AND o.id=d.intake_id WHERE d.tenant_id=$1 AND d.id=$2::uuid`, tenant, v.DraftID).Scan(&intakeID, &cutNo, &po, &fabric, &shade)
	if errors.Is(e, sql.ErrNoRows) {
		return zero, false, errors.New("ERR_CUT_DRAFT_NOT_FOUND")
	}
	if e != nil {
		return zero, false, e
	}
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1),hashtext($2))`, tenant, "CUT08A:"+intakeID); e != nil {
		return zero, false, e
	}
	// Serialize all issue allocations across drafts, including concurrent
	// operators, without relying on client-side counts or UI session state.
	// Advisory key is tenant-scoped; it also avoids scans racing the readback.
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1),hashtext($2))`, tenant, "CUT08B:ISSUE_ALLOCATIONS"); e != nil {
		return zero, false, e
	}
	hash := cutSpreadInputHash(v)
	var receipt cutSpreadReceipt
	var storedHash string
	e = tx.QueryRowContext(ctx, `SELECT id::text,lay_draft_id::text,request_key,input_sha256,reported_actual_plies,measured_marker_length_mm,reported_consumption_mm,draft_snapshot_sha256,state,created_at::text FROM cut_spread_reports_v8 WHERE tenant_id=$1 AND request_key=$2`, tenant, v.RequestKey).Scan(&receipt.ReportID, &receipt.DraftID, &receipt.RequestKey, &storedHash, &receipt.ActualPlies, &receipt.MeasuredMarkerLengthMM, &receipt.ConsumptionMM, &receipt.SnapshotSHA, &receipt.State, &receipt.CreatedAt)
	if e == nil {
		if storedHash != hash {
			return zero, false, errors.New("ERR_CUT_SPREAD_REQUEST_KEY_REUSE")
		}
		if e = tx.Commit(); e != nil {
			return zero, false, e
		}
		return receipt, true, nil
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return zero, false, e
	}
	var existing int
	e = tx.QueryRowContext(ctx, `SELECT count(*) FROM cut_spread_reports_v8 WHERE tenant_id=$1 AND lay_draft_id=$2::uuid`, tenant, v.DraftID).Scan(&existing)
	if e != nil {
		return zero, false, e
	}
	if existing != 0 {
		return zero, false, errors.New("ERR_CUT_SPREAD_DRAFT_ALREADY_REPORTED")
	}
	draft, e := loadCutDraftForSpread(ctx, tx, tenant, v.DraftID)
	if e != nil {
		return zero, false, e
	}
	if draft.ReportedPlies != v.ActualPlies {
		return zero, false, errors.New("ERR_CUT_SPREAD_PLY_MISMATCH_REVISE_DRAFT")
	}
	e = tx.QueryRowContext(ctx, `SELECT max(revision) FROM cut_lay_drafts_v8 WHERE tenant_id=$1 AND intake_id=$2 AND cut_no=$3`, tenant, intakeID, cutNo).Scan(&latest)
	if e != nil {
		return zero, false, e
	}
	if latest != draft.Revision {
		return zero, false, errors.New("ERR_CUT_SPREAD_STALE_DRAFT")
	}
	// Every claimed mm must have a corresponding signed warehouse issue whose
	// real cut, order, SKU and shade all agree with the source-linked draft.
	// Never treat raw roll passport entries or 07D pick proposals as issued stock.
	sorted := normalizedCutSpread(v).Allocations
	for _, alloc := range sorted {
		var issueMM, previousMM int64
		var rowCount int
		e = tx.QueryRowContext(ctx, `SELECT count(*)::int,COALESCE(max(i.length_mm),0)::bigint FROM material_issues_v5 i JOIN store_rolls sr ON sr.tenant_id=i.tenant_id AND sr.id=i.roll_id
 WHERE i.tenant_id=$1 AND i.id=$2::uuid AND  (i.order_id=$3 OR (i.order_id=$4 AND (SELECT count(*) FROM order_intakes_v5 oo WHERE oo.tenant_id=$1 AND oo.po_number=$4)=1)) AND i.cut_no=$5 AND sr.factory_sku=$6 AND sr.shade_band=$7`, tenant, alloc.IssueID, intakeID, po, cutNo, fabric, shade).Scan(&rowCount, &issueMM)
		if e != nil {
			return zero, false, e
		}
		if rowCount != 1 {
			return zero, false, errors.New("ERR_CUT_SPREAD_ISSUE_NOT_MATCHED")
		}
		e = tx.QueryRowContext(ctx, `SELECT COALESCE(sum(allocated_mm),0)::bigint FROM cut_spread_issue_allocations_v8 WHERE tenant_id=$1 AND issue_id=$2::uuid`, tenant, alloc.IssueID).Scan(&previousMM)
		if e != nil {
			return zero, false, e
		}
		if previousMM < 0 || previousMM > issueMM || alloc.AllocatedMM > issueMM-previousMM {
			return zero, false, errors.New("ERR_CUT_SPREAD_ISSUE_OVERATTRIBUTED")
		}
	}
	e = tx.QueryRowContext(ctx, `INSERT INTO cut_spread_reports_v8(tenant_id,lay_draft_id,request_key,input_sha256,draft_snapshot_sha256,reported_actual_plies,measured_marker_length_mm,reported_consumption_mm,evidence_note)
 VALUES($1,$2::uuid,$3,$4,$5,$6,$7,$8,$9) RETURNING id::text,created_at::text`, tenant, v.DraftID, v.RequestKey, hash, draft.SnapshotSHA256, v.ActualPlies, v.MeasuredMarkerLengthMM, v.ConsumptionMM, v.Note).Scan(&receipt.ReportID, &receipt.CreatedAt)
	if e != nil {
		return zero, false, e
	}
	for _, alloc := range sorted {
		if _, e = tx.ExecContext(ctx, `INSERT INTO cut_spread_issue_allocations_v8(tenant_id,spread_report_id,issue_id,allocated_mm) VALUES($1,$2::uuid,$3::uuid,$4)`, tenant, receipt.ReportID, alloc.IssueID, alloc.AllocatedMM); e != nil {
			return zero, false, e
		}
	}
	if e = tx.Commit(); e != nil {
		return zero, false, e
	}
	receipt.DraftID = v.DraftID
	receipt.RequestKey = v.RequestKey
	receipt.ActualPlies = v.ActualPlies
	receipt.MeasuredMarkerLengthMM = v.MeasuredMarkerLengthMM
	receipt.ConsumptionMM = v.ConsumptionMM
	receipt.SnapshotSHA = draft.SnapshotSHA256
	receipt.State = cutSpreadHold
	receipt.Allocations = sorted
	receipt.TotalBundles = draft.BundleCount
	return receipt, false, nil
}

func (s *EdgeServer) readCuttingSpreads(ctx context.Context, tenant, draftID string) (map[string]any, error) {
	tx, e := s.docTransaction(ctx, tenant)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	out := []cutSpreadReceipt{}
	var rows *sql.Rows
	if draftID != "" {
		rows, e = tx.QueryContext(ctx, `SELECT s.id::text,s.lay_draft_id::text,s.request_key,s.reported_actual_plies,s.measured_marker_length_mm,s.reported_consumption_mm,s.draft_snapshot_sha256,s.state,s.created_at::text,
 (SELECT count(*) FROM cut_bundle_scan_witnesses_v8 x WHERE x.tenant_id=s.tenant_id AND x.spread_report_id=s.id),
 (SELECT count(*) FROM cut_bundle_drafts_v8 b WHERE b.tenant_id=s.tenant_id AND b.lay_draft_id=s.lay_draft_id)
 FROM cut_spread_reports_v8 s WHERE s.tenant_id=$1 AND s.lay_draft_id=$2::uuid`, tenant, draftID)
	} else {
		rows, e = tx.QueryContext(ctx, `SELECT s.id::text,s.lay_draft_id::text,s.request_key,s.reported_actual_plies,s.measured_marker_length_mm,s.reported_consumption_mm,s.draft_snapshot_sha256,s.state,s.created_at::text,
 (SELECT count(*) FROM cut_bundle_scan_witnesses_v8 x WHERE x.tenant_id=s.tenant_id AND x.spread_report_id=s.id),
 (SELECT count(*) FROM cut_bundle_drafts_v8 b WHERE b.tenant_id=s.tenant_id AND b.lay_draft_id=s.lay_draft_id)
 FROM cut_spread_reports_v8 s WHERE s.tenant_id=$1 ORDER BY s.created_at DESC LIMIT 100`, tenant)
	}
	if e != nil {
		return nil, e
	}
	for rows.Next() {
		var x cutSpreadReceipt
		if e = rows.Scan(&x.ReportID, &x.DraftID, &x.RequestKey, &x.ActualPlies, &x.MeasuredMarkerLengthMM, &x.ConsumptionMM, &x.SnapshotSHA, &x.State, &x.CreatedAt, &x.ScannedBundles, &x.TotalBundles); e != nil {
			break
		}
		out = append(out, x)
	}
	if x := rows.Err(); e == nil {
		e = x
	}
	rows.Close()
	if e != nil {
		return nil, e
	}
	issues := []cutEligibleIssue{}
	scans := []map[string]any{}
	if draftID != "" {
		draft, e := loadCutDraftForSpread(ctx, tx, tenant, draftID)
		if e != nil {
			return nil, e
		}
		// Only an issue matching this exact order/cut/material/shade may be offered
		// for a new spread; historical allocation totals remain visible.
		rows, e = tx.QueryContext(ctx, `SELECT i.id::text,i.roll_id,i.length_mm,COALESCE((SELECT sum(a.allocated_mm) FROM cut_spread_issue_allocations_v8 a WHERE a.tenant_id=i.tenant_id AND a.issue_id=i.id),0)::bigint
 FROM material_issues_v5 i JOIN store_rolls sr ON sr.tenant_id=i.tenant_id AND sr.id=i.roll_id
 WHERE i.tenant_id=$1 AND  (i.order_id=$2 OR (i.order_id=$3 AND (SELECT count(*) FROM order_intakes_v5 oo WHERE oo.tenant_id=$1 AND oo.po_number=$3)=1)) AND i.cut_no=$4 AND sr.factory_sku=$5 AND sr.shade_band=$6
 ORDER BY i.committed_at,i.id LIMIT 80`, tenant, draft.IntakeID, draft.PONumber, draft.CutNo, draft.FabricSKU, draft.ShadeBand)
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			var item cutEligibleIssue
			if e = rows.Scan(&item.IssueID, &item.RollID, &item.IssuedMM, &item.AlreadyAttributedMM); e != nil {
				break
			}
			item.AvailableForAttributionMM = item.IssuedMM - item.AlreadyAttributedMM
			if item.AvailableForAttributionMM < 0 {
				return nil, errors.New("CUT_ISSUE_LEDGER_CORRUPT")
			}
			issues = append(issues, item)
		}
		if x := rows.Err(); e == nil {
			e = x
		}
		rows.Close()
		if e != nil {
			return nil, e
		}
		rows, e = tx.QueryContext(ctx, `SELECT label_code,epc_hex,scanner_subject,created_at::text,witness_state FROM cut_bundle_scan_witnesses_v8 WHERE tenant_id=$1 AND lay_draft_id=$2::uuid ORDER BY created_at,label_code LIMIT 650`, tenant, draftID)
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			var label, epc, subject, at, state string
			if e = rows.Scan(&label, &epc, &subject, &at, &state); e != nil {
				break
			}
			scans = append(scans, map[string]any{"label_code": label, "epc_hex": epc, "witness_subject": subject, "created_at": at, "state": state})
		}
		if x := rows.Err(); e == nil {
			e = x
		}
		rows.Close()
		if e != nil {
			return nil, e
		}
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return map[string]any{"spreads": out, "matching_signed_issues": issues, "bundle_witnesses": scans, "authority": "OPERATOR_REPORTED_ONLY_NO_RELEASE", "hardware_rfid_proven": false}, nil
}
