// Author: Sunil Jain, 2026-10-27
// Verify source-independent unit normalization and refusal to guess non-garment ratios.
package main

import "testing"

func TestCanonicalBuyerUOMFromNonstandardFiles(t *testing.T) {
	cases := map[string]string{
		"m / pcs": "M", "m/pc": "M", "METRES PER GARMENT": "M",
		"meter / piece": "M", "M": "M", "yards / garments": "YD",
		"pcs / garment": "PCS", "Pieces": "PCS", "cm per pc": "CM",
		"kg / garment": "KG", "Pair": "PAIR", "ROLL": "ROLL", "CONE": "CONE",
		"CUSTOM-LOT": "CUSTOM-LOT",
	}
	for input, want := range cases {
		got, err := canonicalBuyerUOM(input)
		if err != nil || got != want {
			t.Errorf("%q: got %q err %v, want %q", input, got, err, want)
		}
	}
}

func TestCanonicalBuyerUOMRejectsUnsafeInterpretations(t *testing.T) {
	for _, raw := range []string{"", "m/kg", "kg / m", "M / pcs / set", "=SUM(A1)", "m / box", "kg / unit", "!!!!!!"} {
		if got, err := canonicalBuyerUOM(raw); err == nil {
			t.Errorf("unsafe unit %q accepted as %q", raw, got)
		}
	}
}

func TestIntakeNormalizesUOMButRetainsOriginalSource(t *testing.T) {
	po := []byte("po_number,style_code,color_code,size_code,ordered_qty,upc\nREP3,GM09Y094,SOFT_SILVER,30,7200,\n")
	bom := []byte("style_code,material_sku,category,qty_per_garment,uom\nGM09Y094,009,FABRIC,1.850,m / pcs\n")
	proposal, err := parseIntake(po, bom, "buyer.csv", "bom.csv")
	if err != nil {
		t.Fatal(err)
	}
	if proposal.Report.Materials[0].UOM != "M" || proposal.Report.Materials[0].Required != "13320.000" {
		t.Fatalf("unexpected quantity or unit: %+v", proposal.Report.Materials[0])
	}
	if string(proposal.BOM) != string(bom) {
		t.Fatal("normalization must not overwrite buyer's original source")
	}
	bad := []byte("style_code,material_sku,category,qty_per_garment,uom\nGM09Y094,009,FABRIC,1.850,m/kg\n")
	if _, err := parseIntake(po, bad, "buyer.csv", "bad.csv"); err == nil {
		t.Fatal("ambiguous units must require review")
	}
}
