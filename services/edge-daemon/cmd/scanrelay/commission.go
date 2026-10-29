// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// F3.12 — Verify authentic gateway signatures and independently reconcile a
// F3.11 delivery receipt before the PostgreSQL read-only commissioning audit.
// Neither a matching signature nor a server receipt proves an RF read occurred.
package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var keyHexRE = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

type commissionEvidence struct {
	Authority                  string `json:"authority"`
	TenantID                   string `json:"tenant_id"`
	PlantID                    string `json:"plant_id"`
	GatewayID                  string `json:"gateway_id"`
	StationID                  string `json:"station_id"`
	EventID                    string `json:"event_id"`
	PartID                     string `json:"part_id"`
	CarrierID                  string `json:"carrier_id"`
	EPCHex                     string `json:"epc_hex"`
	ObservedAt                 string `json:"observed_at"`
	PublicKeyHex               string `json:"public_key_hex"`
	EventSHA256                string `json:"event_sha256"`
	SignatureSHA256            string `json:"signature_sha256"`
	EnvelopeSHA256             string `json:"envelope_sha256"`
	SignedEnvelopeValid        bool   `json:"signed_envelope_valid"`
	EnrolledKeyDatabaseChecked bool   `json:"enrolled_key_database_checked"`
	ReceiptVerified            bool   `json:"receipt_verified"`
	ScanID                     string `json:"scan_id,omitempty"`
	ServerReportedReplay       *bool  `json:"server_reported_replay,omitempty"`
	DatabaseVerified           bool   `json:"database_verified"`
	PhysicalRFCertification    bool   `json:"physical_rf_certification"`
}

func sumHex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// This precise newline-delimited message MUST match wipScanBytes in F3.3.
// Adding/removing whitespace changes the signature and database digest.
func gatewaySignedBytes(e scanEvent) []byte {
	return []byte("AGIPAREL_WIP_GATEWAY_V1\n" + strings.Join([]string{
		e.Version, e.TenantID, e.PlantID, e.GatewayID, e.StationID,
		e.EventID, e.PartID, e.CarrierID, e.EPCHex, e.ObservedAt,
	}, "\n") + "\n")
}

func commissioningVerify(raw []byte, publicKeyHex, tenant, plant, gateway, station string, now time.Time) (commissionEvidence, error) {
	var result commissionEvidence
	if !keyHexRE.MatchString(publicKeyHex) {
		return result, errors.New("public key must be a 32-byte Ed25519 key in hex")
	}
	if !codeRE.MatchString(tenant) || !codeRE.MatchString(plant) || !codeRE.MatchString(gateway) || !codeRE.MatchString(station) {
		return result, errors.New("independently supplied tenant/plant/gateway/station scope required")
	}
	env, canonical, err := parseEnvelope(raw)
	if err != nil {
		return result, err
	}
	e := env.Event
	if e.TenantID != tenant || e.PlantID != plant || e.GatewayID != gateway || e.StationID != station {
		return result, errors.New("signed event does not match expected scope")
	}
	observed, _ := time.Parse(time.RFC3339Nano, e.ObservedAt)
	if observed.After(now.Add(2 * time.Minute)) {
		return result, errors.New("signed gateway clock is more than two minutes ahead")
	}
	pub, _ := hex.DecodeString(publicKeyHex)
	signature, _ := hex.DecodeString(env.SignatureHex)
	if !ed25519.Verify(ed25519.PublicKey(pub), gatewaySignedBytes(e), signature) {
		return result, errors.New("gateway Ed25519 signature invalid for the original signed fields")
	}
	result = commissionEvidence{
		Authority: receiptAuthority,
		TenantID:  e.TenantID, PlantID: e.PlantID, GatewayID: e.GatewayID, StationID: e.StationID,
		EventID: e.EventID, PartID: e.PartID, CarrierID: e.CarrierID, EPCHex: e.EPCHex,
		ObservedAt:   e.ObservedAt,
		PublicKeyHex: strings.ToLower(publicKeyHex),
		EventSHA256:  sumHex(gatewaySignedBytes(e)), SignatureSHA256: sumHex(signature),
		EnvelopeSHA256: contentHash(canonical), SignedEnvelopeValid: true,
		EnrolledKeyDatabaseChecked: false, ReceiptVerified: false, DatabaseVerified: false,
		PhysicalRFCertification: false,
	}
	return result, nil
}

func commissioningReceipt(s state, e commissionEvidence) (commissionEvidence, error) {
	key := identityKey(scanEvent{TenantID: e.TenantID, GatewayID: e.GatewayID, EventID: e.EventID})
	err := locked(s.root, "state", func() error {
		p := s.file("accepted", key)
		fi, err := os.Lstat(p)
		if err != nil {
			return fmt.Errorf("no accepted receipt for this event; enqueue and flush the real signed envelope first: %w", err)
		}
		if !fi.Mode().IsRegular() || fi.Mode().Perm()&0077 != 0 || fi.Size() > 4096 {
			return errors.New("untrusted receipt file type, permissions, or size")
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		var r receipt
		if err := json.Unmarshal(b, &r); err != nil {
			return err
		}
		if r.EnvelopeSHA256 != e.EnvelopeSHA256 || !uuidRE.MatchString(r.ServerScanID) || r.AcknowledgedAt.IsZero() {
			return errors.New("accepted receipt does not match signed envelope or has an invalid scan ID")
		}
		if _, err = os.Stat(s.file("pending", key)); err == nil {
			return errors.New("event is still pending; ambiguous delivery")
		} else if !os.IsNotExist(err) {
			return err
		}
		if _, err = os.Stat(s.file("inflight", key)); err == nil {
			return errors.New("event is still inflight; ambiguous delivery")
		} else if !os.IsNotExist(err) {
			return err
		}
		if _, err = os.Stat(s.file("quarantine", key)); err == nil {
			return errors.New("event also quarantined; conflicting local state")
		} else if !os.IsNotExist(err) {
			return err
		}
		e.ScanID = r.ServerScanID
		e.ServerReportedReplay = &r.Replayed
		e.ReceiptVerified = true
		return nil
	})
	return e, err
}

func runCommission(op, file, publicKeyHex, tenant, plant, gateway, station string, s *state, out io.Writer) error {
	if file == "" || publicKeyHex == "" || tenant == "" || plant == "" || gateway == "" || station == "" {
		return errors.New("verify/audit requires --file, --public-key-hex, --tenant, --plant, --gateway, --station")
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	e, err := commissioningVerify(raw, publicKeyHex, tenant, plant, gateway, station, time.Now().UTC())
	if err != nil {
		return err
	}
	if op == "audit" {
		if s == nil {
			return errors.New("audit requires private --dir")
		}
		e, err = commissioningReceipt(*s, e)
		if err != nil {
			return err
		}
	}
	return json.NewEncoder(out).Encode(e)
}

// Audit opens only a pre-existing commissioned outbox. It must not initialize
// a missing outbox and accidentally present an empty one as evidence.
func openExistingState(root string) (state, error) {
	s := state{root: filepath.Clean(root)}
	if !filepath.IsAbs(root) {
		return s, errors.New("commissioning outbox path must be absolute")
	}
	for _, dir := range []string{s.root, s.sub("pending"), s.sub("inflight"), s.sub("accepted"), s.sub("quarantine")} {
		f, err := os.Lstat(dir)
		if err != nil {
			return s, fmt.Errorf("commissioning outbox is not initialized: %w", err)
		}
		if !f.IsDir() || f.Mode().Perm()&0077 != 0 {
			return s, errors.New("commissioning outbox directories must be real private directories")
		}
	}
	return s, nil
}
