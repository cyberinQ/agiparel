package web

import (
	"strings"
	"testing"
)

func TestStoresReconciliationUIAndAuthorityBoundary(t *testing.T) {
	b, e := GetStoresHTML()
	if e != nil {
		t.Fatal(e)
	}
	for _, s := range []string{"/static/stores_reconciliation.js", "id=\"reconciliation-form\"", "id=\"reconcile-history\"", "id=\"secure-reservations\"", "independent", "No ledger changes"} {
		if !strings.Contains(strings.ToLower(string(b)), strings.ToLower(s)) {
			t.Errorf("stores view missing %s", s)
		}
	}
	js, e := staticFS.ReadFile("static/stores_reconciliation.js")
	if e != nil {
		t.Fatal(e)
	}
	for _, s := range []string{"/api/v1/stores/reconciliation", "/api/v1/stores/reconciliation-requests", "/api/v1/stores/reservations"} {
		if !strings.Contains(string(js), s) {
			t.Errorf("missing endpoint %s", s)
		}
	}
	if strings.Contains(string(js), "/api/v1/material/issues\"") {
		t.Fatal("browser must not issue stock with Basic credentials")
	}
}
