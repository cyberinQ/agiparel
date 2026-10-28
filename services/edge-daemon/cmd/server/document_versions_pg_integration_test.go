// Author: Sunil Jain, 2026-10-27 — Verify revision history, exact-duplicate recognition, and original bytes.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Runs only against explicitly opted-in PostgreSQL scratch fixtures, never factory tenants.
func TestDocumentUploadVersionsPostgres(t *testing.T) {
	adminURL, appURL := os.Getenv("AGIPAREL_TEST_ADMIN_DATABASE_URL"), os.Getenv("AGIPAREL_TEST_APP_DATABASE_URL")
	if adminURL == "" || appURL == "" {
		t.Skip("PostgreSQL fixtures not configured")
	}
	if os.Getenv("AGIPAREL_DESTRUCTIVE_DB_TESTS") != "I_UNDERSTAND_TEST_FIXTURES" {
		t.Fatal("explicit scratch-tenant opt-in required")
	}
	admin, err := sql.Open("pgx", adminURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	app, err := sql.Open("pgx", appURL)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var rnd [16]byte
	if _, err = rand.Read(rnd[:]); err != nil {
		t.Fatal(err)
	}
	tenant := "DV-" + hex.EncodeToString(rnd[:])[:13]
	if _, err = admin.ExecContext(ctx, `INSERT INTO tenants(id,name) VALUES($1,'Document versions fixture')`, tenant); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, q := range []string{`DELETE FROM document_segments_v65 WHERE tenant_id=$1`, `DELETE FROM document_jobs_v65 WHERE tenant_id=$1`, `DELETE FROM document_sources_v65 WHERE tenant_id=$1`, `DELETE FROM tenants WHERE id=$1`} {
			_, _ = admin.ExecContext(context.Background(), q, tenant)
		}
	})
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_OPERATOR_TENANT", tenant)
	t.Setenv("AGIPAREL_CLAMD_ADDR", "")
	server := &EdgeServer{db: app}
	upload := func(name, content string) docSummary {
		t.Helper()
		var b bytes.Buffer
		form := multipart.NewWriter(&b)
		f, e := form.CreateFormFile("files", name)
		if e != nil {
			t.Fatal(e)
		}
		_, _ = io.WriteString(f, content)
		_ = form.Close()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/documents/uploads", &b)
		req.Header.Set("Content-Type", form.FormDataContentType())
		w := httptest.NewRecorder()
		server.handleDocumentUpload(w, req)
		var result docUploadResult
		if e = json.Unmarshal(w.Body.Bytes(), &result); e != nil {
			t.Fatal(e)
		}
		if w.Code != http.StatusAccepted || len(result.Items) != 1 || len(result.Errors) != 0 {
			t.Fatalf("upload %q status=%d result=%+v", name, w.Code, result)
		}
		return result.Items[0]
	}
	first := upload("garment.csv", "material,quantity\nCOTTON,100\n")
	if first.NewVersion || first.AlreadyPresent || first.Version != 1 {
		t.Fatalf("first=%+v", first)
	}
	repeat := upload("garment.csv", "material,quantity\nCOTTON,100\n")
	if !repeat.AlreadyPresent || repeat.ID != first.ID {
		t.Fatalf("duplicate=%+v", repeat)
	}
	revised := upload("garment.csv", "material,quantity\nCOTTON,200\n")
	if !revised.NewVersion || revised.AlreadyPresent || revised.ID == first.ID || revised.Version != 2 {
		t.Fatalf("revision=%+v", revised)
	}
	w := httptest.NewRecorder()
	server.handleDocuments(w, httptest.NewRequest("GET", "/api/v1/documents", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("list: %d %s", w.Code, w.Body.String())
	}
	var listing struct {
		Items []docSummary `json:"items"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &listing); err != nil {
		t.Fatal(err)
	}
	if len(listing.Items) != 2 {
		t.Fatalf("count=%d", len(listing.Items))
	}
	var current, previous int
	for _, entry := range listing.Items {
		if entry.VersionCount != 2 {
			t.Fatalf("wrong count: %+v", entry)
		}
		if entry.CurrentVersion {
			current++
			if entry.ID != revised.ID {
				t.Fatalf("wrong current: %+v", entry)
			}
		} else {
			previous++
			if entry.ID != first.ID {
				t.Fatalf("lost earlier version: %+v", entry)
			}
		}
	}
	if current != 1 || previous != 1 {
		t.Fatalf("current=%d previous=%d", current, previous)
	}
	// Previously admitted legacy binary bytes must report AlreadyPresent, even
	// without a configured AV service; a changed binary must still fail closed.
	legacy := append([]byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}, []byte("old-binary-record")...)
	legacySaved, legacyErr := server.saveDocument(ctx, tenant, "purchase.xls", legacy)
	if legacyErr != nil {
		t.Fatal(legacyErr)
	}
	legacyRepeat := upload("purchase.xls", string(legacy))
	if !legacyRepeat.AlreadyPresent || legacyRepeat.ID != legacySaved.ID {
		t.Fatalf("legacy duplicate=%+v", legacyRepeat)
	}
	legacyChanged := append([]byte{}, legacy...)
	legacyChanged = append(legacyChanged, 'X')
	var legacyForm bytes.Buffer
	legacyWriter := multipart.NewWriter(&legacyForm)
	legacyPart, err := legacyWriter.CreateFormFile("files", "purchase.xls")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = legacyPart.Write(legacyChanged)
	_ = legacyWriter.Close()
	legacyRequest := httptest.NewRequest(http.MethodPost, "/api/v1/documents/uploads", &legacyForm)
	legacyRequest.Header.Set("Content-Type", legacyWriter.FormDataContentType())
	legacyResponse := httptest.NewRecorder()
	server.handleDocumentUpload(legacyResponse, legacyRequest)
	var rejected docUploadResult
	if err = json.Unmarshal(legacyResponse.Body.Bytes(), &rejected); err != nil {
		t.Fatal(err)
	}
	if legacyResponse.Code != http.StatusBadRequest || len(rejected.Items) != 0 || len(rejected.Errors) != 1 {
		t.Fatalf("changed legacy should require AV: %+v", rejected)
	}
	for _, tc := range []struct{ id, want string }{{first.ID, "material,quantity\nCOTTON,100\n"}, {revised.ID, "material,quantity\nCOTTON,200\n"}} {
		w = httptest.NewRecorder()
		server.handleDocumentDetail(w, httptest.NewRequest("GET", "/api/v1/documents/"+tc.id+"/source", nil))
		if w.Code != http.StatusOK || w.Body.String() != tc.want {
			t.Fatalf("source %s status=%d body=%q", tc.id, w.Code, w.Body.String())
		}
	}
}
