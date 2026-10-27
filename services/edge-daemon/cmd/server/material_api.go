// Author: Sunil Jain, CyberinQ LLC, 2026-10-26.
// V5 Batch 02: separately authenticated, fail-closed material transactions.
// All principal/tenant/plant fields are extracted from trusted signed assertions.
package main

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/attestation"
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/stores"
)

type materialAPI struct {
	repository stores.MaterialRepository
	verifier   *attestation.Verifier
	initErr    error
}

type signedMaterialVerifier struct{ v *attestation.Verifier }

func (a signedMaterialVerifier) VerifyMaterialWitness(_ context.Context, p stores.WitnessProof, challenge string) error {
	if p.Role != "stores_issuer" && p.Role != "stores_recipient" {
		return errors.New("ERR_WITNESS_ROLE")
	}
	c, err := a.v.Verify(p.Assertion, "material.issue.witness", challenge, p.Role, true)
	if err != nil {
		return err
	}
	if c.Subject != p.Subject || c.TenantID != p.TenantID || c.JWTID != p.ProofID || c.PlantID != p.PlantID {
		return errors.New("ERR_WITNESS_IDENTITY_MISMATCH")
	}
	return nil
}
func (a signedMaterialVerifier) VerifyCEOApproval(_ context.Context, tenant, subject, proofID, assertion, challenge string) error {
	c, err := a.v.Verify(assertion, "material.variance.approve", challenge, "ceo", true)
	if err != nil {
		return err
	}
	if c.TenantID != tenant || c.Subject != subject || c.JWTID != proofID {
		return errors.New("ERR_CEO_ASSERTION_IDENTITY")
	}
	return nil
}

func newMaterialAPI(db *sql.DB) *materialAPI {
	a := &materialAPI{}
	if os.Getenv("AGIPAREL_MATERIAL_API_ENABLED") != "true" {
		a.initErr = errors.New("material API not enabled")
		return a
	}
	if db == nil {
		a.initErr = errors.New("database not configured")
		return a
	}
	key, err := attestation.DecodePublicKey(os.Getenv("AGIPAREL_IDP_ED25519_PUBLIC_KEY_B64"))
	if err != nil {
		a.initErr = err
		return a
	}
	kid, issuer, audience := os.Getenv("AGIPAREL_IDP_KID"), os.Getenv("AGIPAREL_IDP_ISSUER"), os.Getenv("AGIPAREL_IDP_AUDIENCE")
	holder := os.Getenv("AGIPAREL_MATERIAL_AUTHORITY_HOLDER")
	if kid == "" || issuer == "" || audience == "" || holder == "" {
		a.initErr = errors.New("missing IdP or authority configuration")
		return a
	}
	a.verifier = &attestation.Verifier{PublicKeys: map[string]ed25519.PublicKey{kid: key}, Issuer: issuer, Audience: audience, MaxLifetime: 5 * time.Minute}
	auth := signedMaterialVerifier{v: a.verifier}
	a.repository = stores.MaterialRepository{DB: db, WitnessVerifier: auth, CEOApprovalVerifier: auth, AuthorityHolder: holder}
	return a
}

func registerMaterialRoutes(mux *http.ServeMux, db *sql.DB) {
	a := newMaterialAPI(db)
	if a.initErr != nil {
		log.Printf("[material-v5] disabled (fail closed): %v", a.initErr)
	}
	mux.HandleFunc("/api/v1/material/reservations", a.reserve)
	mux.HandleFunc("/api/v1/material/issues/challenge", a.issueChallenge)
	mux.HandleFunc("/api/v1/material/issues", a.issue)
	mux.HandleFunc("/api/v1/material/variances", a.varianceRequest)
	mux.HandleFunc("/api/v1/material/variances/", a.varianceAction)
	// Batch 07C: signed two-person registration of new, reviewed rolls only.
	mux.HandleFunc("/api/v1/material/roll-admissions/challenge", a.rollAdmissionChallenge)
	mux.HandleFunc("/api/v1/material/roll-admissions", a.rollAdmission)
	// Batch 08B: externally signed cutting RFID witness; no machine-gateway attestation.
	mux.HandleFunc("/api/v1/cutting/bundle-scans/challenge", a.cutScanChallengeRoute)
	mux.HandleFunc("/api/v1/cutting/bundle-scans", a.cutSignedScanRoute)
	mux.HandleFunc("/api/v1/cutting/releases/challenge", a.cutReleaseChallengeRoute)
	mux.HandleFunc("/api/v1/cutting/releases", a.cutReleaseRoute)
}

func (a *materialAPI) ready(w http.ResponseWriter) bool {
	if a.initErr != nil || a.verifier == nil || a.repository.DB == nil {
		http.Error(w, "ERR_MATERIAL_SECURE_API_DISABLED", http.StatusServiceUnavailable)
		return false
	}
	return true
}
func decodeMaterialJSON(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("extra JSON after request object")
	}
	return nil
}
func bearer(r *http.Request) (string, error) {
	v := r.Header.Get("Authorization")
	if !strings.HasPrefix(v, "Bearer ") || len(strings.TrimSpace(strings.TrimPrefix(v, "Bearer "))) == 0 {
		return "", errors.New("ERR_BEARER_REQUIRED")
	}
	return strings.TrimSpace(strings.TrimPrefix(v, "Bearer ")), nil
}
func (a *materialAPI) authorize(r *http.Request, action, role string, strong bool) (attestation.Claims, error) {
	token, err := bearer(r)
	if err != nil {
		return attestation.Claims{}, err
	}
	return a.verifier.Verify(token, action, "", role, strong)
}
func respondMaterial(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func materialError(w http.ResponseWriter, err error) {
	// Avoid returning DB internals (table names, SQL statements, connection secrets).
	code := err.Error()
	status := http.StatusUnprocessableEntity
	switch {
	case strings.HasPrefix(code, "ERR_IDP"), strings.HasPrefix(code, "ERR_BEARER"), strings.HasPrefix(code, "ERR_JWS"), strings.HasPrefix(code, "ERR_ASSERTION"), strings.HasPrefix(code, "ERR_INVALID_ASSERTION"), strings.HasPrefix(code, "ERR_STRONG_AUTH"), strings.HasPrefix(code, "ERR_CEO_ATTESTATION"):
		status = http.StatusUnauthorized
	case strings.HasPrefix(code, "ERR_MATERIAL_AUTHORITY_UNCONFIGURED"), strings.HasPrefix(code, "ERR_CEO_VERIFIER_REQUIRED"):
		status = http.StatusServiceUnavailable
	case strings.HasPrefix(code, "ERR_MATERIAL_SCOPE"), strings.HasPrefix(code, "ERR_CEO_ONLY"), strings.HasPrefix(code, "ERR_PLANT_SCOPE"), strings.HasPrefix(code, "ERR_ISSUER"), strings.HasPrefix(code, "ERR_RECIPIENT"):
		status = http.StatusForbidden
	case strings.HasPrefix(code, "ERR_RESERVATION_NOT_FOUND"), strings.HasPrefix(code, "ERR_ROLL_NOT_FOUND"):
		status = http.StatusNotFound
	case strings.HasPrefix(code, "ERR_ROLL_NOT_READY"), strings.HasPrefix(code, "ERR_REVIEW_SNAPSHOT"), strings.HasPrefix(code, "ERR_REVIEW_STATE"):
		status = http.StatusConflict
	case strings.HasPrefix(code, "ERR_IDEMPOTENCY"), strings.HasPrefix(code, "ERR_VARIANCE_ALREADY"), strings.HasPrefix(code, "ERR_UNRESERVED"), strings.HasPrefix(code, "ERR_CEO_CANNOT"), strings.HasPrefix(code, "ERR_RESERVED"):
		status = http.StatusConflict
	case !strings.HasPrefix(code, "ERR_"):
		log.Printf("[material-v5] transaction error: %v", err)
		code = "ERR_MATERIAL_INTERNAL"
		status = http.StatusInternalServerError
	}
	respondMaterial(w, status, map[string]string{"error": code})
}
func requirePOST(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	return true
}

type materialParams struct {
	ReservationID  string `json:"reservation_id"`
	RollID         string `json:"roll_id"`
	OrderID        string `json:"order_id"`
	CutNo          int    `json:"cut_no"`
	LengthMM       int64  `json:"length_mm"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (a *materialAPI) reserve(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !a.ready(w) {
		return
	}
	actor, err := a.authorize(r, "material.reserve", "stores_manager", true)
	if err != nil {
		materialError(w, err)
		return
	}
	var q materialParams
	if err = decodeMaterialJSON(w, r, &q); err != nil {
		http.Error(w, "bad JSON", 400)
		return
	}
	got, err := a.repository.Reserve(r.Context(), stores.MaterialReserveCommand{TenantID: actor.TenantID, PlantID: actor.PlantID, RollID: q.RollID, OrderID: q.OrderID, CutNo: q.CutNo, LengthMM: q.LengthMM, IdempotencyKey: q.IdempotencyKey})
	if err != nil {
		materialError(w, err)
		return
	}
	respondMaterial(w, 200, got)
}
func (a *materialAPI) issueChallenge(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !a.ready(w) {
		return
	}
	actor, err := a.authorize(r, "material.issue.prepare", "stores_issuer", true)
	if err != nil {
		materialError(w, err)
		return
	}
	var q materialParams
	if err = decodeMaterialJSON(w, r, &q); err != nil {
		http.Error(w, "bad JSON", 400)
		return
	}
	c := stores.MaterialIssueCommand{TenantID: actor.TenantID, PlantID: actor.PlantID, ReservationID: q.ReservationID, RollID: q.RollID, OrderID: q.OrderID, CutNo: q.CutNo, LengthMM: q.LengthMM, IdempotencyKey: q.IdempotencyKey}
	if c.ReservationID == "" || c.RollID == "" || c.OrderID == "" || c.IdempotencyKey == "" || c.CutNo <= 0 || c.LengthMM <= 0 {
		materialError(w, errors.New("ERR_ISSUE_INPUT"))
		return
	}
	respondMaterial(w, 200, map[string]string{"challenge": stores.MaterialIssueChallenge(c), "action": "material.issue.witness", "required_issuer_role": "stores_issuer", "required_recipient_role": "stores_recipient"})
}

type materialIssueBody struct {
	materialParams
	IssuerAttestation    string `json:"issuer_attestation"`
	RecipientAttestation string `json:"recipient_attestation"`
}

func (a *materialAPI) issue(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !a.ready(w) {
		return
	}
	actor, err := a.authorize(r, "material.issue.submit", "stores_issuer", true)
	if err != nil {
		materialError(w, err)
		return
	}
	var q materialIssueBody
	if err = decodeMaterialJSON(w, r, &q); err != nil {
		http.Error(w, "bad JSON", 400)
		return
	}
	c := stores.MaterialIssueCommand{TenantID: actor.TenantID, PlantID: actor.PlantID, ReservationID: q.ReservationID, RollID: q.RollID, OrderID: q.OrderID, CutNo: q.CutNo, LengthMM: q.LengthMM, IdempotencyKey: q.IdempotencyKey}
	challenge := stores.MaterialIssueChallenge(c)
	issuer, err := a.verifier.Verify(q.IssuerAttestation, "material.issue.witness", challenge, "stores_issuer", true)
	if err != nil {
		materialError(w, err)
		return
	}
	recipient, err := a.verifier.Verify(q.RecipientAttestation, "material.issue.witness", challenge, "stores_recipient", true)
	if err != nil {
		materialError(w, err)
		return
	}
	if issuer.TenantID != actor.TenantID || recipient.TenantID != actor.TenantID || issuer.PlantID != actor.PlantID || recipient.PlantID != actor.PlantID || issuer.Subject != actor.Subject || recipient.Subject == issuer.Subject || issuer.JWTID == recipient.JWTID {
		materialError(w, errors.New("ERR_INDEPENDENT_WITNESS"))
		return
	}
	c.Issuer = stores.WitnessProof{TenantID: issuer.TenantID, PlantID: issuer.PlantID, Subject: issuer.Subject, ProofID: issuer.JWTID, Assertion: q.IssuerAttestation, Role: "stores_issuer"}
	c.Recipient = stores.WitnessProof{TenantID: recipient.TenantID, PlantID: recipient.PlantID, Subject: recipient.Subject, ProofID: recipient.JWTID, Assertion: q.RecipientAttestation, Role: "stores_recipient"}
	got, err := a.repository.Issue(r.Context(), c)
	if err != nil {
		materialError(w, err)
		return
	}
	respondMaterial(w, 200, got)
}

type varianceBody struct {
	ReservationID  string `json:"reservation_id"`
	RollID         string `json:"roll_id"`
	OrderID        string `json:"order_id"`
	CutNo          int    `json:"cut_no"`
	AdditionalMM   int64  `json:"additional_mm"`
	ActualStory    string `json:"actual_story"`
	ReasonCode     string `json:"reason_code"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (a *materialAPI) varianceRequest(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !a.ready(w) {
		return
	}
	actor, err := a.authorize(r, "material.variance.request", "stores_manager", true)
	if err != nil {
		materialError(w, err)
		return
	}
	var q varianceBody
	if err = decodeMaterialJSON(w, r, &q); err != nil {
		http.Error(w, "bad JSON", 400)
		return
	}
	got, err := a.repository.RequestVariance(r.Context(), stores.VarianceRequestCommand{TenantID: actor.TenantID, PlantID: actor.PlantID, ReservationID: q.ReservationID, RollID: q.RollID, OrderID: q.OrderID, CutNo: q.CutNo, AdditionalMM: q.AdditionalMM, ActualStory: q.ActualStory, ReasonCode: q.ReasonCode, RequestedBy: actor.Subject, IdempotencyKey: q.IdempotencyKey})
	if err != nil {
		materialError(w, err)
		return
	}
	respondMaterial(w, 200, got)
}
func (a *materialAPI) varianceAction(w http.ResponseWriter, r *http.Request) {
	if !a.ready(w) {
		return
	}
	tail := strings.TrimPrefix(r.URL.Path, "/api/v1/material/variances/")
	chunks := strings.Split(tail, "/")
	if len(chunks) != 2 || chunks[0] == "" {
		http.NotFound(w, r)
		return
	}
	id, action := chunks[0], chunks[1]
	switch action {
	case "challenge":
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", 405)
			return
		}
		actor, err := a.authorize(r, "material.variance.review", "ceo", true)
		if err != nil {
			materialError(w, err)
			return
		}
		challenge, err := a.repository.LookupVarianceChallenge(r.Context(), actor.TenantID, actor.PlantID, id, actor.Subject)
		if err != nil {
			materialError(w, err)
			return
		}
		respondMaterial(w, 200, map[string]string{"challenge": challenge, "action": "material.variance.approve"})
	case "approve":
		if !requirePOST(w, r) {
			return
		}
		token, err := bearer(r)
		if err != nil {
			materialError(w, err)
			return
		}
		actor, err := a.verifier.VerifyChallengeBound(token, "material.variance.approve", "ceo", true)
		if err != nil {
			materialError(w, err)
			return
		}
		got, err := a.repository.ApproveVariance(r.Context(), stores.VarianceApprovalCommand{TenantID: actor.TenantID, PlantID: actor.PlantID, RequestID: id, CEOSubject: actor.Subject, CEOProofID: actor.JWTID, CEOAssertion: token})
		if err != nil {
			materialError(w, err)
			return
		}
		respondMaterial(w, 200, got)
	default:
		http.NotFound(w, r)
	}
}
