// Gate 06.5-B1: factory-first discovery, deliberately non-authoritative.
// Stores only explicitly entered human observations; never writes master data,
// inventory balances, production plans, personnel records or buyer orders.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/internal/web"
)

type factoryFieldSpec struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Group string `json:"group"`
	Kind  string `json:"kind"`
	Help  string `json:"help"`
	Unit  string `json:"unit,omitempty"`
}

// The field registry is intentionally narrow and semantic. Unlike a generic
// extracted-text-to-SQL pipeline, every field has validated business meaning.
var factoryFieldSpecs = []factoryFieldSpec{
	{"factory.name", "Factory name", "Factory", "TEXT", "What should we call your factory?", ""},
	{"factory.location", "Site or city", "Factory", "TEXT", "Where is this unit located? An approximate city is fine.", ""},
	{"factory.product_types", "Product types", "Factory", "TEXT", "What do you make (for example, woven trousers)?", ""},
	{"machines.sewing_count", "Sewing machine count", "Machines", "INTEGER", "An estimate is fine to begin. Count working sewing machines, not all equipment.", "machines"},
	{"machines.capabilities", "Machine capabilities", "Machines", "TEXT", "For example lockstitch, overlock, bartack, buttonhole, CAD cutting.", ""},
	{"machines.maintenance", "Machine availability notes", "Machines", "TEXT", "Note machines awaiting repair or inspection.", ""},
	{"workforce.headcount", "Production employee count", "People & skills", "INTEGER", "An approximate active production headcount is acceptable.", "people"},
	{"workforce.skills", "Skills and certifications", "People & skills", "TEXT", "Which critical operations are covered? Avoid employee names in this development demo.", ""},
	{"workforce.shifts", "Shift pattern", "People & skills", "TEXT", "For example one day shift, two shifts or variable shifts.", ""},
	{"materials.categories", "Raw material categories", "Materials & stores", "TEXT", "For example woven fabric, buttons, zippers, interlining and labels.", ""},
	{"materials.stock_status", "Opening inventory status", "Materials & stores", "CHOICE", "Do you have a counted, inspected opening balance?", ""},
	{"costs.currency", "Costing currency", "Costs & rates", "CURRENCY", "Three-letter ISO currency, e.g. INR or USD.", ""},
	{"costs.labor_rate", "Labor cost per hour", "Costs & rates", "DECIMAL", "Estimated hourly direct labor cost; currency entered separately.", "per hour"},
	{"costs.overhead_rate", "Overhead per hour", "Costs & rates", "DECIMAL", "Estimated factory overhead rate per production hour.", "per hour"},
	{"calendar.shift_hours", "Hours per shift", "Calendar", "DECIMAL", "Scheduled hours in a normal shift (not guaranteed effective capacity).", "hours"},
	{"calendar.workdays_week", "Working days per week", "Calendar", "INTEGER", "Normally 1–7 days per week.", "days"},
	{"quality.inspection_policy", "Quality inspection practice", "Quality", "TEXT", "Describe your inline and final inspection process.", ""},
}

var factoryDecimal = regexp.MustCompile(`^(0|[1-9][0-9]{0,8})(\.[0-9]{1,4})?$`)
var factoryCurrency = regexp.MustCompile(`^[A-Z]{3}$`)

func factorySpec(key string) (factoryFieldSpec, bool) {
	for _, f := range factoryFieldSpecs {
		if f.Key == key {
			return f, true
		}
	}
	return factoryFieldSpec{}, false
}

func canonicalFactoryValue(spec factoryFieldSpec, raw string) (string, json.RawMessage, error) {
	v := strings.TrimSpace(raw)
	if v == "" || len(v) > 2000 || strings.IndexByte(v, 0) >= 0 || strings.ContainsAny(v, "\r") {
		return "", nil, errors.New("enter a value (maximum 2,000 characters)")
	}
	switch spec.Kind {
	case "INTEGER":
		n, e := strconv.ParseInt(v, 10, 32)
		if e != nil || n < 0 || n > 1000000 {
			return "", nil, errors.New("enter a whole number from 0 to 1,000,000")
		}
		if spec.Key == "calendar.workdays_week" && (n < 1 || n > 7) {
			return "", nil, errors.New("working days must be 1–7")
		}
		v = strconv.FormatInt(n, 10)
		b, _ := json.Marshal(map[string]any{"integer": n, "unit": spec.Unit})
		return v, b, nil
	case "DECIMAL":
		if !factoryDecimal.MatchString(v) {
			return "", nil, errors.New("enter a nonnegative decimal, up to four places")
		}
		if spec.Key == "calendar.shift_hours" {
			hours, _ := strconv.ParseFloat(v, 64)
			if hours <= 0 || hours > 24 {
				return "", nil, errors.New("shift hours must be above zero and no more than 24")
			}
		}
		b, _ := json.Marshal(map[string]any{"decimal": v, "unit": spec.Unit})
		return v, b, nil
	case "CURRENCY":
		v = strings.ToUpper(v)
		if !factoryCurrency.MatchString(v) {
			return "", nil, errors.New("enter a three-letter currency code")
		}
		b, _ := json.Marshal(map[string]any{"currency": v})
		return v, b, nil
	case "CHOICE":
		switch v {
		case "UNKNOWN", "ESTIMATED", "COUNTED", "INSPECTED":
		default:
			return "", nil, errors.New("select a listed inventory status")
		}
		b, _ := json.Marshal(map[string]any{"choice": v})
		return v, b, nil
	case "TEXT":
		for _, c := range v {
			if c < 32 && c != '\n' && c != '\t' {
				return "", nil, errors.New("remove control characters")
			}
		}
		b, _ := json.Marshal(map[string]any{"text": v})
		return v, b, nil
	}
	return "", nil, errors.New("unsupported field type")
}

type factoryFact struct {
	FieldKey         string    `json:"field_key"`
	Value            string    `json:"value"`
	State            string    `json:"state"`
	Revision         int       `json:"revision"`
	SourceDocumentID string    `json:"source_document_id,omitempty"`
	Note             string    `json:"note,omitempty"`
	RecordedAt       time.Time `json:"recorded_at"`
}

type factorySubmit struct {
	FieldKey         string `json:"field_key"`
	Value            string `json:"value"`
	State            string `json:"state"`
	SourceDocumentID string `json:"source_document_id"`
	Note             string `json:"note"`
	ExpectedRevision int    `json:"expected_revision"`
}

type factoryReadiness struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Status      string   `json:"status"`
	Missing     []string `json:"missing"`
	Explanation string   `json:"explanation"`
}

type factoryDocHint struct {
	ID            string `json:"id"`
	Filename      string `json:"filename"`
	Status        string `json:"status"`
	SuggestedArea string `json:"suggested_area"`
	Reason        string `json:"reason"`
}

// factoryGuidedQuestions returns a single canonical question order for the UI.
// Incomplete/provisional facts remain visible without treating them as approved.
type factoryGuidedQuestion struct {
	FieldKey string `json:"field_key"`
	Question string `json:"question"`
}

func factoryGuidedQuestions(facts map[string]factoryFact) []factoryGuidedQuestion {
	priority := []factoryGuidedQuestion{
		{"factory.name", "What is your factory called?"},
		{"machines.sewing_count", "About how many working sewing machines do you have? An estimate is okay."},
		{"workforce.headcount", "Approximately how many people work in production?"},
		{"machines.capabilities", "Which operations can your machines perform?"},
		{"workforce.skills", "Which sewing or finishing skills are available?"},
		{"materials.categories", "What raw materials do you normally keep?"},
		{"calendar.shift_hours", "How many hours are in a typical shift?"},
		{"costs.currency", "Which currency do you use for factory costing?"},
		{"costs.labor_rate", "What is your approximate hourly direct labor rate?"},
		{"costs.overhead_rate", "Do you know your approximate factory overhead per hour?"},
	}
	questions := make([]factoryGuidedQuestion, 0, len(factoryFieldSpecs))
	seen := make(map[string]bool, len(factoryFieldSpecs))
	for _, q := range priority {
		seen[q.FieldKey] = true
		if facts[q.FieldKey].State != "CONFIRMED" {
			questions = append(questions, q)
		}
	}
	for _, spec := range factoryFieldSpecs {
		if !seen[spec.Key] && facts[spec.Key].State != "CONFIRMED" {
			questions = append(questions, factoryGuidedQuestion{spec.Key, spec.Help})
		}
	}
	return questions
}

func factorySuggestions(facts map[string]factoryFact) ([]factoryReadiness, string, int, int) {
	confirmed := func(k string) bool { f, ok := facts[k]; return ok && f.State == "CONFIRMED" }
	known := 0
	verified := 0
	for _, sp := range factoryFieldSpecs {
		if f, ok := facts[sp.Key]; ok && f.Value != "" {
			known++
			if f.State == "CONFIRMED" {
				verified++
			}
		}
	}
	rules := []struct {
		key, label, reason string
		req                []string
	}{
		{"discover", "Document discovery", "Always available. Upload originals even when the factory is not yet documented.", nil},
		{"buyer_documents", "Buyer document capture", "You may receive buyers' documents now; releasing production is separate.", nil},
		{"capacity_baseline", "Draft capacity baseline", "Confirmed machine count, workforce, skill coverage and shift hours are needed for a useful initial estimate.", []string{"machines.sewing_count", "workforce.headcount", "workforce.skills", "calendar.shift_hours"}},
		{"costing_baseline", "Draft costing baseline", "Rates establish a starting point; buyer BOM, material costs and approval are additionally required for an actual quote.", []string{"costs.currency", "costs.labor_rate", "costs.overhead_rate"}},
		{"material_baseline", "Opening material picture", "Material categories and inspected stock status help initial planning; this does not authorize reservations or issuance.", []string{"materials.categories", "materials.stock_status"}},
	}
	out := make([]factoryReadiness, 0, len(rules)+2)
	for _, r := range rules {
		missing := []string{}
		for _, k := range r.req {
			if !confirmed(k) {
				sp, _ := factorySpec(k)
				missing = append(missing, sp.Label)
			} else if r.key == "capacity_baseline" && (k == "machines.sewing_count" || k == "workforce.headcount") && facts[k].Value == "0" {
				sp, _ := factorySpec(k)
				missing = append(missing, sp.Label+" is confirmed as zero; no active capacity")
			}
		}
		status := "FOUNDATION_PRESENT"
		if len(r.req) == 0 {
			status = "AVAILABLE"
		} else if len(missing) > 0 {
			status = "NEEDS_CONFIRMATION"
		}
		if r.key == "material_baseline" && len(missing) == 0 && facts["materials.stock_status"].Value != "INSPECTED" {
			missing = append(missing, "Stock requires verified inspection before it can support issuance")
			status = "NEEDS_CONFIRMATION"
		}
		out = append(out, factoryReadiness{r.key, r.label, status, missing, r.reason})
	}
	out = append(out, factoryReadiness{"production_release", "Production release", "NOT_AUTHORIZED", []string{"RFID commissioning, qualified stations and approved buyer/order facts"}, "Onboarding answers never authorize production release."})
	out = append(out, factoryReadiness{"material_issue", "Material issuance", "NOT_AUTHORIZED", []string{"Verified ledger, reserved rolls, two independent authenticated witnesses"}, "The issuance gate cannot be bypassed by marking an onboarding field confirmed."})
	question := "Your factory details are checked. Upload new evidence or revise them anytime."
	if questions := factoryGuidedQuestions(facts); len(questions) > 0 {
		question = questions[0].Question
	}
	return out, question, known, verified
}

// Suggestions are conservative filename/content hints; never classifier authority.
func factoryDocumentHint(name, snippet string) (string, string) {
	text := strings.ToLower(name + " " + snippet)
	categories := []struct {
		area string
		keys []string
	}{
		{"Machines", []string{"machine list", "machine inventory", "equipment", "lockstitch", "overlock", "sewing machine"}},
		{"People & skills", []string{"skill matrix", "worker roster", "employee list", "operator skill", "manpower", "attendance"}},
		{"Materials & stores", []string{"stock report", "inventory", "fabric roll", "grn", "raw material", "trims"}},
		{"Costs & rates", []string{"cost sheet", "labor rate", "overhead", "wages", "costing rate"}},
		{"Buyer & orders", []string{"purchase order", "tech pack", "packing instructions", "buyer po", "style no", "shipment"}},
	}
	max := 0
	area := "Unclassified"
	ties := false
	for _, c := range categories {
		score := 0
		for _, k := range c.keys {
			if strings.Contains(text, k) {
				score++
			}
		}
		if score > max {
			max = score
			area = c.area
			ties = false
		} else if score > 0 && score == max {
			ties = true
		}
	}
	if max == 0 || ties {
		return "Unclassified", "We need your help identifying this file."
	}
	return area, "Suggested from filename/extracted terms; not verified."
}

func (s *EdgeServer) handleFactorySetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", 405)
		return
	}
	data, e := web.GetFactorySetupHTML()
	if e != nil {
		http.Error(w, "Factory setup unavailable", 503)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

func (s *EdgeServer) handleFactoryDiscovery(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/v1/factory-onboarding" {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.getFactoryDiscovery(w, r)
	default:
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", 405)
	}
}
func (s *EdgeServer) handleFactoryFact(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/v1/factory-onboarding/facts" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", 405)
		return
	}
	tenant, e := intakeTenant()
	if e != nil {
		http.Error(w, "Development tenant is not configured", 503)
		return
	}
	if s.db == nil {
		http.Error(w, "PostgreSQL unavailable", 503)
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "Expected JSON", 415)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	var v factorySubmit
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if e = dec.Decode(&v); e != nil {
		http.Error(w, "Invalid fact payload", 400)
		return
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		http.Error(w, "Only one JSON fact may be submitted", 400)
		return
	}
	spec, ok := factorySpec(v.FieldKey)
	if !ok {
		http.Error(w, "Choose a supported factory field", 400)
		return
	}
	normalizedValue, normalized, e := canonicalFactoryValue(spec, v.Value)
	if e != nil {
		http.Error(w, e.Error(), 400)
		return
	}
	if v.State != "PROVISIONAL" && v.State != "CONFIRMED" {
		http.Error(w, "Choose provisional or confirmed", 400)
		return
	}
	if len(v.Note) > 1000 || strings.IndexByte(v.Note, 0) >= 0 {
		http.Error(w, "Review note is too long", 400)
		return
	}
	if v.ExpectedRevision < 0 {
		http.Error(w, "Invalid revision", 400)
		return
	}
	if v.SourceDocumentID != "" {
		if len(v.SourceDocumentID) != 36 || !strings.HasPrefix(v.SourceDocumentID, "DOC-") {
			http.Error(w, "Invalid source document", 400)
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, e := s.docTransaction(ctx, tenant)
	if e != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	defer tx.Rollback()
	// Serialize writers on the exact tenant+field, so revision checks are atomic.
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1),hashtext($2))`, tenant, v.FieldKey); e != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	revision := 0
	e = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(revision),0) FROM factory_fact_events_v65b WHERE tenant_id=$1 AND field_key=$2`, tenant, v.FieldKey).Scan(&revision)
	if e != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	if v.ExpectedRevision != revision {
		docJSON(w, 409, map[string]any{"error": "This field was updated elsewhere. Refresh before trying again.", "current_revision": revision})
		return
	}
	if v.SourceDocumentID != "" {
		var exists bool
		e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM document_sources_v65 WHERE tenant_id=$1 AND id=$2)`, tenant, v.SourceDocumentID).Scan(&exists)
		if e != nil {
			http.Error(w, "Database unavailable", 503)
			return
		}
		if !exists {
			http.Error(w, "Attached source document not found in this tenant", 400)
			return
		}
	}
	actor := "dev" // Development environment only; NOT a verified production identity.
	_, e = tx.ExecContext(ctx, `INSERT INTO factory_fact_events_v65b (tenant_id,field_key,revision,value_kind,value_text,normalized_value,review_state,source_document_id,review_note,recorded_by)
  VALUES($1,$2,$3,$4,$5,$6::jsonb,$7,NULLIF($8,''),$9,$10)`, tenant, v.FieldKey, revision+1, spec.Kind, normalizedValue, string(normalized), v.State, v.SourceDocumentID, v.Note, actor)
	if e != nil {
		http.Error(w, "Unable to record this fact", 503)
		return
	}
	if e = tx.Commit(); e != nil {
		http.Error(w, "Unable to commit this fact", 503)
		return
	}
	docJSON(w, 201, map[string]any{"field_key": v.FieldKey, "revision": revision + 1, "state": v.State, "meaning": "Human-entered onboarding evidence; not an operational approval"})
}

func (s *EdgeServer) getFactoryDiscovery(w http.ResponseWriter, r *http.Request) {
	tenant, e := intakeTenant()
	if e != nil {
		http.Error(w, "Development tenant is not configured", 503)
		return
	}
	if s.db == nil {
		http.Error(w, "PostgreSQL unavailable", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, e := s.docTransaction(ctx, tenant)
	if e != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, `SELECT DISTINCT ON (field_key) field_key,value_text,review_state,revision,
  COALESCE(source_document_id,''),review_note,recorded_at
  FROM factory_fact_events_v65b WHERE tenant_id=$1 ORDER BY field_key,revision DESC`, tenant)
	if e != nil {
		http.Error(w, "Factory records unavailable; apply migration 0010", 503)
		return
	}
	facts := map[string]factoryFact{}
	for rows.Next() {
		var f factoryFact
		if e = rows.Scan(&f.FieldKey, &f.Value, &f.State, &f.Revision, &f.SourceDocumentID, &f.Note, &f.RecordedAt); e != nil {
			break
		}
		facts[f.FieldKey] = f
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		http.Error(w, "Unable to read factory facts", 503)
		return
	}
	hints := []factoryDocHint{}
	dr, e := tx.QueryContext(ctx, `SELECT d.id,d.filename,j.status,COALESCE(
  (SELECT LEFT(s.content,1500) FROM document_segments_v65 s
   WHERE s.tenant_id=d.tenant_id AND s.document_id=d.id ORDER BY s.ordinal LIMIT 1),'')
  FROM document_sources_v65 d JOIN document_jobs_v65 j ON (d.tenant_id=j.tenant_id AND d.id=j.document_id)
  WHERE d.tenant_id=$1 ORDER BY d.captured_at DESC LIMIT 40`, tenant)
	if e != nil {
		http.Error(w, "Document suggestions unavailable", 503)
		return
	}
	for dr.Next() {
		var hint factoryDocHint
		var snippet string
		if e = dr.Scan(&hint.ID, &hint.Filename, &hint.Status, &snippet); e != nil {
			break
		}
		hint.SuggestedArea, hint.Reason = factoryDocumentHint(hint.Filename, snippet)
		hints = append(hints, hint)
	}
	if e == nil {
		e = dr.Err()
	}
	dr.Close()
	if e != nil {
		http.Error(w, "Unable to read document suggestions", 503)
		return
	}
	if e = tx.Commit(); e != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	readiness, prompt, known, verified := factorySuggestions(facts)
	docJSON(w, 200, map[string]any{
		"tenant": tenant, "scope": "development evidence only", "fields": factoryFieldSpecs,
		"facts": facts, "documents": hints, "readiness": readiness, "next_question": prompt,
		"known_fields": known, "confirmed_fields": verified, "total_fields": len(factoryFieldSpecs),
		"next_questions":        factoryGuidedQuestions(facts),
		"allowed_now":           []string{"upload_documents", "explore_demo", "stage_buyer_documents"},
		"production_authorized": false,
	})
}
