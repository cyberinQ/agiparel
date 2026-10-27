package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func portalTestMux() *http.ServeMux {
	mux := http.NewServeMux()
	registerPublicPortal(mux)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/api/v1/ledger/append", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusAccepted) })
	mux.HandleFunc("/api/v1/planning/approve", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusAccepted) })
	mux.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/plant", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	return mux
}
func TestPublicPortalAndAssetsNoOperatorAuth(t *testing.T) {
	gate := operatorAccessGate(portalTestMux(), "", "")
	for _, c := range []struct {
		path        string
		needle      string
		contentType string
	}{
		{"/", `href="/documents"`, "text/html"},
		{"/demo", `href="/stores"`, "text/html"},
		{"/demo-assets/portal.css", ".phase-grid", "text/css"},
		{"/demo-assets/portal.js", "No fabricated orders", "text/javascript"},
	} {
		w := httptest.NewRecorder()
		gate.ServeHTTP(w, httptest.NewRequest(http.MethodGet, c.path, nil))
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), c.needle) {
			t.Errorf("%s: status %d, body=%q", c.path, w.Code, w.Body.String()[:min(60, w.Body.Len())])
			continue
		}
		// The public Home must not present made-up buyers as live operational data.
		if c.contentType == "text/html" {
			for _, fake := range []string{"GP240812", "PO-4518", "MK6574J9"} {
				if strings.Contains(w.Body.String(), fake) {
					t.Errorf("%s leaks fictional production record %q", c.path, fake)
				}
			}
		}
		if !strings.Contains(w.Header().Get("Content-Type"), c.contentType) {
			t.Errorf("%s: content type %q", c.path, w.Header().Get("Content-Type"))
		}
		if w.Header().Get("Content-Security-Policy") == "" {
			t.Errorf("%s: missing CSP", c.path)
		}
	}
	for _, path := range []string{"/", "/demo", "/demo-assets/portal.css", "/demo-assets/portal.js"} {
		w := httptest.NewRecorder()
		gate.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, nil))
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST %s allowed: %d", path, w.Code)
		}
	}
}
func TestOperationalRoutesFailClosedWithoutCredentials(t *testing.T) {
	gate := operatorAccessGate(portalTestMux(), "", "")
	for _, path := range []string{"/api/v1/ledger/append", "/api/v1/planning/approve", "/plant", "/static/plant_view.html", "/api/v1/material/issues", "/demo-assets/not-public.html"} {
		w := httptest.NewRecorder()
		gate.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, nil))
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("%s was not gated: %d", path, w.Code)
		}
	}
}
func TestOperatorGateRequiresBothCredentials(t *testing.T) {
	gate := operatorAccessGate(portalTestMux(), "operator", "a-long-random-development-password")
	for _, pair := range []struct {
		user, pass string
		want       int
	}{
		{"", "", http.StatusUnauthorized},
		{"operator", "wrong-password-12345", http.StatusUnauthorized},
		{"other", "a-long-random-development-password", http.StatusUnauthorized},
		{"operator", "a-long-random-development-password", http.StatusAccepted},
	} {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/ledger/append", nil)
		if pair.user != "" {
			r.SetBasicAuth(pair.user, pair.pass)
		}
		w := httptest.NewRecorder()
		gate.ServeHTTP(w, r)
		if w.Code != pair.want {
			t.Errorf("user=%q: got %d want %d", pair.user, w.Code, pair.want)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("missing no-store")
		}
		if w.Code == http.StatusUnauthorized && !strings.Contains(w.Header().Get("WWW-Authenticate"), "Basic") {
			t.Error("missing WWW-Authenticate")
		}
	}
	// Four-character passwords are accepted for development.
	devGate := operatorAccessGate(portalTestMux(), "dev", "1212")
	r := httptest.NewRequest(http.MethodPost, "/api/v1/ledger/append", nil)
	r.SetBasicAuth("dev", "1212")
	w := httptest.NewRecorder()
	devGate.ServeHTTP(w, r)
	if w.Code != http.StatusAccepted {
		t.Errorf("four-character development password rejected: %d", w.Code)
	}

	// Passwords shorter than four characters remain invalid.
	for _, password := range []string{"", "123"} {
		shortGate := operatorAccessGate(portalTestMux(), "dev", password)
		r = httptest.NewRequest(http.MethodPost, "/api/v1/ledger/append", nil)
		r.SetBasicAuth("dev", password)
		w = httptest.NewRecorder()
		shortGate.ServeHTTP(w, r)
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("password length %d allowed: %d", len(password), w.Code)
		}
	}
}
func TestPortalHealthOnlyAnonymousOperationalException(t *testing.T) {
	gate := operatorAccessGate(portalTestMux(), "", "")
	w := httptest.NewRecorder()
	gate.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("health status %d", w.Code)
	}
	w = httptest.NewRecorder()
	gate.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/ledger/append", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("anon API status %d", w.Code)
	}
}

func TestCrossSiteOperatorWriteRejected(t *testing.T) {
	gate := operatorAccessGate(portalTestMux(), "operator", "a-long-random-development-password")
	for _, tc := range []struct {
		origin, fetchSite string
		want              int
	}{
		{"https://other.example", "cross-site", http.StatusForbidden},
		{"https://other.example", "", http.StatusForbidden},
		{"", "cross-site", http.StatusForbidden},
		{"https://factory.example", "same-origin", http.StatusAccepted},
	} {
		r := httptest.NewRequest(http.MethodPost, "https://factory.example/api/v1/ledger/append", nil)
		r.SetBasicAuth("operator", "a-long-random-development-password")
		if tc.origin != "" {
			r.Header.Set("Origin", tc.origin)
		}
		if tc.fetchSite != "" {
			r.Header.Set("Sec-Fetch-Site", tc.fetchSite)
		}
		w := httptest.NewRecorder()
		gate.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Errorf("origin=%q site=%q: got %d want %d", tc.origin, tc.fetchSite, w.Code, tc.want)
		}
	}
}
func TestConfiguredGateOnlyAvailableInDevelopment(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("AGIPAREL_OPERATOR_USER", "operator")
	t.Setenv("AGIPAREL_OPERATOR_PASSWORD", "a-long-random-development-password")
	r := httptest.NewRequest(http.MethodGet, "/plant", nil)
	r.SetBasicAuth("operator", "a-long-random-development-password")
	w := httptest.NewRecorder()
	configuredOperatorGate(portalTestMux()).ServeHTTP(w, r)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("non-development operator access enabled: %d", w.Code)
	}
}
