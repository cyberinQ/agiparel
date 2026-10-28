// Author: Sunil Jain, 2026-10-28 — Read checked Excel evidence and retain originals for sourcing drafts.
// Batch 06 — deterministic sourcing what-if scenarios, not authoritative stock/quotes.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Quantity = 3 decimal places; price = 4. Money extensions use arbitrary
// precision intermediate arithmetic and round HALF UP to 4 fractional places.
var scenarioDecimal = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]+)?$`)

func scenarioScaled(s string, places int, maxWhole int64, allowZero bool) (int64, error) {
	if !scenarioDecimal.MatchString(s) {
		return 0, errors.New("must be a positive plain decimal (no exponent/sign)")
	}
	parts := strings.SplitN(s, ".", 2)
	if len(parts) == 2 && len(parts[1]) > places {
		return 0, fmt.Errorf("maximum %d decimal places", places)
	}
	whole, e := strconv.ParseInt(parts[0], 10, 64)
	if e != nil || whole > maxWhole {
		return 0, errors.New("amount exceeds permitted bound")
	}
	scale := int64(1)
	for i := 0; i < places; i++ {
		scale *= 10
	}
	frac := int64(0)
	if len(parts) == 2 {
		padded := parts[1] + strings.Repeat("0", places-len(parts[1]))
		frac, e = strconv.ParseInt(padded, 10, 64)
		if e != nil {
			return 0, e
		}
	}
	value := whole*scale + frac
	if !allowZero && value == 0 {
		return 0, errors.New("must be greater than zero")
	}
	return value, nil
}
func scenarioFmt(v int64) string { return fmt.Sprintf("%d.%03d", v/1000, v%1000) }

// (quantity milli × unit price ten-thousandths) / 1000 = amount ten-thousandths.
func scenarioMoney(qtyMilli, priceTenThousandths int64) string {
	n := new(big.Int).Mul(big.NewInt(qtyMilli), big.NewInt(priceTenThousandths))
	n.Add(n, big.NewInt(500))
	n.Div(n, big.NewInt(1000))
	return fmt.Sprintf("%s.%04d", new(big.Int).Div(new(big.Int).Set(n), big.NewInt(10000)).String(), new(big.Int).Mod(n, big.NewInt(10000)).Int64())
}
func scenarioMoneySum(pairs [][2]int64) string {
	total := new(big.Int)
	for _, p := range pairs {
		total.Add(total, new(big.Int).Mul(big.NewInt(p[0]), big.NewInt(p[1])))
	}
	total.Add(total, big.NewInt(500))
	total.Div(total, big.NewInt(1000))
	whole, frac := new(big.Int).QuoRem(total, big.NewInt(10000), new(big.Int))
	return fmt.Sprintf("%s.%04d", whole.String(), frac.Int64())
}

type scenarioCandidate struct {
	LotID           string          `json:"lot_id"`
	MaterialSKU     string          `json:"material_sku"`
	SourceReference string          `json:"source_reference"`
	SubmittedQty    string          `json:"submitted_qty"`
	SuggestedQty    string          `json:"suggested_qty"`
	Verdict         string          `json:"verdict"`
	Checks          []scenarioCheck `json:"checks"`
}
type scenarioCheck struct {
	Criterion string `json:"criterion"`
	Pass      bool   `json:"pass"`
	Detail    string `json:"detail"`
}
type scenarioMaterial struct {
	MaterialSKU        string `json:"material_sku"`
	Category           string `json:"category"`
	UOM                string `json:"uom"`
	Gross              string `json:"gross"`
	ConditionalSurplus string `json:"conditional_surplus"`
	NetIndent          string `json:"net_indent"`
	BuyerSupplyGap     string `json:"buyer_supply_gap"`
	ProcurementOwner   string `json:"procurement_owner"`
	UnitPrice          string `json:"unit_price"`
	PriceSource        string `json:"price_source"`
	GrossCost          string `json:"gross_cost"`
	ReuseValue         string `json:"reuse_value"`
	NetCost            string `json:"net_cost"`
}
type scenarioReport struct {
	ID                    string              `json:"id,omitempty"`
	IntakeID              string              `json:"intake_id"`
	PONumber              string              `json:"po_number"`
	StyleCode             string              `json:"style_code"`
	OrderedQty            int64               `json:"ordered_qty"`
	IntakeSourceSHA256    string              `json:"intake_source_sha256"`
	ScenarioSHA256        string              `json:"scenario_sha256"`
	StockFile             string              `json:"stock_filename"`
	PricesFile            string              `json:"prices_filename"`
	Currency              string              `json:"currency"`
	Status                string              `json:"status"`
	ReleaseGate           string              `json:"release_gate"`
	EvidenceClass         string              `json:"evidence_class"`
	GrossMaterialCost     string              `json:"gross_material_cost"`
	ConditionalReuseValue string              `json:"conditional_reuse_value"`
	NetPurchaseExposure   string              `json:"net_purchase_exposure"`
	Materials             []scenarioMaterial  `json:"materials"`
	Candidates            []scenarioCandidate `json:"candidates"`
	Holds                 []string            `json:"holds"`
	CreatedAt             *time.Time          `json:"created_at,omitempty"`
}
type scenarioSources struct {
	Stock, Prices []byte
	Report        scenarioReport
}

var stockColumns = []string{"lot_id", "material_sku", "uom", "available_qty", "fabric_spec", "required_spec", "gsm", "required_gsm", "gsm_tolerance_pct", "shade_delta_e", "shade_max_delta_e", "usable_width_mm", "marker_width_mm", "inspection_status", "age_days", "max_age_days", "buyer_escrow", "held_for_po", "source_reference"}
var priceColumns = []string{"material_sku", "category", "uom", "unit_price", "currency", "procurement_owner", "source_reference"}

func scenarioDigest(intakeID, sourceHash string, stock, prices []byte) string {
	h := sha256.New()
	h.Write([]byte("AGIPAREL-SOURCING-SCENARIO-V6\x00"))
	for _, v := range [][]byte{[]byte(intakeID), []byte(sourceHash), stock, prices} {
		h.Write([]byte(strconv.Itoa(len(v))))
		h.Write([]byte{0})
		h.Write(v)
	}
	return hex.EncodeToString(h.Sum(nil))
}
func scenarioKey(sku, cat, uom string) string { return sku + "\x00" + cat + "\x00" + uom }

// NOTE: Six-dimensional evaluation is CONDITIONAL upon operator-provided CSV
// evidence; it is not inspection, owner clearance or live stock availability.
func buildScenario(intake intakeReport, stock, prices []byte, stockName, pricesName string) (scenarioSources, error) {
	if intake.ID == "" || intake.SourceSHA256 == "" || intake.Status != "PROPOSED" {
		return scenarioSources{}, errors.New("requires a stored, immutable PROPOSED intake")
	}
	var stockRows [][]string
	var e error
	// An explicit header-only stock file is a valid no-surplus scenario.
	if len(stock) > intakeMaxFile || !bytes.Equal(bytes.TrimSpace(bytes.TrimPrefix(stock, []byte{0xEF, 0xBB, 0xBF})), []byte(strings.Join(stockColumns, ","))) {
		stockRows, e = parseCSV(stock, stockColumns)
		if e != nil {
			return scenarioSources{}, fmt.Errorf("stock candidates: %w", e)
		}
	}
	priceRows, e := parseCSV(prices, priceColumns)
	if e != nil {
		return scenarioSources{}, fmt.Errorf("price evidence: %w", e)
	}
	rep := scenarioReport{IntakeID: intake.ID, PONumber: intake.PONumber, StyleCode: intake.StyleCode, OrderedQty: intake.OrderedQty, IntakeSourceSHA256: intake.SourceSHA256, ScenarioSHA256: scenarioDigest(intake.ID, intake.SourceSHA256, stock, prices), StockFile: stockName, PricesFile: pricesName, Status: "PROPOSED", ReleaseGate: "HOLD", EvidenceClass: "OPERATOR_SUBMITTED_CSV_UNVERIFIED", Materials: []scenarioMaterial{}, Candidates: []scenarioCandidate{}, Holds: []string{"These surplus and prices are operator-submitted scenarios, not authenticated stock, GRN, roll inspection, buyer policy or supplier quotes", "Buyer ownership, reservation conflicts and six compatibility conditions must be independently verified against authoritative stores records before any commitment", "Buyer-owned material remains a buyer supply gap, never factory procurement cash; submitted owner labels need independent confirmation", "No indent/PO, fabric reservation, quote, margin approval or production release is issued; recheck race/concurrency and eligibility at actual commit"}}
	type demand struct {
		idx                int
		gross, used, price int64
		owner              string
	}
	demands := map[string]*demand{}
	for i, m := range intake.Materials {
		gross, e := scenarioScaled(m.Required, 3, 100000000000000, false)
		if e != nil {
			return scenarioSources{}, fmt.Errorf("stored BOM required quantity %s: %w", m.SKU, e)
		}
		key := scenarioKey(m.SKU, m.Category, m.UOM)
		if _, ok := demands[key]; ok {
			return scenarioSources{}, errors.New("duplicate stored BOM line")
		}
		demands[key] = &demand{idx: i, gross: gross}
		rep.Materials = append(rep.Materials, scenarioMaterial{MaterialSKU: m.SKU, Category: m.Category, UOM: m.UOM, Gross: scenarioFmt(gross), ConditionalSurplus: "0.000", NetIndent: scenarioFmt(gross)})
	}
	if len(demands) == 0 {
		return scenarioSources{}, errors.New("intake BOM is empty")
	}
	seenPrice := map[string]bool{}
	for i, r := range priceRows {
		key := scenarioKey(r[0], r[1], r[2])
		d, ok := demands[key]
		if !ok {
			return scenarioSources{}, fmt.Errorf("price row %d: material/category/UOM not on intake BOM", i+2)
		}
		if seenPrice[key] {
			return scenarioSources{}, fmt.Errorf("price row %d: duplicate material price", i+2)
		}
		seenPrice[key] = true
		if e = checkCode("price source_reference", r[6], 128); e != nil {
			return scenarioSources{}, fmt.Errorf("price row %d: %w", i+2, e)
		}
		if len(r[4]) != 3 || strings.ToUpper(r[4]) != r[4] || !regexp.MustCompile(`^[A-Z]{3}$`).MatchString(r[4]) {
			return scenarioSources{}, fmt.Errorf("price row %d: use three-letter ISO currency", i+2)
		}
		if rep.Currency != "" && rep.Currency != r[4] {
			return scenarioSources{}, errors.New("mixed currencies require separately approved FX assumptions")
		}
		rep.Currency = r[4]
		val, pe := scenarioScaled(r[3], 4, 1000000, true)
		if pe != nil {
			return scenarioSources{}, fmt.Errorf("price row %d: %w", i+2, pe)
		}
		if r[5] != "FACTORY" && r[5] != "BUYER_OWNED" {
			return scenarioSources{}, fmt.Errorf("price row %d: procurement_owner must be FACTORY or BUYER_OWNED", i+2)
		}
		// An unquoted/zero factory rate is UNKNOWN, not free material.
		// Reject it rather than publish false zero exposure or savings.
		if r[5] == "FACTORY" && val == 0 {
			return scenarioSources{}, fmt.Errorf("price row %d: factory unit_price must be greater than zero; obtain a sourced quote", i+2)
		}
		d.owner = r[5]
		d.price = val
		rep.Materials[d.idx].ProcurementOwner = r[5]
		rep.Materials[d.idx].UnitPrice = fmt.Sprintf("%d.%04d", val/10000, val%10000)
		rep.Materials[d.idx].PriceSource = r[6]
	}
	if len(seenPrice) != len(demands) {
		return scenarioSources{}, errors.New("price sheet must include exactly one rate per BOM material")
	}
	seenLot := map[string]bool{}
	for i, r := range stockRows {
		line := i + 2
		for _, item := range []struct {
			label, value string
			max          int
		}{{"lot_id", r[0], 100}, {"material_sku", r[1], 128}, {"UOM", r[2], 16}, {"source_reference", r[18], 128}} {
			if e = checkCode(item.label, item.value, item.max); e != nil {
				return scenarioSources{}, fmt.Errorf("stock row %d: %w", line, e)
			}
		}
		if seenLot[r[0]] {
			return scenarioSources{}, fmt.Errorf("stock row %d: duplicate lot ID", line)
		}
		seenLot[r[0]] = true
		d, ok := demands[scenarioKey(r[1], "FABRIC", r[2])]
		if !ok {
			return scenarioSources{}, fmt.Errorf("stock row %d: only fabric BOM materials with matching UOM can be candidates", line)
		}
		qty, ee := scenarioScaled(r[3], 3, 1000000000, false)
		if ee != nil {
			return scenarioSources{}, fmt.Errorf("stock row %d quantity: %w", line, ee)
		}
		for _, j := range []int{4, 5, 13, 16} {
			if strings.TrimSpace(r[j]) == "" {
				return scenarioSources{}, fmt.Errorf("stock row %d: missing required evidence field %d", line, j+1)
			}
		}
		// All submitted tolerances are unverified assertions; never default from a factory standard.
		gsm, ee := scenarioScaled(r[6], 3, 1000000, false)
		if ee != nil {
			return scenarioSources{}, fmt.Errorf("stock row %d GSM: %w", line, ee)
		}
		requiredGSM, ee := scenarioScaled(r[7], 3, 1000000, false)
		if ee != nil {
			return scenarioSources{}, fmt.Errorf("stock row %d required GSM: %w", line, ee)
		}
		tolerance, ee := scenarioScaled(r[8], 3, 100, true)
		if ee != nil {
			return scenarioSources{}, fmt.Errorf("stock row %d GSM tolerance: %w", line, ee)
		}
		delta, ee := scenarioScaled(r[9], 3, 1000000, true)
		if ee != nil {
			return scenarioSources{}, fmt.Errorf("stock row %d delta-E: %w", line, ee)
		}
		maxDelta, ee := scenarioScaled(r[10], 3, 1000000, true)
		if ee != nil {
			return scenarioSources{}, fmt.Errorf("stock row %d allowed delta-E: %w", line, ee)
		}
		width, ee := scenarioScaled(r[11], 3, 1000000, false)
		if ee != nil {
			return scenarioSources{}, fmt.Errorf("stock row %d usable width: %w", line, ee)
		}
		marker, ee := scenarioScaled(r[12], 3, 1000000, false)
		if ee != nil {
			return scenarioSources{}, fmt.Errorf("stock row %d marker width: %w", line, ee)
		}
		age, ae := strconv.ParseInt(r[14], 10, 32)
		limit, le := strconv.ParseInt(r[15], 10, 32)
		if ae != nil || le != nil || age < 0 || limit <= 0 {
			return scenarioSources{}, fmt.Errorf("stock row %d invalid age limit", line)
		}
		if r[17] != "" && r[17] != intake.PONumber {
			if e = checkCode("held_for_po", r[17], 64); e != nil {
				return scenarioSources{}, fmt.Errorf("stock row %d: %w", line, e)
			}
		}
		checks := []scenarioCheck{
			{"SPEC_GSM", r[4] == r[5] && new(big.Int).Mul(big.NewInt(abs64(gsm-requiredGSM)), big.NewInt(100000)).Cmp(new(big.Int).Mul(big.NewInt(requiredGSM), big.NewInt(tolerance))) <= 0, "Exact spec plus submitted GSM tolerance; buyer approval not verified"},
			{"SHADE", delta <= maxDelta, "Submitted ΔE versus submitted buyer limit; actual shade approval not verified"},
			{"WIDTH", width >= marker, "Submitted usable roll width versus claimed marker width; CAD not verified"},
			{"INSPECTION", r[13] == "ACCEPTED_PASS", "Submitted inspection status; signed GRN not verified"},
			{"AGE", age <= limit, "Submitted stock age versus claimed policy limit"},
			{"BUYER_ESCROW", r[16] == "ALLOWED" && (r[17] == "" || r[17] == intake.PONumber), "Submitted ownership/escrow clearance and no cross-order hold; neither independently verified"},
		}
		eligible := true
		for _, c := range checks {
			if !c.Pass {
				eligible = false
			}
		}
		applied := int64(0)
		verdict := "INELIGIBLE"
		if eligible {
			remaining := d.gross - d.used
			if remaining > 0 {
				applied = qty
				if applied > remaining {
					applied = remaining
				}
				d.used += applied
				verdict = "CONDITIONALLY_MATCHED"
			} else {
				verdict = "EXCESS_NOT_ALLOCATED"
			}
		}
		rep.Candidates = append(rep.Candidates, scenarioCandidate{LotID: r[0], MaterialSKU: r[1], SourceReference: r[18], SubmittedQty: scenarioFmt(qty), SuggestedQty: scenarioFmt(applied), Verdict: verdict, Checks: checks})
	}
	var grossPairs, usedPairs, netPairs [][2]int64
	for _, m := range rep.Materials {
		key := scenarioKey(m.MaterialSKU, m.Category, m.UOM)
		d := demands[key]
		left := d.gross - d.used
		m.ConditionalSurplus = scenarioFmt(d.used)
		m.BuyerSupplyGap = "0.000"
		if d.owner == "BUYER_OWNED" {
			// Buyer-supplied material is a supply gap, never factory procurement cash.
			m.BuyerSupplyGap = scenarioFmt(left)
			m.NetIndent = "0.000"
			m.GrossCost, m.ReuseValue, m.NetCost = "0.0000", "0.0000", "0.0000"
		} else {
			m.NetIndent = scenarioFmt(left)
			m.GrossCost = scenarioMoney(d.gross, d.price)
			m.ReuseValue = scenarioMoney(d.used, d.price)
			m.NetCost = scenarioMoney(left, d.price)
			grossPairs = append(grossPairs, [2]int64{d.gross, d.price})
			usedPairs = append(usedPairs, [2]int64{d.used, d.price})
			netPairs = append(netPairs, [2]int64{left, d.price})
		}
		rep.Materials[d.idx] = m
	}
	rep.GrossMaterialCost = scenarioMoneySum(grossPairs)
	rep.ConditionalReuseValue = scenarioMoneySum(usedPairs)
	rep.NetPurchaseExposure = scenarioMoneySum(netPairs)
	return scenarioSources{Stock: stock, Prices: prices, Report: rep}, nil
}
func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func scenarioReadFile(r *http.Request, key string) ([]byte, string, error) {
	files := r.MultipartForm.File[key]
	if len(files) != 1 {
		return nil, "", fmt.Errorf("upload exactly one %s", key)
	}
	cols := stockColumns
	allowEmpty := true
	if key == "prices_file" {
		cols = priceColumns
		allowEmpty = false
	}
	src, e := readSourceTable(files[0], cols, allowEmpty)
	if e != nil {
		return nil, "", e
	}
	return src.CSV, src.Name, nil
}
func receiveScenario(w http.ResponseWriter, r *http.Request) (string, sourceTable, sourceTable, error) {
	r.Body = http.MaxBytesReader(w, r.Body, 18<<20)
	if e := r.ParseMultipartForm(8 << 20); e != nil {
		return "", sourceTable{}, sourceTable{}, e
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	if len(r.MultipartForm.File) != 2 || len(r.MultipartForm.File["stock_file"]) != 1 || len(r.MultipartForm.File["prices_file"]) != 1 {
		return "", sourceTable{}, sourceTable{}, errors.New("choose one stock sheet and one rate sheet")
	}
	id := r.FormValue("intake_id")
	if len(id) != 35 || !strings.HasPrefix(id, "OI-") {
		return "", sourceTable{}, sourceTable{}, errors.New("valid staged intake_id required")
	}
	for _, c := range id[3:] {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return "", sourceTable{}, sourceTable{}, errors.New("invalid intake ID")
		}
	}
	stock, e := readSourceTable(r.MultipartForm.File["stock_file"][0], stockColumns, true)
	if e != nil {
		return "", sourceTable{}, sourceTable{}, e
	}
	prices, e := readSourceTable(r.MultipartForm.File["prices_file"][0], priceColumns, false)
	if e != nil {
		return "", sourceTable{}, sourceTable{}, e
	}
	return id, stock, prices, nil
}

func (s *EdgeServer) scenarioIntake(ctx context.Context, tenant, id string) (intakeReport, error) {
	var rep intakeReport
	tx, e := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return rep, e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id',$1,true)`, tenant); e != nil {
		return rep, e
	}
	e = tx.QueryRowContext(ctx, `SELECT id,po_number,style_code,ordered_qty,status,source_sha256 FROM order_intakes_v5 WHERE tenant_id=$1 AND id=$2`, tenant, id).Scan(&rep.ID, &rep.PONumber, &rep.StyleCode, &rep.OrderedQty, &rep.Status, &rep.SourceSHA256)
	if e != nil {
		return rep, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT material_sku,category,qty_per_garment::text,uom,(qty_per_garment*$3)::numeric(24,3)::text FROM order_intake_bom_v5 WHERE tenant_id=$1 AND intake_id=$2 ORDER BY line_no`, tenant, id, rep.OrderedQty)
	if e != nil {
		return rep, e
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
		return rep, e
	}
	if e = tx.Commit(); e != nil {
		return rep, e
	}
	return rep, nil
}
func (s *EdgeServer) handleMaterialScenario(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", 405)
		return
	}
	if s.db == nil {
		http.Error(w, "PostgreSQL unavailable", 503)
		return
	}
	tenant, e := intakeTenant()
	if e != nil {
		http.Error(w, "Development operator tenant unavailable", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if r.Method == http.MethodGet {
		s.readMaterialScenario(ctx, w, r, tenant)
		return
	}
	action := r.URL.Query().Get("action")
	if action != "preview" && action != "stage" {
		http.Error(w, "action must be preview or stage", 400)
		return
	}
	id, stock, prices, e := receiveScenario(w, r)
	if e != nil {
		http.Error(w, e.Error(), 400)
		return
	}
	intake, e := s.scenarioIntake(ctx, tenant, id)
	if e == sql.ErrNoRows {
		http.Error(w, "Staged intake not found for operator tenant", 404)
		return
	}
	if e != nil {
		http.Error(w, "Staged intake could not be read", 503)
		return
	}
	src, e := buildScenario(intake, stock.CSV, prices.CSV, stock.Name, prices.Name)
	if e != nil {
		http.Error(w, e.Error(), 400)
		return
	}
	if stock.Spreadsheet || prices.Spreadsheet {
		src.Report.ScenarioSHA256 = scenarioDigest(id, intake.SourceSHA256, stock.Original, prices.Original)
		src.Report.EvidenceClass = "OPERATOR_SUBMITTED_SPREADSHEET_UNVERIFIED"
	}
	src.Stock, src.Prices = stock.Original, prices.Original
	if action == "stage" && !scenarioPreviewMatches(r.FormValue("preview_sha256"), src.Report.ScenarioSHA256) {
		http.Error(w, "Preview no longer matches the order and uploaded sheets. Check materials again before saving.", http.StatusConflict)
		return
	}
	if action == "preview" {
		writeIntakeJSON(w, 200, src.Report)
		return
	}
	report, reused, e := s.persistMaterialScenario(ctx, tenant, src)
	if e != nil {
		http.Error(w, "Scenario could not be staged", 503)
		return
	}
	status := 201
	if reused {
		status = 200
	}
	writeIntakeJSON(w, status, report)
}

// Stage requires the hash returned by the preview. The digest binds immutable
// order identity/source plus original stock and rate bytes (including XLSX).
// No state-changing request may be staged without this exact prior review.
func scenarioPreviewMatches(provided, calculated string) bool {
	if len(provided) != 64 || len(calculated) != 64 || provided != strings.ToLower(provided) {
		return false
	}
	p, pe := hex.DecodeString(provided)
	c, ce := hex.DecodeString(calculated)
	return pe == nil && ce == nil && subtle.ConstantTimeCompare(p, c) == 1
}

func (s *EdgeServer) persistMaterialScenario(ctx context.Context, tenant string, src scenarioSources) (scenarioReport, bool, error) {
	id, e := newOrderIntakeID()
	if e != nil {
		return scenarioReport{}, false, e
	}
	id = "MS-" + strings.TrimPrefix(id, "OI-")
	raw, e := json.Marshal(src.Report)
	if e != nil {
		return scenarioReport{}, false, e
	}
	tx, e := s.db.BeginTx(ctx, &sql.TxOptions{})
	if e != nil {
		return scenarioReport{}, false, e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id',$1,true)`, tenant); e != nil {
		return scenarioReport{}, false, e
	}
	res, e := tx.ExecContext(ctx, `INSERT INTO material_scenarios_v6(id,tenant_id,intake_id,source_sha256,scenario_sha256,stock_filename,prices_filename,stock_original,prices_original,report_json) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb) ON CONFLICT(tenant_id,intake_id,scenario_sha256) DO NOTHING`, id, tenant, src.Report.IntakeID, src.Report.IntakeSourceSHA256, src.Report.ScenarioSHA256, src.Report.StockFile, src.Report.PricesFile, src.Stock, src.Prices, string(raw))
	if e != nil {
		return scenarioReport{}, false, e
	}
	n, e := res.RowsAffected()
	if e != nil {
		return scenarioReport{}, false, e
	}
	if n == 0 {
		e = tx.QueryRowContext(ctx, `SELECT id,created_at FROM material_scenarios_v6 WHERE tenant_id=$1 AND intake_id=$2 AND scenario_sha256=$3`, tenant, src.Report.IntakeID, src.Report.ScenarioSHA256).Scan(&src.Report.ID, &src.Report.CreatedAt)
	} else {
		src.Report.ID = id
		now := time.Now().UTC()
		src.Report.CreatedAt = &now
	}
	if e != nil {
		return scenarioReport{}, false, e
	}
	if e = tx.Commit(); e != nil {
		return scenarioReport{}, false, e
	}
	return src.Report, n == 0, nil
}
func (s *EdgeServer) readMaterialScenario(ctx context.Context, w http.ResponseWriter, r *http.Request, tenant string) {
	tx, e := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id',$1,true)`, tenant); e != nil {
		http.Error(w, "Tenant unavailable", 503)
		return
	}
	id := r.URL.Query().Get("id")
	if id != "" {
		if len(id) != 35 || !strings.HasPrefix(id, "MS-") {
			http.Error(w, "Invalid scenario ID", 400)
			return
		}
		var repjson []byte
		var created time.Time
		e = tx.QueryRowContext(ctx, `SELECT report_json::text,created_at FROM material_scenarios_v6 WHERE tenant_id=$1 AND id=$2`, tenant, id).Scan(&repjson, &created)
		if e == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		if e != nil {
			http.Error(w, "Query failed", 503)
			return
		}
		var rep scenarioReport
		if e = json.Unmarshal(repjson, &rep); e != nil {
			http.Error(w, "Stored report invalid", 503)
			return
		}
		rep.ID = id
		rep.CreatedAt = &created
		if e = tx.Commit(); e != nil {
			http.Error(w, "Query failed", 503)
			return
		}
		writeIntakeJSON(w, 200, rep)
		return
	}
	intake := r.URL.Query().Get("intake")
	if len(intake) != 35 || !strings.HasPrefix(intake, "OI-") {
		http.Error(w, "intake query parameter required", 400)
		return
	}
	rows, e := tx.QueryContext(ctx, `SELECT id,scenario_sha256,created_at FROM material_scenarios_v6 WHERE tenant_id=$1 AND intake_id=$2 ORDER BY created_at DESC LIMIT 30`, tenant, intake)
	if e != nil {
		http.Error(w, "Query failed", 503)
		return
	}
	type entry struct {
		ID        string    `json:"id"`
		SHA       string    `json:"scenario_sha256"`
		CreatedAt time.Time `json:"created_at"`
	}
	items := []entry{}
	for rows.Next() {
		var item entry
		if e = rows.Scan(&item.ID, &item.SHA, &item.CreatedAt); e != nil {
			break
		}
		items = append(items, item)
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		http.Error(w, "Query failed", 503)
		return
	}
	if e = tx.Commit(); e != nil {
		http.Error(w, "Query failed", 503)
		return
	}
	writeIntakeJSON(w, 200, map[string]any{"items": items, "count": len(items), "release_gate": "HOLD"})
}
