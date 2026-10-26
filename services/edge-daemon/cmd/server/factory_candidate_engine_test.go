package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func src(n int, kind, content, sheet string) factoryEvidenceSegment {
	loc := map[string]any{"row": n}
	if sheet != "" {
		loc["sheet"] = sheet
	}
	raw, _ := json.Marshal(loc)
	return factoryEvidenceSegment{Ordinal: n, Kind: kind, Locator: raw, Content: content}
}
func TestFactoryCandidateTypedTableRows(t *testing.T) {
	doc := "DOC-0123456789abcdef0123456789abcdef"
	cases := []struct {
		name, filename string
		segs           []factoryEvidenceSegment
		expected       string
		key            string
		has            string
	}{
		{"machines", "equipment.xlsx", []factoryEvidenceSegment{src(1, "SHEET_ROW", "A1: Machine ID | B1: Machine Type | C1: Status", "Equipment"), src(2, "SHEET_ROW", "A2: MC001 | B2: Lockstitch | C2: Working", "Equipment")}, "MACHINE", "MC001", "machine_type"},
		{"skills", "skill-matrix.csv", []factoryEvidenceSegment{src(1, "ROW", `["worker_id","skill","level"]`, ""), src(2, "ROW", `["E-17","Bartack","A"]`, "")}, "SKILL", "E-17:Bartack", "skill"},
		{"inventory", "opening-stock.xlsx", []factoryEvidenceSegment{src(1, "SHEET_ROW", "A1: Material Code | B1: Stock Qty | C1: UOM", "Stock"), src(2, "SHEET_ROW", "A2: FAB-01 | B2: 123.250 | C2: m", "Stock")}, "MATERIAL", "FAB-01:m", "quantity"},
		{"rates", "factory-rates.csv", []factoryEvidenceSegment{src(1, "ROW", `["cost_type","rate","currency","unit"]`, ""), src(2, "ROW", `["Direct labor","2.375","USD","hour"]`, "")}, "RATE", "Direct labor:USD", "rate"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := factoryProposalsFromSegments(doc, tc.filename, tc.segs)
			if len(got) != 1 {
				t.Fatalf("got %d proposals: %+v", len(got), got)
			}
			if got[0].EntityType != tc.expected || got[0].EntityKey != tc.key {
				t.Fatalf("wrong typed proposal %+v", got[0])
			}
			var attrs map[string]string
			if json.Unmarshal(got[0].Attributes, &attrs) != nil || attrs[tc.has] == "" {
				t.Fatalf("missing attribute %q: %+v", tc.has, attrs)
			}
			if got[0].SourceOrdinal != 2 || len(got[0].Fingerprint) != 64 {
				t.Fatalf("missing provenance %+v", got[0])
			}
		})
	}
}
func TestFactoryCandidateStrictNoSilentBusinessMeaning(t *testing.T) {
	doc := "DOC-0123456789abcdef0123456789abcdef"
	tests := []struct {
		name, file string
		segments   []factoryEvidenceSegment
	}{
		{"buyer PO not factory register", "buyer_po.csv", []factoryEvidenceSegment{src(1, "ROW", `["material_code","stock_qty","uom"]`, ""), src(2, "ROW", `["FAB1","300","m"]`, "")}},
		{"BOM not stock", "bom.csv", []factoryEvidenceSegment{src(1, "ROW", `["material_code","qty","uom"]`, ""), src(2, "ROW", `["FAB1","300","m"]`, "")}},
		{"absent machine type", "machines.csv", []factoryEvidenceSegment{src(1, "ROW", `["machine_id","machine_type"]`, ""), src(2, "ROW", `["M1",""]`, "")}},
		{"personal name not employee ID", "skills.csv", []factoryEvidenceSegment{src(1, "ROW", `["employee_name","skill"]`, ""), src(2, "ROW", `["Jane Doe","Cutting"]`, "")}},
		{"ambiguous currency", "rates.csv", []factoryEvidenceSegment{src(1, "ROW", `["cost_type","rate"]`, ""), src(2, "ROW", `["Labor","4.5"]`, "")}},
		{"formula as inventory qty", "stock.csv", []factoryEvidenceSegment{src(1, "ROW", `["material_code","stock_qty","uom"]`, ""), src(2, "ROW", `["FAB01","=SUM(A1:A3) [cached: 20]","m"]`, "")}},
		{"invalid inventory qty", "stock.csv", []factoryEvidenceSegment{src(1, "ROW", `["material_code","stock_qty","uom"]`, ""), src(2, "ROW", `["FAB01","-25","m"]`, "")}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := factoryProposalsFromSegments(doc, tc.file, tc.segments); len(got) != 0 {
				t.Fatalf("unsafe inference %+v", got)
			}
		})
	}
}
func TestFactoryExplicitFactsNotInvented(t *testing.T) {
	doc := "DOC-0123456789abcdef0123456789abcdef"
	got := factoryProposalsFromSegments(doc, "factory-notes.txt", []factoryEvidenceSegment{
		src(1, "LINE", "Sewing machines: 125", ""),
		src(2, "LINE", "Shift hours: 8.5", ""),
		src(3, "LINE", "Labor rate per hour: 25.1250", ""),
		src(4, "LINE", "Workforce headcount approximately 80", ""),
		src(5, "LINE", "Working days per week: 8", ""),
		src(6, "LINE", "Labor rate per hour: 1e10", ""),
	})
	if len(got) != 3 {
		t.Fatalf("wanted 3 explicitly labelled facts, got %+v", got)
	}
	for _, p := range got {
		if p.EntityType != "FIELD" || p.FieldKey == "" {
			t.Fatalf("not an explicit field %+v", p)
		}
	}
}
func TestFactoryCandidatesSourceIdempotenceAndBounds(t *testing.T) {
	doc := "DOC-0123456789abcdef0123456789abcdef"
	seg := src(1, "LINE", "Sewing machines: 3", "")
	a := factoryProposalsFromSegments(doc, "factory.txt", []factoryEvidenceSegment{seg})
	b := factoryProposalsFromSegments(doc, "factory.txt", []factoryEvidenceSegment{seg})
	if a[0].Fingerprint != b[0].Fingerprint {
		t.Fatal("not deterministic")
	}
	if a[0].Fingerprint == factoryProposalsFromSegments("DOC-ffffffffffffffffffffffffffffffff", "factory.txt", []factoryEvidenceSegment{seg})[0].Fingerprint {
		t.Fatal("document identity absent from fingerprint")
	}
	segments := []factoryEvidenceSegment{}
	for i := 1; i <= 1000; i++ {
		segments = append(segments, src(i, "LINE", fmt.Sprintf("Sewing machines: %d", i), ""))
	}
	got := factoryProposalsFromSegments(doc, "factory.txt", segments)
	if len(got) > maxCandidateProposals {
		t.Fatalf("unbounded suggestions %d", len(got))
	}
	if !strings.HasPrefix(candidateKindLabel("MATERIAL"), "Inventory") {
		t.Fatal("incorrect type label")
	}
}
func TestCandidateIdentifierValidation(t *testing.T) {
	if !validCandidateID("FC-0123456789abcdef0123456789abcdef") || validCandidateID("FC-0123456789abcdef0123456789abcdeg") || validDocSourceID("DOC-../../etc/passwd") {
		t.Fatal("identifier validation error")
	}
}
