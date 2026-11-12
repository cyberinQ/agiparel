// Author: Sunil Jain, 2026-11-11 — Deterministic, bounded SOP-2 cutting target optimizer.
// Recommendations do not represent production cuts, approvals or stock movements.
package sop2

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// SKU keys are canonical "color|size" identifiers; ordered quantities come from the buyer PO.
type SKU struct {
	Key        string `json:"key"`
	Ordered    int64  `json:"ordered"`
	MinExtraBP int64  `json:"min_extra_bp"` // basis points; 100 == 1%.
	MaxExtraBP int64  `json:"max_extra_bp"`
}

type Marker struct {
	ID            string           `json:"id"`
	FabricSKU     string           `json:"fabric_sku"`
	Shade         string           `json:"shade"`
	LengthMM      int64            `json:"length_mm"`
	WidthMM       int64            `json:"width_mm"`
	TableLengthMM int64            `json:"table_length_mm"`
	MaxPlies      int64            `json:"max_plies"`
	MinutesPerCut int64            `json:"minutes_per_cut"`
	Ratios        map[string]int64 `json:"ratios"`
	Approved      bool             `json:"approved"`
}

type TargetRequest struct {
	SKUs         []SKU    `json:"skus"`
	Markers      []Marker `json:"markers"`
	POMinExtraBP int64    `json:"po_min_extra_bp"`
	POMaxExtraBP int64    `json:"po_max_extra_bp"`
	MaxCuts      int      `json:"max_cuts"`
	MaxStates    int      `json:"max_states"`
}

type CutStep struct {
	MarkerID string           `json:"marker_id"`
	Plies    int64            `json:"plies"`
	Yield    map[string]int64 `json:"yield"`
	FabricMM int64            `json:"fabric_mm"`
	Minutes  int64            `json:"minutes"`
}

type TargetResult struct {
	Feasible         bool             `json:"feasible"`
	OptimalityProven bool             `json:"optimality_proven"`
	SearchTruncated  bool             `json:"search_truncated"`
	Objective        string           `json:"objective"`
	Frozen           bool             `json:"frozen"`
	MaterialVerified bool             `json:"material_verified"`
	TargetBySKU      map[string]int64 `json:"target_by_sku"`
	ExtrasBySKU      map[string]int64 `json:"extras_by_sku"`
	Steps            []CutStep        `json:"steps"`
	Cuts             int              `json:"cuts"`
	FabricMM         int64            `json:"fabric_mm"`
	Minutes          int64            `json:"minutes"`
	Reason           string           `json:"reason,omitempty"`
}

type searchState struct {
	qty             []int64
	length, minutes int64
	steps           []CutStep
}

func ceilBP(q, bp int64) int64 { return (q*bp + 9999) / 10000 }
func qtyKey(v []int64) string {
	var b strings.Builder
	for _, n := range v {
		b.WriteString(strconv.FormatInt(n, 10))
		b.WriteByte('/')
	}
	return b.String()
}
func cheaper(a, b searchState) bool {
	return a.length < b.length || (a.length == b.length && a.minutes < b.minutes)
}

// SolveTargets minimizes cuts, then marker-length consumption, then cutting time.
// It is an exact discrete search *only* for the supplied approved markers and ply
// capacities. Roll availability, defect avoidance and actual cutter throughput are
// not inferred. Exhausted search budgets return an explicitly unproven result.
func SolveTargets(in TargetRequest) (TargetResult, error) {
	result := TargetResult{Objective: "MIN_CUTS_THEN_FABRIC_MM_THEN_MINUTES", Frozen: false, MaterialVerified: false, TargetBySKU: map[string]int64{}, ExtrasBySKU: map[string]int64{}}
	if len(in.SKUs) == 0 || len(in.SKUs) > 12 || len(in.Markers) == 0 || len(in.Markers) > 24 {
		return result, errors.New("SOP2_SKUS_OR_MARKERS_OUT_OF_RANGE")
	}
	if in.POMinExtraBP < 0 || in.POMaxExtraBP > 10000 || in.POMinExtraBP > in.POMaxExtraBP {
		return result, errors.New("SOP2_PO_EXTRA_BAND_INVALID")
	}
	if in.MaxCuts == 0 {
		in.MaxCuts = 24
	}
	if in.MaxStates == 0 {
		in.MaxStates = 40000
	}
	if in.MaxCuts < 1 || in.MaxCuts > 100 || in.MaxStates < 1 || in.MaxStates > 200000 {
		return result, errors.New("SOP2_SEARCH_LIMIT_INVALID")
	}
	skus := append([]SKU(nil), in.SKUs...)
	sort.Slice(skus, func(i, j int) bool { return skus[i].Key < skus[j].Key })
	minV, maxV := make([]int64, len(skus)), make([]int64, len(skus))
	idx := make(map[string]int)
	var ordered, minPO, maxPO int64
	for i, s := range skus {
		if strings.TrimSpace(s.Key) == "" || strings.ContainsAny(s.Key, "\x00\r\n") || s.Ordered <= 0 || s.Ordered > 10000000 || s.MinExtraBP < 0 || s.MaxExtraBP > 10000 || s.MinExtraBP > s.MaxExtraBP {
			return result, errors.New("SOP2_SKU_OR_EXTRA_BAND_INVALID")
		}
		if _, found := idx[s.Key]; found {
			return result, errors.New("SOP2_DUPLICATE_SKU")
		}
		idx[s.Key] = i
		minV[i] = s.Ordered + ceilBP(s.Ordered, s.MinExtraBP)
		maxV[i] = s.Ordered + ceilBP(s.Ordered, s.MaxExtraBP)
		ordered += s.Ordered
		minPO += minV[i]
		maxPO += maxV[i]
	}
	requiredPO := ordered + ceilBP(ordered, in.POMinExtraBP)
	allowedPO := ordered + ceilBP(ordered, in.POMaxExtraBP)
	if requiredPO > maxPO || allowedPO < minPO {
		result.Reason = "EXTRA_BANDS_INFEASIBLE"
		return result, nil
	}
	type action struct {
		step       CutStep
		increments []int64
	}
	actions := make([]action, 0)
	sort.Slice(in.Markers, func(i, j int) bool { return in.Markers[i].ID < in.Markers[j].ID })
	seenMarkers := map[string]bool{}
	for _, m := range in.Markers {
		if seenMarkers[m.ID] || m.ID == "" || !m.Approved || m.LengthMM <= 0 || m.WidthMM <= 0 || m.TableLengthMM < m.LengthMM || m.MaxPlies < 1 || m.MaxPlies > 210 || m.MinutesPerCut < 1 || len(m.Ratios) == 0 {
			return result, fmt.Errorf("SOP2_MARKER_INVALID: %s", m.ID)
		}
		seenMarkers[m.ID] = true
		vec := make([]int64, len(skus))
		for key, ratio := range m.Ratios {
			i, ok := idx[key]
			if !ok || ratio < 1 || ratio > 80 {
				return result, errors.New("SOP2_MARKER_RATIO_INVALID")
			}
			vec[i] = ratio
		}
		for plies := int64(1); plies <= m.MaxPlies; plies++ {
			out, count := make(map[string]int64), int64(0)
			inc := make([]int64, len(vec))
			for i, r := range vec {
				inc[i] = r * plies
				if inc[i] > maxV[i] {
					count = -1
					break
				}
				if r > 0 {
					out[skus[i].Key] = inc[i]
					count += inc[i]
				}
			}
			if count < 0 {
				break
			}
			actions = append(actions, action{CutStep{MarkerID: m.ID, Plies: plies, Yield: out, FabricMM: m.LengthMM * plies, Minutes: m.MinutesPerCut}, inc})
		}
	}
	if len(actions) == 0 {
		result.Reason = "NO_FEASIBLE_MARKER_ACTIONS"
		return result, nil
	}
	zero := make([]int64, len(skus))
	states := map[string]searchState{qtyKey(zero): {qty: zero, steps: []CutStep{}}}
	visited := 0
	attempts := 0
	complete := func(q []int64) bool {
		total := int64(0)
		for i, n := range q {
			if n < minV[i] || n > maxV[i] {
				return false
			}
			total += n
		}
		return total >= requiredPO && total <= allowedPO
	}
	for depth := 1; depth <= in.MaxCuts; depth++ {
		next := make(map[string]searchState)
		for _, state := range states {
			for _, a := range actions {
				qty := make([]int64, len(skus))
				total := int64(0)
				ok := true
				for i := range qty {
					qty[i] = state.qty[i] + a.increments[i]
					if qty[i] > maxV[i] {
						ok = false
						break
					}
					total += qty[i]
				}
				if !ok || total > allowedPO {
					continue
				}
				cand := searchState{qty: qty, length: state.length + a.step.FabricMM, minutes: state.minutes + a.step.Minutes, steps: append(append([]CutStep(nil), state.steps...), a.step)}
				key := qtyKey(qty)
				if old, exists := next[key]; !exists || cheaper(cand, old) {
					if !exists {
						visited++
					}
					next[key] = cand
				}
				attempts++
				if visited > in.MaxStates || attempts > in.MaxStates*100 {
					result.SearchTruncated = true
					result.Reason = "SEARCH_LIMIT_REACHED_MORE_CAPACITY_OR_SMALLER_PO_REQUIRED"
					break
				}
			}
			if result.SearchTruncated {
				break
			}
		}
		var best *searchState
		for _, s := range next {
			if complete(s.qty) && (best == nil || cheaper(s, *best)) {
				cp := s
				best = &cp
			}
		}
		if best != nil {
			for i, s := range skus {
				result.TargetBySKU[s.Key] = best.qty[i]
				result.ExtrasBySKU[s.Key] = best.qty[i] - s.Ordered
			}
			result.Feasible = true
			result.Steps = best.steps
			result.Cuts = depth
			result.FabricMM = best.length
			result.Minutes = best.minutes
			result.OptimalityProven = !result.SearchTruncated
			if result.SearchTruncated {
				result.Reason = "FEASIBLE_BUT_UNPROVEN_SEARCH_TRUNCATED"
			}
			return result, nil
		}
		if result.SearchTruncated {
			return result, nil
		}
		if len(next) == 0 {
			result.Reason = "DISCRETE_TARGETS_UNREACHABLE"
			return result, nil
		}
		states = next
	}
	result.Reason = "NO_FEASIBLE_PLAN_WITHIN_CUT_LIMIT"
	return result, nil
}

// ValidateFrozenTargets checks the proposed frozen quantities against authoritative
// buyer order quantities and both per-SKU and PO-wide extra bands.
func ValidateFrozenTargets(skus []SKU, targets map[string]int64, minPOBP, maxPOBP int64) error {
	if len(skus) == 0 || len(skus) != len(targets) || minPOBP < 0 || maxPOBP > 10000 || minPOBP > maxPOBP {
		return errors.New("TARGET_MATRIX_INVALID")
	}
	ordered, total := int64(0), int64(0)
	for _, sku := range skus {
		if sku.Key == "" || sku.Ordered <= 0 || sku.Ordered > 10000000 || sku.MinExtraBP < 0 || sku.MaxExtraBP > 10000 || sku.MinExtraBP > sku.MaxExtraBP {
			return errors.New("TARGET_BAND_INVALID")
		}
		t, ok := targets[sku.Key]
		if !ok || t < sku.Ordered+ceilBP(sku.Ordered, sku.MinExtraBP) || t > sku.Ordered+ceilBP(sku.Ordered, sku.MaxExtraBP) {
			return errors.New("TARGET_OUTSIDE_SKU_BAND")
		}
		ordered += sku.Ordered
		total += t
	}
	if total < ordered+ceilBP(ordered, minPOBP) || total > ordered+ceilBP(ordered, maxPOBP) {
		return errors.New("TARGET_OUTSIDE_PO_BAND")
	}
	return nil
}

// Remaining never goes negative; actual plies, not planned plies, must drive confirmation.
func Remaining(targets, confirmed map[string]int64) (map[string]int64, error) {
	result := make(map[string]int64, len(targets))
	for k, n := range confirmed {
		if _, ok := targets[k]; !ok || n < 0 {
			return nil, errors.New("UNKNOWN_OR_NEGATIVE_CONFIRMED_SKU")
		}
	}
	for k, t := range targets {
		if t < 0 || confirmed[k] > t {
			return nil, errors.New("OVER_CUT_FROZEN_TARGET")
		}
		result[k] = t - confirmed[k]
	}
	return result, nil
}
