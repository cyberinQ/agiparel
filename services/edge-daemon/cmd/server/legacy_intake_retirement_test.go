package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLegacyOperationalImportAndPlanLockAreDisabled(t *testing.T) {
	s := &EdgeServer{} // deliberate: endpoints must never touch DB, stores or ledger.
	for _, tc := range []struct {
		name, method, url string
		handler           http.HandlerFunc
		code              int
	}{
		{"factory CSV write", http.MethodPost, "/api/v1/onboarding/upload", s.handleOnboardingUpload, http.StatusGone},
		{"planning document write", http.MethodPost, "/api/v1/planning/upload-docs", s.handlePlanningUploadDocs, http.StatusGone},
		{"legacy assessment", http.MethodGet, "/api/v1/planning/assessment?po=REP3", s.handlePlanningAssessment, http.StatusGone},
		{"legacy plan locking", http.MethodPost, "/api/v1/planning/approve", s.handlePlanningApprove, http.StatusConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(tc.method, tc.url, strings.NewReader("untrusted,factory,data"))
			tc.handler(w, r)
			if w.Code != tc.code {
				t.Fatalf("legacy endpoint returned %d, expected %d: %s", w.Code, tc.code, w.Body.String())
			}
			if !strings.Contains(strings.ToLower(w.Body.String()), "legacy") {
				t.Fatalf("not explanatory: %s", w.Body.String())
			}
		})
	}
}
