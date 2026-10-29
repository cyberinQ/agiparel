// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// F3.19C/D + F3.20A: independently signed safety review and internal AQL
// assessment; dock preflight remains READ-ONLY and is NOT shipment custody.
package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/attestation"
)

type factoryReleaseCommandV33 struct {
	Kind               string `json:"kind"`
	CommandID          string `json:"command_id"`
	PlantID            string `json:"plant_id"`
	IncidentID         string `json:"incident_id,omitempty"`
	FollowupPassID     string `json:"followup_pass_id,omitempty"`
	GarmentID          string `json:"garment_id,omitempty"`
	AccountedFragments int    `json:"accounted_fragments,omitempty"`
	EvidenceDocumentID string `json:"evidence_document_id,omitempty"`
	EvidenceSHA256     string `json:"evidence_sha256,omitempty"`
	PlanID             string `json:"plan_id,omitempty"`
	Decision           string `json:"decision,omitempty"`
	PassCount          int    `json:"pass_count,omitempty"`
	MinorCount         int    `json:"minor_count,omitempty"`
	MajorCount         int    `json:"major_count,omitempty"`
	CriticalCount      int    `json:"critical_count,omitempty"`
	Note               string `json:"note"`
}
type factoryReleaseRequestV33 struct {
	Command factoryReleaseCommandV33 `json:"command"`
}
type factoryReleaseAPIv33 struct {
	base    *factoryEvidenceAPIv32
	enabled bool
}

const factoryReleaseAuthorityV33 = "INDEPENDENT_INTERNAL_REVIEW_NOT_BUYER_ACCEPTANCE_SHIPMENT_OR_MACHINE_CERTIFICATION"

func newFactoryReleaseAPIv33(s *EdgeServer) *factoryReleaseAPIv33 {
	p := &factoryReleaseAPIv33{base: newFactoryEvidenceAPIv32(s)}
	p.enabled = p.base.enabled && os.Getenv("AGIPAREL_FACTORY_RELEASE_V33_ENABLED") == "true"
	return p
}
func (p *factoryReleaseAPIv33) ready(w http.ResponseWriter) bool {
	if !p.enabled {
		docJSON(w, 503, map[string]string{"error": "ERR_F33_RELEASE_DISABLED"})
		return false
	}
	return true
}
func factoryReleasePolicyV33(c factoryReleaseCommandV33) (string, string, bool) {
	if !sewCommandID.MatchString(c.CommandID) || !sewFields(c.PlantID) || len(strings.TrimSpace(c.Note)) < 20 || len(c.Note) > 800 || strings.ContainsRune(c.Note, 0) {
		return "", "", false
	}
	switch c.Kind {
	case "SAFETY_CLEARANCE":
		if !sewUUID(c.IncidentID) || !sewUUID(c.FollowupPassID) || !sewUUID(c.GarmentID) ||
			c.AccountedFragments < 0 || c.AccountedFragments > 20 || !sewFields(c.EvidenceDocumentID) || len(c.EvidenceDocumentID) > 36 ||
			!f317bHash.MatchString(c.EvidenceSHA256) || c.PlanID != "" || c.Decision != "" ||
			c.PassCount != 0 || c.MinorCount != 0 || c.MajorCount != 0 || c.CriticalCount != 0 {
			return "", "", false
		}
		return "quality.needle.clear", "quality_inspector", true
	case "AQL_REVIEW":
		if !sewUUID(c.PlanID) || c.IncidentID != "" || c.FollowupPassID != "" || c.GarmentID != "" ||
			c.AccountedFragments != 0 || c.EvidenceDocumentID != "" || c.EvidenceSHA256 != "" ||
			c.PassCount < 0 || c.MinorCount < 0 || c.MajorCount < 0 || c.CriticalCount < 0 ||
			(c.Decision != "INTERNAL_RECOMMEND_ACCEPT" && c.Decision != "HOLD_FOR_BUYER_REVIEW") {
			return "", "", false
		}
		return "quality.aql.review", "quality_inspector", true
	}
	return "", "", false
}
func factoryReleasePathV33(path, kind string) bool {
	switch path {
	case "/api/v1/quality/safety/clearances/challenge", "/api/v1/quality/safety/clearances":
		return kind == "SAFETY_CLEARANCE"
	case "/api/v1/quality/aql/reviews/challenge", "/api/v1/quality/aql/reviews":
		return kind == "AQL_REVIEW"
	}
	return false
}
func factoryReleaseChallengeV33(c factoryReleaseCommandV33) string {
	return "FACTORY_REVIEW_V33:" + sewDigest(c)
}
func (p *factoryReleaseAPIv33) challenge(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var req factoryReleaseRequestV33
	if !sewProdStrictJSON(w, r, &req) {
		return
	}
	action, role, ok := factoryReleasePolicyV33(req.Command)
	if !ok || !factoryReleasePathV33(r.URL.Path, req.Command.Kind) {
		docJSON(w, 422, map[string]string{"error": "ERR_F33_FIELDS"})
		return
	}
	docJSON(w, 200, map[string]any{"challenge": factoryReleaseChallengeV33(req.Command), "required_action": action, "required_role": role, "required_auth": "webauthn_or_fido2", "authority": factoryReleaseAuthorityV33, "shipment": false, "buyer_acceptance": false, "qc_release": false})
}
func (p *factoryReleaseAPIv33) event(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var req factoryReleaseRequestV33
	if !sewProdStrictJSON(w, r, &req) {
		return
	}
	c := req.Command
	action, role, ok := factoryReleasePolicyV33(c)
	if !ok || !factoryReleasePathV33(r.URL.Path, c.Kind) {
		docJSON(w, 422, map[string]string{"error": "ERR_F33_FIELDS"})
		return
	}
	token, err := bearer(r)
	if err != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_F33_AUTH"})
		return
	}
	a, err := p.base.material.verifier.Verify(token, action, factoryReleaseChallengeV33(c), role, true)
	if err != nil || !validChapterActor(a) || a.PlantID != c.PlantID || a.JWTID == "" || len(a.JWTID) > 255 {
		docJSON(w, 401, map[string]string{"error": "ERR_F33_ASSERTION"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	id, replay, err := p.persist(ctx, c, action, a, token)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	code := 201
	if replay {
		code = 200
	}
	docJSON(w, code, map[string]any{"id": id, "kind": c.Kind, "replayed": replay, "authority": factoryReleaseAuthorityV33, "shipment": false, "buyer_acceptance": false, "qc_release": false})
}
func (p *factoryReleaseAPIv33) persist(ctx context.Context, c factoryReleaseCommandV33, action string, a attestation.Claims, token string) (string, bool, error) {
	tx, err := p.base.server.docTransaction(ctx, a.TenantID)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SET LOCAL ROLE agiparel_app`); err != nil {
		return "", false, err
	}
	if err = workforceChapterRequire(ctx, tx, a, action); err != nil {
		return "", false, err
	}
	if err = sewTxLock(ctx, tx, a.TenantID, "F33:"+c.Kind+":"+c.CommandID); err != nil {
		return "", false, err
	}
	table := "buyer_aql_reviews_v33"
	if c.Kind == "SAFETY_CLEARANCE" {
		table = "needle_safety_clearances_v33"
	}
	hash := sewDigest(c)
	var id, oldHash, oldSubject string
	err = tx.QueryRowContext(ctx, `SELECT id::text,command_sha256,reviewer_subject FROM `+table+` WHERE tenant_id=$1 AND command_id=$2`, a.TenantID, c.CommandID).Scan(&id, &oldHash, &oldSubject)
	if err == nil {
		if hash != oldHash || a.Subject != oldSubject {
			return "", false, sewFail(409, "ERR_F33_COMMAND_CONFLICT")
		}
		return id, true, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, err
	}
	if c.Kind == "SAFETY_CLEARANCE" {
		err = tx.QueryRowContext(ctx, `INSERT INTO needle_safety_clearances_v33
   (tenant_id,plant_id,incident_id,garment_id,followup_pass_id,accounted_fragments,evidence_document_id,evidence_sha256,note,command_id,command_sha256,reviewer_subject,reviewer_proof_id,assertion_sha256)
   VALUES($1,$2,$3::uuid,$4::uuid,$5::uuid,$6,$7,$8,$9,$10,$11,$12,$13,$14) RETURNING id::text`,
			a.TenantID, c.PlantID, c.IncidentID, c.GarmentID, c.FollowupPassID, c.AccountedFragments, c.EvidenceDocumentID, c.EvidenceSHA256, c.Note, c.CommandID, hash, a.Subject, a.JWTID, sewProdHashString(token)).Scan(&id)
	} else {
		err = tx.QueryRowContext(ctx, `INSERT INTO buyer_aql_reviews_v33
   (tenant_id,plant_id,plan_id,decision,pass_count,minor_count,major_count,critical_count,note,command_id,command_sha256,reviewer_subject,reviewer_proof_id,assertion_sha256)
   VALUES($1,$2,$3::uuid,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) RETURNING id::text`,
			a.TenantID, c.PlantID, c.PlanID, c.Decision, c.PassCount, c.MinorCount, c.MajorCount, c.CriticalCount, c.Note, c.CommandID, hash, a.Subject, a.JWTID, sewProdHashString(token)).Scan(&id)
	}
	if err != nil {
		return "", false, jobworkAdmissionError(err)
	}
	return id, false, tx.Commit()
}
func (p *factoryReleaseAPIv33) dockReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	if !p.ready(w) {
		return
	}
	q := r.URL.Query()
	for key, values := range q {
		if (key != "tenant_id" && key != "plant_id" && key != "manifest_id") || len(values) != 1 {
			docJSON(w, 422, map[string]string{"error": "ERR_F320A_QUERY"})
			return
		}
	}
	tenant, plant, manifest := q.Get("tenant_id"), q.Get("plant_id"), q.Get("manifest_id")
	if len(q["tenant_id"]) != 1 || len(q["plant_id"]) != 1 || len(q["manifest_id"]) != 1 || !sewFields(tenant, plant) || !sewUUID(manifest) {
		docJSON(w, 422, map[string]string{"error": "ERR_F320A_QUERY"})
		return
	}
	access := newChapterAccessAPI(p.base.server)
	if !access.enabled {
		docJSON(w, 503, map[string]string{"error": "ERR_F320A_AUTH_DISABLED"})
		return
	}
	a, err := (&commissioningAPI{access: access}).identify(r, tenant, plant)
	if err != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_F320A_AUTH"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, err := commissionTx(ctx, p.base.server.db, tenant)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	defer tx.Rollback()
	allow, err := workforceChapterPermit(ctx, tx, a, "packing.read")
	if err != nil {
		sewProdReply(w, err)
		return
	}
	if !allow {
		docJSON(w, 403, map[string]string{"error": "ERR_F320A_GRANT"})
		return
	}
	var spec string
	var cartonCount, staged int64
	err = tx.QueryRowContext(ctx, `SELECT spec_id::text,count(*) FILTER(WHERE action='ADD_CARTON'),count(*) FILTER(WHERE action='STAGE')
 FROM pack_manifest_events_v22 WHERE tenant_id=$1 AND plant_id=$2 AND manifest_id=$3::uuid
 GROUP BY spec_id`, tenant, plant, manifest).Scan(&spec, &cartonCount, &staged)
	if errors.Is(err, sql.ErrNoRows) {
		docJSON(w, 404, map[string]string{"error": "ERR_F320A_MANIFEST_NOT_FOUND"})
		return
	}
	if err != nil {
		sewProdReply(w, err)
		return
	}
	// All cartons must still have a final SEAL and no unresolved garment safety incident.
	var invalidCartons, blockedGarments int64
	err = tx.QueryRowContext(ctx, `SELECT
 count(*) FILTER(WHERE NOT EXISTS(SELECT 1 FROM pack_carton_events_v22 z WHERE z.tenant_id=m.tenant_id AND z.carton_id=m.carton_id AND z.action='SEAL')),
 COALESCE(sum((SELECT count(*) FROM pack_carton_events_v22 e WHERE e.tenant_id=m.tenant_id AND e.carton_id=m.carton_id AND e.action='ADD' AND (garment_needle_safety_blocked_v32(e.tenant_id,e.garment_id)
 OR (SELECT q.action FROM qc_garment_events_v21 q WHERE q.tenant_id=e.tenant_id AND q.garment_id=e.garment_id ORDER BY q.event_no DESC LIMIT 1) IS DISTINCT FROM 'QUALITY_RELEASE'))),0)
 FROM pack_manifest_events_v22 m WHERE m.tenant_id=$1 AND m.plant_id=$2 AND m.manifest_id=$3::uuid AND m.action='ADD_CARTON'`, tenant, plant, manifest).Scan(&invalidCartons, &blockedGarments)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	// Buyer plans are selected by the manifest's immutable packing spec lay.
	var activePlans, recommendedPlans int64
	err = tx.QueryRowContext(ctx, `SELECT count(*),count(*) FILTER(WHERE EXISTS(
 SELECT 1 FROM buyer_aql_reviews_v33 r JOIN buyer_aql_review_rules_v33 rules
 ON rules.tenant_id=r.tenant_id AND rules.plan_id=r.plan_id AND rules.enabled
 JOIN documents doc ON doc.tenant_id=rules.tenant_id AND doc.id=rules.source_document_id
 WHERE r.tenant_id=p.tenant_id AND r.plan_id=p.id AND r.decision='INTERNAL_RECOMMEND_ACCEPT'
 AND doc.status='APPROVED' AND upper(doc.issuer)='CUSTOMER'
 AND doc.file_checksum=rules.source_sha256))
 FROM buyer_aql_plans_v32 p JOIN pack_carton_specs_v22 s ON s.tenant_id=p.tenant_id AND s.lay_draft_id=p.lay_draft_id
 WHERE s.tenant_id=$1 AND s.plant_id=$2 AND s.id=$3::uuid AND p.enabled AND p.plant_id=$2
 AND p.revision=(SELECT max(x.revision) FROM buyer_aql_plans_v32 x WHERE x.tenant_id=p.tenant_id AND x.plant_id=p.plant_id AND x.lot_code=p.lot_code)`, tenant, plant, spec).Scan(&activePlans, &recommendedPlans)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		sewProdReply(w, err)
		return
	}
	blocked := staged != 1 || cartonCount == 0 || invalidCartons != 0 || blockedGarments != 0 || activePlans == 0 || recommendedPlans != activePlans
	docJSON(w, 200, map[string]any{"manifest_id": manifest, "staged_count": staged, "cartons": cartonCount, "unsealed_cartons": invalidCartons,
		"garments_with_unresolved_safety_or_qc": blockedGarments, "current_buyer_aql_plans": activePlans, "internally_recommended_plans": recommendedPlans,
		"dock_preflight_blocked": blocked, "authority": "READ_ONLY_DOCK_PREFLIGHT_NOT_SHIPMENT_OR_CARRIER_CUSTODY",
		"carrier_receipt_verified": false, "asn_transmitted": false, "shipment_credit": false, "buyer_acceptance": false})
}
func registerFactoryReleaseRoutesV33(mux *http.ServeMux, s *EdgeServer) {
	p := newFactoryReleaseAPIv33(s)
	mux.HandleFunc("/api/v1/quality/safety/clearances/challenge", p.challenge)
	mux.HandleFunc("/api/v1/quality/safety/clearances", p.event)
	mux.HandleFunc("/api/v1/quality/aql/reviews/challenge", p.challenge)
	mux.HandleFunc("/api/v1/quality/aql/reviews", p.event)
	mux.HandleFunc("/api/v1/packing/dock/readiness", p.dockReadiness)
}
