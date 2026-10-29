// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestF314BAndF315BRoutesStayDisabled(t *testing.T) {
	t.Setenv("AGIPAREL_MATERIAL_API_ENABLED", "false")
	mux := http.NewServeMux()
	registerMaterialRoutes(mux, nil)
	for _, methodPath := range []struct{ method, path string }{
		{"POST", "/api/v1/material/trims/admissions/challenge"}, {"POST", "/api/v1/material/trims/admissions"},
		{"GET", "/api/v1/material/readiness/source?work_order_id=WO-1"},
	} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(methodPath.method, methodPath.path, strings.NewReader("{}")))
		if w.Code != 503 {
			t.Fatalf("%s returned %d", methodPath.path, w.Code)
		}
	}
}
func TestF314BRejectsSpoofedWitnessesInBody(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"stock_id":"TR-1","trim_code":"ZIP","supplier_lot":"L1","uom":"PCS","quantity_milli":1000,"manager_subject":"spoof","manager_attestation":"a","inspector_attestation":"b"}`))
	var p trimAdmissionBody
	if err := decodeMaterialJSON(w, r, &p); err == nil {
		t.Fatal("accepted self-declared principal")
	}
}
