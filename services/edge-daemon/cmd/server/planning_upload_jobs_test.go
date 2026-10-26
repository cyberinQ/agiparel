// Author: Sunil Jain, 2026-10-25
// Verify intake job polling, method restrictions, and non-disclosing job identifiers.
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIntakeJobStatusReadOnly(t *testing.T) {
	id, err := newIntakeJobID()
	if err != nil || len(id) != 32 {
		t.Fatalf("job id: %q, %v", id, err)
	}
	storeIntakeJob(intakeJob{ID: id, Status: "PROCESSING", Files: []string{"order.pdf"}, Created: time.Now().UTC()})
	srv := &EdgeServer{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/planning/upload-docs?job="+id, nil)
	rr := httptest.NewRecorder()
	srv.handlePlanningUploadDocs(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Header().Get("Cache-Control"), "no-store") {
		t.Fatal("missing no-store")
	}
	var result intakeJob
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.ID != id || result.Status != "PROCESSING" {
		t.Fatalf("wrong job: %+v", result)
	}
}

func TestIntakeUploadRejectedMethods(t *testing.T) {
	srv := &EdgeServer{}
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		rr := httptest.NewRecorder()
		srv.handlePlanningUploadDocs(rr, httptest.NewRequest(method, "/api/v1/planning/upload-docs", nil))
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: got %d", method, rr.Code)
		}
	}
	rr := httptest.NewRecorder()
	srv.handlePlanningUploadDocs(rr, httptest.NewRequest(http.MethodGet, "/api/v1/planning/upload-docs?job=unknown", nil))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("bad id: %d", rr.Code)
	}
}
