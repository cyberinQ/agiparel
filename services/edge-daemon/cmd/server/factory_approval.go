// Author: Sunil Jain, 2026-10-27
// Source-linked, field-confirmed document reviews and atomic DEV-only manager approval.
// Approved snapshots are factory information references, NEVER operating masters.
package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"
)

const maxApprovalCandidates = 500

type approvalCandidate struct {
	ID            string            `json:"id"`
	DocumentID    string            `json:"document_id"`
	Filename      string            `json:"filename"`
	EntityType    string            `json:"entity_type"`
	EntityKey     string            `json:"entity_key"`
	SourceOrdinal int               `json:"source_ordinal"`
	SourceLocator json.RawMessage   `json:"source_locator"`
	Attributes    map[string]string `json:"attributes"`
	Action        string            `json:"action"`
	Revision      int               `json:"revision"`
}
type approvalReviewInput struct {
	CandidateID      string            `json:"candidate_id"`
	Action           string            `json:"action"`
	Attributes       map[string]string `json:"attributes"`
	CheckedFields    []string          `json:"checked_fields"`
	ExpectedRevision int               `json:"expected_revision"`
}
type approvalCommitInput struct {
	DocumentID    string `json:"document_id"`
	ExpectedCount int    `json:"expected_count"`
	Confirmation  string `json:"confirmation"`
}

// A shared Basic credential cannot establish production manager identity.
// Gate all manager submissions explicitly to a configured development reviewer.
func developmentReviewActor(r *http.Request) (string, bool) {
	actor, password, ok := r.BasicAuth()
	enabled := strings.TrimSpace(os.Getenv("AGIPAREL_REVIEW_MANAGER_USER"))
	return actor, ok && os.Getenv("ENV") == "development" && enabled != "" && len(os.Getenv("AGIPAREL_OPERATOR_PASSWORD")) >= 4 && sameSecret(password, os.Getenv("AGIPAREL_OPERATOR_PASSWORD")) &&
		len(actor) <= 64 && sameSecret(actor, enabled) && sameSecret(actor, os.Getenv("AGIPAREL_OPERATOR_USER"))
}

// Only recognized factory fields may enter manager-approved references.
// In particular, extracted material quantities cannot become stock ledgers.
func allowedApprovalKey(typ, key string) bool {
	allowed := map[string]map[string]bool{
		"MACHINE":  {"machine_id": true, "machine_type": true, "status_note": true, "location_note": true, "capacity_note": true},
		"SKILL":    {"worker_id": true, "skill": true, "level_note": true, "department_note": true},
		"MATERIAL": {"material_code": true, "quantity": true, "unit": true, "description": true, "ply": true, "size": true, "shade": true, "lot": true},
		"RATE":     {"cost_type": true, "rate": true, "currency": true, "unit_note": true, "rate_basis": true},
		"FIELD":    {"value": true},
	}
	return allowed[typ][key]
}
func validApprovalAttrs(attrs map[string]string) error {
	if len(attrs) == 0 || len(attrs) > 24 {
		return fmt.Errorf("enter 1–24 fields")
	}
	for k, v := range attrs {
		if k == "" || len(k) > 100 || strings.ContainsAny(k, "\x00\n\r") || len(strings.TrimSpace(v)) == 0 || len(v) > 300 || strings.ContainsAny(v, "\x00\r") {
			return fmt.Errorf("invalid value for %s", k)
		}
	}
	return nil
}
func checkedEveryField(attrs map[string]string, checked []string) bool {
	if len(attrs) != len(checked) {
		return false
	}
	seen := make(map[string]bool, len(checked))
	for _, key := range checked {
		if _, ok := attrs[key]; !ok || seen[key] {
			return false
		}
		seen[key] = true
	}
	return true
}

// Normalize edited facts before storing or committing them. Identity keys are
// derived from the edited content, not trusted from the client or LLM.
func validatedApproval(typ, field string, attrs map[string]string) (string, map[string]string, error) {
	if err := validApprovalAttrs(attrs); err != nil {
		return "", nil, err
	}
	for key := range attrs {
		if !allowedApprovalKey(typ, key) {
			return "", nil, fmt.Errorf("unsupported %s field: %s", typ, key)
		}
	}
	clean := make(map[string]string, len(attrs))
	for k, v := range attrs {
		clean[k] = strings.TrimSpace(v)
	}
	switch typ {
	case "FIELD":
		if len(clean) != 1 || clean["value"] == "" {
			return "", nil, fmt.Errorf("expected one factory value")
		}
		spec, ok := factorySpec(field)
		if !ok {
			return "", nil, fmt.Errorf("unsupported factory field")
		}
		value, _, err := canonicalFactoryValue(spec, clean["value"])
		if err != nil {
			return "", nil, err
		}
		clean["value"] = value
		return field, clean, nil
	case "MACHINE":
		key := clean["machine_id"]
		if _, err := parseReferenceFacts(typ, key, mustJSON(clean), ""); err != nil {
			return "", nil, err
		}
		return key, clean, nil
	case "SKILL":
		key := clean["worker_id"] + ":" + clean["skill"]
		if _, err := parseReferenceFacts(typ, key, mustJSON(clean), ""); err != nil {
			return "", nil, err
		}
		return key, clean, nil
	case "RATE":
		key := clean["cost_type"] + ":" + clean["currency"]
		basis := strings.ToUpper(clean["rate_basis"])
		clean["rate_basis"] = basis
		if _, err := parseReferenceFacts(typ, key, mustJSON(clean), basis); err != nil {
			return "", nil, err
		}
		return key, clean, nil
	case "MATERIAL":
		code, unit, qty := clean["material_code"], clean["unit"], clean["quantity"]
		if !referenceIDPattern.MatchString(code) || len(unit) > 24 || unit == "" || !referenceDecimalPattern.MatchString(qty) {
			return "", nil, fmt.Errorf("material needs a code, quantity and unit")
		}
		return code + ":" + unit, clean, nil
	default:
		return "", nil, fmt.Errorf("unsupported record type")
	}
}
func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }
func newSubmissionID() (string, error) {
	var b [16]byte
	_, err := rand.Read(b[:])
	return "FS-" + hex.EncodeToString(b[:]), err
}

func approvalCandidates(ctx context.Context, tx *sql.Tx, tenant, doc string) ([]approvalCandidate, error) {
	rows, err := tx.QueryContext(ctx, `SELECT c.candidate_id,c.document_id,d.filename,c.entity_type,c.entity_key,c.field_key,c.proposed_value,c.attributes,
 c.source_ordinal,c.source_locator,
 COALESCE(v.action,''),COALESCE(v.revision,0),v.attributes
 FROM factory_document_candidates_v65b2 c
 JOIN document_sources_v65 d ON d.tenant_id=c.tenant_id AND d.id=c.document_id
 LEFT JOIN LATERAL (
  SELECT action,revision,attributes FROM factory_candidate_review_versions_v65b5
  WHERE tenant_id=c.tenant_id AND candidate_id=c.candidate_id ORDER BY revision DESC LIMIT 1
 ) v ON TRUE
 WHERE c.tenant_id=$1 AND c.document_id=$2 ORDER BY c.source_ordinal,c.candidate_id LIMIT $3`, tenant, doc, maxApprovalCandidates+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []approvalCandidate{}
	for rows.Next() {
		var item approvalCandidate
		var field, value string
		var original, loc []byte
		var draft []byte
		if err = rows.Scan(&item.ID, &item.DocumentID, &item.Filename, &item.EntityType, &item.EntityKey, &field, &value, &original, &item.SourceOrdinal, &loc, &item.Action, &item.Revision, &draft); err != nil {
			return nil, err
		}
		item.SourceLocator = json.RawMessage(loc)
		if len(draft) != 0 && item.Action != "SKIP" {
			if err = json.Unmarshal(draft, &item.Attributes); err != nil {
				return nil, err
			}
		} else if item.EntityType == "FIELD" {
			item.Attributes = map[string]string{"value": value}
		} else {
			if err = json.Unmarshal(original, &item.Attributes); err != nil {
				return nil, err
			}
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
func (s *EdgeServer) handleFactoryApprovalList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", 405)
		return
	}
	doc := r.URL.Query().Get("document_id")
	if !validDocSourceID(doc) {
		http.Error(w, "Invalid document", 400)
		return
	}
	tenant, err := intakeTenant()
	if err != nil || s.db == nil {
		http.Error(w, "Database unavailable", 503)
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
	items, err := approvalCandidates(ctx, tx, tenant, doc)
	if err != nil {
		http.Error(w, "Review unavailable; apply migration 0026", 503)
		return
	}
	if len(items) > maxApprovalCandidates {
		docJSON(w, 409, map[string]any{"error": "Too many records for one approval. Split the source."})
		return
	}
	var submittedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT approved_at FROM factory_document_submissions_v65b5 WHERE tenant_id=$1 AND document_id=$2`, tenant, doc).Scan(&submittedAt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Review unavailable", 503)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "Review unavailable", 503)
		return
	}
	docJSON(w, 200, map[string]any{"items": items, "submitted": submittedAt.Valid, "review_mode": "SOURCE_LINKED", "operational_master_updated": false})
}
func (s *EdgeServer) handleFactoryApprovalReview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", 405)
		return
	}
	var in approvalReviewInput
	if !candidateJSONBody(w, r, 9000, &in) {
		return
	}
	if !validCandidateID(in.CandidateID) || in.ExpectedRevision < 0 || (in.Action != "KEEP" && in.Action != "SKIP") {
		http.Error(w, "Invalid review", 400)
		return
	}
	if in.Action == "KEEP" && (!checkedEveryField(in.Attributes, in.CheckedFields) || validApprovalAttrs(in.Attributes) != nil) {
		http.Error(w, "Check every field before saving", 400)
		return
	}
	if in.Action == "SKIP" {
		in.Attributes = map[string]string{}
		in.CheckedFields = []string{}
	}
	tenant, err := intakeTenant()
	if err != nil || s.db == nil {
		http.Error(w, "Database unavailable", 503)
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
	var doc, typ, field string
	// Find the source, then lock it BEFORE the candidate. Submission takes the
	// same source lock, so review and final commit cannot overtake each other.
	err = tx.QueryRowContext(ctx, `SELECT document_id FROM factory_document_candidates_v65b2 WHERE tenant_id=$1 AND candidate_id=$2`, tenant, in.CandidateID).Scan(&doc)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Record not found", 404)
		return
	}
	if err != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	var locked string
	err = tx.QueryRowContext(ctx, `SELECT id FROM document_sources_v65 WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, tenant, doc).Scan(&locked)
	if err != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	err = tx.QueryRowContext(ctx, `SELECT entity_type,field_key FROM factory_document_candidates_v65b2 WHERE tenant_id=$1 AND candidate_id=$2 FOR UPDATE`, tenant, in.CandidateID).Scan(&typ, &field)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Record not found", 404)
		return
	}
	if err != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	var submitted bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM factory_document_submissions_v65b5 WHERE tenant_id=$1 AND document_id=$2)`, tenant, doc).Scan(&submitted)
	if err != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	if submitted {
		http.Error(w, "Already submitted; immutable history", 409)
		return
	}
	if in.Action == "KEEP" {
		_, in.Attributes, err = validatedApproval(typ, field, in.Attributes)
		if err != nil {
			docJSON(w, 400, map[string]any{"error": err.Error()})
			return
		}
	}
	var revision int
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(revision),0) FROM factory_candidate_review_versions_v65b5 WHERE tenant_id=$1 AND candidate_id=$2`, tenant, in.CandidateID).Scan(&revision)
	if err != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	if revision != in.ExpectedRevision {
		docJSON(w, 409, map[string]any{"error": "This record changed. Refresh before editing.", "revision": revision})
		return
	}
	actor, _, ok := r.BasicAuth()
	if !ok || actor == "" || len(actor) > 64 {
		http.Error(w, "Authenticated reviewer required", 403)
		return
	}
	// Revalidate checked fields after canonicalization; canonicalization may add a required rate basis.
	if in.Action == "KEEP" && !checkedEveryField(in.Attributes, in.CheckedFields) {
		http.Error(w, "Confirm all corrected fields, including the rate basis", 400)
		return
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO factory_candidate_review_versions_v65b5(tenant_id,candidate_id,revision,action,attributes,checked_fields,reviewer)
 VALUES($1,$2,$3,$4,$5::jsonb,$6::jsonb,$7)`, tenant, in.CandidateID, revision+1, in.Action, string(mustJSON(in.Attributes)), string(mustJSON(in.CheckedFields)), actor)
	if err != nil {
		http.Error(w, "Could not save review", 503)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "Could not save review", 503)
		return
	}
	docJSON(w, 200, map[string]any{"candidate_id": in.CandidateID, "revision": revision + 1, "saved": true, "submitted": false})
}
func (s *EdgeServer) handleFactoryApprovalSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", 405)
		return
	}
	actor, allowed := developmentReviewActor(r)
	if !allowed {
		http.Error(w, "Manager identity unavailable. Development-only approval must be explicitly enabled.", 403)
		return
	}
	var in approvalCommitInput
	if !candidateJSONBody(w, r, 2048, &in) {
		return
	}
	if !validDocSourceID(in.DocumentID) || in.ExpectedCount < 1 || in.ExpectedCount > maxApprovalCandidates || in.Confirmation != "SUBMIT" {
		http.Error(w, "Invalid final submission", 400)
		return
	}
	tenant, err := intakeTenant()
	if err != nil || s.db == nil {
		http.Error(w, "Database unavailable", 503)
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
	// Serialize review and submit across all candidates of this source, including new scans.
	var found string
	err = tx.QueryRowContext(ctx, `SELECT id FROM document_sources_v65 WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, tenant, in.DocumentID).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Source document not found", 404)
		return
	}
	if err != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	var already bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM factory_document_submissions_v65b5 WHERE tenant_id=$1 AND document_id=$2)`, tenant, in.DocumentID).Scan(&already)
	if err != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	if already {
		http.Error(w, "Document already submitted", 409)
		return
	}
	items, err := approvalCandidates(ctx, tx, tenant, in.DocumentID)
	if err != nil {
		http.Error(w, "Review unavailable; apply migration 0026", 503)
		return
	}
	if len(items) != in.ExpectedCount || len(items) > maxApprovalCandidates {
		docJSON(w, 409, map[string]any{"error": "Records changed since review. Refresh and check again."})
		return
	}
	type approved struct {
		c     approvalCandidate
		key   string
		attrs map[string]string
	}
	approvedItems := []approved{}
	skipped := 0
	seen := map[string]bool{}
	for _, item := range items {
		if item.Revision < 1 || (item.Action != "KEEP" && item.Action != "SKIP") {
			docJSON(w, 409, map[string]any{"error": "Review or skip every record before submitting."})
			return
		}
		if item.Action == "SKIP" {
			skipped++
			continue
		}
		field := ""
		if item.EntityType == "FIELD" {
			field = item.EntityKey
		}
		key, attrs, e := validatedApproval(item.EntityType, field, item.Attributes)
		if e != nil {
			docJSON(w, 409, map[string]any{"error": "Invalid saved record: " + e.Error()})
			return
		}
		id := item.EntityType + "\x00" + key
		if seen[id] {
			docJSON(w, 409, map[string]any{"error": "Two records describe the same item. Set one aside before submitting."})
			return
		}
		seen[id] = true
		approvedItems = append(approvedItems, approved{item, key, attrs})
	}
	if len(approvedItems) == 0 {
		http.Error(w, "No approved records to submit", 409)
		return
	}
	// Hold source row lock and validate all before any inserts. One SQL transaction
	// either publishes the entire reference snapshot set or none of it.
	subID, err := newSubmissionID()
	if err != nil {
		http.Error(w, "Unable to issue submission ID", 503)
		return
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO factory_document_submissions_v65b5(tenant_id,document_id,submission_id,approved_count,skipped_count,reviewer) VALUES($1,$2,$3,$4,$5,$6)`, tenant, in.DocumentID, subID, len(approvedItems), skipped, actor)
	if err != nil {
		http.Error(w, "Unable to start submission", 503)
		return
	}
	// Stable ordering avoids deadlocks when two documents share reference keys.
	sort.Slice(approvedItems, func(i, j int) bool {
		return approvedItems[i].c.EntityType+"\x00"+approvedItems[i].key < approvedItems[j].c.EntityType+"\x00"+approvedItems[j].key
	})
	for _, a := range approvedItems {
		_, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1),hashtext($2))`, tenant, a.c.EntityType+"\x00"+a.key)
		if err != nil {
			http.Error(w, "Unable to lock reference", 503)
			return
		}
		var rev int
		var previous []byte
		err = tx.QueryRowContext(ctx, `SELECT revision,attributes FROM factory_approved_reference_versions_v65b5
  WHERE tenant_id=$1 AND entity_type=$2 AND entity_key=$3 ORDER BY revision DESC LIMIT 1`, tenant, a.c.EntityType, a.key).Scan(&rev, &previous)
		if errors.Is(err, sql.ErrNoRows) {
			rev = 0
		} else if err != nil {
			http.Error(w, "Unable to version reference", 503)
			return
		}
		if rev > 0 {
			var old map[string]string
			if err = json.Unmarshal(previous, &old); err != nil {
				http.Error(w, "Unable to compare earlier reference", 503)
				return
			}
			if !reflect.DeepEqual(old, a.attrs) {
				docJSON(w, 409, map[string]any{"error": "An earlier approved reference differs for " + a.key + ". Resolve the conflict before replacing it."})
				return
			}
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO factory_approved_reference_versions_v65b5
   (tenant_id,entity_type,entity_key,revision,candidate_id,document_id,review_revision,attributes,approved_by)
   VALUES($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9)`, tenant, a.c.EntityType, a.key, rev+1, a.c.ID, in.DocumentID, a.c.Revision, string(mustJSON(a.attrs)), actor)
		if err != nil {
			http.Error(w, "Unable to publish audited references", 503)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "Unable to commit references", 503)
		return
	}
	docJSON(w, 200, map[string]any{"submission_id": subID, "approved_records": len(approvedItems), "skipped_records": skipped, "approved_by": actor, "authority": "MANAGER_REVIEWED_FACTORY_REFERENCES_DEV_ONLY", "operational_master_updated": false})
}

// Pending count excludes already reviewed items and already submitted documents.
// Unlike the legacy 300-row candidate listing, this count is complete.
func (s *EdgeServer) handleFactoryApprovalPending(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", 405)
		return
	}
	tenant, err := intakeTenant()
	if err != nil || s.db == nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	tx, err := s.docTransaction(ctx, tenant)
	if err != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	defer tx.Rollback()
	var count int
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM factory_document_candidates_v65b2 c
 LEFT JOIN factory_document_submissions_v65b5 s ON s.tenant_id=c.tenant_id AND s.document_id=c.document_id
 LEFT JOIN LATERAL (SELECT action FROM factory_candidate_review_versions_v65b5 v
 WHERE v.tenant_id=c.tenant_id AND v.candidate_id=c.candidate_id
 ORDER BY revision DESC LIMIT 1) r ON TRUE
 WHERE c.tenant_id=$1 AND s.document_id IS NULL AND r.action IS NULL`, tenant).Scan(&count)
	if err != nil {
		http.Error(w, "Pending count unavailable; apply migration 0026", 503)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	docJSON(w, 200, map[string]any{"pending_records": count})
}
