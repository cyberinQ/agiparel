// Gate 06.5-B3: bounded, non-authoritative source-supported document routing.
// Possible categories do not establish that a PO, BOM or register is valid.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

type documentHint struct {
	Category      string `json:"category"`
	Label         string `json:"label"`
	Confidence    string `json:"confidence"`
	SourceOrdinal int    `json:"source_ordinal"`
	Reason        string `json:"reason"`
}
type hintRule struct {
	category, label string
	tags            []*regexp.Regexp
}

var factoryDocumentHintRules = []hintRule{
	{"MACHINE_REGISTER", "Machine / equipment register", []*regexp.Regexp{regexp.MustCompile(`(?i)\b(machine[_ ]?(id|code|number)|equipment[_ ]?id)\b`), regexp.MustCompile(`(?i)\b(machine[_ ]?(type|class)|equipment[_ ]?type)\b`)}},
	{"SKILL_MATRIX", "Employee skill register", []*regexp.Regexp{regexp.MustCompile(`(?i)\b(worker[_ ]?id|employee[_ ]?id|operator[_ ]?id)\b`), regexp.MustCompile(`(?i)\b(skill|proficiency|operation[_ ]?skill)\b`)}},
	{"FACTORY_INVENTORY", "Factory inventory", []*regexp.Regexp{regexp.MustCompile(`(?i)\b(material[_ ]?code|item[_ ]?code|material[_ ]?id)\b`), regexp.MustCompile(`(?i)\b(stock[_ ]?qty|opening[_ ]?qty|on[_ ]?hand|physical[_ ]?balance|inventory)\b`)}},
	{"FACTORY_RATES", "Factory rates and costing", []*regexp.Regexp{regexp.MustCompile(`(?i)\b(cost[_ ]?type|rate[_ ]?type|cost[_ ]?category|labor rate)\b`), regexp.MustCompile(`(?i)\b(currency|unit[_ ]?rate|hour|cost per hour)\b`)}},
	{"BUYER_ORDER", "Buyer purchase order", []*regexp.Regexp{regexp.MustCompile(`(?i)\b(purchase order|buyer po|po[_ ]?number|order[_ ]?number)\b`), regexp.MustCompile(`(?i)\b(style|quantity|delivery|ship[_ ]?date)\b`)}},
	{"TECH_PACK", "Buyer tech pack", []*regexp.Regexp{regexp.MustCompile(`(?i)\b(tech[_ ]?pack|technical[_ ]?package|specification|measurement chart|size spec)\b`), regexp.MustCompile(`(?i)\b(style|fabric|tolerance|measurement|construction)\b`)}},
	{"SEWING_BULLETIN", "Sewing operation bulletin", []*regexp.Regexp{regexp.MustCompile(`(?i)\b(operation bulletin|operation sequence|sewing operation|operation[_ ]?number)\b`), regexp.MustCompile(`(?i)\b(sam|machine[_ ]?type|standard minute|operation)\b`)}},
	{"PACKING_RULES", "Packing instructions", []*regexp.Regexp{regexp.MustCompile(`(?i)\b(packing instruction|carton|polyb[a]?g|assortment)\b`), regexp.MustCompile(`(?i)\b(size|quantity|sku|units per carton|ratio)\b`)}},
}

// Classification requires two distinct explicit signals from one source
// segment, or both signals within a short adjacent window. A suggestive
// filename alone is never sufficient and cannot turn a buyer BOM into stock.
func routeDocumentEvidence(segments []factoryEvidenceSegment) []documentHint {
	found := map[string]documentHint{}
	for i, seg := range segments {
		if i >= 600 {
			break
		}
		if len(seg.Content) > 16000 {
			continue
		}
		text := seg.Content
		// A spreadsheet header row (often split into cells) is one segment.
		// PDF may separate heading from table labels, so check one previous segment
		// only when both lie on the same PDF page / same spreadsheet sheet.
		if i > 0 && seg.Kind == segments[i-1].Kind && len(segments[i-1].Content) < 4000 && sameEvidenceScope(segments[i-1], seg) {
			text = segments[i-1].Content + "\n" + text
		}
		for _, rule := range factoryDocumentHintRules {
			if _, ok := found[rule.category]; ok {
				continue
			}
			if rule.tags[0].MatchString(text) && rule.tags[1].MatchString(text) {
				found[rule.category] = documentHint{Category: rule.category, Label: rule.label, Confidence: "POSSIBLE", SourceOrdinal: seg.Ordinal, Reason: "Two relevant terms appear in extracted evidence. Confirm the document purpose before accepting its facts."}
			}
		}
	}
	items := make([]documentHint, 0, len(found))
	for _, v := range found {
		items = append(items, v)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Category < items[j].Category })
	return items
}
func sameEvidenceScope(a, b factoryEvidenceSegment) bool {
	var x, y struct {
		Page  int    `json:"page"`
		Sheet string `json:"sheet"`
	}
	if json.Unmarshal(a.Locator, &x) != nil || json.Unmarshal(b.Locator, &y) != nil {
		return false
	}
	// Only combine adjacent records if their source scope is explicit.
	return (x.Page > 0 && x.Page == y.Page) || (x.Sheet != "" && x.Sheet == y.Sheet)
}
func (s *EdgeServer) handleDocumentTriage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", 405)
		return
	}
	id := strings.TrimSpace(r.URL.Query().Get("document_id"))
	if !validDocSourceID(id) {
		http.Error(w, "Select an uploaded document", 400)
		return
	}
	tenant, e := intakeTenant()
	if e != nil || s.db == nil {
		http.Error(w, "Development tenant or PostgreSQL unavailable", 503)
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
	var status string
	e = tx.QueryRowContext(ctx, `SELECT j.status FROM document_jobs_v65 j WHERE j.tenant_id=$1 AND j.document_id=$2`, tenant, id).Scan(&status)
	if errors.Is(e, sql.ErrNoRows) {
		http.Error(w, "Document not found", 404)
		return
	}
	if e != nil {
		http.Error(w, "Document status unavailable", 503)
		return
	}
	hints := []documentHint{}
	if status == "EXTRACTED" || status == "PARTIAL" {
		rows, err := tx.QueryContext(ctx, `SELECT ordinal,kind,locator,content FROM document_segments_v65 WHERE tenant_id=$1 AND document_id=$2 ORDER BY ordinal LIMIT 600`, tenant, id)
		if err != nil {
			http.Error(w, "Cannot read evidence", 503)
			return
		}
		segs := []factoryEvidenceSegment{}
		for rows.Next() {
			var x factoryEvidenceSegment
			var loc []byte
			if err = rows.Scan(&x.Ordinal, &x.Kind, &loc, &x.Content); err != nil {
				break
			}
			x.Locator = json.RawMessage(loc)
			segs = append(segs, x)
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			http.Error(w, "Cannot read source", 503)
			return
		}
		hints = routeDocumentEvidence(segs)
	}
	if e = tx.Commit(); e != nil {
		http.Error(w, "Database unavailable", 503)
		return
	}
	docJSON(w, 200, map[string]any{"document_id": id, "source_status": status, "possible_categories": hints, "review_required": true, "meaning": "Suggestions from extracted text only. No automatic change to any factory or buyer record."})
}
