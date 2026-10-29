// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTrimRoutesFailClosedWhenMaterialDisabled(t *testing.T) {
	t.Setenv("AGIPAREL_MATERIAL_API_ENABLED", "false")
	mux := http.NewServeMux()
	registerMaterialRoutes(mux, nil)
	for _, path := range []string{"/api/v1/material/trims/issues/challenge", "/api/v1/material/trims/issues"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("POST", path, strings.NewReader("{}")))
		if w.Code != 503 {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
}
func TestTrimBodyCannotCarryArbitraryWitnessIDs(t *testing.T) {
	var p trimIssueBody
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"trim_stock_id":"TR-1","order_id":"ORDER-1","line_id":"LINE-1","quantity_milli":1000,"command_id":"trim_command_0001","issuer_id":"spoof","issuer_attestation":"jwt-a","recipient_attestation":"jwt-b"}`))
	if err := decodeMaterialJSON(w, r, &p); err == nil {
		t.Fatal("accepted spoofed principal")
	}
}
