// Author: Sunil Jain, 2026-10-28.
// Chapter 07 production API must bypass shared dev Basic Auth only on exact paths.
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQualityProductionAccessGate(t *testing.T) {
	t.Setenv("ENV", "production")
	handler := configuredOperatorGate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	for _, path := range []string{
		"/api/v1/quality/production/events",
		"/api/v1/quality/production/events/challenge",
		"/api/v1/quality/production/catalog",
	} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusNoContent {
			t.Errorf("IdP-secured API %s blocked at shared gate: %d", path, w.Code)
		}
	}
	for _, path := range []string{
		"/api/v1/quality/production/unknown", "/api/v1/quality/sim/catalog",
		"/api/v1/quality/production/catalog/", "/quality",
	} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("Untrusted path %s bypassed operator gate: %d", path, w.Code)
		}
	}
}
