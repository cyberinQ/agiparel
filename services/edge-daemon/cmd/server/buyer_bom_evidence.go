// Gate 06.5-C2: conservative, page-anchored recognition of Centric-style
// colorway/material BOM tables in buyer tech packs. These are evidence hints.
// No guessed consumption, UOM, buyer approval, or factory stock authority.
package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type buyerBOMHint struct {
	Component     string `json:"component"`
	MaterialLabel string `json:"material_label"`
	SuggestedCode string `json:"suggested_code,omitempty"`
	Category      string `json:"category"`
	Section       string `json:"section"`
	RawQuantity   string `json:"raw_quantity,omitempty"`
	QuantityState string `json:"quantity_state"`
	SourceOrdinal int    `json:"source_ordinal"`
	Page          int    `json:"page"`
	Hint          string `json:"hint"`
}
type buyerBOMCoverage struct {
	BOMPages                   int      `json:"bom_pages"`
	MaterialRows               int      `json:"material_rows"`
	WithoutVerifiedConsumption int      `json:"without_verified_consumption"`
	Sections                   []string `json:"sections"`
	Classification             string   `json:"classification"`
}

var bomQuantityText = regexp.MustCompile(`^\d{1,6}(?:\.\d{1,3})?$`)
var bomNumberedMaterial = regexp.MustCompile(`^([0-9]{4,8})(?:[-\s]|$)`)
var bomPageSection = regexp.MustCompile(`(?i)^\s*Section:\s*([^\n]+?)\s{3,}`)

func bomPageNumber(locator json.RawMessage) int {
	var x struct {
		Page int `json:"page"`
	}
	if json.Unmarshal(locator, &x) != nil {
		return 0
	}
	return x.Page
}
func bomRegion(s string, start, end int) string {
	if start < 0 || start >= len(s) {
		return ""
	}
	if end < start || end > len(s) {
		end = len(s)
	}
	return strings.TrimSpace(s[start:end])
}
func bomSectionKind(section string) string {
	s := strings.ToUpper(section)
	switch {
	case strings.Contains(s, "FABRIC"), strings.Contains(s, "LINING"), strings.Contains(s, "POCKETING"):
		return "FABRIC"
	case strings.Contains(s, "TRIM"), strings.Contains(s, "PACK"), strings.Contains(s, "LABEL"), strings.Contains(s, "ACCESSOR"):
		return "TRIM"
	default:
		return ""
	}
}
func bomSection(line string) string {
	m := bomPageSection.FindStringSubmatch(line)
	if len(m) != 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}
func bomHeaderOffsets(line string) (int, int, int, int, int, bool) {
	lower := strings.ToLower(line)
	comp := strings.Index(lower, "component or")
	material := strings.Index(lower, "material")
	placement := strings.Index(lower, "placement")
	quantity := strings.Index(lower, "quantity")
	comments := strings.Index(lower, "comments")
	return comp, material, placement, quantity, comments, comp >= 0 && material > comp && placement > material && quantity > placement && comments > quantity && comments-quantity >= 9
}
func bomSuggestedCode(label string) string {
	x := bomNumberedMaterial.FindStringSubmatch(label)
	if len(x) == 2 {
		return x[1]
	}
	return ""
}
func bomTrimSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
func bomReason(raw string) (string, string) {
	if raw == "" {
		return "UNKNOWN", "Quantity absent. Verify per-garment consumption and unit from an authoritative BOM or marker."
	}
	if !bomQuantityText.MatchString(raw) {
		return "INVALID", "Printed quantity is not a validated per-garment usage rate; review the original page."
	}
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return "INVALID", "Quantity requires manual review."
	}
	if n == 0 {
		return "ZERO_UNVERIFIED", "Printed 0.000 is not proof of zero fabric usage. Obtain measured per-garment consumption and unit."
	}
	return "SOURCE_QUANTITY_UNVERIFIED", "Printed quantity has no verified usage unit or basis. Confirm per-garment rate and unit before staging."
}

// PDF extraction may split a long physical page into multiple 6,000-byte
// evidence segments. Reassemble per page for table interpretation while
// preserving the segment ordinal containing each detected row's first byte.
type bomSourceChunk struct {
	Offset  int
	Ordinal int
}
type bomSourcePage struct {
	Page   int
	Text   string
	Chunks []bomSourceChunk
}

func bomPages(segments []buyerEvidence) []bomSourcePage {
	pages := []bomSourcePage{}
	for _, seg := range segments {
		if seg.Kind != "PAGE" {
			continue
		}
		pn := bomPageNumber(seg.Locator)
		if pn <= 0 {
			continue
		}
		if len(pages) == 0 || pages[len(pages)-1].Page != pn {
			pages = append(pages, bomSourcePage{Page: pn})
		}
		pg := &pages[len(pages)-1]
		pg.Chunks = append(pg.Chunks, bomSourceChunk{len(pg.Text), seg.Ordinal})
		pg.Text += seg.Content
	}
	return pages
}
func (pg bomSourcePage) ordinalAt(offset int) int {
	result := 0
	for _, ch := range pg.Chunks {
		if ch.Offset > offset {
			break
		}
		result = ch.Ordinal
	}
	return result
}

// detectTechPackBOM returns candidate component identities, not ready-to-stage
// materials. If a table is absent or disrupted, it reports insufficient coverage.
// The parser uses layout column positions *from each source page*, not global
// whitespace splitting; wrapped multi-line descriptions remain with the row.
func detectTechPackBOM(segments []buyerEvidence) ([]buyerBOMHint, buyerBOMCoverage, []string) {
	hints := []buyerBOMHint{}
	coverage := buyerBOMCoverage{Sections: []string{}, Classification: "EVIDENCE_ONLY"}
	issues := []string{}
	sections := map[string]bool{}
	for _, page := range bomPages(segments) {
		if !strings.Contains(strings.ToUpper(page.Text), "BOM:") {
			continue
		}
		lines := strings.Split(page.Text, "\n")
		byteOffset := 0
		foundHeader := false
		activeSection := ""
		colComp, colMat, colPlace, colQty, colComments := -1, -1, -1, -1, -1
		var pending *buyerBOMHint
		flush := func() {
			if pending == nil {
				return
			}
			pending.Component = bomTrimSpace(pending.Component)
			pending.MaterialLabel = bomTrimSpace(pending.MaterialLabel)
			if pending.Component != "" && pending.MaterialLabel != "" && pending.RawQuantity != "" {
				pending.SuggestedCode = bomSuggestedCode(pending.MaterialLabel)
				pending.QuantityState, pending.Hint = bomReason(pending.RawQuantity)
				hints = append(hints, *pending)
				coverage.WithoutVerifiedConsumption++
			}
			pending = nil
		}
		pageHasTable := false
		for _, line := range lines {
			lineStart := byteOffset
			byteOffset += len(line) + 1
			if ss := bomSection(line); ss != "" {
				flush()
				activeSection = ss
				if !sections[ss] {
					sections[ss] = true
					coverage.Sections = append(coverage.Sections, ss)
				}
				// Table headings may repeat for every section on a single page.
				foundHeader = false
				continue
			}
			c, m, p, q, z, header := bomHeaderOffsets(line)
			if header && bomSectionKind(activeSection) != "" {
				flush()
				colComp, colMat, colPlace, colQty, colComments = c, m, p, q, z
				foundHeader = true
				pageHasTable = true
				continue
			}
			if !foundHeader || activeSection == "" || bomSectionKind(activeSection) == "" {
				continue
			}
			if strings.Contains(line, "\f") || strings.Contains(strings.ToLower(line), "page ") && strings.Contains(line, " of ") {
				flush()
				foundHeader = false
				continue
			}
			if len(strings.TrimSpace(line)) == 0 {
				continue
			}
			if len(line) < colMat {
				continue
			}
			component := bomRegion(line, colComp, colMat)
			material := bomRegion(line, colMat, colPlace)
			quantity := bomRegion(line, colQty, colComments)
			// A row start requires an explicitly printed quantity in its own
			// layout column. Do not interpret adjacent colorway numbers or
			// standalone figures as a new per-garment consumption value.
			isStart := component != "" && bomQuantityText.MatchString(quantity)
			if isStart {
				flush()
				pending = &buyerBOMHint{Component: component, MaterialLabel: material, Category: bomSectionKind(activeSection), Section: activeSection, RawQuantity: quantity, SourceOrdinal: page.ordinalAt(lineStart), Page: page.Page}
				continue
			}
			// PDF text layout wraps long material descriptions across lines.
			// Do not start a new row merely because its component wraps.
			if pending != nil {
				if component != "" {
					pending.Component += " " + component
				}
				if material != "" {
					pending.MaterialLabel += " " + material
				}
			}
		}
		flush()
		if pageHasTable {
			coverage.BOMPages++
		}
	}
	sort.Strings(coverage.Sections)
	coverage.MaterialRows = len(hints)
	if coverage.BOMPages > 0 && len(hints) == 0 {
		issues = append(issues, "A BOM table was detected but no reliable component rows were recognized; inspect the source manually")
	}
	if len(hints) > 0 {
		issues = append(issues, fmt.Sprintf("%d tech-pack BOM row(s) were recognized as evidence only; printed quantities are not verified material consumption and have no validated UOM", len(hints)))
	}
	// No candidate can independently authorize a purchase indent.
	return hints, coverage, issues
}
