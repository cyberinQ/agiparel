// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEngineerOpsV43_2ActualInventoryAndUnknownMetrics(t *testing.T) {
	t.Setenv("AGIPAREL_ENGINEER_OPS_ENABLED", "true")
	t.Setenv("AGIPAREL_ENGINEER_OPS_USER", "platform-admin")
	t.Setenv("AGIPAREL_ENGINEER_OPS_PASSWORD", "correct-this-is-a-long-random-ops-secret")
	engineerOpsRequestsSinceStart.Store(0)
	m := opsMuxForTest()
	handler := isolateEngineerOps(operatorAccessGate(m, "dev", "1212"), m)

	for _, path := range []string{"/", "/healthz", "/demo-assets/portal.css"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "https://agiparel.empowerai.us"+path, nil))
	}

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, reqOps("/api/v1/engineer-ops/overview", true))
	if w.Code != http.StatusOK {
		t.Fatalf("overview status %d: %s", w.Code, w.Body.String())
	}
	var out engineerOpsOverview
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.APIs) < 160 {
		t.Fatalf("API catalog unexpectedly short: %d", len(out.APIs))
	}
	if out.Analytics.RequestsSinceStart != 1 {
		t.Fatalf("unexpected counted requests: %d", out.Analytics.RequestsSinceStart)
	}
	if out.Analytics.CustomerCount != nil || out.Analytics.UserCount != nil {
		t.Fatal("No database: customer and user counts must be unknown, not fabricated zero")
	}
	if !strings.Contains(out.Analytics.SourceStatus, "unavailable") {
		t.Fatalf("missing source limitation: %q", out.Analytics.SourceStatus)
	}
}
