package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func techPackPage(lines ...string) buyerEvidence {
	return buyerEvidence{Ordinal: 7, Kind: "PAGE", Locator: json.RawMessage(`{"page":6}`), Content: strings.Join(lines, "\n")}
}
func testHeader() string {
	return "                   Component or           Material            Placement      Quantity           Comments   Reference No."
}
func TestTechpackBOMNeverConvertsZeroToConsumption(t *testing.T) {
	rows, cover, issues := detectTechPackBOM([]buyerEvidence{techPackPage(
		"006 : Trousers -- BOM: Colorway - Material Color",
		"    Section: Fabric                                                                006 : Trousers F21 DESIGN      BOM: Colorway - Material Color",
		testHeader(),
		"                   FABRIC 1=BODY             60570-73%           SEE MODEL                0.000   HT-   FT35430",
		"                                            Polyester, 21%      IMAGES",
		"    Section: Trims                                                       006 : Trousers F21 DESIGN        BOM: Colorway - Material Color",
		testHeader(),
		"                   24L BUTTON:              24052-4 hole       SEE MODEL                3.000   20, 22, 24",
	)})
	if len(rows) != 2 {
		t.Fatalf("rows=%d: %+v issues=%v", len(rows), rows, issues)
	}
	if cover.BOMPages != 1 || cover.WithoutVerifiedConsumption != 2 {
		t.Fatalf("cover %+v", cover)
	}
	if rows[0].Category != "FABRIC" || rows[0].QuantityState != "ZERO_UNVERIFIED" || rows[0].SuggestedCode != "60570" {
		t.Fatalf("fabric %+v", rows[0])
	}
	if rows[1].Category != "TRIM" || rows[1].RawQuantity != "3.000" || rows[1].QuantityState != "SOURCE_QUANTITY_UNVERIFIED" {
		t.Fatalf("trim %+v", rows[1])
	}
	if rows[0].Page != 6 || rows[0].SourceOrdinal != 7 {
		t.Fatalf("source lost: %+v", rows[0])
	}
}
func TestTechpackBOMIgnoresUnstructuredNoise(t *testing.T) {
	rows, coverage, issues := detectTechPackBOM([]buyerEvidence{techPackPage("A BILL OF MATERIALS", "size 30 32 34", "100 pcs")})
	if len(rows) != 0 || coverage.MaterialRows != 0 || len(issues) > 0 {
		t.Fatalf("%+v %+v %+v", rows, coverage, issues)
	}
}
func TestTechpackBOMNoInventedUnitOrRates(t *testing.T) {
	rows, _, _ := detectTechPackBOM([]buyerEvidence{techPackPage("BOM: Colorway - Material Color", "Section: Product Packaging      BOM: Colorway - Material Color", testHeader(), "                   SIZE / COO LABEL        71637--Label-       REFER TO                 1.000         LAB2464")})
	if len(rows) != 1 {
		t.Fatalf("%+v", rows)
	}
	text, _ := json.Marshal(rows[0])
	if strings.Contains(string(text), "per_garment") || strings.Contains(string(text), "uom") {
		t.Fatalf("fabricated unit: %s", text)
	}
}

func TestTechpackBOMReassemblesPageSegmentsAndPreservesExactOrdinal(t *testing.T) {
	prefix := "006 : Trousers -- BOM: Colorway - Material Color\n    Section: Fabric                                                                006 : Trousers F21 DESIGN      BOM: Colorway - Material Color\n" + testHeader() + "\n"
	row := "                   FABRIC 1=BODY             60570-73%           SEE MODEL                0.000   FT35430\n"
	// Split in the middle of a source row, as the document worker does for long pages.
	full := prefix + strings.Repeat(" ", 5989-len(prefix)) + "\n" + row
	parts := []buyerEvidence{{Ordinal: 21, Kind: "PAGE", Locator: json.RawMessage(`{"page":5}`), Content: full[:6000]}, {Ordinal: 22, Kind: "PAGE", Locator: json.RawMessage(`{"page":5}`), Content: full[6000:]}}
	result, cover, _ := detectTechPackBOM(parts)
	if len(result) != 1 || result[0].SourceOrdinal != 21 || result[0].Page != 5 || cover.BOMPages != 1 {
		t.Fatalf("split lost row: %+v %+v", result, cover)
	}
}
