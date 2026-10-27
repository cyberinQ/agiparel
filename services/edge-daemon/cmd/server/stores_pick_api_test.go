package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStoresPickAPIFailsClosedWithoutDatabase(t *testing.T) {
	s := &EdgeServer{}
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/api/v1/stores/pick-proposals", nil)
		s.handleStoresPickProposals(w, req)
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s expected 503, got %d", method, w.Code)
		}
	}
}
