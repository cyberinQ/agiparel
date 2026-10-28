// Author: Sunil Jain, 2026-10-28.
// Production-only, evidence-gated sewing observation and worker-attested completion.
// Gateway signatures identify an enrolled source, not independently verified RF physics.
// Completions are provisional work attestations, not QC, payroll, shipment or margin credit.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/attestation"
)

const sewProdVersion = "SEW_PRODUCTION_OBSERVATION_V1"
const sewProdGatewayState = "SIGNED_GATEWAY_REPORT_NOT_INDEPENDENT_RF_PROOF"
const sewProdCompletionState = "WORKER_ATTESTED_PENDING_QUALITY_ACCEPTANCE"

type sewProdObservation struct {
	Version    string `json:"version"`
	TenantID   string `json:"tenant_id"`
	PlantID    string `json:"plant_id"`
	GatewayID  string `json:"gateway_id"`
	StationID  string `json:"station_id"`
	EventID    string `json:"event_id"`
	LayID      string `json:"lay_id"`
	LabelCode  string `json:"label_code"`
	EPCHex     string `json:"epc_hex"`
	ObservedAt string `json:"observed_at"`
}
type sewProdObservationEnvelope struct {
	Event        sewProdObservation `json:"event"`
	SignatureHex string             `json:"signature_hex"`
}
type sewProdCompleteCommand struct {
	CommandID      string   `json:"command_id"`
	LayID          string   `json:"lay_id"`
	PlantID        string   `json:"plant_id"`
	StationID      string   `json:"station_id"`
	RouteID        string   `json:"route_id"`
	MarkerPosition int      `json:"marker_position"`
	PlyGroup       int      `json:"ply_group"`
	ObservationIDs []string `json:"observation_ids"`
}
type sewProdCompleteRequest struct {
	Command sewProdCompleteCommand `json:"command"`
}

type sewProdAPI struct {
	server      *EdgeServer
	verifier    *attestation.Verifier
	enabled     bool
	configError string
}

func newSewProdAPI(s *EdgeServer) *sewProdAPI {
	p := &sewProdAPI{server: s}
	if os.Getenv("AGIPAREL_SEWING_PRODUCTION_ENABLED") != "true" {
		p.configError = "ERR_SEW_PROD_DISABLED"
		return p
	}
	if os.Getenv("ENV") != "production" && os.Getenv("ENV") != "staging" {
		p.configError = "ERR_SEW_PROD_ENVIRONMENT"
		return p
	}
	if s == nil || s.db == nil {
		p.configError = "ERR_SEW_PROD_DATABASE"
		return p
	}
	pub, err := attestation.DecodePublicKey(os.Getenv("AGIPAREL_IDP_ED25519_PUBLIC_KEY_B64"))
	if err != nil {
		p.configError = "ERR_SEW_PROD_IDP"
		return p
	}
	kid, issuer, audience := os.Getenv("AGIPAREL_IDP_KID"), os.Getenv("AGIPAREL_IDP_ISSUER"), os.Getenv("AGIPAREL_IDP_AUDIENCE")
	if kid == "" || issuer == "" || audience == "" {
		p.configError = "ERR_SEW_PROD_IDP"
		return p
	}
	p.verifier = &attestation.Verifier{PublicKeys: map[string]ed25519.PublicKey{kid: pub}, Issuer: issuer, Audience: audience, MaxLifetime: 5 * time.Minute}
	p.enabled = true
	return p
}
func (p *sewProdAPI) ready(w http.ResponseWriter) bool {
	if p == nil || !p.enabled {
		code := "ERR_SEW_PROD_DISABLED"
		if p != nil {
			code = p.configError
		}
		docJSON(w, 503, map[string]string{"error": code})
		return false
	}
	return true
}
func sewProdInvalid(code string) error { return sewFail(422, code) }
func sewProdValidObservation(v sewProdObservation, now time.Time) error {
	if v.Version != sewProdVersion || !sewFields(v.TenantID, v.PlantID, v.GatewayID, v.StationID) ||
		!sewCommandID.MatchString(v.EventID) || !sewUUID(v.LayID) || len(v.LabelCode) != 29 || !strings.HasPrefix(v.LabelCode, "CB08-") {
		return sewProdInvalid("ERR_SEW_PROD_OBSERVATION_FIELDS")
	}
	for _, c := range v.LabelCode[5:] {
		if !(c >= '0' && c <= '9' || c >= 'A' && c <= 'F') {
			return sewProdInvalid("ERR_SEW_PROD_LABEL")
		}
	}
	epc, e := canonicalEPCHex(v.EPCHex)
	if e != nil || epc != v.EPCHex {
		return sewProdInvalid("ERR_SEW_PROD_EPC")
	}
	observed, e := time.Parse(time.RFC3339Nano, v.ObservedAt)
	if e != nil || observed.After(now.Add(2*time.Minute)) {
		return sewProdInvalid("ERR_SEW_PROD_OBSERVATION_CLOCK")
	}
	return nil
}
func sewProdAgeLimit() (time.Duration, error) {
	raw := os.Getenv("AGIPAREL_SEWING_MAX_OBSERVATION_AGE")
	if raw == "" {
		return 48 * time.Hour, nil
	}
	d, e := time.ParseDuration(raw)
	if e != nil || d < time.Minute || d > 30*24*time.Hour {
		return 0, errors.New("ERR_SEW_PROD_AGE_CONFIG")
	}
	return d, nil
}
func sewProdValidCommand(c sewProdCompleteCommand) error {
	if !sewCommandID.MatchString(c.CommandID) || !sewUUID(c.LayID) || !sewUUID(c.RouteID) || !sewFields(c.PlantID, c.StationID) || c.MarkerPosition < 1 || c.PlyGroup < 1 || len(c.ObservationIDs) == 0 {
		return sewProdInvalid("ERR_SEW_PROD_COMPLETION_FIELDS")
	}
	seen := map[string]bool{}
	for _, id := range c.ObservationIDs {
		if !sewUUID(id) || seen[id] {
			return sewProdInvalid("ERR_SEW_PROD_OBSERVATION_IDS")
		}
		seen[id] = true
	}
	return nil
}
func sewProdCommandHash(c sewProdCompleteCommand) string {
	b, _ := json.Marshal(c)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func sewProdChallenge(c sewProdCompleteCommand) string {
	return "SEW_PROD_COMPLETE_V1:" + sewProdCommandHash(c)
}
func sewProdHashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// Vendor-independent signing: fixed newline-delimited, ASCII-only fields.
// All included values are validated against newline-free code/hex/time formats.
func sewProdEventBytes(v sewProdObservation) []byte {
	return []byte("AGIPAREL_SEW_READER_V1\n" + strings.Join([]string{
		v.Version, v.TenantID, v.PlantID, v.GatewayID, v.StationID,
		v.EventID, v.LayID, v.LabelCode, v.EPCHex, v.ObservedAt,
	}, "\n") + "\n")
}
func sewProdSignatureHash(s string) string {
	raw, _ := hex.DecodeString(s)
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}
func sewProdSigned(v sewProdObservation, signature string, pub []byte) bool {
	if len(pub) != ed25519.PublicKeySize || len(signature) != ed25519.SignatureSize*2 {
		return false
	}
	raw, e := hex.DecodeString(signature)
	return e == nil && ed25519.Verify(ed25519.PublicKey(pub), sewProdEventBytes(v), raw)
}

// Resource bounds protect the ingress, never encode a factory-specific part count.
func sewProdStrictJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	kind, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || kind != "application/json" {
		docJSON(w, 415, map[string]string{"error": "ERR_SEW_PROD_CONTENT_TYPE"})
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e = d.Decode(dst); e != nil {
		docJSON(w, 400, map[string]string{"error": "ERR_SEW_PROD_JSON"})
		return false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		docJSON(w, 400, map[string]string{"error": "ERR_SEW_PROD_EXTRA_JSON"})
		return false
	}
	return true
}
func sewProdReply(w http.ResponseWriter, e error) {
	var sf *sewSimError
	if errors.As(e, &sf) {
		docJSON(w, sf.status, map[string]string{"error": sf.code})
		return
	}
	// Never disclose database queries or private provisioning details.
	docJSON(w, 503, map[string]string{"error": "ERR_SEW_PROD_UNAVAILABLE"})
}
func (p *sewProdAPI) challenge(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var req sewProdCompleteRequest
	if !sewProdStrictJSON(w, r, &req) {
		return
	}
	if e := sewProdValidCommand(req.Command); e != nil {
		sewProdReply(w, e)
		return
	}
	docJSON(w, 200, map[string]string{"challenge": sewProdChallenge(req.Command), "action": "sewing.complete", "required_role": "sewing_operator", "required_auth": "webauthn_or_fido2"})
}
func (p *sewProdAPI) observation(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var req sewProdObservationEnvelope
	if !sewProdStrictJSON(w, r, &req) {
		return
	}
	v := req.Event
	if e := sewProdValidObservation(v, time.Now().UTC()); e != nil {
		sewProdReply(w, e)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	id, replay, e := p.persistObservation(ctx, req)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	status := 201
	if replay {
		status = 200
	}
	docJSON(w, status, map[string]any{"id": id, "replayed": replay, "evidence_state": sewProdGatewayState, "production_credit": false})
}
func (p *sewProdAPI) persistObservation(ctx context.Context, req sewProdObservationEnvelope) (string, bool, error) {
	v := req.Event
	tx, e := p.server.docTransaction(ctx, v.TenantID)
	if e != nil {
		return "", false, e
	}
	defer tx.Rollback()
	// Lock only this gateway/event: no whole-factory throughput bottleneck.
	if e = sewTxLock(ctx, tx, v.TenantID, "PROD_OBSERVATION:"+v.GatewayID+":"+v.EventID); e != nil {
		return "", false, e
	}
	var key []byte
	var plant, station string
	var enabled bool
	e = tx.QueryRowContext(ctx, `SELECT public_key,plant_id,station_id,enabled FROM sew_prod_gateways_v10 WHERE tenant_id=$1 AND gateway_id=$2`, v.TenantID, v.GatewayID).Scan(&key, &plant, &station, &enabled)
	if errors.Is(e, sql.ErrNoRows) {
		return "", false, sewFail(403, "ERR_SEW_PROD_GATEWAY_NOT_ENROLLED")
	}
	if e != nil {
		return "", false, e
	}
	if !enabled || plant != v.PlantID || station != v.StationID || !sewProdSigned(v, req.SignatureHex, key) {
		return "", false, sewFail(403, "ERR_SEW_PROD_GATEWAY_SIGNATURE_OR_SCOPE")
	}
	hash := sewProdHashString(string(sewProdEventBytes(v)))
	var id, stored string
	e = tx.QueryRowContext(ctx, `SELECT id::text,event_sha256 FROM sew_prod_observations_v10 WHERE tenant_id=$1 AND gateway_id=$2 AND event_id=$3`, v.TenantID, v.GatewayID, v.EventID).Scan(&id, &stored)
	if e == nil {
		if stored != hash {
			return "", false, sewFail(409, "ERR_SEW_PROD_EVENT_REUSE")
		}
		return id, true, tx.Commit()
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return "", false, e
	}
	ageLimit, e := sewProdAgeLimit()
	if e != nil {
		return "", false, e
	}
	when, _ := time.Parse(time.RFC3339Nano, v.ObservedAt)
	if when.Before(time.Now().Add(-ageLimit)) {
		return "", false, sewFail(422, "ERR_SEW_PROD_OBSERVATION_EXPIRED")
	}
	var release, part string
	var releaseAt time.Time
	var pos, group int
	e = tx.QueryRowContext(ctx, `SELECT rel.id::text,b.marker_position,b.ply_group,b.part_code,rel.created_at FROM cut_bundle_drafts_v8 b
 JOIN cut_bundle_scan_witnesses_v8 witness ON witness.tenant_id=b.tenant_id AND witness.lay_draft_id=b.lay_draft_id AND witness.label_code=b.label_code
 JOIN cut_sewing_releases_v8 rel ON rel.tenant_id=witness.tenant_id AND rel.spread_report_id=witness.spread_report_id
 WHERE b.tenant_id=$1 AND b.lay_draft_id=$2::uuid AND b.label_code=$3 AND witness.epc_hex=$4
 AND witness.plant_id=$5 AND rel.plant_id=$5`, v.TenantID, v.LayID, v.LabelCode, v.EPCHex, v.PlantID).Scan(&release, &pos, &group, &part, &releaseAt)
	if errors.Is(e, sql.ErrNoRows) {
		return "", false, sewFail(409, "ERR_SEW_PROD_SIGNED_CUT_EVIDENCE_REQUIRED")
	}
	if e != nil {
		return "", false, e
	}
	if when.Before(releaseAt.Add(-2 * time.Minute)) {
		return "", false, sewFail(409, "ERR_SEW_PROD_OBSERVED_BEFORE_RELEASE")
	}
	e = tx.QueryRowContext(ctx, `INSERT INTO sew_prod_observations_v10(tenant_id,event_id,gateway_id,plant_id,station_id,lay_draft_id,release_id,label_code,epc_hex,marker_position,ply_group,part_code,observed_at,event_sha256,signature_sha256)
 VALUES($1,$2,$3,$4,$5,$6::uuid,$7::uuid,$8,$9,$10,$11,$12,$13::timestamptz,$14,$15) RETURNING id::text`, v.TenantID, v.EventID, v.GatewayID, v.PlantID, v.StationID, v.LayID, release, v.LabelCode, v.EPCHex, pos, group, part, v.ObservedAt, hash, sewProdSignatureHash(req.SignatureHex)).Scan(&id)
	if e != nil {
		return "", false, e
	}
	return id, false, tx.Commit()
}
func (p *sewProdAPI) completion(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var req sewProdCompleteRequest
	if !sewProdStrictJSON(w, r, &req) {
		return
	}
	c := req.Command
	if e := sewProdValidCommand(c); e != nil {
		sewProdReply(w, e)
		return
	}
	token, e := bearer(r)
	if e != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_SEW_PROD_BEARER_REQUIRED"})
		return
	}
	actor, e := p.verifier.Verify(token, "sewing.complete", sewProdChallenge(c), "sewing_operator", true)
	if e != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_SEW_PROD_WORKER_ASSERTION"})
		return
	}
	if actor.PlantID != c.PlantID || !sewFields(actor.TenantID) || len(actor.Subject) > 128 || len(actor.JWTID) > 255 {
		docJSON(w, 403, map[string]string{"error": "ERR_SEW_PROD_WORKER_PLANT_SCOPE"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	id, replay, count, e := p.persistCompletion(ctx, c, actor, token)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	status := 201
	if replay {
		status = 200
	}
	docJSON(w, status, map[string]any{"id": id, "replayed": replay, "nominal_ply_count": count, "evidence_state": sewProdCompletionState, "quality_accepted": false, "payroll_credit": false, "shipping_released": false})
}
func (p *sewProdAPI) persistCompletion(ctx context.Context, c sewProdCompleteCommand, actor attestation.Claims, token string) (string, bool, int, error) {
	tx, e := p.server.docTransaction(ctx, actor.TenantID)
	if e != nil {
		return "", false, 0, e
	}
	defer tx.Rollback()
	if e = workforceChapterRequire(ctx, tx, actor, "sewing.complete"); e != nil {
		return "", false, 0, e
	}
	if e = sewTxLock(ctx, tx, actor.TenantID, "PROD_COMPLETE:"+c.RouteID+":"+strconv.Itoa(c.MarkerPosition)+":"+strconv.Itoa(c.PlyGroup)); e != nil {
		return "", false, 0, e
	}
	hash := sewProdCommandHash(c)
	var id, stored string
	var count int
	e = tx.QueryRowContext(ctx, `SELECT id::text,command_sha256,nominal_ply_count FROM sew_prod_completions_v10 WHERE tenant_id=$1 AND command_id=$2`, actor.TenantID, c.CommandID).Scan(&id, &stored, &count)
	if e == nil {
		if stored != hash {
			return "", false, 0, sewFail(409, "ERR_SEW_PROD_COMMAND_REUSE")
		}
		return id, true, count, tx.Commit()
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return "", false, 0, e
	}
	// Exact route, release, station and part-lineage validation is repeated in the DB trigger.
	var release string
	e = tx.QueryRowContext(ctx, `SELECT rel.id::text FROM sew_prod_routes_v10 route
 JOIN cut_sewing_releases_v8 rel ON rel.tenant_id=route.tenant_id AND rel.lay_draft_id=route.lay_draft_id AND rel.plant_id=route.plant_id
 WHERE route.tenant_id=$1 AND route.id=$2::uuid AND route.lay_draft_id=$3::uuid AND route.plant_id=$4 AND route.station_id=$5 AND route.enabled`, actor.TenantID, c.RouteID, c.LayID, c.PlantID, c.StationID).Scan(&release)
	if errors.Is(e, sql.ErrNoRows) {
		return "", false, 0, sewFail(409, "ERR_SEW_PROD_APPROVED_ROUTE_AND_RELEASE_REQUIRED")
	}
	if e != nil {
		return "", false, 0, e
	}
	var workerAssigned bool
	e = tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM sew_prod_worker_stations_v10 WHERE tenant_id=$1 AND plant_id=$2 AND station_id=$3 AND worker_subject=$4 AND enabled)`, actor.TenantID, c.PlantID, c.StationID, actor.Subject).Scan(&workerAssigned)
	if e != nil {
		return "", false, 0, e
	}
	if !workerAssigned {
		return "", false, 0, sewFail(403, "ERR_SEW_PROD_WORKER_NOT_ASSIGNED")
	}
	// DB trigger checks ALL required parts and their identical ply/size/shade range.
	e = tx.QueryRowContext(ctx, `SELECT max(b.ply_end)-min(b.ply_start)+1 FROM sew_prod_observations_v10 o
 JOIN cut_bundle_drafts_v8 b ON b.tenant_id=o.tenant_id AND b.lay_draft_id=o.lay_draft_id AND b.label_code=o.label_code
 WHERE o.tenant_id=$1 AND o.id::text=ANY($2::text[]) AND o.lay_draft_id=$3::uuid AND o.plant_id=$4 AND o.station_id=$5 AND o.marker_position=$6 AND o.ply_group=$7`, actor.TenantID, pgTextArray(c.ObservationIDs), c.LayID, c.PlantID, c.StationID, c.MarkerPosition, c.PlyGroup).Scan(&count)
	if e != nil {
		return "", false, 0, e
	}
	if count <= 0 {
		return "", false, 0, sewFail(409, "ERR_SEW_PROD_GENEALOGY")
	}
	e = tx.QueryRowContext(ctx, `INSERT INTO sew_prod_completions_v10(tenant_id,command_id,command_sha256,lay_draft_id,release_id,route_id,plant_id,station_id,marker_position,ply_group,observation_ids,nominal_ply_count,worker_subject,worker_proof_id,assertion_sha256)
 VALUES($1,$2,$3,$4::uuid,$5::uuid,$6::uuid,$7,$8,$9,$10,$11::uuid[],$12,$13,$14,$15) RETURNING id::text`, actor.TenantID, c.CommandID, hash, c.LayID, release, c.RouteID, c.PlantID, c.StationID, c.MarkerPosition, c.PlyGroup, pgUUIDArray(c.ObservationIDs), count, actor.Subject, actor.JWTID, sewProdHashString(token)).Scan(&id)
	if e != nil { // Typically a legitimate conflict, unmet predecessor, or invalid evidence.
		return "", false, 0, sewFail(409, "ERR_SEW_PROD_CONFLICT_OR_MISSING_EVIDENCE")
	}
	return id, false, count, tx.Commit()
}
func pgUUIDArray(xs []string) string { return "{" + strings.Join(xs, ",") + "}" }
func registerSewProdRoutes(mux *http.ServeMux, s *EdgeServer) {
	p := newSewProdAPI(s)
	if !p.enabled { // Do not expose any write capability on a misconfigured instance.
		fmt.Printf("[sew-production-v10] disabled (fail closed): %s\n", p.configError)
	}
	mux.HandleFunc("/api/v1/sewing/production/observations", p.observation)
	mux.HandleFunc("/api/v1/sewing/production/completions/challenge", p.challenge)
	mux.HandleFunc("/api/v1/sewing/production/completions", p.completion)
}
