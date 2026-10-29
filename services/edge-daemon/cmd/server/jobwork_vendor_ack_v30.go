// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// F3.17C: a provisioned subcontractor's detached Ed25519 acknowledgement,
// independently accompanied by an enrolled internal worker's strong assertion.
// A vendor signature is not physical RF presence, return, QC or a payment.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/attestation"
)

const vendorAckAuthorityV30 = "SIGNED_VENDOR_ACKNOWLEDGEMENT_NOT_RF_PROOF_QC_OR_SETTLEMENT"

type jobworkVendorAckCommandV30 struct {
	TenantID     string `json:"tenant_id"`
	CommandID    string `json:"command_id"`
	PlantID      string `json:"plant_id"`
	OutwardID    string `json:"outward_id"`
	PartID       string `json:"part_id"`
	JobReference string `json:"job_reference"`
	Process      string `json:"process"`
	VendorID     string `json:"vendor_id"`
	KeyID        string `json:"key_id"`
	AckKind      string `json:"ack_kind"`
}
type jobworkVendorAckRequestV30 struct {
	Command            jobworkVendorAckCommandV30 `json:"command"`
	VendorSignatureHex string                     `json:"vendor_signature_hex,omitempty"`
}

func validVendorAckV30(c jobworkVendorAckCommandV30) bool {
	return sewCommandID.MatchString(c.CommandID) && sewFields(c.TenantID, c.PlantID, c.VendorID, c.KeyID, c.JobReference) &&
		sewUUID(c.OutwardID) && sewUUID(c.PartID) && vendorProcessV30(c.Process) && c.AckKind == "OUTWARD_REPORTED_RECEIVED" && len(c.KeyID) >= 4
}
func vendorProcessV30(p string) bool {
	switch p {
	case "EMBROIDERY", "PRINTING", "WASHING", "DYEING", "OTHER_FINISHING":
		return true
	}
	return false
}
func vendorAckChallengeV30(c jobworkVendorAckCommandV30) string {
	return "JOBWORK_VENDOR_ACK_V30:" + sewDigest(c)
}
func verifyVendorDetachedV30(keyHex, sigHex, challenge string) bool {
	pk, e1 := hex.DecodeString(keyHex)
	sig, e2 := hex.DecodeString(sigHex)
	return e1 == nil && e2 == nil && len(pk) == ed25519.PublicKeySize && len(sig) == ed25519.SignatureSize &&
		ed25519.Verify(ed25519.PublicKey(pk), []byte(challenge), sig)
}
func (p *jobworkAPI) vendorAckChallenge(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var req jobworkVendorAckRequestV30
	if !sewProdStrictJSON(w, r, &req) {
		return
	}
	if !validVendorAckV30(req.Command) || req.VendorSignatureHex != "" {
		docJSON(w, 422, map[string]string{"error": "ERR_F317C_FIELDS"})
		return
	}
	docJSON(w, 200, map[string]any{"challenge": vendorAckChallengeV30(req.Command),
		"vendor_signature_algorithm": "Ed25519 over UTF-8 challenge bytes (not a hash)",
		"required_action":            "jobwork.vendor.ack.record", "required_role": "sewing_supervisor",
		"requires_vendor_enrolled_public_key": true, "required_auth": "webauthn_or_fido2",
		"authority": vendorAckAuthorityV30, "physical_vendor_receipt_proven": false,
		"quality_release": false, "settlement": false})
}
func (p *jobworkAPI) vendorAckEvent(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var req jobworkVendorAckRequestV30
	if !sewProdStrictJSON(w, r, &req) {
		return
	}
	c := req.Command
	if !validVendorAckV30(c) || len(req.VendorSignatureHex) != 128 || strings.ToLower(req.VendorSignatureHex) != req.VendorSignatureHex {
		docJSON(w, 422, map[string]string{"error": "ERR_F317C_FIELDS"})
		return
	}
	token, err := bearer(r)
	if err != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_F317C_AUTH"})
		return
	}
	a, err := p.holds.wip.gen.sewing.verifier.Verify(token, "jobwork.vendor.ack.record", vendorAckChallengeV30(c), "sewing_supervisor", true)
	if err != nil || !validChapterActor(a) || a.PlantID != c.PlantID || a.TenantID != c.TenantID || a.JWTID == "" || len(a.JWTID) > 255 {
		docJSON(w, 401, map[string]string{"error": "ERR_F317C_ASSERTION"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	id, replay, err := p.persistVendorAckV30(ctx, c, req.VendorSignatureHex, a, token)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	status := 201
	if replay {
		status = 200
	}
	docJSON(w, status, map[string]any{"id": id, "replayed": replay, "outward_id": c.OutwardID,
		"authority": vendorAckAuthorityV30, "verified_vendor_signature": true,
		"physical_presence": false, "quality_release": false, "settlement": false})
}
func (p *jobworkAPI) persistVendorAckV30(ctx context.Context, c jobworkVendorAckCommandV30, sig string, a attestation.Claims, token string) (string, bool, error) {
	tx, err := p.holds.wip.gen.sewing.server.docTransaction(ctx, a.TenantID)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SET LOCAL ROLE agiparel_app`); err != nil {
		return "", false, err
	}
	if err = workforceChapterRequire(ctx, tx, a, "jobwork.vendor.ack.record"); err != nil {
		return "", false, err
	}
	if err = sewTxLock(ctx, tx, a.TenantID, "F317C:COMMAND:"+c.CommandID); err != nil {
		return "", false, err
	}
	hash := sewDigest(c)
	var id, oldHash, oldSubject, oldSig string
	err = tx.QueryRowContext(ctx, `SELECT id::text,command_sha256,recorder_subject,vendor_signature_hex
  FROM jobwork_vendor_acks_v30 WHERE tenant_id=$1 AND command_id=$2`, a.TenantID, c.CommandID).
		Scan(&id, &oldHash, &oldSubject, &oldSig)
	if err == nil {
		if oldHash != hash || oldSubject != a.Subject || oldSig != sig {
			return "", false, sewFail(409, "ERR_F317C_COMMAND_CONFLICT")
		}
		return id, true, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, err
	}
	var pk string
	err = tx.QueryRowContext(ctx, `SELECT public_key_hex FROM jobwork_vendor_keys_v30
 WHERE tenant_id=$1 AND plant_id=$2 AND vendor_id=$3 AND key_id=$4 AND enabled`,
		a.TenantID, c.PlantID, c.VendorID, c.KeyID).Scan(&pk)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, sewFail(409, "ERR_F317C_UNPROVISIONED_VENDOR_KEY")
	}
	if err != nil {
		return "", false, err
	}
	if !verifyVendorDetachedV30(pk, sig, vendorAckChallengeV30(c)) {
		return "", false, sewFail(401, "ERR_F317C_BAD_VENDOR_SIGNATURE")
	}
	keyBytes, _ := hex.DecodeString(pk)
	kh := sha256.Sum256(keyBytes)
	err = tx.QueryRowContext(ctx, `INSERT INTO jobwork_vendor_acks_v30
 (tenant_id,plant_id,outward_id,part_id,job_reference,process_code,vendor_id,key_id,ack_kind,command_id,command_sha256,
  vendor_signature_hex,key_sha256,recorder_subject,recorder_proof_id,recorder_assertion_sha256)
 VALUES ($1,$2,$3::uuid,$4::uuid,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16) RETURNING id::text`,
		a.TenantID, c.PlantID, c.OutwardID, c.PartID, c.JobReference, c.Process, c.VendorID, c.KeyID, c.AckKind, c.CommandID, hash,
		sig, hex.EncodeToString(kh[:]), a.Subject, a.JWTID, sewProdHashString(token)).Scan(&id)
	if err != nil {
		return "", false, jobworkAdmissionError(err)
	}
	return id, false, tx.Commit()
}
func registerVendorAckRoutesV30(mux *http.ServeMux, s *EdgeServer) {
	p := newJobworkAPI(s)
	mux.HandleFunc("/api/v1/jobwork/vendor-ack/challenge", p.vendorAckChallenge)
	mux.HandleFunc("/api/v1/jobwork/vendor-ack", p.vendorAckEvent)
}
