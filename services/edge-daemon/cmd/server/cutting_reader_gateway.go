// Cutting reader gateway ingress. An externally provisioned Ed25519 key
// authenticates the reporting gateway; it does NOT certify antenna physics.
package main

import (
	"context"
	"crypto/ed25519"
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

type cutReaderEvent struct {
	Version     string `json:"version"`
	TenantID    string `json:"tenant_id"`
	PlantID     string `json:"plant_id"`
	GatewayID   string `json:"gateway_id"`
	StationID   string `json:"station_id"`
	EventID     string `json:"event_id"`
	ObservedAt  string `json:"observed_at"`
	DraftID     string `json:"draft_id"`
	SpreadID    string `json:"spread_id"`
	LabelCode   string `json:"label_code"`
	EPCHex      string `json:"epc_hex"`
	AntennaPort int    `json:"antenna_port"`
}
type cutReaderEnvelope struct {
	Event        cutReaderEvent `json:"event"`
	SignatureHex string         `json:"signature_hex"`
}

func cutReaderPayload(v cutReaderEvent) ([]byte, error) { return json.Marshal(v) }
func validReaderEvent(v cutReaderEvent, now time.Time) error {
	if v.Version != "CUT_READER_V1" || !passportCode.MatchString(v.GatewayID) || len(v.GatewayID) > 64 || len(v.GatewayID) < 3 ||
		!passportCode.MatchString(v.StationID) || len(v.StationID) > 64 || len(v.StationID) < 2 ||
		!storesRequestKey.MatchString(v.EventID) || len(v.EventID) < 12 || len(v.EventID) > 64 ||
		v.TenantID == "" || len(v.TenantID) > 64 || v.PlantID == "" || len(v.PlantID) > 64 ||
		!cutDraftUUID.MatchString(v.DraftID) || !cutDraftUUID.MatchString(v.SpreadID) ||
		len(v.LabelCode) != 29 || !strings.HasPrefix(v.LabelCode, "CB08-") || v.AntennaPort < 1 || v.AntennaPort > 64 {
		return errors.New("ERR_CUT_READER_FIELDS")
	}
	for _, c := range v.LabelCode[5:] {
		if !(c >= '0' && c <= '9' || c >= 'A' && c <= 'F') {
			return errors.New("ERR_CUT_READER_LABEL")
		}
	}
	epc, e := canonicalEPCHex(v.EPCHex)
	if e != nil || epc != v.EPCHex {
		return errors.New("ERR_CUT_READER_EPC")
	}
	t, e := time.Parse(time.RFC3339Nano, v.ObservedAt)
	if e != nil || t.After(now.Add(30*time.Second)) || t.Before(now.Add(-2*time.Minute)) {
		return errors.New("ERR_CUT_READER_STALE_EVENT")
	}
	return nil
}
func readerSignatureOK(pub []byte, v cutReaderEvent, signatureHex string) bool {
	if len(pub) != ed25519.PublicKeySize || len(signatureHex) != 128 {
		return false
	}
	sig, e := hex.DecodeString(signatureHex)
	if e != nil || len(sig) != ed25519.SignatureSize {
		return false
	}
	b, e := cutReaderPayload(v)
	return e == nil && ed25519.Verify(ed25519.PublicKey(pub), b, sig)
}
func (s *EdgeServer) handleCuttingReaderEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", 405)
		return
	}
	if s.db == nil {
		docJSON(w, 503, map[string]string{"error": "ERR_CUT_READER_DB_UNAVAILABLE"})
		return
	}
	// The operator gate is NOT a device identity. Gateway key enrollment is an
	// administrative, out-of-band action; the private key is never sent to Agiparel.
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "Expected JSON", 415)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var req cutReaderEnvelope
	if e := dec.Decode(&req); e != nil {
		docJSON(w, 400, map[string]string{"error": "ERR_CUT_READER_JSON"})
		return
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		docJSON(w, 400, map[string]string{"error": "ERR_CUT_READER_EXTRA_JSON"})
		return
	}
	if e := validReaderEvent(req.Event, time.Now().UTC()); e != nil {
		docJSON(w, 422, map[string]string{"error": e.Error()})
		return
	}
	tenant, e := intakeTenant()
	if e != nil || tenant != req.Event.TenantID {
		docJSON(w, 403, map[string]string{"error": "ERR_CUT_READER_TENANT"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	id, replay, e := s.persistReaderEvent(ctx, req)
	if e != nil {
		code := e.Error()
		status := 409
		if !strings.HasPrefix(code, "ERR_CUT_") {
			code = "ERR_CUT_READER_INTERNAL"
			status = 503
		}
		docJSON(w, status, map[string]string{"error": code})
		return
	}
	status := 201
	if replay {
		status = 200
	}
	docJSON(w, status, map[string]any{"id": id, "replayed": replay, "state": "SIGNED_GATEWAY_REPORT_UNVERIFIED_RF", "production_released": false})
}
func (s *EdgeServer) persistReaderEvent(ctx context.Context, req cutReaderEnvelope) (string, bool, error) {
	v := req.Event
	if e := validReaderEvent(v, time.Now().UTC()); e != nil {
		return "", false, e
	}
	tx, e := s.docTransaction(ctx, v.TenantID)
	if e != nil {
		return "", false, e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1),hashtext($2))`, v.TenantID, "CUT08C:READER_EVENTS"); e != nil {
		return "", false, e
	}
	var key []byte
	var plant, station string
	var enabled bool
	e = tx.QueryRowContext(ctx, `SELECT public_key,plant_id,station_id,enabled FROM cut_reader_gateways_v8 WHERE tenant_id=$1 AND gateway_id=$2`, v.TenantID, v.GatewayID).Scan(&key, &plant, &station, &enabled)
	if errors.Is(e, sql.ErrNoRows) {
		return "", false, errors.New("ERR_CUT_READER_NOT_ENROLLED")
	}
	if e != nil {
		return "", false, e
	}
	if !enabled || plant != v.PlantID || station != v.StationID {
		return "", false, errors.New("ERR_CUT_READER_NOT_AUTHORIZED")
	}
	if !readerSignatureOK(key, v, req.SignatureHex) {
		return "", false, errors.New("ERR_CUT_READER_SIGNATURE")
	}
	payload, _ := cutReaderPayload(v)
	digest := sha256.Sum256(payload)
	signature, _ := hex.DecodeString(req.SignatureHex)
	signDigest := sha256.Sum256(signature)
	hash := hex.EncodeToString(digest[:])
	sigHash := hex.EncodeToString(signDigest[:])
	var id, storedHash string
	e = tx.QueryRowContext(ctx, `SELECT id::text,event_sha256 FROM cut_reader_events_v8 WHERE tenant_id=$1 AND gateway_id=$2 AND event_id=$3`, v.TenantID, v.GatewayID, v.EventID).Scan(&id, &storedHash)
	if e == nil {
		if storedHash != hash {
			return "", false, errors.New("ERR_CUT_READER_REPLAY_CONFLICT")
		}
		if e = tx.Commit(); e != nil {
			return "", false, e
		}
		return id, true, nil
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return "", false, e
	}
	// Do not credit tags solely from gateway input: their EPC and bundle must
	// already have an independently signed human witness on the same spread.
	var matched int
	e = tx.QueryRowContext(ctx, `SELECT count(*)::int FROM cut_spread_reports_v8 s
 JOIN cut_bundle_scan_witnesses_v8 w ON w.tenant_id=s.tenant_id AND w.spread_report_id=s.id AND w.lay_draft_id=s.lay_draft_id
 WHERE s.tenant_id=$1 AND s.id=$2::uuid AND s.lay_draft_id=$3::uuid AND w.label_code=$4 AND w.epc_hex=$5 AND w.plant_id=$6`, v.TenantID, v.SpreadID, v.DraftID, v.LabelCode, v.EPCHex, v.PlantID).Scan(&matched)
	if e != nil {
		return "", false, e
	}
	if matched != 1 {
		return "", false, errors.New("ERR_CUT_READER_HUMAN_WITNESS_MISSING")
	}
	var released int
	e = tx.QueryRowContext(ctx, `SELECT count(*)::int FROM cut_sewing_releases_v8 WHERE tenant_id=$1 AND spread_report_id=$2::uuid`, v.TenantID, v.SpreadID).Scan(&released)
	if e != nil {
		return "", false, e
	}
	if released != 0 {
		return "", false, errors.New("ERR_CUT_READER_SPREAD_RELEASED")
	}
	e = tx.QueryRowContext(ctx, `INSERT INTO cut_reader_events_v8(tenant_id,plant_id,gateway_id,station_id,event_id,observed_at,lay_draft_id,spread_report_id,label_code,epc_hex,antenna_port,event_sha256,signature_sha256)
 VALUES($1,$2,$3,$4,$5,$6::timestamptz,$7::uuid,$8::uuid,$9,$10,$11,$12,$13) RETURNING id::text`, v.TenantID, v.PlantID, v.GatewayID, v.StationID, v.EventID, v.ObservedAt, v.DraftID, v.SpreadID, v.LabelCode, v.EPCHex, v.AntennaPort, hash, sigHash).Scan(&id)
	if e != nil {
		return "", false, e
	}
	if e = tx.Commit(); e != nil {
		return "", false, e
	}
	return id, false, nil
}
