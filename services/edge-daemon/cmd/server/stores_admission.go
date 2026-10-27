// Batch 07C: signed, two-person NEW-ROLL admission only. A basic dev operator
// can read preparation status, but never authorize any inventory mutation.
// Existing ledger records, shortage adjustments, reservations and custody
// issuances remain governed by their separate authenticated workflows.
package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/attestation"
)

var admissionUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type admissionCandidate struct {
	RequestID        string               `json:"request_id"`
	RollID           string               `json:"roll_id"`
	PassportRevision int                  `json:"passport_revision"`
	AssessmentHash   string               `json:"assessment_hash"`
	Inspection       string               `json:"inspection_result"`
	MaterialSKU      string               `json:"material_sku"`
	SupplierLot      string               `json:"supplier_lot"`
	ShadeBand        string               `json:"shade_band"`
	BinLocation      string               `json:"bin_location"`
	EPC              string               `json:"epc_hex"`
	SourceDocumentID string               `json:"source_document_id"`
	MeasuredMM       int64                `json:"measured_mm"`
	WidthMM          int                  `json:"usable_width_mm"`
	Defects          string               `json:"defects"`
	LatestRevision   int                  `json:"latest_revision"`
	LedgerExists     bool                 `json:"ledger_exists"`
	IsAdmitted       bool                 `json:"is_admitted"`
	AdmittedAt       *time.Time           `json:"admitted_at,omitempty"`
	Assessment       storesRollAssessment `json:"assessment"`
}

type admissionStatus struct {
	RequestID        string     `json:"request_id"`
	RollID           string     `json:"roll_id"`
	Revision         int        `json:"passport_revision"`
	MeasuredMM       int64      `json:"measured_mm"`
	EPC              string     `json:"epc_hex,omitempty"`
	SourceDocumentID string     `json:"source_document_id,omitempty"`
	State            string     `json:"state"`
	Reasons          []string   `json:"reasons"`
	AdmittedAt       *time.Time `json:"admitted_at,omitempty"`
}

// An admission requires an explicit PASS inspection, immutable source document,
// permanently bound EPC, latest passport, absent ledger roll, and a matching
// immutable review snapshot. This is not proof that a physical reader scanned.
func admissionReasons(c admissionCandidate) []string {
	reasons := make([]string, 0, 8)
	if c.IsAdmitted {
		return []string{"ALREADY_ADMITTED"}
	}
	if c.LedgerExists {
		reasons = append(reasons, "ROLL_ALREADY_IN_LEDGER")
	}
	if c.LatestRevision != c.PassportRevision {
		reasons = append(reasons, "PASSPORT_REVISION_STALE")
	}
	if c.Inspection != "PASS" || c.Assessment.PassportInspection != "PASS" {
		reasons = append(reasons, "INSPECTION_NOT_PASSED")
	}
	if c.EPC == "" || c.Assessment.EPC != c.EPC {
		reasons = append(reasons, "RFID_EPC_NOT_BOUND")
	}
	if c.SourceDocumentID == "" || c.Assessment.SourceDocumentID != c.SourceDocumentID {
		reasons = append(reasons, "SOURCE_DOCUMENT_REQUIRED")
	}
	if c.Assessment.Ledger.Exists || c.Assessment.PassportRevision != c.PassportRevision || c.Assessment.RollID != c.RollID || c.Assessment.MeasuredMM != c.MeasuredMM || c.Assessment.ObservedMaterialSKU != c.MaterialSKU || c.Assessment.ObservedShade != c.ShadeBand || c.Assessment.ObservedWidthMM != c.WidthMM {
		reasons = append(reasons, "REVIEW_SNAPSHOT_CONFLICT")
	}
	if c.MeasuredMM <= 0 || c.WidthMM < 100 || c.WidthMM > 4000 {
		reasons = append(reasons, "INVALID_MEASUREMENTS")
	}
	if !passportID.MatchString(c.RollID) || !passportCode.MatchString(c.MaterialSKU) {
		reasons = append(reasons, "INVALID_ROLL_IDENTITY")
	}
	if !admissionUUID.MatchString(c.RequestID) {
		reasons = append(reasons, "INVALID_REVIEW_REQUEST")
	}
	if len(c.AssessmentHash) != 64 {
		reasons = append(reasons, "INVALID_SNAPSHOT_HASH")
	}
	return reasons
}

type admissionChallengeFields struct {
	Version          string `json:"version"`
	Tenant           string `json:"tenant_id"`
	Plant            string `json:"plant_id"`
	RequestID        string `json:"request_id"`
	RollID           string `json:"roll_id"`
	PassportRevision int    `json:"passport_revision"`
	AssessmentHash   string `json:"assessment_hash"`
	MaterialSKU      string `json:"material_sku"`
	SupplierLot      string `json:"supplier_lot"`
	ShadeBand        string `json:"shade_band"`
	BinLocation      string `json:"bin_location"`
	EPC              string `json:"epc_hex"`
	SourceDocumentID string `json:"source_document_id"`
	MeasuredMM       int64  `json:"measured_mm"`
	WidthMM          int    `json:"usable_width_mm"`
	Defects          string `json:"defects"`
}

func storesAdmissionChallenge(tenant, plant string, c admissionCandidate) string {
	v := admissionChallengeFields{"AGIPAREL_ROLL_ADMISSION_V7C", tenant, plant, c.RequestID, c.RollID, c.PassportRevision, c.AssessmentHash, c.MaterialSKU, c.SupplierLot, c.ShadeBand, c.BinLocation, c.EPC, c.SourceDocumentID, c.MeasuredMM, c.WidthMM, c.Defects}
	b, _ := json.Marshal(v)
	digest := sha256.Sum256(b)
	return hex.EncodeToString(digest[:])
}

func loadAdmissionCandidate(ctx context.Context, tx *sql.Tx, tenant, requestID string) (admissionCandidate, error) {
	var c admissionCandidate
	var snapshot []byte
	var epc, doc sql.NullString
	var when sql.NullTime
	var state string
	err := tx.QueryRowContext(ctx, `SELECT q.id::text,q.roll_id,q.passport_revision,q.assessment_hash,q.assessment,
   p.inspection_result,p.material_sku,p.supplier_lot,p.shade_band,p.bin_location,p.epc_hex,p.source_document_id,
   p.measured_length_mm,p.usable_width_mm,p.defects,
   COALESCE((SELECT max(revision) FROM store_roll_passport_events_v7 x WHERE x.tenant_id=q.tenant_id AND x.roll_id=q.roll_id),0),
   EXISTS(SELECT 1 FROM store_rolls x WHERE x.tenant_id=q.tenant_id AND x.id=q.roll_id),
   a.admitted_at, q.state
  FROM store_reconciliation_requests_v7 q
  JOIN store_roll_passport_events_v7 p ON p.tenant_id=q.tenant_id AND p.roll_id=q.roll_id AND p.revision=q.passport_revision
  LEFT JOIN store_roll_admissions_v7 a ON a.tenant_id=q.tenant_id AND a.request_id=q.id
  WHERE q.tenant_id=$1 AND q.id=$2::uuid`, tenant, requestID).
		Scan(&c.RequestID, &c.RollID, &c.PassportRevision, &c.AssessmentHash, &snapshot,
			&c.Inspection, &c.MaterialSKU, &c.SupplierLot, &c.ShadeBand, &c.BinLocation, &epc, &doc,
			&c.MeasuredMM, &c.WidthMM, &c.Defects, &c.LatestRevision, &c.LedgerExists, &when, &state)
	if err != nil {
		return c, err
	}
	c.EPC, c.SourceDocumentID = epc.String, doc.String
	if state != "PENDING_INDEPENDENT_RECONCILIATION" {
		return c, errors.New("ERR_REVIEW_STATE")
	}
	if err = json.Unmarshal(snapshot, &c.Assessment); err != nil {
		return c, err
	}
	canonical, err := json.Marshal(c.Assessment)
	if err != nil {
		return c, err
	}
	h := sha256.Sum256(canonical)
	if hex.EncodeToString(h[:]) != c.AssessmentHash {
		return c, errors.New("ERR_REVIEW_SNAPSHOT_TAMPERED")
	}
	c.IsAdmitted = when.Valid
	if when.Valid {
		v := when.Time
		c.AdmittedAt = &v
	}
	return c, nil
}

func (s *EdgeServer) handleStoresAdmissions(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/v1/stores/admissions" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", 405)
		return
	}
	tenant, err := intakeTenant()
	if err != nil || s.db == nil {
		http.Error(w, "Stores unavailable", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	tx, err := s.docTransaction(ctx, tenant)
	if err != nil {
		http.Error(w, "Stores unavailable", 503)
		return
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id::text FROM store_reconciliation_requests_v7 WHERE tenant_id=$1 ORDER BY created_at DESC,id DESC LIMIT 100`, tenant)
	if err != nil {
		http.Error(w, "Review queue unavailable", 503)
		return
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			break
		}
		ids = append(ids, id)
	}
	if e := rows.Err(); err == nil {
		err = e
	}
	rows.Close()
	if err != nil {
		http.Error(w, "Review queue unavailable", 503)
		return
	}
	results := make([]admissionStatus, 0, len(ids))
	for _, id := range ids {
		c, e := loadAdmissionCandidate(ctx, tx, tenant, id)
		if e != nil {
			http.Error(w, "Review evidence unavailable", 503)
			return
		}
		reasons := admissionReasons(c)
		state := "READY_FOR_SIGNED_REVIEW"
		if c.IsAdmitted {
			state = "ADMITTED_TO_LEDGER"
		} else if len(reasons) > 0 {
			state = "NOT_READY"
		}
		results = append(results, admissionStatus{RequestID: c.RequestID, RollID: c.RollID, Revision: c.PassportRevision, MeasuredMM: c.MeasuredMM, EPC: c.EPC, SourceDocumentID: c.SourceDocumentID, State: state, Reasons: reasons, AdmittedAt: c.AdmittedAt})
	}
	if tx.Commit() != nil {
		http.Error(w, "Review read failed", 503)
		return
	}
	docJSON(w, 200, map[string]any{"items": results, "authority": "SIGNED_IDP_ONLY", "identity_enabled": newMaterialAPI(s.db).initErr == nil})
}

type admissionSubmitBody struct {
	RequestID            string `json:"request_id"`
	ManagerAttestation   string `json:"manager_attestation"`
	InspectorAttestation string `json:"inspector_attestation"`
}

type admissionReceipt struct {
	ID         string    `json:"id"`
	RequestID  string    `json:"request_id"`
	RollID     string    `json:"roll_id"`
	Revision   int       `json:"passport_revision"`
	PlantID    string    `json:"plant_id"`
	AdmittedMM int64     `json:"admitted_mm"`
	AdmittedAt time.Time `json:"admitted_at"`
}

func (a *materialAPI) rollAdmissionChallenge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", 405)
		return
	}
	if !a.ready(w) {
		return
	}
	actor, err := a.authorize(r, "material.roll.admit.prepare", "stores_manager", true)
	if err != nil {
		materialError(w, err)
		return
	}
	id := r.URL.Query().Get("request_id")
	if !admissionUUID.MatchString(id) {
		materialError(w, errors.New("ERR_REVIEW_ID"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	tx, err := a.repository.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		materialError(w, err)
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id',$1,true)`, actor.TenantID); err != nil {
		materialError(w, err)
		return
	}
	c, err := loadAdmissionCandidate(ctx, tx, actor.TenantID, id)
	if err != nil {
		materialAdmissionError(w, err)
		return
	}
	if why := admissionReasons(c); len(why) > 0 {
		respondMaterial(w, 409, map[string]any{"error": "ERR_ROLL_NOT_READY", "reasons": why})
		return
	}
	if err = tx.Commit(); err != nil {
		materialError(w, err)
		return
	}
	respondMaterial(w, 200, map[string]any{"challenge": storesAdmissionChallenge(actor.TenantID, actor.PlantID, c), "request_id": id, "roll_id": c.RollID, "passport_revision": c.PassportRevision, "action": "material.roll.admit.witness", "required_manager_role": "stores_manager", "required_inspector_role": "stores_inspector", "proof": "Ed25519 JWT + FIDO2/WebAuthn", "identity_enabled": true})
}

func materialAdmissionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		respondMaterial(w, 404, map[string]string{"error": "ERR_REVIEW_NOT_FOUND"})
	case strings.HasPrefix(err.Error(), "ERR_"):
		materialError(w, err)
	default:
		materialError(w, err)
	}
}

func (a *materialAPI) rollAdmission(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !a.ready(w) {
		return
	}
	actor, err := a.authorize(r, "material.roll.admit.submit", "stores_manager", true)
	if err != nil {
		materialError(w, err)
		return
	}
	var input admissionSubmitBody
	if err = decodeMaterialJSON(w, r, &input); err != nil {
		http.Error(w, "Bad JSON", 400)
		return
	}
	if !admissionUUID.MatchString(input.RequestID) {
		materialError(w, errors.New("ERR_REVIEW_ID"))
		return
	}
	if len(input.ManagerAttestation) == 0 || len(input.InspectorAttestation) == 0 {
		materialError(w, errors.New("ERR_WITNESS_REQUIRED"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 18*time.Second)
	defer cancel()
	receipt, err := a.saveSignedRollAdmission(ctx, actor, input)
	if err != nil {
		materialAdmissionError(w, err)
		return
	}
	respondMaterial(w, 201, receipt)
}

func (a *materialAPI) saveSignedRollAdmission(ctx context.Context, actor attestation.Claims, input admissionSubmitBody) (admissionReceipt, error) {
	var result admissionReceipt
	tx, err := a.repository.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id',$1,true)`, actor.TenantID); err != nil {
		return result, err
	}
	// The request lock and the same per-roll advisory lock as passport writer
	// serialize against any new observation revision.
	var rollID string
	err = tx.QueryRowContext(ctx, `SELECT roll_id FROM store_reconciliation_requests_v7 WHERE tenant_id=$1 AND id=$2::uuid`, actor.TenantID, input.RequestID).Scan(&rollID)
	if err != nil {
		return result, err
	}
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1),hashtext($2))`, actor.TenantID, rollID); err != nil {
		return result, err
	}
	c, err := loadAdmissionCandidate(ctx, tx, actor.TenantID, input.RequestID)
	if err != nil {
		return result, err
	}
	if why := admissionReasons(c); len(why) > 0 {
		return result, errors.New("ERR_ROLL_NOT_READY_" + strings.Join(why, "_"))
	}
	challenge := storesAdmissionChallenge(actor.TenantID, actor.PlantID, c)
	mgr, err := a.verifier.Verify(input.ManagerAttestation, "material.roll.admit.witness", challenge, "stores_manager", true)
	if err != nil {
		return result, err
	}
	ins, err := a.verifier.Verify(input.InspectorAttestation, "material.roll.admit.witness", challenge, "stores_inspector", true)
	if err != nil {
		return result, err
	}
	if mgr.TenantID != actor.TenantID || ins.TenantID != actor.TenantID || mgr.PlantID != actor.PlantID || ins.PlantID != actor.PlantID || mgr.Subject != actor.Subject || mgr.Subject == ins.Subject || mgr.JWTID == ins.JWTID {
		return result, errors.New("ERR_INDEPENDENT_INSPECTION_REQUIRED")
	}
	if len(mgr.Subject) > 128 || len(ins.Subject) > 128 || len(mgr.JWTID) > 255 || len(ins.JWTID) > 255 {
		return result, errors.New("ERR_WITNESS_IDENTITY_LENGTH")
	}
	// For new stock, measurement and width are immutable source observations.
	// No partial receipts or manual balance adjustments through this endpoint.
	_, err = tx.ExecContext(ctx, `INSERT INTO store_rolls
   (id,tenant_id,plant_id,factory_sku,original_length_m,remaining_length_m,usable_width_cm,shade_band,defect_map_json,bin_location,status,inspection_status)
   VALUES($1,$2,$3,$4,$5::numeric/1000,$5::numeric/1000,$6::numeric/10,$7,jsonb_build_array(jsonb_build_object('reported_defects',$8)), $9,'AVAILABLE','PASS')`,
		c.RollID, actor.TenantID, actor.PlantID, c.MaterialSKU, c.MeasuredMM, c.WidthMM, c.ShadeBand, c.Defects, c.BinLocation)
	if err != nil {
		return result, err
	}
	mh := sha256.Sum256([]byte(input.ManagerAttestation))
	ih := sha256.Sum256([]byte(input.InspectorAttestation))
	err = tx.QueryRowContext(ctx, `INSERT INTO store_roll_admissions_v7
 (tenant_id,request_id,roll_id,passport_revision,plant_id,assessment_hash,signed_challenge_hash,admitted_mm,manager_subject,inspector_subject,manager_proof_id,inspector_proof_id,manager_assertion_hash,inspector_assertion_hash)
 VALUES($1,$2::uuid,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
 RETURNING id::text,admitted_at`, actor.TenantID, c.RequestID, c.RollID, c.PassportRevision, actor.PlantID, c.AssessmentHash, challenge, c.MeasuredMM, mgr.Subject, ins.Subject, mgr.JWTID, ins.JWTID, hex.EncodeToString(mh[:]), hex.EncodeToString(ih[:])).Scan(&result.ID, &result.AdmittedAt)
	if err != nil {
		return admissionReceipt{}, err
	}
	for _, proof := range []struct{ id, role string }{{mgr.JWTID, "STORES_MANAGER"}, {ins.JWTID, "STORES_INSPECTOR"}} {
		if _, err = tx.ExecContext(ctx, `INSERT INTO store_roll_admission_proofs_v7(tenant_id,proof_id,admission_id,witness_role) VALUES($1,$2,$3::uuid,$4)`, actor.TenantID, proof.id, result.ID, proof.role); err != nil {
			return admissionReceipt{}, err
		}
	}
	result.RequestID = c.RequestID
	result.RollID = c.RollID
	result.Revision = c.PassportRevision
	result.PlantID = actor.PlantID
	result.AdmittedMM = c.MeasuredMM
	outboxBody, err := json.Marshal(result)
	if err != nil {
		return admissionReceipt{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO material_outbox_v5(tenant_id,aggregate_kind,aggregate_id,event_kind,payload)
	 VALUES($1,'Roll',$2,'StoreRollAdmitted',$3::jsonb)`, actor.TenantID, c.RollID, string(outboxBody)); err != nil {
		return admissionReceipt{}, err
	}
	if err = tx.Commit(); err != nil {
		return admissionReceipt{}, err
	}
	return result, nil
}
