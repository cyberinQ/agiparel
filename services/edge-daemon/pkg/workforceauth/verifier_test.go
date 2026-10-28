// Author: Sunil Jain, 2026-10-28.
// Regression tests for pinned OIDC JWT verification, scope and MFA.
package workforceauth

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func encode(v any) string { b, _ := json.Marshal(v); return base64.RawURLEncoding.EncodeToString(b) }
func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
func testJWKS(t *testing.T, keys ...map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "jwks.json")
	v := map[string]any{"keys": keys}
	b, _ := json.Marshal(v)
	if e := os.WriteFile(p, b, 0600); e != nil {
		t.Fatal(e)
	}
	return p
}
func jwt(t *testing.T, priv any, alg string, claims map[string]any) string {
	t.Helper()
	h := encode(map[string]string{"alg": alg, "kid": "test", "typ": "at+jwt"})
	payload := h + "." + encode(claims)
	var sig []byte
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		sum := sha256.Sum256([]byte(payload))
		var e error
		sig, e = rsa.SignPKCS1v15(rand.Reader, k, crypto.SHA256, sum[:])
		if e != nil {
			t.Fatal(e)
		}
	case ed25519.PrivateKey:
		sig = ed25519.Sign(k, []byte(payload))
	}
	return payload + "." + b64(sig)
}
func coreClaims(now time.Time) map[string]any {
	return map[string]any{"iss": "https://idp.example.test", "aud": []string{"other", "agiparel"}, "sub": "worker-007", "iat": now.Unix(), "nbf": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(), "amr": []string{"pwd", "mfa"}}
}
func TestOIDCVerifyRSAAndNegativeContracts(t *testing.T) {
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	jwk := map[string]string{"kty": "RSA", "kid": "test", "alg": "RS256", "use": "sig", "n": b64(key.N.Bytes()), "e": b64(big.NewInt(int64(key.E)).Bytes())}
	path := testJWKS(t, jwk)
	v, e := New("https://idp.example.test", "agiparel", path)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Unix(1793200000, 0)
	v.Now = func() time.Time { return now }
	c := coreClaims(now)
	good := jwt(t, key, "RS256", c)
	if identity, e := v.Verify(good); e != nil || identity.Subject != "worker-007" || identity.Issuer != "https://idp.example.test" {
		t.Fatalf("valid: %+v %v", identity, e)
	}
	mutations := []struct {
		name   string
		change func(map[string]any)
	}{
		{"wrong issuer", func(c map[string]any) { c["iss"] = "https://evil.example" }},
		{"wrong audience", func(c map[string]any) { c["aud"] = "not-agiparel" }},
		{"missing subject", func(c map[string]any) { c["sub"] = "" }},
		{"expired", func(c map[string]any) { c["exp"] = now.Unix() - 1 }},
		{"long lived", func(c map[string]any) { c["exp"] = now.Add(24 * time.Hour).Unix() }},
		{"future nbf", func(c map[string]any) { c["nbf"] = now.Add(time.Minute).Unix() }},
		{"no mfa", func(c map[string]any) { c["amr"] = []string{"pwd"} }},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			cc := coreClaims(now)
			tc.change(cc)
			if _, e := v.Verify(jwt(t, key, "RS256", cc)); e == nil {
				t.Fatal("unauthorized token accepted")
			}
		})
	}
	tampered := strings.Split(good, ".")
	tampered[1] = encode(map[string]any{"sub": "admin"})
	if _, e := v.Verify(strings.Join(tampered, ".")); e == nil {
		t.Fatal("tampered JWT accepted")
	}
	if _, e := v.Verify(jwt(t, key, "HS256", c)); e == nil {
		t.Fatal("algorithm substitution accepted")
	}
	if _, e := New("http://idp.example.test", "agiparel", path); e == nil {
		t.Fatal("non-TLS issuer accepted")
	}
	if _, e := New("https://idp.example.test", "agiparel", testJWKS(t, jwk, jwk)); e == nil {
		t.Fatal("duplicate kid accepted")
	}
}
func TestOIDCEdDSAAndKeyRotation(t *testing.T) {
	pub, priv, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Unix(1793200000, 0)
	path := testJWKS(t, map[string]string{"kty": "OKP", "kid": "test", "alg": "EdDSA", "crv": "Ed25519", "use": "sig", "x": b64(pub)})
	v, e := New("https://idp.example.test", "agiparel", path)
	if e != nil {
		t.Fatal(e)
	}
	v.Now = func() time.Time { return now }
	if _, e = v.Verify(jwt(t, priv, "EdDSA", coreClaims(now))); e != nil {
		t.Fatal(e)
	}
	if _, e = v.Verify(jwt(t, priv, "RS256", coreClaims(now))); e == nil {
		t.Fatal("algorithm confusion accepted")
	}
}
