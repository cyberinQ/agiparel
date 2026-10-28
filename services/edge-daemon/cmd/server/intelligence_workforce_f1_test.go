// Author: Sunil Jain, 2026-10-28.
// F1.1: Enterprise access is scoped to approved tenant and plant, never token roles.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/workforceauth"
)

func TestProductionReporterRequiresIndependentScope(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	jwks := map[string]any{"keys": []any{map[string]any{"kty": "OKP", "kid": "unit", "alg": "EdDSA", "use": "sig", "crv": "Ed25519", "x": base64.RawURLEncoding.EncodeToString(pub)}}}
	b, _ := json.Marshal(jwks)
	path := filepath.Join(t.TempDir(), "idp.json")
	if err = os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	verifier, err := workforceauth.New("https://idp.example.test", "agiparel", path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	header, _ := json.Marshal(map[string]any{"typ": "at+jwt", "alg": "EdDSA", "kid": "unit"})
	claims, _ := json.Marshal(map[string]any{"iss": "https://idp.example.test", "sub": "worker_7", "aud": "agiparel", "iat": now.Unix(), "nbf": now.Unix(), "exp": now.Add(2 * time.Minute).Unix(), "amr": []string{"mfa"}})
	raw := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	token := raw + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(priv, []byte(raw)))
	api := &intelProdAPI{workforce: verifier}
	for _, tc := range []struct {
		path  string
		valid bool
	}{
		{"/api/v1/intelligence/production/report?tenant_id=tenant_A&plant_id=plant_A", true},
		{"/api/v1/intelligence/production/report?tenant_id=tenant_A", false},
		{"/api/v1/intelligence/production/report?tenant_id=tenant_A&plant_id=plant_A&plant_id=plant_B", false},
		{"/api/v1/intelligence/production/report?tenant_id=tenant_A&plant_id=bad%0Aplant", false},
	} {
		req := httptest.NewRequest("GET", tc.path, nil)
		actor, enterprise, e := api.identifyReporter(token, req)
		if tc.valid && (e != nil || !enterprise || actor.TenantID != "tenant_A" || actor.PlantID != "plant_A" || actor.Subject != "worker_7") {
			t.Fatalf("valid scope: %+v %v", actor, e)
		}
		if !tc.valid && e == nil {
			t.Fatalf("accepted bad scope %s", tc.path)
		}
	}
}
func TestWorkforceCannotSilentlyEnableInDevelopment(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_INTELLIGENCE_PRODUCTION_ENABLED", "true")
	t.Setenv("AGIPAREL_WORKFORCE_OIDC_ENABLED", "true")
	if p := newIntelProdAPI(nil); p.enabled || p.reason != "ERR_INTEL_PROD_ENVIRONMENT" {
		t.Fatalf("fail closed %+v", p)
	}
}
