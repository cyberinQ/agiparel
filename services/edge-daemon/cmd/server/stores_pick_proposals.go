// Batch 07D. Stage a source-linked, ledger-based order-to-roll pick plan.
// This code NEVER changes authoritative stock, reservations, or issuances.
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
	"strings"
	"time"
)

type pickInput struct {
	IntakeID   string `json:"intake_id"`
	RequestKey string `json:"request_key"`
	Note       string `json:"note"`
}
type pickSaved struct {
	ID             string     `json:"id"`
	IntakeID       string     `json:"intake_id"`
	RequestKey     string     `json:"request_key"`
	Note           string     `json:"note"`
	Snapshot       pickReport `json:"snapshot"`
	SnapshotSHA256 string     `json:"snapshot_sha256"`
	State          string     `json:"state"`
	CreatedAt      time.Time  `json:"created_at"`
}

func validatePickInput(v pickInput) error {
	if !passportID.MatchString(v.IntakeID) || len(v.IntakeID) > 40 {
		return errors.New("intake_id: choose a staged order")
	}
	if !storesRequestKey.MatchString(v.RequestKey) {
		return errors.New("request_key: use 8–128 safe characters")
	}
	if len(strings.TrimSpace(v.Note)) < 12 || len(v.Note) > 1000 || strings.ContainsRune(v.Note, 0) {
		return errors.New("note: explain the material request (12–1000 characters)")
	}
	return nil
}

func (s *EdgeServer) handleStoresPickProposals(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/v1/stores/pick-proposals" {
		http.NotFound(w, r)
		return
	}
	tenant, err := intakeTenant()
	if err != nil || s.db == nil {
		http.Error(w, "Stores tenant/database unavailable", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 18*time.Second)
	defer cancel()
	switch r.Method {
	case http.MethodGet:
		orders, history, err := s.listPickProposals(ctx, tenant)
		if err != nil {
			http.Error(w, "Cannot load order and pick proposal register", 503)
			return
		}
		docJSON(w, 200, map[string]any{"orders": orders, "proposals": history, "authority": "PROPOSAL_ONLY"})
	case http.MethodPost:
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "Expected JSON", 415)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var v pickInput
		if err := dec.Decode(&v); err != nil {
			http.Error(w, "Invalid JSON", 400)
			return
		}
		var extra any
		if dec.Decode(&extra) != io.EOF {
			http.Error(w, "One request only", 400)
			return
		}
		if err := validatePickInput(v); err != nil {
			docJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		saved, replayed, err := s.savePickProposal(ctx, tenant, v)
		if err != nil {
			switch err.Error() {
			case "INTAKE_NOT_FOUND":
				docJSON(w, 404, map[string]string{"error": err.Error()})
			case "REQUEST_KEY_REUSE", "INTAKE_BOM_INVALID", "LEDGER_SNAPSHOT_TOO_LARGE":
				docJSON(w, 409, map[string]string{"error": err.Error()})
			default:
				http.Error(w, "Unable to create pick proposal", 503)
			}
			return
		}
		code := 201
		if replayed {
			code = 200
		}
		docJSON(w, code, map[string]any{"proposal": saved, "replayed": replayed, "authority": "PROPOSAL_ONLY"})
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", 405)
	}
}

func (s *EdgeServer) listPickProposals(ctx context.Context, tenant string) ([]pickOrder, []pickSaved, error) {
	tx, err := s.docTransaction(ctx, tenant)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id,po_number,style_code,ordered_qty,source_sha256,status
      FROM order_intakes_v5 WHERE tenant_id=$1 ORDER BY captured_at DESC,id DESC LIMIT 100`, tenant)
	if err != nil {
		return nil, nil, err
	}
	orders := make([]pickOrder, 0)
	for rows.Next() {
		var v pickOrder
		if err = rows.Scan(&v.IntakeID, &v.PONumber, &v.StyleCode, &v.Quantity, &v.SourceSHA256, &v.Status); err != nil {
			break
		}
		orders = append(orders, v)
	}
	if e := rows.Err(); err == nil {
		err = e
	}
	rows.Close()
	if err != nil {
		return nil, nil, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT id::text,intake_id,request_key,review_note,snapshot,snapshot_sha256,state,created_at
      FROM store_pick_proposals_v7 WHERE tenant_id=$1 ORDER BY created_at DESC,id DESC LIMIT 50`, tenant)
	if err != nil {
		return nil, nil, err
	}
	proposals := make([]pickSaved, 0)
	for rows.Next() {
		var v pickSaved
		var raw []byte
		if err = rows.Scan(&v.ID, &v.IntakeID, &v.RequestKey, &v.Note, &raw, &v.SnapshotSHA256, &v.State, &v.CreatedAt); err != nil {
			break
		}
		if err = json.Unmarshal(raw, &v.Snapshot); err != nil {
			break
		}
		canonical, hashErr := json.Marshal(v.Snapshot)
		if hashErr != nil {
			err = hashErr
			break
		}
		digest := sha256.Sum256(canonical)
		if hex.EncodeToString(digest[:]) != v.SnapshotSHA256 {
			err = errors.New("ERR_PICK_SNAPSHOT_TAMPERED")
			break
		}
		proposals = append(proposals, v)
	}
	if e := rows.Err(); err == nil {
		err = e
	}
	rows.Close()
	if err != nil {
		return nil, nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, nil, err
	}
	return orders, proposals, nil
}

func readPickOrderAndRolls(ctx context.Context, tx *sql.Tx, tenant, id string) (pickReport, error) {
	var o pickOrder
	err := tx.QueryRowContext(ctx, `SELECT id,po_number,style_code,ordered_qty,source_sha256,status
      FROM order_intakes_v5 WHERE tenant_id=$1 AND id=$2`, tenant, id).
		Scan(&o.IntakeID, &o.PONumber, &o.StyleCode, &o.Quantity, &o.SourceSHA256, &o.Status)
	if err != nil {
		return pickReport{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT material_sku,qty_per_garment::text,uom FROM order_intake_bom_v5
      WHERE tenant_id=$1 AND intake_id=$2 AND category='FABRIC' ORDER BY material_sku`, tenant, id)
	if err != nil {
		return pickReport{}, err
	}
	demands := make([]pickDemand, 0)
	for rows.Next() {
		var d pickDemand
		var rate string
		if err = rows.Scan(&d.MaterialSKU, &rate, &d.Unit); err != nil {
			break
		}
		d.PerGarmentMilli, err = scenarioScaled(rate, 3, 1000000000, false)
		if err != nil {
			break
		}
		demands = append(demands, d)
	}
	if e := rows.Err(); err == nil {
		err = e
	}
	rows.Close()
	if err != nil {
		return pickReport{}, errors.New("INTAKE_BOM_INVALID")
	}
	if len(demands) == 0 {
		return pickReport{}, errors.New("INTAKE_BOM_INVALID")
	}
	// Never silently truncate a large stock universe: an incomplete response
	// could look like a real shortage and cause a false procurement decision.
	rows, err = tx.QueryContext(ctx, `SELECT id,factory_sku,shade_band,status,inspection_status,reserved_order_id,
      ROUND(remaining_length_m*1000)::bigint,ROUND(usable_width_cm*10)::bigint,
      COALESCE((SELECT SUM(reserved_mm-issued_mm-released_mm) FROM material_reservations_v5 mr
        WHERE mr.tenant_id=store_rolls.tenant_id AND mr.roll_id=store_rolls.id),0)::bigint,
      COALESCE((SELECT SUM(reserved_mm-issued_mm-released_mm) FROM material_reservations_v5 mr
        WHERE mr.tenant_id=store_rolls.tenant_id AND mr.roll_id=store_rolls.id
          AND mr.order_id NOT IN ($2,$3)),0)::bigint,
      COALESCE((SELECT SUM(length_mm) FROM material_issues_v5 mi
        WHERE mi.tenant_id=store_rolls.tenant_id AND mi.roll_id=store_rolls.id
          AND mi.order_id NOT IN ($2,$3)),0)::bigint
      FROM store_rolls WHERE tenant_id=$1 ORDER BY id LIMIT 1001`, tenant, id, o.PONumber)
	if err != nil {
		return pickReport{}, err
	}
	rolls := make([]pickRoll, 0)
	for rows.Next() {
		var x pickRoll
		var owner sql.NullString
		if err = rows.Scan(&x.ID, &x.MaterialSKU, &x.Shade, &x.State, &x.Inspection, &owner, &x.RemainingMM, &x.WidthMM, &x.OpenReservationMM, &x.ForeignReservationMM, &x.ForeignIssuedMM); err != nil {
			break
		}
		x.OwnerOrder = owner.String
		rolls = append(rolls, x)
	}
	if e := rows.Err(); err == nil {
		err = e
	}
	rows.Close()
	if err != nil {
		return pickReport{}, err
	}
	if len(rolls) > 1000 {
		return pickReport{}, errors.New("LEDGER_SNAPSHOT_TOO_LARGE")
	}
	return buildPickReport(o, demands, rolls)
}

func (s *EdgeServer) savePickProposal(ctx context.Context, tenant string, input pickInput) (pickSaved, bool, error) {
	var result pickSaved
	// REPEATABLE READ gives one consistent order/BOM/roll snapshot. The
	// per-key advisory lock makes retries idempotent; physical authorization
	// MUST independently recheck stock under row locks.
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return result, false, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id',$1,true)`, tenant); err != nil {
		return result, false, err
	}
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1),hashtext($2))`, tenant, input.RequestKey); err != nil {
		return result, false, err
	}
	var raw []byte
	err = tx.QueryRowContext(ctx, `SELECT id::text,intake_id,request_key,review_note,snapshot,snapshot_sha256,state,created_at
      FROM store_pick_proposals_v7 WHERE tenant_id=$1 AND request_key=$2`, tenant, input.RequestKey).
		Scan(&result.ID, &result.IntakeID, &result.RequestKey, &result.Note, &raw, &result.SnapshotSHA256, &result.State, &result.CreatedAt)
	if err == nil {
		if result.IntakeID != input.IntakeID || result.Note != input.Note {
			return pickSaved{}, false, errors.New("REQUEST_KEY_REUSE")
		}
		if err = json.Unmarshal(raw, &result.Snapshot); err != nil {
			return pickSaved{}, false, err
		}
		canonical, e := json.Marshal(result.Snapshot)
		if e != nil {
			return pickSaved{}, false, e
		}
		digest := sha256.Sum256(canonical)
		if hex.EncodeToString(digest[:]) != result.SnapshotSHA256 {
			return pickSaved{}, false, errors.New("ERR_PICK_SNAPSHOT_TAMPERED")
		}
		return result, true, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return pickSaved{}, false, err
	}
	snap, err := readPickOrderAndRolls(ctx, tx, tenant, input.IntakeID)
	if errors.Is(err, sql.ErrNoRows) {
		return result, false, errors.New("INTAKE_NOT_FOUND")
	}
	if err != nil {
		return result, false, err
	}
	raw, err = json.Marshal(snap)
	if err != nil {
		return result, false, err
	}
	hash := sha256.Sum256(raw)
	result = pickSaved{IntakeID: input.IntakeID, RequestKey: input.RequestKey, Note: input.Note, Snapshot: snap, SnapshotSHA256: hex.EncodeToString(hash[:]), State: "PROPOSED_HOLD"}
	err = tx.QueryRowContext(ctx, `INSERT INTO store_pick_proposals_v7(tenant_id,intake_id,request_key,review_note,source_order_sha256,snapshot,snapshot_sha256)
      VALUES($1,$2,$3,$4,$5,$6::jsonb,$7) RETURNING id::text,created_at`, tenant, input.IntakeID, input.RequestKey, input.Note, snap.Order.SourceSHA256, string(raw), result.SnapshotSHA256).
		Scan(&result.ID, &result.CreatedAt)
	if err != nil {
		return pickSaved{}, false, err
	}
	return result, false, tx.Commit()
}
