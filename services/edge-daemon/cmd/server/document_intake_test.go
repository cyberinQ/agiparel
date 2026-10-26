package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDocumentIntakeWorkspace(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/documents", nil)
	w := httptest.NewRecorder()
	(&EdgeServer{}).handleDocumentWorkspace(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "One place for every file.") {
		t.Fatalf("workspace status=%d body=%q", w.Code, w.Body.String())
	}
}
func TestDocumentIntakeFailClosedWithoutTenant(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_OPERATOR_TENANT", "")
	w := httptest.NewRecorder()
	(&EdgeServer{}).handleDocuments(w, httptest.NewRequest("GET", "/api/v1/documents", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 without tenant got %d", w.Code)
	}
}
func TestDocumentFilenameGuard(t *testing.T) {
	for _, name := range []string{"", "../bad.csv", "folder/bad.pdf", ".hidden", "bad\x00.pdf", "bad\\path.docx"} {
		if acceptableDocumentName(name) {
			t.Errorf("accepted %q", name)
		}
	}
	if !acceptableDocumentName("PO REP3.pdf") {
		t.Fatal("valid buyer document rejected")
	}
}
func TestDocumentIntakeReadOnlyByDefault(t *testing.T) {
	s := &EdgeServer{}
	for _, item := range []struct {
		run    func(http.ResponseWriter, *http.Request)
		url    string
		method string
	}{
		{s.handleDocumentUpload, "/api/v1/documents/uploads", "GET"},
		{s.handleDocuments, "/api/v1/documents", "POST"},
		{s.handleDocumentDetail, "/api/v1/documents/DOC-123", "POST"},
	} {
		w := httptest.NewRecorder()
		item.run(w, httptest.NewRequest(item.method, item.url, nil))
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s status=%d", item.method, item.url, w.Code)
		}
	}
}
