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
	ID         string    `json:"id"`
	Filename   string    `json:"filename"`
	Format     string    `json:"format"`
	MediaType  string    `json:"media_type"`
	Bytes      int       `json:"bytes"`
	SHA256     string    `json:"sha256"`
	Status     string    `json:"status"`
	Outcome    string    `json:"outcome"`
	Segments   int       `json:"segment_count"`
	CapturedAt time.Time `json:"captured_at"`
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
	// Immutable source: ON CONFLICT returns the existing id via SELECT, never overwrites bytes.
	result, e := tx.ExecContext(ctx, `INSERT INTO document_sources_v65(tenant_id,id,filename,media_type,format,byte_length,sha256,original)
  VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(tenant_id,sha256) DO NOTHING`, tenant, id, name, media, format, len(data), hash, data)
	if e != nil {
		return item, e
	}
	n, _ := result.RowsAffected()
	if n == 0 {
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
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
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

func (s *EdgeServer) handleDocuments(w http.ResponseWriter, r *http.Request) {
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
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, e := s.docTransaction(ctx, tenant)
	if e != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, `SELECT d.id,d.filename,d.format,d.media_type,d.byte_length,d.sha256,j.status,j.outcome,j.segment_count,d.captured_at
 FROM document_sources_v65 d JOIN document_jobs_v65 j ON j.tenant_id=d.tenant_id AND j.document_id=d.id
 WHERE d.tenant_id=$1 ORDER BY d.captured_at DESC LIMIT 100`, tenant)
	if e != nil {
		http.Error(w, "Document list unavailable", 503)
		return
	}
	items := []docSummary{}
	for rows.Next() {
		var item docSummary
		if e = rows.Scan(&item.ID, &item.Filename, &item.Format, &item.MediaType, &item.Bytes, &item.SHA256, &item.Status, &item.Outcome, &item.Segments, &item.CapturedAt); e != nil {
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
	if e = tx.Commit(); e != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	docJSON(w, 200, map[string]any{"items": items, "tenant": tenant, "review_state": "UNREVIEWED"})
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
	rows, e := tx.QueryContext(ctx, `SELECT ordinal,kind,locator,content FROM document_segments_v65 WHERE tenant_id=$1 AND document_id=$2 ORDER BY ordinal LIMIT 200`, tenant, id)
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
	docJSON(w, 200, map[string]any{"document": item, "segments": segments, "segment_limit": 200, "evidence_state": "UNREVIEWED", "ready_for_order_intake": false})
}
func (s *EdgeServer) handleDocumentWorkspace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", 405)
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
