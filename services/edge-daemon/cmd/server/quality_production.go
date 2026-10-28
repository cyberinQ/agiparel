// Author: Sunil Jain, 2026-10-28.
// Chapter 07: challenge-bound, tenant/plant-scoped quality evidence for real
// sewing completion GROUPS. A reviewed group is NOT a released garment, QC
// certificate, warehouse receipt, payroll credit, or shipping authorization.
package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/attestation"
)

const qcProdEvidence = "SEWING_COMPLETION_GROUP_QC_EVIDENCE_NO_GARMENT_OR_SHIPPING_RELEASE"
const qcProdReadChallenge = "QUALITY_PROD_CATALOG_V1"

type qcProdCommand struct {
	CommandID      string `json:"command_id"`
	CompletionID   string `json:"completion_id"`
	PlantID        string `json:"plant_id"`
	StationID      string `json:"station_id"`
	Action         string `json:"action"`
	InspectedUnits int    `json:"inspected_units"`
	DefectiveUnits int    `json:"defective_units"`
	DefectCode     string `json:"defect_code"`
	Note           string `json:"note"`
}
type qcProdRequest struct {
	Command qcProdCommand `json:"command"`
}
type qcProdAPI struct {
	sewing  *sewProdAPI
	enabled bool
	reason  string
}

func newQCProdAPI(s *EdgeServer) *qcProdAPI {
	q := &qcProdAPI{reason: "ERR_QC_PROD_DISABLED"}
	if os.Getenv("AGIPAREL_QUALITY_PRODUCTION_ENABLED") != "true" {
		return q
	}
	p := newSewProdAPI(s)
	if !p.enabled {
		q.reason = "ERR_QC_PROD_SEWING_PREREQUISITE"
		return q
	}
	q.sewing = p
	q.enabled = true
	return q
}
func (q *qcProdAPI) ready(w http.ResponseWriter) bool {
	if !q.enabled {
		docJSON(w, 503, map[string]string{"error": q.reason})
		return false
	}
	return true
}
func qcProdRole(action string) (string, string) {
	if action == "REPAIR_COMPLETE" {
		return "quality.repair", "quality_repairer"
	}
	return "quality.inspect", "quality_inspector"
}
func qcProdValid(c qcProdCommand) error {
	if !sewCommandID.MatchString(c.CommandID) || !sewUUID(c.CompletionID) ||
		!sewFields(c.PlantID, c.StationID) || len(c.Note) > 1000 || strings.ContainsRune(c.Note, 0) ||
		c.InspectedUnits < 0 || c.DefectiveUnits < 0 {
		return sewProdInvalid("ERR_QC_PROD_FIELDS")
	}
	switch c.Action {
	case "INSPECT_PASS", "REINSPECT_PASS":
		if c.InspectedUnits < 1 || c.DefectiveUnits != 0 || c.DefectCode != "" {
			return sewProdInvalid("ERR_QC_PROD_PASS_COUNTS")
		}
	case "INSPECT_HOLD", "REINSPECT_HOLD":
		if c.InspectedUnits < 1 || c.DefectiveUnits < 1 || c.DefectiveUnits > c.InspectedUnits || !sewCode.MatchString(c.DefectCode) {
			return sewProdInvalid("ERR_QC_PROD_HOLD_COUNTS")
		}
	case "REPAIR_COMPLETE":
		if c.InspectedUnits != 0 || c.DefectiveUnits != 0 || c.DefectCode != "" || strings.TrimSpace(c.Note) == "" {
			return sewProdInvalid("ERR_QC_PROD_REPAIR_NOTE_REQUIRED")
		}
	default:
		return sewProdInvalid("ERR_QC_PROD_ACTION")
	}
	return nil
}
func qcProdHash(c qcProdCommand) string      { return sewDigest(c) }
func qcProdChallenge(c qcProdCommand) string { return "QUALITY_PROD_V1:" + qcProdHash(c) }
func (q *qcProdAPI) challenge(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !q.ready(w) {
		return
	}
	var v qcProdRequest
	if !sewProdStrictJSON(w, r, &v) {
		return
	}
	if e := qcProdValid(v.Command); e != nil {
		sewProdReply(w, e)
		return
	}
	action, role := qcProdRole(v.Command.Action)
	docJSON(w, 200, map[string]any{"challenge": qcProdChallenge(v.Command), "action": action, "required_role": role, "required_auth": "webauthn_or_fido2", "scope": "sewing_completion_group"})
}
func (q *qcProdAPI) event(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !q.ready(w) {
		return
	}
	var v qcProdRequest
	if !sewProdStrictJSON(w, r, &v) {
		return
	}
	c := v.Command
	if e := qcProdValid(c); e != nil {
		sewProdReply(w, e)
		return
	}
	token, e := bearer(r)
	if e != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_QC_PROD_BEARER_REQUIRED"})
		return
	}
	action, role := qcProdRole(c.Action)
	actor, e := q.sewing.verifier.Verify(token, action, qcProdChallenge(c), role, true)
	if e != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_QC_PROD_ASSERTION"})
		return
	}
	if actor.PlantID != c.PlantID || !sewFields(actor.TenantID) || len(actor.Subject) > 128 || len(actor.JWTID) > 255 {
		docJSON(w, 403, map[string]string{"error": "ERR_QC_PROD_ACTOR_SCOPE"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	id, state, replay, revision, e := q.commit(ctx, c, actor, token)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	status := 201
	if replay {
		status = 200
	}
	docJSON(w, status, map[string]any{"event_id": id, "state": state, "revision": revision, "replayed": replay, "evidence": qcProdEvidence, "garment_release": false, "packing_release": false, "shipping_release": false})
}
func (q *qcProdAPI) commit(ctx context.Context, c qcProdCommand, actor attestation.Claims, token string) (string, string, bool, int, error) {
	tx, e := q.sewing.server.docTransaction(ctx, actor.TenantID)
	if e != nil {
		return "", "", false, 0, e
	}
	defer tx.Rollback()
	permission, _ := qcProdRole(c.Action)
	if e = workforceChapterRequire(ctx, tx, actor, permission); e != nil {
		return "", "", false, 0, e
	}
	// Identical lock with SQL trigger protects concurrent writers and retries.
	_, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1),hashtext('QC11:'||$2))`, actor.TenantID, c.CompletionID)
	if e != nil {
		return "", "", false, 0, e
	}
	hash := qcProdHash(c)
	var id, state, stored, subject string
	var revision int
	e = tx.QueryRowContext(ctx, `SELECT id::text,next_state,payload_sha256,actor_subject,revision FROM qc_prod_group_events_v11 WHERE tenant_id=$1 AND command_id=$2`, actor.TenantID, c.CommandID).Scan(&id, &state, &stored, &subject, &revision)
	if e == nil {
		if hash != stored || subject != actor.Subject {
			return "", "", false, 0, sewFail(409, "ERR_QC_PROD_COMMAND_REUSE")
		}
		return id, state, true, revision, tx.Commit()
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return "", "", false, 0, e
	}
	e = tx.QueryRowContext(ctx, `INSERT INTO qc_prod_group_events_v11
  (tenant_id,command_id,payload_sha256,completion_id,plant_id,station_id,action,inspected_units,defective_units,defect_code,note,actor_subject,proof_id,assertion_sha256)
  VALUES($1,$2,$3,$4::uuid,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
  RETURNING id::text,next_state,revision`, actor.TenantID, c.CommandID, hash, c.CompletionID, c.PlantID, c.StationID, c.Action, c.InspectedUnits, c.DefectiveUnits, c.DefectCode, c.Note, actor.Subject, actor.JWTID, sewProdHashString(token)).Scan(&id, &state, &revision)
	if e != nil {
		return "", "", false, 0, sewFail(409, "ERR_QC_PROD_SOURCE_OR_TRANSITION_OR_EVIDENCE")
	}
	return id, state, false, revision, tx.Commit()
}
func qcProdCursor(at time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(at.UTC().Format(time.RFC3339Nano) + "|" + id))
}
func qcProdParseCursor(raw string) (time.Time, string, error) {
	if raw == "" {
		return time.Time{}, "", nil
	}
	if len(raw) > 192 {
		return time.Time{}, "", sewProdInvalid("ERR_QC_PROD_CURSOR")
	}
	b, e := base64.RawURLEncoding.DecodeString(raw)
	if e != nil {
		return time.Time{}, "", sewProdInvalid("ERR_QC_PROD_CURSOR")
	}
	parts := strings.Split(string(b), "|")
	if len(parts) != 2 || !sewUUID(parts[1]) {
		return time.Time{}, "", sewProdInvalid("ERR_QC_PROD_CURSOR")
	}
	at, e := time.Parse(time.RFC3339Nano, parts[0])
	if e != nil {
		return time.Time{}, "", sewProdInvalid("ERR_QC_PROD_CURSOR")
	}
	return at, parts[1], nil
}
func (q *qcProdAPI) catalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	if !q.ready(w) {
		return
	}
	token, e := bearer(r)
	if e != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_QC_PROD_BEARER_REQUIRED"})
		return
	}
	actor, e := q.sewing.verifier.Verify(token, "quality.read", qcProdReadChallenge, "quality_inspector", true)
	if e != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_QC_PROD_ASSERTION"})
		return
	}
	if !sewFields(actor.TenantID, actor.PlantID) {
		docJSON(w, 403, map[string]string{"error": "ERR_QC_PROD_SCOPE"})
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 100 {
			docJSON(w, 422, map[string]string{"error": "ERR_QC_PROD_PAGE_LIMIT"})
			return
		}
		limit = n
	}
	at, id, e := qcProdParseCursor(r.URL.Query().Get("cursor"))
	if e != nil {
		sewProdReply(w, e)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, e := q.sewing.server.docTransaction(ctx, actor.TenantID)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	defer tx.Rollback()
	if e = workforceChapterRequire(ctx, tx, actor, "quality.read"); e != nil {
		sewProdReply(w, e)
		return
	}
	// Read access also requires an enabled factory assignment, not merely a role in a JWT.
	var authorized bool
	e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM qc_prod_assignments_v11
   WHERE tenant_id=$1 AND plant_id=$2 AND worker_subject=$3 AND role='quality_inspector' AND enabled)`,
		actor.TenantID, actor.PlantID, actor.Subject).Scan(&authorized)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	if !authorized {
		docJSON(w, 403, map[string]string{"error": "ERR_QC_PROD_ASSIGNMENT"})
		return
	}
	rows, e := tx.QueryContext(ctx, `SELECT c.id::text,c.lay_draft_id::text,c.route_id::text,c.nominal_ply_count,c.created_at,
 COALESCE(q.next_state,'PENDING'),COALESCE(q.revision,0),COALESCE(q.defect_code,''),q.created_at
 FROM sew_prod_completions_v10 c
 LEFT JOIN LATERAL (
  SELECT next_state,revision,defect_code,created_at FROM qc_prod_group_events_v11 e
  WHERE e.tenant_id=c.tenant_id AND e.completion_id=c.id ORDER BY revision DESC LIMIT 1
 ) q ON true
 WHERE c.tenant_id=$1 AND c.plant_id=$2 AND
 ($3::timestamptz IS NULL OR (c.created_at,c.id)<($3::timestamptz,$4::uuid))
 ORDER BY c.created_at DESC,c.id DESC LIMIT $5`, actor.TenantID, actor.PlantID, qcNullableTime(at), qcNullableID(id), limit+1)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	entries := []map[string]any{}
	type edge struct {
		at time.Time
		id string
	}
	var last edge
	hasExtra := false
	for rows.Next() {
		var completion, lay, route, state, defect string
		var qty, revision int
		var created time.Time
		var inspected sql.NullTime
		if e = rows.Scan(&completion, &lay, &route, &qty, &created, &state, &revision, &defect, &inspected); e != nil {
			break
		}
		if len(entries) == limit {
			hasExtra = true
			break
		}
		var qcAt any
		if inspected.Valid {
			qcAt = inspected.Time
		}
		entries = append(entries, map[string]any{"completion_id": completion, "lay_id": lay, "route_id": route, "nominal_ply_count": qty, "qc_group_state": state, "revision": revision, "last_defect": defect, "sewing_at": created, "qc_at": qcAt, "packing_release": false})
		last = edge{at: created, id: completion}
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
	more := hasExtra && last.id != ""
	// A page of precisely `limit` records may have no further rows; query limit+1.
	next := ""
	if more {
		next = qcProdCursor(last.at, last.id)
	}
	docJSON(w, 200, map[string]any{"groups": entries, "next_cursor": next, "evidence": qcProdEvidence, "plant_id": actor.PlantID, "page_size": len(entries)})
}
func qcNullableTime(v time.Time) any {
	if v.IsZero() {
		return nil
	}
	return v
}
func qcNullableID(v string) any {
	if v == "" {
		return nil
	}
	return v
}
func registerQCProdRoutes(mux *http.ServeMux, s *EdgeServer) {
	q := newQCProdAPI(s)
	mux.HandleFunc("/api/v1/quality/production/events/challenge", q.challenge)
	mux.HandleFunc("/api/v1/quality/production/events", q.event)
	mux.HandleFunc("/api/v1/quality/production/catalog", q.catalog)
}
