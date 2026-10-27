// Author: Sunil Jain, 2026-10-27.
// E09 09A-C. Transactional sewing workflow for UI-emulated RFID scans.
// IMPORTANT: All records are SIMULATED; none are RF observations, signed worker
// assertions, operational credit, payroll, cutting release or shipping evidence.
package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/internal/web"
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/sewing"
)

var sewCode = regexp.MustCompile(`^[A-Za-z0-9_-]{2,64}$`)
var sewCommandID = regexp.MustCompile(`^[A-Za-z0-9_-]{12,80}$`)

const sewProvenance = "UI_SIMULATED_NO_RF_OBSERVATION"
const sewCreditState = "SIMULATED_NOT_PRODUCTION_CREDIT"
const sewMaxActiveSegment = 4 * time.Hour

type sewSimError struct {
	status int
	code   string
}

func (e *sewSimError) Error() string        { return e.code }
func sewFail(status int, code string) error { return &sewSimError{status, code} }
func sewReplyError(w http.ResponseWriter, e error) {
	var d *sewSimError
	if errors.As(e, &d) {
		docJSON(w, d.status, map[string]any{"error": d.code, "production_credit": false})
		return
	}
	docJSON(w, 503, map[string]any{"error": "ERR_SEW_SIM_DATABASE", "production_credit": false})
}
func sewDigest(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func sewStrictJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		docJSON(w, 415, map[string]string{"error": "ERR_JSON_CONTENT_TYPE"})
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		docJSON(w, 400, map[string]string{"error": "ERR_SEW_SIM_JSON"})
		return false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		docJSON(w, 400, map[string]string{"error": "ERR_SEW_SIM_EXTRA_JSON"})
		return false
	}
	return true
}
func (s *EdgeServer) sewTenant(w http.ResponseWriter) (string, bool) {
	if os.Getenv("ENV") != "development" {
		docJSON(w, 403, map[string]string{"error": "ERR_SIM_ONLY_DEVELOPMENT"})
		return "", false
	}
	tenant, e := intakeTenant()
	if e != nil || s.db == nil {
		docJSON(w, 503, map[string]string{"error": "ERR_SEW_SIM_NOT_CONFIGURED"})
		return "", false
	}
	return tenant, true
}
func sewFields(xs ...string) bool {
	for _, s := range xs {
		if !sewCode.MatchString(s) {
			return false
		}
	}
	return true
}
func sewUUID(s string) bool { return cutDraftUUID.MatchString(s) }
func sewTxLock(ctx context.Context, tx *sql.Tx, tenant, scope string) error {
	_, e := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1),hashtext($2))`, tenant, "SEW09:"+scope)
	return e
}

// If Cutting has a signed release, the simulation cannot borrow that lay
// under another plant identity. Unreleased drafts remain explicitly unverified.
func sewSimPlantGuard(ctx context.Context, tx *sql.Tx, tenant, lay, plant string) error {
	var mismatches int
	e := tx.QueryRowContext(ctx, `SELECT count(*)::int FROM cut_sewing_releases_v8
 WHERE tenant_id=$1 AND lay_draft_id=$2::uuid AND plant_id<>$3`, tenant, lay, plant).Scan(&mismatches)
	if e != nil {
		return e
	}
	if mismatches != 0 {
		return sewFail(409, "ERR_SEW_SIGNED_CUT_PLANT_MISMATCH")
	}
	return nil
}

type sewOpRequest struct {
	LayID         string               `json:"lay_id"`
	PlantID       string               `json:"plant_id"`
	OperationCode string               `json:"operation_code"`
	Mode          sewing.ExecutionMode `json:"mode"`
	RequiredParts []string             `json:"required_parts"`
	Predecessors  []string             `json:"predecessors"`
	MachineClass  string               `json:"machine_class"`
	SAMSeconds    float64              `json:"sam_seconds"`
}

func validSewOp(v sewOpRequest) error {
	if !sewUUID(v.LayID) || !sewFields(v.PlantID, v.OperationCode, v.MachineClass) || v.SAMSeconds <= 0 || v.SAMSeconds > 36000 {
		return sewFail(422, "ERR_SEW_OP_FIELDS")
	}
	if v.Mode != sewing.SingleToSingle && v.Mode != sewing.BundleToBundle && v.Mode != sewing.BundleToSingle {
		return sewFail(422, "ERR_SEW_OP_MODE")
	}
	if len(v.RequiredParts) == 0 || len(v.RequiredParts) > 16 {
		return sewFail(422, "ERR_SEW_OP_PARTS")
	}
	if v.Mode == sewing.BundleToSingle && len(v.RequiredParts) < 2 || v.Mode != sewing.BundleToSingle && len(v.RequiredParts) != 1 {
		return sewFail(422, "ERR_SEW_OP_CARDINALITY")
	}
	seen := map[string]bool{}
	for _, p := range v.RequiredParts {
		if !sewFields(p) || seen[p] {
			return sewFail(422, "ERR_SEW_OP_PART_DUPLICATE")
		}
		seen[p] = true
	}
	seen = map[string]bool{}
	for _, p := range v.Predecessors {
		if !sewFields(p) || p == v.OperationCode || seen[p] {
			return sewFail(422, "ERR_SEW_OP_PREDECESSOR")
		}
		seen[p] = true
	}
	if len(v.Predecessors) > 16 {
		return sewFail(422, "ERR_SEW_OP_PREDECESSOR")
	}
	return nil
}

// GET catalog pulls actual 08A bundle drafts. UI labels are never created in JS.
func (s *EdgeServer) handleSewSimCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "GET required", 405)
		return
	}
	tenant, ok := s.sewTenant(w)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, e := s.docTransaction(ctx, tenant)
	if e != nil {
		sewReplyError(w, e)
		return
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, `SELECT d.id::text,d.intake_id,d.cut_no,d.marker_code,d.shade_band,
 EXISTS(SELECT 1 FROM cut_sewing_releases_v8 rel WHERE rel.tenant_id=d.tenant_id AND rel.lay_draft_id=d.id)
 FROM cut_lay_drafts_v8 d WHERE d.tenant_id=$1 ORDER BY d.created_at DESC LIMIT 80`, tenant)
	if e != nil {
		sewReplyError(w, e)
		return
	}
	lays := []map[string]any{}
	for rows.Next() {
		var id, order, marker, shade string
		var cut int
		var rel bool
		if e = rows.Scan(&id, &order, &cut, &marker, &shade, &rel); e != nil {
			break
		}
		lays = append(lays, map[string]any{"lay_id": id, "order": order, "cut": cut, "marker": marker, "shade": shade, "signed_cut_release": rel})
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		sewReplyError(w, e)
		return
	}
	lay := r.URL.Query().Get("lay_id")
	groups := []map[string]any{}
	if lay != "" {
		if !sewUUID(lay) {
			docJSON(w, 400, map[string]string{"error": "ERR_SEW_LAY_ID"})
			return
		}
		rs, err := tx.QueryContext(ctx, `SELECT b.marker_position,b.ply_group,b.part_code,b.label_code,b.color_code,b.size_code,b.ply_start,b.ply_end,b.pieces_per_garment,
 (SELECT count(*) FROM sew_sim_scans_v9 ss WHERE ss.tenant_id=b.tenant_id AND ss.lay_draft_id=b.lay_draft_id AND ss.label_code=b.label_code)
 FROM cut_bundle_drafts_v8 b WHERE b.tenant_id=$1 AND b.lay_draft_id=$2::uuid ORDER BY b.marker_position,b.ply_group,b.part_code LIMIT 800`, tenant, lay)
		if err != nil {
			sewReplyError(w, err)
			return
		}
		for rs.Next() {
			var pos, group, start, end, mult, scanCount int
			var part, label, color, size string
			if err = rs.Scan(&pos, &group, &part, &label, &color, &size, &start, &end, &mult, &scanCount); err != nil {
				break
			}
			groups = append(groups, map[string]any{"marker_position": pos, "ply_group": group, "part_code": part, "label_code": label, "color": color, "size": size, "ply_start": start, "ply_end": end, "pieces_per_garment": mult, "simulated_scan_count": scanCount})
		}
		if err == nil {
			err = rs.Err()
		}
		rs.Close()
		if err != nil {
			sewReplyError(w, err)
			return
		}
	}
	if e = tx.Commit(); e != nil {
		sewReplyError(w, e)
		return
	}
	docJSON(w, 200, map[string]any{"lays": lays, "bundles": groups, "evidence_source": sewProvenance, "production_credit": false})
}
func (s *EdgeServer) handleSewSimOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "POST" {
		http.Error(w, "GET or POST required", 405)
		return
	}
	tenant, ok := s.sewTenant(w)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	tx, e := s.docTransaction(ctx, tenant)
	if e != nil {
		sewReplyError(w, e)
		return
	}
	defer tx.Rollback()
	if r.Method == "POST" {
		var req sewOpRequest
		if !sewStrictJSON(w, r, &req) {
			return
		}
		if e = validSewOp(req); e != nil {
			sewReplyError(w, e)
			return
		}
		if e = sewTxLock(ctx, tx, tenant, "OPERATIONS"); e != nil {
			sewReplyError(w, e)
			return
		}
		if e = sewSimPlantGuard(ctx, tx, tenant, req.LayID, req.PlantID); e != nil {
			sewReplyError(w, e)
			return
		}
		var matched, predecessors int
		e = tx.QueryRowContext(ctx, `SELECT count(DISTINCT part_code)::int FROM cut_bundle_drafts_v8
 WHERE tenant_id=$1 AND lay_draft_id=$2::uuid AND part_code=ANY($3::text[])`, tenant, req.LayID, pgTextArray(req.RequiredParts)).Scan(&matched)
		if e != nil {
			sewReplyError(w, e)
			return
		}
		if matched != len(req.RequiredParts) {
			sewReplyError(w, sewFail(409, "ERR_SEW_OP_PART_NOT_IN_CUT"))
			return
		}
		e = tx.QueryRowContext(ctx, `SELECT count(*)::int FROM sew_sim_operations_v9 WHERE tenant_id=$1 AND lay_draft_id=$2::uuid AND plant_id=$3 AND operation_code=ANY($4::text[])`, tenant, req.LayID, req.PlantID, pgTextArray(req.Predecessors)).Scan(&predecessors)
		if e != nil {
			sewReplyError(w, e)
			return
		}
		if predecessors != len(req.Predecessors) {
			sewReplyError(w, sewFail(409, "ERR_SEW_OP_PREDECESSOR_UNDEFINED"))
			return
		}
		var id string
		e = tx.QueryRowContext(ctx, `INSERT INTO sew_sim_operations_v9(tenant_id,lay_draft_id,plant_id,operation_code,mode,required_parts,predecessors,machine_class,sam_seconds)
 VALUES($1,$2::uuid,$3,$4,$5,$6::text[],$7::text[],$8,$9) RETURNING id::text`, tenant, req.LayID, req.PlantID, req.OperationCode, string(req.Mode), pgTextArray(req.RequiredParts), pgTextArray(req.Predecessors), req.MachineClass, req.SAMSeconds).Scan(&id)
		if e != nil {
			sewReplyError(w, sewFail(409, "ERR_SEW_OP_ALREADY_EXISTS_OR_INVALID"))
			return
		}
		if e = tx.Commit(); e != nil {
			sewReplyError(w, e)
			return
		}
		docJSON(w, 201, map[string]any{"id": id, "evidence_state": "UNPUBLISHED_SIMULATION_CONTRACT", "production_credit": false})
		return
	}
	lay := r.URL.Query().Get("lay_id")
	plant := r.URL.Query().Get("plant_id")
	if !sewUUID(lay) || !sewFields(plant) {
		docJSON(w, 400, map[string]string{"error": "ERR_SEW_LAY_OR_PLANT"})
		return
	}
	rows, e := tx.QueryContext(ctx, `SELECT id::text,operation_code,mode,required_parts::text,predecessors::text,machine_class,sam_seconds::float8
 FROM sew_sim_operations_v9 WHERE tenant_id=$1 AND lay_draft_id=$2::uuid AND plant_id=$3 ORDER BY created_at`, tenant, lay, plant)
	if e != nil {
		sewReplyError(w, e)
		return
	}
	ops := []map[string]any{}
	for rows.Next() {
		var id, code, mode, machine string
		var parts, preds string
		var sam float64
		if e = rows.Scan(&id, &code, &mode, &parts, &preds, &machine, &sam); e != nil {
			break
		}
		ops = append(ops, map[string]any{"id": id, "operation_code": code, "mode": mode, "required_parts": parsePGTextArray(parts), "predecessors": parsePGTextArray(preds), "machine_class": machine, "sam_seconds": sam})
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		sewReplyError(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		sewReplyError(w, e)
		return
	}
	docJSON(w, 200, map[string]any{"operations": ops, "production_credit": false})
}

// PostgreSQL text arrays of alphanumeric operation/part IDs need no quoted values.
func pgTextArray(xs []string) string { return "{" + strings.Join(xs, ",") + "}" }
func parsePGTextArray(s string) []string {
	if s == "{}" || s == "" {
		return []string{}
	}
	return strings.Split(strings.Trim(s, "{}"), ",")
}

type sewScanInput struct {
	EventID   string `json:"event_id"`
	LayID     string `json:"lay_id"`
	PlantID   string `json:"plant_id"`
	LabelCode string `json:"label_code"`
	StationID string `json:"station_id"`
	WorkerID  string `json:"worker_id"`
	Reader    struct {
		Version     string   `json:"version"`
		Source      string   `json:"source"`
		ReaderID    string   `json:"reader_id"`
		ClickedAt   string   `json:"clicked_at"`
		AntennaPort int      `json:"antenna_port"`
		EPCHex      *string  `json:"epc_hex"`
		RSSIDBm     *float64 `json:"rssi_dbm"`
	} `json:"reader"`
}

func validSewScan(req sewScanInput) error {
	if !sewCommandID.MatchString(req.EventID) || !sewUUID(req.LayID) || !sewFields(req.PlantID, req.StationID, req.WorkerID) || !strings.HasPrefix(req.LabelCode, "CB08-") || len(req.LabelCode) != 29 {
		return sewFail(422, "ERR_SEW_SCAN_FIELDS")
	}
	if req.Reader.Version != "SEW_READER_V1" || req.Reader.Source != "UI_SIMULATED" || !sewFields(req.Reader.ReaderID) || req.Reader.AntennaPort < 1 || req.Reader.AntennaPort > 64 || req.Reader.EPCHex != nil || req.Reader.RSSIDBm != nil {
		return sewFail(422, "ERR_SEW_SIM_READER_PROVENANCE")
	}
	// UI clock is diagnostic only; received_at is the authoritative ingest clock.
	clicked, err := time.Parse(time.RFC3339Nano, req.Reader.ClickedAt)
	if err != nil || clicked.After(time.Now().Add(2*time.Minute)) {
		return sewFail(422, "ERR_SEW_SIM_CLIENT_CLOCK")
	}
	return nil
}
func (s *EdgeServer) handleSewSimScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "POST required", 405)
		return
	}
	tenant, ok := s.sewTenant(w)
	if !ok {
		return
	}
	var req sewScanInput
	if !sewStrictJSON(w, r, &req) {
		return
	}
	if e := validSewScan(req); e != nil {
		sewReplyError(w, e)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	tx, e := s.docTransaction(ctx, tenant)
	if e != nil {
		sewReplyError(w, e)
		return
	}
	defer tx.Rollback()
	if e = sewTxLock(ctx, tx, tenant, "COMMANDS"); e != nil {
		sewReplyError(w, e)
		return
	}
	if e = sewSimPlantGuard(ctx, tx, tenant, req.LayID, req.PlantID); e != nil {
		sewReplyError(w, e)
		return
	}
	hash := sewDigest(req)
	var id, oldHash string
	e = tx.QueryRowContext(ctx, `SELECT id::text,payload_sha256 FROM sew_sim_scans_v9 WHERE tenant_id=$1 AND event_id=$2`, tenant, req.EventID).Scan(&id, &oldHash)
	if e == nil {
		if oldHash != hash {
			sewReplyError(w, sewFail(409, "ERR_SEW_SCAN_REPLAY_CONFLICT"))
			return
		}
		if e = tx.Commit(); e != nil {
			sewReplyError(w, e)
			return
		}
		docJSON(w, 200, map[string]any{"scan_id": id, "replayed": true, "provenance": sewProvenance, "production_credit": false})
		return
	}
	if !errors.Is(e, sql.ErrNoRows) {
		sewReplyError(w, e)
		return
	}
	e = tx.QueryRowContext(ctx, `INSERT INTO sew_sim_scans_v9(tenant_id,event_id,payload_sha256,lay_draft_id,plant_id,label_code,station_id,worker_id,reader_id,operator_clicked_at,antenna_port)
 VALUES($1,$2,$3,$4::uuid,$5,$6,$7,$8,$9,$10,$11) RETURNING id::text`, tenant, req.EventID, hash, req.LayID, req.PlantID, req.LabelCode, req.StationID, req.WorkerID, req.Reader.ReaderID, req.Reader.ClickedAt, req.Reader.AntennaPort).Scan(&id)
	if e != nil {
		sewReplyError(w, sewFail(409, "ERR_SEW_SCAN_UNKNOWN_BUNDLE_OR_SCOPE"))
		return
	}
	if e = tx.Commit(); e != nil {
		sewReplyError(w, e)
		return
	}
	docJSON(w, 201, map[string]any{"scan_id": id, "replayed": false, "provenance": sewProvenance, "production_credit": false})
}

type sewSimCommand struct {
	CommandID      string `json:"command_id"`
	Action         string `json:"action"`
	OperationID    string `json:"operation_id,omitempty"`
	LayID          string `json:"lay_id,omitempty"`
	MarkerPosition int    `json:"marker_position,omitempty"`
	PlyGroup       int    `json:"ply_group,omitempty"`
	CycleID        string `json:"cycle_id,omitempty"`
	PlantID        string `json:"plant_id"`
	StationID      string `json:"station_id"`
	WorkerID       string `json:"worker_id"`
	MachineClass   string `json:"machine_class,omitempty"`
	Detail         string `json:"detail,omitempty"`
}

func validSewCommand(v sewSimCommand) error {
	if !sewCommandID.MatchString(v.CommandID) || !sewFields(v.PlantID, v.StationID, v.WorkerID) || len(v.Detail) > 800 {
		return sewFail(422, "ERR_SEW_COMMAND_FIELDS")
	}
	switch v.Action {
	case "CLAIM":
		if !sewUUID(v.OperationID) || !sewUUID(v.LayID) || v.MarkerPosition < 1 || v.PlyGroup < 1 || !sewFields(v.MachineClass) || v.CycleID != "" {
			return sewFail(422, "ERR_SEW_CLAIM_FIELDS")
		}
	case "PAUSE", "RESUME", "COMPLETE", "ISSUE":
		if !sewUUID(v.CycleID) || v.OperationID != "" || v.LayID != "" || v.MarkerPosition != 0 || v.PlyGroup != 0 || v.MachineClass != "" {
			return sewFail(422, "ERR_SEW_CYCLE_FIELDS")
		}
	default:
		return sewFail(422, "ERR_SEW_ACTION")
	}
	if v.Action == "ISSUE" && strings.TrimSpace(v.Detail) == "" {
		return sewFail(422, "ERR_SEW_ISSUE_DETAIL")
	}
	return nil
}

type sewGroupPart struct {
	label, part, shade, size string
	start, end, mult         int
}

// Deterministic, server-owned derivation. No caller-supplied piece count/serials.
func sewCheckGroup(mode sewing.ExecutionMode, parts []sewGroupPart, required []string, layID string, pos, group int) ([]string, error) {
	if len(parts) != len(required) || len(parts) == 0 {
		return nil, sewFail(409, "ERR_SEW_MISSING_OR_EXTRA_PART")
	}
	requiredSet := map[string]bool{}
	for _, p := range required {
		requiredSet[p] = true
	}
	seen := map[string]bool{}
	first := parts[0]
	if first.start <= 0 || first.end < first.start || first.end > 210 || pos < 1 || group < 1 {
		return nil, sewFail(409, "ERR_SEW_PLY_RANGE")
	}
	for _, p := range parts {
		if !requiredSet[p.part] || seen[p.part] || p.start != first.start || p.end != first.end || p.shade != first.shade || p.size != first.size || p.mult < 1 || p.mult > 8 {
			return nil, sewFail(409, "ERR_SEW_SHADE_SIZE_PLY_OR_PART")
		}
		seen[p.part] = true
	}
	if mode == sewing.SingleToSingle && (len(parts) != 1 || first.end != first.start || first.mult != 1) {
		return nil, sewFail(409, "ERR_SEW_SINGLE_TO_SINGLE_CARDINALITY")
	}
	if mode == sewing.BundleToSingle && len(parts) < 2 || mode != sewing.BundleToSingle && len(parts) != 1 {
		return nil, sewFail(409, "ERR_SEW_MODE_CARDINALITY")
	}
	serials := make([]string, 0, first.end-first.start+1)
	for p := first.start; p <= first.end; p++ {
		serials = append(serials, fmt.Sprintf("SIM-G-%s-%d-%d", layID, pos, p))
	}
	return serials, nil
}
func sewElapsed(from, to time.Time) (int64, error) {
	duration := to.Sub(from)
	if from.IsZero() || duration < 0 || duration > sewMaxActiveSegment {
		return 0, sewFail(409, "ERR_SEW_ACTIVE_SEGMENT_EXPIRED")
	}
	return duration.Milliseconds(), nil
}
func (s *EdgeServer) handleSewSimCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "POST required", 405)
		return
	}
	tenant, ok := s.sewTenant(w)
	if !ok {
		return
	}
	var v sewSimCommand
	if !sewStrictJSON(w, r, &v) {
		return
	}
	if e := validSewCommand(v); e != nil {
		sewReplyError(w, e)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	tx, e := s.docTransaction(ctx, tenant)
	if e != nil {
		sewReplyError(w, e)
		return
	}
	defer tx.Rollback()
	if e = sewTxLock(ctx, tx, tenant, "COMMANDS"); e != nil {
		sewReplyError(w, e)
		return
	}
	hash := sewDigest(v)
	var oldHash, oldCycle, oldAction string
	e = tx.QueryRowContext(ctx, `SELECT payload_sha256,COALESCE(cycle_id::text,''),action FROM sew_sim_events_v9 WHERE tenant_id=$1 AND command_id=$2`, tenant, v.CommandID).Scan(&oldHash, &oldCycle, &oldAction)
	if e == nil {
		if oldHash != hash {
			sewReplyError(w, sewFail(409, "ERR_SEW_COMMAND_REPLAY_CONFLICT"))
			return
		}
		if e = tx.Commit(); e != nil {
			sewReplyError(w, e)
			return
		}
		docJSON(w, 200, map[string]any{"cycle_id": oldCycle, "action": oldAction, "replayed": true, "evidence_state": sewCreditState, "production_credit": false})
		return
	}
	if !errors.Is(e, sql.ErrNoRows) {
		sewReplyError(w, e)
		return
	}
	var cycleID, detail string
	var creditCount int
	if v.Action == "CLAIM" {
		cycleID, detail, e = s.sewSimClaim(ctx, tx, tenant, v, hash)
	} else {
		cycleID = v.CycleID
		creditCount, detail, e = s.sewSimProgress(ctx, tx, tenant, v)
	}
	if e != nil {
		sewReplyError(w, e)
		return
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO sew_sim_events_v9(tenant_id,command_id,payload_sha256,cycle_id,plant_id,action,station_id,worker_id,detail)
 VALUES($1,$2,$3,$4::uuid,$5,$6,$7,$8,$9)`, tenant, v.CommandID, hash, cycleID, v.PlantID, v.Action, v.StationID, v.WorkerID, detail)
	if e != nil {
		sewReplyError(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		sewReplyError(w, e)
		return
	}
	docJSON(w, 201, map[string]any{"cycle_id": cycleID, "action": v.Action, "replayed": false, "simulated_units": creditCount, "evidence_state": sewCreditState, "production_credit": false, "detail": detail})
}
func (s *EdgeServer) sewSimClaim(ctx context.Context, tx *sql.Tx, tenant string, v sewSimCommand, hash string) (string, string, error) {
	var mode, code, machine, lay, plant string
	var requiredRaw, predRaw string
	e := tx.QueryRowContext(ctx, `SELECT mode,operation_code,machine_class,lay_draft_id::text,plant_id,required_parts::text,predecessors::text FROM sew_sim_operations_v9
 WHERE tenant_id=$1 AND id=$2::uuid`, tenant, v.OperationID).Scan(&mode, &code, &machine, &lay, &plant, &requiredRaw, &predRaw)
	if errors.Is(e, sql.ErrNoRows) {
		return "", "", sewFail(409, "ERR_SEW_OPERATION_NOT_FOUND")
	}
	if e != nil {
		return "", "", e
	}
	if lay != v.LayID || plant != v.PlantID || machine != v.MachineClass {
		return "", "", sewFail(409, "ERR_SEW_MACHINE_OR_ROUTE_MISMATCH")
	}
	if e = sewSimPlantGuard(ctx, tx, tenant, lay, plant); e != nil {
		return "", "", e
	}
	required := parsePGTextArray(requiredRaw)
	preds := parsePGTextArray(predRaw)
	for _, pred := range preds {
		var credited bool
		e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sew_sim_credits_v9 cr JOIN sew_sim_operations_v9 op ON cr.tenant_id=op.tenant_id AND cr.operation_id=op.id
 WHERE cr.tenant_id=$1 AND cr.lay_draft_id=$2::uuid AND cr.marker_position=$3 AND cr.ply_group=$4 AND op.operation_code=$5 AND op.plant_id=$6)`, tenant, v.LayID, v.MarkerPosition, v.PlyGroup, pred, v.PlantID).Scan(&credited)
		if e != nil {
			return "", "", e
		}
		if !credited {
			return "", "", sewFail(409, "ERR_SEW_PREDECESSOR_NOT_COMPLETED")
		}
	}
	// Load all actual parts for the exact source group, never accept caller-provided identities.
	rows, e := tx.QueryContext(ctx, `SELECT label_code,part_code,color_code,size_code,ply_start,ply_end,pieces_per_garment
 FROM cut_bundle_drafts_v8 WHERE tenant_id=$1 AND lay_draft_id=$2::uuid AND marker_position=$3 AND ply_group=$4
 AND part_code=ANY($5::text[]) ORDER BY part_code`, tenant, v.LayID, v.MarkerPosition, v.PlyGroup, pgTextArray(required))
	if e != nil {
		return "", "", e
	}
	parts := []sewGroupPart{}
	for rows.Next() {
		var p sewGroupPart
		if e = rows.Scan(&p.label, &p.part, &p.shade, &p.size, &p.start, &p.end, &p.mult); e != nil {
			break
		}
		parts = append(parts, p)
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		return "", "", e
	}
	_, e = sewCheckGroup(sewing.ExecutionMode(mode), parts, required, v.LayID, v.MarkerPosition, v.PlyGroup)
	if e != nil {
		return "", "", e
	}
	// Bind fresh, not-yet-consumed simulated scans of every required part to this claim.
	scanIDs := make(map[string]string)
	for _, p := range parts {
		var scan string
		e = tx.QueryRowContext(ctx, `SELECT s.id::text FROM sew_sim_scans_v9 s
 WHERE s.tenant_id=$1 AND s.lay_draft_id=$2::uuid AND s.label_code=$3 AND s.station_id=$4 AND s.worker_id=$5
 AND s.plant_id=$6 AND s.received_at>=now()-interval '15 minutes'
 AND NOT EXISTS(SELECT 1 FROM sew_sim_cycle_inputs_v9 ci WHERE ci.tenant_id=s.tenant_id AND ci.scan_id=s.id)
 ORDER BY s.received_at DESC LIMIT 1`, tenant, v.LayID, p.label, v.StationID, v.WorkerID, v.PlantID).Scan(&scan)
		if errors.Is(e, sql.ErrNoRows) {
			return "", "", sewFail(409, "ERR_SEW_SIM_SCAN_REQUIRED")
		}
		if e != nil {
			return "", "", e
		}
		scanIDs[p.part] = scan
	}
	var id string
	e = tx.QueryRowContext(ctx, `INSERT INTO sew_sim_cycles_v9(tenant_id,claim_id,claim_sha256,operation_id,lay_draft_id,plant_id,marker_position,ply_group,station_id,worker_id,machine_class,active_since)
 VALUES($1,$2,$3,$4::uuid,$5::uuid,$6,$7,$8,$9,$10,$11,now()) RETURNING id::text`, tenant, v.CommandID, hash, v.OperationID, v.LayID, v.PlantID, v.MarkerPosition, v.PlyGroup, v.StationID, v.WorkerID, v.MachineClass).Scan(&id)
	if e != nil {
		return "", "", sewFail(409, "ERR_SEW_ALREADY_CLAIMED_OR_COMPLETED")
	}
	for _, p := range parts {
		_, e = tx.ExecContext(ctx, `INSERT INTO sew_sim_cycle_inputs_v9(tenant_id,cycle_id,scan_id,label_code,part_code) VALUES($1,$2::uuid,$3::uuid,$4,$5)`, tenant, id, scanIDs[p.part], p.label, p.part)
		if e != nil {
			return "", "", sewFail(409, "ERR_SEW_SIM_SCAN_ALREADY_CONSUMED")
		}
	}
	return id, "Claimed with fresh UI-emulated scans; no physical RF observation", nil
}
func (s *EdgeServer) sewSimProgress(ctx context.Context, tx *sql.Tx, tenant string, v sewSimCommand) (stringCount int, detail string, err error) {
	var state, station, worker, opID, layID, plant string
	var pos, group int
	var activeSince, pausedSince sql.NullTime
	var activeMS, pausedMS int64
	e := tx.QueryRowContext(ctx, `SELECT state,station_id,worker_id,operation_id::text,lay_draft_id::text,plant_id,marker_position,ply_group,active_since,paused_since,active_ms,paused_ms
 FROM sew_sim_cycles_v9 WHERE tenant_id=$1 AND id=$2::uuid FOR UPDATE`, tenant, v.CycleID).Scan(&state, &station, &worker, &opID, &layID, &plant, &pos, &group, &activeSince, &pausedSince, &activeMS, &pausedMS)
	if errors.Is(e, sql.ErrNoRows) {
		return 0, "", sewFail(404, "ERR_SEW_CYCLE_UNKNOWN")
	}
	if e != nil {
		return 0, "", e
	}
	if plant != v.PlantID || station != v.StationID || worker != v.WorkerID {
		return 0, "", sewFail(403, "ERR_SEW_CYCLE_ACTOR_SCOPE")
	}
	now := time.Now().UTC()
	switch v.Action {
	case "ISSUE":
		if state == "COMPLETED" || state == "REVIEW_REQUIRED" {
			return 0, "", sewFail(409, "ERR_SEW_CYCLE_NOT_OPEN")
		}
		// A reported anomaly is an explicit hold, not merely a log message.
		// In-progress intervals are stopped, with no operation credit granted.
		if state == "CLAIMED" && activeSince.Valid {
			if elapsed, ex := sewElapsed(activeSince.Time, now); ex == nil {
				activeMS += elapsed
			}
		}
		_, e = tx.ExecContext(ctx, `UPDATE sew_sim_cycles_v9 SET state='REVIEW_REQUIRED',active_since=NULL,paused_since=NULL,active_ms=$3 WHERE tenant_id=$1 AND id=$2::uuid`, tenant, v.CycleID, activeMS)
		return 0, "HOLD: issue requires independent review; zero simulated output: " + v.Detail, e
	case "PAUSE":
		if state != "CLAIMED" || !activeSince.Valid {
			return 0, "", sewFail(409, "ERR_SEW_NOT_ACTIVE")
		}
		ms, e := sewElapsed(activeSince.Time, now)
		if e != nil {
			_, er := tx.ExecContext(ctx, `UPDATE sew_sim_cycles_v9 SET state='REVIEW_REQUIRED',active_since=NULL WHERE tenant_id=$1 AND id=$2::uuid`, tenant, v.CycleID)
			return 0, "HOLD: active segment exceeded four hours; zero credit", er
		}
		_, e = tx.ExecContext(ctx, `UPDATE sew_sim_cycles_v9 SET state='PAUSED',active_since=NULL,paused_since=$3,active_ms=active_ms+$4 WHERE tenant_id=$1 AND id=$2::uuid`, tenant, v.CycleID, now, ms)
		return 0, "Paused; UI-measured active time (not machine telemetry)", e
	case "RESUME":
		if state != "PAUSED" || !pausedSince.Valid {
			return 0, "", sewFail(409, "ERR_SEW_NOT_PAUSED")
		}
		pauseMS := now.Sub(pausedSince.Time).Milliseconds()
		if pauseMS < 0 {
			return 0, "", sewFail(409, "ERR_SEW_CLOCK")
		}
		_, e = tx.ExecContext(ctx, `UPDATE sew_sim_cycles_v9 SET state='CLAIMED',paused_since=NULL,active_since=$3,paused_ms=paused_ms+$4 WHERE tenant_id=$1 AND id=$2::uuid`, tenant, v.CycleID, now, pauseMS)
		return 0, "Resumed; paused time excluded", e
	case "COMPLETE":
		if state != "CLAIMED" || !activeSince.Valid {
			return 0, "", sewFail(409, "ERR_SEW_NOT_ACTIVE")
		}
		ms, e := sewElapsed(activeSince.Time, now)
		if e != nil {
			_, er := tx.ExecContext(ctx, `UPDATE sew_sim_cycles_v9 SET state='REVIEW_REQUIRED',active_since=NULL WHERE tenant_id=$1 AND id=$2::uuid`, tenant, v.CycleID)
			return 0, "HOLD: active segment exceeded four hours; zero credit", er
		}
		total := activeMS + ms
		if total < 1000 {
			return 0, "", sewFail(409, "ERR_SEW_TOO_SHORT_FOR_UI_CONFIRMATION")
		}
		var mode, requiredRaw string
		e = tx.QueryRowContext(ctx, `SELECT mode,required_parts::text FROM sew_sim_operations_v9 WHERE tenant_id=$1 AND id=$2::uuid`, tenant, opID).Scan(&mode, &requiredRaw)
		if e != nil {
			return 0, "", e
		}
		required := parsePGTextArray(requiredRaw)
		rows, e := tx.QueryContext(ctx, `SELECT b.label_code,b.part_code,b.color_code,b.size_code,b.ply_start,b.ply_end,b.pieces_per_garment
 FROM sew_sim_cycle_inputs_v9 i JOIN cut_bundle_drafts_v8 b ON b.tenant_id=i.tenant_id AND b.label_code=i.label_code
 WHERE i.tenant_id=$1 AND i.cycle_id=$2::uuid AND b.lay_draft_id=$3::uuid ORDER BY b.part_code`, tenant, v.CycleID, layID)
		if e != nil {
			return 0, "", e
		}
		parts := []sewGroupPart{}
		for rows.Next() {
			var p sewGroupPart
			if e = rows.Scan(&p.label, &p.part, &p.shade, &p.size, &p.start, &p.end, &p.mult); e != nil {
				break
			}
			parts = append(parts, p)
		}
		if e == nil {
			e = rows.Err()
		}
		rows.Close()
		if e != nil {
			return 0, "", e
		}
		serials, e := sewCheckGroup(sewing.ExecutionMode(mode), parts, required, layID, pos, group)
		if e != nil {
			return 0, "", e
		}
		_, e = tx.ExecContext(ctx, `UPDATE sew_sim_cycles_v9 SET state='COMPLETED',active_since=NULL,active_ms=$3,finished_at=$4 WHERE tenant_id=$1 AND id=$2::uuid`, tenant, v.CycleID, total, now)
		if e != nil {
			return 0, "", e
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO sew_sim_credits_v9(tenant_id,cycle_id,operation_id,lay_draft_id,marker_position,ply_group,garment_count,serials,active_ms)
 VALUES($1,$2::uuid,$3::uuid,$4::uuid,$5,$6,$7,$8::text[],$9)`, tenant, v.CycleID, opID, layID, pos, group, len(serials), pgTextArray(serials), total)
		if e != nil {
			return 0, "", sewFail(409, "ERR_SEW_DUPLICATE_SIMULATED_CREDIT")
		}
		return len(serials), "Simulated completion only; not earned output or payroll", nil
	}
	return 0, "", sewFail(422, "ERR_SEW_ACTION")
}
func (s *EdgeServer) handleSewSimStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "GET required", 405)
		return
	}
	tenant, ok := s.sewTenant(w)
	if !ok {
		return
	}
	lay := r.URL.Query().Get("lay_id")
	plant := r.URL.Query().Get("plant_id")
	if !sewUUID(lay) || !sewFields(plant) {
		docJSON(w, 400, map[string]string{"error": "ERR_SEW_LAY_OR_PLANT"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, e := s.docTransaction(ctx, tenant)
	if e != nil {
		sewReplyError(w, e)
		return
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, `SELECT c.id::text,o.operation_code,c.marker_position,c.ply_group,c.state,c.station_id,c.worker_id,c.active_ms,c.paused_ms,
 COALESCE(cr.garment_count,0),c.claimed_at
 FROM sew_sim_cycles_v9 c JOIN sew_sim_operations_v9 o ON o.tenant_id=c.tenant_id AND o.id=c.operation_id
 LEFT JOIN sew_sim_credits_v9 cr ON cr.tenant_id=c.tenant_id AND cr.cycle_id=c.id
 WHERE c.tenant_id=$1 AND c.lay_draft_id=$2::uuid AND c.plant_id=$3 ORDER BY c.claimed_at DESC LIMIT 250`, tenant, lay, plant)
	if e != nil {
		sewReplyError(w, e)
		return
	}
	items := []map[string]any{}
	for rows.Next() {
		var id, code, state, station, worker string
		var pos, group, units int
		var active, paused int64
		var claimed time.Time
		if e = rows.Scan(&id, &code, &pos, &group, &state, &station, &worker, &active, &paused, &units, &claimed); e != nil {
			break
		}
		items = append(items, map[string]any{"cycle_id": id, "operation_code": code, "marker_position": pos, "ply_group": group, "state": state, "station_id": station, "worker_id": worker, "active_ms": active, "paused_ms": paused, "simulated_units": units, "claimed_at": claimed})
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		sewReplyError(w, e)
		return
	}
	var total int
	e = tx.QueryRowContext(ctx, `SELECT COALESCE(sum(cr.garment_count),0)::int FROM sew_sim_credits_v9 cr JOIN sew_sim_cycles_v9 c ON c.tenant_id=cr.tenant_id AND c.id=cr.cycle_id WHERE cr.tenant_id=$1 AND cr.lay_draft_id=$2::uuid AND c.plant_id=$3`, tenant, lay, plant).Scan(&total)
	if e != nil {
		sewReplyError(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		sewReplyError(w, e)
		return
	}
	docJSON(w, 200, map[string]any{"cycles": items, "total_simulated_operation_units": total, "production_credit": 0, "evidence_state": sewCreditState})
}
func serveSewSimulation(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("ENV") != "development" {
		http.Error(w, "Simulation unavailable outside development", 403)
		return
	}
	data, e := web.GetSewSimulationHTML()
	if e != nil {
		http.Error(w, "Sewing simulation not installed", 404)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

// Helpers retained as unit-testable contract functions.
func sortSimCodes(xs []string) []string {
	out := append([]string{}, xs...)
	sort.Strings(out)
	return out
}
