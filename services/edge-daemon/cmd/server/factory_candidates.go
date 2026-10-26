// Gate 06.5-B2: reviewable candidates from a shared source library. No production writes.
package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

type candidateRow struct {
	ID            string          `json:"id"`
	DocumentID    string          `json:"document_id"`
	Filename      string          `json:"filename"`
	SourceOrdinal int             `json:"source_ordinal"`
	SourceLocator json.RawMessage `json:"source_locator"`
	EntityType    string          `json:"entity_type"`
	EntityLabel   string          `json:"entity_label"`
	EntityKey     string          `json:"entity_key"`
	FieldKey      string          `json:"field_key,omitempty"`
	Value         string          `json:"value"`
	Attributes    json.RawMessage `json:"attributes"`
	Explanation   string          `json:"explanation"`
	Status        string          `json:"status"`
	CapturedAt    time.Time       `json:"captured_at"`
}

func candidateID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "FC-" + hex.EncodeToString(b[:]), nil
}
func validCandidateID(id string) bool {
	if len(id) != 35 || !strings.HasPrefix(id, "FC-") {
		return false
	}
	for _, c := range id[3:] {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}
func candidateJSONBody(w http.ResponseWriter, r *http.Request, max int, target any) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "Expected application/json", http.StatusUnsupportedMediaType)
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, int64(max))
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		http.Error(w, "Invalid request payload", 400)
		return false
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		http.Error(w, "Exactly one JSON object is required", 400)
		return false
	}
	return true
}
func (s *EdgeServer) handleFactoryCandidateScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", 405)
		return
	}
	var req struct {
		DocumentID string `json:"document_id"`
	}
	if !candidateJSONBody(w, r, 2048, &req) {
		return
	}
	if !validDocSourceID(req.DocumentID) {
		http.Error(w, "Select a valid uploaded document", 400)
		return
	}
	tenant, err := intakeTenant()
	if err != nil || s.db == nil {
		http.Error(w, "Development tenant or PostgreSQL unavailable", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	tx, err := s.docTransaction(ctx, tenant)
	if err != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	defer tx.Rollback()
	var name, status string
	err = tx.QueryRowContext(ctx, `SELECT d.filename,j.status FROM document_sources_v65 d
 JOIN document_jobs_v65 j ON j.tenant_id=d.tenant_id AND j.document_id=d.id
 WHERE d.tenant_id=$1 AND d.id=$2`, tenant, req.DocumentID).Scan(&name, &status)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Document not found in this tenant", 404)
		return
	}
	if err != nil {
		http.Error(w, "Unable to load source", 503)
		return
	}
	if status != "EXTRACTED" && status != "PARTIAL" {
		docJSON(w, 409, map[string]any{"error": "Wait until source extraction completes, or upload a supported format.", "source_status": status})
		return
	}
	rows, err := tx.QueryContext(ctx, `SELECT ordinal,kind,locator,content FROM document_segments_v65
 WHERE tenant_id=$1 AND document_id=$2 ORDER BY ordinal LIMIT $3`, tenant, req.DocumentID, maxCandidateSegments)
	if err != nil {
		http.Error(w, "Unable to read document evidence", 503)
		return
	}
	segments := []factoryEvidenceSegment{}
	for rows.Next() {
		var x factoryEvidenceSegment
		var loc []byte
		if err = rows.Scan(&x.Ordinal, &x.Kind, &loc, &x.Content); err != nil {
			break
		}
		x.Locator = json.RawMessage(loc)
		segments = append(segments, x)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		http.Error(w, "Document evidence unavailable", 503)
		return
	}
	proposed := factoryProposalsFromSegments(req.DocumentID, name, segments)
	inserted := 0
	for _, p := range proposed {
		id, e := candidateID()
		if e != nil {
			http.Error(w, "Unable to create candidate identity", 503)
			return
		}
		result, e := tx.ExecContext(ctx, `INSERT INTO factory_document_candidates_v65b2
   (tenant_id,candidate_id,document_id,source_ordinal,source_locator,entity_type,entity_key,field_key,proposed_value,attributes,explanation,fingerprint)
   VALUES($1,$2,$3,$4,$5::jsonb,$6,$7,$8,$9,$10::jsonb,$11,$12)
   ON CONFLICT(tenant_id,fingerprint) DO NOTHING`, tenant, id, p.DocumentID, p.SourceOrdinal, string(p.Locator), p.EntityType, p.EntityKey, p.FieldKey, p.Value, string(p.Attributes), p.Explanation, p.Fingerprint)
		if e != nil {
			http.Error(w, "Unable to stage candidates; apply migration 0011", 503)
			return
		}
		n, _ := result.RowsAffected()
		inserted += int(n)
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "Unable to commit proposed candidates", 503)
		return
	}
	docJSON(w, 200, map[string]any{"document_id": req.DocumentID, "source_status": status, "matched": len(proposed), "new_candidates": inserted, "review_state": "UNREVIEWED", "explanation": "Suggestions based on explicit source labels, not verified machine, skills, material balances or costs. Unrecognized data needs manual entry."})
}
func validDocSourceID(id string) bool {
	if len(id) != 36 || !strings.HasPrefix(id, "DOC-") {
		return false
	}
	for _, c := range id[4:] {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}
func (s *EdgeServer) handleFactoryCandidates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", 405)
		return
	}
	tenant, err := intakeTenant()
	if err != nil || s.db == nil {
		http.Error(w, "Development tenant or PostgreSQL unavailable", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, err := s.docTransaction(ctx, tenant)
	if err != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT c.candidate_id,c.document_id,d.filename,c.source_ordinal,c.source_locator,c.entity_type,c.entity_key,
 c.field_key,c.proposed_value,c.attributes,c.explanation,COALESCE(v.decision,'PENDING'),c.captured_at
 FROM factory_document_candidates_v65b2 c
 JOIN document_sources_v65 d ON d.tenant_id=c.tenant_id AND d.id=c.document_id
 LEFT JOIN factory_candidate_reviews_v65b2 v ON v.tenant_id=c.tenant_id AND v.candidate_id=c.candidate_id
 WHERE c.tenant_id=$1 ORDER BY c.captured_at DESC,c.candidate_id DESC LIMIT 300`, tenant)
	if err != nil {
		http.Error(w, "Candidate query unavailable; apply migration 0011", 503)
		return
	}
	items := []candidateRow{}
	for rows.Next() {
		var x candidateRow
		var locator, attributes []byte
		if err = rows.Scan(&x.ID, &x.DocumentID, &x.Filename, &x.SourceOrdinal, &locator, &x.EntityType, &x.EntityKey, &x.FieldKey, &x.Value, &attributes, &x.Explanation, &x.Status, &x.CapturedAt); err != nil {
			break
		}
		x.SourceLocator = json.RawMessage(locator)
		x.Attributes = json.RawMessage(attributes)
		x.EntityLabel = candidateKindLabel(x.EntityType)
		items = append(items, x)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		http.Error(w, "Unable to read candidates", 503)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	docJSON(w, 200, map[string]any{"items": items, "tenant": tenant, "meaning": "Review-only evidence. No authoritative factory master, inventory or cost updates."})
}
func (s *EdgeServer) handleFactoryCandidateReview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", 405)
		return
	}
	var req struct {
		CandidateID string `json:"candidate_id"`
		Decision    string `json:"decision"`
		Note        string `json:"note"`
	}
	if !candidateJSONBody(w, r, 4096, &req) {
		return
	}
	if !validCandidateID(req.CandidateID) || (req.Decision != "ACCEPT_PROVISIONAL" && req.Decision != "REJECT") || len(req.Note) > 900 || strings.ContainsRune(req.Note, 0) {
		http.Error(w, "Invalid review decision", 400)
		return
	}
	tenant, err := intakeTenant()
	if err != nil || s.db == nil {
		http.Error(w, "Development tenant or PostgreSQL unavailable", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	tx, err := s.docTransaction(ctx, tenant)
	if err != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	defer tx.Rollback()
	var p factoryCandidateProposal
	var rawAttributes []byte
	err = tx.QueryRowContext(ctx, `SELECT document_id,source_ordinal,entity_type,entity_key,field_key,proposed_value,attributes
 FROM factory_document_candidates_v65b2 WHERE tenant_id=$1 AND candidate_id=$2 FOR UPDATE`, tenant, req.CandidateID).
		Scan(&p.DocumentID, &p.SourceOrdinal, &p.EntityType, &p.EntityKey, &p.FieldKey, &p.Value, &rawAttributes)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Candidate not found", 404)
		return
	}
	if err != nil {
		http.Error(w, "Candidate review unavailable", 503)
		return
	}
	var existing string
	err = tx.QueryRowContext(ctx, `SELECT decision FROM factory_candidate_reviews_v65b2 WHERE tenant_id=$1 AND candidate_id=$2`, tenant, req.CandidateID).Scan(&existing)
	if err == nil {
		docJSON(w, 409, map[string]any{"error": "Already reviewed. Use Factory Setup to revise accepted facts; uploads remain immutable.", "decision": existing})
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Unable to check previous reviews", 503)
		return
	}
	p.Attributes = json.RawMessage(rawAttributes)
	if req.Decision == "ACCEPT_PROVISIONAL" {
		if p.EntityType == "FIELD" {
			spec, ok := factorySpec(p.FieldKey)
			if !ok {
				http.Error(w, "Unsupported factory field", 400)
				return
			}
			normalizedValue, normalized, e := canonicalFactoryValue(spec, p.Value)
			if e != nil {
				http.Error(w, "Source value is no longer valid", 400)
				return
			}
			// The confirmed value must not be silently displaced by a document heuristic.
			if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1),hashtext($2))`, tenant, p.FieldKey); e != nil {
				http.Error(w, "Unable to lock fact", 503)
				return
			}
			var revision int
			var currentState string
			e = tx.QueryRowContext(ctx, `SELECT revision,review_state FROM factory_fact_events_v65b
    WHERE tenant_id=$1 AND field_key=$2 ORDER BY revision DESC LIMIT 1`, tenant, p.FieldKey).Scan(&revision, &currentState)
			if e != nil && !errors.Is(e, sql.ErrNoRows) {
				http.Error(w, "Unable to read current fact", 503)
				return
			}
			if currentState == "CONFIRMED" {
				docJSON(w, 409, map[string]any{"error": "A confirmed value already exists. Review and update it explicitly in Factory Setup rather than replacing it from a document."})
				return
			}
			if _, e = tx.ExecContext(ctx, `INSERT INTO factory_fact_events_v65b
    (tenant_id,field_key,revision,value_kind,value_text,normalized_value,review_state,source_document_id,review_note,recorded_by)
    VALUES($1,$2,$3,$4,$5,$6::jsonb,'PROVISIONAL',$7,$8,'dev')`, tenant, p.FieldKey, revision+1, spec.Kind, normalizedValue, string(normalized), p.DocumentID, "Imported proposal "+req.CandidateID+". "+req.Note); e != nil {
				http.Error(w, "Unable to stage factory fact", 503)
				return
			}
		} else {
			if _, e := tx.ExecContext(ctx, `INSERT INTO factory_proposed_register_v65b2(tenant_id,candidate_id,entity_type,entity_key,attributes)
    VALUES($1,$2,$3,$4,$5::jsonb)`, tenant, req.CandidateID, p.EntityType, p.EntityKey, string(p.Attributes)); e != nil {
				http.Error(w, "Unable to stage proposed register", 503)
				return
			}
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO factory_candidate_reviews_v65b2(tenant_id,candidate_id,decision,review_note,reviewed_by)
  VALUES($1,$2,$3,$4,'dev')`, tenant, req.CandidateID, req.Decision, req.Note); err != nil {
		http.Error(w, "Unable to record review", 503)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "Unable to complete review", 503)
		return
	}
	docJSON(w, 200, map[string]any{"candidate_id": req.CandidateID, "decision": req.Decision, "authority": "PROVISIONAL_EVIDENCE_ONLY", "operational_master_updated": false})
}
func (s *EdgeServer) handleFactoryRegisterProposals(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", 405)
		return
	}
	tenant, err := intakeTenant()
	if err != nil || s.db == nil {
		http.Error(w, "Development tenant or PostgreSQL unavailable", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, err := s.docTransaction(ctx, tenant)
	if err != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT r.candidate_id,r.entity_type,r.entity_key,r.attributes,c.document_id,c.source_ordinal,c.source_locator,d.filename
  FROM factory_proposed_register_v65b2 r
  JOIN factory_document_candidates_v65b2 c ON c.tenant_id=r.tenant_id AND c.candidate_id=r.candidate_id
  JOIN document_sources_v65 d ON d.tenant_id=c.tenant_id AND d.id=c.document_id
  WHERE r.tenant_id=$1 ORDER BY r.recorded_at DESC LIMIT 200`, tenant)
	if err != nil {
		http.Error(w, "Register proposals unavailable", 503)
		return
	}
	items := []map[string]any{}
	for rows.Next() {
		var id, typ, key, docID, name string
		var attrs, loc []byte
		var ordinal int
		if err = rows.Scan(&id, &typ, &key, &attrs, &docID, &ordinal, &loc, &name); err != nil {
			break
		}
		items = append(items, map[string]any{"candidate_id": id, "type": typ, "entity_key": key, "attributes": json.RawMessage(attrs), "document_id": docID, "source_ordinal": ordinal, "locator": json.RawMessage(loc), "filename": name, "state": "PROVISIONAL"})
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		http.Error(w, "Unable to read proposed register", 503)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	docJSON(w, 200, map[string]any{"items": items, "scope": "proposed only; not inventory, master records or issued materials"})
}
