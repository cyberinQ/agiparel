// Author: Sunil Jain, 2026-10-28.
// F3.5 signed individual-carton packing and staged manifest genealogy.
// Weight is human-attested, not calibrated; no shipment, inventory or payroll credit.
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

const packUnitReadChallenge = "PACK_UNIT_GENEALOGY_READ_V1"
const packUnitEvidence = "SIGNED_OPERATOR_PACKING_WEIGHT_NOT_CALIBRATED_NOT_SHIPPED"
const packManifestEvidence = "STAGED_DISPATCH_MANIFEST_NOT_CARRIER_HANDOVER_OR_SHIPMENT"

type packUnitCommand struct {
	CommandID      string `json:"command_id"`
	PlantID        string `json:"plant_id"`
	CartonID       string `json:"carton_id"`
	SpecID         string `json:"spec_id"`
	Action         string `json:"action"`
	CartonCode     string `json:"carton_code"`
	GarmentID      string `json:"garment_id"`
	ObservedG      int64  `json:"observed_g"`
	ExpectedG      int64  `json:"expected_g"`
	ScaleReference string `json:"scale_reference"`
	Note           string `json:"note"`
}
type packManifestCommand struct {
	CommandID      string `json:"command_id"`
	PlantID        string `json:"plant_id"`
	ManifestID     string `json:"manifest_id"`
	SpecID         string `json:"spec_id"`
	Action         string `json:"action"`
	ManifestRef    string `json:"manifest_ref"`
	DestinationRef string `json:"destination_ref"`
	CartonID       string `json:"carton_id"`
	Note           string `json:"note"`
}
type packUnitRequest struct {
	Command packUnitCommand `json:"command"`
}
type packManifestRequest struct {
	Command packManifestCommand `json:"command"`
}
type cartonGenealogyAPI struct {
	quality *garmentQCAPI
	enabled bool
	reason  string
}

func newCartonGenealogyAPI(s *EdgeServer) *cartonGenealogyAPI {
	p := &cartonGenealogyAPI{reason: "ERR_F35_DISABLED"}
	if os.Getenv("AGIPAREL_CARTON_GENEALOGY_ENABLED") != "true" {
		return p
	}
	p.quality = newGarmentQCAPI(s)
	if !p.quality.enabled {
		p.reason = "ERR_F35_GARMENT_QUALITY_PREREQUISITE"
		return p
	}
	p.enabled = true
	return p
}
func (p *cartonGenealogyAPI) ready(w http.ResponseWriter) bool {
	if p == nil || !p.enabled {
		reason := "ERR_F35_DISABLED"
		if p != nil {
			reason = p.reason
		}
		docJSON(w, 503, map[string]string{"error": reason})
		return false
	}
	return true
}
func packUnitPermission(action string) (string, string, bool) {
	switch action {
	case "OPEN", "ADD":
		return "packing.pack", "packing_operator", true
	case "SEAL":
		return "packing.seal", "packing_supervisor", true
	default:
		return "", "", false
	}
}
func packManifestPermission(action string) (string, string, bool) {
	switch action {
	case "OPEN", "ADD_CARTON":
		return "packing.manifest", "dispatch_coordinator", true
	case "STAGE":
		return "packing.dispatch_stage", "dispatch_supervisor", true
	default:
		return "", "", false
	}
}
func packUnitValid(c packUnitCommand) bool {
	if !sewCommandID.MatchString(c.CommandID) || !sewFields(c.PlantID) ||
		!sewUUID(c.CartonID) || !sewUUID(c.SpecID) || len(strings.TrimSpace(c.Note)) < 12 || len(c.Note) > 1000 ||
		strings.ContainsRune(c.Note, 0) {
		return false
	}
	switch c.Action {
	case "OPEN":
		return sewFields(c.CartonCode) && len(c.CartonCode) <= 80 && c.GarmentID == "" && c.ObservedG == 0 && c.ExpectedG == 0 && c.ScaleReference == ""
	case "ADD":
		return sewUUID(c.GarmentID) && c.CartonCode == "" && c.ObservedG == 0 && c.ExpectedG == 0 && c.ScaleReference == ""
	case "SEAL":
		return c.GarmentID == "" && c.CartonCode == "" && c.ObservedG > 0 && c.ExpectedG > 0 &&
			c.ObservedG <= 1000000000000 && c.ExpectedG <= 1000000000000 && sewFields(c.ScaleReference) && len(c.ScaleReference) <= 128
	default:
		return false
	}
}
func packManifestValid(c packManifestCommand) bool {
	if !sewCommandID.MatchString(c.CommandID) || !sewFields(c.PlantID) || !sewUUID(c.ManifestID) ||
		!sewUUID(c.SpecID) || len(strings.TrimSpace(c.Note)) < 12 || len(c.Note) > 1000 || strings.ContainsRune(c.Note, 0) {
		return false
	}
	switch c.Action {
	case "OPEN":
		return sewFields(c.ManifestRef, c.DestinationRef) && len(c.ManifestRef) <= 80 && len(c.DestinationRef) <= 128 && c.CartonID == ""
	case "ADD_CARTON":
		return sewUUID(c.CartonID) && c.ManifestRef == "" && c.DestinationRef == ""
	case "STAGE":
		return c.CartonID == "" && c.ManifestRef == "" && c.DestinationRef == ""
	default:
		return false
	}
}
func packUnitChallenge(c packUnitCommand) string         { return "PACK_CARTON_V22:" + sewDigest(c) }
func packManifestChallenge(c packManifestCommand) string { return "PACK_MANIFEST_V22:" + sewDigest(c) }
func (p *cartonGenealogyAPI) unitChallenge(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var v packUnitRequest
	if !sewProdStrictJSON(w, r, &v) {
		return
	}
	if !packUnitValid(v.Command) {
		docJSON(w, 422, map[string]string{"error": "ERR_F35_CARTON_FIELDS"})
		return
	}
	perm, role, _ := packUnitPermission(v.Command.Action)
	docJSON(w, 200, map[string]any{"challenge": packUnitChallenge(v.Command), "permission": perm, "role": role,
		"auth": "webauthn_or_fido2", "packing_evidence": packUnitEvidence, "shipment_authorized": false})
}
func (p *cartonGenealogyAPI) manifestChallenge(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var v packManifestRequest
	if !sewProdStrictJSON(w, r, &v) {
		return
	}
	if !packManifestValid(v.Command) {
		docJSON(w, 422, map[string]string{"error": "ERR_F35_MANIFEST_FIELDS"})
		return
	}
	perm, role, _ := packManifestPermission(v.Command.Action)
	docJSON(w, 200, map[string]any{"challenge": packManifestChallenge(v.Command), "permission": perm, "role": role,
		"auth": "webauthn_or_fido2", "evidence": packManifestEvidence, "shipment_authorized": false})
}
func (p *cartonGenealogyAPI) actor(w http.ResponseWriter, r *http.Request, perm, role, challenge string) (attestation.Claims, string, bool) {
	token, e := bearer(r)
	if e != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_F35_SIGNED_BEARER_REQUIRED"})
		return attestation.Claims{}, "", false
	}
	a, e := p.quality.wip.gen.sewing.verifier.Verify(token, perm, challenge, role, true)
	if e != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_F35_SIGNED_ASSERTION"})
		return attestation.Claims{}, "", false
	}
	if !validChapterActor(a) || len(a.JWTID) == 0 || len(a.JWTID) > 255 {
		docJSON(w, 403, map[string]string{"error": "ERR_F35_SCOPE"})
		return attestation.Claims{}, "", false
	}
	return a, token, true
}
func (p *cartonGenealogyAPI) unitEvent(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var v packUnitRequest
	if !sewProdStrictJSON(w, r, &v) {
		return
	}
	c := v.Command
	if !packUnitValid(c) {
		docJSON(w, 422, map[string]string{"error": "ERR_F35_CARTON_FIELDS"})
		return
	}
	perm, role, _ := packUnitPermission(c.Action)
	a, token, ok := p.actor(w, r, perm, role, packUnitChallenge(c))
	if !ok {
		return
	}
	if a.PlantID != c.PlantID {
		docJSON(w, 403, map[string]string{"error": "ERR_F35_PLANT"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	id, seq, replay, e := p.saveUnit(ctx, c, a, token, perm)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	status := 201
	if replay {
		status = 200
	}
	docJSON(w, status, map[string]any{"id": id, "event_no": seq, "replayed": replay, "action": c.Action,
		"carton_sealed": c.Action == "SEAL", "weight_calibrated": false, "shipment_authorized": false, "evidence": packUnitEvidence})
}
func (p *cartonGenealogyAPI) saveUnit(ctx context.Context, c packUnitCommand, a attestation.Claims, token, perm string) (id string, seq int64, replay bool, err error) {
	tx, e := p.quality.wip.gen.sewing.server.docTransaction(ctx, a.TenantID)
	if e != nil {
		return "", 0, false, e
	}
	defer tx.Rollback()
	if e = workforceChapterRequire(ctx, tx, a, perm); e != nil {
		return "", 0, false, e
	}
	// Same namespace as DB trigger. Serializes status lookup and command replay.
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1),hashtext('F35:CARTON:'||$2))`, a.TenantID, c.CartonID); e != nil {
		return "", 0, false, e
	}
	hash := sewDigest(c)
	var oldHash, oldActor string
	e = tx.QueryRowContext(ctx, `SELECT id::text,event_no,command_sha256,actor_subject FROM pack_carton_events_v22
        WHERE tenant_id=$1 AND command_id=$2`, a.TenantID, c.CommandID).Scan(&id, &seq, &oldHash, &oldActor)
	if e == nil {
		if oldHash != hash || oldActor != a.Subject {
			return "", 0, false, sewFail(409, "ERR_F35_COMMAND_REUSE")
		}
		return id, seq, true, tx.Commit()
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return "", 0, false, e
	}
	var observed, expected any
	if c.Action == "SEAL" {
		observed = c.ObservedG
		expected = c.ExpectedG
	}
	e = tx.QueryRowContext(ctx, `INSERT INTO pack_carton_events_v22
        (tenant_id,plant_id,carton_id,event_no,action,spec_id,carton_code,garment_id,observed_g,expected_g,scale_reference,
        note,command_id,command_sha256,actor_subject,proof_id,assertion_sha256)
        VALUES ($1,$2,$3::uuid,1,$4,$5::uuid,NULLIF($6,''),NULLIF($7,'')::uuid,$8,$9,NULLIF($10,''),$11,$12,$13,$14,$15,$16)
        RETURNING id::text,event_no`, a.TenantID, c.PlantID, c.CartonID, c.Action, c.SpecID, c.CartonCode, c.GarmentID,
		observed, expected, c.ScaleReference, c.Note, c.CommandID, hash, a.Subject, a.JWTID, sewProdHashString(token)).Scan(&id, &seq)
	if e != nil {
		return "", 0, false, sewFail(409, "ERR_F35_CARTON_CONFLICT_OR_POLICY")
	}
	return id, seq, false, tx.Commit()
}
func (p *cartonGenealogyAPI) manifestEvent(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var v packManifestRequest
	if !sewProdStrictJSON(w, r, &v) {
		return
	}
	c := v.Command
	if !packManifestValid(c) {
		docJSON(w, 422, map[string]string{"error": "ERR_F35_MANIFEST_FIELDS"})
		return
	}
	perm, role, _ := packManifestPermission(c.Action)
	a, token, ok := p.actor(w, r, perm, role, packManifestChallenge(c))
	if !ok {
		return
	}
	if a.PlantID != c.PlantID {
		docJSON(w, 403, map[string]string{"error": "ERR_F35_PLANT"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	id, seq, replay, e := p.saveManifest(ctx, c, a, token, perm)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	status := 201
	if replay {
		status = 200
	}
	docJSON(w, status, map[string]any{"id": id, "event_no": seq, "replayed": replay, "action": c.Action,
		"manifest_staged": c.Action == "STAGE", "physical_handover": false, "shipped": false, "evidence": packManifestEvidence})
}
func (p *cartonGenealogyAPI) saveManifest(ctx context.Context, c packManifestCommand, a attestation.Claims, token, perm string) (id string, seq int64, replay bool, err error) {
	tx, e := p.quality.wip.gen.sewing.server.docTransaction(ctx, a.TenantID)
	if e != nil {
		return "", 0, false, e
	}
	defer tx.Rollback()
	if e = workforceChapterRequire(ctx, tx, a, perm); e != nil {
		return "", 0, false, e
	}
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1),hashtext('F35:MANIFEST:'||$2))`, a.TenantID, c.ManifestID); e != nil {
		return "", 0, false, e
	}
	hash := sewDigest(c)
	var oldHash, oldActor string
	e = tx.QueryRowContext(ctx, `SELECT id::text,event_no,command_sha256,actor_subject FROM pack_manifest_events_v22
       WHERE tenant_id=$1 AND command_id=$2`, a.TenantID, c.CommandID).Scan(&id, &seq, &oldHash, &oldActor)
	if e == nil {
		if oldHash != hash || oldActor != a.Subject {
			return "", 0, false, sewFail(409, "ERR_F35_COMMAND_REUSE")
		}
		return id, seq, true, tx.Commit()
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return "", 0, false, e
	}
	e = tx.QueryRowContext(ctx, `INSERT INTO pack_manifest_events_v22
       (tenant_id,plant_id,manifest_id,event_no,action,spec_id,manifest_ref,destination_ref,carton_id,
       note,command_id,command_sha256,actor_subject,proof_id,assertion_sha256)
       VALUES ($1,$2,$3::uuid,1,$4,$5::uuid,NULLIF($6,''),NULLIF($7,''),NULLIF($8,'')::uuid,
       $9,$10,$11,$12,$13,$14) RETURNING id::text,event_no`,
		a.TenantID, c.PlantID, c.ManifestID, c.Action, c.SpecID, c.ManifestRef, c.DestinationRef, c.CartonID,
		c.Note, c.CommandID, hash, a.Subject, a.JWTID, sewProdHashString(token)).Scan(&id, &seq)
	if e != nil {
		return "", 0, false, sewFail(409, "ERR_F35_MANIFEST_CONFLICT_OR_POLICY")
	}
	return id, seq, false, tx.Commit()
}
func (p *cartonGenealogyAPI) history(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	if !p.ready(w) {
		return
	}
	q := r.URL.Query()
	for k := range q {
		if k != "kind" && k != "id" && k != "limit" && k != "cursor" {
			docJSON(w, 422, map[string]string{"error": "ERR_F35_QUERY"})
			return
		}
	}
	if len(q["kind"]) != 1 || len(q["id"]) != 1 || len(q["limit"]) > 1 || len(q["cursor"]) > 1 || !sewUUID(q.Get("id")) ||
		(q.Get("kind") != "carton" && q.Get("kind") != "manifest") {
		docJSON(w, 422, map[string]string{"error": "ERR_F35_QUERY"})
		return
	}
	limit := 50
	if q.Get("limit") != "" {
		n, e := strconv.Atoi(q.Get("limit"))
		if e != nil || n < 1 || n > 100 {
			docJSON(w, 422, map[string]string{"error": "ERR_F35_LIMIT"})
			return
		}
		limit = n
	}
	cursor := int64(0)
	if q.Get("cursor") != "" {
		n, e := strconv.ParseInt(q.Get("cursor"), 10, 64)
		if e != nil || n < 0 {
			docJSON(w, 422, map[string]string{"error": "ERR_F35_CURSOR"})
			return
		}
		cursor = n
	}
	a, _, ok := p.actor(w, r, "packing.read", "packing_supervisor", packUnitReadChallenge)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, e := p.quality.wip.gen.sewing.server.docTransaction(ctx, a.TenantID)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	defer tx.Rollback()
	if e = workforceChapterRequire(ctx, tx, a, "packing.read"); e != nil {
		sewProdReply(w, e)
		return
	}
	table, field := "pack_carton_events_v22", "carton_id"
	detail := "COALESCE(garment_id::text,''),COALESCE(carton_code,''),COALESCE(observed_g,0),COALESCE(expected_g,0),COALESCE(scale_reference,'')"
	if q.Get("kind") == "manifest" {
		table, field = "pack_manifest_events_v22", "manifest_id"
		detail = "COALESCE(carton_id::text,''),COALESCE(manifest_ref,''),0::bigint,0::bigint,COALESCE(destination_ref,'')"
	}
	// All interpolated fragments are literals chosen above; never user input.
	query := "SELECT id::text,event_no,action,recorded_at,actor_subject,note,command_id,spec_id::text," + detail + " FROM " + table +
		" WHERE tenant_id=$1 AND plant_id=$2 AND " + field + "=$3::uuid AND event_no>$4 ORDER BY event_no LIMIT $5"
	rows, e := tx.QueryContext(ctx, query, a.TenantID, a.PlantID, q.Get("id"), cursor, limit+1)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	entries := []map[string]any{}
	var next int64
	more := false
	for rows.Next() {
		var id, action, actor, note, cmd, spec, linked, reference, scale string
		var seq, observed, expected int64
		var at time.Time
		if e = rows.Scan(&id, &seq, &action, &at, &actor, &note, &cmd, &spec, &linked, &reference, &observed, &expected, &scale); e != nil {
			break
		}
		if len(entries) >= limit {
			more = true
			break
		}
		detail := map[string]any{"id": id, "event_no": seq, "action": action, "recorded_at": at,
			"actor_subject": actor, "note": note, "command_id": cmd, "spec_id": spec}
		if q.Get("kind") == "carton" {
			detail["garment_id"] = linked
			detail["carton_code"] = reference
			detail["observed_g"] = observed
			detail["expected_g"] = expected
			detail["scale_reference"] = scale
		} else {
			detail["carton_id"] = linked
			detail["manifest_ref"] = reference
			detail["destination_ref"] = scale
		}
		entries = append(entries, detail)
		next = seq
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
	nextCursor := ""
	if more {
		nextCursor = strconv.FormatInt(next, 10)
	}
	docJSON(w, 200, map[string]any{"kind": q.Get("kind"), "id": q.Get("id"), "events": entries, "next_cursor": nextCursor,
		"shipped": false, "weight_calibrated": false, "source_authority": "SIGNED_HUMAN_PACKING_OR_STAGING_ONLY"})
}

// specification exposes only the provisioned, currently enabled plan and
// size-weight rules for a plant-scoped signed reader. It never generates plans.
func (p *cartonGenealogyAPI) specification(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	if !p.ready(w) {
		return
	}
	q := r.URL.Query()
	for key := range q {
		if key != "lay_id" {
			docJSON(w, 422, map[string]string{"error": "ERR_F35_SPEC_QUERY"})
			return
		}
	}
	if len(q["lay_id"]) != 1 || !sewUUID(q.Get("lay_id")) {
		docJSON(w, 422, map[string]string{"error": "ERR_F35_SPEC_QUERY"})
		return
	}
	a, _, ok := p.actor(w, r, "packing.read", "packing_supervisor", packUnitReadChallenge)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, e := p.quality.wip.gen.sewing.server.docTransaction(ctx, a.TenantID)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	defer tx.Rollback()
	if e = workforceChapterRequire(ctx, tx, a, "packing.read"); e != nil {
		sewProdReply(w, e)
		return
	}
	var id, rev, source, hash, approvedBy string
	var min, max int
	var tare, tolerance int64
	e = tx.QueryRowContext(ctx, `SELECT id::text,revision,source_document_id,source_sha256,approved_by,
       min_units,max_units,tare_g,tolerance_g FROM pack_carton_specs_v22
       WHERE tenant_id=$1 AND plant_id=$2 AND lay_draft_id=$3::uuid AND enabled`,
		a.TenantID, a.PlantID, q.Get("lay_id")).Scan(&id, &rev, &source, &hash, &approvedBy, &min, &max, &tare, &tolerance)
	if errors.Is(e, sql.ErrNoRows) {
		docJSON(w, 404, map[string]string{"error": "ERR_F35_APPROVED_PACK_PLAN_REQUIRED"})
		return
	}
	if e != nil {
		sewProdReply(w, e)
		return
	}
	rows, e := tx.QueryContext(ctx, `SELECT size_code,expected_g FROM pack_size_weights_v22
       WHERE tenant_id=$1 AND spec_id=$2::uuid ORDER BY size_code`, a.TenantID, id)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	weights := []map[string]any{}
	for rows.Next() {
		var size string
		var grams int64
		if e = rows.Scan(&size, &grams); e != nil {
			break
		}
		weights = append(weights, map[string]any{"size_code": size, "expected_g": grams})
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
	docJSON(w, 200, map[string]any{"spec_id": id, "revision": rev, "source_document_id": source,
		"source_sha256": hash, "approved_by": approvedBy, "min_units": min, "max_units": max,
		"tare_g": tare, "tolerance_g": tolerance, "size_weights": weights,
		"scale_calibrated": false, "dispatch_shipped": false})
}

func registerCartonGenealogyRoutes(mux *http.ServeMux, s *EdgeServer) {
	p := newCartonGenealogyAPI(s)
	mux.HandleFunc("/api/v1/packing/units/challenge", p.unitChallenge)
	mux.HandleFunc("/api/v1/packing/units/events", p.unitEvent)
	mux.HandleFunc("/api/v1/packing/manifests/challenge", p.manifestChallenge)
	mux.HandleFunc("/api/v1/packing/manifests/events", p.manifestEvent)
	mux.HandleFunc("/api/v1/packing/genealogy", p.history)
	mux.HandleFunc("/api/v1/packing/specification", p.specification)
}
