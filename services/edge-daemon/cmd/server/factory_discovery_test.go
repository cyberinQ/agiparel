package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFactoryCanonicalValues(t *testing.T) {
	cases := []struct {
		key, in string
		valid   bool
	}{
		{"machines.sewing_count", "15", true}, {"machines.sewing_count", "-1", false},
		{"machines.sewing_count", "3.5", false}, {"machines.sewing_count", "1000001", false},
		{"workforce.headcount", "unknown", false}, {"costs.labor_rate", "27.125", true},
		{"costs.labor_rate", "1e10", false}, {"costs.labor_rate", "-2", false},
		{"calendar.shift_hours", "8.50", true}, {"calendar.shift_hours", "25", false},
		{"calendar.shift_hours", "0", false}, {"calendar.workdays_week", "7", true},
		{"calendar.workdays_week", "8", false}, {"materials.stock_status", "INSPECTED", true},
		{"materials.stock_status", "AVAILABLE", false}, {"costs.currency", "inr", true},
		{"costs.currency", "12A", false}, {"factory.name", "Noble Garments", true},
		{"factory.name", "garments\x00", false}, {"factory.name", "\n", false},
	}
	for _, c := range cases {
		s, ok := factorySpec(c.key)
		if !ok {
			t.Fatalf("missing spec %s", c.key)
		}
		_, _, e := canonicalFactoryValue(s, c.in)
		if (e == nil) != c.valid {
			t.Errorf("%s %q valid=%t error=%v", c.key, c.in, c.valid, e)
		}
	}
	if _, ok := factorySpec("buyer.order_approved"); ok {
		t.Fatal("unsafe arbitrary field accepted")
	}
}
func TestFactoryReadinessSeparatesProvisionalFromConfirmed(t *testing.T) {
	f := map[string]factoryFact{
		"machines.sewing_count": {Value: "40", State: "PROVISIONAL"},
		"workforce.headcount":   {Value: "50", State: "CONFIRMED"},
		"workforce.skills":      {Value: "sewing", State: "CONFIRMED"},
		"calendar.shift_hours":  {Value: "8", State: "CONFIRMED"},
	}
	result, question, known, confirmed := factorySuggestions(f)
	if known != 4 || confirmed != 3 {
		t.Fatalf("known=%d confirmed=%d", known, confirmed)
	}
	if !strings.Contains(question, "factory") {
		t.Errorf("unexpected next question %q", question)
	}
	for _, r := range result {
		if r.Key == "capacity_baseline" && (r.Status != "NEEDS_CONFIRMATION" || len(r.Missing) != 1) {
			t.Fatalf("provisional machine count unlocked capacity %+v", r)
		}
		if (r.Key == "production_release" || r.Key == "material_issue") && r.Status != "NOT_AUTHORIZED" {
			t.Fatalf("unsafe gate opened %+v", r)
		}
	}
}
func TestFactoryMaterialStockRequiresInspected(t *testing.T) {
	f := map[string]factoryFact{
		"materials.categories":   {Value: "Cotton", State: "CONFIRMED"},
		"materials.stock_status": {Value: "COUNTED", State: "CONFIRMED"},
	}
	result, _, _, _ := factorySuggestions(f)
	for _, r := range result {
		if r.Key == "material_baseline" {
			if r.Status != "NEEDS_CONFIRMATION" {
				t.Fatal("counted stock treated as inspected")
			}
			return
		}
	}
	t.Fatal("material readiness missing")
}
func TestFactoryDocumentSuggestionsNotAuthority(t *testing.T) {
	area, _ := factoryDocumentHint("sewing machine list.xlsx", "")
	if area != "Machines" {
		t.Fatalf("machine classification %q", area)
	}
	area, _ = factoryDocumentHint("employee skill matrix.xlsx", "")
	if area != "People & skills" {
		t.Fatalf("skills classification %q", area)
	}
	area, _ = factoryDocumentHint("something.bin", "")
	if area != "Unclassified" {
		t.Fatalf("unknown falsely classified %q", area)
	}
	area, _ = factoryDocumentHint("machine list inventory.xlsx", "")
	if area != "Unclassified" {
		t.Fatalf("ambiguous file falsely claimed as verified %q", area)
	}
}
func TestFactoryHTTPMethodAndFailClosed(t *testing.T) {
	s := &EdgeServer{}
	w := httptest.NewRecorder()
	s.handleFactorySetup(w, httptest.NewRequest(http.MethodGet, "/factory-setup", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("factory setup HTTP=%d", w.Code)
	}
	// The copy is intentionally editable. Verify the actual factory setup controls.
	for _, required := range []string{`id="topics"`, `id="fact-form"`, `id="candidate-source"`, `id="factory-references"`} {
		if !strings.Contains(w.Body.String(), required) {
			t.Errorf("factory setup missing %q", required)
		}
	}
	w = httptest.NewRecorder()
	s.handleFactoryFact(w, httptest.NewRequest(http.MethodGet, "/api/v1/factory-onboarding/facts", nil))
	if w.Code != 405 {
		t.Fatalf("unexpected mutation GET status %d", w.Code)
	}
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_OPERATOR_TENANT", "")
	w = httptest.NewRecorder()
	s.handleFactoryDiscovery(w, httptest.NewRequest(http.MethodGet, "/api/v1/factory-onboarding", nil))
	if w.Code != 503 {
		t.Fatalf("missing tenant returned %d", w.Code)
	}
}

func TestFactoryZeroIsNotUnknownButCannotProvideCapacity(t *testing.T) {
	facts := map[string]factoryFact{
		"machines.sewing_count": {Value: "0", State: "CONFIRMED"},
		"workforce.headcount":   {Value: "0", State: "CONFIRMED"},
		"workforce.skills":      {Value: "lockstitch", State: "CONFIRMED"},
		"calendar.shift_hours":  {Value: "8", State: "CONFIRMED"},
	}
	results, _, known, confirmed := factorySuggestions(facts)
	if known != 4 || confirmed != 4 {
		t.Fatal("zero should be known/confirmed, not missing")
	}
	for _, r := range results {
		if r.Key == "capacity_baseline" {
			if r.Status == "FOUNDATION_PRESENT" || len(r.Missing) != 2 {
				t.Fatalf("zero capacity incorrectly accepted %+v", r)
			}
			return
		}
	}
	t.Fatal("capacity baseline not evaluated")
}
