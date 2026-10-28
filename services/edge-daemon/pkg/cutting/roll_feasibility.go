// Author: Sunil Jain, 2026-10-25
// Read-only high-level fabric planning; marker geometry remains in Gerber/CNC.
package cutting

import (
	"errors"
	"math"
	"sort"
	"strings"
)

// RollCandidate describes usable stock, not merely physical roll length.
// Stock reserved for another order must be excluded by the caller.
type RollCandidate struct {
	RollID        string  `json:"roll_id"`
	MaterialSKU   string  `json:"material_sku"`
	WidthCM       float64 `json:"width_cm"`
	UsableLengthM float64 `json:"usable_length_m"`
	ShadeBand     string  `json:"shade_band"`
	ReservedFor   string  `json:"reserved_for,omitempty"`
}

// MarkerCandidate is a verified output from an external marker system or IE.
// No marker nesting geometry is calculated or invented here.
type MarkerCandidate struct {
	MarkerID       string  `json:"marker_id"`
	MaterialSKU    string  `json:"material_sku"`
	MinimumWidthCM float64 `json:"minimum_width_cm"`
	MaximumWidthCM float64 `json:"maximum_width_cm"`
	LengthPerPlyM  float64 `json:"length_per_ply_m"`
	PiecesPerPly   int     `json:"pieces_per_ply"`
	MaxPlies       int     `json:"max_plies"`
}

type RollAllocation struct {
	RollID           string  `json:"roll_id"`
	MarkerID         string  `json:"marker_id"`
	Plies            int     `json:"plies"`
	Pieces           int     `json:"pieces"`
	UsedLengthM      float64 `json:"used_length_m"`
	RemainingLengthM float64 `json:"remaining_length_m"`
}

type FabricFeasibility struct {
	RequestedPieces        int              `json:"requested_pieces"`
	PlannedPieces          int              `json:"planned_pieces"`
	UnfilledPieces         int              `json:"unfilled_pieces"`
	EstimatedUnusedLengthM float64          `json:"estimated_unused_length_m"`
	Allocations            []RollAllocation `json:"allocations"`
	Questions              []string         `json:"questions,omitempty"`
	AdvisoryOnly           bool             `json:"advisory_only"`
}

// EvaluateRollAllocation selects feasible full plies by a deterministic
// best-remaining-length heuristic. Results are estimates only, never stock
// reservations, cutting instructions, or permission to mix shade bands.
func EvaluateRollAllocation(orderID, materialSKU, shadeBand string, pieces int, rolls []RollCandidate, markers []MarkerCandidate) (FabricFeasibility, error) {
	out := FabricFeasibility{RequestedPieces: pieces, AdvisoryOnly: true, Allocations: []RollAllocation{}}
	if strings.TrimSpace(orderID) == "" || strings.TrimSpace(materialSKU) == "" || pieces <= 0 {
		return out, errors.New("order, material and positive pieces required")
	}
	if len(markers) == 0 {
		out.Questions = append(out.Questions, "Please share the approved marker choices from cutting")
	}
	if strings.TrimSpace(shadeBand) == "" {
		out.Questions = append(out.Questions, "Which shade lot can we use? No shade mixing will be assumed")
	}
	rr := append([]RollCandidate(nil), rolls...)
	sort.Slice(rr, func(i, j int) bool {
		if rr[i].UsableLengthM == rr[j].UsableLengthM {
			return rr[i].RollID < rr[j].RollID
		}
		return rr[i].UsableLengthM > rr[j].UsableLengthM
	})
	for _, roll := range rr {
		if roll.MaterialSKU != materialSKU || roll.UsableLengthM <= 0 || math.IsNaN(roll.UsableLengthM) || math.IsInf(roll.UsableLengthM, 0) || roll.WidthCM <= 0 {
			continue
		}
		if roll.ReservedFor != "" && roll.ReservedFor != orderID {
			continue
		}
		if shadeBand == "" || roll.ShadeBand != shadeBand {
			continue
		}
		var best MarkerCandidate
		bestPieces := 0
		bestPlies := 0
		for _, m := range markers {
			if m.MaterialSKU != materialSKU || m.LengthPerPlyM <= 0 || math.IsNaN(m.LengthPerPlyM) || math.IsInf(m.LengthPerPlyM, 0) || m.PiecesPerPly <= 0 || m.MaxPlies <= 0 || m.MinimumWidthCM <= 0 || roll.WidthCM < m.MinimumWidthCM || (m.MaximumWidthCM > 0 && roll.WidthCM > m.MaximumWidthCM) {
				continue
			}
			remaining := pieces - out.PlannedPieces
			if remaining <= 0 {
				break
			}
			plies := int(math.Floor((roll.UsableLengthM + 1e-9) / m.LengthPerPlyM))
			if plies > m.MaxPlies {
				plies = m.MaxPlies
			}
			need := (remaining + m.PiecesPerPly - 1) / m.PiecesPerPly
			if plies > need {
				plies = need
			}
			score := plies * m.PiecesPerPly
			if score > bestPieces || (score == bestPieces && score > 0 && m.LengthPerPlyM*float64(plies) < best.LengthPerPlyM*float64(bestPlies)) {
				best = m
				bestPieces = score
				bestPlies = plies
			}
		}
		if bestPieces == 0 {
			continue
		}
		used := float64(bestPlies) * best.LengthPerPlyM
		remaining := roll.UsableLengthM - used
		if remaining < 0 && remaining > -1e-8 {
			remaining = 0
		}
		out.Allocations = append(out.Allocations, RollAllocation{RollID: roll.RollID, MarkerID: best.MarkerID, Plies: bestPlies, Pieces: bestPieces, UsedLengthM: used, RemainingLengthM: remaining})
		out.PlannedPieces += bestPieces
	}
	out.UnfilledPieces = pieces - out.PlannedPieces
	if out.UnfilledPieces < 0 {
		out.Questions = append(out.Questions, "Approved over-cut quantity is needed before cutting extra pieces")
	}
	if out.UnfilledPieces > 0 {
		out.Questions = append(out.Questions, "Not enough approved fabric and marker capacity. Can we add stock or change the plan?")
	}
	for _, a := range out.Allocations {
		out.EstimatedUnusedLengthM += a.RemainingLengthM
	}
	return out, nil
}
