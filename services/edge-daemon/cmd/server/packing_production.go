// Author: Sunil Jain, 2026-10-28.
// Chapter 08: authenticated, plant-scoped packing readiness and immutable staging
// HOLDs for sewing completion groups. NEVER creates physical cartons or dispatch.
package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/attestation"
)

const packProdEvidence = "GROUP_STAGING_HOLD_NO_GARMENT_CARTON_OR_DISPATCH_RELEASE"
const packProdReadChallenge = "PACK_PROD_READINESS_V1"

type packProdCommand struct {
	CommandID          string `json:"command_id"`
	CompletionID       string `json:"completion_id"`
	PlantID            string `json:"plant_id"`
	StationID          string `json:"station_id"`
	ExpectedQCRevision int    `json:"expected_qc_revision"`
	Note               string `json:"note"`
}
type packProdRequest struct {
	Command packProdCommand `json:"command"`
}
type packProdAPI struct {
	quality *qcProdAPI
	enabled bool
	reason  string
}

func newPackProdAPI(s *EdgeServer) *packProdAPI {
	p := &packProdAPI{reason: "ERR_PACK_PROD_DISABLED"}
	if os.Getenv("AGIPAREL_PACKING_PRODUCTION_ENABLED") != "true" {
		return p
	}
	q := newQCProdAPI(s)
	if !q.enabled {
		p.reason = "ERR_PACK_PROD_QUALITY_PREREQUISITE"
		return p
	}
	p.quality = q
	p.enabled = true
	return p
}
func (p *packProdAPI) ready(w http.ResponseWriter) bool {
	if !p.enabled {
		docJSON(w, 503, map[string]string{"error": p.reason})
		return false
	}
	return true
}
func packProdValid(c packProdCommand) error {
	if !sewCommandID.MatchString(c.CommandID) || !sewUUID(c.CompletionID) ||
		!sewFields(c.PlantID, c.StationID) || c.ExpectedQCRevision < 1 ||
		len(c.Note) > 1000 || strings.ContainsRune(c.Note, 0) {
		return sewProdInvalid("ERR_PACK_PROD_FIELDS")
	}
	return nil
}
func packProdChallenge(c packProdCommand) string { return "PACK_PROD_STAGE_V1:" + sewDigest(c) }
func (p *packProdAPI) challenge(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var v packProdRequest
	if !sewProdStrictJSON(w, r, &v) {
		return
	}
	if err := packProdValid(v.Command); err != nil {
		sewProdReply(w, err)
		return
	}
	docJSON(w, 200, map[string]string{"challenge": packProdChallenge(v.Command), "action": "packing.stage", "required_role": "packing_operator", "required_auth": "webauthn_or_fido2", "effect": "HOLD_ONLY"})
}
func (p *packProdAPI) stage(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var v packProdRequest
	if !sewProdStrictJSON(w, r, &v) {
		return
	}
	c := v.Command
	if err := packProdValid(c); err != nil {
		sewProdReply(w, err)
		return
	}
	token, err := bearer(r)
	if err != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_PACK_PROD_BEARER_REQUIRED"})
		return
	}
	actor, err := p.quality.sewing.verifier.Verify(token, "packing.stage", packProdChallenge(c), "packing_operator", true)
	if err != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_PACK_PROD_ASSERTION"})
		return
	}
	if actor.PlantID != c.PlantID || !sewFields(actor.TenantID) || len(actor.Subject) > 128 || len(actor.JWTID) > 255 {
		docJSON(w, 403, map[string]string{"error": "ERR_PACK_PROD_SCOPE"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	id, replayed, err := p.commit(ctx, c, actor, token)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	status := 201
	if replayed {
		status = 200
	}
	docJSON(w, status, map[string]any{"hold_id": id, "replayed": replayed, "state": "HOLD_PENDING_UNIT_TRACEABILITY", "evidence": packProdEvidence, "carton_release": false, "shipping_release": false, "inventory_credit": false})
}
func (p *packProdAPI) commit(ctx context.Context, c packProdCommand, actor attestation.Claims, token string) (string, bool, error) {
	tx, err := p.quality.sewing.server.docTransaction(ctx, actor.TenantID)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback()
	if err = workforceChapterRequire(ctx, tx, actor, "packing.stage"); err != nil {
		return "", false, err
	}
	// Same lock as QC state-machine trigger: last-moment QC HOLD wins or fails staging.
	_, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1),hashtext('QC11:'||$2))`, actor.TenantID, c.CompletionID)
	if err != nil {
		return "", false, err
	}
	hash := sewDigest(c)
	var id, stored, subject string
	err = tx.QueryRowContext(ctx, `SELECT id::text,payload_sha256,actor_subject FROM pack_prod_holds_v12 WHERE tenant_id=$1 AND command_id=$2`, actor.TenantID, c.CommandID).Scan(&id, &stored, &subject)
	if err == nil {
		if stored != hash || subject != actor.Subject {
			return "", false, sewFail(409, "ERR_PACK_PROD_COMMAND_REUSE")
		}
		return id, true, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, err
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO pack_prod_holds_v12
  (tenant_id,command_id,payload_sha256,completion_id,qc_revision,plant_id,station_id,note,actor_subject,proof_id,assertion_sha256)
  VALUES($1,$2,$3,$4::uuid,$5,$6,$7,$8,$9,$10,$11) RETURNING id::text`,
		actor.TenantID, c.CommandID, hash, c.CompletionID, c.ExpectedQCRevision, c.PlantID, c.StationID, c.Note, actor.Subject, actor.JWTID, sewProdHashString(token)).Scan(&id)
	if err != nil {
		return "", false, sewFail(409, "ERR_PACK_PROD_QC_OR_ASSIGNMENT_OR_DUPLICATE_HOLD")
	}
	return id, false, tx.Commit()
}
func (p *packProdAPI) readiness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	if !p.ready(w) {
		return
	}
	token, err := bearer(r)
	if err != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_PACK_PROD_BEARER_REQUIRED"})
		return
	}
	actor, err := p.quality.sewing.verifier.Verify(token, "packing.read", packProdReadChallenge, "packing_supervisor", true)
	if err != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_PACK_PROD_ASSERTION"})
		return
	}
	if !sewFields(actor.TenantID, actor.PlantID) {
		docJSON(w, 403, map[string]string{"error": "ERR_PACK_PROD_SCOPE"})
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			docJSON(w, 422, map[string]string{"error": "ERR_PACK_PROD_LIMIT"})
			return
		}
	}
	at, id, err := qcProdParseCursor(r.URL.Query().Get("cursor"))
	if err != nil {
		sewProdReply(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, err := p.quality.sewing.server.docTransaction(ctx, actor.TenantID)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	defer tx.Rollback()
	if err = workforceChapterRequire(ctx, tx, actor, "packing.read"); err != nil {
		sewProdReply(w, err)
		return
	}
	var authorized bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pack_prod_assignments_v12 WHERE tenant_id=$1 AND plant_id=$2 AND worker_subject=$3 AND role='packing_supervisor' AND enabled)`, actor.TenantID, actor.PlantID, actor.Subject).Scan(&authorized)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	if !authorized {
		docJSON(w, 403, map[string]string{"error": "ERR_PACK_PROD_ASSIGNMENT"})
		return
	}
	rows, err := tx.QueryContext(ctx, `SELECT c.id::text,c.nominal_ply_count,c.created_at,
 COALESCE(q.next_state,'PENDING'),COALESCE(q.revision,0),
 COALESCE(h.id::text,''),COALESCE(h.qc_revision,0),COALESCE(h.state,'')
 FROM sew_prod_completions_v10 c
 LEFT JOIN LATERAL (SELECT next_state,revision FROM qc_prod_group_events_v11 e
  WHERE e.tenant_id=c.tenant_id AND e.completion_id=c.id ORDER BY revision DESC LIMIT 1) q ON true
 LEFT JOIN LATERAL (
  SELECT id,qc_revision,state FROM pack_prod_holds_v12 h
  WHERE h.tenant_id=c.tenant_id AND h.completion_id=c.id
  ORDER BY qc_revision DESC LIMIT 1
 ) h ON true
 WHERE c.tenant_id=$1 AND c.plant_id=$2
 AND ($3::timestamptz IS NULL OR (c.created_at,c.id)<($3::timestamptz,$4::uuid))
 ORDER BY c.created_at DESC,c.id DESC LIMIT $5`, actor.TenantID, actor.PlantID, qcNullableTime(at), qcNullableID(id), limit+1)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	entries := []map[string]any{}
	var lastAt time.Time
	var lastID string
	more := false
	for rows.Next() {
		var completion, state, hold, holdState string
		var qty, revision, holdRevision int
		var created time.Time
		if err = rows.Scan(&completion, &qty, &created, &state, &revision, &hold, &holdRevision, &holdState); err != nil {
			break
		}
		if len(entries) >= limit {
			more = true
			break
		}
		blocker := "QC_NOT_REVIEWED"
		if state == "REVIEWED" {
			blocker = "NEEDS_STAGING_HOLD"
		}
		if hold != "" {
			blocker = "INDIVIDUAL_UNIT_TRACEABILITY_MISSING"
			if state != "REVIEWED" || revision != holdRevision {
				blocker = "QC_CHANGED_AFTER_STAGING"
			}
		}
		entries = append(entries, map[string]any{"completion_id": completion, "nominal_group_units": qty, "sewing_at": created,
			"qc_group_state": state, "qc_revision": revision, "hold_id": hold, "hold_state": holdState,
			"blocker": blocker, "garment_units_verified": false, "carton_release": false, "dispatch_release": false})
		lastAt = created
		lastID = completion
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		sewProdReply(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		sewProdReply(w, err)
		return
	}
	next := ""
	if more {
		next = qcProdCursor(lastAt, lastID)
	}
	docJSON(w, 200, map[string]any{"groups": entries, "next_cursor": next, "plant_id": actor.PlantID, "page_size": len(entries),
		"evidence": packProdEvidence, "releasable_cartons": 0, "dispatch_authority": false})
}
func registerPackProdRoutes(mux *http.ServeMux, s *EdgeServer) {
	p := newPackProdAPI(s)
	mux.HandleFunc("/api/v1/packing/production/holds/challenge", p.challenge)
	mux.HandleFunc("/api/v1/packing/production/holds", p.stage)
	mux.HandleFunc("/api/v1/packing/production/readiness", p.readiness)
}
