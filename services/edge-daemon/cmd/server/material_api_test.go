package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMaterialAPIStaysDisabledWithoutIdentityAndDatabase(t *testing.T) {
	t.Setenv("AGIPAREL_MATERIAL_API_ENABLED", "false")
	mux := http.NewServeMux()
	registerMaterialRoutes(mux, nil)
	for _, p := range []string{"/api/v1/material/reservations", "/api/v1/material/issues", "/api/v1/material/issues/challenge", "/api/v1/material/variances"} {
		r := httptest.NewRequest(http.MethodPost, p, strings.NewReader("{}"))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s returned %d, expected 503", p, w.Code)
		}
	}
}
func TestMaterialIssueBodyUsesOnlyAttestationFields(t *testing.T) {
	var q materialIssueBody
	raw := `{"reservation_id":"res-1","roll_id":"roll-1","order_id":"order-1","cut_no":1,"length_mm":5000,"idempotency_key":"issue-1","issuer_attestation":"signed-token-1","recipient_attestation":"signed-token-2"}`
	if err := json.Unmarshal([]byte(raw), &q); err != nil {
		t.Fatal(err)
	}
	if q.ReservationID != "res-1" || q.IssuerAttestation != "signed-token-1" || q.RecipientAttestation != "signed-token-2" {
		t.Fatalf("incorrect contract: %+v", q)
	}
}
