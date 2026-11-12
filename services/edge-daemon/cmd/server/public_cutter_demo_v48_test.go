// Author: Sunil Jain, 2026-11-12.
// Verify production browser demo remains opt-in and cannot unlock operational endpoints.
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublicCutterDemoIsOptInBrowserOnly(t *testing.T) {
	t.Setenv("ENV", "production")
	mux := http.NewServeMux()
	registerPublicPortal(mux)
	// A generic protected route must never become public by enabling the demo.
	mux.HandleFunc("/cutting/sop2", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	handler := configuredOperatorGate(mux)
	paths := []string{"/demo/cutting", "/demo-assets/cutter.css", "/demo-assets/cutter-model.js", "/demo-assets/cutter.js"}
	t.Setenv("AGIPAREL_PUBLIC_CUTTER_DEMO_ENABLED", "false")
	for _, p := range paths {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s disabled: %d", p, rec.Code)
		}
	}
	t.Setenv("AGIPAREL_PUBLIC_CUTTER_DEMO_ENABLED", "true")
	for _, p := range paths {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s enabled: %d", p, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Sunil Jain") && p != "/demo/cutting" {
			t.Fatalf("%s asset missing author provenance", p)
		}
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/demo/cutting", nil))
	body := rec.Body.String()
	for _, expected := range []string{`data-demo-mode="public"`, "/demo-assets/cutter.js", "TRAINING SIMULATION", "Nothing is cut"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("missing public training marker %q", expected)
		}
	}
	if strings.Contains(body, `src="/static/cutter_demo.js"`) {
		t.Fatal("private assets leaked into public page")
	}
	post := httptest.NewRecorder()
	handler.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/demo/cutting", strings.NewReader("{}")))
	if post.Code != http.StatusMethodNotAllowed {
		t.Fatalf("public demo POST: %d", post.Code)
	}
	protected := httptest.NewRecorder()
	handler.ServeHTTP(protected, httptest.NewRequest(http.MethodGet, "/cutting/sop2", nil))
	if protected.Code == http.StatusOK {
		t.Fatal("public demo flag bypasses operational access gate")
	}
}
