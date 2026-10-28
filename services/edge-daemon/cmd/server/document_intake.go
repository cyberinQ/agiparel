// Author: Sunil Jain, 2026-10-27 — Preserve immutable revisions; identify duplicate bytes before repeated scanning.
// Gate 06.5-A: tenant-pinned, review-only universal source intake gateway.
// A separate document-intake worker consumes durable queued jobs.
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/document-intake/extract"
	"github.com/cyberinq/agiparel/services/edge-daemon/internal/web"
)

type docSummary struct {
	ID             string    `json:"id"`
	Filename       string    `json:"filename"`
	Format         string    `json:"format"`
	MediaType      string    `json:"media_type"`
	Bytes          int       `json:"bytes"`
	SHA256         string    `json:"sha256"`
	Status         string    `json:"status"`
	Outcome        string    `json:"outcome"`
	Segments       int       `json:"segment_count"`
	CapturedAt     time.Time `json:"captured_at"`
	AlreadyPresent bool      `json:"already_present,omitempty"` // An identical SHA-256 is already stored for this tenant.
	NewVersion     bool      `json:"new_version,omitempty"`     // New contents at an existing filename.
	Version        int       `json:"version,omitempty"`
	VersionCount   int       `json:"version_count,omitempty"`
	CurrentVersion bool      `json:"current_version"`
}
type docUploadResult struct {
	Items       []docSummary `json:"items"`
	Errors      []string     `json:"errors"`
	ReviewState string       `json:"review_state"`
}

func docJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
func randomDocID() (string, error) {
	var b [16]byte
	_, e := rand.Read(b[:])
	return "DOC-" + hex.EncodeToString(b[:]), e
}
func acceptableDocumentName(name string) bool {
	if name == "" || len(name) > 255 || strings.ContainsAny(name, "/\\\x00\r\n") || filepath.Base(name) != name {
		return false
	}
	return !strings.HasPrefix(name, ".")
}

func (s *EdgeServer) docTransaction(ctx context.Context, tenant string) (*sql.Tx, error) {
	tx, e := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		return nil, e
	}
	if _, e = tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id',$1,true)`, tenant); e != nil {
		_ = tx.Rollback()
		return nil, e
	}
	return tx, nil
}

// existingDocument checks bytes already admitted under the same tenant. It does not
// trust or write the new payload, so an exact retry needs no second security scan.
func (s *EdgeServer) existingDocument(ctx context.Context, tenant string, data []byte) (docSummary, bool, error) {
	var item docSummary
	hash := sha256.Sum256(data)
	tx, err := s.docTransaction(ctx, tenant)
	if err != nil {
		return item, false, err
	}
	defer tx.Rollback()
	err = tx.QueryRowContext(ctx, `SELECT d.id,d.filename,d.format,d.media_type,d.byte_length,d.sha256,j.status,j.outcome,j.segment_count,d.captured_at
 FROM document_sources_v65 d JOIN document_jobs_v65 j ON j.tenant_id=d.tenant_id AND j.document_id=d.id
 WHERE d.tenant_id=$1 AND d.sha256=$2`, tenant, hex.EncodeToString(hash[:])).Scan(
		&item.ID, &item.Filename, &item.Format, &item.MediaType, &item.Bytes, &item.SHA256, &item.Status, &item.Outcome, &item.Segments, &item.CapturedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return item, false, nil
	}
	if err != nil {
		return item, false, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT count(*), count(*) FILTER (WHERE captured_at < $3 OR (captured_at=$3 AND id <= $4))
 FROM document_sources_v65 WHERE tenant_id=$1 AND lower(filename)=lower($2)`, tenant, item.Filename, item.CapturedAt, item.ID).Scan(&item.VersionCount, &item.Version); err != nil {
		return item, false, err
	}
	item.CurrentVersion = item.Version == item.VersionCount
	item.AlreadyPresent = true
	if err = tx.Commit(); err != nil {
		return item, false, err
	}
	return item, true, nil
}

func (s *EdgeServer) saveDocument(ctx context.Context, tenant, name string, data []byte) (docSummary, error) {
	var item docSummary
	format, media := extract.Detect(name, data)
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	id, e := randomDocID()
	if e != nil {
		return item, e
	}
	tx, e := s.docTransaction(ctx, tenant)
	if e != nil {
		return item, e
	}
	defer tx.Rollback()
	// Filename determines the logical series; bytes determine immutable source identity.
	// Advisory transaction lock serializes concurrent revisions for this tenant/file.
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, tenant+"/"+strings.ToLower(name)); e != nil {
		return item, e
	}
	var previous int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM document_sources_v65 WHERE tenant_id=$1 AND lower(filename)=lower($2)`, tenant, name).Scan(&previous); e != nil {
		return item, e
	}
	// Immutable source: ON CONFLICT returns the existing id via SELECT, never overwrites bytes.
	result, e := tx.ExecContext(ctx, `INSERT INTO document_sources_v65(tenant_id,id,filename,media_type,format,byte_length,sha256,original)
  VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(tenant_id,sha256) DO NOTHING`, tenant, id, name, media, format, len(data), hash, data)
	if e != nil {
		return item, e
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		item.AlreadyPresent = true
		if e = tx.QueryRowContext(ctx, `SELECT id FROM document_sources_v65 WHERE tenant_id=$1 AND sha256=$2`, tenant, hash).Scan(&id); e != nil {
			return item, e
		}
	}
	if n != 0 {
		if _, e = tx.ExecContext(ctx, `INSERT INTO document_jobs_v65(tenant_id,document_id) VALUES($1,$2)`, tenant, id); e != nil {
			return item, e
		}
	}
	if e = tx.QueryRowContext(ctx, `SELECT d.id,d.filename,d.format,d.media_type,d.byte_length,d.sha256,j.status,j.outcome,j.segment_count,d.captured_at
   FROM document_sources_v65 d JOIN document_jobs_v65 j ON j.tenant_id=d.tenant_id AND j.document_id=d.id
   WHERE d.tenant_id=$1 AND d.id=$2`, tenant, id).Scan(&item.ID, &item.Filename, &item.Format, &item.MediaType, &item.Bytes, &item.SHA256, &item.Status, &item.Outcome, &item.Segments, &item.CapturedAt); e != nil {
		return item, e
	}
	if n == 0 {
		// A byte-identical upload may race through preflight, so report it accurately.
		if e = tx.QueryRowContext(ctx, `SELECT count(*), count(*) FILTER (WHERE captured_at < $3 OR (captured_at=$3 AND id <= $4)) FROM document_sources_v65 WHERE tenant_id=$1 AND lower(filename)=lower($2)`, tenant, item.Filename, item.CapturedAt, item.ID).Scan(&item.VersionCount, &item.Version); e != nil {
			return item, e
		}
	} else {
		item.Version = previous + 1
		item.VersionCount = item.Version
		item.NewVersion = previous > 0
	}
	item.CurrentVersion = item.Version == item.VersionCount
	if e = tx.Commit(); e != nil {
		return item, e
	}
	return item, nil
}
func (s *EdgeServer) handleDocumentUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", 405)
		return
	}
	tenant, e := intakeTenant()
	if e != nil {
		http.Error(w, "Operator tenant not configured", 503)
		return
	}
	if s.db == nil {
		http.Error(w, "PostgreSQL unavailable", 503)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 60<<20)
	if e = r.ParseMultipartForm(8 << 20); e != nil {
		http.Error(w, "Cannot read multipart upload (maximum request 60 MiB)", 413)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	if len(r.MultipartForm.File) != 1 || len(r.MultipartForm.File["files"]) == 0 || len(r.MultipartForm.File["files"]) > 10 {
		http.Error(w, "Upload 1–10 original documents in files field", 400)
		return
	}
	w.Header().Set("X-Agiparel-Upload-Build", "20261027.v9")
	response := docUploadResult{Items: []docSummary{}, Errors: []string{}, ReviewState: "UNREVIEWED"}
	for _, h := range r.MultipartForm.File["files"] {
		name := h.Filename
		if !acceptableDocumentName(name) || h.Size < 1 || h.Size > extract.MaxBytes {
			response.Errors = append(response.Errors, fmt.Sprintf("%q: invalid name or size (20 MiB per file)", name))
			continue
		}
		f, er := h.Open()
		if er != nil {
			response.Errors = append(response.Errors, fmt.Sprintf("%q: cannot open", name))
			continue
		}
		data, er := io.ReadAll(io.LimitReader(f, extract.MaxBytes+1))
		_ = f.Close()
		if er != nil || len(data) > extract.MaxBytes || len(data) == 0 {
			response.Errors = append(response.Errors, fmt.Sprintf("%q: upload too large or unreadable", name))
			continue
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		// Idempotent retries do not introduce new bytes: check the admitted SHA-256
		// before legacy-format/AV restrictions, but NEVER skip screening for changed bytes.
		old, found, lookupErr := s.existingDocument(ctx, tenant, data)
		if lookupErr != nil {
			cancel()
			response.Errors = append(response.Errors, fmt.Sprintf("%q: cannot check existing files; retry later", name))
			continue
		}
		if found {
			cancel()
			response.Items = append(response.Items, old)
			continue
		}
		if scanErr := checkDocumentSafety(ctx, name, data); scanErr != nil {
			cancel()
			response.Errors = append(response.Errors, fmt.Sprintf("%q: %s", name, scanErr.Error()))
			continue
		}
		item, er := s.saveDocument(ctx, tenant, name, data)
		cancel()
		if er != nil {
			response.Errors = append(response.Errors, fmt.Sprintf("%q: database staging failed", name))
			continue
		}
		response.Items = append(response.Items, item)
	}
	code := http.StatusAccepted
	if len(response.Items) == 0 {
		code = http.StatusBadRequest
	}
	docJSON(w, code, response)
}

// Author: Sunil Jain, 2026-10-28. Tenant-scoped keyset paging with explicit end-of-catalog evidence.
func (s *EdgeServer) handleDocuments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tenant, e := intakeTenant()
	if e != nil {
		http.Error(w, "Operator tenant not configured", 503)
		return
	}
	if s.db == nil {
		http.Error(w, "PostgreSQL unavailable", 503)
		return
	}
	pageSize, e := docPageSize(r.URL.Query().Get("limit"))
	if e != nil {
		http.Error(w, e.Error(), http.StatusBadRequest)
		return
	}
	cursor, e := docCursorDecode(r.URL.Query().Get("cursor"))
	if e != nil {
		http.Error(w, e.Error(), http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, e := s.docTransaction(ctx, tenant)
	if e != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	defer tx.Rollback()
	// Window functions calculate versions over the WHOLE tenant, never the page.
	// The outer keyset filter does not corrupt current-version determination.
	rows, e := tx.QueryContext(ctx, `WITH numbered AS (
  SELECT d.tenant_id,d.id,d.filename,d.format,d.media_type,d.byte_length,d.sha256,d.captured_at,
   ROW_NUMBER() OVER(PARTITION BY lower(d.filename) ORDER BY d.captured_at,d.id) AS version,
   COUNT(*) OVER(PARTITION BY lower(d.filename)) AS version_count
  FROM document_sources_v65 d WHERE d.tenant_id=$1
 ) SELECT d.id,d.filename,d.format,d.media_type,d.byte_length,d.sha256,j.status,j.outcome,j.segment_count,d.captured_at,
 d.version,d.version_count,(d.version=d.version_count) AS current_version
 FROM numbered d JOIN document_jobs_v65 j ON j.tenant_id=d.tenant_id AND j.document_id=d.id
 WHERE ($2::timestamptz IS NULL OR (d.captured_at,d.id) < ($2::timestamptz,$3::text))
 ORDER BY d.captured_at DESC,d.id DESC LIMIT $4`, tenant, docNullableTime(cursor.CapturedAt), cursor.ID, pageSize+1)
	if e != nil {
		http.Error(w, "Document list unavailable", 503)
		return
	}
	items := make([]docSummary, 0, pageSize+1)
	for rows.Next() {
		var item docSummary
		if e = rows.Scan(&item.ID, &item.Filename, &item.Format, &item.MediaType, &item.Bytes, &item.SHA256, &item.Status, &item.Outcome, &item.Segments, &item.CapturedAt, &item.Version, &item.VersionCount, &item.CurrentVersion); e != nil {
			break
		}
		items = append(items, item)
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		http.Error(w, "Cannot read documents", 503)
		return
	}
	hasMore := len(items) > pageSize
	if hasMore {
		items = items[:pageSize]
	}
	next := ""
	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		next = docCursorEncode(last.CapturedAt, last.ID)
	}
	if e = tx.Commit(); e != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	docJSON(w, 200, map[string]any{"items": items, "tenant": tenant, "review_state": "UNREVIEWED", "next_cursor": next, "has_more": hasMore, "page_limit": pageSize})
}

func (s *EdgeServer) handleDocumentDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", 405)
		return
	}
	tenant, e := intakeTenant()
	if e != nil {
		http.Error(w, "Operator tenant not configured", 503)
		return
	}
	if s.db == nil {
		http.Error(w, "PostgreSQL unavailable", 503)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/documents/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || len(parts) > 2 || !strings.HasPrefix(parts[0], "DOC-") || len(parts[0]) != 36 {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	for _, c := range id[4:] {
		if !strings.ContainsRune("0123456789abcdef", c) {
			http.NotFound(w, r)
			return
		}
	}
	action := "detail"
	if len(parts) == 2 {
		action = parts[1]
	}
	if action != "detail" && action != "source" {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	tx, e := s.docTransaction(ctx, tenant)
	if e != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	defer tx.Rollback()
	var item docSummary
	e = tx.QueryRowContext(ctx, `SELECT d.id,d.filename,d.format,d.media_type,d.byte_length,d.sha256,j.status,j.outcome,j.segment_count,d.captured_at
 FROM document_sources_v65 d JOIN document_jobs_v65 j ON j.tenant_id=d.tenant_id AND j.document_id=d.id
 WHERE d.tenant_id=$1 AND d.id=$2`, tenant, id).Scan(&item.ID, &item.Filename, &item.Format, &item.MediaType, &item.Bytes, &item.SHA256, &item.Status, &item.Outcome, &item.Segments, &item.CapturedAt)
	if errors.Is(e, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if e != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	if action == "source" {
		var data []byte
		if e = tx.QueryRowContext(ctx, `SELECT original FROM document_sources_v65 WHERE tenant_id=$1 AND id=$2`, tenant, id).Scan(&data); e != nil {
			http.Error(w, "Cannot retrieve original", 503)
			return
		}
		if e = tx.Commit(); e != nil {
			http.Error(w, "Database unavailable", 503)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", item.Filename))
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(data)
		return
	}
	type seg struct {
		Ordinal int             `json:"ordinal"`
		Kind    string          `json:"kind"`
		Locator json.RawMessage `json:"locator"`
		Content string          `json:"content"`
	}
	segments := []seg{}
	after, err := docSegmentAfter(r.URL.Query().Get("after_ordinal"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	rows, e := tx.QueryContext(ctx, `SELECT ordinal,kind,locator,content FROM document_segments_v65 WHERE tenant_id=$1 AND document_id=$2 AND ordinal > $3 ORDER BY ordinal LIMIT $4`, tenant, id, after, docSegmentsPageMax+1)
	if e != nil {
		http.Error(w, "Segments unavailable", 503)
		return
	}
	for rows.Next() {
		var v seg
		var locator []byte
		if e = rows.Scan(&v.Ordinal, &v.Kind, &locator, &v.Content); e != nil {
			break
		}
		v.Locator = json.RawMessage(locator)
		segments = append(segments, v)
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		http.Error(w, "Segment read failure", 503)
		return
	}
	if e = tx.Commit(); e != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	hasMore := len(segments) > docSegmentsPageMax
	if hasMore {
		segments = segments[:docSegmentsPageMax]
	}
	nextOrdinal := 0
	if hasMore && len(segments) > 0 {
		nextOrdinal = segments[len(segments)-1].Ordinal
	}
	docJSON(w, 200, map[string]any{"document": item, "segments": segments, "segment_limit": docSegmentsPageMax, "has_more_segments": hasMore, "next_ordinal": nextOrdinal, "evidence_state": "UNREVIEWED", "ready_for_order_intake": false})
}
func (s *EdgeServer) handleDocumentWorkspace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", 405)
		return
	}
	// Development uses a fixed operator tenant. Production has no approved
	// browser actor/plant identity for this document inbox yet: do not display
	// an operable form that inevitably fails, or imply static Basic Auth is SSO.
	if _, err := intakeTenant(); err != nil {
		http.Error(w, "Document inbox is unavailable until workforce identity and tenant-scoped authorization are provisioned", http.StatusServiceUnavailable)
		return
	}
	data, e := web.GetDocumentsHTML()
	if e != nil {
		http.Error(w, "Documents workspace unavailable", 503)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}
