package cutting

import (
	"errors"
	"fmt"
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

func CalculateCutTarget(orderID, sizeVariant string, orderQty int) CutPlanTarget {
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
	}
}

type LayRollRecord struct {
	RollID         string  `json:"roll_id"`
	OrderID        string  `json:"order_id"`
	CutNo          int     `json:"cut_no"`
	LayStart       int     `json:"lay_start"`
	LayEnd         int     `json:"lay_end"`
	PliesCount     int     `json:"plies_count"`
	EndBitLengthM  float64 `json:"end_bit_length_m"`
	EndBitAction   string  `json:"end_bit_action"`
}

func EvaluateEndBitDisposition(rollID, orderID string, cutNo, layStart, layEnd int, endBitM float64) (*LayRollRecord, error) {
	if layStart <= 0 || layEnd < layStart {
		return nil, errors.New("ERR_INVALID_LAY_RANGE: lay_start must be > 0 and lay_end >= lay_start")
	}

	plies := layEnd - layStart + 1
	if plies > 210 {
		return nil, fmt.Errorf("ERR_EXCEEDS_MAX_LAYS: %d plies exceeds maximum threshold of 210", plies)
	}

	var action string
	if endBitM < 1.5 {
		action = "SCRAP"
	} else if endBitM <= 5.0 {
		action = "RECUT_HOLD"
	} else {
		action = "STORE_RETURN"
	}

	return &LayRollRecord{
		RollID:        rollID,
		OrderID:       orderID,
		CutNo:         cutNo,
		LayStart:      layStart,
		LayEnd:        layEnd,
		PliesCount:    plies,
		EndBitLengthM: endBitM,
		EndBitAction:  action,
	}, nil
}