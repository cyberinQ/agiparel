// Batch 07A: source-linked, append-only roll receipt/inspection observations.
// Operator Basic login is not material authority. Nothing here mutates stock,
// reservations, issued lengths, CEO exceptions or physical RFID readers.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/internal/web"
)

var passportID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)
var passportCode = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:/-]{0,127}$`)
var passportEPC = regexp.MustCompile(`^[0-9A-F]{24,64}$`)

type rollPassportInput struct {
	RollID           string `json:"roll_id"`
	EPC              string `json:"epc_hex"`
	MaterialSKU      string `json:"material_sku"`
	SupplierLot      string `json:"supplier_lot"`
	ShadeBand        string `json:"shade_band"`
	BinLocation      string `json:"bin_location"`
	DeclaredLengthM  string `json:"declared_length_m"`
	MeasuredLengthM  string `json:"measured_length_m"`
	UsableWidthMM    int    `json:"usable_width_mm"`
	InspectionResult string `json:"inspection_result"`
	Defects          string `json:"defects"`
	SourceDocumentID string `json:"source_document_id"`
	SourceNote       string `json:"source_note"`
	ExpectedRevision int    `json:"expected_revision"`
}

type rollPassport struct {
	RollID                 string    `json:"roll_id"`
	Revision               int       `json:"revision"`
	EventKind              string    `json:"event_kind"`
	EPC                    string    `json:"epc_hex"`
	MaterialSKU            string    `json:"material_sku"`
	SupplierLot            string    `json:"supplier_lot"`
	ShadeBand              string    `json:"shade_band"`
	BinLocation            string    `json:"bin_location"`
	DeclaredLengthM        string    `json:"declared_length_m"`
	MeasuredLengthM        string    `json:"measured_length_m"`
	LengthVarianceM        string    `json:"length_variance_m"`
	UsableWidthMM          int       `json:"usable_width_mm"`
	InspectionResult       string    `json:"inspection_result"`
	Defects                string    `json:"defects"`
	SourceDocumentID       string    `json:"source_document_id,omitempty"`
	SourceNote             string    `json:"source_note"`
	RecordedAt             time.Time `json:"recorded_at"`
	EvidenceClass          string    `json:"evidence_class"`
	LedgerStatus           string    `json:"ledger_status"`
	LedgerRemainingM       string    `json:"ledger_remaining_m,omitempty"`
	LedgerOpenReservationM string    `json:"ledger_open_reservation_m,omitempty"`
}

// Interpret decimal meters exactly, never using floating point for stock.
func passportMillimeters(v string) (int64, error) {
	n, e := scenarioScaled(v, 3, 1000000000, false)
	if e != nil {
		return 0, errors.New("enter positive metres with at most three decimals")
	}
	return n, nil
}
func passportMeters(mm int64) string { return fmt.Sprintf("%d.%03d", mm/1000, mm%1000) }

// Positive values are excess and negative values are measured shortages.
func passportVariance(mm int64) string {
	if mm < 0 {
		return "-" + passportMeters(-mm)
	}
	return "+" + passportMeters(mm)
}
func validatePassport(v rollPassportInput) (int64, int64, error) {
	if !passportID.MatchString(v.RollID) {
		return 0, 0, errors.New("roll_id: use letters, numbers, dash or underscore, maximum 64")
	}
	if !passportCode.MatchString(v.MaterialSKU) {
		return 0, 0, errors.New("material_sku: a valid material code is required")
	}
	if len(v.SupplierLot) == 0 || len(v.SupplierLot) > 128 || strings.ContainsAny(v.SupplierLot, "\x00\r\n") {
		return 0, 0, errors.New("supplier_lot: required, maximum 128 characters")
	}
	if len(v.ShadeBand) == 0 || len(v.ShadeBand) > 32 || strings.ContainsAny(v.ShadeBand, "\x00\r\n") {
		return 0, 0, errors.New("shade_band: required, maximum 32 characters")
	}
	if len(v.BinLocation) == 0 || len(v.BinLocation) > 64 || strings.ContainsAny(v.BinLocation, "\x00\r\n") {
		return 0, 0, errors.New("bin_location: required, maximum 64 characters")
	}
	if v.EPC != "" && (len(v.EPC)%2 != 0 || !passportEPC.MatchString(v.EPC)) {
		return 0, 0, errors.New("epc_hex: enter 24–64 uppercase hex characters or leave blank for an untagged roll")
	}
	if v.UsableWidthMM < 100 || v.UsableWidthMM > 4000 {
		return 0, 0, errors.New("usable_width_mm: enter 100–4000")
	}
	if v.InspectionResult != "PASS" && v.InspectionResult != "HOLD" && v.InspectionResult != "REJECTED" {
		return 0, 0, errors.New("inspection_result: choose PASS, HOLD or REJECTED")
	}
	if len(strings.TrimSpace(v.SourceNote)) < 8 || len(v.SourceNote) > 1000 || strings.ContainsRune(v.SourceNote, 0) {
		return 0, 0, errors.New("source_note: provide 8–1000 characters explaining the physical observation")
	}
	if len(v.Defects) > 2000 || strings.ContainsRune(v.Defects, 0) {
		return 0, 0, errors.New("defects: maximum 2000 characters")
	}
	if v.SourceDocumentID != "" && (len(v.SourceDocumentID) != 36 || !strings.HasPrefix(v.SourceDocumentID, "DOC-")) {
		return 0, 0, errors.New("source_document_id: invalid document reference")
	}
	if v.ExpectedRevision < 0 {
		return 0, 0, errors.New("expected_revision: cannot be negative")
	}
	declared, e := passportMillimeters(v.DeclaredLengthM)
	if e != nil {
		return 0, 0, fmt.Errorf("declared_length_m: %w", e)
	}
	measured, e := passportMillimeters(v.MeasuredLengthM)
	if e != nil {
		return 0, 0, fmt.Errorf("measured_length_m: %w", e)
	}
	return declared, measured, nil
}

func (s *EdgeServer) handleStoresWorkspace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", 405)
		return
	}
	b, e := web.GetStoresHTML()
	if e != nil {
		http.Error(w, "Stores workspace unavailable", 503)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(b)
}
func (s *EdgeServer) handleStoresPassports(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/v1/stores/passports" {
		http.NotFound(w, r)
		return
	}
	tenant, e := intakeTenant()
	if e != nil || s.db == nil {
		http.Error(w, "Stores database or tenant unavailable", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	switch r.Method {
	case http.MethodGet:
		if rollID := r.URL.Query().Get("roll_id"); rollID != "" {
			if !passportID.MatchString(rollID) {
				docJSON(w, 400, map[string]string{"error": "Invalid roll ID"})
				return
			}
			history, e := s.loadPassportHistory(ctx, tenant, rollID)
			if e != nil {
				http.Error(w, "Unable to load roll history", 503)
				return
			}
			docJSON(w, 200, map[string]any{"items": history, "authority": "OBSERVATIONS_ONLY", "roll_id": rollID})
			return
		}
		list, e := s.loadRollPassports(ctx, tenant)
		if e != nil {
			http.Error(w, "Stores passport query failed", 503)
			return
		}
		docJSON(w, 200, map[string]any{"items": list, "authority": "OBSERVATIONS_ONLY", "issuance_gate": "SECURE_IDP_REQUIRED"})
	case http.MethodPost:
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "Expected JSON", 415)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 12<<10)
		var v rollPassportInput
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if e = dec.Decode(&v); e != nil {
			http.Error(w, "Invalid passport JSON", 400)
			return
		}
		var extra any
		if dec.Decode(&extra) != io.EOF {
			http.Error(w, "Only one passport may be submitted", 400)
			return
		}
		declared, measured, e := validatePassport(v)
		if e != nil {
			docJSON(w, 400, map[string]string{"error": e.Error()})
			return
		}
		record, e := s.saveRollPassport(ctx, tenant, v, declared, measured)
		if e != nil {
			switch e.Error() {
			case "REVISION_CONFLICT", "RFID_ALREADY_BOUND", "RFID_CHANGE_NOT_ALLOWED", "ROLL_ALREADY_HAS_RFID":
				docJSON(w, 409, map[string]string{"error": e.Error()})
			case "SOURCE_NOT_FOUND":
				docJSON(w, 400, map[string]string{"error": e.Error()})
			default:
				http.Error(w, "Unable to save roll observation", 503)
			}
			return
		}
		docJSON(w, 201, map[string]any{"passport": record, "meaning": "Recorded physical observation only; not an inventory balance, material release or authenticated inspection"})
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", 405)
	}
}

func (s *EdgeServer) saveRollPassport(ctx context.Context, tenant string, v rollPassportInput, declared, measured int64) (rollPassport, error) {
	var p rollPassport
	tx, e := s.docTransaction(ctx, tenant)
	if e != nil {
		return p, e
	}
	defer tx.Rollback()
	// Serialize revisions per roll; RFID unique index prevents cross-roll reuse.
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1),hashtext($2))`, tenant, v.RollID); e != nil {
		return p, e
	}
	revision := 0
	var oldEPC sql.NullString
	e = tx.QueryRowContext(ctx, `SELECT revision,epc_hex FROM store_roll_passport_events_v7 WHERE tenant_id=$1 AND roll_id=$2 ORDER BY revision DESC LIMIT 1`, tenant, v.RollID).Scan(&revision, &oldEPC)
	if e != nil && e != sql.ErrNoRows {
		return p, e
	}
	if v.ExpectedRevision != revision {
		return p, errors.New("REVISION_CONFLICT")
	}
	if oldEPC.Valid && oldEPC.String != "" && v.EPC != oldEPC.String {
		return p, errors.New("RFID_CHANGE_NOT_ALLOWED")
	}
	if v.SourceDocumentID != "" {
		var exists bool
		if e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM document_sources_v65 WHERE tenant_id=$1 AND id=$2)`, tenant, v.SourceDocumentID).Scan(&exists); e != nil {
			return p, e
		}
		if !exists {
			return p, errors.New("SOURCE_NOT_FOUND")
		}
	}
	if v.EPC != "" {
		// Once a roll receives an EPC the binding remains permanent; never reuse or silently reassign it.
		var existing string
		e = tx.QueryRowContext(ctx, `SELECT epc_hex FROM store_roll_rfid_bindings_v7 WHERE tenant_id=$1 AND roll_id=$2`, tenant, v.RollID).Scan(&existing)
		if e != nil && e != sql.ErrNoRows {
			return p, e
		}
		if e == nil && existing != v.EPC {
			return p, errors.New("ROLL_ALREADY_HAS_RFID")
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO store_roll_rfid_bindings_v7(tenant_id,epc_hex,roll_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, tenant, v.EPC, v.RollID)
		if e != nil {
			return p, e
		}
		var owner string
		if e = tx.QueryRowContext(ctx, `SELECT roll_id FROM store_roll_rfid_bindings_v7 WHERE tenant_id=$1 AND epc_hex=$2`, tenant, v.EPC).Scan(&owner); e != nil {
			if e == sql.ErrNoRows {
				return p, errors.New("ROLL_ALREADY_HAS_RFID")
			}
			return p, e
		}
		if owner != v.RollID {
			return p, errors.New("RFID_ALREADY_BOUND")
		}
	}
	action := "RECEIPT"
	if revision > 0 {
		action = "REINSPECT"
	}
	if v.InspectionResult == "HOLD" {
		action = "HOLD"
	}
	e = tx.QueryRowContext(ctx, `INSERT INTO store_roll_passport_events_v7(tenant_id,roll_id,revision,event_kind,epc_hex,material_sku,supplier_lot,shade_band,bin_location,declared_length_mm,measured_length_mm,usable_width_mm,inspection_result,defects,source_document_id,source_note,recorded_by)
 VALUES($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,$9,$10,$11,$12,$13,$14,NULLIF($15,''),$16,'dev') RETURNING recorded_at`, tenant, v.RollID, revision+1, action, v.EPC, v.MaterialSKU, v.SupplierLot, v.ShadeBand, v.BinLocation, declared, measured, v.UsableWidthMM, v.InspectionResult, v.Defects, v.SourceDocumentID, v.SourceNote).Scan(&p.RecordedAt)
	if e != nil {
		return p, e
	}
	if e = tx.Commit(); e != nil {
		return p, e
	}
	p = rollPassport{RollID: v.RollID, Revision: revision + 1, EventKind: action, EPC: v.EPC, MaterialSKU: v.MaterialSKU, SupplierLot: v.SupplierLot, ShadeBand: v.ShadeBand, BinLocation: v.BinLocation, DeclaredLengthM: passportMeters(declared), MeasuredLengthM: passportMeters(measured), LengthVarianceM: passportVariance(measured - declared), UsableWidthMM: v.UsableWidthMM, InspectionResult: v.InspectionResult, Defects: v.Defects, SourceDocumentID: v.SourceDocumentID, SourceNote: v.SourceNote, RecordedAt: p.RecordedAt, EvidenceClass: "OPERATOR_REPORTED", LedgerStatus: "NOT_RECONCILED"}
	return p, nil
}

func (s *EdgeServer) loadRollPassports(ctx context.Context, tenant string) ([]rollPassport, error) {
	tx, e := s.docTransaction(ctx, tenant)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	// On reading authoritative ledger, separate it from operator observations;
	// even matching numbers are not evidence of physical material approval.
	rows, e := tx.QueryContext(ctx, `SELECT p.roll_id,p.revision,p.event_kind,COALESCE(p.epc_hex,''),p.material_sku,p.supplier_lot,p.shade_band,p.bin_location,p.declared_length_mm,p.measured_length_mm,p.usable_width_mm,p.inspection_result,p.defects,COALESCE(p.source_document_id,''),p.source_note,p.recorded_at,
 CASE WHEN sr.id IS NULL THEN 'NOT_IN_STOCK_LEDGER' ELSE sr.inspection_status||' / '||sr.status END,
 COALESCE(ROUND(sr.remaining_length_m*1000)::bigint,0),
 COALESCE((SELECT SUM(m.reserved_mm-m.issued_mm-m.released_mm) FROM material_reservations_v5 m WHERE m.tenant_id=p.tenant_id AND m.roll_id=p.roll_id),0)
 FROM store_roll_passport_events_v7 p
 JOIN (SELECT tenant_id,roll_id,MAX(revision) AS revision FROM store_roll_passport_events_v7 WHERE tenant_id=$1 GROUP BY tenant_id,roll_id) last USING(tenant_id,roll_id,revision)
 LEFT JOIN store_rolls sr ON sr.tenant_id=p.tenant_id AND sr.id=p.roll_id
 WHERE p.tenant_id=$1 ORDER BY p.recorded_at DESC,p.roll_id LIMIT 500`, tenant)
	if e != nil {
		return nil, e
	}
	out := make([]rollPassport, 0)
	for rows.Next() {
		var p rollPassport
		var declared, measured, remaining, open int64
		if e = rows.Scan(&p.RollID, &p.Revision, &p.EventKind, &p.EPC, &p.MaterialSKU, &p.SupplierLot, &p.ShadeBand, &p.BinLocation, &declared, &measured, &p.UsableWidthMM, &p.InspectionResult, &p.Defects, &p.SourceDocumentID, &p.SourceNote, &p.RecordedAt, &p.LedgerStatus, &remaining, &open); e != nil {
			rows.Close()
			return nil, e
		}
		p.DeclaredLengthM = passportMeters(declared)
		p.MeasuredLengthM = passportMeters(measured)
		p.LengthVarianceM = passportVariance(measured - declared)
		p.EvidenceClass = "OPERATOR_REPORTED"
		if p.LedgerStatus != "NOT_IN_STOCK_LEDGER" {
			p.LedgerRemainingM = passportMeters(remaining)
			p.LedgerOpenReservationM = passportMeters(open)
		}
		out = append(out, p)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return out, nil
}

// A small read-only status endpoint for the existing authenticated material API.
func (s *EdgeServer) handleStoresAuthority(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", 405)
		return
	}
	tenant, e := intakeTenant()
	if e != nil || s.db == nil {
		http.Error(w, "Unavailable", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	tx, e := s.docTransaction(ctx, tenant)
	if e != nil {
		http.Error(w, "Unavailable", 503)
		return
	}
	defer tx.Rollback()
	var rollCount, passing, openReservations int64
	e = tx.QueryRowContext(ctx, `SELECT COUNT(*),COUNT(*) FILTER (WHERE inspection_status='PASS') FROM store_rolls WHERE tenant_id=$1`, tenant).Scan(&rollCount, &passing)
	if e == nil {
		e = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM material_reservations_v5 WHERE tenant_id=$1 AND reserved_mm>issued_mm+released_mm`, tenant).Scan(&openReservations)
	}
	if e != nil {
		http.Error(w, "Ledger unavailable", 503)
		return
	}
	if e = tx.Commit(); e != nil {
		http.Error(w, "Ledger unavailable", 503)
		return
	}
	docJSON(w, 200, map[string]any{"ledger_rolls": rollCount, "inspected_pass_in_ledger": passing, "open_reservations": openReservations, "warning": "Roll passports are operator observations; no stock balance or reservation is changed by this screen", "secure_material_api": "Separate signed identity, lease, two-witness issuance and CEO approval required"})
}

// Read every append-only observation of one roll, newest first. This history
// cannot be altered by the runtime DB role and does not contain material issue events.
func (s *EdgeServer) loadPassportHistory(ctx context.Context, tenant, rollID string) ([]rollPassport, error) {
	tx, e := s.docTransaction(ctx, tenant)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, `SELECT revision,event_kind,COALESCE(epc_hex,''),material_sku,supplier_lot,shade_band,bin_location,declared_length_mm,measured_length_mm,usable_width_mm,inspection_result,defects,COALESCE(source_document_id,''),source_note,recorded_at
 FROM store_roll_passport_events_v7 WHERE tenant_id=$1 AND roll_id=$2 ORDER BY revision DESC LIMIT 500`, tenant, rollID)
	if e != nil {
		return nil, e
	}
	out := make([]rollPassport, 0)
	for rows.Next() {
		var p rollPassport
		var declared, measured int64
		p.RollID = rollID
		if e = rows.Scan(&p.Revision, &p.EventKind, &p.EPC, &p.MaterialSKU, &p.SupplierLot, &p.ShadeBand, &p.BinLocation, &declared, &measured, &p.UsableWidthMM, &p.InspectionResult, &p.Defects, &p.SourceDocumentID, &p.SourceNote, &p.RecordedAt); e != nil {
			rows.Close()
			return nil, e
		}
		p.DeclaredLengthM = passportMeters(declared)
		p.MeasuredLengthM = passportMeters(measured)
		p.LengthVarianceM = passportVariance(measured - declared)
		p.EvidenceClass = "OPERATOR_REPORTED"
		p.LedgerStatus = "NOT_EVALUATED_IN_HISTORY"
		out = append(out, p)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return out, nil
}
