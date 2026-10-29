// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// F3.20B-D: signed gateway carton scans, independently signed container seal,
// onboarded-carrier receipt and internal ASN draft. No shipment credit is issued.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/attestation"
)

const dockV34Authority = "VERIFIED_EVIDENCE_CHAIN_NOT_CARRIER_API_DELIVERY_BUYER_ACCEPTANCE_OR_SHIPMENT_CREDIT"

type dockScanV34 struct {
	Version    string `json:"version"`
	TenantID   string `json:"tenant_id"`
	PlantID    string `json:"plant_id"`
	ManifestID string `json:"manifest_id"`
	CartonID   string `json:"carton_id"`
	CartonCode string `json:"carton_code"`
	GatewayID  string `json:"gateway_id"`
	EventID    string `json:"event_id"`
	ObservedAt string `json:"observed_at"`
}
type dockScanEnvelopeV34 struct {
	Event        dockScanV34 `json:"event"`
	SignatureHex string      `json:"signature_hex"`
}

func dockScanBytesV34(e dockScanV34) []byte {
	return []byte("AGIPAREL_DOCK_GATEWAY_V34\n" + strings.Join([]string{
		e.Version, e.TenantID, e.PlantID, e.ManifestID, e.CartonID, e.CartonCode, e.GatewayID, e.EventID, e.ObservedAt,
	}, "\n") + "\n")
}
func dockScanValidV34(e dockScanV34, now time.Time) bool {
	if e.Version != "V34" || !sewFields(e.TenantID, e.PlantID, e.CartonCode, e.GatewayID) ||
		len(e.CartonCode) > 80 || len(e.GatewayID) > 64 || !sewUUID(e.ManifestID) || !sewUUID(e.CartonID) ||
		!sewCommandID.MatchString(e.EventID) {
		return false
	}
	t, err := time.Parse(time.RFC3339Nano, e.ObservedAt)
	return err == nil && !t.After(now.Add(2*time.Minute)) && !t.Before(now.Add(-15*time.Minute))
}

type dockCommandV34 struct {
	Kind             string `json:"kind"`
	CommandID        string `json:"command_id"`
	PlantID          string `json:"plant_id"`
	ManifestID       string `json:"manifest_id"`
	SealID           string `json:"seal_id,omitempty"`
	CarrierReceiptID string `json:"carrier_receipt_id,omitempty"`
	ContainerRef     string `json:"container_ref,omitempty"`
	SealCode         string `json:"seal_code,omitempty"`
	ScannedCartons   int    `json:"scanned_cartons,omitempty"`
	CarrierCode      string `json:"carrier_code,omitempty"`
	ReceiptReference string `json:"receipt_reference,omitempty"`
	ASNReference     string `json:"asn_reference,omitempty"`
	PayloadSHA256    string `json:"payload_sha256,omitempty"`
	Note             string `json:"note"`
}
type dockCommandEnvelopeV34 struct {
	Command dockCommandV34 `json:"command"`
}

func dockCommandPolicyV34(c dockCommandV34) (string, string, bool) {
	if !sewCommandID.MatchString(c.CommandID) || !sewFields(c.PlantID) || !sewUUID(c.ManifestID) ||
		len(strings.TrimSpace(c.Note)) < 20 || len(c.Note) > 800 || strings.ContainsRune(c.Note, 0) {
		return "", "", false
	}
	switch c.Kind {
	case "CONTAINER_SEAL":
		if !sewFields(c.ContainerRef, c.SealCode) || len(c.ContainerRef) > 80 || len(c.SealCode) > 80 ||
			c.ScannedCartons <= 0 || c.ScannedCartons > 1000000 || c.SealID != "" || c.CarrierReceiptID != "" ||
			c.CarrierCode != "" || c.ReceiptReference != "" || c.ASNReference != "" || c.PayloadSHA256 != "" {
			return "", "", false
		}
		return "packing.dock.seal", "dispatch_supervisor", true
	case "CARRIER_RECEIPT":
		if !sewUUID(c.SealID) || !sewFields(c.CarrierCode, c.ReceiptReference) ||
			len(c.CarrierCode) > 64 || len(c.ReceiptReference) > 128 || len(c.ReceiptReference) < 6 ||
			c.ContainerRef != "" || c.SealCode != "" || c.ScannedCartons != 0 || c.CarrierReceiptID != "" ||
			c.ASNReference != "" || c.PayloadSHA256 != "" {
			return "", "", false
		}
		return "packing.dock.carrier", "carrier_agent", true
	case "ASN_DRAFT":
		if !sewUUID(c.CarrierReceiptID) || !sewFields(c.ASNReference) || len(c.ASNReference) < 6 || len(c.ASNReference) > 128 ||
			!f317bHash.MatchString(c.PayloadSHA256) || c.SealID != "" || c.CarrierCode != "" ||
			c.ReceiptReference != "" || c.ContainerRef != "" || c.SealCode != "" || c.ScannedCartons != 0 {
			return "", "", false
		}
		return "packing.dock.asn.prepare", "dispatch_coordinator", true
	}
	return "", "", false
}
func dockChallengeV34(c dockCommandV34) string { return "DOCK_HANDOVER_V34:" + sewDigest(c) }
func dockPathValidV34(path, kind string) bool {
	switch path {
	case "/api/v1/packing/dock/seals/challenge", "/api/v1/packing/dock/seals":
		return kind == "CONTAINER_SEAL"
	case "/api/v1/packing/dock/carrier-receipts/challenge", "/api/v1/packing/dock/carrier-receipts":
		return kind == "CARRIER_RECEIPT"
	case "/api/v1/packing/dock/asn-drafts/challenge", "/api/v1/packing/dock/asn-drafts":
		return kind == "ASN_DRAFT"
	}
	return false
}

type dockAPIv34 struct {
	server   *EdgeServer
	evidence *factoryEvidenceAPIv32
	enabled  bool
}

func newDockAPIv34(s *EdgeServer) *dockAPIv34 {
	p := &dockAPIv34{server: s, evidence: newFactoryEvidenceAPIv32(s)}
	p.enabled = p.evidence.enabled && os.Getenv("AGIPAREL_FACTORY_RELEASE_V33_ENABLED") == "true" &&
		os.Getenv("AGIPAREL_DOCK_V34_ENABLED") == "true"
	return p
}
func (p *dockAPIv34) ready(w http.ResponseWriter) bool {
	if !p.enabled {
		docJSON(w, 503, map[string]string{"error": "ERR_F34_DOCK_DISABLED"})
		return false
	}
	return true
}
func (p *dockAPIv34) scan(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var input dockScanEnvelopeV34
	if !sewProdStrictJSON(w, r, &input) {
		return
	}
	if !dockScanValidV34(input.Event, time.Now().UTC()) {
		docJSON(w, 422, map[string]string{"error": "ERR_F34_SCAN_FIELDS_OR_TIME"})
		return
	}
	sig, err := hex.DecodeString(input.SignatureHex)
	if err != nil || len(sig) != ed25519.SignatureSize {
		docJSON(w, 422, map[string]string{"error": "ERR_F34_SCAN_SIGNATURE"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	id, replayed, err := p.persistScan(ctx, input, sig)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	status := http.StatusCreated
	if replayed {
		status = http.StatusOK
	}
	docJSON(w, status, map[string]any{"id": id, "replayed": replayed, "authority": dockV34Authority, "shipment_credit": false})
}
func (p *dockAPIv34) persistScan(ctx context.Context, input dockScanEnvelopeV34, sig []byte) (string, bool, error) {
	e := input.Event
	tx, err := p.server.docTransaction(ctx, e.TenantID)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SET LOCAL ROLE agiparel_app`); err != nil {
		return "", false, err
	}
	// Limit database lock waits and individual statements independently of the
	// request deadline. Deadlocks and blocked admissions must fail closed.
	if _, err = tx.ExecContext(ctx, `SET LOCAL lock_timeout = '2000ms'`); err != nil {
		return "", false, err
	}
	if _, err = tx.ExecContext(ctx, `SET LOCAL statement_timeout = '12000ms'`); err != nil {
		return "", false, err
	}
	if err = sewTxLock(ctx, tx, e.TenantID, "F34:SCAN:"+e.GatewayID+":"+e.EventID); err != nil {
		return "", false, err
	}
	var key []byte
	var enabled bool
	var plant string
	err = tx.QueryRowContext(ctx, `SELECT plant_id,public_key,enabled FROM dock_gateways_v34 WHERE tenant_id=$1 AND gateway_id=$2`, e.TenantID, e.GatewayID).Scan(&plant, &key, &enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, sewFail(403, "ERR_F34_DOCK_GATEWAY_NOT_ENROLLED")
	}
	if err != nil {
		return "", false, err
	}
	if !enabled || plant != e.PlantID || len(key) != ed25519.PublicKeySize || !ed25519.Verify(ed25519.PublicKey(key), dockScanBytesV34(e), sig) {
		return "", false, sewFail(403, "ERR_F34_GATEWAY_SIGNATURE_OR_PLANT")
	}
	hash := sewProdHashString(string(dockScanBytesV34(e)))
	keyDigest := sha256.Sum256(key)
	var id, existing string
	err = tx.QueryRowContext(ctx, `SELECT id::text,event_sha256 FROM dock_carton_scans_v34 WHERE tenant_id=$1 AND gateway_id=$2 AND event_id=$3`, e.TenantID, e.GatewayID, e.EventID).Scan(&id, &existing)
	if err == nil {
		if existing != hash {
			return "", false, sewFail(409, "ERR_F34_SCAN_REPLAY_CONFLICT")
		}
		return id, true, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, err
	}
	// Recheck age after any lock wait; backend device time is not trusted.
	if !dockScanValidV34(e, time.Now().UTC()) {
		return "", false, sewFail(422, "ERR_F34_SCAN_EXPIRED")
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO dock_carton_scans_v34
      (tenant_id,plant_id,manifest_id,carton_id,carton_code,gateway_id,event_id,observed_at,event_sha256,signature_sha256,gateway_key_sha256)
      VALUES($1,$2,$3::uuid,$4::uuid,$5,$6,$7,$8::timestamptz,$9,$10,$11) RETURNING id::text`,
		e.TenantID, e.PlantID, e.ManifestID, e.CartonID, e.CartonCode, e.GatewayID, e.EventID, e.ObservedAt, hash, sewProdSignatureHash(input.SignatureHex), hex.EncodeToString(keyDigest[:])).Scan(&id)
	if err != nil {
		return "", false, sewFail(409, "ERR_F34_DOCK_SCAN_NOT_ADMITTED")
	}
	return id, false, tx.Commit()
}
func (p *dockAPIv34) challenge(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var req dockCommandEnvelopeV34
	if !sewProdStrictJSON(w, r, &req) {
		return
	}
	action, role, ok := dockCommandPolicyV34(req.Command)
	if !ok || !dockPathValidV34(r.URL.Path, req.Command.Kind) {
		docJSON(w, 422, map[string]string{"error": "ERR_F34_COMMAND_FIELDS"})
		return
	}
	docJSON(w, 200, map[string]any{"challenge": dockChallengeV34(req.Command), "required_action": action, "required_role": role,
		"required_auth": "webauthn_or_fido2", "authority": dockV34Authority, "shipment_credit": false})
}
func (p *dockAPIv34) event(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var req dockCommandEnvelopeV34
	if !sewProdStrictJSON(w, r, &req) {
		return
	}
	c := req.Command
	action, role, ok := dockCommandPolicyV34(c)
	if !ok || !dockPathValidV34(r.URL.Path, c.Kind) {
		docJSON(w, 422, map[string]string{"error": "ERR_F34_COMMAND_FIELDS"})
		return
	}
	token, err := bearer(r)
	if err != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_F34_AUTH_REQUIRED"})
		return
	}
	a, err := p.evidence.material.verifier.Verify(token, action, dockChallengeV34(c), role, true)
	if err != nil || !validChapterActor(a) || a.PlantID != c.PlantID || a.JWTID == "" || len(a.JWTID) > 255 {
		docJSON(w, 401, map[string]string{"error": "ERR_F34_SIGNED_ASSERTION_REQUIRED"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	id, replayed, err := p.persistEvent(ctx, c, action, a, token)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	status := http.StatusCreated
	if replayed {
		status = http.StatusOK
	}
	docJSON(w, status, map[string]any{"id": id, "replayed": replayed, "kind": c.Kind, "authority": dockV34Authority,
		"asn_transmitted": false, "external_carrier_api_verified": false, "buyer_acceptance": false, "shipment_credit": false})
}
func dockEventTableV34(kind string) string {
	switch kind {
	case "CONTAINER_SEAL":
		return "dock_container_seals_v34"
	case "CARRIER_RECEIPT":
		return "dock_carrier_receipts_v34"
	case "ASN_DRAFT":
		return "dock_asn_drafts_v34"
	}
	return ""
}
func (p *dockAPIv34) persistEvent(ctx context.Context, c dockCommandV34, action string, a attestation.Claims, token string) (string, bool, error) {
	tx, err := p.server.docTransaction(ctx, a.TenantID)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SET LOCAL ROLE agiparel_app`); err != nil {
		return "", false, err
	}
	if err = workforceChapterRequire(ctx, tx, a, action); err != nil {
		return "", false, err
	}
	if err = sewTxLock(ctx, tx, a.TenantID, "F34:"+c.Kind+":"+c.CommandID); err != nil {
		return "", false, err
	}
	table := dockEventTableV34(c.Kind)
	if table == "" {
		return "", false, sewFail(422, "ERR_F34_KIND")
	}
	digest := sewDigest(c)
	var id, oldHash, oldActor string
	err = tx.QueryRowContext(ctx, `SELECT id::text,command_sha256,actor_subject FROM `+table+` WHERE tenant_id=$1 AND command_id=$2`, a.TenantID, c.CommandID).Scan(&id, &oldHash, &oldActor)
	if err == nil {
		if digest != oldHash || oldActor != a.Subject {
			return "", false, sewFail(409, "ERR_F34_COMMAND_REUSED")
		}
		return id, true, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, err
	}
	args := []any{a.TenantID, c.PlantID, c.ManifestID, c.Note, c.CommandID, digest, a.Subject, a.JWTID, sewProdHashString(token)}
	switch c.Kind {
	case "CONTAINER_SEAL":
		err = tx.QueryRowContext(ctx, `INSERT INTO dock_container_seals_v34
          (tenant_id,plant_id,manifest_id,note,command_id,command_sha256,actor_subject,proof_id,assertion_sha256,container_ref,seal_code,scanned_cartons)
          VALUES($1,$2,$3::uuid,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id::text`, append(args, c.ContainerRef, c.SealCode, c.ScannedCartons)...).Scan(&id)
	case "CARRIER_RECEIPT":
		err = tx.QueryRowContext(ctx, `INSERT INTO dock_carrier_receipts_v34
          (tenant_id,plant_id,manifest_id,note,command_id,command_sha256,actor_subject,proof_id,assertion_sha256,seal_id,carrier_code,receipt_reference,actor_issuer)
          VALUES($1,$2,$3::uuid,$4,$5,$6,$7,$8,$9,$10::uuid,$11,$12,$13) RETURNING id::text`, append(args, c.SealID, c.CarrierCode, c.ReceiptReference, a.Issuer)...).Scan(&id)
	case "ASN_DRAFT":
		err = tx.QueryRowContext(ctx, `INSERT INTO dock_asn_drafts_v34
          (tenant_id,plant_id,manifest_id,note,command_id,command_sha256,actor_subject,proof_id,assertion_sha256,carrier_receipt_id,asn_reference,payload_sha256)
          VALUES($1,$2,$3::uuid,$4,$5,$6,$7,$8,$9,$10::uuid,$11,$12) RETURNING id::text`, append(args, c.CarrierReceiptID, c.ASNReference, c.PayloadSHA256)...).Scan(&id)
	}
	if err != nil {
		return "", false, sewFail(409, "ERR_F34_EVENT_NOT_ADMITTED")
	}
	return id, false, tx.Commit()
}
func (p *dockAPIv34) status(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	if !p.ready(w) {
		return
	}
	q := r.URL.Query()
	for k, v := range q {
		if (k != "tenant_id" && k != "plant_id" && k != "manifest_id") || len(v) != 1 {
			docJSON(w, 422, map[string]string{"error": "ERR_F34_STATUS_QUERY"})
			return
		}
	}
	tenant, plant, manifest := q.Get("tenant_id"), q.Get("plant_id"), q.Get("manifest_id")
	if len(q["tenant_id"]) != 1 || len(q["plant_id"]) != 1 || len(q["manifest_id"]) != 1 ||
		!sewFields(tenant, plant) || !sewUUID(manifest) {
		docJSON(w, 422, map[string]string{"error": "ERR_F34_STATUS_QUERY"})
		return
	}
	access := newChapterAccessAPI(p.server)
	if !access.enabled {
		docJSON(w, 503, map[string]string{"error": "ERR_F34_STATUS_IDENTITY_DISABLED"})
		return
	}
	a, err := (&commissioningAPI{access: access}).identify(r, tenant, plant)
	if err != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_F34_STATUS_AUTH"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, err := commissionTx(ctx, p.server.db, tenant)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	defer tx.Rollback()
	permitted, err := workforceChapterPermit(ctx, tx, a, "packing.read")
	if err != nil {
		sewProdReply(w, err)
		return
	}
	if !permitted {
		docJSON(w, 403, map[string]string{"error": "ERR_F34_STATUS_GRANT"})
		return
	}
	var staged, cartons, scans int64
	var safe, sealed, carrier, asn, evidenceCurrent bool
	err = tx.QueryRowContext(ctx, `SELECT count(*) FILTER(WHERE action='STAGE'),count(*) FILTER(WHERE action='ADD_CARTON')
         FROM pack_manifest_events_v22 WHERE tenant_id=$1 AND plant_id=$2 AND manifest_id=$3::uuid`, tenant, plant, manifest).Scan(&staged, &cartons)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	if staged == 0 {
		docJSON(w, 404, map[string]string{"error": "ERR_F34_MANIFEST_NOT_STAGED"})
		return
	}
	err = tx.QueryRowContext(ctx, `SELECT dock_manifest_safe_v34($1,$2,$3::uuid), dock_evidence_current_v34($1,$2,$3::uuid),
      (SELECT count(*) FROM dock_carton_scans_v34 WHERE tenant_id=$1 AND plant_id=$2 AND manifest_id=$3::uuid),
      EXISTS(SELECT 1 FROM dock_container_seals_v34 WHERE tenant_id=$1 AND plant_id=$2 AND manifest_id=$3::uuid),
      EXISTS(SELECT 1 FROM dock_carrier_receipts_v34 WHERE tenant_id=$1 AND plant_id=$2 AND manifest_id=$3::uuid),
      EXISTS(SELECT 1 FROM dock_asn_drafts_v34 WHERE tenant_id=$1 AND plant_id=$2 AND manifest_id=$3::uuid)`, tenant, plant, manifest).Scan(&safe, &evidenceCurrent, &scans, &sealed, &carrier, &asn)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		sewProdReply(w, err)
		return
	}
	docJSON(w, 200, map[string]any{"manifest_id": manifest, "cartons": cartons, "signed_gateway_scans": scans,
		"current_preflight_safe": safe, "supervisor_seal_recorded": sealed, "onboarded_carrier_receipt_recorded": carrier,
		"asn_draft_recorded": asn, "internal_evidence_chain_complete": evidenceCurrent && scans == cartons && sealed && carrier && asn,
		"external_carrier_api_verified": false, "asn_transmitted": false, "buyer_acceptance": false, "shipment_credit": false,
		"authority": dockV34Authority})
}
func registerDockRoutesV34(mux *http.ServeMux, s *EdgeServer) {
	p := newDockAPIv34(s)
	mux.HandleFunc("/api/v1/packing/dock/scans", p.scan)
	for _, path := range []string{"/api/v1/packing/dock/seals", "/api/v1/packing/dock/carrier-receipts", "/api/v1/packing/dock/asn-drafts"} {
		mux.HandleFunc(path, p.event)
		mux.HandleFunc(path+"/challenge", p.challenge)
	}
	mux.HandleFunc("/api/v1/packing/dock/verification", p.status)
}
