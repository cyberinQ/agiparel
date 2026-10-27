// Batch 07B: source-linked ledger comparison and immutable RECONCILIATION REQUESTS.
// Basic operator credentials are never physical stock authority; this file never
// inserts/updates authoritative stock, reservations, issues or inspection state.
package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var storesRequestKey = regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`)

type storesLedgerSnapshot struct {
	Exists            bool   `json:"exists"`
	MaterialSKU       string `json:"material_sku,omitempty"`
	ShadeBand         string `json:"shade_band,omitempty"`
	Status            string `json:"stock_state,omitempty"`
	Inspection        string `json:"inspection_status,omitempty"`
	PlantID           string `json:"plant_id,omitempty"`
	OwnerOrder        string `json:"reserved_order_id,omitempty"`
	OriginalMM        int64  `json:"original_mm"`
	RemainingMM       int64  `json:"remaining_mm"`
	WidthMM           int64  `json:"usable_width_mm"`
	OpenReservationMM int64  `json:"open_reservation_mm"`
	IssuedMM          int64  `json:"issued_mm"`
}

type storesRollAssessment struct {
	RollID              string               `json:"roll_id"`
	PassportRevision    int                  `json:"passport_revision"`
	PassportInspection  string               `json:"passport_inspection"`
	ObservedMaterialSKU string               `json:"observed_material_sku"`
	ObservedShade       string               `json:"observed_shade"`
	MeasuredMM          int64                `json:"measured_mm"`
	ObservedWidthMM     int                  `json:"observed_width_mm"`
	SourceDocumentID    string               `json:"source_document_id,omitempty"`
	EPC                 string               `json:"epc_hex,omitempty"`
	Ledger              storesLedgerSnapshot `json:"ledger"`
	Findings            []string             `json:"findings"`
	State               string               `json:"state"`
	UncommittedMM       int64                `json:"ledger_uncommitted_mm"`
	Meaning             string               `json:"meaning"`
}

type storesReconciliationInput struct {
	RollID                   string `json:"roll_id"`
	ExpectedPassportRevision int    `json:"expected_passport_revision"`
	RequestKey               string `json:"request_key"`
	Note                     string `json:"note"`
}

type storesReconciliationRecord struct {
	ID               string               `json:"id"`
	RollID           string               `json:"roll_id"`
	PassportRevision int                  `json:"passport_revision"`
	RequestKey       string               `json:"request_key"`
	Note             string               `json:"note"`
	Assessment       storesRollAssessment `json:"assessment"`
	AssessmentHash   string               `json:"assessment_hash"`
	State            string               `json:"state"`
	CreatedAt        time.Time            `json:"created_at"`
}

type storesReservationStatus struct {
	ReservationID string    `json:"reservation_id"`
	RollID        string    `json:"roll_id"`
	OrderID       string    `json:"order_id"`
	CutNo         int       `json:"cut_no"`
	ReservedMM    int64     `json:"reserved_mm"`
	IssuedMM      int64     `json:"issued_mm"`
	ReleasedMM    int64     `json:"released_mm"`
	OpenMM        int64     `json:"open_mm"`
	CreatedAt     time.Time `json:"created_at"`
}

// Evaluate sources as *observations*, not as stock authority. A reported PASS
// cannot authorize reservations. Original ledger length is historical receipt
// length; it is not the expected *remaining* length after genuine issues.
func compareStoresEvidence(v storesRollAssessment) storesRollAssessment {
	findings := make([]string, 0, 8)
	if v.PassportInspection != "PASS" {
		findings = append(findings, "PASSPORT_NOT_PASSED")
	}
	l := v.Ledger
	if !l.Exists {
		findings = append(findings, "NOT_IN_STOCK_LEDGER")
	} else {
		if v.ObservedMaterialSKU != l.MaterialSKU {
			findings = append(findings, "MATERIAL_SKU_CONFLICT")
		}
		if v.ObservedShade != l.ShadeBand {
			findings = append(findings, "SHADE_BAND_CONFLICT")
		}
		if int64(v.ObservedWidthMM) != l.WidthMM {
			findings = append(findings, "USABLE_WIDTH_CONFLICT")
		}
		if l.Inspection != "PASS" {
			findings = append(findings, "LEDGER_INSPECTION_NOT_PASSED")
		}
		if l.Status != "AVAILABLE" && l.Status != "RESERVED" {
			findings = append(findings, "LEDGER_NOT_RELEASED")
		}
		if v.MeasuredMM != l.OriginalMM {
			// An issued roll might legitimately differ from original length;
			// preserve the discrepancy for human reconciliation, not a verdict.
			findings = append(findings, "MEASURED_VS_ORIGINAL_REVIEW")
		}
		v.UncommittedMM = l.RemainingMM - l.OpenReservationMM
		if v.UncommittedMM < 0 {
			findings = append(findings, "LEDGER_OVERCOMMITTED")
		}
	}
	if len(findings) == 0 {
		findings = append(findings, "AWAITING_INDEPENDENT_VERIFICATION")
		v.State = "AWAITING_SECURE_RECONCILIATION"
	} else {
		v.State = "REQUIRES_REVIEW"
	}
	v.Findings = findings
	v.Meaning = "Comparison of operator-reported passport with separate authoritative ledger. No physical stock, reservation, inspection, or issuance changed."
	return v
}

func validateStoresReconciliation(v storesReconciliationInput) error {
	if !passportID.MatchString(v.RollID) {
		return errors.New("roll_id: invalid")
	}
	if v.ExpectedPassportRevision < 1 {
		return errors.New("expected_passport_revision: must be positive")
	}
	if !storesRequestKey.MatchString(v.RequestKey) {
		return errors.New("request_key: use 8–128 safe characters")
	}
	if len(strings.TrimSpace(v.Note)) < 12 || len(v.Note) > 1000 || strings.ContainsRune(v.Note, 0) {
		return errors.New("note: explain the review request (12–1000 characters)")
	}
	return nil
}

// Snapshot inside an existing tenant-scoped SQL transaction. Caller may lock
// the roll's advisory key before reading to serialize against new passports.
func readStoresAssessment(ctx context.Context, tx *sql.Tx, tenant, rollID string) (storesRollAssessment, error) {
	var v storesRollAssessment
	v.RollID = rollID
	var doc, epc sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT revision,inspection_result,material_sku,shade_band,measured_length_mm,usable_width_mm,source_document_id,epc_hex
       FROM store_roll_passport_events_v7 WHERE tenant_id=$1 AND roll_id=$2 ORDER BY revision DESC LIMIT 1`, tenant, rollID).
		Scan(&v.PassportRevision, &v.PassportInspection, &v.ObservedMaterialSKU, &v.ObservedShade, &v.MeasuredMM, &v.ObservedWidthMM, &doc, &epc)
	if err != nil {
		return v, err
	}
	v.SourceDocumentID, v.EPC = doc.String, epc.String
	var sku, shade, state, inspection, plant string
	var owner sql.NullString
	var original, remaining, width, open, issued int64
	err = tx.QueryRowContext(ctx, `SELECT factory_sku,shade_band,status,inspection_status,plant_id,reserved_order_id,
      ROUND(original_length_m*1000)::bigint,ROUND(remaining_length_m*1000)::bigint,ROUND(usable_width_cm*10)::bigint,
      COALESCE((SELECT SUM(reserved_mm-issued_mm-released_mm) FROM material_reservations_v5 WHERE tenant_id=$1 AND roll_id=$2),0)::bigint,
      COALESCE((SELECT SUM(length_mm) FROM material_issues_v5 WHERE tenant_id=$1 AND roll_id=$2),0)::bigint
      FROM store_rolls WHERE tenant_id=$1 AND id=$2`, tenant, rollID).
		Scan(&sku, &shade, &state, &inspection, &plant, &owner, &original, &remaining, &width, &open, &issued)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return v, err
	}
	if err == nil {
		v.Ledger = storesLedgerSnapshot{Exists: true, MaterialSKU: sku, ShadeBand: shade, Status: state, Inspection: inspection, PlantID: plant, OwnerOrder: owner.String, OriginalMM: original, RemainingMM: remaining, WidthMM: width, OpenReservationMM: open, IssuedMM: issued}
	}
	return compareStoresEvidence(v), nil
}

func (s *EdgeServer) storesBegin(ctx context.Context, tenant string) (*sql.Tx, error) {
	return s.docTransaction(ctx, tenant)
}

func (s *EdgeServer) handleStoresReconciliation(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/v1/stores/reconciliation" {
		http.NotFound(w, r)
		return
	}
	tenant, err := intakeTenant()
	if err != nil || s.db == nil {
		http.Error(w, "Stores tenant/database unavailable", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	switch r.Method {
	case http.MethodGet:
		tx, e := s.storesBegin(ctx, tenant)
		if e != nil {
			http.Error(w, "Stores unavailable", 503)
			return
		}
		defer tx.Rollback()
		rows, e := tx.QueryContext(ctx, `SELECT roll_id FROM store_roll_passport_events_v7 p
          WHERE p.tenant_id=$1 AND revision=(SELECT MAX(revision) FROM store_roll_passport_events_v7 x WHERE x.tenant_id=p.tenant_id AND x.roll_id=p.roll_id)
          ORDER BY p.recorded_at DESC,p.roll_id LIMIT 200`, tenant)
		if e != nil {
			http.Error(w, "Cannot load passports", 503)
			return
		}
		ids := make([]string, 0)
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				break
			}
			ids = append(ids, id)
		}
		if ee := rows.Err(); e == nil {
			e = ee
		}
		rows.Close()
		if e != nil {
			http.Error(w, "Cannot read passports", 503)
			return
		}
		items := make([]storesRollAssessment, 0, len(ids))
		for _, id := range ids {
			v, ee := readStoresAssessment(ctx, tx, tenant, id)
			if ee != nil {
				http.Error(w, "Cannot evaluate ledger", 503)
				return
			}
			items = append(items, v)
		}
		if e = tx.Commit(); e != nil {
			http.Error(w, "Cannot complete ledger read", 503)
			return
		}
		docJSON(w, 200, map[string]any{"items": items, "evidence": "OBSERVATION_VS_LEDGER", "authority": "READ_ONLY"})
	case http.MethodPost:
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "Expected JSON", 415)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var input storesReconciliationInput
		if dec.Decode(&input) != nil {
			http.Error(w, "Invalid JSON", 400)
			return
		}
		var extra any
		if dec.Decode(&extra) != io.EOF {
			http.Error(w, "One request only", 400)
			return
		}
		if e := validateStoresReconciliation(input); e != nil {
			docJSON(w, 400, map[string]string{"error": e.Error()})
			return
		}
		rec, replayed, e := s.saveStoresReconciliation(ctx, tenant, input)
		if e != nil {
			switch e.Error() {
			case "PASSPORT_REVISION_CHANGED", "REQUEST_KEY_REUSE":
				docJSON(w, 409, map[string]string{"error": e.Error()})
			case "PASSPORT_NOT_FOUND":
				docJSON(w, 404, map[string]string{"error": e.Error()})
			default:
				http.Error(w, "Unable to create reconciliation request", 503)
			}
			return
		}
		code := 201
		if replayed {
			code = 200
		}
		docJSON(w, code, map[string]any{"request": rec, "replayed": replayed, "authority": "REVIEW_REQUEST_ONLY"})
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", 405)
	}
}

func (s *EdgeServer) saveStoresReconciliation(ctx context.Context, tenant string, input storesReconciliationInput) (storesReconciliationRecord, bool, error) {
	var rec storesReconciliationRecord
	tx, e := s.storesBegin(ctx, tenant)
	if e != nil {
		return rec, false, e
	}
	defer tx.Rollback()
	// Lock the request key first so cross-roll concurrent replays do not race
	// into a generic unique-constraint failure. Then share 07A's roll lock.
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1),hashtext($2))`, tenant, input.RequestKey); e != nil {
		return rec, false, e
	}
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1),hashtext($2))`, tenant, input.RollID); e != nil {
		return rec, false, e
	}
	var raw []byte
	e = tx.QueryRowContext(ctx, `SELECT id::text,roll_id,passport_revision,request_key,note,assessment,assessment_hash,state,created_at
         FROM store_reconciliation_requests_v7 WHERE tenant_id=$1 AND request_key=$2`, tenant, input.RequestKey).
		Scan(&rec.ID, &rec.RollID, &rec.PassportRevision, &rec.RequestKey, &rec.Note, &raw, &rec.AssessmentHash, &rec.State, &rec.CreatedAt)
	if e == nil {
		if rec.RollID != input.RollID || rec.PassportRevision != input.ExpectedPassportRevision || rec.Note != input.Note {
			return storesReconciliationRecord{}, false, errors.New("REQUEST_KEY_REUSE")
		}
		if e = json.Unmarshal(raw, &rec.Assessment); e != nil {
			return rec, false, e
		}
		return rec, true, tx.Commit()
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return rec, false, e
	}
	assessment, e := readStoresAssessment(ctx, tx, tenant, input.RollID)
	if errors.Is(e, sql.ErrNoRows) {
		return rec, false, errors.New("PASSPORT_NOT_FOUND")
	}
	if e != nil {
		return rec, false, e
	}
	if assessment.PassportRevision != input.ExpectedPassportRevision {
		return rec, false, errors.New("PASSPORT_REVISION_CHANGED")
	}
	snapshot, e := json.Marshal(assessment)
	if e != nil {
		return rec, false, e
	}
	digest := sha256.Sum256(snapshot)
	rec = storesReconciliationRecord{RollID: input.RollID, PassportRevision: input.ExpectedPassportRevision, RequestKey: input.RequestKey, Note: input.Note, Assessment: assessment, AssessmentHash: hex.EncodeToString(digest[:]), State: "PENDING_INDEPENDENT_RECONCILIATION"}
	e = tx.QueryRowContext(ctx, `INSERT INTO store_reconciliation_requests_v7(tenant_id,roll_id,passport_revision,request_key,note,assessment,assessment_hash)
       VALUES($1,$2,$3,$4,$5,$6::jsonb,$7) RETURNING id::text,created_at`, tenant, input.RollID, input.ExpectedPassportRevision, input.RequestKey, input.Note, string(snapshot), rec.AssessmentHash).Scan(&rec.ID, &rec.CreatedAt)
	if e != nil {
		return storesReconciliationRecord{}, false, e
	}
	return rec, false, tx.Commit()
}

func (s *EdgeServer) handleStoresReconciliationRequests(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", 405)
		return
	}
	tenant, e := intakeTenant()
	if e != nil || s.db == nil {
		http.Error(w, "Stores unavailable", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	tx, e := s.storesBegin(ctx, tenant)
	if e != nil {
		http.Error(w, "Unavailable", 503)
		return
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, `SELECT id::text,roll_id,passport_revision,request_key,note,assessment,assessment_hash,state,created_at
         FROM store_reconciliation_requests_v7 WHERE tenant_id=$1 ORDER BY created_at DESC,id DESC LIMIT 100`, tenant)
	if e != nil {
		http.Error(w, "Cannot load reconciliation history", 503)
		return
	}
	items := make([]storesReconciliationRecord, 0)
	for rows.Next() {
		var v storesReconciliationRecord
		var raw []byte
		if e = rows.Scan(&v.ID, &v.RollID, &v.PassportRevision, &v.RequestKey, &v.Note, &raw, &v.AssessmentHash, &v.State, &v.CreatedAt); e != nil {
			break
		}
		if e = json.Unmarshal(raw, &v.Assessment); e != nil {
			break
		}
		items = append(items, v)
	}
	if ee := rows.Err(); e == nil {
		e = ee
	}
	rows.Close()
	if e != nil {
		http.Error(w, "Cannot read history", 503)
		return
	}
	if e = tx.Commit(); e != nil {
		http.Error(w, "Cannot finish", 503)
		return
	}
	docJSON(w, 200, map[string]any{"items": items, "authority": "REVIEW_REQUEST_ONLY"})
}

func (s *EdgeServer) handleStoresReservations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", 405)
		return
	}
	tenant, e := intakeTenant()
	if e != nil || s.db == nil {
		http.Error(w, "Stores unavailable", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	tx, e := s.storesBegin(ctx, tenant)
	if e != nil {
		http.Error(w, "Unavailable", 503)
		return
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, `SELECT id::text,roll_id,order_id,cut_no,reserved_mm,issued_mm,released_mm,created_at
        FROM material_reservations_v5 WHERE tenant_id=$1 ORDER BY created_at DESC,id DESC LIMIT 200`, tenant)
	if e != nil {
		http.Error(w, "Cannot load reservations", 503)
		return
	}
	items := make([]storesReservationStatus, 0)
	for rows.Next() {
		var v storesReservationStatus
		if e = rows.Scan(&v.ReservationID, &v.RollID, &v.OrderID, &v.CutNo, &v.ReservedMM, &v.IssuedMM, &v.ReleasedMM, &v.CreatedAt); e != nil {
			break
		}
		v.OpenMM = v.ReservedMM - v.IssuedMM - v.ReleasedMM
		items = append(items, v)
	}
	if ee := rows.Err(); e == nil {
		e = ee
	}
	rows.Close()
	if e != nil {
		http.Error(w, "Cannot read reservations", 503)
		return
	}
	if e = tx.Commit(); e != nil {
		http.Error(w, "Cannot finish", 503)
		return
	}
	docJSON(w, 200, map[string]any{"items": items, "authority": "SECURE_SIGNED_MATERIAL_API_ONLY"})
}
