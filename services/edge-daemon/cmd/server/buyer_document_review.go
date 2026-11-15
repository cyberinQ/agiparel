// Author: Sunil Jain, CyberinQ LLC
// Gate 06.5-C1: customer-agnostic mapping of extracted buyer evidence into *proposed* PO/BOM records,
// with pre-save colleague conflict detection and line-by-line discrepancy reconciliation.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
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
type buyerDrawing struct {
	Title   string `json:"title"`
	Source  string `json:"source"`
	Page    int    `json:"page"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	DataURL string `json:"data_url"`
	SHA256  string `json:"sha256"`
}
type buyerSKU struct {
	MaterialNumber string `json:"material_number,omitempty"`
	BuyerStyleNo   string `json:"buyer_style_no,omitempty"`
	Color          string `json:"color"`
	Size           string `json:"size"`
	UPC            string `json:"upc"`
	PackRatio      string `json:"pack_ratio,omitempty"`
	Quantity       int64  `json:"quantity"`
	Cases          string `json:"cases,omitempty"`
	UnitCost       string `json:"unit_cost,omitempty"`
	LineTotal      string `json:"line_total,omitempty"`
	SourceOrdinal  int    `json:"source_ordinal"`
}
type buyerMaterial struct {
	StyleCode     string `json:"style_code,omitempty"`
	SKU           string `json:"material_sku"`
	Category      string `json:"category"`
	PerGarment    string `json:"per_garment"`
	UOM           string `json:"uom"`
	SourceOrdinal int    `json:"source_ordinal"`
}
type buyerOrderDetails struct {
	SalesOrderNo     string            `json:"sales_order_no,omitempty"`
	CustomerPONo     string            `json:"customer_po_no,omitempty"`
	ProgramNo        string            `json:"program_no,omitempty"`
	SupplierNo       string            `json:"supplier_no,omitempty"`
	PLMNo            string            `json:"plm_no,omitempty"`
	ProjectNo        string            `json:"project_no,omitempty"`
	BuyerStyleNo     string            `json:"buyer_style_no,omitempty"`
	MaterialNo       string            `json:"material_no,omitempty"`
	Customer         string            `json:"customer,omitempty"`
	Buyer            string            `json:"buyer,omitempty"`
	BrandManager     string            `json:"brand_manager,omitempty"`
	CountryOfOrigin  string            `json:"country_of_origin,omitempty"`
	FactoryID        string            `json:"factory_id,omitempty"`
	VendorCode       string            `json:"vendor_code,omitempty"`
	GoodsSupplier    string            `json:"goods_supplier,omitempty"`
	ShipTo           string            `json:"ship_to,omitempty"`
	Warehouse        string            `json:"warehouse,omitempty"`
	Season           string            `json:"season,omitempty"`
	SeasonCode       string            `json:"season_code,omitempty"`
	Dept             string            `json:"dept,omitempty"`
	Fineline         string            `json:"fineline,omitempty"`
	RetailPrice      string            `json:"retail_price,omitempty"`
	UnitCost         string            `json:"unit_cost,omitempty"`
	TotalOrderValue  string            `json:"total_order_value,omitempty"`
	TotalCases       string            `json:"total_cases,omitempty"`
	ItemDesc         string            `json:"item_desc,omitempty"`
	InWhseDate       string            `json:"in_whse_date,omitempty"`
	MABD             string            `json:"mabd,omitempty"`
	RequiredDelivery string            `json:"required_delivery_date,omitempty"`
	CancelDate       string            `json:"cancel_date,omitempty"`
	ValidFromDate    string            `json:"valid_from_date,omitempty"`
	ExFactoryDate    string            `json:"ex_factory_date,omitempty"`
	ExPortDate       string            `json:"ex_port_date,omitempty"`
	ETAToPort        string            `json:"eta_to_port,omitempty"`
	DeliveryDate     string            `json:"delivery_date,omitempty"`
	Incoterms        string            `json:"incoterms,omitempty"`
	PortOfExit       string            `json:"port_of_exit,omitempty"`
	PortOfEntry      string            `json:"port_of_entry,omitempty"`
	FreightForwarder string            `json:"freight_forwarder,omitempty"`
	ShipMode         string            `json:"ship_mode,omitempty"`
	FabricSpec       string            `json:"fabric_spec,omitempty"`
	FabricContent    string            `json:"fabric_content,omitempty"`
	FabricWeightGSM  string            `json:"fabric_weight_gsm,omitempty"`
	FabricCategory   string            `json:"fabric_category,omitempty"`
	HTSCode          string            `json:"hts_code,omitempty"`
	Accessories      map[string]string `json:"accessories,omitempty"`
	PackingRules     []string          `json:"packing_rules,omitempty"`
}
type buyerOrderOption struct {
	PONumber    string             `json:"po_number"`
	StyleCode   string             `json:"style_code"`
	SKUs        []buyerSKU         `json:"skus"`
	Details     *buyerOrderDetails `json:"details,omitempty"`
	POMaterials []buyerBOMHint     `json:"po_materials,omitempty"`
}
type buyerReviewRequest struct {
	PODocumentID    string             `json:"po_document_id"`
	BOMDocumentID   string             `json:"bom_document_id"`
	PONumber        string             `json:"po_number"`
	StyleCode       string             `json:"style_code"`
	SKUs            []buyerSKU         `json:"skus"`
	Materials       []buyerMaterial    `json:"materials"`
	OrderDetails    *buyerOrderDetails `json:"order_details,omitempty"`
	ReviewNote      string             `json:"review_note"`
	Acknowledged    bool               `json:"acknowledged"`
	SupersedeDigest string             `json:"supersede_digest,omitempty"`
}
type buyerFieldDiff struct {
	Scope          string `json:"scope"`
	Key            string `json:"key"`
	ColleagueValue string `json:"colleague_value"`
	YourValue      string `json:"your_value"`
}
type buyerSavedDraft struct {
	HandoffID    string          `json:"handoff_id"`
	IntakeID     string          `json:"intake_id"`
	PONumber     string          `json:"po_number"`
	StyleCode    string          `json:"style_code"`
	OrderedQty   int64           `json:"ordered_qty"`
	SourceDigest string          `json:"source_digest"`
	ReviewedBy   string          `json:"reviewed_by"`
	ReviewNote   string          `json:"review_note"`
	CapturedAt   time.Time       `json:"captured_at"`
	SKUs         []buyerSKU      `json:"skus"`
	Materials    []buyerMaterial `json:"materials"`
}

var (
	buyerCell            = regexp.MustCompile(`^([A-Z]{1,3})[0-9]+:\s*(.*)$`)
	buyerHeaderNoise     = regexp.MustCompile(`[^a-z0-9]+`)
	genericCompositePORe = regexp.MustCompile(`(?im)^\s*([A-Za-z0-9_-]{2,24})\s+(?:SO/PO#|PO\s*#|ORDER\s*#)\s*[:/]?\s*([A-Za-z0-9_-]{3,32})?`)
	genericStyleRe       = regexp.MustCompile(`(?im)(?:PROJECT\s*#|STYLE\s*(?:CODE|#|NO\.?)|ARTICLE\s*#|ITEM\s*#)\s*:\s*([A-Za-z0-9_./-]{2,64})`)
	genericSizeTokenRe   = regexp.MustCompile(`^(?:[2-5][0-9]|[0-9]{1,2}(?:W|L|X[0-9]{2})?|XXS|XS|S|M|L|XL|[2-6]XL|OS)$`)
	genericStyleTokenRe  = regexp.MustCompile(`^[A-Z]{2,6}[0-9]{3,16}[A-Z0-9_-]*$`)
)

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
		if (color != "" || size != "" || po != "" && qtyText != "") &&
			(po == "" || style == "" || color == "" || size == "" || qtyText == "") {
			issues = append(issues, fmt.Sprintf("PO source segment %d has missing order/style/color/size/quantity; inspect the original", seg.Ordinal))
		}
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
			option.SKUs = append(option.SKUs, buyerSKU{
				Color:         color,
				Size:          size,
				UPC:           buyerField(row, "upc", "barcode", "ean"),
				Quantity:      qty,
				UnitCost:      buyerField(row, "unit_cost", "unit_price", "price"),
				SourceOrdinal: seg.Ordinal,
			})
		}
		material := buyerField(row, "material_sku", "material_code", "item_code", "material")
		cat := strings.ToUpper(buyerField(row, "category", "material_category", "type"))
		rate := buyerField(row, "qty_per_garment", "consumption", "per_garment")
		uom := buyerField(row, "uom", "unit", "units")
		if material != "" && ((cat != "FABRIC" && cat != "TRIM") || rate == "" || uom == "") {
			issues = append(issues, fmt.Sprintf("BOM source segment %d has missing/invalid category, usage or unit; inspect the original", seg.Ordinal))
		}
		if material != "" && (cat == "FABRIC" || cat == "TRIM") && rate != "" && uom != "" {
			materials = append(materials, buyerMaterial{StyleCode: style, SKU: material, Category: cat, PerGarment: rate, UOM: uom, SourceOrdinal: seg.Ordinal})
		}
	}
	var lastPDFOption *buyerOrderOption
	for _, seg := range segments {
		if mergeCompanionOrderPage(seg, poMap, lastPDFOption) {
			continue
		}
		option, warn := detectStructuredPDFOrder(seg)
		if warn != "" {
			issues = append(issues, warn)
		}
		if option == nil {
			continue
		}
		lastPDFOption = option
		key := option.PONumber + "\x00" + option.StyleCode
		if existing, ok := poMap[key]; ok {
			existing.SKUs = append(existing.SKUs, option.SKUs...)
			if existing.Details == nil {
				existing.Details = option.Details
			}
			if len(existing.POMaterials) == 0 {
				existing.POMaterials = option.POMaterials
			}
		} else {
			poMap[key] = option
			order = append(order, key)
		}
	}
	opts := make([]buyerOrderOption, 0, len(order))
	for _, key := range order {
		opts = append(opts, *poMap[key])
	}
	sort.SliceStable(opts, func(i, j int) bool { return opts[i].PONumber+opts[i].StyleCode < opts[j].PONumber+opts[j].StyleCode })
	return opts, materials, issues
}

func detectWalmartSalesOrder(seg buyerEvidence) (*buyerOrderOption, string) {
	return detectStructuredPDFOrder(seg)
}

func extractFirstMatch(text string, patterns ...string) string {
	for _, p := range patterns {
		m := regexp.MustCompile(p).FindStringSubmatch(text)
		if len(m) >= 2 && strings.TrimSpace(m[1]) != "" {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

func detectStructuredPDFOrder(seg buyerEvidence) (*buyerOrderOption, string) {
	if seg.Kind != "PAGE" {
		return nil, ""
	}
	upper := strings.ToUpper(seg.Content)
	if !strings.Contains(upper, "ORDER") && !strings.Contains(upper, "PO#") && !strings.Contains(upper, "PO #") {
		return nil, ""
	}
	if !strings.Contains(upper, "UNITS") && !strings.Contains(upper, "TOTAL") && !strings.Contains(upper, "QTY") {
		return nil, ""
	}
	custPO, salesOrderNo := "", ""
	if m := genericCompositePORe.FindStringSubmatch(seg.Content); len(m) >= 2 {
		custPO = strings.TrimSpace(m[1])
		if len(m) >= 3 {
			salesOrderNo = strings.TrimSpace(m[2])
		}
	}
	if custPO == "" {
		custPO = extractFirstMatch(seg.Content, `(?im)(?:CUSTOMER\s+PO\s*#|PURCHASE\s+ORDER\s*#?|PO\s*#|SO/PO\s*#)\s*[:/]?\s*([A-Za-z0-9_-]{2,32})`)
	}
	styleCode := ""
	if m := genericStyleRe.FindStringSubmatch(seg.Content); len(m) == 2 {
		styleCode = strings.TrimSpace(m[1])
	}
	if custPO == "" || styleCode == "" {
		if strings.Contains(upper, "SALES ORDER ITEM INFO") {
			return nil, fmt.Sprintf("Order page in segment %d: PO or style not found; review manually", seg.Ordinal)
		}
		return nil, ""
	}
	sizes, packRatios := []string{}, []string{}
	lines := strings.Split(seg.Content, "\n")
	for _, line := range lines {
		uLine := strings.ToUpper(line)
		if strings.Contains(uLine, "COLOR & UNIT INFORMATION") || strings.Contains(uLine, "PACK RATIO") {
			for _, f := range strings.Fields(line) {
				if f == "PPK" || f == "YES" || f == "NO" {
					break
				}
				if strings.Contains(f, "/") && len(f) <= 8 {
					packRatios = append(packRatios, f)
				}
			}
		}
		if !strings.Contains(uLine, "UNITS") && !strings.Contains(uLine, "QTY") {
			continue
		}
		candidate := line
		if idx := strings.Index(uLine, "ITEM INFO"); idx >= 0 {
			candidate = line[idx+len("ITEM INFO"):]
		} else if idx := strings.Index(uLine, "SIZE"); idx >= 0 {
			candidate = line[idx+len("SIZE"):]
		}
		for _, f := range strings.Fields(candidate) {
			uf := strings.ToUpper(f)
			if uf == "UNITS" || uf == "QTY" || uf == "TOTAL" || uf == "PPK" || uf == "CASES" {
				break
			}
			if genericSizeTokenRe.MatchString(uf) {
				sizes = append(sizes, f)
				continue
			}
			if len(sizes) > 0 {
				break
			}
		}
		if len(sizes) > 0 {
			break
		}
	}
	if len(sizes) < 1 || len(sizes) > 30 {
		if strings.Contains(upper, "SALES ORDER ITEM INFO") {
			return nil, fmt.Sprintf("Order page %d: size columns could not be determined", seg.Ordinal)
		}
		return nil, ""
	}
	details := extractGenericPageDetails(seg.Content, custPO, salesOrderNo, styleCode)
	poMats := extractGenericPOMaterials(seg.Ordinal, details)
	option := &buyerOrderOption{PONumber: custPO, StyleCode: styleCode, SKUs: []buyerSKU{}, Details: details, POMaterials: poMats}
	for _, line := range lines {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(line)), "TOTAL") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < len(sizes)+2 {
			continue
		}
		qtyStart := -1
		for start := 1; start+len(sizes) < len(fields); start++ {
			var sum int64
			allValid := true
			for i := 0; i < len(sizes); i++ {
				v, err := strconv.ParseInt(strings.ReplaceAll(fields[start+i], ",", ""), 10, 64)
				if err != nil || v < 0 || v > 100000000 {
					allValid = false
					break
				}
				sum += v
			}
			if !allValid || sum <= 0 {
				continue
			}
			tot, err := strconv.ParseInt(strings.ReplaceAll(fields[start+len(sizes)], ",", ""), 10, 64)
			if err != nil {
				if start >= 2 && genericStyleTokenRe.MatchString(fields[start-1]) {
					return nil, fmt.Sprintf("Order page %d: size quantities do not equal printed total", seg.Ordinal)
				}
				continue
			}
			if tot != sum {
				return nil, fmt.Sprintf("Order page %d: size quantities do not equal printed total", seg.Ordinal)
			}
			qtyStart = start
			break
		}
		if qtyStart < 1 {
			continue
		}
		prefix := fields[:qtyStart]
		materialNo, buyerStyleNo, color := "", "", ""
		switch {
		case len(prefix) >= 3 && genericStyleTokenRe.MatchString(prefix[len(prefix)-1]):
			materialNo = prefix[0]
			buyerStyleNo = prefix[len(prefix)-1]
			color = strings.Join(prefix[1:len(prefix)-1], "_")
		case len(prefix) == 2 && genericStyleTokenRe.MatchString(prefix[1]):
			color = prefix[0]
			buyerStyleNo = prefix[1]
		default:
			color = strings.Join(prefix, "_")
		}
		if materialNo != "" {
			details.MaterialNo = materialNo
		}
		if buyerStyleNo != "" {
			details.BuyerStyleNo = buyerStyleNo
		}
		totalIdx := qtyStart + len(sizes)
		cases, unitCost, lineTotal := "", "", ""
		for _, tail := range fields[totalIdx+1:] {
			if strings.HasPrefix(tail, "$") || strings.HasPrefix(tail, "€") || strings.HasPrefix(tail, "£") {
				if unitCost == "" {
					unitCost = tail
					details.UnitCost = tail
				} else if lineTotal == "" {
					lineTotal = tail
					details.TotalOrderValue = tail
				}
			} else if cases == "" {
				cases = tail
				details.TotalCases = tail
			}
		}
		for i, size := range sizes {
			q, _ := strconv.ParseInt(strings.ReplaceAll(fields[qtyStart+i], ",", ""), 10, 64)
			if q > 0 {
				ratio := ""
				if i < len(packRatios) {
					ratio = packRatios[i]
				}
				option.SKUs = append(option.SKUs, buyerSKU{
					MaterialNumber: materialNo,
					BuyerStyleNo:   buyerStyleNo,
					Color:          color,
					Size:           size,
					PackRatio:      ratio,
					Quantity:       q,
					Cases:          cases,
					UnitCost:       unitCost,
					LineTotal:      lineTotal,
					SourceOrdinal:  seg.Ordinal,
				})
			}
		}
	}
	if len(option.SKUs) == 0 {
		if strings.Contains(upper, "SALES ORDER ITEM INFO") {
			return nil, fmt.Sprintf("Order page %d: no reconciled color/size detail located", seg.Ordinal)
		}
		return nil, ""
	}
	soLabel := ""
	if salesOrderNo != "" {
		soLabel = " (SO/PO# " + salesOrderNo + ")"
	}
	return option, fmt.Sprintf("Structured order page %d suggested PO %s%s; check project/style mapping, packing and buyer revisions before staging", seg.Ordinal, option.PONumber, soLabel)
}

func mergeCompanionOrderPage(seg buyerEvidence, poMap map[string]*buyerOrderOption, fallback *buyerOrderOption) bool {
	if seg.Kind != "PAGE" || (!strings.Contains(seg.Content, "PRODUCTION ORDER FORM") && !strings.Contains(seg.Content, "PO Terms (Incoterms)")) {
		return false
	}
	custPO := extractFirstMatch(seg.Content, `(?im)\*?Customer\s+PO\s*#:\s*([A-Za-z0-9_-]+)`)
	refStyle := extractFirstMatch(seg.Content, `(?im)(?:Reference|Project|Style)\s*#:\s*([A-Za-z0-9_./-]+)`)
	var target *buyerOrderOption
	for _, opt := range poMap {
		if custPO != "" && (opt.PONumber == custPO || (opt.Details != nil && opt.Details.CustomerPONo == custPO)) {
			if refStyle == "" || opt.StyleCode == refStyle {
				target = opt
				break
			}
		}
	}
	if target == nil {
		target = fallback
	}
	if target == nil {
		return false
	}
	if target.Details == nil {
		target.Details = &buyerOrderDetails{}
	}
	d := target.Details
	if v := extractFirstMatch(seg.Content, `(?im)\*?Required\s+Delivery\s+Date:\s*([0-9/-]+)`); v != "" {
		d.RequiredDelivery = v
	}
	if v := extractFirstMatch(seg.Content, `(?im)\*?Cancel\s+Date:\s*([0-9/-]+)`); v != "" {
		d.CancelDate = v
	}
	if v := extractFirstMatch(seg.Content, `(?im)Valid\s+From\s+Date:\s*([0-9/-]+)`); v != "" {
		d.ValidFromDate = v
	}
	if v := extractFirstMatch(seg.Content, `(?im)(?:PO\s+Terms\s*\(Incoterms\)|Incoterms):\s*([A-Za-z0-9_-]+)`); v != "" {
		d.Incoterms = v
	}
	if v := extractFirstMatch(seg.Content, `(?im)Ex-Factory\s+Date:\s*([0-9/-]+)`); v != "" {
		d.ExFactoryDate = v
	}
	if v := extractFirstMatch(seg.Content, `(?im)Ex-Port\s+Date:\s*([0-9/-]+)`); v != "" {
		d.ExPortDate = v
	}
	if v := extractFirstMatch(seg.Content, `(?im)ETA\s+TO\s+PORT:\s*([0-9/-]+)`); v != "" {
		d.ETAToPort = v
	}
	if v := extractFirstMatch(seg.Content, `(?im)Delivery\s+Date:\s*([0-9/-]+)`); v != "" {
		d.DeliveryDate = v
	}
	if v := extractFirstMatch(seg.Content, `(?im)Port\s+of\s+Exit:\s*([^\n]+)`); v != "" {
		d.PortOfExit = strings.Join(strings.Fields(v), " ")
	}
	if v := extractFirstMatch(seg.Content, `(?im)Port\s+of\s+Entry:\s*([^\n]+)`); v != "" {
		d.PortOfEntry = strings.Join(strings.Fields(v), " ")
	}
	if v := extractFirstMatch(seg.Content, `(?im)Freight\s+Forwarder:\s*([^\n]+)`); v != "" {
		d.FreightForwarder = strings.Join(strings.Fields(v), " ")
	}
	if v := extractFirstMatch(seg.Content, `(?im)Ship\s+Mode:\s*([A-Za-z0-9_-]+)`); v != "" {
		d.ShipMode = v
	}
	return true
}

func extractGenericPageDetails(content, custPO, salesOrderNo, projectNo string) *buyerOrderDetails {
	d := &buyerOrderDetails{
		CustomerPONo: custPO,
		SalesOrderNo: salesOrderNo,
		ProjectNo:    projectNo,
		Accessories:  map[string]string{},
		PackingRules: []string{},
	}
	d.InWhseDate = extractFirstMatch(content, `(?im)IN\s+WHSE\s+([0-9/-]+)`)
	d.Season = extractFirstMatch(content, `(?im)IN\s+WHSE\s+[0-9/-]+\s+([A-Z]+\s+[0-9]{4})`, `(?im)SEASON:\s*([^\n]+?)(?:\s{2,}|$)`)
	d.SeasonCode = extractFirstMatch(content, `(?im)SEASON\s+CODE:\s*([0-9A-Za-z_-]+)`)
	d.ProgramNo = extractFirstMatch(content, `(?im)PROGRAM\s*#:\s*([0-9A-Za-z_-]+)`)
	d.CountryOfOrigin = extractFirstMatch(content, `(?im)COUNTRY\s+OF\s+ORIGIN:\s*([^\n]+?)(?:\s{2,}|DEPT:|$)`)
	d.Dept = extractFirstMatch(content, `(?im)DEPT(?:\s*#)?:\s*([0-9A-Za-z_-]+)`)
	d.SupplierNo = extractFirstMatch(content, `(?im)SUPPLIER\s*#:\s*([0-9A-Za-z_-]+)`)
	d.PLMNo = extractFirstMatch(content, `(?im)PLM\s*#:\s*([0-9A-Za-z_-]+)`)
	d.FactoryID = extractFirstMatch(content, `(?im)FACTORY\s+ID:\s*([0-9A-Za-z_-]+)`)
	d.Fineline = extractFirstMatch(content, `(?im)FINELINE:\s*([0-9A-Za-z_-]+)`)
	d.RetailPrice = extractFirstMatch(content, `(?im)RETAIL(?:\s+PRICE)?:\s*([\$€£]?[0-9.,]+)`)
	d.Customer = extractFirstMatch(content, `(?im)CUSTOMER:\s*([A-Za-z0-9 _-]+?)(?:\s{2,}|GOODS\s+SUPPLIER:|$)`)
	d.Buyer = extractFirstMatch(content, `(?im)BUYER:\s*([A-Za-z0-9 _-]+?)(?:\s{2,}|SHIP\s+TO:|$)`)
	d.BrandManager = extractFirstMatch(content, `(?im)BR(?:A)?ND\s+M(?:A)?N(?:A)?G(?:E)?R:\s*([A-Za-z0-9 _-]+?)(?:\s{2,}|PORT:|$)`)
	d.VendorCode = extractFirstMatch(content, `(?im)VENDOR(?:\s+CODE)?:\s*([^\n]+?)(?:\s{2,}(?:RETAIL:|$))`)
	d.GoodsSupplier = extractFirstMatch(content, `(?im)GOODS\s+SUPPLIER(?:\s+CODE)?:\s*([^\n]+?)(?:\s{2,}(?:REP\s+CODE:|$))`)
	d.ShipTo = extractFirstMatch(content, `(?im)SHIP\s+TO:\s*([^\n]+?)(?:\s{2,}(?:ITEM\s+DESC:|$))`)
	d.PortOfEntry = extractFirstMatch(content, `(?im)PORT:\s*([0-9A-Za-z_-]+)`)
	d.Warehouse = extractFirstMatch(content, `(?im)(?:WHSE|STORAGE\s+LOCATION):\s*([0-9A-Za-z_-]+)`)
	d.ItemDesc = extractFirstMatch(content, `(?im)ITEM\s+DESC(?:RIPTION)?:\s*([^\n]+)`)
	d.MABD = extractFirstMatch(content, `(?im)MABD:\s*([0-9/-]+)`)
	d.FabricSpec = extractFirstMatch(content, `(?im)FABRIC:\s*([^\n]+?)(?:\s{2,}|HTS\s+CODE|$)`)
	d.HTSCode = extractFirstMatch(content, `(?im)HTS\s+CODE(?:/CAT)?:\s*([0-9A-Za-z_.-]+)`)
	d.FabricContent = extractFirstMatch(content, `(?im)CONTENT:\s*([^\n]+?)(?:\s{2,}|DUTY:|$)`)
	d.FabricWeightGSM = extractFirstMatch(content, `(?im)WEIGHT:\s*([0-9]+\s*(?:G/M2|GSM|OZ))`)
	d.FabricCategory = extractFirstMatch(content, `(?im)FABRIC\s+CAT(?:EGORY)?:\s*([A-Za-z0-9_-]+)`)
	for _, kv := range []struct{ key, pat string }{
		{"MAIN_SIZE_COO", `(?im)MAIN/SIZE/COO:\s*([A-Za-z0-9_-]+)`},
		{"JOKER_TAG", `(?im)JOKER\s+TAG:\s*([A-Za-z0-9_-]+)`},
		{"SIZE_STRIP", `(?im)SIZE\s+STRIP:\s*([^\n]+?)(?:\s{2,}|[0-9]+\.\s|$)`},
		{"CARE_LABEL", `(?im)CARE\s+LABEL:\s*([A-Za-z0-9_-]+)`},
		{"POLYBAG", `(?im)(?:RFID\s+)?POLYBAG:\s*([^\n]+)`},
		{"POLYBAG_STICKER", `(?im)(?:RFID\s+)?POLYBAG\s+STICKER:\s*([A-Za-z0-9_-]+)`},
		{"CARTON_STICKER", `(?im)Carton\s+Sticker:\s*(YES|NO)`},
	} {
		if val := extractFirstMatch(content, kv.pat); val != "" {
			d.Accessories[kv.key] = val
		}
	}
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		u := strings.ToUpper(trimmed)
		if strings.HasPrefix(u, "FOLDED ") ||
			strings.HasPrefix(u, "SIZES - ") ||
			strings.HasPrefix(u, "SIZES-") ||
			strings.HasPrefix(u, "PACK ") ||
			strings.HasPrefix(u, "EACH ") ||
			strings.Contains(u, "CARTON STICKER REQUIRED") {
			d.PackingRules = append(d.PackingRules, strings.Join(strings.Fields(trimmed), " "))
		}
	}
	return d
}

func extractGenericPOMaterials(ordinal int, d *buyerOrderDetails) []buyerBOMHint {
	if d == nil {
		return nil
	}
	hints := []buyerBOMHint{}
	if d.FabricSpec != "" {
		code := "PRIMARY-FABRIC"
		if m := regexp.MustCompile(`([A-Za-z0-9]{2,}#?[0-9A-Za-z_-]+)$`).FindStringSubmatch(strings.TrimSpace(d.FabricSpec)); len(m) == 2 {
			code = strings.ReplaceAll(m[1], "#", "-")
		}
		hints = append(hints, buyerBOMHint{
			Component:     "PO PRIMARY FABRIC",
			MaterialLabel: strings.TrimSpace(d.FabricSpec + " · " + d.FabricContent + " · " + d.FabricWeightGSM),
			SuggestedCode: code,
			Category:      "FABRIC",
			Section:       "PO Product Information",
			RawQuantity:   "0.000",
			QuantityState: "ZERO_UNVERIFIED",
			SourceOrdinal: ordinal,
			Page:          ordinal,
			Hint:          "Extracted from PO Product Information. Enter verified per-garment fabric consumption and UOM before staging.",
		})
	}
	for _, key := range []string{"MAIN_SIZE_COO", "JOKER_TAG", "SIZE_STRIP", "CARE_LABEL", "POLYBAG", "POLYBAG_STICKER"} {
		val := d.Accessories[key]
		if val == "" {
			continue
		}
		code := strings.Fields(val)[0]
		hints = append(hints, buyerBOMHint{
			Component:     "PO ACCESSORY · " + strings.ReplaceAll(key, "_", " "),
			MaterialLabel: val,
			SuggestedCode: code,
			Category:      "TRIM",
			Section:       "PO Accessory Information",
			RawQuantity:   "1.000",
			QuantityState: "SOURCE_QUANTITY_UNVERIFIED",
			SourceOrdinal: ordinal,
			Page:          ordinal,
			Hint:          "Extracted from PO Accessory Information. Confirm per-garment/polybag ratio and unit (PCS).",
		})
	}
	return hints
}

func extractPDFDrawings(ctx context.Context, tx *sql.Tx, tenant string, src buyerSource, roleLabel string, maxOut int) []buyerDrawing {
	if src.Format != "PDF" || maxOut <= 0 {
		return nil
	}
	var rawPDF []byte
	if err := tx.QueryRowContext(ctx, `SELECT original FROM document_sources_v65 WHERE tenant_id=$1 AND id=$2`, tenant, src.ID).Scan(&rawPDF); err != nil || len(rawPDF) == 0 {
		return nil
	}
	c, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	dir, err := os.MkdirTemp("", "agiparel-drawings-")
	if err != nil {
		return nil
	}
	defer os.RemoveAll(dir)
	pdfPath := filepath.Join(dir, "doc.pdf")
	if err = os.WriteFile(pdfPath, rawPDF, 0600); err != nil {
		return nil
	}
	prefix := filepath.Join(dir, "img")
	if err = exec.CommandContext(c, "pdfimages", "-png", "-p", pdfPath, prefix).Run(); err != nil {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	pageTitles := map[int]string{}
	for _, seg := range src.Segments {
		if seg.Kind == "PAGE" {
			pn := bomPageNumber(seg.Locator)
			firstLine := strings.TrimSpace(strings.SplitN(seg.Content, "\n", 2)[0])
			if idx := strings.Index(firstLine, "In Work"); idx > 0 {
				firstLine = strings.TrimSpace(firstLine[:idx])
			}
			if pn > 0 && firstLine != "" && pageTitles[pn] == "" {
				pageTitles[pn] = firstLine
			}
		}
	}
	type rawImg struct {
		page          int
		width, height int
		data          []byte
		hash          string
	}
	candidates := []rawImg{}
	hashCount := map[string]int{}
	pageNumRe := regexp.MustCompile(`^img-([0-9]+)-[0-9]+\.png$`)
	for _, e := range entries {
		m := pageNumRe.FindStringSubmatch(e.Name())
		if len(m) != 2 {
			continue
		}
		page, _ := strconv.Atoi(m[1])
		p := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(p)
		if err != nil || len(b) < 12000 || len(b) > 900000 {
			continue
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(b))
		if err != nil || cfg.Width < 260 || cfg.Height < 180 {
			continue
		}
		if cfg.Width > cfg.Height*3 || cfg.Height > cfg.Width*3 {
			continue
		}
		sum := sha256.Sum256(b)
		h := hex.EncodeToString(sum[:])
		hashCount[h]++
		candidates = append(candidates, rawImg{page: page, width: cfg.Width, height: cfg.Height, data: b, hash: h})
	}
	seenHash := map[string]bool{}
	out := []buyerDrawing{}
	for _, item := range candidates {
		if hashCount[item.hash] >= 3 || seenHash[item.hash] {
			continue
		}
		seenHash[item.hash] = true
		title := fmt.Sprintf("%s · Page %d", roleLabel, item.page)
		if t := pageTitles[item.page]; t != "" && !strings.Contains(strings.ToUpper(t), "CONFIRMED SALES ORDER") {
			title = fmt.Sprintf("%s (p.%d): %s", roleLabel, item.page, t)
		} else if roleLabel == "PO" {
			title = fmt.Sprintf("PO Product Sketch (Page %d)", item.page)
		}
		out = append(out, buyerDrawing{
			Title:   title,
			Source:  src.Filename,
			Page:    item.page,
			Width:   item.width,
			Height:  item.height,
			DataURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(item.data),
			SHA256:  item.hash[:16],
		})
		if len(out) >= maxOut {
			break
		}
	}
	return out
}

func readExistingBuyerDrafts(ctx context.Context, tx *sql.Tx, tenant, poDocID, bomDocID string) []buyerSavedDraft {
	rows, err := tx.QueryContext(ctx, `
		SELECT h.id::text, h.intake_id, o.po_number, o.style_code, o.ordered_qty,
		       h.source_digest, h.reviewed_by, h.review_note, h.captured_at, h.mapping
		FROM buyer_order_handoffs_v65c1 h
		JOIN order_intakes_v5 o ON o.tenant_id = h.tenant_id AND o.id = h.intake_id
		WHERE h.tenant_id = $1 AND (h.po_document_id = $2 OR h.bom_document_id = $3)
		ORDER BY h.captured_at DESC, h.id DESC LIMIT 25`, tenant, poDocID, bomDocID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []buyerSavedDraft{}
	for rows.Next() {
		var d buyerSavedDraft
		var rawMap []byte
		if err := rows.Scan(&d.HandoffID, &d.IntakeID, &d.PONumber, &d.StyleCode, &d.OrderedQty,
			&d.SourceDigest, &d.ReviewedBy, &d.ReviewNote, &d.CapturedAt, &rawMap); err != nil {
			continue
		}
		var savedReq buyerReviewRequest
		if json.Unmarshal(rawMap, &savedReq) == nil {
			d.SKUs = savedReq.SKUs
			d.Materials = savedReq.Materials
		}
		out = append(out, d)
	}
	return out
}

func findConflictingBuyerDraft(ctx context.Context, tx *sql.Tx, tenant, poDocID, poNumber, newDigest string) (*buyerSavedDraft, error) {
	var d buyerSavedDraft
	var rawMap []byte
	err := tx.QueryRowContext(ctx, `
		SELECT h.id::text, h.intake_id, o.po_number, o.style_code, o.ordered_qty,
		       h.source_digest, h.reviewed_by, h.review_note, h.captured_at, h.mapping
		FROM buyer_order_handoffs_v65c1 h
		JOIN order_intakes_v5 o ON o.tenant_id = h.tenant_id AND o.id = h.intake_id
		WHERE h.tenant_id = $1 AND (o.po_number = $2 OR h.po_document_id = $3 AND o.po_number = $2)
		ORDER BY h.captured_at DESC, h.id DESC LIMIT 1`, tenant, poNumber, poDocID).Scan(
		&d.HandoffID, &d.IntakeID, &d.PONumber, &d.StyleCode, &d.OrderedQty,
		&d.SourceDigest, &d.ReviewedBy, &d.ReviewNote, &d.CapturedAt, &rawMap)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if d.SourceDigest == newDigest {
		return nil, nil
	}
	var savedReq buyerReviewRequest
	if json.Unmarshal(rawMap, &savedReq) == nil {
		d.SKUs = savedReq.SKUs
		d.Materials = savedReq.Materials
	}
	return &d, nil
}

func computeBuyerDraftDiffs(saved buyerSavedDraft, current buyerReviewRequest) []buyerFieldDiff {
	diffs := []buyerFieldDiff{}
	if saved.StyleCode != current.StyleCode {
		diffs = append(diffs, buyerFieldDiff{
			Scope:          "Order Header",
			Key:            "Style Code",
			ColleagueValue: saved.StyleCode,
			YourValue:      current.StyleCode,
		})
	}
	savedSKUs := map[string]buyerSKU{}
	skuKeys := []string{}
	seenKey := map[string]bool{}
	for _, s := range saved.SKUs {
		k := s.Color + " · Size " + s.Size
		savedSKUs[k] = s
		if !seenKey[k] {
			seenKey[k] = true
			skuKeys = append(skuKeys, k)
		}
	}
	currSKUs := map[string]buyerSKU{}
	for _, s := range current.SKUs {
		k := s.Color + " · Size " + s.Size
		currSKUs[k] = s
		if !seenKey[k] {
			seenKey[k] = true
			skuKeys = append(skuKeys, k)
		}
	}
	for _, k := range skuKeys {
		a, okA := savedSKUs[k]
		b, okB := currSKUs[k]
		switch {
		case okA && !okB:
			diffs = append(diffs, buyerFieldDiff{Scope: "SKU Line", Key: k, ColleagueValue: fmt.Sprintf("%d pcs (UPC: %s)", a.Quantity, emptyDash(a.UPC)), YourValue: "Removed / Missing"})
		case !okA && okB:
			diffs = append(diffs, buyerFieldDiff{Scope: "SKU Line", Key: k, ColleagueValue: "Not present", YourValue: fmt.Sprintf("%d pcs (UPC: %s)", b.Quantity, emptyDash(b.UPC))})
		case a.Quantity != b.Quantity || a.UPC != b.UPC || a.SourceOrdinal != b.SourceOrdinal:
			diffs = append(diffs, buyerFieldDiff{
				Scope:          "SKU Line",
				Key:            k,
				ColleagueValue: fmt.Sprintf("%d pcs · UPC %s (src #%d)", a.Quantity, emptyDash(a.UPC), a.SourceOrdinal),
				YourValue:      fmt.Sprintf("%d pcs · UPC %s (src #%d)", b.Quantity, emptyDash(b.UPC), b.SourceOrdinal),
			})
		}
	}
	savedBOM := map[string]buyerMaterial{}
	bomKeys := []string{}
	seenBOM := map[string]bool{}
	for _, m := range saved.Materials {
		k := m.Category + " · " + m.SKU
		savedBOM[k] = m
		if !seenBOM[k] {
			seenBOM[k] = true
			bomKeys = append(bomKeys, k)
		}
	}
	currBOM := map[string]buyerMaterial{}
	for _, m := range current.Materials {
		k := m.Category + " · " + m.SKU
		currBOM[k] = m
		if !seenBOM[k] {
			seenBOM[k] = true
			bomKeys = append(bomKeys, k)
		}
	}
	for _, k := range bomKeys {
		a, okA := savedBOM[k]
		b, okB := currBOM[k]
		switch {
		case okA && !okB:
			diffs = append(diffs, buyerFieldDiff{Scope: "BOM Material", Key: k, ColleagueValue: fmt.Sprintf("%s %s", a.PerGarment, a.UOM), YourValue: "Removed / Missing"})
		case !okA && okB:
			diffs = append(diffs, buyerFieldDiff{Scope: "BOM Material", Key: k, ColleagueValue: "Not present", YourValue: fmt.Sprintf("%s %s", b.PerGarment, b.UOM)})
		case a.PerGarment != b.PerGarment || !strings.EqualFold(a.UOM, b.UOM) || a.SourceOrdinal != b.SourceOrdinal:
			diffs = append(diffs, buyerFieldDiff{
				Scope:          "BOM Material",
				Key:            k,
				ColleagueValue: fmt.Sprintf("%s %s (src #%d)", a.PerGarment, a.UOM, a.SourceOrdinal),
				YourValue:      fmt.Sprintf("%s %s (src #%d)", b.PerGarment, b.UOM, b.SourceOrdinal),
			})
		}
	}
	return diffs
}

func emptyDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "none"
	}
	return s
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
		techHints, techCoverage, techIssues := detectTechPackBOM(bom.Segments)
		drawings := extractPDFDrawings(ctx, tx, tenant, po, "PO", 4)
		if req.BOMDocumentID != req.PODocumentID {
			drawings = append(drawings, extractPDFDrawings(ctx, tx, tenant, bom, "Tech Pack", 6)...)
		}
		existingDrafts := readExistingBuyerDrafts(ctx, tx, tenant, po.ID, bom.ID)
		issues = append(issues, bomIssues...)
		issues = append(issues, techIssues...)
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
		docJSON(w, 200, map[string]any{
			"po_source":               po,
			"bom_source":              bom,
			"po_options":              opts,
			"materials":               mats,
			"techpack_material_hints": techHints,
			"techpack_bom_coverage":   techCoverage,
			"drawings":                drawings,
			"existing_drafts":         existingDrafts,
			"issues":                  issues,
			"status":                  "UNREVIEWED",
			"release":                 "HOLD",
		})
		return
	}
	sub, err := prepareBuyerSubmission(req, po, bom)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	// Serialize concurrent saves for the same tenant + PO number so two colleagues
	// clicking Save simultaneously cannot race past the conflict detector.
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, tenant+"/po/"+strings.ToUpper(strings.TrimSpace(req.PONumber))); err != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	conflictDraft, err := findConflictingBuyerDraft(ctx, tx, tenant, po.ID, req.PONumber, sub.Report.SourceSHA256)
	if err != nil {
		writeBuyerSaveFailure(w, err, "conflict check")
		return
	}
	if conflictDraft != nil && strings.TrimSpace(req.SupersedeDigest) != conflictDraft.SourceDigest {
		diffs := computeBuyerDraftDiffs(*conflictDraft, req)
		docJSON(w, http.StatusConflict, map[string]any{
			"code":          "COLLEAGUE_DRAFT_CONFLICT",
			"message":       fmt.Sprintf("Colleague %q already saved a different draft for PO %s at %s. Review the %d discrepancy(s) below and coordinate before saving.", conflictDraft.ReviewedBy, conflictDraft.PONumber, conflictDraft.CapturedAt.UTC().Format("2006-01-02 15:04 UTC"), len(diffs)),
			"colleague":     conflictDraft,
			"discrepancies": diffs,
		})
		return
	}
	rep, reused, err := persistIntakeInTx(ctx, tx, tenant, sub)
	if err != nil {
		writeBuyerSaveFailure(w, err, "order draft")
		return
	}
	mapping, err := json.Marshal(req)
	if err != nil {
		http.Error(w, "Invalid mapping", 400)
		return
	}
	reviewedBy := strings.TrimSpace(r.Header.Get("X-Agiparel-Operator"))
	if reviewedBy == "" {
		if u, _, ok := r.BasicAuth(); ok && strings.TrimSpace(u) != "" {
			reviewedBy = strings.TrimSpace(u)
		}
	}
	if reviewedBy == "" {
		reviewedBy = os.Getenv("AGIPAREL_OPERATOR_USER")
	}
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
		writeBuyerSaveFailure(w, err, "document handoff")
		return
	}
	if err = tx.Commit(); err != nil {
		writeBuyerSaveFailure(w, err, "transaction commit")
		return
	}
	rep.Origin = "REVIEWED_DOCUMENT_MAPPING"
	docJSON(w, map[bool]int{true: 200, false: 201}[reused], map[string]any{"intake": rep, "handoff_id": handoffID, "reused": reused, "status": "PROPOSED", "release": "HOLD", "next": "/material-planning", "source_note": "The existing order and BOM quantities are proposals. Original sources remain in Document Intelligence."})
}

func buyerSaveError(err error) (int, string, string) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "42P01", "42703":
			return http.StatusServiceUnavailable, "Order database setup is incomplete. Ask an administrator to check migrations 0007 and 0014. No draft was saved.", pgErr.Code
		case "42501":
			return http.StatusServiceUnavailable, "The application cannot write to the order database. Ask an administrator to check the database role permissions. No draft was saved.", pgErr.Code
		case "23505":
			return http.StatusConflict, "This order conflicts with an existing saved record. Ask an administrator to inspect the conflicting entry. No new draft was saved.", pgErr.Code
		case "23503":
			switch pgErr.ConstraintName {
			case "order_intakes_v5_tenant_id_fkey", "buyer_order_handoffs_v65c1_tenant_id_fkey":
				return http.StatusServiceUnavailable, "Factory account setup is incomplete. Ask an administrator to verify the configured factory identity. No draft was saved.", pgErr.Code
			case "buyer_order_handoffs_v65c1_tenant_id_po_document_id_fkey", "buyer_order_handoffs_v65c1_tenant_id_bom_document_id_fkey":
				return http.StatusConflict, "A selected source document is not linked to this factory. Refresh the file list. No draft was saved.", pgErr.Code
			default:
				return http.StatusServiceUnavailable, "A required database link could not be verified. Ask an administrator to check the server log. No draft was saved.", pgErr.Code
			}
		case "23514", "22001", "22003", "22P02":
			return http.StatusConflict, "An order value does not fit the stored-data rules. Check the draft fields and try again. No draft was saved.", pgErr.Code
		case "57014":
			return http.StatusServiceUnavailable, "Saving timed out. Please try again. No draft was saved.", pgErr.Code
		}
		return http.StatusServiceUnavailable, "The order database rejected this draft. Ask an administrator to check the server log. No draft was saved.", pgErr.Code
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return http.StatusServiceUnavailable, "Saving timed out. Please try again. No draft was saved.", "deadline"
	}
	return http.StatusServiceUnavailable, "The order database could not save this draft. Ask an administrator to check the server log. No draft was saved.", "unknown"
}

func writeBuyerSaveFailure(w http.ResponseWriter, err error, stage string) {
	status, message, code := buyerSaveError(err)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		log.Printf("buyer-review: failed at %s (sqlstate=%s table=%s constraint=%s)", stage, code, pgErr.TableName, pgErr.ConstraintName)
	} else {
		log.Printf("buyer-review: failed at %s (class=%s type=%T)", stage, code, err)
	}
	http.Error(w, message, status)
}
