// Author: Sunil Jain, 2026-10-25
// Check the additive preview API cannot be used for writes.
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/planning"
)

func TestReadinessQuestionsReadOnly(t *testing.T) {
	s := &EdgeServer{}
	post := httptest.NewRecorder()
	s.handlePlanningReadinessQuestions(post, httptest.NewRequest(http.MethodPost, "/api/v1/planning/readiness-questions", nil))
	if post.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d", post.Code)
	}

	longPO := httptest.NewRecorder()
	s.handlePlanningReadinessQuestions(longPO, httptest.NewRequest(http.MethodGet, "/api/v1/planning/readiness-questions?po="+strings.Repeat("A", 129), nil))
	if longPO.Code != http.StatusBadRequest {
		t.Fatalf("long PO status = %d", longPO.Code)
	}

	get := httptest.NewRecorder()
	s.handlePlanningReadinessQuestions(get, httptest.NewRequest(http.MethodGet, "/api/v1/planning/readiness-questions?po=TEST", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("GET status = %d: %s", get.Code, get.Body.String())
	}
	var out planning.ReadinessInterview
	if err := json.Unmarshal(get.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.ReadyForApproval {
		t.Fatal("preview must not approve a plan")
	}
	if len(out.Questions) == 0 {
		t.Fatal("expected questions without order evidence")
	}
}
