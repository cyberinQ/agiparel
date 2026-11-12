// Author: Sunil Jain, 2026-11-11 — SOP-2 inspection, roll-pool, remnant and conservation decisions.
package sop2

import (
	"errors"
	"fmt"
	"sort"
)

type Defect struct {
	Type   string `json:"type"`
	AtMM   int64  `json:"at_mm"`
	SizeMM int64  `json:"size_mm"`
	Points int64  `json:"points"`
}

type Roll struct {
	ID        string `json:"id"`
	PlantID   string `json:"plant_id"`
	FabricSKU string `json:"fabric_sku"`
	Shade     string `json:"shade"`
	LengthMM  int64  `json:"length_mm"`
	WidthMM   int64  `json:"width_mm"`
	Inspected bool   `json:"inspected"`
	Accepted  bool   `json:"accepted"`
	Relaxed   bool   `json:"relaxed"`
}

type Inspection struct {
	RollID          string   `json:"roll_id"`
	TicketLengthMM  int64    `json:"ticket_length_mm"`
	ActualLengthMM  int64    `json:"actual_length_mm"`
	WidthSamplesMM  []int64  `json:"width_samples_mm"`
	Shade           string   `json:"shade"`
	Defects         []Defect `json:"defects"`
	RelaxationHours int64    `json:"relaxation_hours"`
	RelaxationDone  bool     `json:"relaxation_done"`
}

type InspectionPolicy struct {
	MinUsableWidthMM   int64 `json:"min_usable_width_mm"`
	MaxDefectPoints    int64 `json:"max_defect_points"`
	MaxLengthLossBP    int64 `json:"max_length_loss_bp"`
	RequiredSamples    int   `json:"required_width_samples"`
	RequiredRelaxHours int64 `json:"required_relax_hours"`
}

type InspectionDecision struct {
	Accepted      bool     `json:"accepted"`
	Status        string   `json:"status"`
	UsableWidthMM int64    `json:"usable_width_mm"`
	DefectPoints  int64    `json:"defect_points"`
	Reasons       []string `json:"reasons"`
}

// InspectRoll rejects unknown inspection/acceptance criteria instead of inventing
// a defect-points standard or a knit relaxation interval.
func InspectRoll(in Inspection, policy InspectionPolicy) (InspectionDecision, error) {
	out := InspectionDecision{Status: "HOLD", Reasons: []string{}}
	if in.RollID == "" || in.TicketLengthMM <= 0 || in.ActualLengthMM <= 0 || in.Shade == "" || policy.MinUsableWidthMM <= 0 || policy.MaxDefectPoints < 0 || policy.MaxLengthLossBP < 0 || policy.MaxLengthLossBP > 10000 || policy.RequiredSamples < 2 || policy.RequiredRelaxHours < 0 {
		return out, errors.New("FABRIC_INSPECTION_POLICY_OR_MEASUREMENT_INVALID")
	}
	if len(in.WidthSamplesMM) < policy.RequiredSamples {
		out.Reasons = append(out.Reasons, "WIDTH_SAMPLES_INCOMPLETE")
	}
	for i, w := range in.WidthSamplesMM {
		if w <= 0 {
			return out, fmt.Errorf("WIDTH_SAMPLE_%d_INVALID", i)
		}
		if out.UsableWidthMM == 0 || w < out.UsableWidthMM {
			out.UsableWidthMM = w
		}
	}
	if out.UsableWidthMM < policy.MinUsableWidthMM {
		out.Reasons = append(out.Reasons, "USABLE_WIDTH_BELOW_MINIMUM")
	}
	if in.TicketLengthMM > in.ActualLengthMM && (in.TicketLengthMM-in.ActualLengthMM)*10000 > in.TicketLengthMM*policy.MaxLengthLossBP {
		out.Reasons = append(out.Reasons, "LENGTH_VARIANCE_OUTSIDE_POLICY")
	}
	for _, d := range in.Defects {
		if d.Type == "" || d.AtMM < 0 || d.AtMM > in.ActualLengthMM || d.SizeMM < 0 || d.Points < 0 {
			return out, errors.New("DEFECT_MAP_INVALID")
		}
		out.DefectPoints += d.Points
	}
	if out.DefectPoints > policy.MaxDefectPoints {
		out.Reasons = append(out.Reasons, "DEFECT_POINTS_EXCEED_POLICY")
	}
	if policy.RequiredRelaxHours > 0 && (!in.RelaxationDone || in.RelaxationHours < policy.RequiredRelaxHours) {
		out.Reasons = append(out.Reasons, "RELAXATION_NOT_VERIFIED")
	}
	if len(out.Reasons) == 0 {
		out.Status = "PASS"
		out.Accepted = true
	}
	return out, nil
}

type LayDemand struct {
	ID        string `json:"id"`
	PlantID   string `json:"plant_id"`
	FabricSKU string `json:"fabric_sku"`
	Shade     string `json:"shade"`
	MarkerMM  int64  `json:"marker_mm"`
	WidthMM   int64  `json:"width_mm"`
	Plies     int64  `json:"plies"`
}

type RollAllocationRequest struct {
	Rolls     []Roll      `json:"rolls"`
	Lays      []LayDemand `json:"lays"`
	ReserveMM int64       `json:"reserve_mm"`
	MaxStates int         `json:"max_states"`
}

type RollAssignment struct {
	RollID     string `json:"roll_id"`
	LayID      string `json:"lay_id"`
	Plies      int64  `json:"plies"`
	FabricMM   int64  `json:"fabric_mm"`
	ReservedMM int64  `json:"reserved_mm"`
	EndBitMM   int64  `json:"end_bit_mm"`
}

type RollAllocationResult struct {
	Feasible         bool             `json:"feasible"`
	OptimalityProven bool             `json:"optimality_proven"`
	SearchTruncated  bool             `json:"search_truncated"`
	MaterialVerified bool             `json:"material_verified"`
	Assignments      []RollAssignment `json:"assignments"`
	UnfilledPlies    map[string]int64 `json:"unfilled_plies"`
	EndBitMM         int64            `json:"end_bit_mm"`
	Reason           string           `json:"reason,omitempty"`
}

// AllocateRolls searches across the whole supplied pool, not just the nearest
// roll. Conservative physical model: a roll is allocated to at most one lay,
// reserving one fixed end bit. It cannot prove multi-lay reuse or CAD fit.
func AllocateRolls(in RollAllocationRequest) (RollAllocationResult, error) {
	out := RollAllocationResult{Assignments: []RollAssignment{}, UnfilledPlies: map[string]int64{}, MaterialVerified: false}
	if len(in.Rolls) == 0 || len(in.Lays) == 0 || len(in.Rolls) > 24 || len(in.Lays) > 12 || in.ReserveMM < 0 {
		return out, errors.New("ROLL_PLAN_INPUT_INVALID")
	}
	if in.MaxStates == 0 {
		in.MaxStates = 70000
	}
	if in.MaxStates < 1 || in.MaxStates > 200000 {
		return out, errors.New("ROLL_SEARCH_LIMIT_INVALID")
	}
	sort.Slice(in.Rolls, func(i, j int) bool { return in.Rolls[i].ID < in.Rolls[j].ID })
	sort.Slice(in.Lays, func(i, j int) bool { return in.Lays[i].ID < in.Lays[j].ID })
	seen := map[string]bool{}
	remaining := make([]int64, len(in.Lays))
	for i, l := range in.Lays {
		if l.ID == "" || seen["lay:"+l.ID] || l.PlantID == "" || l.Shade == "" || l.FabricSKU == "" || l.MarkerMM < 1 || l.WidthMM < 1 || l.Plies < 1 || l.Plies > 210 {
			return out, errors.New("LAY_DEMAND_INVALID")
		}
		seen["lay:"+l.ID] = true
		remaining[i] = l.Plies
	}
	for _, r := range in.Rolls {
		if r.ID == "" || seen["roll:"+r.ID] || r.LengthMM < 0 || r.WidthMM < 0 {
			return out, errors.New("ROLL_DATA_INVALID")
		}
		seen["roll:"+r.ID] = true
	}
	bestWaste := int64(-1)
	bestRolls := int(^uint(0) >> 1)
	var best []RollAssignment
	explored := 0
	var walk func(int, int64, []RollAssignment)
	walk = func(i int, waste int64, assign []RollAssignment) {
		if out.SearchTruncated {
			return
		}
		explored++
		if explored > in.MaxStates {
			out.SearchTruncated = true
			return
		}
		filled := true
		for _, n := range remaining {
			if n != 0 {
				filled = false
				break
			}
		}
		if filled {
			if bestWaste < 0 || waste < bestWaste || (waste == bestWaste && len(assign) < bestRolls) {
				bestWaste = waste
				bestRolls = len(assign)
				best = append([]RollAssignment(nil), assign...)
			}
			return
		}
		if i >= len(in.Rolls) || (bestWaste >= 0 && waste > bestWaste) {
			return
		}
		// Permit unused rolls; avoids consuming unrelated inventory to make waste appear small.
		walk(i+1, waste, assign)
		roll := in.Rolls[i]
		if !roll.Inspected || !roll.Accepted || !roll.Relaxed || roll.LengthMM <= in.ReserveMM {
			return
		}
		for j, lay := range in.Lays {
			if remaining[j] == 0 || lay.PlantID != roll.PlantID || lay.FabricSKU != roll.FabricSKU || lay.Shade != roll.Shade || roll.WidthMM < lay.WidthMM {
				continue
			}
			maxPlies := (roll.LengthMM - in.ReserveMM) / lay.MarkerMM
			if maxPlies > remaining[j] {
				maxPlies = remaining[j]
			}
			for p := maxPlies; p >= 1; p-- {
				rem := roll.LengthMM - in.ReserveMM - p*lay.MarkerMM
				remaining[j] -= p
				entry := RollAssignment{RollID: roll.ID, LayID: lay.ID, Plies: p, FabricMM: p * lay.MarkerMM, ReservedMM: in.ReserveMM, EndBitMM: rem}
				walk(i+1, waste+rem, append(assign, entry))
				remaining[j] += p
				if out.SearchTruncated {
					return
				}
			}
		}
	}
	walk(0, 0, nil)
	if bestWaste < 0 {
		out.Reason = "INSUFFICIENT_COMPATIBLE_ROLL_CAPACITY_OR_SEARCH_LIMIT"
		for i, l := range in.Lays {
			out.UnfilledPlies[l.ID] = remaining[i]
		}
		return out, nil
	}
	out.Feasible = true
	out.Assignments = best
	out.EndBitMM = bestWaste
	out.OptimalityProven = !out.SearchTruncated
	for _, l := range in.Lays {
		out.UnfilledPlies[l.ID] = 0
	}
	if out.SearchTruncated {
		out.Reason = "FEASIBLE_ALLOCATION_NOT_PROVEN_OPTIMAL"
	}
	return out, nil
}

type FabricBalance struct {
	IssuedMM          int64 `json:"issued_mm"`
	PliesMM           int64 `json:"plies_mm"`
	ReserveUsedMM     int64 `json:"reserve_used_mm"`
	ReserveReturnedMM int64 `json:"reserve_returned_mm"`
	EndBitsMM         int64 `json:"end_bits_mm"`
	DefectCutoutsMM   int64 `json:"defect_cutouts_mm"`
}

type FabricReconciliation struct {
	DifferenceMM int64  `json:"difference_mm"`
	Balanced     bool   `json:"balanced"`
	Status       string `json:"status"`
}

func ReconcileFabric(v FabricBalance) (FabricReconciliation, error) {
	if v.IssuedMM < 0 || v.PliesMM < 0 || v.ReserveUsedMM < 0 || v.ReserveReturnedMM < 0 || v.EndBitsMM < 0 || v.DefectCutoutsMM < 0 {
		return FabricReconciliation{}, errors.New("NEGATIVE_FABRIC_LEDGER_FIELD")
	}
	components := v.PliesMM + v.ReserveUsedMM + v.ReserveReturnedMM + v.EndBitsMM + v.DefectCutoutsMM
	delta := v.IssuedMM - components
	status := "RECONCILIATION_HOLD"
	if delta == 0 {
		status = "BALANCED"
	}
	return FabricReconciliation{DifferenceMM: delta, Balanced: delta == 0, Status: status}, nil
}

type RecutRequest struct {
	OriginalRollID      string `json:"original_roll_id"`
	ReserveRollID       string `json:"reserve_roll_id"`
	OriginalPanelNumber string `json:"original_panel_number"`
	RecutPanelNumber    string `json:"recut_panel_number"`
	OriginalShade       string `json:"original_shade"`
	ReserveShade        string `json:"reserve_shade"`
	ReserveAvailableMM  int64  `json:"reserve_available_mm"`
	NeededMM            int64  `json:"needed_mm"`
}

func ValidateRecut(v RecutRequest) error {
	if v.OriginalRollID == "" || v.OriginalRollID != v.ReserveRollID {
		return errors.New("RECUT_REQUIRES_ORIGINAL_ROLL_RESERVE")
	}
	if v.OriginalPanelNumber == "" || v.OriginalPanelNumber != v.RecutPanelNumber {
		return errors.New("RECUT_MUST_RETAIN_PANEL_NUMBER")
	}
	if v.OriginalShade == "" || v.OriginalShade != v.ReserveShade {
		return errors.New("RECUT_SHADE_MISMATCH")
	}
	if v.NeededMM <= 0 || v.ReserveAvailableMM < v.NeededMM {
		return errors.New("RECUT_RESERVE_INSUFFICIENT")
	}
	return nil
}
