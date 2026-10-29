// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// F3.22: verify independent Ed25519 carrier ASN and buyer review assertions,
// retain immutable evidence and report current state without shipment credit.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"regexp"
	"time"
)

var dockExternalReferenceV35 = regexp.MustCompile(`^[A-Za-z0-9_.:/-]{6,128}$`)

const dockExternalAuthorityV35 = "EXTERNAL_SIGNED_ASSERTION_NOT_CARRIER_NETWORK_PROOF_LEGAL_BUYER_ACCEPTANCE_OR_SHIPMENT_CREDIT"

type dockExternalEventV35 struct {
	Version           string `json:"version"`
	TenantID          string `json:"tenant_id"`
	PlantID           string `json:"plant_id"`
	ManifestID        string `json:"manifest_id"`
	ASNDraftID        string `json:"asn_draft_id"`
	SignerID          string `json:"signer_id"`
	PartyKind         string `json:"party_kind"`
	PartyCode         string `json:"party_code"`
	AssertionKind     string `json:"assertion_kind"`
	Decision          string `json:"decision"`
	EventID           string `json:"event_id"`
	ExternalReference string `json:"external_reference"`
	ASNReference      string `json:"asn_reference"`
	PayloadSHA256     string `json:"payload_sha256"`
	IssuedAt          string `json:"issued_at"`
}
type dockExternalEnvelopeV35 struct {
	Event        dockExternalEventV35 `json:"event"`
	SignatureHex string               `json:"signature_hex"`
}

func dockExternalBytesV35(e dockExternalEventV35) []byte {
	b, _ := json.Marshal(e) // exact struct field order and domain separator
	return append([]byte("AGIPAREL_EXTERNAL_ASSERTION_V35\n"), b...)
}
func dockExternalValidV35(e dockExternalEventV35, now time.Time) bool {
	if e.Version != "V35" || !sewFields(e.TenantID, e.PlantID, e.SignerID, e.PartyCode) ||
		len(e.SignerID) > 64 || len(e.PartyCode) > 64 ||
		!sewUUID(e.ManifestID) || !sewUUID(e.ASNDraftID) ||
		!sewCommandID.MatchString(e.EventID) ||
		!dockExternalReferenceV35.MatchString(e.ExternalReference) ||
		!dockExternalReferenceV35.MatchString(e.ASNReference) ||
		!f317bHash.MatchString(e.PayloadSHA256) {
		return false
	}
	switch e.AssertionKind {
	case "CARRIER_ASN_ACK":
		if e.PartyKind != "CARRIER" || e.Decision != "ACKNOWLEDGED" {
			return false
		}
	case "BUYER_LOT_REVIEW":
		if e.PartyKind != "BUYER" || (e.Decision != "REVIEWED_ACCEPT" && e.Decision != "REVIEWED_REJECT") {
			return false
		}
	default:
		return false
	}
	t, err := time.Parse(time.RFC3339Nano, e.IssuedAt)
	return err == nil && !t.After(now.Add(2*time.Minute)) && !t.Before(now.Add(-24*time.Hour))
}

type dockExternalAPIv35 struct {
	server  *EdgeServer
	enabled bool
}

func newDockExternalAPIv35(s *EdgeServer) *dockExternalAPIv35 {
	return &dockExternalAPIv35{server: s, enabled: newDockAPIv34(s).enabled && os.Getenv("AGIPAREL_DOCK_EXTERNAL_V35_ENABLED") == "true"}
}
func (p *dockExternalAPIv35) ready(w http.ResponseWriter) bool {
	if !p.enabled {
		docJSON(w, 503, map[string]string{"error": "ERR_F35_EXTERNAL_DISABLED"})
		return false
	}
	return true
}
func (p *dockExternalAPIv35) receive(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var req dockExternalEnvelopeV35
	if !sewProdStrictJSON(w, r, &req) {
		return
	}
	if !dockExternalValidV35(req.Event, time.Now().UTC()) {
		docJSON(w, 422, map[string]string{"error": "ERR_F35_EXTERNAL_FIELDS_OR_TIME"})
		return
	}
	signature, err := hex.DecodeString(req.SignatureHex)
	if err != nil || len(signature) != ed25519.SignatureSize {
		docJSON(w, 422, map[string]string{"error": "ERR_F35_EXTERNAL_SIGNATURE"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	id, replayed, err := p.persist(ctx, req, signature)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	status := http.StatusCreated
	if replayed {
		status = http.StatusOK
	}
	docJSON(w, status, map[string]any{"id": id, "replayed": replayed, "authority": dockExternalAuthorityV35,
		"shipment_credit": false, "carrier_network_delivery_proven": false, "legal_buyer_acceptance": false})
}
func (p *dockExternalAPIv35) persist(ctx context.Context, input dockExternalEnvelopeV35, signature []byte) (string, bool, error) {
	e := input.Event
	tx, err := p.server.docTransaction(ctx, e.TenantID)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SET LOCAL ROLE agiparel_app`); err != nil {
		return "", false, err
	}
	if err = sewTxLock(ctx, tx, e.TenantID, "F35:ACK:"+e.SignerID+":"+e.EventID); err != nil {
		return "", false, err
	}
	var publicKey []byte
	var enabled bool
	var partyKind, partyCode string
	err = tx.QueryRowContext(ctx, `SELECT public_key,enabled,party_kind,party_code FROM dock_external_signers_v35
 WHERE tenant_id=$1 AND plant_id=$2 AND signer_id=$3`, e.TenantID, e.PlantID, e.SignerID).
		Scan(&publicKey, &enabled, &partyKind, &partyCode)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, sewFail(403, "ERR_F35_SIGNER_NOT_ENROLLED")
	}
	if err != nil {
		return "", false, err
	}
	message := dockExternalBytesV35(e)
	if !enabled || partyKind != e.PartyKind || partyCode != e.PartyCode ||
		len(publicKey) != ed25519.PublicKeySize || !ed25519.Verify(ed25519.PublicKey(publicKey), message, signature) {
		return "", false, sewFail(403, "ERR_F35_EXTERNAL_SIGNATURE_OR_SCOPE")
	}
	digest := sha256.Sum256(message)
	hash := hex.EncodeToString(digest[:])
	keyDigest := sha256.Sum256(publicKey)
	var id, priorHash string
	err = tx.QueryRowContext(ctx, `SELECT id::text,event_sha256 FROM dock_external_assertions_v35
 WHERE tenant_id=$1 AND plant_id=$2 AND signer_id=$3 AND event_id=$4`, e.TenantID, e.PlantID, e.SignerID, e.EventID).
		Scan(&id, &priorHash)
	if err == nil {
		if priorHash != hash {
			return "", false, sewFail(409, "ERR_F35_REPLAY_CONFLICT")
		}
		return id, true, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, err
	}
	if !dockExternalValidV35(e, time.Now().UTC()) {
		return "", false, sewFail(422, "ERR_F35_EXTERNAL_EXPIRED")
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO dock_external_assertions_v35
 (tenant_id,plant_id,manifest_id,asn_draft_id,signer_id,party_kind,party_code,assertion_kind,decision,event_id,
  external_reference,asn_reference,payload_sha256,issued_at,event_sha256,signer_key_sha256,signature_sha256)
 VALUES($1,$2,$3::uuid,$4::uuid,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14::timestamptz,$15,$16,$17)
 RETURNING id::text`, e.TenantID, e.PlantID, e.ManifestID, e.ASNDraftID, e.SignerID, e.PartyKind, e.PartyCode,
		e.AssertionKind, e.Decision, e.EventID, e.ExternalReference, e.ASNReference, e.PayloadSHA256, e.IssuedAt,
		hash, hex.EncodeToString(keyDigest[:]), sewProdSignatureHash(input.SignatureHex)).Scan(&id)
	if err != nil {
		return "", false, sewFail(409, "ERR_F35_ASSERTION_NOT_ADMITTED")
	}
	return id, false, tx.Commit()
}
func (p *dockExternalAPIv35) status(w http.ResponseWriter, r *http.Request) {
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
			docJSON(w, 422, map[string]string{"error": "ERR_F35_QUERY"})
			return
		}
	}
	tenant, plant, manifest := q.Get("tenant_id"), q.Get("plant_id"), q.Get("manifest_id")
	if len(q["tenant_id"]) != 1 || len(q["plant_id"]) != 1 || len(q["manifest_id"]) != 1 ||
		!sewFields(tenant, plant) || !sewUUID(manifest) {
		docJSON(w, 422, map[string]string{"error": "ERR_F35_QUERY"})
		return
	}
	access := newChapterAccessAPI(p.server)
	if !access.enabled {
		docJSON(w, 503, map[string]string{"error": "ERR_F35_IDENTITY_DISABLED"})
		return
	}
	actor, err := (&commissioningAPI{access: access}).identify(r, tenant, plant)
	if err != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_F35_AUTH"})
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
	allowed, err := workforceChapterPermit(ctx, tx, actor, "packing.read")
	if err != nil {
		sewProdReply(w, err)
		return
	}
	if !allowed {
		docJSON(w, 403, map[string]string{"error": "ERR_F35_GRANT"})
		return
	}
	var count int
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM pack_manifest_events_v22
 WHERE tenant_id=$1 AND plant_id=$2 AND manifest_id=$3::uuid AND action='STAGE'`, tenant, plant, manifest).Scan(&count)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	if count != 1 {
		docJSON(w, 404, map[string]string{"error": "ERR_F35_MANIFEST_NOT_STAGED"})
		return
	}
	var internal, external, cartonReconciled bool
	var carrier, buyer, accepted int
	err = tx.QueryRowContext(ctx, `SELECT dock_evidence_current_v34($1,$2,$3::uuid),
 dock_external_current_v35($1,$2,$3::uuid),
 dock_carton_reconciled_v35($1,$2,$3::uuid),
 (SELECT count(*) FROM dock_external_assertions_v35 x WHERE x.tenant_id=$1 AND x.plant_id=$2
   AND x.manifest_id=$3::uuid AND x.assertion_kind='CARRIER_ASN_ACK'),
 (SELECT count(*) FROM dock_external_assertions_v35 x WHERE x.tenant_id=$1 AND x.plant_id=$2
   AND x.manifest_id=$3::uuid AND x.assertion_kind='BUYER_LOT_REVIEW'),
 (SELECT count(*) FROM dock_external_assertions_v35 x JOIN dock_external_signers_v35 s
   ON s.tenant_id=x.tenant_id AND s.plant_id=x.plant_id AND s.signer_id=x.signer_id
   WHERE x.tenant_id=$1 AND x.plant_id=$2 AND x.manifest_id=$3::uuid
    AND x.assertion_kind='BUYER_LOT_REVIEW' AND x.decision='REVIEWED_ACCEPT' AND s.enabled
    AND encode(digest(s.public_key,'sha256'),'hex')=x.signer_key_sha256)`, tenant, plant, manifest).
		Scan(&internal, &external, &cartonReconciled, &carrier, &buyer, &accepted)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		sewProdReply(w, err)
		return
	}
	docJSON(w, 200, map[string]any{"manifest_id": manifest, "internal_chain_current": internal, "carton_scan_qc_conserved": cartonReconciled,
		"carrier_external_assertions_recorded": carrier, "buyer_external_reviews_recorded": buyer,
		"buyer_reviewed_accept_current": external && accepted == 1, "external_assertions_current": external,
		"asn_transmitted_by_system": false, "carrier_network_delivery_proven": false,
		"legal_buyer_acceptance": false, "inventory_or_shipment_credit": false,
		"authority": dockExternalAuthorityV35})
}
func registerDockExternalRoutesV35(mux *http.ServeMux, s *EdgeServer) {
	p := newDockExternalAPIv35(s)
	mux.HandleFunc("/api/v1/packing/dock/external-assertions", p.receive)
	mux.HandleFunc("/api/v1/packing/dock/external-verification", p.status)
}
