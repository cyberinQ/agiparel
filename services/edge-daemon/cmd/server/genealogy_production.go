// Author: Sunil Jain, 2026-10-28.
// F3.1 permanent garment/part/bundle identity reservation and reversible RFID
// carrier custody. Identity reservation is NOT accepted physical production.
package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/attestation"
)

const genReserveState = "IDENTITY_RESERVED_NOT_PHYSICALLY_VERIFIED"
const genCarrierEvidence = "SIGNED_HUMAN_CARRIER_CLAIM_NOT_RF_PROOF"
const genReadChallenge = "GENEALOGY_READ_V1"

type genReserveCommand struct {
	CommandID      string `json:"command_id"`
	PlantID        string `json:"plant_id"`
	ReleaseID      string `json:"release_id"`
	LayID          string `json:"lay_id"`
	MarkerPosition int    `json:"marker_position"`
	PlyNumber      int    `json:"ply_number"`
}
type genCarrierCommand struct {
	CommandID  string `json:"command_id"`
	PlantID    string `json:"plant_id"`
	CarrierID  string `json:"carrier_id"`
	Action     string `json:"action"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
}
type genReserveRequest struct {
	Command genReserveCommand `json:"command"`
}
type genCarrierRequest struct {
	Command genCarrierCommand `json:"command"`
}

type genProductionAPI struct {
	sewing  *sewProdAPI
	enabled bool
	reason  string
}

func newGenProductionAPI(s *EdgeServer) *genProductionAPI {
	p := &genProductionAPI{reason: "ERR_GEN_DISABLED"}
	if os.Getenv("AGIPAREL_GENEALOGY_PRODUCTION_ENABLED") != "true" {
		return p
	}
	p.sewing = newSewProdAPI(s)
	if !p.sewing.enabled {
		p.reason = "ERR_GEN_SIGNED_EVIDENCE_PREREQUISITE"
		return p
	}
	p.enabled = true
	return p
}
func (p *genProductionAPI) ready(w http.ResponseWriter) bool {
	if !p.enabled {
		docJSON(w, 503, map[string]string{"error": p.reason})
		return false
	}
	return true
}
func genValidReserve(c genReserveCommand) bool {
	return sewCommandID.MatchString(c.CommandID) && sewFields(c.PlantID) &&
		sewUUID(c.ReleaseID) && sewUUID(c.LayID) && c.MarkerPosition > 0 && c.PlyNumber > 0
}
func genValidCarrier(c genCarrierCommand) bool {
	return sewCommandID.MatchString(c.CommandID) && sewFields(c.PlantID) &&
		sewUUID(c.CarrierID) && sewUUID(c.TargetID) &&
		(c.Action == "BIND" || c.Action == "RETIRE") &&
		(c.TargetType == "GARMENT" || c.TargetType == "CUT_PART" || c.TargetType == "BUNDLE")
}
func genReserveChallenge(c genReserveCommand) string { return "GENEALOGY_RESERVE_V1:" + sewDigest(c) }
func genCarrierChallenge(c genCarrierCommand) string { return "GENEALOGY_CARRIER_V1:" + sewDigest(c) }
func (p *genProductionAPI) reserveChallenge(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var v genReserveRequest
	if !sewProdStrictJSON(w, r, &v) {
		return
	}
	if !genValidReserve(v.Command) {
		docJSON(w, 422, map[string]string{"error": "ERR_GEN_FIELDS"})
		return
	}
	docJSON(w, 200, map[string]any{"challenge": genReserveChallenge(v.Command), "action": "cutting.release", "required_role": "cutting_supervisor", "production_credit": false})
}
func (p *genProductionAPI) carrierChallenge(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var v genCarrierRequest
	if !sewProdStrictJSON(w, r, &v) {
		return
	}
	if !genValidCarrier(v.Command) {
		docJSON(w, 422, map[string]string{"error": "ERR_GEN_CARRIER_FIELDS"})
		return
	}
	docJSON(w, 200, map[string]any{"challenge": genCarrierChallenge(v.Command), "action": "cutting.release", "required_role": "cutting_supervisor", "hardware_rf_verified": false})
}
func (p *genProductionAPI) actor(w http.ResponseWriter, r *http.Request, action, challenge string) (attestation.Claims, string, bool) {
	raw, err := bearer(r)
	if err != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_GEN_BEARER_REQUIRED"})
		return attestation.Claims{}, "", false
	}
	claim, err := p.sewing.verifier.Verify(raw, action, challenge, "cutting_supervisor", true)
	if err != nil || !sewFields(claim.TenantID, claim.PlantID) || len(claim.Subject) > 128 || len(claim.JWTID) > 255 {
		docJSON(w, 401, map[string]string{"error": "ERR_GEN_STRONG_ASSERTION"})
		return attestation.Claims{}, "", false
	}
	return claim, raw, true
}
func (p *genProductionAPI) reserve(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var v genReserveRequest
	if !sewProdStrictJSON(w, r, &v) {
		return
	}
	c := v.Command
	if !genValidReserve(c) {
		docJSON(w, 422, map[string]string{"error": "ERR_GEN_FIELDS"})
		return
	}
	a, raw, ok := p.actor(w, r, "cutting.release", genReserveChallenge(c))
	if !ok {
		return
	}
	if c.PlantID != a.PlantID {
		docJSON(w, 403, map[string]string{"error": "ERR_GEN_PLANT_SCOPE"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	id, count, replayed, err := p.reserveTxn(ctx, c, a, raw)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	code := 201
	if replayed {
		code = 200
	}
	docJSON(w, code, map[string]any{"garment_id": id, "cut_part_references": count, "replayed": replayed, "state": genReserveState, "physical_acceptance": false, "sewing_credit": false, "quality_release": false, "finished_inventory": false, "shipment_release": false})
}
func (p *genProductionAPI) reserveTxn(ctx context.Context, c genReserveCommand, a attestation.Claims, raw string) (string, int, bool, error) {
	tx, err := p.sewing.server.docTransaction(ctx, a.TenantID)
	if err != nil {
		return "", 0, false, err
	}
	defer tx.Rollback()
	if err = workforceChapterRequire(ctx, tx, a, "cutting.release"); err != nil {
		return "", 0, false, err
	}
	hash := sewDigest(c)
	// Command replay must match exact payload and actor, before any item is created.
	var id, oldHash, oldWorker string
	err = tx.QueryRowContext(ctx, `SELECT id::text,command_sha256,reserved_by FROM gen_garments_v18 WHERE tenant_id=$1 AND command_id=$2`, a.TenantID, c.CommandID).Scan(&id, &oldHash, &oldWorker)
	if err == nil {
		if hash != oldHash || a.Subject != oldWorker {
			return "", 0, false, sewFail(409, "ERR_GEN_COMMAND_REUSE")
		}
		var n int
		if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM gen_cut_parts_v18 WHERE tenant_id=$1 AND garment_id=$2::uuid`, a.TenantID, id).Scan(&n); err != nil {
			return "", 0, false, err
		}
		return id, n, true, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", 0, false, err
	}
	// One canonical bundle per released physical bundle label; do not invent an EPC.
	_, err = tx.ExecContext(ctx, `INSERT INTO gen_bundles_v18(tenant_id,plant_id,release_id,lay_draft_id,label_code)
 SELECT $1,$2,$3::uuid,$4::uuid,b.label_code FROM cut_bundle_drafts_v8 b
 WHERE b.tenant_id=$1 AND b.lay_draft_id=$4::uuid AND b.marker_position=$5 AND $6 BETWEEN b.ply_start AND b.ply_end
 ON CONFLICT (tenant_id,lay_draft_id,label_code) DO NOTHING`, a.TenantID, c.PlantID, c.ReleaseID, c.LayID, c.MarkerPosition, c.PlyNumber)
	if err != nil {
		return "", 0, false, sewFail(409, "ERR_GEN_BUNDLE_RELEASE")
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO gen_garments_v18
 (tenant_id,plant_id,release_id,lay_draft_id,marker_position,ply_number,command_id,command_sha256,reserved_by,proof_id,assertion_sha256)
 VALUES($1,$2,$3::uuid,$4::uuid,$5,$6,$7,$8,$9,$10,$11) RETURNING id::text`,
		a.TenantID, c.PlantID, c.ReleaseID, c.LayID, c.MarkerPosition, c.PlyNumber, c.CommandID, hash, a.Subject, a.JWTID, sewProdHashString(raw)).Scan(&id)
	if err != nil {
		return "", 0, false, sewFail(409, "ERR_GEN_ALREADY_RESERVED_OR_INCOMPLETE_RELEASE")
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO gen_cut_parts_v18(tenant_id,garment_id,bundle_id,lay_draft_id,label_code,part_code,part_slot)
 SELECT $1,$2::uuid,g.id,$3::uuid,b.label_code,b.part_code,s.slot
 FROM cut_bundle_drafts_v8 b JOIN gen_bundles_v18 g ON g.tenant_id=b.tenant_id AND g.lay_draft_id=b.lay_draft_id AND g.label_code=b.label_code
 CROSS JOIN LATERAL generate_series(1,b.pieces_per_garment) s(slot)
 WHERE b.tenant_id=$1 AND b.lay_draft_id=$3::uuid AND b.marker_position=$4 AND $5 BETWEEN b.ply_start AND b.ply_end`,
		a.TenantID, id, c.LayID, c.MarkerPosition, c.PlyNumber)
	if err != nil {
		return "", 0, false, sewFail(409, "ERR_GEN_PARTS_NOT_CONSISTENT")
	}
	n, err := result.RowsAffected()
	if err != nil {
		return "", 0, false, err
	}
	if err = tx.Commit(); err != nil {
		return "", 0, false, sewFail(409, "ERR_GEN_PART_COVERAGE_OR_CONFLICT")
	}
	return id, int(n), false, nil
}
func (p *genProductionAPI) carrierEvent(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var v genCarrierRequest
	if !sewProdStrictJSON(w, r, &v) {
		return
	}
	c := v.Command
	if !genValidCarrier(c) {
		docJSON(w, 422, map[string]string{"error": "ERR_GEN_CARRIER_FIELDS"})
		return
	}
	a, raw, ok := p.actor(w, r, "cutting.release", genCarrierChallenge(c))
	if !ok {
		return
	}
	if c.PlantID != a.PlantID {
		docJSON(w, 403, map[string]string{"error": "ERR_GEN_PLANT_SCOPE"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	tx, err := p.sewing.server.docTransaction(ctx, a.TenantID)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	defer tx.Rollback()
	if err = workforceChapterRequire(ctx, tx, a, "cutting.release"); err != nil {
		sewProdReply(w, err)
		return
	}
	hash := sewDigest(c)
	var id, oldHash, oldWorker string
	err = tx.QueryRowContext(ctx, `SELECT id::text,command_sha256,worker_subject FROM gen_carrier_events_v18 WHERE tenant_id=$1 AND command_id=$2`, a.TenantID, c.CommandID).Scan(&id, &oldHash, &oldWorker)
	if err == nil {
		if oldHash != hash || oldWorker != a.Subject {
			docJSON(w, 409, map[string]string{"error": "ERR_GEN_COMMAND_REUSE"})
			return
		}
		if err = tx.Commit(); err != nil {
			sewProdReply(w, err)
			return
		}
		docJSON(w, 200, map[string]any{"event_id": id, "replayed": true, "hardware_rf_verified": false})
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		sewProdReply(w, err)
		return
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO gen_carrier_events_v18
 (tenant_id,command_id,command_sha256,carrier_id,plant_id,action,target_type,target_id,worker_subject,proof_id,assertion_sha256)
 VALUES($1,$2,$3,$4::uuid,$5,$6,$7,$8::uuid,$9,$10,$11) RETURNING id::text`,
		a.TenantID, c.CommandID, hash, c.CarrierID, c.PlantID, c.Action, c.TargetType, c.TargetID, a.Subject, a.JWTID, sewProdHashString(raw)).Scan(&id)
	if err != nil {
		docJSON(w, 409, map[string]string{"error": "ERR_GEN_CARRIER_NOT_ENROLLED_OR_CONFLICT"})
		return
	}
	if err = tx.Commit(); err != nil {
		sewProdReply(w, err)
		return
	}
	docJSON(w, 201, map[string]any{"event_id": id, "action": c.Action, "evidence": genCarrierEvidence, "replayed": false, "hardware_rf_verified": false, "production_credit": false})
}

type genPartView struct {
	ID        string `json:"id"`
	BundleID  string `json:"bundle_id"`
	LabelCode string `json:"label_code"`
	PartCode  string `json:"part_code"`
	Slot      int    `json:"slot"`
}

func genPageLimit(q string) (int, error) {
	if q == "" {
		return 50, nil
	}
	v, e := strconv.Atoi(q)
	if e != nil || v < 1 || v > 100 {
		return 0, sewProdInvalid("ERR_GEN_PAGE_LIMIT")
	}
	return v, nil
}
func (p *genProductionAPI) trace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	if !p.ready(w) {
		return
	}
	q := r.URL.Query()
	if len(q["id"]) != 1 || !sewUUID(q.Get("id")) || len(q["cursor"]) > 1 || len(q["limit"]) > 1 || (q.Get("cursor") != "" && !sewUUID(q.Get("cursor"))) {
		docJSON(w, 422, map[string]string{"error": "ERR_GEN_TRACE_QUERY"})
		return
	}
	limit, err := genPageLimit(q.Get("limit"))
	if err != nil {
		sewProdReply(w, err)
		return
	}
	a, _, ok := p.actor(w, r, "cutting.read", genReadChallenge)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, err := p.sewing.server.docTransaction(ctx, a.TenantID)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	defer tx.Rollback()
	if err = workforceChapterRequire(ctx, tx, a, "cutting.read"); err != nil {
		sewProdReply(w, err)
		return
	}
	var id, release, lay, plant, state string
	var marker, ply int
	err = tx.QueryRowContext(ctx, `SELECT id::text,release_id::text,lay_draft_id::text,plant_id,marker_position,ply_number,state FROM gen_garments_v18 WHERE tenant_id=$1 AND id=$2::uuid AND plant_id=$3`, a.TenantID, q.Get("id"), a.PlantID).Scan(&id, &release, &lay, &plant, &marker, &ply, &state)
	if errors.Is(err, sql.ErrNoRows) {
		docJSON(w, 404, map[string]string{"error": "ERR_GEN_NOT_FOUND"})
		return
	}
	if err != nil {
		sewProdReply(w, err)
		return
	}
	var total int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM gen_cut_parts_v18 WHERE tenant_id=$1 AND garment_id=$2::uuid`, a.TenantID, id).Scan(&total); err != nil {
		sewProdReply(w, err)
		return
	}
	rows, err := tx.QueryContext(ctx, `SELECT id::text,bundle_id::text,label_code,part_code,part_slot FROM gen_cut_parts_v18
 WHERE tenant_id=$1 AND garment_id=$2::uuid AND ($3='' OR id>$3::uuid) ORDER BY id LIMIT $4`, a.TenantID, id, q.Get("cursor"), limit+1)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	parts := make([]genPartView, 0)
	for rows.Next() {
		var pt genPartView
		if e := rows.Scan(&pt.ID, &pt.BundleID, &pt.LabelCode, &pt.PartCode, &pt.Slot); e != nil {
			rows.Close()
			sewProdReply(w, e)
			return
		}
		parts = append(parts, pt)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		sewProdReply(w, err)
		return
	}
	more := len(parts) > limit
	if more {
		parts = parts[:limit]
	}
	next := ""
	if more {
		next = parts[len(parts)-1].ID
	}
	// No automatic physical acceptance, stock credit or packing release.
	if err = tx.Commit(); err != nil {
		sewProdReply(w, err)
		return
	}
	docJSON(w, 200, map[string]any{"garment_id": id, "release_id": release, "lay_id": lay, "plant_id": plant, "marker_position": marker, "ply_number": ply, "state": state, "cut_part_references": total, "parts": parts, "next_cursor": next, "has_more": more, "physical_acceptance": false, "finished_inventory": false, "shipping_release": false})
}

// A carrier can be bound and retired many times, but historical assignments remain
// append-only and are scoped to the requesting plant. The sequence is a cursor,
// not a count of physically observed scans.
type genCarrierHistoryItem struct {
	EventNo       int64     `json:"event_no"`
	Action        string    `json:"action"`
	TargetType    string    `json:"target_type"`
	TargetID      string    `json:"target_id"`
	WorkerSubject string    `json:"worker_subject"`
	RecordedAt    time.Time `json:"recorded_at"`
}

func genHistoryCursor(raw string) (int64, error) {
	if raw == "" {
		return 0, nil
	}
	n, e := strconv.ParseInt(raw, 10, 64)
	if e != nil || n < 1 {
		return 0, sewProdInvalid("ERR_GEN_HISTORY_CURSOR")
	}
	return n, nil
}
func (p *genProductionAPI) carrierHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	if !p.ready(w) {
		return
	}
	q := r.URL.Query()
	if len(q["carrier_id"]) != 1 || !sewUUID(q.Get("carrier_id")) || len(q["cursor"]) > 1 || len(q["limit"]) > 1 {
		docJSON(w, 422, map[string]string{"error": "ERR_GEN_CARRIER_QUERY"})
		return
	}
	cursor, err := genHistoryCursor(q.Get("cursor"))
	if err != nil {
		sewProdReply(w, err)
		return
	}
	limit, err := genPageLimit(q.Get("limit"))
	if err != nil {
		sewProdReply(w, err)
		return
	}
	a, _, ok := p.actor(w, r, "cutting.read", genReadChallenge)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, err := p.sewing.server.docTransaction(ctx, a.TenantID)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	defer tx.Rollback()
	if err = workforceChapterRequire(ctx, tx, a, "cutting.read"); err != nil {
		sewProdReply(w, err)
		return
	}
	rows, err := tx.QueryContext(ctx, `SELECT event_no,action,target_type,target_id::text,worker_subject,created_at
 FROM gen_carrier_events_v18 WHERE tenant_id=$1 AND plant_id=$2 AND carrier_id=$3::uuid
 AND ($4::bigint=0 OR event_no<$4) ORDER BY event_no DESC LIMIT $5`,
		a.TenantID, a.PlantID, q.Get("carrier_id"), cursor, limit+1)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	events := make([]genCarrierHistoryItem, 0)
	for rows.Next() {
		var e genCarrierHistoryItem
		if er := rows.Scan(&e.EventNo, &e.Action, &e.TargetType, &e.TargetID, &e.WorkerSubject, &e.RecordedAt); er != nil {
			rows.Close()
			sewProdReply(w, er)
			return
		}
		events = append(events, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		sewProdReply(w, err)
		return
	}
	more := len(events) > limit
	if more {
		events = events[:limit]
	}
	next := ""
	if more {
		next = strconv.FormatInt(events[len(events)-1].EventNo, 10)
	}
	if err = tx.Commit(); err != nil {
		sewProdReply(w, err)
		return
	}
	docJSON(w, 200, map[string]any{"carrier_id": q.Get("carrier_id"), "plant_id": a.PlantID, "events": events, "has_more": more, "next_cursor": next, "physical_rf_verified": false})
}
func registerGenProductionRoutes(mux *http.ServeMux, s *EdgeServer) {
	p := newGenProductionAPI(s)
	mux.HandleFunc("/api/v1/genealogy/units/challenge", p.reserveChallenge)
	mux.HandleFunc("/api/v1/genealogy/units", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			p.trace(w, r)
		} else {
			p.reserve(w, r)
		}
	})
	mux.HandleFunc("/api/v1/genealogy/carriers/challenge", p.carrierChallenge)
	mux.HandleFunc("/api/v1/genealogy/carriers/events", p.carrierEvent)
	mux.HandleFunc("/api/v1/genealogy/carriers/history", p.carrierHistory)
}
