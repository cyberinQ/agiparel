// Author: Sunil Jain, 2026-11-11 — Native SOP-2 cutting planning API and conservative source-backed gates.
// Preview operations cannot mint physical cutting, NFC, inventory or sewing release authority.
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
	"os"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/internal/web"
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/cutting/sop2"
)

const sop2Advisory = "ADVISORY_ONLY_NO_PHYSICAL_OR_SIGNED_AUTHORITY"

func registerCuttingSOP2Routes(mux *http.ServeMux, s *EdgeServer) {
	// Author: Sunil Jain, 2026-11-12 — Loopback-only operator training simulation.
	// All cutter demo state lives in the browser; this route grants NO production authority.
	mux.HandleFunc("/cutting/demo", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "GET required", http.StatusMethodNotAllowed)
			return
		}
		if os.Getenv("ENV") != "development" || os.Getenv("AGIPAREL_BIND_HOST") != "127.0.0.1" {
			http.NotFound(w, r)
			return
		}
		data, err := web.GetCutterDemoHTML()
		if err != nil {
			http.Error(w, "Cutter training page unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self'; img-src 'self' data:; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
		_, _ = w.Write(data)
	})
	mux.HandleFunc("/cutting/sop2", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET required", http.StatusMethodNotAllowed)
			return
		}
		data, err := web.GetCuttingSOP2HTML()
		if err != nil {
			http.Error(w, "SOP-2 view unavailable", 503)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(data)
	})
	mux.HandleFunc("/api/v1/cutting/sop2/solve", s.handleSOP2Solve)
	mux.HandleFunc("/api/v1/cutting/sop2/allocate-rolls", s.handleSOP2Rolls)
	mux.HandleFunc("/api/v1/cutting/sop2/schedule", s.handleSOP2Schedule)
	mux.HandleFunc("/api/v1/cutting/sop2/family-audit", s.handleSOP2Family)
	mux.HandleFunc("/api/v1/cutting/sop2/reconcile", s.handleSOP2Reconcile)
	mux.HandleFunc("/api/v1/cutting/sop2/targets", s.handleSOP2Targets)
	mux.HandleFunc("/api/v1/cutting/sop2/control-tower", s.handleSOP2ControlTower)
	mux.HandleFunc("/api/v1/cutting/sop2/inspections", s.handleSOP2Inspections)
}

func sop2Decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "POST required", 405)
		return false
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "JSON required", 415)
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(v) != nil {
		docJSON(w, 400, map[string]string{"error": "INVALID_SOP2_JSON"})
		return false
	}
	var rest any
	if decoder.Decode(&rest) != io.EOF {
		docJSON(w, 400, map[string]string{"error": "JSON_MUST_CONTAIN_ONE_OBJECT"})
		return false
	}
	return true
}

func sop2Ready(s *EdgeServer, w http.ResponseWriter) (string, bool) {
	tenant, err := intakeTenant()
	if err != nil || s.db == nil {
		docJSON(w, 503, map[string]string{"error": "SOP2_REQUIRES_DEVELOPMENT_TENANT_AND_DATABASE"})
		return "", false
	}
	return tenant, true
}

func sop2WriteEnabled(w http.ResponseWriter) bool {
	// The existing cutting chapter does not yet have an authenticated
	// production command. No client-provided actor or claimed proof may grant it.
	if os.Getenv("ENV") != "development" || os.Getenv("AGIPAREL_CUT_SOP2_WRITES_ENABLED") != "true" {
		docJSON(w, 403, map[string]string{"error": "SOP2_WRITES_DISABLED_PENDING_PRODUCTION_IDENTITY"})
		return false
	}
	return true
}

func (s *EdgeServer) handleSOP2Solve(w http.ResponseWriter, r *http.Request) {
	var input sop2.TargetRequest
	if !sop2Decode(w, r, &input) {
		return
	}
	out, err := sop2.SolveTargets(input)
	if err != nil {
		docJSON(w, 422, map[string]string{"error": err.Error()})
		return
	}
	docJSON(w, 200, map[string]any{"result": out, "authority": sop2Advisory, "warning": "Unverified marker templates, rolls and cutter capacities require independent admission before freezing or execution"})
}
func (s *EdgeServer) handleSOP2Rolls(w http.ResponseWriter, r *http.Request) {
	var input sop2.RollAllocationRequest
	if !sop2Decode(w, r, &input) {
		return
	}
	out, err := sop2.AllocateRolls(input)
	if err != nil {
		docJSON(w, 422, map[string]string{"error": err.Error()})
		return
	}
	docJSON(w, 200, map[string]any{"result": out, "authority": sop2Advisory})
}
func (s *EdgeServer) handleSOP2Schedule(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Needs    []sop2.SewingNeed  `json:"needs"`
		Capacity []sop2.CapacityDay `json:"capacity"`
	}
	if !sop2Decode(w, r, &input) {
		return
	}
	out, err := sop2.SuggestDailyCut(input.Needs, input.Capacity)
	if err != nil {
		docJSON(w, 422, map[string]string{"error": err.Error()})
		return
	}
	docJSON(w, 200, map[string]any{"result": out, "authority": sop2Advisory, "warning": "Caller-supplied sewing frozen flags are not authenticated; this is NOT a frozen production daily cut plan"})
}
func (s *EdgeServer) handleSOP2Family(w http.ResponseWriter, r *http.Request) {
	var input struct {
		SubBundles []sop2.SubBundle    `json:"sub_bundles"`
		Marriage   []sop2.MarriagePair `json:"marriage"`
		Scanned    []string            `json:"scanned"`
	}
	if !sop2Decode(w, r, &input) {
		return
	}
	out, err := sop2.VerifyFamily(input.SubBundles, input.Marriage, input.Scanned)
	if err != nil {
		docJSON(w, 422, map[string]string{"error": err.Error()})
		return
	}
	docJSON(w, 200, map[string]any{"result": out, "authority": sop2Advisory, "warning": "Scans in this endpoint are simulations; only the signed EPC release workflow can issue to sewing"})
}
func (s *EdgeServer) handleSOP2Reconcile(w http.ResponseWriter, r *http.Request) {
	var input sop2.FabricBalance
	if !sop2Decode(w, r, &input) {
		return
	}
	out, err := sop2.ReconcileFabric(input)
	if err != nil {
		docJSON(w, 422, map[string]string{"error": err.Error()})
		return
	}
	docJSON(w, 200, map[string]any{"result": out, "authority": sop2Advisory, "warning": "Caller inputs not reconciled against signed warehouse issues"})
}

type sop2TargetLine struct {
	Color      string `json:"color_code"`
	Size       string `json:"size_code"`
	MinExtraBP int64  `json:"min_extra_bp"`
	MaxExtraBP int64  `json:"max_extra_bp"`
	Target     int64  `json:"target"`
}
type sop2FreezeRequest struct {
	IntakeID     string           `json:"intake_id"`
	RequestKey   string           `json:"request_key"`
	POMinExtraBP int64            `json:"po_min_extra_bp"`
	POMaxExtraBP int64            `json:"po_max_extra_bp"`
	Lines        []sop2TargetLine `json:"lines"`
	Reason       string           `json:"reason"`
	ActorNote    string           `json:"actor_note"`
}

func sop2Hash(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func (s *EdgeServer) handleSOP2Targets(w http.ResponseWriter, r *http.Request) {
	tenant, ok := sop2Ready(s, w)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("intake_id")
		if !passportID.MatchString(id) {
			docJSON(w, 400, map[string]string{"error": "INVALID_INTAKE_ID"})
			return
		}
		result, err := s.loadSOP2Board(ctx, tenant, id)
		if err != nil {
			http.Error(w, "SOP2 target register unavailable; apply migration 0005", 503)
			return
		}
		docJSON(w, 200, result)
	case http.MethodPost:
		if !sop2WriteEnabled(w) {
			return
		}
		var input sop2FreezeRequest
		if !sop2Decode(w, r, &input) {
			return
		}
		result, status, err := s.saveSOP2Targets(ctx, tenant, input)
		if err != nil {
			docJSON(w, status, map[string]string{"error": err.Error()})
			return
		}
		docJSON(w, status, map[string]any{"target": result, "authority": "DEVELOPMENT_FROZEN_POLICY_SNAPSHOT_NOT_PRODUCTION_APPROVAL"})
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", 405)
	}
}

// Freeze only the first version. All later target changes require a separately
// authenticated, stakeholder-approved change request; we deliberately do not
// expose a shortcut that silently amends quantities after physical work.
func (s *EdgeServer) saveSOP2Targets(ctx context.Context, tenant string, v sop2FreezeRequest) (map[string]any, int, error) {
	if !passportID.MatchString(v.IntakeID) || !storesRequestKey.MatchString(v.RequestKey) || len(v.Lines) == 0 || len(v.Lines) > 500 || len(strings.TrimSpace(v.Reason)) < 12 || len(v.Reason) > 1000 || len(strings.TrimSpace(v.ActorNote)) < 3 || len(v.ActorNote) > 128 {
		return nil, 400, errors.New("SOP2_TARGET_REQUEST_INVALID")
	}
	tx, err := s.docTransaction(ctx, tenant)
	if err != nil {
		return nil, 503, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1),hashtext($2))`, tenant, "CUT08A:"+v.IntakeID); err != nil {
		return nil, 503, err
	}
	h := sop2Hash(v)
	var replayID, oldHash string
	err = tx.QueryRowContext(ctx, `SELECT id::text,request_sha256 FROM cut_sop2_target_versions_v46 WHERE tenant_id=$1 AND request_key=$2`, tenant, v.RequestKey).Scan(&replayID, &oldHash)
	if err == nil {
		if oldHash != h {
			return nil, 409, errors.New("REQUEST_KEY_REUSED_DIFFERENT_PAYLOAD")
		}
		if err = tx.Commit(); err != nil {
			return nil, 503, err
		}
		return map[string]any{"id": replayID, "version": 1, "replayed": true}, 200, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, 503, err
	}
	var versions int
	if err = tx.QueryRowContext(ctx, `SELECT count(*)::int FROM cut_sop2_target_versions_v46 WHERE tenant_id=$1 AND intake_id=$2`, tenant, v.IntakeID).Scan(&versions); err != nil {
		return nil, 503, err
	}
	if versions > 0 {
		return nil, 409, errors.New("FROZEN_TARGET_CHANGE_REQUIRES_STAKEHOLDER_APPROVALS")
	}
	var sourceHash, status string
	if err = tx.QueryRowContext(ctx, `SELECT source_sha256,status FROM order_intakes_v5 WHERE tenant_id=$1 AND id=$2`, tenant, v.IntakeID).Scan(&sourceHash, &status); err != nil {
		return nil, 404, errors.New("BUYER_ORDER_NOT_FOUND")
	}
	if status != "PROPOSED" {
		return nil, 409, errors.New("BUYER_ORDER_STATUS_NOT_SUPPORTED")
	}
	orderRows, err := tx.QueryContext(ctx, `SELECT color_code,size_code,ordered_qty FROM order_intake_skus_v5 WHERE tenant_id=$1 AND intake_id=$2`, tenant, v.IntakeID)
	if err != nil {
		return nil, 503, err
	}
	ordered := map[string]int64{}
	for orderRows.Next() {
		var color, size string
		var qty int64
		if err = orderRows.Scan(&color, &size, &qty); err != nil {
			break
		}
		ordered[color+"\x00"+size] += qty
	}
	if x := orderRows.Err(); err == nil {
		err = x
	}
	orderRows.Close()
	if err != nil {
		return nil, 503, err
	}
	if len(ordered) != len(v.Lines) {
		return nil, 422, errors.New("TARGET_SKU_MATRIX_DOES_NOT_MATCH_BUYER_ORDER")
	}
	skuList := make([]sop2.SKU, 0, len(v.Lines))
	targets := map[string]int64{}
	for _, line := range v.Lines {
		key := line.Color + "\x00" + line.Size
		o, ok := ordered[key]
		if !ok || o <= 0 {
			return nil, 422, errors.New("UNKNOWN_TARGET_SKU")
		}
		if _, exists := targets[key]; exists {
			return nil, 422, errors.New("DUPLICATE_TARGET_SKU")
		}
		targets[key] = line.Target
		skuList = append(skuList, sop2.SKU{Key: key, Ordered: o, MinExtraBP: line.MinExtraBP, MaxExtraBP: line.MaxExtraBP})
	}
	if err = sop2.ValidateFrozenTargets(skuList, targets, v.POMinExtraBP, v.POMaxExtraBP); err != nil {
		return nil, 422, err
	}
	// Existing latest draft quantities must also fit the new frozen ceiling.
	rows, err := tx.QueryContext(ctx, `SELECT p.color_code,p.size_code,COALESCE(sum(d.reported_plies),0)::bigint
 FROM cut_lay_positions_v8 p JOIN cut_lay_drafts_v8 d ON d.tenant_id=p.tenant_id AND d.id=p.lay_draft_id
 WHERE d.tenant_id=$1 AND d.intake_id=$2 AND d.revision=(SELECT max(x.revision) FROM cut_lay_drafts_v8 x WHERE x.tenant_id=d.tenant_id AND x.intake_id=d.intake_id AND x.cut_no=d.cut_no)
 GROUP BY p.color_code,p.size_code`, tenant, v.IntakeID)
	if err != nil {
		return nil, 503, err
	}
	for rows.Next() {
		var c, z string
		var n int64
		if err = rows.Scan(&c, &z, &n); err != nil {
			break
		}
		if n > targets[c+"\x00"+z] {
			err = errors.New("EXISTING_DRAFTS_EXCEED_FROZEN_TARGET")
			break
		}
	}
	if x := rows.Err(); err == nil {
		err = x
	}
	rows.Close()
	if err != nil {
		return nil, 409, err
	}
	var id string
	err = tx.QueryRowContext(ctx, `INSERT INTO cut_sop2_target_versions_v46(tenant_id,intake_id,version,po_min_extra_bp,po_max_extra_bp,source_order_sha256,request_key,request_sha256,actor_note,reason) VALUES($1,$2,1,$3,$4,$5,$6,$7,$8,$9) RETURNING id::text`, tenant, v.IntakeID, v.POMinExtraBP, v.POMaxExtraBP, sourceHash, v.RequestKey, h, v.ActorNote, v.Reason).Scan(&id)
	if err != nil {
		return nil, 503, err
	}
	for _, line := range v.Lines {
		_, err = tx.ExecContext(ctx, `INSERT INTO cut_sop2_target_skus_v46(tenant_id,target_id,color_code,size_code,ordered_qty,min_extra_bp,max_extra_bp,frozen_target) VALUES($1,$2::uuid,$3,$4,$5,$6,$7,$8)`, tenant, id, line.Color, line.Size, ordered[line.Color+"\x00"+line.Size], line.MinExtraBP, line.MaxExtraBP, line.Target)
		if err != nil {
			return nil, 503, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, 503, err
	}
	return map[string]any{"id": id, "version": 1, "replayed": false}, 201, nil
}

// Reconciliation of signed released garments is separate from draft-planned
// and operator-reported plies; no preview value is presented as physical truth.
func (s *EdgeServer) loadSOP2Board(ctx context.Context, tenant, id string) (map[string]any, error) {
	tx, err := s.docTransaction(ctx, tenant)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var po, style string
	err = tx.QueryRowContext(ctx, `SELECT po_number,style_code FROM order_intakes_v5 WHERE tenant_id=$1 AND id=$2`, tenant, id).Scan(&po, &style)
	if err != nil {
		return nil, err
	}
	var version int
	var targetID string
	err = tx.QueryRowContext(ctx, `SELECT id::text,version FROM cut_sop2_target_versions_v46 WHERE tenant_id=$1 AND intake_id=$2 ORDER BY version DESC LIMIT 1`, tenant, id).Scan(&targetID, &version)
	if errors.Is(err, sql.ErrNoRows) {
		targetID = ""
		version = 0
	} else if err != nil {
		return nil, err
	}
	targetMap := map[string]int64{}
	if targetID != "" {
		rows, e := tx.QueryContext(ctx, `SELECT color_code,size_code,frozen_target FROM cut_sop2_target_skus_v46 WHERE tenant_id=$1 AND target_id=$2::uuid`, tenant, targetID)
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			var c, z string
			var n int64
			if e = rows.Scan(&c, &z, &n); e != nil {
				break
			}
			targetMap[c+"\x00"+z] = n
		}
		if x := rows.Err(); e == nil {
			e = x
		}
		rows.Close()
		if e != nil {
			return nil, e
		}
	}
	// A signed release is stronger than spread operator testimony; it still
	// does not prove defect-free garments or final QC acceptance.
	type quantities struct{ Planned, Reported, Signed int64 }
	qmap := map[string]*quantities{}
	rows, err := tx.QueryContext(ctx, `SELECT p.color_code,p.size_code,
 COALESCE(sum(d.reported_plies),0)::bigint,
 COALESCE(sum(CASE WHEN s.id IS NOT NULL THEN s.reported_actual_plies ELSE 0 END),0)::bigint,
 COALESCE(sum(CASE WHEN rel.id IS NOT NULL THEN s.reported_actual_plies ELSE 0 END),0)::bigint
 FROM cut_lay_positions_v8 p JOIN cut_lay_drafts_v8 d ON d.tenant_id=p.tenant_id AND d.id=p.lay_draft_id
 LEFT JOIN cut_spread_reports_v8 s ON s.tenant_id=d.tenant_id AND s.lay_draft_id=d.id
 LEFT JOIN cut_sewing_releases_v8 rel ON rel.tenant_id=s.tenant_id AND rel.spread_report_id=s.id
 WHERE d.tenant_id=$1 AND d.intake_id=$2 AND d.revision=(SELECT MAX(x.revision) FROM cut_lay_drafts_v8 x WHERE x.tenant_id=d.tenant_id AND x.intake_id=d.intake_id AND x.cut_no=d.cut_no)
 GROUP BY p.color_code,p.size_code`, tenant, id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var c, z string
		var n quantities
		if err = rows.Scan(&c, &z, &n.Planned, &n.Reported, &n.Signed); err != nil {
			break
		}
		qmap[c+"\x00"+z] = &n
	}
	if x := rows.Err(); err == nil {
		err = x
	}
	rows.Close()
	if err != nil {
		return nil, err
	}
	baseRows, err := tx.QueryContext(ctx, `SELECT color_code,size_code,sum(ordered_qty)::bigint FROM order_intake_skus_v5 WHERE tenant_id=$1 AND intake_id=$2 GROUP BY color_code,size_code ORDER BY color_code,size_code`, tenant, id)
	if err != nil {
		return nil, err
	}
	lines := []map[string]any{}
	alerts := []string{}
	for baseRows.Next() {
		var c, z string
		var n int64
		if err = baseRows.Scan(&c, &z, &n); err != nil {
			break
		}
		k := c + "\x00" + z
		q := qmap[k]
		if q == nil {
			q = &quantities{}
		}
		entry := map[string]any{"color_code": c, "size_code": z, "buyer_ordered": n, "draft_planned": q.Planned, "operator_reported": q.Reported, "signed_released_equivalent": q.Signed}
		if q.Planned > q.Reported {
			alerts = append(alerts, "CUT_DRAFT_AWAITING_SPREAD:"+c+":"+z)
		}
		if q.Reported > q.Signed {
			alerts = append(alerts, "SPREAD_AWAITING_SIGNED_RELEASE:"+c+":"+z)
		}
		if targetID != "" {
			target := targetMap[k]
			balance := target - q.Signed
			entry["frozen_target"] = target
			entry["balance_after_signed_release"] = balance
			entry["draft_remaining"] = target - q.Planned
			if q.Planned > target {
				alerts = append(alerts, "DRAFT_OVER_TARGET:"+c+":"+z)
			}
			if q.Signed > target {
				alerts = append(alerts, "SIGNED_RELEASE_OVER_TARGET:"+c+":"+z)
			}
		}
		lines = append(lines, entry)
	}
	if x := baseRows.Err(); err == nil {
		err = x
	}
	baseRows.Close()
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return map[string]any{"intake_id": id, "po_number": po, "style": style, "target_version": version, "target_id": targetID, "lines": lines, "alerts": alerts, "authority": "SIGNED_RELEASE_DISTINGUISHED_FROM_DRAFT_AND_REPORTED", "warning": "Signed release is not final QC acceptance; frozen target is a development policy snapshot"}, nil
}

func (s *EdgeServer) handleSOP2ControlTower(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	tenant, ok := sop2Ready(s, w)
	if !ok {
		return
	}
	id := r.URL.Query().Get("intake_id")
	if !passportID.MatchString(id) {
		docJSON(w, 400, map[string]string{"error": "INVALID_INTAKE_ID"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	out, err := s.loadSOP2Board(ctx, tenant, id)
	if err != nil {
		http.Error(w, "SOP2 control tower unavailable", 503)
		return
	}
	docJSON(w, 200, out)
}

// Inspections are append-only evidence, not warehouse PASS admission.
// The acceptance standard must already exist in the tenant's policy register.
func (s *EdgeServer) handleSOP2Inspections(w http.ResponseWriter, r *http.Request) {
	tenant, ok := sop2Ready(s, w)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	if r.Method == http.MethodGet {
		roll := r.URL.Query().Get("roll_id")
		if !passportID.MatchString(roll) {
			docJSON(w, 400, map[string]string{"error": "INVALID_ROLL_ID"})
			return
		}
		tx, err := s.docTransaction(ctx, tenant)
		if err != nil {
			http.Error(w, "DB unavailable", 503)
			return
		}
		defer tx.Rollback()
		rows, err := tx.QueryContext(ctx, `SELECT id::text,decision,policy_sku,policy_revision,usable_width_mm,points,reasons,measured_at FROM cut_sop2_roll_inspections_v46 WHERE tenant_id=$1 AND roll_id=$2 ORDER BY measured_at DESC LIMIT 30`, tenant, roll)
		if err != nil {
			http.Error(w, "SOP2 inspection register unavailable", 503)
			return
		}
		entries := []map[string]any{}
		for rows.Next() {
			var id, state, sku string
			var version int
			var width, points int64
			var raw []byte
			var at time.Time
			if err = rows.Scan(&id, &state, &sku, &version, &width, &points, &raw, &at); err != nil {
				break
			}
			var reasons []string
			_ = json.Unmarshal(raw, &reasons)
			entries = append(entries, map[string]any{"id": id, "decision": state, "fabric_sku": sku, "policy_revision": version, "usable_width_mm": width, "points": points, "reasons": reasons, "measured_at": at})
		}
		if x := rows.Err(); err == nil {
			err = x
		}
		rows.Close()
		if err != nil {
			http.Error(w, "Inspection register corrupt", 503)
			return
		}
		if err = tx.Commit(); err != nil {
			http.Error(w, "Inspection transaction failure", 503)
			return
		}
		docJSON(w, 200, map[string]any{"items": entries, "authority": "INSPECTION_EVIDENCE_ONLY_NOT_WAREHOUSE_ADMISSION"})
		return
	}
	if !sop2WriteEnabled(w) {
		return
	}
	var input struct {
		Inspection sop2.Inspection `json:"inspection"`
		ActorNote  string          `json:"actor_note"`
	}
	if !sop2Decode(w, r, &input) {
		return
	}
	if !passportID.MatchString(input.Inspection.RollID) || len(strings.TrimSpace(input.ActorNote)) < 3 || len(input.ActorNote) > 128 {
		docJSON(w, 400, map[string]string{"error": "INSPECTION_IDENTITY_INVALID"})
		return
	}
	tx, err := s.docTransaction(ctx, tenant)
	if err != nil {
		http.Error(w, "DB unavailable", 503)
		return
	}
	defer tx.Rollback()
	var sku, shade string
	var ticket int64
	err = tx.QueryRowContext(ctx, `SELECT factory_sku,shade_band,(original_length_m*1000)::bigint FROM store_rolls WHERE tenant_id=$1 AND id=$2`, tenant, input.Inspection.RollID).Scan(&sku, &shade, &ticket)
	if err != nil {
		docJSON(w, 404, map[string]string{"error": "SOURCE_ROLL_NOT_FOUND"})
		return
	}
	if input.Inspection.Shade != shade || input.Inspection.TicketLengthMM != ticket {
		docJSON(w, 422, map[string]string{"error": "INSPECTION_DOES_NOT_MATCH_ROLL_PASSPORT"})
		return
	}
	var policy sop2.InspectionPolicy
	var revision int
	err = tx.QueryRowContext(ctx, `SELECT revision,min_width_mm,max_defect_points,max_length_loss_bp,width_samples,relax_hours FROM cut_sop2_inspection_policies_v46 WHERE tenant_id=$1 AND fabric_sku=$2 ORDER BY revision DESC LIMIT 1`, tenant, sku).Scan(&revision, &policy.MinUsableWidthMM, &policy.MaxDefectPoints, &policy.MaxLengthLossBP, &policy.RequiredSamples, &policy.RequiredRelaxHours)
	if err != nil {
		docJSON(w, 409, map[string]string{"error": "ACCEPTANCE_POLICY_REQUIRED_NO_UNVERIFIED_DEFAULT"})
		return
	}
	decision, err := sop2.InspectRoll(input.Inspection, policy)
	if err != nil {
		docJSON(w, 422, map[string]string{"error": err.Error()})
		return
	}
	widths, _ := json.Marshal(input.Inspection.WidthSamplesMM)
	defects, _ := json.Marshal(input.Inspection.Defects)
	reasons, _ := json.Marshal(decision.Reasons)
	var id string
	err = tx.QueryRowContext(ctx, `INSERT INTO cut_sop2_roll_inspections_v46(tenant_id,roll_id,policy_sku,policy_revision,ticket_length_mm,measured_length_mm,usable_width_mm,shade,measurements,defects,points,decision,reasons,inspected_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10::jsonb,$11,$12,$13::jsonb,$14) RETURNING id::text`, tenant, input.Inspection.RollID, sku, revision, ticket, input.Inspection.ActualLengthMM, decision.UsableWidthMM, shade, string(widths), string(defects), decision.DefectPoints, decision.Status, string(reasons), input.ActorNote).Scan(&id)
	if err != nil {
		http.Error(w, "SOP2 inspection save failed", 503)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "SOP2 inspection commit failed", 503)
		return
	}
	docJSON(w, 201, map[string]any{"id": id, "decision": decision, "authority": "INSPECTION_EVIDENCE_NOT_WAREHOUSE_ADMISSION"})
}
