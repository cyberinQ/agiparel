package attestation

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func mint(t *testing.T, priv ed25519.PrivateKey, claims Claims) string {
	t.Helper()
	h, _ := json.Marshal(header{Algorithm: "EdDSA", Type: "JWT", KeyID: "unit"})
	b, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(h) + "." + base64.RawURLEncoding.EncodeToString(b)
	return payload + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(priv, []byte(payload)))
}

func TestPinningScopeRoleAndStrongAuth(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Unix(1793000000, 0)
	v := &Verifier{PublicKeys: map[string]ed25519.PublicKey{"unit": pub}, Issuer: "issuer", Audience: "agiparel-material", Now: func() time.Time { return now }}
	c := Claims{Issuer: "issuer", Audience: "agiparel-material", Subject: "worker1", TenantID: "T1", PlantID: "P1", Action: "material.issue.witness", Challenge: "hash123", JWTID: "nonce-1", IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: now.Add(2 * time.Minute).Unix(), Roles: []string{"stores_issuer"}, AuthMethods: []string{"webauthn"}}
	token := mint(t, priv, c)
	if got, err := v.Verify(token, c.Action, c.Challenge, "stores_issuer", true); err != nil || got.Subject != "worker1" {
		t.Fatalf("valid proof: %v", err)
	}
	for _, test := range []struct {
		desc, action, challenge, role string
		strong                        bool
	}{
		{"wrong action", "material.reserve", "hash123", "stores_issuer", true},
		{"wrong challenge", c.Action, "other", "stores_issuer", true},
		{"role escalation", c.Action, c.Challenge, "ceo", true},
	} {
		if _, err := v.Verify(token, test.action, test.challenge, test.role, test.strong); err == nil {
			t.Fatalf("accepted %s", test.desc)
		}
	}
	chunks := strings.Split(token, ".")
	chunks[1] = strings.Replace(chunks[1], "A", "B", 1)
	if _, err := v.Verify(strings.Join(chunks, "."), c.Action, c.Challenge, "stores_issuer", true); err == nil {
		t.Fatal("tampered token accepted")
	}
	c.AuthMethods = []string{"pwd"}
	if _, err := v.Verify(mint(t, priv, c), c.Action, c.Challenge, "stores_issuer", true); err == nil {
		t.Fatal("password-only witness accepted")
	}
	c.AuthMethods = []string{"fido2"}
	c.ExpiresAt = now.Add(20 * time.Minute).Unix()
	if _, err := v.Verify(mint(t, priv, c), c.Action, c.Challenge, "stores_issuer", true); err == nil {
		t.Fatal("long-lived proof accepted")
	}
}

func TestVerifyChallengeBoundDoesNotTrustUnsignedContents(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Unix(1793000000, 0)
	v := &Verifier{PublicKeys: map[string]ed25519.PublicKey{"unit": pub}, Issuer: "issuer", Audience: "factory", Now: func() time.Time { return now }}
	c := Claims{Issuer: "issuer", Audience: "factory", Subject: "ceo1", TenantID: "T1", PlantID: "P1", Action: "material.variance.approve", Challenge: "server-bound-digest", JWTID: "one-time-unique", IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix(), Roles: []string{"ceo"}, AuthMethods: []string{"fido2"}}
	token := mint(t, priv, c)
	if _, err := v.VerifyChallengeBound(token, c.Action, "ceo", true); err != nil {
		t.Fatal(err)
	}
	c.Challenge = ""
	if _, err := v.VerifyChallengeBound(mint(t, priv, c), c.Action, "ceo", true); err == nil {
		t.Fatal("unbound authorization accepted")
	}
}
