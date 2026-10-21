package cutting

import (
	"errors"
	"math"
)

type CutPlanRequest struct {
	OrderQty      int     `json:"order_qty"`
	MaxLayHeight  int     `json:"max_lay_height"` // 210 ply ceiling
	FabricMeters  float64 `json:"fabric_meters"`
	Consumption   float64 `json:"consumption_per_garment"` // meters
}

type CutPlanResult struct {
	OrderQty        int `json:"order_qty"`
	ClampedExtra    int `json:"clamped_extra"`
	TargetCutVolume int `json:"target_cut_volume"`
	PlannedLays     int `json:"planned_lays"`
}

// CalculateClampedCutPlan enforces the 210 lay limit and the 2% buffer clamped to [1, 15] pcs
func CalculateClampedCutPlan(req CutPlanRequest) (*CutPlanResult, error) {
	if req.OrderQty <= 0 {
		return nil, errors.New("order quantity must be > 0")
	}

	// Clamp(round(0.02 * OrderQty), 1, 15)
	rawExtra := int(math.Round(0.02 * float64(req.OrderQty)))
	clampedExtra := rawExtra
	if clampedExtra < 1 {
		clampedExtra = 1
	} else if clampedExtra > 15 {
		clampedExtra = 15
	}

	targetCut := req.OrderQty + clampedExtra
	maxPlies := req.MaxLayHeight
	if maxPlies <= 0 || maxPlies > 210 {
		maxPlies = 210
	}

	plannedLays := targetCut
	if plannedLays > maxPlies {
		plannedLays = maxPlies
	}

	return &CutPlanResult{
		OrderQty:        req.OrderQty,
		ClampedExtra:    clampedExtra,
		TargetCutVolume: targetCut,
		PlannedLays:     plannedLays,
	}, nil
}

type EndBitDisposition string

const (
	DispositionScrap       EndBitDisposition = "SCRAP"
	DispositionTableRecut  EndBitDisposition = "TABLE_RECUT"
	DispositionStoreReturn EndBitDisposition = "STORE_RETURN"
)

// ClassifyEndBit routes leftover roll fabric according to PRD Invariant
func ClassifyEndBit(remainingMeters float64) EndBitDisposition {
	if remainingMeters < 1.5 {
		return DispositionScrap
	} else if remainingMeters <= 5.0 {
		return DispositionTableRecut
	}
	return DispositionStoreReturn
}