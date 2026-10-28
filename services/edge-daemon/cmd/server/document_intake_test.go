package main

import (
	"encoding/json"
	"github.com/cyberinq/agiparel/services/edge-daemon/internal/web"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDocumentIntakeWorkspace(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_OPERATOR_TENANT", "test-tenant")
	r := httptest.NewRequest(http.MethodGet, "/documents", nil)
	w := httptest.NewRecorder()
	(&EdgeServer{}).handleDocumentWorkspace(w, r)
	// Assert the usable intake workspace rather than a marketing headline.
	if w.Code != http.StatusOK {
		t.Fatalf("workspace status=%d", w.Code)
	}
	for _, required := range []string{`id="files"`, `id="upload"`, `id="document-triage"`, `/static/documents.js`} {
		if !strings.Contains(w.Body.String(), required) {
			t.Errorf("document workspace missing %q", required)
		}
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

// Author: Sunil Jain, 2026-10-27 — Prevent a stale front-end or duplicate count regression.
func TestDocumentUploadV9Contract(t *testing.T) {
	raw, err := web.GetDocumentsHTML()
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{`name="agiparel-upload-version" content="20261028.v10"`, `/static/documents.js?v=20261028.v10`, `/static/review_submit.js?v=20261028.v10`} {
		if !strings.Contains(string(raw), token) {
			t.Errorf("missing versioned asset %q", token)
		}
	}
	sample := docSummary{ID: "DOC-abc", AlreadyPresent: true, Version: 2, VersionCount: 3, CurrentVersion: false}
	b, err := json.Marshal(sample)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"version_count":3`) || !strings.Contains(string(b), `"already_present":true`) {
		t.Errorf("duplicate upload result missing marker: %s", b)
	}
}

// Author: Sunil Jain, 2026-10-28. Production must not expose an unusable
// development identity-scoped document inbox.
func TestDocumentInboxRequiresProductionIdentityV14(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("AGIPAREL_OPERATOR_TENANT", "test-tenant")
	w := httptest.NewRecorder()
	(&EdgeServer{}).handleDocumentWorkspace(w, httptest.NewRequest("GET", "/documents", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("want fail-closed inbox, got %d", w.Code)
	}
}
