package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func testBuyerDoc(id, hash, filename string, segs []buyerEvidence) buyerSource {
	return buyerSource{ID: id, SHA256: hash, Filename: filename, Status: "EXTRACTED", Format: "CSV", Segments: segs}
}
func TestBuyerReviewMultiplePOsAndBOMMapping(t *testing.T) {
	po := []buyerEvidence{{1, "ROW", json.RawMessage(`{"row":1}`), `["PO Number","Style Code","Color Code","Size Code","Ordered Qty","UPC"]`}, {2, "ROW", json.RawMessage(`{"row":2}`), `["REP3","GM09Y094","NAVY","32","57600","0999"]`}, {3, "ROW", json.RawMessage(`{"row":3}`), `["REP4","GM09Y094","BLACK","34","72000","0998"]`}}
	opts, mats, issues := detectBuyerOptions(po)
	if len(opts) != 2 || len(mats) != 0 || len(issues) != 0 || opts[0].PONumber != "REP3" || opts[1].PONumber != "REP4" || opts[1].SKUs[0].Quantity != 72000 || opts[1].SKUs[0].SourceOrdinal != 3 {
		t.Fatalf("multi-PO extraction failed: %v %v %v", opts, mats, issues)
	}
	bom := []buyerEvidence{{1, "SHEET_ROW", json.RawMessage(`{"sheet":"BOM","row":1}`), "A1: style_code | B1: material_sku | C1: category | D1: qty_per_garment | E1: uom"}, {2, "SHEET_ROW", json.RawMessage(`{"sheet":"BOM","row":2}`), "A2: GM09Y094 | B2: TWILL | C2: FABRIC | D2: 1.850 | E2: M"}, {3, "SHEET_ROW", json.RawMessage(`{"sheet":"BOM","row":3}`), "A3: OTHER | B3: ZIP | C3: TRIM | D3: 1.000 | E3: PCS"}}
	_, mats, issues = detectBuyerOptions(bom)
	if len(mats) != 2 || mats[0].StyleCode != "GM09Y094" || mats[1].StyleCode != "OTHER" || len(issues) != 0 {
		t.Fatalf("BOM extraction mismatch: %v %v", mats, issues)
	}
	req := buyerReviewRequest{PONumber: "REP3", StyleCode: "GM09Y094", SKUs: opts[0].SKUs, Materials: mats, ReviewNote: "Verified selected PO and BOM against originals", Acknowledged: true}
	srcPO := testBuyerDoc("DOC-"+strings.Repeat("1", 32), strings.Repeat("a", 64), "real-buyer.pdf", po)
	srcBOM := testBuyerDoc("DOC-"+strings.Repeat("2", 32), strings.Repeat("b", 64), "real-tech-pack.xlsx", bom)
	if _, err := prepareBuyerSubmission(req, srcPO, srcBOM); err == nil || !strings.Contains(err.Error(), "different style") {
		t.Fatalf("mixed-style BOM not rejected: %v", err)
	}
	req.Materials = mats[:1]
	sub, err := prepareBuyerSubmission(req, srcPO, srcBOM)
	if err != nil {
		t.Fatal(err)
	}
	if sub.Report.PONumber != "REP3" || sub.Report.OrderedQty != 57600 || sub.Report.Materials[0].Required != "106560.000" || sub.Report.Readiness != "HOLD" || sub.Report.Origin != "REVIEWED_DOCUMENT_MAPPING" {
		t.Fatalf("bad staged review: %+v", sub.Report)
	}
	clone, err := prepareBuyerSubmission(req, srcPO, srcBOM)
	if err != nil || sub.Report.SourceSHA256 != clone.Report.SourceSHA256 {
		t.Fatal("source mapping not idempotent")
	}
	req.SKUs[0].SourceOrdinal = 0
	changed, err := prepareBuyerSubmission(req, srcPO, srcBOM)
	if err != nil || changed.Report.SourceSHA256 == sub.Report.SourceSHA256 {
		t.Fatal("evidence provenance changes must change mapping digest")
	}
}
func TestBuyerReviewValidationGates(t *testing.T) {
	po := testBuyerDoc("DOC-"+strings.Repeat("1", 32), strings.Repeat("a", 64), "buyer.csv", []buyerEvidence{{2, "ROW", json.RawMessage(`{"row":2}`), `["..."]`}})
	bom := testBuyerDoc("DOC-"+strings.Repeat("2", 32), strings.Repeat("b", 64), "bom.csv", []buyerEvidence{{4, "ROW", json.RawMessage(`{"row":4}`), `["..."]`}})
	base := buyerReviewRequest{PONumber: "REP3", StyleCode: "GM09Y094", SKUs: []buyerSKU{{Color: "NAVY", Size: "M", Quantity: 10, SourceOrdinal: 2}}, Materials: []buyerMaterial{{SKU: "TWILL", Category: "FABRIC", PerGarment: "1.850", UOM: "M", SourceOrdinal: 4}}, ReviewNote: "Verified values against uploaded originals", Acknowledged: true}
	tests := map[string]func(*buyerReviewRequest){
		"ack":              func(r *buyerReviewRequest) { r.Acknowledged = false },
		"note":             func(r *buyerReviewRequest) { r.ReviewNote = "" },
		"fake-po-ordinal":  func(r *buyerReviewRequest) { r.SKUs[0].SourceOrdinal = 999 },
		"fake-bom-ordinal": func(r *buyerReviewRequest) { r.Materials[0].SourceOrdinal = 999 },
		"negative-ordinal": func(r *buyerReviewRequest) { r.Materials[0].SourceOrdinal = -1 },
		"bad-quantity":     func(r *buyerReviewRequest) { r.SKUs[0].Quantity = 0 },
		"bad-unit":         func(r *buyerReviewRequest) { r.Materials[0].UOM = "=SQL" },
		"bad-precision":    func(r *buyerReviewRequest) { r.Materials[0].PerGarment = "1.8505" },
		"duplicate-po-sku": func(r *buyerReviewRequest) { r.SKUs = append(r.SKUs, r.SKUs[0]) },
		"no-fabric":        func(r *buyerReviewRequest) { r.Materials[0].Category = "TRIM" },
	}
	for name, edit := range tests {
		t.Run(name, func(t *testing.T) {
			x := base
			x.SKUs = append([]buyerSKU{}, base.SKUs...)
			x.Materials = append([]buyerMaterial{}, base.Materials...)
			edit(&x)
			if _, err := prepareBuyerSubmission(x, po, bom); err == nil {
				t.Fatal("invalid review accepted")
			}
		})
	}
	// Manual transcription allowed but remains proposed and distinct from cited evidence.
	base.SKUs[0].SourceOrdinal = 0
	sub, err := prepareBuyerSubmission(base, po, bom)
	if err != nil || sub.Report.Readiness != "HOLD" {
		t.Fatalf("manual reviewed values must be proposed: %v", err)
	}
}
func TestBuyerDocumentWorkflowClosedWithoutTenant(t *testing.T) {
	s := &EdgeServer{}
	r := httptest.NewRequest(http.MethodGet, "/api/v1/buyer-review", nil)
	w := httptest.NewRecorder()
	s.handleBuyerReview(w, r)
	if w.Code != 503 {
		t.Fatalf("without db/tenant must fail closed: %d", w.Code)
	}
	if got := buyerHeader(" Ordered QTY "); got != "ordered_qty" {
		t.Fatal(got)
	}
	vals, ok := buyerSegmentValues(buyerEvidence{Kind: "ROW", Content: `["a","b"]`})
	if !ok || !reflect.DeepEqual(vals, map[string]string{"0": "a", "1": "b"}) {
		t.Fatal(vals, ok)
	}
}

func TestBuyerWalmartPDFPageHint(t *testing.T) {
	page := buyerEvidence{Ordinal: 7, Kind: "PAGE", Content: `YSAG CONFIRMED SALES ORDER
 SALES INFORMATION
     REP3                 SO/PO#  / 12345
 PROJECT #: GM09Y094           VENDOR: 8015528
 COLOR & UNIT INFORMATION
 SALES ORDER ITEM INFO    30     32       UNITS     CASES
 MATERIAL # COLOR WM STYLE #
 1154701WM-BLACK SOFT SILVER TT3010003001F  7,200   9,600   16,800  700  $7.20
 TOTAL ALL COLORS 7,200 9,600 16,800
 `}
	result, warning := detectWalmartSalesOrder(page)
	if result == nil || warning == "" || result.PONumber != "REP3" || result.StyleCode != "GM09Y094" || len(result.SKUs) != 2 || result.SKUs[1].Quantity != 9600 || result.SKUs[0].Color != "SOFT_SILVER" {
		t.Fatalf("failed explicit PDF page mapping: %+v %q", result, warning)
	}
	bad := page
	bad.Content = strings.Replace(bad.Content, "16,800  700", "16,900  700", 1)
	invalid, _ := detectWalmartSalesOrder(bad)
	if invalid != nil {
		t.Fatal("misreconciled page may not generate size rows")
	}
}
