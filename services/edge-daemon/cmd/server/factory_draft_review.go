// Gate 06.5-B3: review conflicts among independently sourced factory observations.
// Even a VERIFIED_DRAFT is not an authoritative factory machine, employee,
// stock balance, purchasing permission, or payroll/costing master record.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

const draftReviewLimit = 500

type factoryDraftSource struct {
	CandidateID   string          `json:"candidate_id"`
	DocumentID    string          `json:"document_id"`
	Filename      string          `json:"filename"`
	SourceOrdinal int             `json:"source_ordinal"`
	SourceLocator json.RawMessage `json:"source_locator"`
	Attributes    json.RawMessage `json:"attributes"`
	RecordedAt    time.Time       `json:"recorded_at"`
}
type factoryDraftGroup struct {
	EntityType          string               `json:"entity_type"`
	EntityKey           string               `json:"entity_key"`
	Sources             []factoryDraftSource `json:"sources"`
	DistinctVersions    int                  `json:"distinct_versions"`
	EvidenceState       string               `json:"evidence_state"`
	Decision            string               `json:"decision"`
	SelectedCandidateID string               `json:"selected_candidate_id,omitempty"`
	DecisionNote        string               `json:"decision_note,omitempty"`
	Revision            int                  `json:"revision"`
	NeedsAttention      bool                 `json:"needs_attention"`
}

type factoryDraftDecision struct {
	EntityType  string
	EntityKey   string
	CandidateID string
	Decision    string
	Note        string
	Revision    int
}

func canonicalDraftAttributes(raw json.RawMessage) (string, error) {
	var attrs map[string]any
	if err := json.Unmarshal(raw, &attrs); err != nil || attrs == nil {
		return "", fmt.Errorf("invalid factory attributes")
	}
	// json.Marshal canonicalizes Go map key order. Comparison is by typed values,
	// not by document filename, whitespace, or serialized JSON field order.
	b, err := json.Marshal(attrs)
	return string(b), err
}
func draftGroupID(typ, key string) string { return typ + "\x00" + key }
func summarizeFactoryDrafts(rows []structDraftRow, decisions map[string]factoryDraftDecision) []factoryDraftGroup {
	index := map[string]*factoryDraftGroup{}
	variants := map[string]map[string]bool{}
	for _, x := range rows {
		k := draftGroupID(x.Type, x.Key)
		g := index[k]
		if g == nil {
			g = &factoryDraftGroup{EntityType: x.Type, EntityKey: x.Key, Decision: "UNREVIEWED", Sources: []factoryDraftSource{}}
			index[k] = g
			variants[k] = map[string]bool{}
		}
		g.Sources = append(g.Sources, x.Source)
		normalized, err := canonicalDraftAttributes(x.Source.Attributes)
		if err != nil {
			normalized = "!invalid:" + x.Source.CandidateID
		}
		variants[k][normalized] = true
	}
	groups := make([]factoryDraftGroup, 0, len(index))
	for k, g := range index {
		g.DistinctVersions = len(variants[k])
		switch {
		case g.DistinctVersions > 1:
			g.EvidenceState = "CONFLICT"
		case len(g.Sources) > 1:
			g.EvidenceState = "CORROBORATED"
		default:
			g.EvidenceState = "SINGLE_SOURCE"
		}
		if d, ok := decisions[k]; ok {
			g.Decision = d.Decision
			g.SelectedCandidateID = d.CandidateID
			g.DecisionNote = d.Note
			g.Revision = d.Revision
			// A more recent source cannot be silently taken as confirming an older
			// selection; the UI always shows how many competing versions exist.
		}
		g.NeedsAttention = g.EvidenceState == "CONFLICT" || g.Decision != "VERIFY_DRAFT"
		sort.Slice(g.Sources, func(i, j int) bool { return g.Sources[i].CandidateID < g.Sources[j].CandidateID })
		groups = append(groups, *g)
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].NeedsAttention != groups[j].NeedsAttention {
			return groups[i].NeedsAttention
		}
		if groups[i].EntityType != groups[j].EntityType {
			return groups[i].EntityType < groups[j].EntityType
		}
		return groups[i].EntityKey < groups[j].EntityKey
	})
	return groups
}

type structDraftRow struct {
	Type, Key string
	Source    factoryDraftSource
}

func readFactoryDraftRows(ctx context.Context, tx *sql.Tx, tenant string) ([]structDraftRow, error) {
	rows, err := tx.QueryContext(ctx, `SELECT r.entity_type,r.entity_key,r.candidate_id,c.document_id,d.filename,c.source_ordinal,c.source_locator,r.attributes,r.recorded_at
 FROM factory_proposed_register_v65b2 r
 JOIN factory_document_candidates_v65b2 c ON c.tenant_id=r.tenant_id AND c.candidate_id=r.candidate_id
 JOIN document_sources_v65 d ON d.tenant_id=c.tenant_id AND d.id=c.document_id
 WHERE r.tenant_id=$1 ORDER BY r.recorded_at DESC,r.candidate_id DESC LIMIT $2`, tenant, draftReviewLimit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []structDraftRow{}
	for rows.Next() {
		var item structDraftRow
		var loc, attrs []byte
		e := rows.Scan(&item.Type, &item.Key, &item.Source.CandidateID, &item.Source.DocumentID, &item.Source.Filename, &item.Source.SourceOrdinal, &loc, &attrs, &item.Source.RecordedAt)
		if e != nil {
			return nil, e
		}
		item.Source.SourceLocator = json.RawMessage(loc)
		item.Source.Attributes = json.RawMessage(attrs)
		items = append(items, item)
	}
	return items, rows.Err()
}
func readFactoryDraftDecisions(ctx context.Context, tx *sql.Tx, tenant string) (map[string]factoryDraftDecision, error) {
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT ON(entity_type,entity_key) entity_type,entity_key,candidate_id,decision,review_note,revision
 FROM factory_draft_decisions_v65b3 WHERE tenant_id=$1
 ORDER BY entity_type,entity_key,revision DESC`, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]factoryDraftDecision{}
	for rows.Next() {
		var d factoryDraftDecision
		if err = rows.Scan(&d.EntityType, &d.EntityKey, &d.CandidateID, &d.Decision, &d.Note, &d.Revision); err != nil {
			return nil, err
		}
		out[draftGroupID(d.EntityType, d.EntityKey)] = d
	}
	return out, rows.Err()
}
func validDraftReview(req factoryDraftReviewRequest) error {
	if req.EntityType != "MACHINE" && req.EntityType != "SKILL" && req.EntityType != "MATERIAL" && req.EntityType != "RATE" {
		return fmt.Errorf("invalid type")
	}
	if req.EntityKey == "" || len(req.EntityKey) > 256 || strings.TrimSpace(req.EntityKey) != req.EntityKey || strings.ContainsRune(req.EntityKey, 0) {
		return fmt.Errorf("invalid entity key")
	}
	if !validCandidateID(req.CandidateID) {
		return fmt.Errorf("invalid candidate")
	}
	if req.Decision != "VERIFY_DRAFT" && req.Decision != "NEEDS_CLARIFICATION" {
		return fmt.Errorf("invalid decision")
	}
	if req.ExpectedRevision < 0 {
		return fmt.Errorf("invalid expected revision")
	}
	if len(req.Note) > 1000 || strings.ContainsRune(req.Note, 0) {
		return fmt.Errorf("invalid note")
	}
	return nil
}

type factoryDraftReviewRequest struct {
	EntityType       string `json:"entity_type"`
	EntityKey        string `json:"entity_key"`
	CandidateID      string `json:"candidate_id"`
	Decision         string `json:"decision"`
	Note             string `json:"note"`
	ExpectedRevision int    `json:"expected_revision"`
}

func (s *EdgeServer) handleFactoryDraftMasters(w http.ResponseWriter, r *http.Request) {
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
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	tx, err := s.docTransaction(ctx, tenant)
	if err != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	defer tx.Rollback()
	rows, err := readFactoryDraftRows(ctx, tx, tenant)
	if err != nil {
		http.Error(w, "Read draft registry failed; apply migration 0012", 503)
		return
	}
	decisions, err := readFactoryDraftDecisions(ctx, tx, tenant)
	if err != nil {
		http.Error(w, "Read review history failed; apply migration 0012", 503)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	truncated := len(rows) > draftReviewLimit
	if truncated {
		rows = rows[:draftReviewLimit]
	}
	groups := summarizeFactoryDrafts(rows, decisions)
	docJSON(w, 200, map[string]any{"items": groups, "truncated": truncated, "meaning": "Onboarding drafts only. Verified drafts never change operational masters, material balances or permissions."})
}
func (s *EdgeServer) handleFactoryDraftReview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", 405)
		return
	}
	var req factoryDraftReviewRequest
	if !candidateJSONBody(w, r, 4096, &req) {
		return
	}
	if err := validDraftReview(req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	tenant, err := intakeTenant()
	if err != nil || s.db == nil {
		http.Error(w, "Development tenant or PostgreSQL unavailable", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	tx, err := s.docTransaction(ctx, tenant)
	if err != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	defer tx.Rollback()
	// Serialize revisions of the same logical entity, across different source documents.
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1),hashtext($2))`, tenant, draftGroupID(req.EntityType, req.EntityKey)); err != nil {
		http.Error(w, "Unable to lock review", 503)
		return
	}
	var attrs []byte
	err = tx.QueryRowContext(ctx, `SELECT r.attributes FROM factory_proposed_register_v65b2 r
 JOIN factory_candidate_reviews_v65b2 v ON v.tenant_id=r.tenant_id AND v.candidate_id=r.candidate_id
 WHERE r.tenant_id=$1 AND r.entity_type=$2 AND r.entity_key=$3 AND r.candidate_id=$4 AND v.decision='ACCEPT_PROVISIONAL'`, tenant, req.EntityType, req.EntityKey, req.CandidateID).Scan(&attrs)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Accept this source as provisional evidence first", 409)
		return
	}
	if err != nil {
		http.Error(w, "Cannot validate review source", 503)
		return
	}
	if _, err = canonicalDraftAttributes(json.RawMessage(attrs)); err != nil {
		http.Error(w, "Invalid source attributes", 409)
		return
	}
	// A conflict demands an explicit human explanation. No conflict is silently
	// resolved by first upload, file order, or newest timestamp.
	rows, err := tx.QueryContext(ctx, `SELECT attributes FROM factory_proposed_register_v65b2 WHERE tenant_id=$1 AND entity_type=$2 AND entity_key=$3`, tenant, req.EntityType, req.EntityKey)
	if err != nil {
		http.Error(w, "Unable to check competing evidence", 503)
		return
	}
	variants := map[string]bool{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			break
		}
		var normalized string
		normalized, err = canonicalDraftAttributes(json.RawMessage(raw))
		if err != nil {
			break
		}
		variants[normalized] = true
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		http.Error(w, "Unable to validate competing evidence", 503)
		return
	}
	if len(variants) > 1 && len(strings.TrimSpace(req.Note)) < 12 {
		docJSON(w, 409, map[string]string{"error": "Conflicting documents require an explanation (at least 12 characters) before a decision."})
		return
	}
	var latest int
	err = tx.QueryRowContext(ctx, `SELECT revision FROM factory_draft_decisions_v65b3 WHERE tenant_id=$1 AND entity_type=$2 AND entity_key=$3 ORDER BY revision DESC LIMIT 1`, tenant, req.EntityType, req.EntityKey).Scan(&latest)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Cannot check revision", 503)
		return
	}
	if req.ExpectedRevision != latest {
		docJSON(w, 409, map[string]any{"error": "The record changed while you were reviewing. Refresh before saving.", "latest_revision": latest})
		return
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO factory_draft_decisions_v65b3
 (tenant_id,entity_type,entity_key,revision,candidate_id,decision,review_note,reviewed_by)
 VALUES($1,$2,$3,$4,$5,$6,$7,'dev')`, tenant, req.EntityType, req.EntityKey, latest+1, req.CandidateID, req.Decision, req.Note)
	if err != nil {
		http.Error(w, "Unable to record draft decision", 503)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "Unable to commit draft decision", 503)
		return
	}
	docJSON(w, 200, map[string]any{"entity_type": req.EntityType, "entity_key": req.EntityKey, "revision": latest + 1, "decision": req.Decision, "authority": "ONBOARDING_DRAFT_ONLY", "operational_master_updated": false})
}
