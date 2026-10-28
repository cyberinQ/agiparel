// Author: Sunil Jain, 2026-10-28.
// Verify only independently authenticated production ingress bypasses legacy Basic gate.
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProductionSewingAccessBoundary(t *testing.T) {
	t.Setenv("ENV", "production")
	handler := configuredOperatorGate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	for _, path := range []string{
		"/api/v1/sewing/production/observations",
		"/api/v1/sewing/production/completions/challenge",
		"/api/v1/sewing/production/completions",
	} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("POST", path, nil))
		if w.Code != http.StatusNoContent {
			t.Errorf("signed ingress %s blocked by Basic gate: %d", path, w.Code)
		}
	}
	for _, path := range []string{
		"/api/v1/sewing/sim/scan", "/api/v1/cutting/releases",
		"/api/v1/material/issues", "/api/v1/sewing/production/unknown",
	} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("POST", path, nil))
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("legacy or unknown path %s bypassed gate: %d", path, w.Code)
		}
	}
}
