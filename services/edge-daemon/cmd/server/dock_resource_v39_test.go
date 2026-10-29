// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// Verify dock status operations remain closed unless explicitly enabled.
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDockV39StatusGuards(t *testing.T) {
	t.Setenv("AGIPAREL_DOCK_EXTERNAL_V35_ENABLED", "false")
	t.Setenv("AGIPAREL_DOCK_INTEGRITY_V38_ENABLED", "true")
	t.Setenv("AGIPAREL_DOCK_DIAGNOSTICS_V36_ENABLED", "true")
	t.Setenv("AGIPAREL_DOCK_LINEAGE_AUDIT_V37_ENABLED", "true")
	p := newDockExternalAPIv35(&EdgeServer{})
	paths := []string{
		"/api/v1/packing/dock/external-verification",
		"/api/v1/packing/dock/diagnostics",
		"/api/v1/packing/dock/lineage-audit",
		"/api/v1/packing/dock/integrity-report",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			p.status(w, httptest.NewRequest(http.MethodGet, path, nil))
			if w.Code != http.StatusServiceUnavailable {
				t.Fatalf("upstream disabled: status %d", w.Code)
			}
			w = httptest.NewRecorder()
			p.status(w, httptest.NewRequest(http.MethodPost, path, nil))
			if w.Code != http.StatusMethodNotAllowed {
				t.Fatalf("write method not rejected: status %d", w.Code)
			}
		})
	}
}
