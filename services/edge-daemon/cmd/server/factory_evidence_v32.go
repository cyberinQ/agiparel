// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// F3.18C + F3.19A/B: independently signed human evidence, not device
// attestation, final buyer AQL acceptance, safety release or production credit.
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

const factoryEvidenceAuthorityV32 = "SIGNED_HUMAN_EVIDENCE_NOT_DEVICE_CERTIFICATION_AQL_ACCEPTANCE_OR_SAFETY_RELEASE"

type factoryEvidenceCommandV32 struct {
	Kind                string `json:"kind"`
	CommandID           string `json:"command_id"`
	PlantID             string `json:"plant_id"`
	ObservationID       string `json:"observation_id,omitempty"`
	InstrumentID        string `json:"instrument_id,omitempty"`
	CalibrationRevision int    `json:"calibration_revision,omitempty"`
	PlanID              string `json:"plan_id,omitempty"`
	GarmentID           string `json:"garment_id,omitempty"`
	Finding             string `json:"finding,omitempty"`
	DefectCode          string `json:"defect_code,omitempty"`
	EventKind           string `json:"event_kind,omitempty"`
	ExpectedFragments   int    `json:"expected_fragments,omitempty"`
	RecoveredFragments  int    `json:"recovered_fragments,omitempty"`
	EvidenceDocumentID  string `json:"evidence_document_id,omitempty"`
	EvidenceSHA256      string `json:"evidence_sha256,omitempty"`
	Note                string `json:"note"`
}
type factoryEvidenceRequestV32 struct {
	Command factoryEvidenceCommandV32 `json:"command"`
}

func factoryEvidencePolicyV32(c factoryEvidenceCommandV32) (string, string, bool) {
	if !sewCommandID.MatchString(c.CommandID) || !sewFields(c.PlantID) || len(strings.TrimSpace(c.Note)) < 20 ||
		len(c.Note) > 800 || strings.ContainsRune(c.Note, 0) {
		return "", "", false
	}
	switch c.Kind {
	case "CALIBRATION_LINK":
		if !sewUUID(c.ObservationID) || len(c.InstrumentID) < 4 || len(c.InstrumentID) > 80 || c.CalibrationRevision <= 0 ||
			c.PlanID != "" || c.GarmentID != "" || c.Finding != "" || c.DefectCode != "" || c.EventKind != "" ||
			c.EvidenceDocumentID != "" || c.EvidenceSHA256 != "" || c.ExpectedFragments != 0 || c.RecoveredFragments != 0 {
			return "", "", false
		}
		return "stores.fabric.calibration.link", "stores_inspector", true
	case "AQL_SAMPLE":
		if !sewUUID(c.PlanID) || !sewUUID(c.GarmentID) || c.ObservationID != "" || c.InstrumentID != "" ||
			c.CalibrationRevision != 0 || c.EventKind != "" || c.EvidenceDocumentID != "" || c.EvidenceSHA256 != "" ||
			c.ExpectedFragments != 0 || c.RecoveredFragments != 0 {
			return "", "", false
		}
		if c.Finding != "PASS" && c.Finding != "MINOR" && c.Finding != "MAJOR" && c.Finding != "CRITICAL" {
			return "", "", false
		}
		if (c.Finding == "PASS") != (c.DefectCode == "") {
			return "", "", false
		}
		if c.DefectCode != "" && (!sewFields(c.DefectCode) || len(c.DefectCode) > 64) {
			return "", "", false
		}
		return "quality.aql.inspect", "quality_inspector", true
	case "NEEDLE_SAFETY":
		if !sewUUID(c.GarmentID) || c.PlanID != "" || c.ObservationID != "" || c.CalibrationRevision != 0 ||
			c.Finding != "" || c.DefectCode != "" || len(c.EvidenceDocumentID) == 0 || len(c.EvidenceDocumentID) > 36 ||
			!f317bHash.MatchString(c.EvidenceSHA256) || c.ExpectedFragments < 0 || c.ExpectedFragments > 20 ||
			c.RecoveredFragments < 0 || c.RecoveredFragments > c.ExpectedFragments {
			return "", "", false
		}
		switch c.EventKind {
		case "NEEDLE_BREAK":
			if c.ExpectedFragments == 0 {
				return "", "", false
			}
		case "DETECTOR_CHALLENGE_PASS", "DETECTOR_CHALLENGE_FAIL", "DETECTOR_REJECT":
			if len(c.InstrumentID) < 4 || len(c.InstrumentID) > 80 {
				return "", "", false
			}
		default:
			return "", "", false
		}
		return "quality.needle.report", "quality_inspector", true
	}
	return "", "", false
}
func factoryEvidenceChallengeV32(c factoryEvidenceCommandV32) string {
	return "FACTORY_EVIDENCE_V32:" + sewDigest(c)
}

type factoryEvidenceAPIv32 struct {
	server   *EdgeServer
	material *materialAPI
	enabled  bool
}

func newFactoryEvidenceAPIv32(s *EdgeServer) *factoryEvidenceAPIv32 {
	p := &factoryEvidenceAPIv32{server: s}
	if s == nil || s.db == nil || os.Getenv("AGIPAREL_FACTORY_EVIDENCE_V32_ENABLED") != "true" ||
		(os.Getenv("ENV") != "production" && os.Getenv("ENV") != "staging") {
		return p
	}
	p.material = newMaterialAPI(s.db)
	p.enabled = p.material.initErr == nil && p.material.verifier != nil
	return p
}
func (p *factoryEvidenceAPIv32) ready(w http.ResponseWriter) bool {
	if !p.enabled {
		docJSON(w, 503, map[string]string{"error": "ERR_F32_EVIDENCE_DISABLED"})
		return false
	}
	return true
}
func (p *factoryEvidenceAPIv32) challenge(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var req factoryEvidenceRequestV32
	if !sewProdStrictJSON(w, r, &req) {
		return
	}
	action, role, ok := factoryEvidencePolicyV32(req.Command)
	if !ok {
		docJSON(w, 422, map[string]string{"error": "ERR_F32_FIELDS"})
		return
	}
	docJSON(w, 200, map[string]any{"challenge": factoryEvidenceChallengeV32(req.Command),
		"required_action": action, "required_role": role, "authority": factoryEvidenceAuthorityV32,
		"calibrated_device_reading": false, "buyer_lot_acceptance": false, "safety_clearance": false})
}
func (p *factoryEvidenceAPIv32) event(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var req factoryEvidenceRequestV32
	if !sewProdStrictJSON(w, r, &req) {
		return
	}
	c := req.Command
	action, role, ok := factoryEvidencePolicyV32(c)
	if !ok {
		docJSON(w, 422, map[string]string{"error": "ERR_F32_FIELDS"})
		return
	}
	token, err := bearer(r)
	if err != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_F32_AUTH"})
		return
	}
	actor, err := p.material.verifier.Verify(token, action, factoryEvidenceChallengeV32(c), role, true)
	if err != nil || !validChapterActor(actor) || actor.PlantID != c.PlantID || actor.JWTID == "" || len(actor.JWTID) > 255 {
		docJSON(w, 401, map[string]string{"error": "ERR_F32_ASSERTION"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	id, replay, err := p.persist(ctx, c, action, actor, token)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	code := http.StatusCreated
	if replay {
		code = http.StatusOK
	}
	docJSON(w, code, map[string]any{"id": id, "replayed": replay, "kind": c.Kind, "authority": factoryEvidenceAuthorityV32,
		"device_signed": false, "buyer_lot_acceptance": false, "qc_release": false, "safety_clearance": false, "shipment": false})
}
func (p *factoryEvidenceAPIv32) persist(ctx context.Context, c factoryEvidenceCommandV32, action string, actor attestation.Claims, token string) (string, bool, error) {
	tx, err := p.server.docTransaction(ctx, actor.TenantID)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SET LOCAL ROLE agiparel_app`); err != nil {
		return "", false, err
	}
	if err = workforceChapterRequire(ctx, tx, actor, action); err != nil {
		return "", false, err
	}
	if err = sewTxLock(ctx, tx, actor.TenantID, "F32:"+c.Kind+":"+c.CommandID); err != nil {
		return "", false, err
	}
	table, subject := factoryEvidenceTableV32(c.Kind)
	if table == "" {
		return "", false, sewFail(422, "ERR_F32_KIND")
	}
	hash := sewDigest(c)
	var id, oldHash, oldSubject string
	err = tx.QueryRowContext(ctx, `SELECT id::text,command_sha256,`+subject+` FROM `+table+` WHERE tenant_id=$1 AND command_id=$2`, actor.TenantID, c.CommandID).Scan(&id, &oldHash, &oldSubject)
	if err == nil {
		if oldHash != hash || oldSubject != actor.Subject {
			return "", false, sewFail(409, "ERR_F32_COMMAND_CONFLICT")
		}
		return id, true, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, err
	}
	switch c.Kind {
	case "CALIBRATION_LINK":
		err = tx.QueryRowContext(ctx, `INSERT INTO fabric_calibration_links_v32
    (tenant_id,plant_id,observation_id,instrument_id,calibration_revision,method_note,command_id,command_sha256,
     inspector_subject,inspector_proof_id,assertion_sha256)
    VALUES($1,$2,$3::uuid,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id::text`,
			actor.TenantID, c.PlantID, c.ObservationID, c.InstrumentID, c.CalibrationRevision, c.Note, c.CommandID, hash,
			actor.Subject, actor.JWTID, sewProdHashString(token)).Scan(&id)
	case "AQL_SAMPLE":
		var defect any
		if c.DefectCode != "" {
			defect = c.DefectCode
		}
		err = tx.QueryRowContext(ctx, `INSERT INTO buyer_aql_sample_events_v32
    (tenant_id,plant_id,plan_id,garment_id,finding,defect_code,note,command_id,command_sha256,
     inspector_subject,inspector_proof_id,assertion_sha256)
    VALUES($1,$2,$3::uuid,$4::uuid,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id::text`,
			actor.TenantID, c.PlantID, c.PlanID, c.GarmentID, c.Finding, defect, c.Note, c.CommandID, hash,
			actor.Subject, actor.JWTID, sewProdHashString(token)).Scan(&id)
	case "NEEDLE_SAFETY":
		var instrument any
		if c.InstrumentID != "" {
			instrument = c.InstrumentID
		}
		err = tx.QueryRowContext(ctx, `INSERT INTO needle_safety_events_v32
    (tenant_id,plant_id,garment_id,instrument_id,event_kind,expected_fragments,recovered_fragments,
    evidence_document_id,evidence_sha256,note,command_id,command_sha256,actor_subject,proof_id,assertion_sha256)
    VALUES($1,$2,$3::uuid,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) RETURNING id::text`,
			actor.TenantID, c.PlantID, c.GarmentID, instrument, c.EventKind, c.ExpectedFragments, c.RecoveredFragments,
			c.EvidenceDocumentID, c.EvidenceSHA256, c.Note, c.CommandID, hash, actor.Subject, actor.JWTID, sewProdHashString(token)).Scan(&id)
	}
	if err != nil {
		return "", false, jobworkAdmissionError(err)
	}
	return id, false, tx.Commit()
}
func factoryEvidenceTableV32(kind string) (string, string) {
	switch kind {
	case "CALIBRATION_LINK":
		return "fabric_calibration_links_v32", "inspector_subject"
	case "AQL_SAMPLE":
		return "buyer_aql_sample_events_v32", "inspector_subject"
	case "NEEDLE_SAFETY":
		return "needle_safety_events_v32", "actor_subject"
	}
	return "", ""
}
func registerFactoryEvidenceRoutesV32(mux *http.ServeMux, s *EdgeServer) {
	p := newFactoryEvidenceAPIv32(s)
	mux.HandleFunc("/api/v1/factory/evidence/challenge", p.challenge)
	mux.HandleFunc("/api/v1/factory/evidence/events", p.event)
	mux.HandleFunc("/api/v1/quality/aql/sampling", p.aqlReport)
	mux.HandleFunc("/api/v1/quality/needle/incidents", p.needleReport)
}

func (p *factoryEvidenceAPIv32) authorizedReadV32(w http.ResponseWriter, r *http.Request, scope string) (*sql.Tx, context.Context, context.CancelFunc, bool) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return nil, nil, nil, false
	}
	if !p.ready(w) {
		return nil, nil, nil, false
	}
	q := r.URL.Query()
	for k, v := range q {
		if len(v) != 1 || (k != "tenant_id" && k != "plant_id" && k != scope) {
			docJSON(w, 422, map[string]string{"error": "ERR_F32_READ_QUERY"})
			return nil, nil, nil, false
		}
	}
	t, pl, x := q.Get("tenant_id"), q.Get("plant_id"), q.Get(scope)
	if len(q["tenant_id"]) != 1 || len(q["plant_id"]) != 1 || len(q[scope]) != 1 ||
		!sewFields(t, pl) || !sewUUID(x) {
		docJSON(w, 422, map[string]string{"error": "ERR_F32_READ_QUERY"})
		return nil, nil, nil, false
	}
	access := newChapterAccessAPI(p.server)
	if !access.enabled {
		docJSON(w, 503, map[string]string{"error": "ERR_F32_READ_DISABLED"})
		return nil, nil, nil, false
	}
	actor, err := (&commissioningAPI{access: access}).identify(r, t, pl)
	if err != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_F32_READ_AUTH"})
		return nil, nil, nil, false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	tx, err := commissionTx(ctx, p.server.db, t)
	if err != nil {
		cancel()
		sewProdReply(w, err)
		return nil, nil, nil, false
	}
	allowed, err := workforceChapterPermit(ctx, tx, actor, "quality.read")
	if err != nil || !allowed {
		tx.Rollback()
		cancel()
		if err != nil {
			sewProdReply(w, err)
		} else {
			docJSON(w, 403, map[string]string{"error": "ERR_F32_READ_GRANT"})
		}
		return nil, nil, nil, false
	}
	return tx, ctx, cancel, true
}
func (p *factoryEvidenceAPIv32) aqlReport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	tx, ctx, cancel, ok := p.authorizedReadV32(w, r, "plan_id")
	if !ok {
		return
	}
	defer cancel()
	defer tx.Rollback()
	q := r.URL.Query()
	var planID, lot string
	var rev int
	var sampleSize, total, pass, minor, major, critical int64
	var enabled bool
	err := tx.QueryRowContext(ctx, `SELECT id::text,lot_code,revision,cardinality(sample_garment_ids),enabled
 FROM buyer_aql_plans_v32 WHERE tenant_id=$1 AND plant_id=$2 AND id=$3::uuid`,
		q.Get("tenant_id"), q.Get("plant_id"), q.Get("plan_id")).Scan(&planID, &lot, &rev, &sampleSize, &enabled)
	if errors.Is(err, sql.ErrNoRows) {
		docJSON(w, 404, map[string]string{"error": "ERR_F319A_PLAN_NOT_FOUND"})
		return
	}
	if err != nil {
		sewProdReply(w, err)
		return
	}
	err = tx.QueryRowContext(ctx, `SELECT count(*),count(*) FILTER(WHERE finding='PASS'),
 count(*) FILTER(WHERE finding='MINOR'),count(*) FILTER(WHERE finding='MAJOR'),
 count(*) FILTER(WHERE finding='CRITICAL') FROM buyer_aql_sample_events_v32
 WHERE tenant_id=$1 AND plant_id=$2 AND plan_id=$3::uuid`, q.Get("tenant_id"), q.Get("plant_id"), planID).
		Scan(&total, &pass, &minor, &major, &critical)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		sewProdReply(w, err)
		return
	}
	docJSON(w, 200, map[string]any{"plan_id": planID, "lot_code": lot, "revision": rev, "plan_enabled": enabled,
		"selected_garment_count": sampleSize, "recorded_findings": total, "uninspected_count": sampleSize - total,
		"pass": pass, "minor": minor, "major": major, "critical": critical,
		"buyer_aql_lot_accepted": false, "authority": "SIGNED_SAMPLED_FINDINGS_NOT_LOT_ACCEPTANCE_OR_QC_RELEASE"})
}
func (p *factoryEvidenceAPIv32) needleReport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	tx, ctx, cancel, ok := p.authorizedReadV32(w, r, "garment_id")
	if !ok {
		return
	}
	defer cancel()
	defer tx.Rollback()
	q := r.URL.Query()
	var garmentID string
	var count, breaks, failures, rejections int64
	err := tx.QueryRowContext(ctx, `SELECT id::text FROM gen_garments_v18 WHERE tenant_id=$1 AND plant_id=$2 AND id=$3::uuid`,
		q.Get("tenant_id"), q.Get("plant_id"), q.Get("garment_id")).Scan(&garmentID)
	if errors.Is(err, sql.ErrNoRows) {
		docJSON(w, 404, map[string]string{"error": "ERR_F319B_GARMENT_NOT_FOUND"})
		return
	}
	if err != nil {
		sewProdReply(w, err)
		return
	}
	err = tx.QueryRowContext(ctx, `SELECT count(*),count(*) FILTER(WHERE event_kind='NEEDLE_BREAK'),
 count(*) FILTER(WHERE event_kind='DETECTOR_CHALLENGE_FAIL'),
 count(*) FILTER(WHERE event_kind='DETECTOR_REJECT')
 FROM needle_safety_events_v32 WHERE tenant_id=$1 AND plant_id=$2 AND garment_id=$3::uuid`,
		q.Get("tenant_id"), q.Get("plant_id"), garmentID).Scan(&count, &breaks, &failures, &rejections)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		sewProdReply(w, err)
		return
	}
	docJSON(w, 200, map[string]any{"garment_id": garmentID, "recorded_events": count,
		"needle_break_reports": breaks, "detector_challenge_fail_reports": failures, "detector_reject_reports": rejections,
		"safety_clearance": false, "quality_release": false, "authority": "HUMAN_REPORTED_SAFETY_EVIDENCE_ONLY"})
}
