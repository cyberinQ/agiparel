// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
package main

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func opsMuxForTest() http.Handler {
	m := http.NewServeMux()
	registerPublicPortal(m)
	registerEngineerOpsRoutes(m, &EdgeServer{modules: NewModuleRegistry("mod_stores,mod_cutting")})
	return m
}
func reqOps(path string, auth bool) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "https://agiparel.empowerai.us"+path, nil)
	r.TLS = &tls.ConnectionState{}
	if auth {
		r.SetBasicAuth("platform-admin", "correct-this-is-a-long-random-ops-secret")
	}
	return r
}
func TestEngineerOpsIndependentGate(t *testing.T) {
	t.Setenv("AGIPAREL_ENGINEER_OPS_ENABLED", "true")
	t.Setenv("AGIPAREL_ENGINEER_OPS_USER", "platform-admin")
	t.Setenv("AGIPAREL_ENGINEER_OPS_PASSWORD", "correct-this-is-a-long-random-ops-secret")
	m := isolateEngineerOps(operatorAccessGate(opsMuxForTest(), "dev", "1212"), opsMuxForTest())
	for _, tc := range []struct {
		path   string
		auth   bool
		status int
	}{
		{"/", false, 200},
		{"/engineer-ops", false, 401},
		{"/engineer-ops/assets/ops.css", false, 401},
		{"/api/v1/engineer-ops/overview", false, 401},
		{"/engineer-ops", true, 200},
		{"/api/v1/engineer-ops/overview", true, 200},
	} {
		w := httptest.NewRecorder()
		m.ServeHTTP(w, reqOps(tc.path, tc.auth))
		if w.Code != tc.status {
			t.Fatalf("%s auth %v got %d want %d: %s", tc.path, tc.auth, w.Code, tc.status, w.Body.String())
		}
		if strings.Contains(tc.path, "engineer-ops") && !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
			t.Errorf("no-store missing for %s", tc.path)
		}
	}
}
func TestEngineerOpsDisabledAndHTTPS(t *testing.T) {
	t.Setenv("AGIPAREL_ENGINEER_OPS_ENABLED", "false")
	t.Setenv("AGIPAREL_ENGINEER_OPS_USER", "platform-admin")
	t.Setenv("AGIPAREL_ENGINEER_OPS_PASSWORD", "correct-this-is-a-long-random-ops-secret")
	m := isolateEngineerOps(opsMuxForTest(), opsMuxForTest())
	w := httptest.NewRecorder()
	m.ServeHTTP(w, reqOps("/engineer-ops", true))
	if w.Code != 404 {
		t.Fatalf("disabled should be 404 not %d", w.Code)
	}
	t.Setenv("AGIPAREL_ENGINEER_OPS_ENABLED", "true")
	r := httptest.NewRequest(http.MethodGet, "http://agiparel.empowerai.us/engineer-ops", nil)
	r.SetBasicAuth("platform-admin", "correct-this-is-a-long-random-ops-secret")
	w = httptest.NewRecorder()
	m.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("HTTP on public host should be forbidden, got %d", w.Code)
	}
	r = reqOps("/engineer-ops", true)
	r.Method = http.MethodPost
	w = httptest.NewRecorder()
	m.ServeHTTP(w, r)
	if w.Code != 405 {
		t.Fatalf("ops write method should be blocked: %d", w.Code)
	}
}
func TestEngineerOpsNoDefaultCredential(t *testing.T) {
	t.Setenv("AGIPAREL_ENGINEER_OPS_ENABLED", "true")
	t.Setenv("AGIPAREL_ENGINEER_OPS_USER", "platform-admin")
	t.Setenv("AGIPAREL_ENGINEER_OPS_PASSWORD", "short")
	m := isolateEngineerOps(opsMuxForTest(), opsMuxForTest())
	w := httptest.NewRecorder()
	m.ServeHTTP(w, reqOps("/engineer-ops", true))
	if w.Code != 503 {
		t.Fatalf("weak credential must fail closed: %d", w.Code)
	}
}

// Forwarded HTTPS is accepted only when explicitly enabled for a trusted proxy.
// In production the backend port must remain bound to loopback and the proxy
// must overwrite any incoming X-Forwarded-Proto supplied by web clients.
func TestEngineerOpsTLSProxyForwarding(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("AGIPAREL_ENGINEER_OPS_ENABLED", "true")
	t.Setenv("AGIPAREL_ENGINEER_OPS_USER", "platform-admin")
	t.Setenv("AGIPAREL_ENGINEER_OPS_PASSWORD", "correct-this-is-a-long-random-ops-secret")
	m := isolateEngineerOps(opsMuxForTest(), opsMuxForTest())
	for _, tc := range []struct {
		trusted, forwarded string
		status             int
	}{
		{"false", "https", 403},
		{"true", "", 403},
		{"true", "http", 403},
		{"true", "https", 200},
	} {
		t.Setenv("AGIPAREL_ENGINEER_OPS_TRUST_PROXY_TLS", tc.trusted)
		r := httptest.NewRequest(http.MethodGet, "http://agiparel.empowerai.us/engineer-ops", nil)
		r.Header.Set("X-Forwarded-Proto", tc.forwarded)
		r.SetBasicAuth("platform-admin", "correct-this-is-a-long-random-ops-secret")
		w := httptest.NewRecorder()
		m.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("trusted=%s forwarded=%q got=%d expected=%d", tc.trusted, tc.forwarded, w.Code, tc.status)
		}
	}
}
