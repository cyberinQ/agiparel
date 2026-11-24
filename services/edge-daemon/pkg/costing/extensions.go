// Author: Sunil Jain, CyberinQ LLC (Generated 2026-11-24)
package costing

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type CutPreviewRule struct {
	AllowancePct       float64 `json:"allowance_pct"`
	MinExtraPcs        int     `json:"min_extra_pcs"`
	MaxExtraPcs        int     `json:"max_extra_pcs"`
	MaxPlies           int     `json:"max_plies"`
	BundleSize         int     `json:"bundle_size"`
	CutPartsPerGarment int     `json:"cut_parts_per_garment"`
	MinEconomicCutPcs  int     `json:"min_economic_cut_pcs"`
	RFIDTagUnitCost    float64 `json:"rfid_tag_unit_cost"`
}

type CutSizeBreakdown struct {
	Colour           string `json:"colour"`
	Size             string `json:"size"`
	OrderQty         int    `json:"order_qty"`
	CutQty           int    `json:"cut_qty"`
	EstimatedCuts    int    `json:"estimated_cuts"`
	TailCutPcs       int    `json:"tail_cut_pcs"`
	EstimatedBundles int    `json:"estimated_bundles"`
}

type CutTagPreviewResult struct {
	TotalOrderQty      int                `json:"total_order_qty"`
	TotalCutQty        int                `json:"total_cut_qty"`
	EstimatedCuts      int                `json:"estimated_cuts"`
	EstimatedBundles   int                `json:"estimated_bundles"`
	GarmentRFIDTags    int                `json:"garment_rfid_tags"`
	BundleRFIDTags     int                `json:"bundle_rfid_tags"`
	TotalRFIDTags      int                `json:"total_rfid_tags"`
	RFIDCostPerOrderPc float64            `json:"rfid_cost_per_order_pc"`
	Warnings           []string           `json:"warnings"`
	Breakdown          []CutSizeBreakdown `json:"breakdown"`
}

func ComputeCutAndTagPreview(lines []OrderMatrixLine, rule CutPreviewRule) (CutTagPreviewResult, error) {
	var out CutTagPreviewResult
	if len(lines) == 0 {
		return out, errors.New("ERR_FR9_EMPTY_ORDER_MATRIX")
	}
	if rule.MaxPlies <= 0 {
		rule.MaxPlies = 210
	}
	if rule.BundleSize <= 0 {
		rule.BundleSize = 20
	}
	if rule.CutPartsPerGarment <= 0 {
		rule.CutPartsPerGarment = 16
	}
	if rule.MinEconomicCutPcs <= 0 {
		rule.MinEconomicCutPcs = 10
	}

	sorted := append([]OrderMatrixLine(nil), lines...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Colour == sorted[j].Colour {
			return sorted[i].Size < sorted[j].Size
		}
		return sorted[i].Colour < sorted[j].Colour
	})

	for _, l := range sorted {
		if l.Qty <= 0 {
			return out, errors.New("ERR_FR9_NON_POSITIVE_QTY")
		}
		cutQty := ComputeCutAllowance(l.Qty, rule.AllowancePct, rule.MinExtraPcs, rule.MaxExtraPcs)
		cuts := int(math.Ceil(float64(cutQty) / float64(rule.MaxPlies)))
		if cuts < 1 {
			cuts = 1
		}
		tail := cutQty % rule.MaxPlies
		if tail > 0 && tail < rule.MinEconomicCutPcs {
			out.Warnings = append(out.Warnings, fmt.Sprintf("FR-9 Uneconomic tail cut warning: %s %s has a %d-pc remainder cut (< %d pcs)", l.Colour, l.Size, tail, rule.MinEconomicCutPcs))
		}
		bundles := int(math.Ceil(float64(cutQty) / float64(rule.BundleSize)))

		out.TotalOrderQty += l.Qty
		out.TotalCutQty += cutQty
		out.EstimatedCuts += cuts
		out.EstimatedBundles += bundles
		out.Breakdown = append(out.Breakdown, CutSizeBreakdown{
			Colour:           strings.ToUpper(strings.TrimSpace(l.Colour)),
			Size:             strings.ToUpper(strings.TrimSpace(l.Size)),
			OrderQty:         l.Qty,
			CutQty:           cutQty,
			EstimatedCuts:    cuts,
			TailCutPcs:       tail,
			EstimatedBundles: bundles,
		})
	}

	out.GarmentRFIDTags = out.TotalCutQty
	out.BundleRFIDTags = out.EstimatedBundles * rule.CutPartsPerGarment
	out.TotalRFIDTags = out.GarmentRFIDTags + out.BundleRFIDTags
	if out.TotalOrderQty > 0 && rule.RFIDTagUnitCost > 0 {
		out.RFIDCostPerOrderPc = round6((float64(out.TotalRFIDTags) * rule.RFIDTagUnitCost) / float64(out.TotalOrderQty))
	}
	return out, nil
}

type CartonAssortmentRatio struct {
	Colour    string `json:"colour"`
	Size      string `json:"size"`
	PcsPerBox int    `json:"pcs_per_box"`
}

type CartonPlanLine struct {
	BoxRangeLabel string                  `json:"box_range_label"`
	BoxCount      int                     `json:"box_count"`
	LengthCM      float64                 `json:"length_cm"`
	WidthCM       float64                 `json:"width_cm"`
	HeightCM      float64                 `json:"height_cm"`
	GrossWeightKG float64                 `json:"gross_weight_kg"`
	CartonCost    float64                 `json:"carton_cost"`
	Assortment    []CartonAssortmentRatio `json:"assortment"`
}

type CartonPlanResult struct {
	AssortmentMode     string           `json:"assortment_mode"`
	TotalOrderQty      int              `json:"total_order_qty"`
	PackedQty          int              `json:"packed_qty"`
	DeclaredCartons    int              `json:"declared_cartons"`
	ComputedCartons    int              `json:"computed_cartons"`
	TotalCBM           float64          `json:"total_cbm"`
	TotalGrossWeightKG float64          `json:"total_gross_weight_kg"`
	TotalCartonCost    float64          `json:"total_carton_cost"`
	CartonCostPerPc    float64          `json:"carton_cost_per_pc"`
	IsComplete         bool             `json:"is_complete"`
	Gaps               []string         `json:"gaps"`
	Lines              []CartonPlanLine `json:"lines"`
}

func ComputeCartonPlanPreview(matrix []OrderMatrixLine, declaredCartons int, plans []CartonPlanLine) (CartonPlanResult, error) {
	var out CartonPlanResult
	out.DeclaredCartons = declaredCartons
	out.Lines = plans

	expected := make(map[string]int)
	for _, m := range matrix {
		key := strings.ToUpper(strings.TrimSpace(m.Colour)) + ":" + strings.ToUpper(strings.TrimSpace(m.Size))
		expected[key] += m.Qty
		out.TotalOrderQty += m.Qty
	}
	if out.TotalOrderQty <= 0 {
		return out, errors.New("ERR_FR10_EMPTY_ORDER_MATRIX")
	}

	packed := make(map[string]int)
	hasSolid, hasAssorted := false, false
	for _, p := range plans {
		if p.BoxCount <= 0 {
			return out, errors.New("ERR_FR10_BOX_COUNT_MUST_BE_POSITIVE")
		}
		if len(p.Assortment) > 1 {
			hasAssorted = true
		} else {
			hasSolid = true
		}
		out.ComputedCartons += p.BoxCount
		cbmPerBox := (p.LengthCM * p.WidthCM * p.HeightCM) / 1000000.0
		out.TotalCBM += cbmPerBox * float64(p.BoxCount)
		out.TotalGrossWeightKG += p.GrossWeightKG * float64(p.BoxCount)
		out.TotalCartonCost += p.CartonCost * float64(p.BoxCount)

		for _, a := range p.Assortment {
			if a.PcsPerBox <= 0 {
				return out, errors.New("ERR_FR10_PCS_PER_BOX_MUST_BE_POSITIVE")
			}
			qty := p.BoxCount * a.PcsPerBox
			key := strings.ToUpper(strings.TrimSpace(a.Colour)) + ":" + strings.ToUpper(strings.TrimSpace(a.Size))
			packed[key] += qty
			out.PackedQty += qty
		}
	}

	switch {
	case hasSolid && hasAssorted:
		out.AssortmentMode = "MIXED"
	case hasAssorted:
		out.AssortmentMode = "ASSORTED"
	default:
		out.AssortmentMode = "SOLID"
	}

	if declaredCartons > 0 && declaredCartons != out.ComputedCartons {
		out.Gaps = append(out.Gaps, fmt.Sprintf("V11 Carton count mismatch: declared %d boxes vs defined %d boxes", declaredCartons, out.ComputedCartons))
	}

	allKeys := make(map[string]bool)
	for k := range expected {
		allKeys[k] = true
	}
	for k := range packed {
		allKeys[k] = true
	}
	keys := make([]string, 0, len(allKeys))
	for k := range allKeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		exp := expected[k]
		got := packed[k]
		if exp != got {
			out.Gaps = append(out.Gaps, fmt.Sprintf("V11 Quantity gap for %s: ordered %d pcs vs packed %d pcs (delta %+d)", k, exp, got, got-exp))
		}
	}

	out.TotalCBM = round4(out.TotalCBM)
	out.TotalGrossWeightKG = round4(out.TotalGrossWeightKG)
	out.TotalCartonCost = round4(out.TotalCartonCost)
	out.CartonCostPerPc = round6(out.TotalCartonCost / float64(out.TotalOrderQty))
	out.IsComplete = len(out.Gaps) == 0
	return out, nil
}

type SamplingMilestone struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	DaysBeforeCut int    `json:"days_before_cut"`
	DueDate       string `json:"due_date"`
	Status        string `json:"status"`
	FlagReason    string `json:"flag_reason,omitempty"`
}

type SamplingTNAResult struct {
	OrderDate              string              `json:"order_date"`
	PlanCutDate            string              `json:"plan_cut_date"`
	HasPastOrPreOrderFlags bool                `json:"has_past_or_pre_order_flags"`
	Milestones             []SamplingMilestone `json:"milestones"`
}

func ComputeSamplingTNA(orderDate, planCutDate, today time.Time, offsetsDays map[string]int) SamplingTNAResult {
	defaults := []struct {
		code string
		name string
		days int
	}{
		{"PROTO", "Proto Sample", 42},
		{"FIT", "Fit Sample", 32},
		{"SIZE_SET", "Size Set Sample", 21},
		{"PP", "Pre-Production (PP) Sample", 14},
		{"TOP", "Top of Production (TOP) Sample", 0},
	}

	ordDay := time.Date(orderDate.Year(), orderDate.Month(), orderDate.Day(), 0, 0, 0, 0, time.UTC)
	cutDay := time.Date(planCutDate.Year(), planCutDate.Month(), planCutDate.Day(), 0, 0, 0, 0, time.UTC)
	nowDay := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	res := SamplingTNAResult{
		OrderDate:   ordDay.Format("2006-01-02"),
		PlanCutDate: cutDay.Format("2006-01-02"),
	}

	for _, d := range defaults {
		offset := d.days
		if custom, ok := offsetsDays[d.code]; ok {
			offset = custom
		}
		due := cutDay.AddDate(0, 0, -offset)
		status := "ON_TRACK"
		flag := ""
		if due.Before(ordDay) {
			status = "BEFORE_ORDER_DATE"
			flag = fmt.Sprintf("FR-14 %s (%s) falls before order date (%s)", d.name, due.Format("2006-01-02"), ordDay.Format("2006-01-02"))
			res.HasPastOrPreOrderFlags = true
		} else if due.Before(nowDay) {
			status = "IN_PAST"
			flag = fmt.Sprintf("FR-14 %s (%s) is already in the past", d.name, due.Format("2006-01-02"))
			res.HasPastOrPreOrderFlags = true
		}
		res.Milestones = append(res.Milestones, SamplingMilestone{
			Code:          d.code,
			Name:          d.name,
			DaysBeforeCut: offset,
			DueDate:       due.Format("2006-01-02"),
			Status:        status,
			FlagReason:    flag,
		})
	}
	return res
}

type OCRActualInput struct {
	POID                string    `json:"po_id"`
	CostedQty           int       `json:"costed_qty"`
	ActualShippedQty    int       `json:"actual_shipped_qty"`
	CostedSAM           float64   `json:"costed_sam"`
	ActualAvgSAM        float64   `json:"actual_avg_sam"`
	CostedCMTotal       float64   `json:"costed_cm_total"`
	ActualCMTotal       float64   `json:"actual_cm_total"`
	CostedMaterialTotal float64   `json:"costed_material_total"`
	ActualMaterialTotal float64   `json:"actual_material_total"`
	QuotedExFactory     time.Time `json:"quoted_ex_factory"`
	ActualExFactory     time.Time `json:"actual_ex_factory"`
}

type OCRActualVariance struct {
	OCRActualInput
	SAMVariancePct      float64 `json:"sam_variance_pct"`
	CMVariancePct       float64 `json:"cm_variance_pct"`
	MaterialVariancePct float64 `json:"material_variance_pct"`
	ScheduleVarianceDay int     `json:"schedule_variance_days"`
	ShippedOnTime       bool    `json:"shipped_on_time"`
	WithinKPIBand       bool    `json:"within_kpi_band"`
}

func ComputeOCRCostedVsActual(in OCRActualInput) (OCRActualVariance, error) {
	var out OCRActualVariance
	if strings.TrimSpace(in.POID) == "" || in.CostedCMTotal <= 0 || in.CostedMaterialTotal <= 0 || in.CostedSAM <= 0 {
		return out, errors.New("ERR_FR15_INVALID_COSTED_BASELINE")
	}
	out.OCRActualInput = in
	out.SAMVariancePct = round4(((in.ActualAvgSAM - in.CostedSAM) / in.CostedSAM) * 100.0)
	out.CMVariancePct = round4(((in.ActualCMTotal - in.CostedCMTotal) / in.CostedCMTotal) * 100.0)
	out.MaterialVariancePct = round4(((in.ActualMaterialTotal - in.CostedMaterialTotal) / in.CostedMaterialTotal) * 100.0)

	qDay := time.Date(in.QuotedExFactory.Year(), in.QuotedExFactory.Month(), in.QuotedExFactory.Day(), 0, 0, 0, 0, time.UTC)
	aDay := time.Date(in.ActualExFactory.Year(), in.ActualExFactory.Month(), in.ActualExFactory.Day(), 0, 0, 0, 0, time.UTC)
	out.ScheduleVarianceDay = int(aDay.Sub(qDay).Hours() / 24)
	out.ShippedOnTime = out.ScheduleVarianceDay <= 0
	out.WithinKPIBand = out.ShippedOnTime && math.Abs(out.CMVariancePct) <= 5.0
	return out, nil
}
