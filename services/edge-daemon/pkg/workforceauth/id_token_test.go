// Author: Sunil Jain, 2026-10-28.
// Ensure browser SSO ID tokens are nonce-bound and never valid as access tokens.
package workforceauth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"
)

func TestOIDCIDTokenNonceAndAudience(t *testing.T) {
	pub, priv, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC()
	v, e := New("https://idp.example.test", "factory-web-client", testJWKS(t, map[string]string{"kty": "OKP", "kid": "test", "alg": "EdDSA", "crv": "Ed25519", "use": "sig", "x": b64(pub)}))
	if e != nil {
		t.Fatal(e)
	}
	v.Now = func() time.Time { return now }
	nonce := "nonce_abcdefghijklmnopqrstuvwxyz0123456789"
	c := coreClaims(now)
	c["aud"] = "factory-web-client"
	c["nonce"] = nonce
	issue := func(claims map[string]any) string {
		h := encode(map[string]string{"alg": "EdDSA", "kid": "test"}) // ID tokens may have no typ
		signed := h + "." + encode(claims)
		return signed + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(priv, []byte(signed)))
	}
	good := issue(c)
	if _, e = v.VerifyIDToken(good, nonce); e != nil {
		t.Fatal(e)
	}
	if _, e = v.Verify(good); e == nil {
		t.Fatal("ID token accepted as API bearer")
	}
	if _, e = v.VerifyIDToken(good, "nonce_abcdefghijklmnopqrstuvwxyz9876543210"); e == nil {
		t.Fatal("nonce mismatch accepted")
	}
	modified := coreClaims(now)
	modified["nonce"] = nonce
	modified["aud"] = []string{"factory-web-client", "other"}
	if _, e = v.VerifyIDToken(issue(modified), nonce); e == nil {
		t.Fatal("missing azp accepted")
	}
	modified["azp"] = "factory-web-client"
	if _, e = v.VerifyIDToken(issue(modified), nonce); e != nil {
		t.Fatal("correct azp rejected", e)
	}
	modified["amr"] = []string{"pwd"}
	if _, e = v.VerifyIDToken(issue(modified), nonce); e == nil {
		t.Fatal("weak auth accepted")
	}
}
