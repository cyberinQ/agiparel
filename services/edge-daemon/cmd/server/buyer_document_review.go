// Gate 06.5-C1: map extracted buyer evidence into *proposed* PO/BOM records.
// Original documents remain immutable and linked. No automatic buyer approval.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type buyerEvidence struct {
	Ordinal int             `json:"ordinal"`
	Kind    string          `json:"kind"`
	Locator json.RawMessage `json:"locator"`
	Content string          `json:"content"`
}
type buyerSource struct {
	ID        string          `json:"id"`
	Filename  string          `json:"filename"`
	SHA256    string          `json:"sha256"`
	Status    string          `json:"status"`
	Format    string          `json:"format"`
	Segments  []buyerEvidence `json:"segments"`
	Truncated bool            `json:"truncated"`
}
type buyerSKU struct {
	Color         string `json:"color"`
	Size          string `json:"size"`
	UPC           string `json:"upc"`
	Quantity      int64  `json:"quantity"`
	SourceOrdinal int    `json:"source_ordinal"`
}
type buyerMaterial struct {
	StyleCode     string `json:"style_code,omitempty"`
	SKU           string `json:"material_sku"`
	Category      string `json:"category"`
	PerGarment    string `json:"per_garment"`
	UOM           string `json:"uom"`
	SourceOrdinal int    `json:"source_ordinal"`
}
type buyerOrderOption struct {
	PONumber  string     `json:"po_number"`
	StyleCode string     `json:"style_code"`
	SKUs      []buyerSKU `json:"skus"`
}
type buyerReviewRequest struct {
	PODocumentID  string          `json:"po_document_id"`
	BOMDocumentID string          `json:"bom_document_id"`
	PONumber      string          `json:"po_number"`
	StyleCode     string          `json:"style_code"`
	SKUs          []buyerSKU      `json:"skus"`
	Materials     []buyerMaterial `json:"materials"`
	ReviewNote    string          `json:"review_note"`
	Acknowledged  bool            `json:"acknowledged"`
}

var buyerCell = regexp.MustCompile(`^([A-Z]{1,3})[0-9]+:\s*(.*)$`)
var buyerHeaderNoise = regexp.MustCompile(`[^a-z0-9]+`)
var buyerWalmartPO = regexp.MustCompile(`(?m)^\s*(REP[0-9]{1,8})\s+SO/PO#`)
var buyerWalmartStyle = regexp.MustCompile(`(?m)PROJECT\s*#:\s*([A-Za-z0-9_./-]{1,64})`)
var buyerWalmartColorRow = regexp.MustCompile(`^[A-Za-z0-9_-]*-[A-Za-z0-9_-]+$`)
var buyerWalmartStyleToken = regexp.MustCompile(`^TT[A-Za-z0-9]{5,25}$`)

func buyerHeader(s string) string {
	return strings.Trim(buyerHeaderNoise.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "_"), "_")
}
func buyerField(m map[string]string, keys ...string) string {
	for _, key := range keys {
		if x, ok := m[key]; ok && strings.TrimSpace(x) != "" {
			return strings.TrimSpace(x)
		}
	}
	return ""
}
func buyerSegmentValues(x buyerEvidence) (map[string]string, bool) {
	m := map[string]string{}
	switch x.Kind {
	case "ROW":
		var vals []string
		if json.Unmarshal([]byte(x.Content), &vals) != nil {
			return nil, false
		}
		for i, v := range vals {
			m[strconv.Itoa(i)] = strings.TrimSpace(v)
		}
	case "SHEET_ROW":
		for _, chunk := range strings.Split(x.Content, " | ") {
			hit := buyerCell.FindStringSubmatch(strings.TrimSpace(chunk))
			if len(hit) == 3 {
				m[hit[1]] = strings.TrimSpace(hit[2])
			}
		}
	default:
		return nil, false
	}
	return m, len(m) > 0
}
func buyerScope(x buyerEvidence) string {
	var l struct {
		Sheet string `json:"sheet"`
		Page  int    `json:"page"`
	}
	_ = json.Unmarshal(x.Locator, &l)
	if l.Sheet != "" {
		return l.Sheet
	}
	return strconv.Itoa(l.Page)
}
func buyerHeadersFromRow(m map[string]string) (map[string]string, bool) {
	mapping := map[string]string{}
	hits := 0
	for col, value := range m {
		norm := buyerHeader(value)
		if norm == "" {
			continue
		}
		mapping[col] = norm
		switch norm {
		case "po_number", "purchase_order", "po_no", "style_code", "style", "ordered_qty", "order_qty", "quantity", "qty", "color_code", "color", "size_code", "size", "material_sku", "item_code", "qty_per_garment", "consumption", "uom", "unit":
			hits++
		}
	}
	return mapping, hits >= 2
}
func buyerMappedValues(row map[string]string, header map[string]string) map[string]string {
	out := map[string]string{}
	for col, value := range row {
		if key := header[col]; key != "" {
			out[key] = strings.TrimSpace(value)
		}
	}
	return out
}

// Conservative, header-driven extraction from CSV/XLSX; no scalar facts invented.
// Resets header on every worksheet. PDF/DOCX evidence remains visible for manual review.
func detectBuyerOptions(segments []buyerEvidence) ([]buyerOrderOption, []buyerMaterial, []string) {
	headerByScope := map[string]map[string]string{}
	poMap := map[string]*buyerOrderOption{}
	order := []string{}
	materials := []buyerMaterial{}
	issues := []string{}
	for _, seg := range segments {
		cells, ok := buyerSegmentValues(seg)
		if !ok {
			continue
		}
		scope := buyerScope(seg)
		if h, isHeader := buyerHeadersFromRow(cells); isHeader {
			headerByScope[scope] = h
			continue
		}
		h := headerByScope[scope]
		if len(h) == 0 {
			continue
		}
		row := buyerMappedValues(cells, h)
		po := buyerField(row, "po_number", "purchase_order", "po_no", "po")
		style := buyerField(row, "style_code", "style", "style_no")
		color := buyerField(row, "color_code", "color", "colorway")
		size := buyerField(row, "size_code", "size")
		qtyText := buyerField(row, "ordered_qty", "order_qty", "quantity", "qty")
		if po != "" && style != "" && color != "" && size != "" && qtyText != "" {
			qty, err := strconv.ParseInt(strings.ReplaceAll(qtyText, ",", ""), 10, 64)
			if err != nil || qty <= 0 || qty > 100000000 {
				issues = append(issues, fmt.Sprintf("PO source segment %d has invalid quantity", seg.Ordinal))
				continue
			}
			key := po + "\x00" + style
			option := poMap[key]
			if option == nil {
				option = &buyerOrderOption{PONumber: po, StyleCode: style, SKUs: []buyerSKU{}}
				poMap[key] = option
				order = append(order, key)
			}
			option.SKUs = append(option.SKUs, buyerSKU{Color: color, Size: size, UPC: buyerField(row, "upc", "barcode", "ean"), Quantity: qty, SourceOrdinal: seg.Ordinal})
		}
		material := buyerField(row, "material_sku", "material_code", "item_code", "material")
		cat := strings.ToUpper(buyerField(row, "category", "material_category", "type"))
		rate := buyerField(row, "qty_per_garment", "consumption", "per_garment")
		uom := buyerField(row, "uom", "unit", "units")
		if material != "" && (cat == "FABRIC" || cat == "TRIM") && rate != "" && uom != "" {
			materials = append(materials, buyerMaterial{StyleCode: style, SKU: material, Category: cat, PerGarment: rate, UOM: uom, SourceOrdinal: seg.Ordinal})
		}
	}
	opts := make([]buyerOrderOption, 0, len(poMap))
	for _, key := range order {
		opts = append(opts, *poMap[key])
	}
	sort.SliceStable(opts, func(i, j int) bool { return opts[i].PONumber+opts[i].StyleCode < opts[j].PONumber+opts[j].StyleCode })
	for _, seg := range segments {
		option, warn := detectWalmartSalesOrder(seg)
		if warn != "" {
			issues = append(issues, warn)
		}
		if option == nil {
			continue
		}
		key := option.PONumber + "\x00" + option.StyleCode
		if existing, ok := poMap[key]; ok {
			existing.SKUs = append(existing.SKUs, option.SKUs...)
		} else {
			poMap[key] = option
			order = append(order, key)
		}
	}
	// Rebuild once PDF page records have joined the structured rows.
	opts = opts[:0]
	for _, key := range order {
		opts = append(opts, *poMap[key])
	}
	sort.SliceStable(opts, func(i, j int) bool { return opts[i].PONumber+opts[i].StyleCode < opts[j].PONumber+opts[j].StyleCode })
	return opts, materials, issues
}

// An intentionally narrow parser for the supplied YSAG/Walmart sales-order page layout.
// It refuses to infer quantities unless size columns and explicit line totals reconcile.
// This is a draft hint, NEVER buyer or factory approval.
func detectWalmartSalesOrder(seg buyerEvidence) (*buyerOrderOption, string) {
	if seg.Kind != "PAGE" || !strings.Contains(seg.Content, "YSAG CONFIRMED SALES ORDER") || !strings.Contains(seg.Content, "COLOR & UNIT INFORMATION") || !strings.Contains(seg.Content, "SALES ORDER ITEM INFO") {
		return nil, ""
	}
	poMatch := buyerWalmartPO.FindStringSubmatch(seg.Content)
	styleMatch := buyerWalmartStyle.FindStringSubmatch(seg.Content)
	if len(poMatch) != 2 || len(styleMatch) != 2 {
		return nil, fmt.Sprintf("Walmart-like page in segment %d: PO or style not found; review manually", seg.Ordinal)
	}
	sizes := []string{}
	data := strings.Split(seg.Content, "\n")
	for _, line := range data {
		if !strings.Contains(line, "SALES ORDER ITEM INFO") {
			continue
		}
		fields := strings.Fields(strings.SplitN(line, "SALES ORDER ITEM INFO", 2)[1])
		for _, f := range fields {
			if f == "UNITS" || f == "PPK" {
				break
			}
			if len(f) >= 2 && len(f) <= 4 {
				if _, err := strconv.ParseInt(f, 10, 64); err == nil {
					sizes = append(sizes, f)
					continue
				}
			}
			if len(sizes) > 0 {
				break
			}
		}
		break
	}
	if len(sizes) < 1 || len(sizes) > 30 {
		return nil, fmt.Sprintf("Walmart-like page %d: size columns could not be determined", seg.Ordinal)
	}
	option := &buyerOrderOption{PONumber: poMatch[1], StyleCode: styleMatch[1], SKUs: []buyerSKU{}}
	for _, line := range data {
		fields := strings.Fields(line)
		if len(fields) < len(sizes)+4 || !buyerWalmartColorRow.MatchString(fields[0]) {
			continue
		}
		styleAt := -1
		for i := 1; i < len(fields); i++ {
			if buyerWalmartStyleToken.MatchString(fields[i]) {
				styleAt = i
				break
			}
		}
		if styleAt < 2 || styleAt+len(sizes)+1 >= len(fields) {
			continue
		}
		color := strings.Join(fields[1:styleAt], "_")
		quantities := make([]int64, len(sizes))
		var total int64
		valid := true
		for i := range sizes {
			var err error
			quantities[i], err = strconv.ParseInt(strings.ReplaceAll(fields[styleAt+1+i], ",", ""), 10, 64)
			if err != nil || quantities[i] < 0 || quantities[i] > 100000000 {
				valid = false
				break
			}
			total += quantities[i]
		}
		if !valid {
			continue
		}
		documented, err := strconv.ParseInt(strings.ReplaceAll(fields[styleAt+1+len(sizes)], ",", ""), 10, 64)
		if err != nil || documented != total {
			return nil, fmt.Sprintf("Walmart-like page %d: size quantities do not equal printed total", seg.Ordinal)
		}
		for i, size := range sizes {
			if quantities[i] > 0 {
				option.SKUs = append(option.SKUs, buyerSKU{Color: color, Size: size, Quantity: quantities[i], SourceOrdinal: seg.Ordinal})
			}
		}
	}
	if len(option.SKUs) == 0 {
		return nil, fmt.Sprintf("Walmart-like page %d: no reconciled color/size detail located", seg.Ordinal)
	}
	return option, fmt.Sprintf("Walmart sales-order page %d suggested PO %s; check project/style mapping, packing and buyer revisions before staging", seg.Ordinal, option.PONumber)
}

func readBuyerSource(ctx context.Context, tx *sql.Tx, tenant, id string) (buyerSource, error) {
	var out buyerSource
	if !validDocSourceID(id) {
		return out, errors.New("invalid document ID")
	}
	err := tx.QueryRowContext(ctx, `SELECT d.id,d.filename,d.sha256,d.format,j.status FROM document_sources_v65 d JOIN document_jobs_v65 j ON j.tenant_id=d.tenant_id AND j.document_id=d.id WHERE d.tenant_id=$1 AND d.id=$2`, tenant, id).Scan(&out.ID, &out.Filename, &out.SHA256, &out.Format, &out.Status)
	if err != nil {
		return out, err
	}
	if out.Status != "EXTRACTED" && out.Status != "PARTIAL" {
		return out, errors.New("source has not completed extraction")
	}
	rows, err := tx.QueryContext(ctx, `SELECT ordinal,kind,locator,content FROM document_segments_v65 WHERE tenant_id=$1 AND document_id=$2 ORDER BY ordinal LIMIT 1201`, tenant, id)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	out.Segments = []buyerEvidence{}
	for rows.Next() {
		var x buyerEvidence
		var loc []byte
		if err = rows.Scan(&x.Ordinal, &x.Kind, &loc, &x.Content); err != nil {
			return out, err
		}
		if len(out.Segments) >= 1200 {
			out.Truncated = true
			break
		}
		x.Locator = loc
		out.Segments = append(out.Segments, x)
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	return out, nil
}
func buyerCSV(header []string, rows [][]string) ([]byte, error) {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	if err := w.Write(header); err != nil {
		return nil, err
	}
	for _, r := range rows {
		if err := w.Write(r); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return b.Bytes(), w.Error()
}
func prepareBuyerSubmission(req buyerReviewRequest, po, bom buyerSource) (sourceSubmission, error) {
	if !req.Acknowledged || len(strings.TrimSpace(req.ReviewNote)) < 8 || len(req.ReviewNote) > 1000 {
		return sourceSubmission{}, errors.New("Confirm review and describe how buyer facts were checked (8–1000 characters)")
	}
	if len(req.SKUs) < 1 || len(req.SKUs) > 2000 || len(req.Materials) < 1 || len(req.Materials) > 2000 {
		return sourceSubmission{}, errors.New("Add at least one PO SKU and material; maximum 2000 each")
	}
	poOrd, bomOrd := map[int]bool{}, map[int]bool{}
	for _, s := range po.Segments {
		poOrd[s.Ordinal] = true
	}
	for _, s := range bom.Segments {
		bomOrd[s.Ordinal] = true
	}
	poRows := [][]string{}
	bomRows := [][]string{}
	for i, x := range req.SKUs {
		if x.SourceOrdinal < 0 || (x.SourceOrdinal > 0 && !poOrd[x.SourceOrdinal]) {
			return sourceSubmission{}, fmt.Errorf("PO row %d cites an unavailable source segment", i+1)
		}
		poRows = append(poRows, []string{req.PONumber, req.StyleCode, x.Color, x.Size, strconv.FormatInt(x.Quantity, 10), x.UPC})
	}
	for i, x := range req.Materials {
		if x.SourceOrdinal < 0 || (x.SourceOrdinal > 0 && !bomOrd[x.SourceOrdinal]) {
			return sourceSubmission{}, fmt.Errorf("BOM row %d cites an unavailable source segment", i+1)
		}
		if x.StyleCode != "" && x.StyleCode != req.StyleCode {
			return sourceSubmission{}, fmt.Errorf("BOM row %d belongs to a different style", i+1)
		}
		bomRows = append(bomRows, []string{req.StyleCode, x.SKU, x.Category, x.PerGarment, x.UOM})
	}
	rawPO, err := buyerCSV([]string{"po_number", "style_code", "color_code", "size_code", "ordered_qty", "upc"}, poRows)
	if err != nil {
		return sourceSubmission{}, err
	}
	rawBOM, err := buyerCSV([]string{"style_code", "material_sku", "category", "qty_per_garment", "uom"}, bomRows)
	if err != nil {
		return sourceSubmission{}, err
	}
	sub, err := parseIntake(rawPO, rawBOM, "reviewed-PO-"+po.ID+".csv", "reviewed-BOM-"+bom.ID+".csv")
	if err != nil {
		return sourceSubmission{}, err
	}
	h := sha256.New()
	for _, v := range []string{"AGIPAREL-BUYER-REVIEW-V65C1", po.SHA256, bom.SHA256} {
		_, _ = h.Write([]byte(v))
		_, _ = h.Write([]byte{0})
	}
	_, _ = h.Write(rawPO)
	_, _ = h.Write([]byte{0})
	_, _ = h.Write(rawBOM)
	refs, _ := json.Marshal(struct {
		PO  []buyerSKU      `json:"po"`
		BOM []buyerMaterial `json:"bom"`
	}{req.SKUs, req.Materials})
	_, _ = h.Write(refs)
	sub.Report.SourceSHA256 = hex.EncodeToString(h.Sum(nil))
	sub.Report.Origin = "REVIEWED_DOCUMENT_MAPPING"
	sub.Report.Holds = append(sub.Report.Holds, "Rows were transcribed or corrected by a developer/operator; source links do not prove buyer approval")
	return sub, nil
}
func (s *EdgeServer) handleBuyerReview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", 405)
		return
	}
	tenant, err := intakeTenant()
	if err != nil || s.db == nil {
		http.Error(w, "Development tenant or PostgreSQL unavailable", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	var req buyerReviewRequest
	if r.Method == http.MethodGet {
		req.PODocumentID = r.URL.Query().Get("po_document_id")
		req.BOMDocumentID = r.URL.Query().Get("bom_document_id")
	} else {
		if !candidateJSONBody(w, r, 220000, &req) {
			return
		}
	}
	if !validDocSourceID(req.PODocumentID) || !validDocSourceID(req.BOMDocumentID) {
		http.Error(w, "Choose two original uploaded source documents", 400)
		return
	}
	tx, err := s.docTransaction(ctx, tenant)
	if err != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	defer tx.Rollback()
	po, err := readBuyerSource(ctx, tx, tenant, req.PODocumentID)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Buyer PO source not found", 404)
		return
	}
	if err != nil {
		http.Error(w, "Buyer PO is not extractable", 409)
		return
	}
	bom := po
	if req.PODocumentID != req.BOMDocumentID {
		bom, err = readBuyerSource(ctx, tx, tenant, req.BOMDocumentID)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "BOM source not found", 404)
			return
		}
		if err != nil {
			http.Error(w, "BOM source is not extractable", 409)
			return
		}
	}
	if r.Method == http.MethodGet {
		opts, _, issues := detectBuyerOptions(po.Segments)
		_, mats, bomIssues := detectBuyerOptions(bom.Segments)
		issues = append(issues, bomIssues...)
		if po.Truncated || bom.Truncated {
			issues = append(issues, "Only the first 1200 segments from each source were assessed; inspect originals for omitted content")
		}
		if po.Status == "PARTIAL" || bom.Status == "PARTIAL" {
			issues = append(issues, "A source was only partially extracted; manual verification is essential")
		}
		if err = tx.Commit(); err != nil {
			http.Error(w, "Database unavailable", 503)
			return
		}
		docJSON(w, 200, map[string]any{"po_source": po, "bom_source": bom, "po_options": opts, "materials": mats, "issues": issues, "status": "UNREVIEWED", "release": "HOLD"})
		return
	}
	sub, err := prepareBuyerSubmission(req, po, bom)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	rep, reused, err := persistIntakeInTx(ctx, tx, tenant, sub)
	if err != nil {
		http.Error(w, "Unable to stage reviewed order", 503)
		return
	}
	mapping, err := json.Marshal(req)
	if err != nil {
		http.Error(w, "Invalid mapping", 400)
		return
	}
	reviewedBy := os.Getenv("AGIPAREL_OPERATOR_USER") // Development-only identity; production needs authenticated reviewer identity.
	if reviewedBy == "" {
		reviewedBy = "dev"
	}
	var handoffID string
	err = tx.QueryRowContext(ctx, `INSERT INTO buyer_order_handoffs_v65c1(tenant_id,intake_id,po_document_id,bom_document_id,source_digest,reviewed_by,review_note,mapping)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8)
 ON CONFLICT (tenant_id,source_digest) DO NOTHING
 RETURNING id`, tenant, rep.ID, po.ID, bom.ID, sub.Report.SourceSHA256, reviewedBy, strings.TrimSpace(req.ReviewNote), mapping).Scan(&handoffID)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `SELECT id::text FROM buyer_order_handoffs_v65c1 WHERE tenant_id=$1 AND source_digest=$2`, tenant, sub.Report.SourceSHA256).Scan(&handoffID)
	}
	if err != nil {
		http.Error(w, "Cannot save source review", 503)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "Cannot commit reviewed order", 503)
		return
	}
	rep.Origin = "REVIEWED_DOCUMENT_MAPPING"
	docJSON(w, map[bool]int{true: 200, false: 201}[reused], map[string]any{"intake": rep, "handoff_id": handoffID, "reused": reused, "status": "PROPOSED", "release": "HOLD", "next": "/material-planning", "source_note": "The existing order and BOM quantities are proposals. Original sources remain in Document Intelligence."})
}
