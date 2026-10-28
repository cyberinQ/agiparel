// Author: Sunil Jain, 2026-10-28.
// F3.2: command invariants, independent signatures, fail-closed APIs.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/attestation"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func f32Example() f32Command {
	return f32Command{CommandID: "F32-CUT-RECONCILE-00001", PlantID: "plant-A", Action: "RECONCILE", ReleaseID: "11111111-1111-4111-8111-111111111111", SpreadID: "22222222-2222-4222-8222-222222222222", UsedPartMM: 9000, TrimWasteMM: 800, DamagedMM: 200, Note: "Certified by two independent factory supervisors"}
}
func TestF32ActionValidityAndChallenge(t *testing.T) {
	base := f32Example()
	if !f32Valid(base) {
		t.Fatal("valid material balance rejected")
	}
	digest := f32Challenge(base)
	for _, f := range []func(*f32Command){func(c *f32Command) { c.PlantID = "plant-B" }, func(c *f32Command) { c.UsedPartMM++ }, func(c *f32Command) { c.SpreadID = "33333333-3333-4333-8333-333333333333" }, func(c *f32Command) { c.Note = "Changed evidence note" }} {
		c := base
		f(&c)
		if f32Challenge(c) == digest {
			t.Fatal("tampered challenge not bound")
		}
	}
	for _, f := range []func(*f32Command){func(c *f32Command) { c.Action = "UNKNOWN" }, func(c *f32Command) { c.SpreadID = "invalid" }, func(c *f32Command) { c.PartID = base.PartID + "x" }, func(c *f32Command) { c.TrimWasteMM = -1 }, func(c *f32Command) { c.UsedPartMM = 9223372036854775807 }, func(c *f32Command) { c.Note = "too short" }} {
		c := base
		f(&c)
		if f32Valid(c) {
			t.Fatal("invalid material classification accepted", c)
		}
	}
	c := f32Command{CommandID: base.CommandID, PlantID: base.PlantID, Action: "ACCEPT_PART", ReleaseID: base.ReleaseID, PartID: base.SpreadID, Note: base.Note}
	if !f32Valid(c) {
		t.Fatal("valid part acceptance rejected")
	}
	c.DefectCode = "HOLE"
	if f32Valid(c) {
		t.Fatal("accept cannot name defect")
	}
	c.Action = "REJECT_PART"
	if !f32Valid(c) {
		t.Fatal("valid part rejection rejected")
	}
	c.DefectCode = "bad name"
	if f32Valid(c) {
		t.Fatal("unregistered defect format accepted")
	}
	c.DefectCode = "HOLE"
	c.ReplacementPartID = "33333333-3333-4333-8333-333333333333"
	if f32Valid(c) {
		t.Fatal("rejection cannot link recut")
	}
	c.Action = "LINK_RECUT"
	c.DefectCode = ""
	if !f32Valid(c) {
		t.Fatal("valid recut rejected")
	}
	c.ReplacementPartID = c.PartID
	if f32Valid(c) {
		t.Fatal("self-recut accepted")
	}
}
func TestF32CannotRunInDevelopment(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_CUTTING_RECONCILIATION_ENABLED", "true")
	t.Setenv("AGIPAREL_GENEALOGY_PRODUCTION_ENABLED", "true")
	t.Setenv("AGIPAREL_SEWING_PRODUCTION_ENABLED", "true")
	p := newF32API(&EdgeServer{})
	if p.enabled {
		t.Fatal("production cut reconciliation enabled in dev")
	}
	for _, run := range []func(http.ResponseWriter, *http.Request){p.write, p.challenge, p.balance, p.defects} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/cutting/production/reconciliation/events", strings.NewReader(`{}`))
		run(w, r)
		if w.Code != 503 && w.Code != 405 {
			t.Fatalf("fail-closed route returned %d", w.Code)
		}
	}
}
func TestF32NoDefaultAccess(t *testing.T) {
	t.Setenv("AGIPAREL_CUTTING_RECONCILIATION_ENABLED", "false")
	p := newF32API(nil)
	if p.enabled {
		t.Fatal("default enabled")
	}
}

// The actual Ed25519 challenge-bound verifier is tested here, not a mock IdP.
func f32MintTestAssertion(t *testing.T, priv ed25519.PrivateKey, c attestation.Claims) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT", "kid": "f32-test"})
	payload, _ := json.Marshal(c)
	signed := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	return signed + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(priv, []byte(signed)))
}
func TestF32IndependentSignedActorsAndChallenge(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	v := &attestation.Verifier{Issuer: "https://issuer.example.test", Audience: "agiparel", PublicKeys: map[string]ed25519.PublicKey{"f32-test": pub}, Now: func() time.Time { return now }}
	p := &f32API{enabled: true, gen: &genProductionAPI{enabled: true, sewing: &sewProdAPI{enabled: true, verifier: v}}}
	c := f32Example()
	mk := func(sub, jti, action, role string) string {
		return f32MintTestAssertion(t, priv, attestation.Claims{Issuer: v.Issuer, Audience: v.Audience, Subject: sub, TenantID: "tenant-A", PlantID: c.PlantID, Action: action, Challenge: f32Challenge(c), JWTID: jti, IssuedAt: now.Add(-time.Minute).Unix(), NotBefore: now.Add(-time.Minute).Unix(), ExpiresAt: now.Add(time.Minute).Unix(), Roles: []string{role}, AuthMethods: []string{"webauthn"}})
	}
	sup := mk("supervisor-A", "proof-1", "cutting.release", "cutting_supervisor")
	qual := mk("inspector-B", "proof-2", "quality.inspect", "quality_inspector")
	request := func(tok string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/cutting/production/reconciliation/events", nil)
		r.Header.Set("Authorization", "Bearer "+tok)
		return r
	}
	w := httptest.NewRecorder()
	a, b, _, ok := p.claims(w, request(sup), c, qual)
	if !ok || a.Subject != "supervisor-A" || b.Subject != "inspector-B" {
		t.Fatalf("signed two-person claim rejected: %s", w.Body.String())
	}
	for _, bad := range []string{
		mk("supervisor-A", "proof-3", "quality.inspect", "quality_inspector"),
		mk("inspector-B", "proof-4", "cutting.release", "cutting_supervisor"),
		mk("supervisor-A", "proof-5", "quality.inspect", "quality_inspector"),
		qual + "tamper",
	} {
		w = httptest.NewRecorder()
		_, _, _, ok = p.claims(w, request(sup), c, bad)
		if ok || w.Code != 401 {
			t.Fatalf("invalid reviewer accepted: %d %s", w.Code, w.Body.String())
		}
	}
	changed := c
	changed.TrimWasteMM++
	w = httptest.NewRecorder()
	_, _, _, ok = p.claims(w, request(sup), changed, qual)
	if ok || w.Code != 401 {
		t.Fatal("changing classified millimetres bypassed signatures")
	}
}
