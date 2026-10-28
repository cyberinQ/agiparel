// Author: Sunil Jain, 2026-10-28 — Mark unconvertible fabric demand as unknown, not zero.
// Batch 07D: deterministic, non-authoritative roll pick suggestions.
// No material moves, reservations or inspection approvals happen here.
package main

import (
	"errors"
	"math"
	"sort"
	"strings"
)

type pickOrder struct {
	IntakeID     string `json:"intake_id"`
	PONumber     string `json:"po_number"`
	StyleCode    string `json:"style_code"`
	Quantity     int64  `json:"ordered_qty"`
	SourceSHA256 string `json:"source_sha256"`
	Status       string `json:"status"`
}
type pickDemand struct {
	MaterialSKU     string `json:"material_sku"`
	Unit            string `json:"uom"`
	PerGarmentMilli int64  `json:"per_garment_milli"`
}
type pickRoll struct {
	ID                   string `json:"roll_id"`
	MaterialSKU          string `json:"material_sku"`
	Shade                string `json:"shade_band"`
	State                string `json:"stock_state"`
	Inspection           string `json:"inspection_status"`
	OwnerOrder           string `json:"reserved_order_id,omitempty"`
	RemainingMM          int64  `json:"remaining_mm"`
	OpenReservationMM    int64  `json:"open_reservation_mm"`
	ForeignReservationMM int64  `json:"other_order_open_reservation_mm"`
	ForeignIssuedMM      int64  `json:"other_order_issued_mm"`
	WidthMM              int64  `json:"usable_width_mm"`
}
type pickSelection struct {
	RollID      string `json:"roll_id"`
	Shade       string `json:"shade_band"`
	SuggestedMM int64  `json:"suggested_mm"`
	WidthMM     int64  `json:"usable_width_mm"`
	Basis       string `json:"basis"`
}
type pickExclusion struct {
	RollID string `json:"roll_id"`
	Reason string `json:"reason"`
}
type pickMaterial struct {
	SKU         string          `json:"material_sku"`
	UOM         string          `json:"uom"`
	DemandMM    int64           `json:"demand_mm"`
	DemandKnown bool            `json:"demand_known,omitempty"`
	TentativeMM int64           `json:"tentative_mm"`
	UncoveredMM int64           `json:"uncovered_mm"`
	ChosenShade string          `json:"chosen_shade,omitempty"`
	Picks       []pickSelection `json:"tentative_picks"`
	Excluded    []pickExclusion `json:"excluded_rolls"`
	Blockers    []string        `json:"blockers"`
}
type pickReport struct {
	Order         pickOrder      `json:"order"`
	Materials     []pickMaterial `json:"materials"`
	Status        string         `json:"status"`
	EvidenceClass string         `json:"evidence_class"`
	Authority     string         `json:"authority"`
	Warnings      []string       `json:"warnings"`
}

// Only physically recorded PASS rolls in available/reserved status can be
// considered. Cross-order ownership and existing open reservation always win.
func pickEligibility(r pickRoll, o pickOrder) (int64, string) {
	switch {
	case r.Inspection != "PASS":
		return 0, "LEDGER_INSPECTION_NOT_PASSED"
	case r.State != "AVAILABLE" && r.State != "RESERVED":
		return 0, "LEDGER_STOCK_NOT_AVAILABLE"
	case r.OwnerOrder != "" && r.OwnerOrder != o.IntakeID && r.OwnerOrder != o.PONumber:
		return 0, "HELD_FOR_ANOTHER_ORDER"
	case r.ForeignIssuedMM != 0:
		return 0, "PREVIOUSLY_ISSUED_TO_ANOTHER_ORDER"
	case r.ForeignReservationMM != 0:
		return 0, "OPEN_RESERVATION_FOR_ANOTHER_ORDER"
	case r.RemainingMM < 0 || r.OpenReservationMM < 0 || r.OpenReservationMM > r.RemainingMM:
		return 0, "INVALID_OR_OVERCOMMITTED_STOCK"
	case r.WidthMM <= 0:
		return 0, "WIDTH_NOT_VERIFIED"
	case strings.TrimSpace(r.Shade) == "":
		return 0, "SHADE_UNKNOWN"
	case r.RemainingMM == r.OpenReservationMM:
		return 0, "NO_UNCOMMITTED_LENGTH"
	}
	return r.RemainingMM - r.OpenReservationMM, ""
}

func buildPickReport(order pickOrder, demands []pickDemand, rolls []pickRoll) (pickReport, error) {
	out := pickReport{Order: order, Materials: []pickMaterial{}, Status: "PROPOSED_HOLD", EvidenceClass: "AUTHENTICATED_LEDGER_SNAPSHOT_WITH_UNVERIFIED_ALLOCATION", Authority: "NONE", Warnings: []string{
		"Tentative picks do not reserve or issue stock; only the signed material API can do that.",
		"Buyer ownership, fabric shade-to-colorway match, marker width, lot compatibility, formal roll admission and physical GRN require independent checks.",
		"Uncovered length is a planning gap, not an approved purchasing indent.",
	}}
	if order.Status != "PROPOSED" || order.IntakeID == "" || order.SourceSHA256 == "" || order.Quantity <= 0 || len(demands) == 0 || len(demands) > 250 || len(rolls) > 10000 {
		return pickReport{}, errors.New("invalid staged order, BOM, or inventory snapshot")
	}
	sort.Slice(rolls, func(i, j int) bool { return rolls[i].ID < rolls[j].ID })
	seen := make(map[string]bool)
	for _, d := range demands {
		if d.MaterialSKU == "" || d.PerGarmentMilli <= 0 || seen[d.MaterialSKU] || order.Quantity > math.MaxInt64/d.PerGarmentMilli {
			return pickReport{}, errors.New("invalid or duplicate fabric demand")
		}
		seen[d.MaterialSKU] = true
		m := pickMaterial{SKU: d.MaterialSKU, UOM: d.Unit, Picks: []pickSelection{}, Excluded: []pickExclusion{}, Blockers: []string{"SHADE_COLORWAY_MATCH_UNVERIFIED", "MARKER_WIDTH_UNVERIFIED", "OWNERSHIP_AND_PHYSICAL_RELEASE_UNVERIFIED"}}
		if d.Unit != "M" && d.Unit != "METRE" && d.Unit != "METER" {
			m.Blockers = append(m.Blockers, "UNIT_CONVERSION_REQUIRED")
			out.Materials = append(out.Materials, m)
			continue
		}
		m.DemandKnown = true
		m.DemandMM = order.Quantity * d.PerGarmentMilli
		m.UncoveredMM = m.DemandMM
		byShade := make(map[string][]pickRoll)
		totals := make(map[string]int64)
		for _, r := range rolls {
			if r.MaterialSKU != d.MaterialSKU {
				continue
			}
			eligible, reason := pickEligibility(r, order)
			if reason != "" {
				m.Excluded = append(m.Excluded, pickExclusion{RollID: r.ID, Reason: reason})
				continue
			}
			if eligible > math.MaxInt64-totals[r.Shade] {
				return pickReport{}, errors.New("roll length overflow")
			}
			byShade[r.Shade] = append(byShade[r.Shade], r)
			totals[r.Shade] += eligible
		}
		shades := make([]string, 0, len(totals))
		for shade := range totals {
			shades = append(shades, shade)
		}
		sort.Strings(shades)
		// Never combine fabric from different shade bands just to claim coverage.
		for _, shade := range shades {
			if m.ChosenShade == "" || totals[shade] > totals[m.ChosenShade] {
				m.ChosenShade = shade
			}
		}
		for _, shade := range shades {
			if shade != m.ChosenShade {
				for _, r := range byShade[shade] {
					m.Excluded = append(m.Excluded, pickExclusion{RollID: r.ID, Reason: "ALTERNATE_SHADE_NOT_COMBINED"})
				}
				continue
			}
			for _, r := range byShade[shade] {
				free, _ := pickEligibility(r, order)
				if m.UncoveredMM == 0 {
					m.Excluded = append(m.Excluded, pickExclusion{RollID: r.ID, Reason: "DEMAND_TENTATIVELY_COVERED"})
					continue
				}
				take := free
				if take > m.UncoveredMM {
					take = m.UncoveredMM
				}
				m.Picks = append(m.Picks, pickSelection{RollID: r.ID, Shade: r.Shade, SuggestedMM: take, WidthMM: r.WidthMM, Basis: "PROPOSAL_ONLY"})
				m.TentativeMM += take
				m.UncoveredMM -= take
			}
		}
		if m.UncoveredMM > 0 {
			m.Blockers = append(m.Blockers, "INSUFFICIENT_ELIGIBLE_SINGLE_SHADE_STOCK")
		}
		out.Materials = append(out.Materials, m)
	}
	return out, nil
}
