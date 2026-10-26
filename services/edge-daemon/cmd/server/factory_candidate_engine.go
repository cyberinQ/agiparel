// Gate 06.5-B2: bounded, deterministic, source-anchored suggestions from factory documents.
// This is NOT AI certainty, employee identity verification, stock inspection, or master-data approval.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type factoryEvidenceSegment struct {
	Ordinal int             `json:"ordinal"`
	Kind    string          `json:"kind"`
	Locator json.RawMessage `json:"locator"`
	Content string          `json:"content"`
}
type factoryCandidateProposal struct {
	DocumentID    string          `json:"document_id"`
	SourceOrdinal int             `json:"source_ordinal"`
	Locator       json.RawMessage `json:"locator"`
	EntityType    string          `json:"entity_type"`
	EntityKey     string          `json:"entity_key"`
	FieldKey      string          `json:"field_key,omitempty"`
	Value         string          `json:"value"`
	Attributes    json.RawMessage `json:"attributes"`
	Explanation   string          `json:"explanation"`
	Fingerprint   string          `json:"fingerprint"`
}

const maxCandidateSegments = 1200
const maxCandidateProposals = 150

// Scan only explicitly labeled source facts. No cross-row arithmetic, predicted
// balances, inferred SAM, missing quantities or fictitious people are generated.
var explicitFactoryFacts = []struct {
	field   string
	pattern *regexp.Regexp
}{
	{"machines.sewing_count", regexp.MustCompile(`(?i)^\s*(?:number of\s+|total\s+)?(?:working\s+|operational\s+)?sewing machines?\s*[:=\-]\s*([0-9]{1,7})\s*$`)},
	{"workforce.headcount", regexp.MustCompile(`(?i)^\s*(?:production\s+)?(?:worker|employee|operator)\s*(?:count|headcount|total)\s*[:=\-]\s*([0-9]{1,7})\s*$`)},
	{"calendar.shift_hours", regexp.MustCompile(`(?i)^\s*(?:hours per shift|shift hours)\s*[:=\-]\s*([0-9]{1,2}(?:\.[0-9]{1,2})?)\s*$`)},
	{"calendar.workdays_week", regexp.MustCompile(`(?i)^\s*(?:working days per week|workdays per week)\s*[:=\-]\s*([1-7])\s*$`)},
	{"costs.currency", regexp.MustCompile(`(?i)^\s*(?:costing currency|factory currency)\s*[:=\-]\s*([a-zA-Z]{3})\s*$`)},
	{"costs.labor_rate", regexp.MustCompile(`(?i)^\s*(?:direct )?labor (?:cost|rate) per hour\s*[:=\-]\s*([0-9]{1,9}(?:\.[0-9]{1,4})?)\s*$`)},
	{"costs.overhead_rate", regexp.MustCompile(`(?i)^\s*(?:factory )?overhead (?:cost|rate) per hour\s*[:=\-]\s*([0-9]{1,9}(?:\.[0-9]{1,4})?)\s*$`)},
}
var sheetCell = regexp.MustCompile(`^([A-Z]{1,3})[0-9]+:\s*(.*)$`)
var safeToken = regexp.MustCompile(`^[a-zA-Z0-9_.:/-]{1,64}$`)
var nonnegNumber = regexp.MustCompile(`^(?:0|[1-9][0-9]{0,8})(?:\.[0-9]{1,4})?$`)
var plainColumnName = regexp.MustCompile(`[^a-z0-9]+`)

func candidateDigest(p factoryCandidateProposal) string {
	h := sha256.Sum256([]byte(p.DocumentID + "\x00" + strconv.Itoa(p.SourceOrdinal) + "\x00" + p.EntityType + "\x00" + p.EntityKey + "\x00" + p.FieldKey + "\x00" + p.Value + "\x00" + string(p.Attributes)))
	return hex.EncodeToString(h[:])
}
func normalColumn(s string) string {
	return strings.Trim(plainColumnName.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "_"), "_")
}
func parseSegmentColumns(seg factoryEvidenceSegment) ([]string, []string, bool) {
	switch seg.Kind {
	case "ROW":
		var values []string
		if json.Unmarshal([]byte(seg.Content), &values) != nil {
			return nil, nil, false
		}
		keys := make([]string, len(values))
		for i := range keys {
			keys[i] = strconv.Itoa(i)
		}
		return keys, values, true
	case "SHEET_ROW":
		fields := strings.Split(seg.Content, " | ")
		keys, vals := []string{}, []string{}
		for _, f := range fields {
			parts := sheetCell.FindStringSubmatch(strings.TrimSpace(f))
			if len(parts) != 3 {
				continue
			}
			keys = append(keys, parts[1])
			vals = append(vals, strings.TrimSpace(parts[2]))
		}
		return keys, vals, len(keys) > 0
	}
	return nil, nil, false
}
func colsAsMap(keys, values []string) map[string]string {
	out := map[string]string{}
	for i, k := range keys {
		if i < len(values) {
			out[k] = strings.TrimSpace(values[i])
		}
	}
	return out
}
func columnLabelMap(keys, values []string) map[string]string {
	out := map[string]string{}
	for i, key := range keys {
		if i < len(values) {
			label := normalColumn(values[i])
			if label != "" {
				out[label] = key
			}
		}
	}
	return out
}
func firstColumn(headers map[string]string, names ...string) string {
	for _, name := range names {
		if v, ok := headers[name]; ok {
			return v
		}
	}
	return ""
}
func getColumn(data map[string]string, headers map[string]string, names ...string) string {
	key := firstColumn(headers, names...)
	if key == "" {
		return ""
	}
	return strings.TrimSpace(data[key])
}
func labelHasAny(headers map[string]string, names ...string) bool {
	return firstColumn(headers, names...) != ""
}
func isHeaderRow(headers map[string]string) bool {
	// Recognize an actual named column header, never interpret a random data row as a schema.
	for _, keys := range [][]string{{"machine_id", "machine_code", "equipment_id"}, {"worker_id", "employee_id", "operator_id"}, {"material_code", "item_code", "material_id", "sku"}, {"cost_type", "rate_type", "cost_category"}} {
		if labelHasAny(headers, keys...) {
			return true
		}
	}
	return false
}
func sourceScope(locator json.RawMessage) string {
	var x struct {
		Sheet string `json:"sheet"`
	}
	_ = json.Unmarshal(locator, &x)
	return strings.ToLower(x.Sheet)
}
func isBuyerDoc(filename string) bool {
	name := strings.ToLower(filename)
	for _, word := range []string{"purchase order", "buyer_po", "tech pack", "techpack", "packing list", "buyer order", "sales order"} {
		if strings.Contains(name, word) {
			return true
		}
	}
	return false
}
func qualifiesInventory(filename string, seg factoryEvidenceSegment, headers map[string]string) bool {
	context := strings.ToLower(filename) + " " + sourceScope(seg.Locator)
	for _, w := range []string{"inventory", "stock", "store ledger", "warehouse", "opening balance"} {
		if strings.Contains(context, w) {
			return true
		}
	}
	return labelHasAny(headers, "stock_qty", "stock_quantity", "on_hand_qty", "available_qty", "opening_qty", "quantity_on_hand", "physical_balance")
}
func rowProposal(docID string, seg factoryEvidenceSegment, filename string, headers map[string]string, values map[string]string) (factoryCandidateProposal, bool) {
	p := factoryCandidateProposal{DocumentID: docID, SourceOrdinal: seg.Ordinal, Locator: seg.Locator}
	// Mutually exclusive detection by stable identifier and explicit, typed headers.
	if id := getColumn(values, headers, "machine_id", "machine_code", "equipment_id"); id != "" {
		typ := getColumn(values, headers, "machine_type", "equipment_type", "machine_class", "machine_name")
		if !safeToken.MatchString(id) || typ == "" || len(typ) > 180 {
			return p, false
		}
		attrs := map[string]string{"machine_id": id, "machine_type": typ}
		if status := getColumn(values, headers, "status", "machine_status"); status != "" {
			attrs["status_note"] = status
		}
		p.EntityType = "MACHINE"
		p.EntityKey = id
		p.Value = id + " · " + typ
		p.Explanation = "Possible machine register row. Machine identity and capabilities require human verification."
		p.Attributes, _ = json.Marshal(attrs)
	} else if id := getColumn(values, headers, "worker_id", "employee_id", "operator_id"); id != "" {
		skill := getColumn(values, headers, "skill", "skill_name", "operation_skill", "operation", "capability")
		if !safeToken.MatchString(id) || skill == "" || len(skill) > 180 {
			return p, false
		}
		attrs := map[string]string{"worker_id": id, "skill": skill}
		if level := getColumn(values, headers, "level", "proficiency", "skill_level"); level != "" {
			attrs["level_note"] = level
		}
		p.EntityType = "SKILL"
		p.EntityKey = id + ":" + skill
		p.Value = id + " · " + skill
		p.Explanation = "Possible worker skill. Names and personal attributes are not imported; assess capability separately."
		p.Attributes, _ = json.Marshal(attrs)
	} else if id := getColumn(values, headers, "material_code", "item_code", "material_id", "sku"); id != "" {
		if !qualifiesInventory(filename, seg, headers) {
			return p, false
		}
		qty := getColumn(values, headers, "stock_qty", "stock_quantity", "on_hand_qty", "available_qty", "opening_qty", "quantity_on_hand", "physical_balance", "quantity", "qty")
		unit := getColumn(values, headers, "unit", "uom", "stock_uom")
		if !safeToken.MatchString(id) || !nonnegNumber.MatchString(qty) || unit == "" || len(unit) > 24 {
			return p, false
		}
		attrs := map[string]string{"material_code": id, "quantity": qty, "unit": unit}
		if desc := getColumn(values, headers, "description", "material_type", "item_description"); desc != "" {
			attrs["description"] = desc
		}
		p.EntityType = "MATERIAL"
		p.EntityKey = id + ":" + unit
		p.Value = id + " · " + qty + " " + unit
		p.Explanation = "Unverified inventory-sheet quantity; neither inspection nor unreserved availability is implied."
		p.Attributes, _ = json.Marshal(attrs)
	} else if typ := getColumn(values, headers, "cost_type", "rate_type", "cost_category"); typ != "" {
		rate := getColumn(values, headers, "rate", "unit_rate", "amount", "cost_per_hour")
		curr := strings.ToUpper(getColumn(values, headers, "currency", "currency_code"))
		if !nonnegNumber.MatchString(rate) || !factoryCurrency.MatchString(curr) || len(typ) > 100 {
			return p, false
		}
		attrs := map[string]string{"cost_type": typ, "rate": rate, "currency": curr}
		if unit := getColumn(values, headers, "unit", "rate_unit", "per"); unit != "" {
			attrs["unit_note"] = unit
		}
		p.EntityType = "RATE"
		p.EntityKey = typ + ":" + curr
		p.Value = typ + " · " + rate + " " + curr
		p.Explanation = "Proposed rate only; time basis, effective dates, tax and overhead treatment require confirmation."
		p.Attributes, _ = json.Marshal(attrs)
	} else {
		return p, false
	}
	p.Fingerprint = candidateDigest(p)
	return p, true
}

func explicitProposal(docID string, seg factoryEvidenceSegment, raw string) (factoryCandidateProposal, bool) {
	p := factoryCandidateProposal{DocumentID: docID, SourceOrdinal: seg.Ordinal, Locator: seg.Locator, EntityType: "FIELD"}
	text := strings.TrimSpace(raw)
	if strings.HasPrefix(text, "=") || strings.Contains(text, "[cached:") {
		return p, false
	}
	// PDF page paragraphs may contain many lines. Only accept a FULL labelled line.
	for _, pattern := range explicitFactoryFacts {
		match := pattern.pattern.FindStringSubmatch(text)
		if len(match) != 2 {
			continue
		}
		spec, ok := factorySpec(pattern.field)
		if !ok {
			return p, false
		}
		value, normalized, err := canonicalFactoryValue(spec, match[1])
		if err != nil {
			return p, false
		}
		p.FieldKey = pattern.field
		p.EntityKey = pattern.field
		p.Value = value
		p.Attributes = normalized
		p.Explanation = "Exact labelled value found in the uploaded source. Still needs human confirmation."
		p.Fingerprint = candidateDigest(p)
		return p, true
	}
	return p, false
}

// factoryProposalsFromSegments is deterministic and testable without AI or DB.
// Incomplete or unrecognized source fragments are skipped, never guessed.
func factoryProposalsFromSegments(docID, filename string, segments []factoryEvidenceSegment) []factoryCandidateProposal {
	if isBuyerDoc(filename) {
		return []factoryCandidateProposal{}
	}
	proposals := []factoryCandidateProposal{}
	seen := map[string]bool{}
	appendUnique := func(p factoryCandidateProposal) {
		if len(proposals) >= maxCandidateProposals || seen[p.Fingerprint] {
			return
		}
		seen[p.Fingerprint] = true
		proposals = append(proposals, p)
	}
	type scopedHeader struct {
		headers map[string]string
		ordinal int
	}
	sheetHeaders := map[string]scopedHeader{}
	for i, seg := range segments {
		if i >= maxCandidateSegments || len(proposals) >= maxCandidateProposals {
			break
		}
		if seg.Ordinal < 1 || len(seg.Content) > 16000 {
			continue
		}
		if keys, vals, ok := parseSegmentColumns(seg); ok {
			scope := seg.Kind + ":" + sourceScope(seg.Locator)
			labels := columnLabelMap(keys, vals)
			if isHeaderRow(labels) {
				sheetHeaders[scope] = scopedHeader{labels, seg.Ordinal}
				continue
			}
			if h, ok := sheetHeaders[scope]; ok && seg.Ordinal > h.ordinal {
				if p, ok := rowProposal(docID, seg, filename, h.headers, colsAsMap(keys, vals)); ok {
					appendUnique(p)
				}
			}
			// In 2-column key-value files an explicit label + numeric value is useful.
			for _, key := range keys {
				if raw := colsAsMap(keys, vals)[key]; strings.ContainsAny(raw, ":=-") {
					if p, ok := explicitProposal(docID, seg, raw); ok {
						appendUnique(p)
					}
				}
			}
			if len(vals) == 2 {
				if p, ok := explicitProposal(docID, seg, vals[0]+": "+vals[1]); ok {
					appendUnique(p)
				}
			}
			continue
		}
		for _, line := range strings.Split(seg.Content, "\n") {
			if p, ok := explicitProposal(docID, seg, line); ok {
				appendUnique(p)
			}
		}
	}
	return proposals
}
func candidateKindLabel(t string) string {
	switch t {
	case "MACHINE":
		return "Machine register"
	case "SKILL":
		return "Worker skill"
	case "MATERIAL":
		return "Inventory observation"
	case "RATE":
		return "Cost / rate"
	case "FIELD":
		return "Factory profile"
	}
	return fmt.Sprintf("Unknown: %s", t)
}
