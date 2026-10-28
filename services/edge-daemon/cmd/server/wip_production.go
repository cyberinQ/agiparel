// Author: Sunil Jain, 2026-10-28.
// F3.3: signed enrolled-reader scans plus worker-attested part custody transitions.
// Scanner reports are not independent location proof. No manufacturing/QC/shipping credit.
package main

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/attestation"
)

const wipScanVersion = "WIP_GATEWAY_SCAN_V1"
const wipReadChallenge = "WIP_CUSTODY_READ_V1"
const wipEvidence = "SIGNED_SCAN_AND_WORKER_CUSTODY_ATTESTATION"

type wipScan struct {
	Version    string `json:"version"`
	TenantID   string `json:"tenant_id"`
	PlantID    string `json:"plant_id"`
	GatewayID  string `json:"gateway_id"`
	StationID  string `json:"station_id"`
	EventID    string `json:"event_id"`
	PartID     string `json:"part_id"`
	CarrierID  string `json:"carrier_id"`
	EPCHex     string `json:"epc_hex"`
	ObservedAt string `json:"observed_at"`
}
type wipScanEnvelope struct {
	Event        wipScan `json:"event"`
	SignatureHex string  `json:"signature_hex"`
}
type wipMoveCommand struct {
	CommandID string `json:"command_id"`
	PlantID   string `json:"plant_id"`
	PartID    string `json:"part_id"`
	RouteID   string `json:"route_id"`
	ScanID    string `json:"scan_id"`
	Action    string `json:"action"`
	StationID string `json:"station_id"`
}
type wipMoveRequest struct {
	Command wipMoveCommand `json:"command"`
}
type wipAPI struct {
	gen     *genProductionAPI
	enabled bool
	reason  string
}

func newWIPAPI(s *EdgeServer) *wipAPI {
	p := &wipAPI{reason: "ERR_WIP_DISABLED"}
	if os.Getenv("AGIPAREL_WIP_PRODUCTION_ENABLED") != "true" {
		return p
	}
	p.gen = newGenProductionAPI(s)
	if !p.gen.enabled {
		p.reason = "ERR_WIP_GENEALOGY_PREREQUISITE"
		return p
	}
	if os.Getenv("AGIPAREL_CUTTING_RECONCILIATION_ENABLED") != "true" {
		p.reason = "ERR_WIP_CUT_ACCEPTANCE_PREREQUISITE"
		return p
	}
	p.enabled = true
	return p
}
func (p *wipAPI) ready(w http.ResponseWriter) bool {
	if p == nil || !p.enabled {
		code := "ERR_WIP_DISABLED"
		if p != nil {
			code = p.reason
		}
		docJSON(w, 503, map[string]string{"error": code})
		return false
	}
	return true
}
func wipScanBytes(v wipScan) []byte {
	return []byte("AGIPAREL_WIP_GATEWAY_V1\n" + strings.Join([]string{v.Version, v.TenantID, v.PlantID, v.GatewayID, v.StationID, v.EventID, v.PartID, v.CarrierID, v.EPCHex, v.ObservedAt}, "\n") + "\n")
}
func wipValidScan(v wipScan, now time.Time) bool {
	if v.Version != wipScanVersion || !sewFields(v.TenantID, v.PlantID, v.GatewayID, v.StationID) || !sewCommandID.MatchString(v.EventID) || !sewUUID(v.PartID) || !sewUUID(v.CarrierID) {
		return false
	}
	epc, e := canonicalEPCHex(v.EPCHex)
	if e != nil || epc != v.EPCHex {
		return false
	}
	t, e := time.Parse(time.RFC3339Nano, v.ObservedAt)
	return e == nil && !t.After(now.Add(2*time.Minute))
}
func wipValidMove(c wipMoveCommand) bool {
	return sewCommandID.MatchString(c.CommandID) && sewFields(c.PlantID, c.StationID) && sewUUID(c.PartID) && sewUUID(c.RouteID) && sewUUID(c.ScanID) && (c.Action == "DEPART" || c.Action == "ARRIVE")
}
func wipMoveChallenge(c wipMoveCommand) string { return "WIP_MOVE_V1:" + sewDigest(c) }
func wipScanMaxAge() (time.Duration, error) {
	s := os.Getenv("AGIPAREL_WIP_MAX_SCAN_AGE")
	if s == "" {
		return 15 * time.Minute, nil
	}
	d, e := time.ParseDuration(s)
	if e != nil || d < time.Minute || d > 24*time.Hour {
		return 0, sewFail(503, "ERR_WIP_SCAN_AGE_CONFIG")
	}
	return d, nil
}
func (p *wipAPI) scan(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var v wipScanEnvelope
	if !sewProdStrictJSON(w, r, &v) {
		return
	}
	if !wipValidScan(v.Event, time.Now().UTC()) {
		docJSON(w, 422, map[string]string{"error": "ERR_WIP_SCAN_FIELDS"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	id, replay, e := p.persistScan(ctx, v)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	code := 201
	if replay {
		code = 200
	}
	docJSON(w, code, map[string]any{"id": id, "replayed": replay, "authority": "SIGNED_GATEWAY_REPORT_NOT_INDEPENDENT_RF_PROOF", "production_credit": false})
}
func (p *wipAPI) persistScan(ctx context.Context, v wipScanEnvelope) (string, bool, error) {
	c := v.Event
	tx, e := p.gen.sewing.server.docTransaction(ctx, c.TenantID)
	if e != nil {
		return "", false, e
	}
	defer tx.Rollback()
	if e = sewTxLock(ctx, tx, c.TenantID, "WIP:SCAN:"+c.GatewayID+":"+c.EventID); e != nil {
		return "", false, e
	}
	var key []byte
	var plant, station string
	var enabled bool
	e = tx.QueryRowContext(ctx, `SELECT public_key,plant_id,station_id,enabled FROM sew_prod_gateways_v10 WHERE tenant_id=$1 AND gateway_id=$2`, c.TenantID, c.GatewayID).Scan(&key, &plant, &station, &enabled)
	if errors.Is(e, sql.ErrNoRows) {
		return "", false, sewFail(403, "ERR_WIP_GATEWAY_NOT_ENROLLED")
	}
	if e != nil {
		return "", false, e
	}
	sig, e := hex.DecodeString(v.SignatureHex)
	if e != nil || len(sig) != ed25519.SignatureSize || len(key) != ed25519.PublicKeySize || !enabled || plant != c.PlantID || station != c.StationID || !ed25519.Verify(ed25519.PublicKey(key), wipScanBytes(c), sig) {
		return "", false, sewFail(403, "ERR_WIP_GATEWAY_SIGNATURE_OR_SCOPE")
	}
	hash := sewProdHashString(string(wipScanBytes(c)))
	var id, existing string
	e = tx.QueryRowContext(ctx, `SELECT id::text,event_sha256 FROM wip_gateway_scans_v20 WHERE tenant_id=$1 AND gateway_id=$2 AND event_id=$3`, c.TenantID, c.GatewayID, c.EventID).Scan(&id, &existing)
	if e == nil {
		if existing != hash {
			return "", false, sewFail(409, "ERR_WIP_SCAN_REPLAY_MISMATCH")
		}
		return id, true, tx.Commit()
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return "", false, e
	}
	age, e := wipScanMaxAge()
	if e != nil {
		return "", false, e
	}
	t, _ := time.Parse(time.RFC3339Nano, c.ObservedAt)
	if t.Before(time.Now().Add(-age)) {
		return "", false, sewFail(422, "ERR_WIP_SCAN_EXPIRED")
	}
	// Trigger atomically verifies carrier binding, accepted-part source and tenant scope.
	e = tx.QueryRowContext(ctx, `INSERT INTO wip_gateway_scans_v20 (tenant_id,plant_id,station_id,gateway_id,event_id,part_id,carrier_id,epc_hex,observed_at,event_sha256,signature_sha256)
 VALUES ($1,$2,$3,$4,$5,$6::uuid,$7::uuid,$8,$9::timestamptz,$10,$11) RETURNING id::text`, c.TenantID, c.PlantID, c.StationID, c.GatewayID, c.EventID, c.PartID, c.CarrierID, c.EPCHex, c.ObservedAt, hash, sewProdSignatureHash(v.SignatureHex)).Scan(&id)
	if e != nil {
		return "", false, sewFail(409, "ERR_WIP_SCAN_SOURCE_CONFLICT")
	}
	return id, false, tx.Commit()
}
func (p *wipAPI) challenge(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var v wipMoveRequest
	if !sewProdStrictJSON(w, r, &v) {
		return
	}
	if !wipValidMove(v.Command) {
		docJSON(w, 422, map[string]string{"error": "ERR_WIP_MOVE_FIELDS"})
		return
	}
	docJSON(w, 200, map[string]any{"challenge": wipMoveChallenge(v.Command), "required_action": "sewing.transfer", "required_role": "sewing_operator", "required_auth": "webauthn_or_fido2", "authority": wipEvidence, "production_credit": false})
}
func (p *wipAPI) move(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var v wipMoveRequest
	if !sewProdStrictJSON(w, r, &v) {
		return
	}
	c := v.Command
	if !wipValidMove(c) {
		docJSON(w, 422, map[string]string{"error": "ERR_WIP_MOVE_FIELDS"})
		return
	}
	raw, e := bearer(r)
	if e != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_WIP_WORKER_BEARER_REQUIRED"})
		return
	}
	a, e := p.gen.sewing.verifier.Verify(raw, "sewing.transfer", wipMoveChallenge(c), "sewing_operator", true)
	if e != nil || a.PlantID != c.PlantID || !sewFields(a.TenantID) || len(a.Subject) > 128 || len(a.JWTID) > 255 {
		docJSON(w, 401, map[string]string{"error": "ERR_WIP_WORKER_ASSERTION"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	id, replay, ms, e := p.persistMove(ctx, c, a, raw)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	code := 201
	if replay {
		code = 200
	}
	docJSON(w, code, map[string]any{"id": id, "replayed": replay, "action": c.Action, "server_measured_transit_ms": ms, "authority": wipEvidence, "production_credit": false, "shipping_release": false})
}
func (p *wipAPI) persistMove(ctx context.Context, c wipMoveCommand, a attestation.Claims, raw string) (string, bool, any, error) {
	tx, e := p.gen.sewing.server.docTransaction(ctx, a.TenantID)
	if e != nil {
		return "", false, nil, e
	}
	defer tx.Rollback()
	if e = workforceChapterRequire(ctx, tx, a, "sewing.transfer"); e != nil {
		return "", false, nil, e
	}
	if e = sewTxLock(ctx, tx, a.TenantID, "WIP:PART:"+c.PartID); e != nil {
		return "", false, nil, e
	}
	hash := sewDigest(c)
	var id, priorHash, actor string
	var recorded time.Time
	e = tx.QueryRowContext(ctx, `SELECT id::text,command_sha256,worker_subject,recorded_at FROM wip_movements_v20 WHERE tenant_id=$1 AND command_id=$2`, a.TenantID, c.CommandID).Scan(&id, &priorHash, &actor, &recorded)
	if e == nil {
		if hash != priorHash || a.Subject != actor {
			return "", false, nil, sewFail(409, "ERR_WIP_MOVE_ID_REUSE")
		}
		ms, err := wipArrivalMillis(ctx, tx, a.TenantID, id, c.Action, c.PartID)
		if err != nil {
			return "", false, nil, err
		}
		return id, true, ms, tx.Commit()
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return "", false, nil, e
	}
	// Verify provisioner-defined route, station assignment and signed scan age.
	var from, to string
	var enabled, assigned bool
	e = tx.QueryRowContext(ctx, `SELECT from_station,to_station,enabled FROM wip_transfer_routes_v20 WHERE tenant_id=$1 AND id=$2::uuid AND plant_id=$3`, a.TenantID, c.RouteID, c.PlantID).Scan(&from, &to, &enabled)
	if errors.Is(e, sql.ErrNoRows) {
		return "", false, nil, sewFail(409, "ERR_WIP_APPROVED_ROUTE_REQUIRED")
	}
	if e != nil {
		return "", false, nil, e
	}
	if !enabled || (c.Action == "DEPART" && from != c.StationID) || (c.Action == "ARRIVE" && to != c.StationID) {
		return "", false, nil, sewFail(409, "ERR_WIP_STATION_ROUTE_MISMATCH")
	}
	e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sew_prod_worker_stations_v10 WHERE tenant_id=$1 AND plant_id=$2 AND station_id=$3 AND worker_subject=$4 AND enabled)`, a.TenantID, c.PlantID, c.StationID, a.Subject).Scan(&assigned)
	if e != nil {
		return "", false, nil, e
	}
	if !assigned {
		return "", false, nil, sewFail(403, "ERR_WIP_WORKER_NOT_ASSIGNED")
	}
	var scanAt time.Time
	e = tx.QueryRowContext(ctx, `SELECT received_at FROM wip_gateway_scans_v20 WHERE tenant_id=$1 AND id=$2::uuid AND part_id=$3::uuid AND plant_id=$4 AND station_id=$5`, a.TenantID, c.ScanID, c.PartID, c.PlantID, c.StationID).Scan(&scanAt)
	if errors.Is(e, sql.ErrNoRows) {
		return "", false, nil, sewFail(409, "ERR_WIP_SCAN_REQUIRED")
	}
	if e != nil {
		return "", false, nil, e
	}
	age, e := wipScanMaxAge()
	if e != nil {
		return "", false, nil, e
	}
	if time.Since(scanAt) > age {
		return "", false, nil, sewFail(409, "ERR_WIP_SCAN_TOO_OLD")
	}
	e = tx.QueryRowContext(ctx, `INSERT INTO wip_movements_v20(tenant_id,plant_id,part_id,route_id,scan_id,action,station_id,command_id,command_sha256,worker_subject,worker_proof_id,assertion_sha256)
 VALUES($1,$2,$3::uuid,$4::uuid,$5::uuid,$6,$7,$8,$9,$10,$11,$12) RETURNING id::text`, a.TenantID, c.PlantID, c.PartID, c.RouteID, c.ScanID, c.Action, c.StationID, c.CommandID, hash, a.Subject, a.JWTID, sewProdHashString(raw)).Scan(&id)
	if e != nil {
		return "", false, nil, sewFail(409, "ERR_WIP_CONCURRENT_CUSTODY_CONFLICT")
	}
	ms, e := wipArrivalMillis(ctx, tx, a.TenantID, id, c.Action, c.PartID)
	if e != nil {
		return "", false, nil, e
	}
	return id, false, ms, tx.Commit()
}

// Transit duration is calculated from the database's receipt timestamps, not
// the unaudited physical motion interval or possibly skewed reader clocks.
func wipArrivalMillis(ctx context.Context, tx *sql.Tx, tenant, id, action, part string) (any, error) {
	if action != "ARRIVE" {
		return nil, nil
	}
	var elapsed sql.NullInt64
	err := tx.QueryRowContext(ctx, `SELECT round(extract(epoch FROM (a.recorded_at-d.recorded_at))*1000)::bigint
 FROM wip_movements_v20 a JOIN wip_movements_v20 d ON d.tenant_id=a.tenant_id AND d.part_id=a.part_id
  AND d.event_no=(SELECT max(event_no) FROM wip_movements_v20 p WHERE p.tenant_id=a.tenant_id AND p.part_id=a.part_id AND p.event_no<a.event_no)
 WHERE a.tenant_id=$1 AND a.id=$2::uuid AND a.action='ARRIVE' AND d.action='DEPART' AND a.part_id=$3::uuid`, tenant, id, part).Scan(&elapsed)
	if err != nil {
		return nil, err
	}
	if !elapsed.Valid {
		return nil, nil
	}
	return elapsed.Int64, nil
}
func (p *wipAPI) history(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	if !p.ready(w) {
		return
	}
	q := r.URL.Query()
	for key := range q {
		if key != "part_id" && key != "cursor" && key != "limit" {
			docJSON(w, 422, map[string]string{"error": "ERR_WIP_HISTORY_QUERY"})
			return
		}
	}
	if len(q["part_id"]) != 1 || !sewUUID(q.Get("part_id")) || len(q["cursor"]) > 1 || len(q["limit"]) > 1 {
		docJSON(w, 422, map[string]string{"error": "ERR_WIP_HISTORY_QUERY"})
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
		if e != nil || cursor <= 0 {
			docJSON(w, 422, map[string]string{"error": "ERR_WIP_CURSOR"})
			return
		}
	}
	raw, e := bearer(r)
	if e != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_WIP_READ_BEARER"})
		return
	}
	a, e := p.gen.sewing.verifier.Verify(raw, "sewing.read", wipReadChallenge, "sewing_operator", true)
	if e != nil || !sewFields(a.TenantID, a.PlantID) {
		docJSON(w, 401, map[string]string{"error": "ERR_WIP_READ_ASSERTION"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	tx, e := p.gen.sewing.server.docTransaction(ctx, a.TenantID)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	defer tx.Rollback()
	if e = workforceChapterRequire(ctx, tx, a, "sewing.read"); e != nil {
		sewProdReply(w, e)
		return
	}
	var exists bool
	e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM gen_cut_parts_v18 p JOIN gen_garments_v18 g ON g.tenant_id=p.tenant_id AND g.id=p.garment_id
  WHERE p.tenant_id=$1 AND p.id=$2::uuid AND g.plant_id=$3)`, a.TenantID, q.Get("part_id"), a.PlantID).Scan(&exists)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	if !exists {
		docJSON(w, 404, map[string]string{"error": "ERR_WIP_PART_NOT_FOUND"})
		return
	}
	rows, e := tx.QueryContext(ctx, `SELECT event_no,id::text,action,station_id,route_id::text,worker_subject,recorded_at
 FROM wip_movements_v20 WHERE tenant_id=$1 AND plant_id=$2 AND part_id=$3::uuid AND event_no<$4
 ORDER BY event_no DESC LIMIT $5`, a.TenantID, a.PlantID, q.Get("part_id"), cursor, limit+1)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	type entry struct {
		EventNo    int64     `json:"event_no"`
		ID         string    `json:"id"`
		Action     string    `json:"action"`
		Station    string    `json:"station_id"`
		RouteID    string    `json:"route_id"`
		Worker     string    `json:"worker_subject"`
		RecordedAt time.Time `json:"recorded_at"`
	}
	items := make([]entry, 0)
	for rows.Next() {
		var t entry
		if e = rows.Scan(&t.EventNo, &t.ID, &t.Action, &t.Station, &t.RouteID, &t.Worker, &t.RecordedAt); e != nil {
			rows.Close()
			sewProdReply(w, e)
			return
		}
		items = append(items, t)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		sewProdReply(w, e)
		return
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	next := ""
	if more {
		next = strconv.FormatInt(items[len(items)-1].EventNo, 10)
	}
	if e = tx.Commit(); e != nil {
		sewProdReply(w, e)
		return
	}
	docJSON(w, 200, map[string]any{"part_id": q.Get("part_id"), "plant_id": a.PlantID, "events": items, "next_cursor": next, "has_more": more, "authority": wipEvidence, "physical_presence_independently_verified": false, "sewing_credit": false, "shipping_release": false})
}
func registerWIPRoutes(mux *http.ServeMux, s *EdgeServer) {
	p := newWIPAPI(s)
	mux.HandleFunc("/api/v1/wip/production/scans", p.scan)
	mux.HandleFunc("/api/v1/wip/production/movements/challenge", p.challenge)
	mux.HandleFunc("/api/v1/wip/production/movements", p.move)
	mux.HandleFunc("/api/v1/wip/production/history", p.history)
	mux.HandleFunc("/api/v1/wip/production/metrics", p.metrics)
}

// metrics reports bounded server-receipt durations for completed handoffs only.
// It must not infer operation time, human productivity or physical travel time.
func (p *wipAPI) metrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	if !p.ready(w) {
		return
	}
	q := r.URL.Query()
	for k := range q {
		if k != "route_id" && k != "since" && k != "until" {
			docJSON(w, 422, map[string]string{"error": "ERR_WIP_METRIC_QUERY"})
			return
		}
	}
	if len(q["route_id"]) != 1 || !sewUUID(q.Get("route_id")) || len(q["since"]) != 1 || len(q["until"]) != 1 {
		docJSON(w, 422, map[string]string{"error": "ERR_WIP_METRIC_QUERY"})
		return
	}
	since, e1 := time.Parse(time.RFC3339, q.Get("since"))
	until, e2 := time.Parse(time.RFC3339, q.Get("until"))
	if e1 != nil || e2 != nil || !until.After(since) || until.Sub(since) > 31*24*time.Hour || until.After(time.Now().Add(2*time.Minute)) {
		docJSON(w, 422, map[string]string{"error": "ERR_WIP_METRIC_WINDOW"})
		return
	}
	token, e := bearer(r)
	if e != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_WIP_METRIC_BEARER"})
		return
	}
	a, e := p.gen.sewing.verifier.Verify(token, "sewing.read", wipReadChallenge, "sewing_operator", true)
	if e != nil || !sewFields(a.TenantID, a.PlantID) {
		docJSON(w, 401, map[string]string{"error": "ERR_WIP_METRIC_ASSERTION"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	tx, e := p.gen.sewing.server.docTransaction(ctx, a.TenantID)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	defer tx.Rollback()
	if e = workforceChapterRequire(ctx, tx, a, "sewing.read"); e != nil {
		sewProdReply(w, e)
		return
	}
	var routeOK bool
	e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM wip_transfer_routes_v20 WHERE tenant_id=$1 AND plant_id=$2 AND id=$3::uuid)`, a.TenantID, a.PlantID, q.Get("route_id")).Scan(&routeOK)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	if !routeOK {
		docJSON(w, 404, map[string]string{"error": "ERR_WIP_ROUTE_NOT_FOUND"})
		return
	}
	var count int64
	var min, max, avg, sd, p50, p95 sql.NullFloat64
	// Match each ARRIVE to the immediately prior movement of the same part.
	// Filtering the ARRIVE timestamp bounds the cohort; DEPART may predate it.
	e = tx.QueryRowContext(ctx, `WITH intervals AS (
 SELECT extract(epoch FROM (a.recorded_at - d.recorded_at))*1000 AS ms
 FROM wip_movements_v20 a JOIN LATERAL (
  SELECT p.recorded_at,p.action,p.route_id FROM wip_movements_v20 p
   WHERE p.tenant_id=a.tenant_id AND p.part_id=a.part_id AND p.event_no<a.event_no
   ORDER BY p.event_no DESC LIMIT 1
 ) d ON d.action='DEPART' AND d.route_id=a.route_id
 WHERE a.tenant_id=$1 AND a.plant_id=$2 AND a.route_id=$3::uuid AND a.action='ARRIVE'
  AND a.recorded_at >= $4::timestamptz AND a.recorded_at < $5::timestamptz
 ) SELECT count(*)::bigint,min(ms)::double precision,max(ms)::double precision,
 avg(ms)::double precision,stddev_samp(ms)::double precision,
 percentile_cont(0.5) within group(order by ms)::double precision,
 percentile_cont(0.95) within group(order by ms)::double precision FROM intervals`, a.TenantID, a.PlantID, q.Get("route_id"), since, until).Scan(&count, &min, &max, &avg, &sd, &p50, &p95)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		sewProdReply(w, e)
		return
	}
	// Null metrics, rather than zero, when no closed transfers exist.
	value := func(v sql.NullFloat64) any {
		if v.Valid {
			return v.Float64
		}
		return nil
	}
	docJSON(w, 200, map[string]any{"plant_id": a.PlantID, "route_id": q.Get("route_id"), "since": since, "until": until,
		"completed_handoffs": count, "min_ms": value(min), "max_ms": value(max), "mean_ms": value(avg), "sample_stddev_ms": value(sd), "p50_ms": value(p50), "p95_ms": value(p95),
		"clock_basis": "DATABASE_RECORDED_RECEIPT_TIMESTAMPS", "interpretation": "SERVER_ATTESTATION_INTERVAL_NOT_PHYSICAL_TRAVEL_OR_OPERATION_TIME", "sewing_credit": false})
}
