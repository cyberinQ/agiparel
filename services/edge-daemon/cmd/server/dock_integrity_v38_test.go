// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// Test v38 integrity endpoint cannot bypass disabled external controls.
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDockIntegrityV38FailClosed(t *testing.T) {
	t.Setenv("AGIPAREL_DOCK_INTEGRITY_V38_ENABLED", "")
	if dockIntegrityReportV38Enabled() {
		t.Fatal("default open")
	}
	t.Setenv("AGIPAREL_DOCK_INTEGRITY_V38_ENABLED", "TRUE")
	if dockIntegrityReportV38Enabled() {
		t.Fatal("noncanonical opt-in")
	}
	t.Setenv("AGIPAREL_DOCK_INTEGRITY_V38_ENABLED", "true")
	if !dockIntegrityReportV38Enabled() {
		t.Fatal("canonical opt-in not recognized")
	}
	t.Setenv("AGIPAREL_DOCK_EXTERNAL_V35_ENABLED", "false")
	p := newDockExternalAPIv35(&EdgeServer{})
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		req := httptest.NewRequest(method, "/api/v1/packing/dock/integrity-report", nil)
		out := httptest.NewRecorder()
		p.status(out, req)
		if out.Code != http.StatusServiceUnavailable && out.Code != http.StatusMethodNotAllowed {
			t.Fatalf("unexpected status %d for %s", out.Code, method)
		}
	}
}
