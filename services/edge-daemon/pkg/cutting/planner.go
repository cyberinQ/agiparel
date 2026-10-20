package cutting

import (
	"errors"
	"math"
)

type CutPlanTarget struct {
	OrderID         string `json:"order_id"`
	SizeVariant     string `json:"size_variant"`
	OrderQty        int    `json:"order_qty"`
	ClampedExtra    int    `json:"clamped_extra"`
	TargetCutVolume int    `json:"target_cut_volume"`
	MaxAllowedLays  int    `json:"max_allowed_lays"`
}

// CalculateCutTarget computes the over-cut allowance using the clamped 2% formula.
func CalculateCutTarget(orderID, sizeVariant string, orderQty int) (CutPlanTarget, error) {
	if orderQty <= 0 {
		return CutPlanTarget{}, errors.New("ERR_INVALID_ORDER_QTY: Order quantity must be greater than zero")
	}

	extra := int(math.Round(0.02 * float64(orderQty)))
	if extra < 1 {
		extra = 1
	}
	if extra > 15 {
		extra = 15
	}

	return CutPlanTarget{
		OrderID:         orderID,
		SizeVariant:     sizeVariant,
		OrderQty:        orderQty,
		ClampedExtra:    extra,
		TargetCutVolume: orderQty + extra,
		MaxAllowedLays:  210,
	}, nil
}