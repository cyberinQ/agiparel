// Author: Sunil Jain, 2026-10-28 — Accept workbook evidence without weakening buyer-order validation.
// Agiparel Batch 05 — deterministic, source-backed PO/BOM intake.
// A staged submission is a proposed document, NOT an approved production order.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const intakeMaxFile = 2 << 20
const intakeMaxRows = 2000

var intakeCode = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_./-]*$`)

type intakeSKU struct {
	Color    string `json:"color"`
	Size     string `json:"size"`
	UPC      string `json:"upc"`
	Quantity int64  `json:"quantity"`
}
type intakeMaterial struct {
	SKU        string `json:"material_sku"`
	Category   string `json:"category"`
	PerGarment string `json:"per_garment"`
	UOM        string `json:"uom"`
	Required   string `json:"required"`
}
type intakeReport struct {
	ID           string           `json:"id,omitempty"`
	PONumber     string           `json:"po_number"`
	StyleCode    string           `json:"style_code"`
	OrderedQty   int64            `json:"ordered_qty"`
	Status       string           `json:"status"`
	Readiness    string           `json:"readiness"`
	Origin       string           `json:"origin"`
	SourceSHA256 string           `json:"source_sha256"`
	POFile       string           `json:"po_filename"`
	BOMFile      string           `json:"bom_filename"`
	CapturedAt   *time.Time       `json:"captured_at,omitempty"`
	SKUs         []intakeSKU      `json:"skus"`
	Materials    []intakeMaterial `json:"materials"`
	Holds        []string         `json:"holds"`
}

type sourceSubmission struct {
	Report  intakeReport
	PO, BOM []byte
}

func checkCode(label, value string, max int) error {
	if value == "" || len(value) > max || !intakeCode.MatchString(value) {
		return fmt.Errorf("%s: use 1–%d alphanumeric/_./- characters", label, max)
	}
	return nil
}
func parseCSV(raw []byte, header []string) ([][]string, error) {
	if len(raw) == 0 || len(raw) > intakeMaxFile || bytes.IndexByte(raw, 0) >= 0 {
		return nil, errors.New("CSV must be nonempty UTF-8 text under 2 MiB")
	}
	if !utf8Valid(raw) {
		return nil, errors.New("CSV contains invalid UTF-8")
	}
	rd := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})))
	rd.FieldsPerRecord = len(header)
	rd.TrimLeadingSpace = true
	rd.ReuseRecord = false
	h, err := rd.Read()
	if err != nil {
		return nil, fmt.Errorf("CSV header: %w", err)
	}
	for i, v := range header {
		if strings.TrimSpace(h[i]) != v {
			return nil, fmt.Errorf("CSV header column %d must be %q", i+1, v)
		}
	}
	rows := make([][]string, 0)
	for {
		r, e := rd.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, fmt.Errorf("CSV row %d: %w", len(rows)+2, e)
		}
		if len(rows) >= intakeMaxRows {
			return nil, errors.New("CSV exceeds 2000 data rows")
		}
		for i := range r {
			r[i] = strings.TrimSpace(r[i])
		}
		rows = append(rows, r)
	}
	if len(rows) == 0 {
		return nil, errors.New("CSV has no data rows")
	}
	return rows, nil
}
func utf8Valid(b []byte) bool { return utf8.Valid(b) }

func parseIntake(poRaw, bomRaw []byte, poName, bomName string) (sourceSubmission, error) {
	poRows, err := parseCSV(poRaw, []string{"po_number", "style_code", "color_code", "size_code", "ordered_qty", "upc"})
	if err != nil {
		return sourceSubmission{}, fmt.Errorf("buyer PO: %w", err)
	}
	bomRows, err := parseCSV(bomRaw, []string{"style_code", "material_sku", "category", "qty_per_garment", "uom"})
	if err != nil {
		return sourceSubmission{}, fmt.Errorf("BOM: %w", err)
	}
	r := intakeReport{Status: "PROPOSED", Readiness: "HOLD", Origin: "SOURCE_CSV", POFile: poName, BOMFile: bomName, SKUs: []intakeSKU{}, Materials: []intakeMaterial{}, Holds: []string{"Buyer/source identity and revisions require authorized review", "Tech pack, approved sample, OB, label/pack and shipping obligations require verification", "Material eligibility, inspected stock, capacity and production release are not authorized"}}
	seenSku := map[string]bool{}
	for i, row := range poRows {
		if err = checkCode("PO number", row[0], 64); err != nil {
			return sourceSubmission{}, fmt.Errorf("PO row %d: %w", i+2, err)
		}
		if err = checkCode("style code", row[1], 64); err != nil {
			return sourceSubmission{}, fmt.Errorf("PO row %d: %w", i+2, err)
		}
		if err = checkCode("color", row[2], 64); err != nil {
			return sourceSubmission{}, fmt.Errorf("PO row %d: %w", i+2, err)
		}
		if err = checkCode("size", row[3], 32); err != nil {
			return sourceSubmission{}, fmt.Errorf("PO row %d: %w", i+2, err)
		}
		if row[5] != "" {
			if err = checkCode("UPC", row[5], 64); err != nil {
				return sourceSubmission{}, fmt.Errorf("PO row %d: %w", i+2, err)
			}
		}
		q, e := strconv.ParseInt(row[4], 10, 64)
		if e != nil || q <= 0 || q > 100000000 {
			return sourceSubmission{}, fmt.Errorf("PO row %d: quantity must be 1..100000000", i+2)
		}
		if i == 0 {
			r.PONumber = row[0]
			r.StyleCode = row[1]
		} else if r.PONumber != row[0] || r.StyleCode != row[1] {
			return sourceSubmission{}, errors.New("one intake must contain exactly one buyer PO and one style")
		}
		skuKey := row[2] + "\x00" + row[3]
		if seenSku[skuKey] {
			return sourceSubmission{}, fmt.Errorf("PO row %d: duplicate color/size", i+2)
		}
		seenSku[skuKey] = true
		if r.OrderedQty > 100000000-q {
			return sourceSubmission{}, errors.New("total ordered quantity exceeds 100000000")
		}
		r.OrderedQty += q
		r.SKUs = append(r.SKUs, intakeSKU{row[2], row[3], row[5], q})
	}
	seenBOM := map[string]bool{}
	fabric := false
	for i, row := range bomRows {
		if row[0] != r.StyleCode {
			return sourceSubmission{}, fmt.Errorf("BOM row %d: style does not match buyer PO", i+2)
		}
		if err = checkCode("material SKU", row[1], 128); err != nil {
			return sourceSubmission{}, fmt.Errorf("BOM row %d: %w", i+2, err)
		}
		cat := strings.ToUpper(row[2])
		if cat != "FABRIC" && cat != "TRIM" {
			return sourceSubmission{}, fmt.Errorf("BOM row %d: category must be FABRIC or TRIM", i+2)
		}
		if cat == "FABRIC" {
			fabric = true
		}
		// Normalize heterogeneous labels once for every CSV, document, and LLM path.
		// Preserve the original CSV as immutable evidence; do not invent conversions.
		unit, unitErr := canonicalBuyerUOM(row[4])
		if unitErr != nil {
			return sourceSubmission{}, fmt.Errorf("BOM row %d: %w", i+2, unitErr)
		}
		qty, e := strconv.ParseFloat(row[3], 64)
		if e != nil || math.IsNaN(qty) || math.IsInf(qty, 0) || qty <= 0 || qty > 1000000 {
			return sourceSubmission{}, fmt.Errorf("BOM row %d: invalid positive consumption", i+2)
		}
		// Never silently round buyer-approved material quantities to three decimals.
		milli := math.Round(qty * 1000)
		if math.Abs(qty*1000-milli) > 0.000001 || milli < 1 {
			return sourceSubmission{}, fmt.Errorf("BOM row %d: maximum 3 decimal places, minimum 0.001", i+2)
		}
		key := row[1] + "\x00" + cat + "\x00" + unit
		if seenBOM[key] {
			return sourceSubmission{}, fmt.Errorf("BOM row %d: duplicate material/category/UOM", i+2)
		}
		seenBOM[key] = true
		requiredMilli := int64(milli) * r.OrderedQty
		r.Materials = append(r.Materials, intakeMaterial{row[1], cat, fmt.Sprintf("%.3f", float64(milli)/1000), unit, fmt.Sprintf("%d.%03d", requiredMilli/1000, requiredMilli%1000)})
	}
	if !fabric {
		return sourceSubmission{}, errors.New("BOM must contain at least one FABRIC requirement")
	}
	// Domain-separated digest of original unmodified inputs. File order is significant.
	d := sha256.New()
	_, _ = d.Write([]byte("AGIPAREL-ORDER-INTAKE-V5\x00"))
	for _, v := range [][]byte{poRaw, bomRaw} {
		_, _ = d.Write([]byte(strconv.Itoa(len(v))))
		_, _ = d.Write([]byte{0})
		_, _ = d.Write(v)
	}
	r.SourceSHA256 = hex.EncodeToString(d.Sum(nil))
	return sourceSubmission{r, poRaw, bomRaw}, nil
}

func receiveIntake(w http.ResponseWriter, r *http.Request) (sourceSubmission, error) {
	r.Body = http.MaxBytesReader(w, r.Body, 18<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		return sourceSubmission{}, fmt.Errorf("invalid multipart form: %w", err)
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	if len(r.MultipartForm.File["po_file"]) != 1 || len(r.MultipartForm.File["bom_file"]) != 1 || len(r.MultipartForm.File) != 2 {
		return sourceSubmission{}, errors.New("upload exactly one po_file and one bom_file")
	}
	poHeaders := []string{"po_number", "style_code", "color_code", "size_code", "ordered_qty", "upc"}
	bomHeaders := []string{"style_code", "material_sku", "category", "qty_per_garment", "uom"}
	po, e := readSourceTable(r.MultipartForm.File["po_file"][0], poHeaders, false)
	if e != nil {
		return sourceSubmission{}, e
	}
	bom, e := readSourceTable(r.MultipartForm.File["bom_file"][0], bomHeaders, false)
	if e != nil {
		return sourceSubmission{}, e
	}
	sub, e := parseIntake(po.CSV, bom.CSV, po.Name, bom.Name)
	if e != nil {
		return sourceSubmission{}, e
	}
	// Source identity and bytea evidence always reflect untouched uploaded files.
	if po.Spreadsheet || bom.Spreadsheet {
		d := sha256.New()
		_, _ = d.Write([]byte("AGIPAREL-ORDER-INTAKE-V5\x00"))
		for _, v := range [][]byte{po.Original, bom.Original} {
			_, _ = d.Write([]byte(strconv.Itoa(len(v))))
			_, _ = d.Write([]byte{0})
			_, _ = d.Write(v)
		}
		sub.Report.SourceSHA256 = hex.EncodeToString(d.Sum(nil))
	}
	sub.PO, sub.BOM = po.Original, bom.Original
	return sub, nil
}

func writeIntakeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func newOrderIntakeID() (string, error) {
	buf := make([]byte, 16)
	if _, e := rand.Read(buf); e != nil {
		return "", e
	}
	return "OI-" + hex.EncodeToString(buf), nil
}

func intakeTenant() (string, error) {
	if os.Getenv("ENV") != "development" {
		return "", errors.New("requires production tenant identity provider")
	}
	tenant := strings.TrimSpace(os.Getenv("AGIPAREL_OPERATOR_TENANT"))
	if err := checkCode("AGIPAREL_OPERATOR_TENANT", tenant, 64); err != nil {
		return "", err
	}
	return tenant, nil
}
func (s *EdgeServer) handleOrderIntake(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", 405)
		return
	}
	if s.db == nil {
		http.Error(w, "PostgreSQL not configured", 503)
		return
	}
	tenant, e := intakeTenant()
	if e != nil {
		http.Error(w, "Operator tenant is not configured", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	if r.Method == http.MethodGet {
		s.readOrderIntakes(ctx, w, r, tenant)
		return
	}
	if r.URL.Query().Get("action") != "preview" && r.URL.Query().Get("action") != "stage" {
		http.Error(w, "action must be preview or stage", 400)
		return
	}
	sub, e := receiveIntake(w, r)
	if e != nil {
		http.Error(w, e.Error(), 400)
		return
	}
	if r.URL.Query().Get("action") == "preview" {
		writeIntakeJSON(w, 200, sub.Report)
		return
	}
	rep, reused, e := s.persistIntake(ctx, tenant, sub)
	if e != nil {
		http.Error(w, "Intake not saved; check database configuration", 503)
		return
	}
	writeIntakeJSON(w, map[bool]int{true: 200, false: 201}[reused], rep)
}
func (s *EdgeServer) persistIntake(ctx context.Context, tenant string, sub sourceSubmission) (intakeReport, bool, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return intakeReport{}, false, err
	}
	defer tx.Rollback()
	report, reused, err := persistIntakeInTx(ctx, tx, tenant, sub)
	if err != nil {
		return intakeReport{}, false, err
	}
	if err = tx.Commit(); err != nil {
		return intakeReport{}, false, err
	}
	return report, reused, nil
}

func persistIntakeInTx(ctx context.Context, tx *sql.Tx, tenant string, sub sourceSubmission) (intakeReport, bool, error) {
	var e error
	if _, e = tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id',$1,true)`, tenant); e != nil {
		return intakeReport{}, false, e
	}
	id, e := newOrderIntakeID()
	if e != nil {
		return intakeReport{}, false, e
	}
	res, e := tx.ExecContext(ctx, `INSERT INTO order_intakes_v5(id,tenant_id,po_number,style_code,ordered_qty,source_sha256,po_filename,bom_filename,po_original,bom_original) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT (tenant_id,source_sha256) DO NOTHING`, id, tenant, sub.Report.PONumber, sub.Report.StyleCode, sub.Report.OrderedQty, sub.Report.SourceSHA256, sub.Report.POFile, sub.Report.BOMFile, sub.PO, sub.BOM)
	if e != nil {
		return intakeReport{}, false, e
	}
	n, e := res.RowsAffected()
	if e != nil {
		return intakeReport{}, false, e
	}
	if n == 0 {
		if e = tx.QueryRowContext(ctx, `SELECT id,captured_at FROM order_intakes_v5 WHERE tenant_id=$1 AND source_sha256=$2`, tenant, sub.Report.SourceSHA256).Scan(&sub.Report.ID, &sub.Report.CapturedAt); e != nil {
			return intakeReport{}, false, e
		}
		return sub.Report, true, nil
	}
	for i, line := range sub.Report.SKUs {
		if _, e = tx.ExecContext(ctx, `INSERT INTO order_intake_skus_v5(tenant_id,intake_id,line_no,color_code,size_code,upc,ordered_qty) VALUES($1,$2,$3,$4,$5,$6,$7)`, tenant, id, i+1, line.Color, line.Size, line.UPC, line.Quantity); e != nil {
			return intakeReport{}, false, e
		}
	}
	for i, line := range sub.Report.Materials {
		if _, e = tx.ExecContext(ctx, `INSERT INTO order_intake_bom_v5(tenant_id,intake_id,line_no,material_sku,category,qty_per_garment,uom) VALUES($1,$2,$3,$4,$5,$6,$7)`, tenant, id, i+1, line.SKU, line.Category, line.PerGarment, line.UOM); e != nil {
			return intakeReport{}, false, e
		}
	}
	now := time.Now().UTC()
	sub.Report.ID = id
	sub.Report.CapturedAt = &now
	return sub.Report, false, nil
}

type intakeListRow struct {
	ID         string    `json:"id"`
	PONumber   string    `json:"po_number"`
	StyleCode  string    `json:"style_code"`
	OrderedQty int64     `json:"ordered_qty"`
	Status     string    `json:"status"`
	CapturedAt time.Time `json:"captured_at"`
}

func (s *EdgeServer) readOrderIntakes(ctx context.Context, w http.ResponseWriter, r *http.Request, tenant string) {
	tx, e := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id',$1,true)`, tenant); e != nil {
		http.Error(w, "Tenant context unavailable", 503)
		return
	}
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id != "" {
		if !strings.HasPrefix(id, "OI-") || len(id) != 35 {
			http.Error(w, "Invalid intake ID", 400)
			return
		}
		var pof, bomf, digest string
		var captured time.Time
		var rep intakeReport
		e = tx.QueryRowContext(ctx, `SELECT id,po_number,style_code,ordered_qty,status,source_sha256,po_filename,bom_filename,captured_at FROM order_intakes_v5 WHERE tenant_id=$1 AND id=$2`, tenant, id).Scan(&rep.ID, &rep.PONumber, &rep.StyleCode, &rep.OrderedQty, &rep.Status, &digest, &pof, &bomf, &captured)
		if e == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		if e != nil {
			http.Error(w, "Query failed", 503)
			return
		}
		rep.Readiness = "HOLD"
		rep.Origin = "SOURCE_CSV"
		var linked bool
		if e = tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM buyer_order_handoffs_v65c1 WHERE tenant_id=$1 AND intake_id=$2)`, tenant, id).Scan(&linked); e != nil {
			http.Error(w, "Source linkage query failed", 503)
			return
		}
		if linked {
			rep.Origin = "REVIEWED_DOCUMENT_MAPPING"
		}
		rep.SourceSHA256 = digest
		rep.POFile = pof
		rep.BOMFile = bomf
		rep.CapturedAt = &captured
		rep.Holds = []string{"Source data is proposed; buyer revisions and owner approvals are not verified", "Tech pack, OB, packing and shipment obligations remain unverified", "Material availability and release gates remain outstanding"}
		rep.SKUs = []intakeSKU{}
		rep.Materials = []intakeMaterial{}
		rows, qe := tx.QueryContext(ctx, `SELECT color_code,size_code,upc,ordered_qty FROM order_intake_skus_v5 WHERE tenant_id=$1 AND intake_id=$2 ORDER BY line_no`, tenant, id)
		if qe != nil {
			http.Error(w, "SKU query failed", 503)
			return
		}
		for rows.Next() {
			var s intakeSKU
			if e = rows.Scan(&s.Color, &s.Size, &s.UPC, &s.Quantity); e != nil {
				break
			}
			rep.SKUs = append(rep.SKUs, s)
		}
		if e == nil {
			e = rows.Err()
		}
		rows.Close()
		if e != nil {
			http.Error(w, "SKU rows failed", 503)
			return
		}
		rows, qe = tx.QueryContext(ctx, `SELECT material_sku,category,qty_per_garment::text,uom,(qty_per_garment*$3)::numeric(24,3)::text FROM order_intake_bom_v5 WHERE tenant_id=$1 AND intake_id=$2 ORDER BY line_no`, tenant, id, rep.OrderedQty)
		if qe != nil {
			http.Error(w, "BOM query failed", 503)
			return
		}
		for rows.Next() {
			var m intakeMaterial
			if e = rows.Scan(&m.SKU, &m.Category, &m.PerGarment, &m.UOM, &m.Required); e != nil {
				break
			}
			rep.Materials = append(rep.Materials, m)
		}
		if e == nil {
			e = rows.Err()
		}
		rows.Close()
		if e != nil {
			http.Error(w, "BOM rows failed", 503)
			return
		}
		if e = tx.Commit(); e != nil {
			http.Error(w, "Database unavailable", 503)
			return
		}
		writeIntakeJSON(w, 200, rep)
		return
	}
	rows, e := tx.QueryContext(ctx, `SELECT id,po_number,style_code,ordered_qty,status,captured_at FROM order_intakes_v5 WHERE tenant_id=$1 ORDER BY captured_at DESC LIMIT 30`, tenant)
	if e != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	list := []intakeListRow{}
	for rows.Next() {
		var item intakeListRow
		if e = rows.Scan(&item.ID, &item.PONumber, &item.StyleCode, &item.OrderedQty, &item.Status, &item.CapturedAt); e != nil {
			break
		}
		list = append(list, item)
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	if e = tx.Commit(); e != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	writeIntakeJSON(w, 200, map[string]any{"items": list, "count": len(list), "status": "PROPOSED_ONLY"})
}
