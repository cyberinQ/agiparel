// Gate 06.5-B4: explicit promotion of reviewed factory evidence to an immutable
// *reference catalog* only. No factory operations, payroll, physical inventory,
// capacity release, material transaction, or purchasing authority is modified.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

type referenceMasterRequest struct {
	EntityType       string `json:"entity_type"`
	EntityKey        string `json:"entity_key"`
	CandidateID      string `json:"candidate_id"`
	DraftRevision    int    `json:"draft_revision"`
	ExpectedRevision int    `json:"expected_revision"`
	Action           string `json:"action"`
	RateBasis        string `json:"rate_basis,omitempty"`
	Note             string `json:"note"`
	Acknowledged     bool   `json:"acknowledged"`
}

type referenceMasterRecord struct {
	EntityType           string          `json:"entity_type"`
	EntityKey            string          `json:"entity_key"`
	Revision             int             `json:"revision"`
	Action               string          `json:"action"`
	CandidateID          string          `json:"candidate_id"`
	DraftRevision        int             `json:"draft_revision"`
	Attributes           json.RawMessage `json:"attributes"`
	RateBasis            string          `json:"rate_basis,omitempty"`
	Note                 string          `json:"reviewer_note"`
	ReviewedBy           string          `json:"reviewed_by"`
	RecordedAt           time.Time       `json:"recorded_at"`
	Filename             string          `json:"filename"`
	DocumentID           string          `json:"document_id"`
	SourceOrdinal        int             `json:"source_ordinal"`
	RevalidationRequired bool            `json:"revalidation_required"`
}

func (s *EdgeServer) handleFactoryReferenceMasters(w http.ResponseWriter, r *http.Request) {
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
	history := r.URL.Query().Get("history") == "1"
	// Use a tenant-filtered subquery, not a security-definer view or arbitrary SQL.
	query := `SELECT v.entity_type,v.entity_key,v.revision,v.action,v.candidate_id,v.draft_revision,v.attributes,
 COALESCE(v.rate_basis,''),v.reviewer_note,v.reviewed_by,v.recorded_at,d.filename,c.document_id,c.source_ordinal,
 EXISTS(SELECT 1 FROM factory_proposed_register_v65b2 p
  JOIN factory_candidate_reviews_v65b2 cr ON cr.tenant_id=p.tenant_id AND cr.candidate_id=p.candidate_id
  JOIN factory_draft_decisions_v65b3 dr ON dr.tenant_id=v.tenant_id AND dr.entity_type=v.entity_type AND dr.entity_key=v.entity_key AND dr.revision=v.draft_revision
  WHERE p.tenant_id=v.tenant_id AND p.entity_type=v.entity_type AND p.entity_key=v.entity_key
    AND cr.decision='ACCEPT_PROVISIONAL' AND cr.reviewed_at>dr.reviewed_at) AS stale
 FROM `
	if history {
		query += `factory_reference_versions_v65b4 v`
	} else {
		query += `(SELECT DISTINCT ON(entity_type,entity_key) * FROM factory_reference_versions_v65b4
 WHERE tenant_id=$1 ORDER BY entity_type,entity_key,revision DESC) v`
	}
	query += ` JOIN factory_document_candidates_v65b2 c ON c.tenant_id=v.tenant_id AND c.candidate_id=v.candidate_id
 JOIN document_sources_v65 d ON d.tenant_id=c.tenant_id AND d.id=c.document_id
 WHERE v.tenant_id=$1 ORDER BY v.entity_type,v.entity_key,v.revision DESC LIMIT 501`
	rows, err := tx.QueryContext(ctx, query, tenant)
	if err != nil {
		http.Error(w, "Cannot read reference catalog; apply migration 0013", 503)
		return
	}
	items := []referenceMasterRecord{}
	for rows.Next() {
		var x referenceMasterRecord
		var raw []byte
		err = rows.Scan(&x.EntityType, &x.EntityKey, &x.Revision, &x.Action, &x.CandidateID, &x.DraftRevision, &raw, &x.RateBasis, &x.Note, &x.ReviewedBy, &x.RecordedAt, &x.Filename, &x.DocumentID, &x.SourceOrdinal, &x.RevalidationRequired)
		if err != nil {
			break
		}
		x.Attributes = json.RawMessage(raw)
		items = append(items, x)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		http.Error(w, "Unable to read catalog", 503)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	truncated := len(items) > 500
	if truncated {
		items = items[:500]
	}
	docJSON(w, 200, map[string]any{"items": items, "history": history, "truncated": truncated, "authority": "FACTORY_REFERENCE_ONLY", "meaning": "Published reference records support onboarding review, not machine dispatch, personnel access, payroll, stock, purchase orders, or approved costing."})
}

func (s *EdgeServer) handleFactoryReferenceRevision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", 405)
		return
	}
	var req referenceMasterRequest
	if !candidateJSONBody(w, r, 4096, &req) {
		return
	}
	if err := validReferenceAction(req.Action, req.EntityType, req.EntityKey, req.CandidateID, req.ExpectedRevision, req.DraftRevision, req.Note, req.RateBasis); err != nil {
		docJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if !req.Acknowledged {
		docJSON(w, 400, map[string]string{"error": "Confirm that this updates an onboarding reference only, not production authority."})
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
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1),hashtext($2))`, tenant, draftGroupID(req.EntityType, req.EntityKey)); err != nil {
		http.Error(w, "Unable to lock reference", 503)
		return
	}
	var latest int
	var prior referenceMasterRecord
	var priorAttrs []byte
	err = tx.QueryRowContext(ctx, `SELECT revision,action,candidate_id,draft_revision,attributes,COALESCE(rate_basis,'')
 FROM factory_reference_versions_v65b4 WHERE tenant_id=$1 AND entity_type=$2 AND entity_key=$3
 ORDER BY revision DESC LIMIT 1`, tenant, req.EntityType, req.EntityKey).Scan(&latest, &prior.Action, &prior.CandidateID, &prior.DraftRevision, &priorAttrs, &prior.RateBasis)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Cannot validate catalog revision; apply migration 0013", 503)
		return
	}
	if latest != req.ExpectedRevision {
		docJSON(w, 409, map[string]any{"error": "The reference changed. Refresh and review the latest revision.", "latest_revision": latest})
		return
	}
	var attributes []byte
	if req.Action == "WITHDRAW" {
		if latest == 0 || prior.Action != "PUBLISH" {
			docJSON(w, 409, map[string]string{"error": "Only a currently published reference can be withdrawn."})
			return
		}
		if req.CandidateID != prior.CandidateID || req.DraftRevision != prior.DraftRevision || req.RateBasis != prior.RateBasis {
			docJSON(w, 409, map[string]string{"error": "Refresh the current published reference before withdrawing."})
			return
		}
		attributes = priorAttrs
	} else {
		var state, decisionCandidate string
		var reviewedAt time.Time
		err = tx.QueryRowContext(ctx, `SELECT decision,candidate_id,reviewed_at FROM factory_draft_decisions_v65b3
  WHERE tenant_id=$1 AND entity_type=$2 AND entity_key=$3 ORDER BY revision DESC LIMIT 1`, tenant, req.EntityType, req.EntityKey).Scan(&state, &decisionCandidate, &reviewedAt)
		if errors.Is(err, sql.ErrNoRows) {
			docJSON(w, 409, map[string]string{"error": "Review the document-backed factory draft before publishing a reference."})
			return
		}
		if err != nil {
			http.Error(w, "Cannot load source review", 503)
			return
		}
		var draftRevision int
		err = tx.QueryRowContext(ctx, `SELECT revision FROM factory_draft_decisions_v65b3 WHERE tenant_id=$1 AND entity_type=$2 AND entity_key=$3 ORDER BY revision DESC LIMIT 1`, tenant, req.EntityType, req.EntityKey).Scan(&draftRevision)
		if err != nil {
			http.Error(w, "Cannot validate draft revision", 503)
			return
		}
		if state != "VERIFY_DRAFT" || decisionCandidate != req.CandidateID || draftRevision != req.DraftRevision {
			docJSON(w, 409, map[string]string{"error": "The selected draft is not the latest verified version. Resolve the factory evidence first."})
			return
		}
		var newEvidence int
		err = tx.QueryRowContext(ctx, `SELECT count(*) FROM factory_proposed_register_v65b2 p
  JOIN factory_candidate_reviews_v65b2 cr ON cr.tenant_id=p.tenant_id AND cr.candidate_id=p.candidate_id
  WHERE p.tenant_id=$1 AND p.entity_type=$2 AND p.entity_key=$3 AND cr.decision='ACCEPT_PROVISIONAL'
   AND cr.reviewed_at>$4`, tenant, req.EntityType, req.EntityKey, reviewedAt).Scan(&newEvidence)
		if err != nil {
			http.Error(w, "Cannot check newer evidence", 503)
			return
		}
		if newEvidence > 0 {
			docJSON(w, 409, map[string]string{"error": "New source evidence arrived after the draft review. Re-review the disagreement first."})
			return
		}
		err = tx.QueryRowContext(ctx, `SELECT p.attributes FROM factory_proposed_register_v65b2 p
  JOIN factory_candidate_reviews_v65b2 cr ON cr.tenant_id=p.tenant_id AND cr.candidate_id=p.candidate_id
  WHERE p.tenant_id=$1 AND p.entity_type=$2 AND p.entity_key=$3 AND p.candidate_id=$4 AND cr.decision='ACCEPT_PROVISIONAL'`, tenant, req.EntityType, req.EntityKey, req.CandidateID).Scan(&attributes)
		if errors.Is(err, sql.ErrNoRows) {
			docJSON(w, 409, map[string]string{"error": "Source candidate no longer qualifies for this reference."})
			return
		}
		if err != nil {
			http.Error(w, "Cannot validate source", 503)
			return
		}
		if _, err = parseReferenceFacts(req.EntityType, req.EntityKey, attributes, req.RateBasis); err != nil {
			docJSON(w, 409, map[string]string{"error": err.Error()})
			return
		}
		if latest > 0 && prior.Action == "PUBLISH" && prior.CandidateID == req.CandidateID && prior.DraftRevision == req.DraftRevision && prior.RateBasis == req.RateBasis {
			docJSON(w, 409, map[string]string{"error": "This reference is already published from that review. Add new evidence or withdraw it first."})
			return
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO factory_reference_versions_v65b4
 (tenant_id,entity_type,entity_key,revision,action,candidate_id,draft_revision,attributes,rate_basis,reviewer_note,reviewed_by)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8::jsonb,NULLIF($9,''),$10,'dev')`, tenant, req.EntityType, req.EntityKey, latest+1, req.Action, req.CandidateID, req.DraftRevision, string(attributes), req.RateBasis, strings.TrimSpace(req.Note))
	if err != nil {
		http.Error(w, "Unable to record reference revision", 503)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "Unable to commit reference revision", 503)
		return
	}
	docJSON(w, 200, map[string]any{"entity_type": req.EntityType, "entity_key": req.EntityKey, "revision": latest + 1, "action": req.Action, "authority": "FACTORY_REFERENCE_ONLY", "operational_master_updated": false, "stock_or_payroll_updated": false})
}
