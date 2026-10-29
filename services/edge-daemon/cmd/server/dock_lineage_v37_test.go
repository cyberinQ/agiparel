// Author: Sunil Jain, 2026-10-29.
// Verify the v37 cross-ledger audit is opt-in and blocked by upstream identity gates.
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDockLineageV37DisabledByDefault(t *testing.T) {
	t.Setenv("AGIPAREL_DOCK_LINEAGE_AUDIT_V37_ENABLED", "")
	if dockLineageAuditV37Enabled() {
		t.Fatal("audit enabled by default")
	}
	t.Setenv("AGIPAREL_DOCK_LINEAGE_AUDIT_V37_ENABLED", "TRUE")
	if dockLineageAuditV37Enabled() {
		t.Fatal("noncanonical toggle accepted")
	}
	t.Setenv("AGIPAREL_DOCK_LINEAGE_AUDIT_V37_ENABLED", "true")
	if !dockLineageAuditV37Enabled() {
		t.Fatal("opt-in ignored")
	}
	t.Setenv("AGIPAREL_DOCK_EXTERNAL_V35_ENABLED", "false")
	p := newDockExternalAPIv35(&EdgeServer{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/packing/dock/lineage-audit", nil)
	p.status(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("upstream disabled but got %d", w.Code)
	}
}
