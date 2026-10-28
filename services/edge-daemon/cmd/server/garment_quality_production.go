// Author: Sunil Jain, 2026-10-28.
// F3.4: source-backed, signed per-garment QC, repair, finishing and QUALITY-ONLY
// release. No inferred sewing output, AQL sampling, carton or shipping authority.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/attestation"
)

const garmentQCReadChallenge = "GARMENT_QUALITY_READ_V1"
const garmentQCEvidence = "SIGNED_GARMENT_QC_HUMAN_ATTESTATION_NOT_SHIPPING_AUTHORITY"

type garmentQCCommand struct {
	CommandID      string           `json:"command_id"`
	PlantID        string           `json:"plant_id"`
	GarmentID      string           `json:"garment_id"`
	SpecID         string           `json:"spec_id"`
	Action         string           `json:"action"`
	DefectCode     string           `json:"defect_code"`
	MeasurementsMM map[string]int64 `json:"measurements_mm"`
	Note           string           `json:"note"`
}
type garmentQCRequest struct {
	Command garmentQCCommand `json:"command"`
}
type garmentQCAPI struct {
	wip     *wipAPI
	enabled bool
	reason  string
}

func newGarmentQCAPI(s *EdgeServer) *garmentQCAPI {
	p := &garmentQCAPI{reason: "ERR_F34_DISABLED"}
	if os.Getenv("AGIPAREL_GARMENT_QUALITY_ENABLED") != "true" {
		return p
	}
	p.wip = newWIPAPI(s)
	if !p.wip.enabled {
		p.reason = "ERR_F34_SIGNED_WIP_PREREQUISITE"
		return p
	}
	p.enabled = true
	return p
}
func (p *garmentQCAPI) ready(w http.ResponseWriter) bool {
	if p == nil || !p.enabled {
		code := "ERR_F34_DISABLED"
		if p != nil {
			code = p.reason
		}
		docJSON(w, 503, map[string]string{"error": code})
		return false
	}
	return true
}
func garmentQCAction(action string) (permission, role string, ok bool) {
	switch action {
	case "INSPECT_PASS", "INSPECT_HOLD", "REINSPECT_PASS", "REINSPECT_HOLD":
		return "quality.inspect", "quality_inspector", true
	case "REPAIR_COMPLETE":
		return "quality.repair", "quality_repairer", true
	case "FINISHING_COMPLETE":
		return "quality.finish", "finishing_operator", true
	case "QUALITY_RELEASE":
		return "quality.release", "quality_manager", true
	default:
		return "", "", false
	}
}
func garmentQCValid(c garmentQCCommand) bool {
	if !sewCommandID.MatchString(c.CommandID) || !sewFields(c.PlantID) || !sewUUID(c.GarmentID) || !sewUUID(c.SpecID) ||
		len(strings.TrimSpace(c.Note)) < 12 || len(c.Note) > 1000 || strings.ContainsRune(c.Note, 0) {
		return false
	}
	if _, _, ok := garmentQCAction(c.Action); !ok {
		return false
	}
	hold := c.Action == "INSPECT_HOLD" || c.Action == "REINSPECT_HOLD"
	pass := c.Action == "INSPECT_PASS" || c.Action == "REINSPECT_PASS"
	if hold != (c.DefectCode != "") || (hold && !sewFields(c.DefectCode)) {
		return false
	}
	if pass != (len(c.MeasurementsMM) > 0) {
		return false
	}
	for k, v := range c.MeasurementsMM {
		if !sewFields(k) || v < 0 || v > 999999999999 {
			return false
		}
	}
	return true
}
func garmentQCChallenge(c garmentQCCommand) string { return "GARMENT_QC_V1:" + sewDigest(c) }
func (p *garmentQCAPI) challenge(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var req garmentQCRequest
	if !sewProdStrictJSON(w, r, &req) {
		return
	}
	if !garmentQCValid(req.Command) {
		docJSON(w, 422, map[string]string{"error": "ERR_F34_FIELDS"})
		return
	}
	permission, role, _ := garmentQCAction(req.Command.Action)
	docJSON(w, 200, map[string]any{"challenge": garmentQCChallenge(req.Command), "permission": permission, "role": role,
		"auth": "webauthn_or_fido2", "inspection_basis": "EVERY_GARMENT_NOT_AQL_SAMPLE", "shipping_release": false})
}
func (p *garmentQCAPI) event(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var req garmentQCRequest
	if !sewProdStrictJSON(w, r, &req) {
		return
	}
	c := req.Command
	if !garmentQCValid(c) {
		docJSON(w, 422, map[string]string{"error": "ERR_F34_FIELDS"})
		return
	}
	raw, e := bearer(r)
	if e != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_F34_BEARER_REQUIRED"})
		return
	}
	permission, role, _ := garmentQCAction(c.Action)
	a, e := p.wip.gen.sewing.verifier.Verify(raw, permission, garmentQCChallenge(c), role, true)
	if e != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_F34_SIGNED_ASSERTION"})
		return
	}
	if a.PlantID != c.PlantID || !validChapterActor(a) || len(a.JWTID) == 0 || len(a.JWTID) > 255 {
		docJSON(w, 403, map[string]string{"error": "ERR_F34_SCOPE"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	id, seq, replay, e := p.persist(ctx, c, a, raw, permission)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	status := 201
	if replay {
		status = 200
	}
	docJSON(w, status, map[string]any{"id": id, "event_no": seq, "replayed": replay, "action": c.Action,
		"quality_released": c.Action == "QUALITY_RELEASE", "shipping_release": false, "packing_release": false,
		"sewing_operation_credit": false, "evidence": garmentQCEvidence})
}
func (p *garmentQCAPI) persist(ctx context.Context, c garmentQCCommand, a attestation.Claims, raw, permission string) (id string, seq int64, replay bool, err error) {
	tx, e := p.wip.gen.sewing.server.docTransaction(ctx, a.TenantID)
	if e != nil {
		return "", 0, false, e
	}
	defer tx.Rollback()
	if e = workforceChapterRequire(ctx, tx, a, permission); e != nil {
		return "", 0, false, e
	}
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1),hashtext('F34:GARMENT:'||$2))`, a.TenantID, c.GarmentID); e != nil {
		return "", 0, false, e
	}
	hash := sewDigest(c)
	var oldHash, oldActor string
	e = tx.QueryRowContext(ctx, `SELECT id::text,event_no,command_sha256,actor_subject
       FROM qc_garment_events_v21 WHERE tenant_id=$1 AND command_id=$2`, a.TenantID, c.CommandID).Scan(&id, &seq, &oldHash, &oldActor)
	if e == nil {
		if oldHash != hash || oldActor != a.Subject {
			return "", 0, false, sewFail(409, "ERR_F34_COMMAND_REUSE")
		}
		return id, seq, true, tx.Commit()
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return "", 0, false, e
	}
	// Early checks aid diagnostics; DB trigger independently enforces all rules.
	var exists bool
	e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM gen_garments_v18 g
      JOIN qc_garment_specs_v21 s ON s.tenant_id=g.tenant_id AND s.lay_draft_id=g.lay_draft_id
      WHERE g.tenant_id=$1 AND g.id=$2::uuid AND g.plant_id=$3
      AND s.id=$4::uuid AND s.plant_id=$3 AND s.enabled)`, a.TenantID, c.GarmentID, c.PlantID, c.SpecID).Scan(&exists)
	if e != nil {
		return "", 0, false, e
	}
	if !exists {
		return "", 0, false, sewFail(409, "ERR_F34_APPROVED_SPEC_REQUIRED")
	}
	measurements, e := json.Marshal(c.MeasurementsMM)
	if e != nil {
		return "", 0, false, e
	}
	if len(c.MeasurementsMM) == 0 {
		measurements = []byte("{}")
	}
	e = tx.QueryRowContext(ctx, `INSERT INTO qc_garment_events_v21
      (tenant_id,plant_id,garment_id,spec_id,event_no,action,defect_code,measurements_mm,note,
       command_id,command_sha256,actor_subject,proof_id,assertion_sha256)
       VALUES ($1,$2,$3::uuid,$4::uuid,1,$5,NULLIF($6,''),$7::jsonb,$8,$9,$10,$11,$12,$13)
       RETURNING id::text,event_no`, a.TenantID, c.PlantID, c.GarmentID, c.SpecID, c.Action, c.DefectCode, string(measurements), c.Note, c.CommandID, hash, a.Subject, a.JWTID, sewProdHashString(raw)).Scan(&id, &seq)
	if e != nil {
		return "", 0, false, sewFail(409, "ERR_F34_GARMENT_QC_CONFLICT")
	}
	return id, seq, false, tx.Commit()
}
func (p *garmentQCAPI) readActor(w http.ResponseWriter, r *http.Request) (attestation.Claims, bool) {
	token, e := bearer(r)
	if e != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_F34_READ_BEARER"})
		return attestation.Claims{}, false
	}
	a, e := p.wip.gen.sewing.verifier.Verify(token, "quality.read", garmentQCReadChallenge, "quality_inspector", true)
	if e != nil || !validChapterActor(a) {
		docJSON(w, 401, map[string]string{"error": "ERR_F34_READ_ASSERTION"})
		return attestation.Claims{}, false
	}
	return a, true
}
func (p *garmentQCAPI) history(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	if !p.ready(w) {
		return
	}
	q := r.URL.Query()
	for k := range q {
		if k != "garment_id" && k != "cursor" && k != "limit" {
			docJSON(w, 422, map[string]string{"error": "ERR_F34_HISTORY_QUERY"})
			return
		}
	}
	if len(q["garment_id"]) != 1 || !sewUUID(q.Get("garment_id")) || len(q["cursor"]) > 1 || len(q["limit"]) > 1 {
		docJSON(w, 422, map[string]string{"error": "ERR_F34_HISTORY_QUERY"})
		return
	}
	limit, e := genPageLimit(q.Get("limit"))
	if e != nil {
		sewProdReply(w, e)
		return
	}
	cursor := int64(9223372036854775807)
	if q.Get("cursor") != "" {
		cursor, e = strconv.ParseInt(q.Get("cursor"), 10, 64)
		if e != nil || cursor < 1 {
			docJSON(w, 422, map[string]string{"error": "ERR_F34_HISTORY_CURSOR"})
			return
		}
	}
	a, ok := p.readActor(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	tx, e := p.wip.gen.sewing.server.docTransaction(ctx, a.TenantID)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	defer tx.Rollback()
	if e = workforceChapterRequire(ctx, tx, a, "quality.read"); e != nil {
		sewProdReply(w, e)
		return
	}
	rows, e := tx.QueryContext(ctx, `SELECT e.event_no,e.action,e.defect_code,e.actor_subject,e.recorded_at,e.spec_id::text
      FROM qc_garment_events_v21 e JOIN gen_garments_v18 g ON g.tenant_id=e.tenant_id AND g.id=e.garment_id
      WHERE e.tenant_id=$1 AND e.plant_id=$2 AND g.plant_id=$2 AND e.garment_id=$3::uuid
      AND e.event_no<$4 ORDER BY e.event_no DESC LIMIT $5`, a.TenantID, a.PlantID, q.Get("garment_id"), cursor, limit+1)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	type event struct {
		Number int64     `json:"event_no"`
		Action string    `json:"action"`
		Defect *string   `json:"defect_code"`
		Actor  string    `json:"actor_subject"`
		At     time.Time `json:"recorded_at"`
		Spec   string    `json:"spec_id"`
	}
	items := []event{}
	for rows.Next() {
		var v event
		var defect sql.NullString
		if e = rows.Scan(&v.Number, &v.Action, &defect, &v.Actor, &v.At, &v.Spec); e != nil {
			break
		}
		if defect.Valid {
			v.Defect = &defect.String
		}
		items = append(items, v)
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		sewProdReply(w, e)
		return
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	var next any
	if more && len(items) > 0 {
		next = strconv.FormatInt(items[len(items)-1].Number, 10)
	}
	if e = tx.Commit(); e != nil {
		sewProdReply(w, e)
		return
	}
	docJSON(w, 200, map[string]any{"garment_id": q.Get("garment_id"), "plant_id": a.PlantID, "events": items, "next_cursor": next, "has_more": more, "evidence": garmentQCEvidence, "shipping_release": false})
}
func (p *garmentQCAPI) specification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	if !p.ready(w) {
		return
	}
	q := r.URL.Query()
	if len(q) != 1 || len(q["garment_id"]) != 1 || !sewUUID(q.Get("garment_id")) {
		docJSON(w, 422, map[string]string{"error": "ERR_F34_SPEC_QUERY"})
		return
	}
	a, ok := p.readActor(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	tx, e := p.wip.gen.sewing.server.docTransaction(ctx, a.TenantID)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	defer tx.Rollback()
	if e = workforceChapterRequire(ctx, tx, a, "quality.read"); e != nil {
		sewProdReply(w, e)
		return
	}
	var specID, rev, docID, docHash, station, size string
	e = tx.QueryRowContext(ctx, `SELECT s.id::text,s.revision,s.source_document_id,s.source_sha256,s.finish_station,p.size_code
      FROM gen_garments_v18 g JOIN cut_lay_positions_v8 p ON p.tenant_id=g.tenant_id
       AND p.lay_draft_id=g.lay_draft_id AND p.marker_position=g.marker_position
      JOIN qc_garment_specs_v21 s ON s.tenant_id=g.tenant_id AND s.lay_draft_id=g.lay_draft_id
       AND s.plant_id=g.plant_id AND s.enabled
      WHERE g.tenant_id=$1 AND g.plant_id=$2 AND g.id=$3::uuid
      ORDER BY s.approved_at DESC,s.id DESC LIMIT 1`, a.TenantID, a.PlantID, q.Get("garment_id")).Scan(&specID, &rev, &docID, &docHash, &station, &size)
	if errors.Is(e, sql.ErrNoRows) {
		docJSON(w, 404, map[string]string{"error": "ERR_F34_NO_APPROVED_SPEC"})
		return
	}
	if e != nil {
		sewProdReply(w, e)
		return
	}
	type measure struct {
		Code      string `json:"code"`
		NominalMM int64  `json:"nominal_mm"`
		MinusMM   int64  `json:"minus_mm"`
		PlusMM    int64  `json:"plus_mm"`
	}
	measures := []measure{}
	rows, e := tx.QueryContext(ctx, `SELECT measure_code,nominal_mm,lower_tolerance_mm,upper_tolerance_mm
       FROM qc_garment_measure_rules_v21 WHERE tenant_id=$1 AND spec_id=$2::uuid AND size_code=$3
       ORDER BY measure_code`, a.TenantID, specID, size)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	for rows.Next() {
		var m measure
		e = rows.Scan(&m.Code, &m.NominalMM, &m.MinusMM, &m.PlusMM)
		if e != nil {
			break
		}
		measures = append(measures, m)
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		sewProdReply(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		sewProdReply(w, e)
		return
	}
	docJSON(w, 200, map[string]any{"garment_id": q.Get("garment_id"), "spec_id": specID, "revision": rev,
		"source_document_id": docID, "source_sha256": docHash, "size_code": size, "finish_station": station,
		"measurement_rules": measures, "inspection_basis": "EVERY_GARMENT_NOT_AQL_SAMPLE", "release_scope": "QUALITY_ONLY_NO_PACKING_OR_SHIPPING_AUTHORITY"})
}
func registerGarmentQCRoutes(mux *http.ServeMux, s *EdgeServer) {
	p := newGarmentQCAPI(s)
	mux.HandleFunc("/api/v1/quality/garments/challenge", p.challenge)
	mux.HandleFunc("/api/v1/quality/garments/events", p.event)
	mux.HandleFunc("/api/v1/quality/garments/history", p.history)
	mux.HandleFunc("/api/v1/quality/garments/specification", p.specification)
}
