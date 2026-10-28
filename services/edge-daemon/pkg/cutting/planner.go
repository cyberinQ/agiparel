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

	// Real-world apparel cutting: 2% allowance with realistic tiered thresholds
	extra := int(math.Round(0.02 * float64(orderQty)))
	if extra < 1 {
		extra = 1
	}
	// Scale over-cut allowance proportionally: allow up to 3% for small orders, capped at 300 for huge runs
	maxAllowedExtra := int(math.Max(15.0, float64(orderQty)*0.03))
	if maxAllowedExtra > 500 {
		maxAllowedExtra = 500
	}
	if extra > maxAllowedExtra {
		extra = maxAllowedExtra
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

type SequentialBundleSpec struct {
	SequentialBundleNo int    `json:"sequential_bundle_no"`
	OrderID            string `json:"order_id"`
	CutNo              int    `json:"cut_no"`
	PantPosition       int    `json:"pant_position"`
	PartCode           string `json:"part_code"`
	PartDescription    string `json:"part_description"`
	SSBPlyGroup        int    `json:"ssb_ply_group"`
	PlyStart           int    `json:"ply_start"`
	PlyEnd             int    `json:"ply_end"`
	PieceCount         int    `json:"piece_count"`
	SerialStart        int    `json:"serial_start"`
	SerialEnd          int    `json:"serial_end"`
}

// GeneratePOSequentialBundles creates sequential bundle cards 1..N based on actual cut plies
func GeneratePOSequentialBundles(orderID string, cutNo, pantPositions int, parts []string, totalPlies, bundleSize int) []SequentialBundleSpec {
	if pantPositions <= 0 || totalPlies <= 0 || bundleSize <= 0 || len(parts) == 0 {
		return []SequentialBundleSpec{}
	}
	var bundles []SequentialBundleSpec
	seq := 1
	numGroups := (totalPlies + bundleSize - 1) / bundleSize

	for pant := 1; pant <= pantPositions; pant++ {
		for g := 0; g < numGroups; g++ {
			pStart := g*bundleSize + 1
			pEnd := (g + 1) * bundleSize
			if pEnd > totalPlies {
				pEnd = totalPlies
			}
			count := pEnd - pStart + 1
			for _, part := range parts {
				bundles = append(bundles, SequentialBundleSpec{
					SequentialBundleNo: seq,
					OrderID:            orderID,
					CutNo:              cutNo,
					PantPosition:       pant,
					PartCode:           part,
					PartDescription:    part,
					SSBPlyGroup:        g + 1,
					PlyStart:           pStart,
					PlyEnd:             pEnd,
					PieceCount:         count,
					SerialStart:        (pant-1)*totalPlies + pStart,
					SerialEnd:          (pant-1)*totalPlies + pEnd,
				})
				seq++
			}
		}
	}
	return bundles
}

type LayRollRecord struct {
	RollID        string  `json:"roll_id"`
	OrderID       string  `json:"order_id"`
	CutNo         int     `json:"cut_no"`
	LayStart      int     `json:"lay_start"`
	LayEnd        int     `json:"lay_end"`
	PliesCount    int     `json:"plies_count"`
	EndBitLengthM float64 `json:"end_bit_length_m"`
	EndBitAction  string  `json:"end_bit_action"`
}

func EvaluateEndBitDisposition(rollID, orderID string, cutNo, layStart, layEnd int, endBitM float64) (*LayRollRecord, error) {
	if layStart <= 0 || layEnd < layStart {
		return nil, errors.New("ERR_INVALID_LAY_RANGE: lay_start must be > 0 and lay_end >= lay_start")
	}
	if endBitM < 0 {
		return nil, errors.New("ERR_INVALID_ENDBIT: end_bit_length_m cannot be negative")
	}
	plies := layEnd - layStart + 1
	if plies > 210 {
		return nil, errors.New("ERR_EXCEEDS_MAX_LAYS: plies exceed maximum threshold of 210")
	}
	action := "STORE_RETURN"
	if endBitM < 1.5 {
		action = "SCRAP"
	} else if endBitM <= 5.0 {
		action = "RECUT_HOLD"
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
