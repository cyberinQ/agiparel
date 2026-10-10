package packing

import (
	"errors"
	"fmt"
	"sync"
)

type RatioSlot struct {
	UPC       string `json:"upc"`
	TargetQty int    `json:"target_qty"`
	PackedQty int    `json:"packed_qty"`
}

type CartonPackingSession struct {
	sync.Mutex
	BoxID            int          `json:"box_id"`
	PackType         string       `json:"pack_type"`
	Slots            []*RatioSlot `json:"slots"`
	ExpectedWeightKg float64      `json:"expected_weight_kg"`
	ToleranceKg      float64      `json:"tolerance_kg"`
	IsSealed         bool         `json:"is_sealed"`
}

func NewCartonPackingSession(boxID int, packType string, slots []*RatioSlot, targetWeightKg, tolKg float64) *CartonPackingSession {
	return &CartonPackingSession{
		BoxID:            boxID,
		PackType:         packType,
		Slots:            slots,
		ExpectedWeightKg: targetWeightKg,
		ToleranceKg:      tolKg,
		IsSealed:         false,
	}
}

func (s *CartonPackingSession) ScanItemUPC(upc string) error {
	s.Lock()
	defer s.Unlock()

	if s.IsSealed {
		return errors.New("ERR_CARTON_SEALED: Carton session already completed")
	}

	for _, slot := range s.Slots {
		if slot.UPC == upc {
			if slot.PackedQty >= slot.TargetQty {
				return fmt.Errorf("ERR_SLOT_OVERFLOW: Target quantity (%d) for UPC %s already satisfied",
					slot.TargetQty, upc)
			}
			slot.PackedQty++
			return nil
		}
	}

	return fmt.Errorf("ERR_INVALID_UPC: UPC %s does not belong to this carton manifest", upc)
}

func (s *CartonPackingSession) VerifyAndSeal(scaleWeightKg float64) error {
	s.Lock()
	defer s.Unlock()

	for _, slot := range s.Slots {
		if slot.PackedQty != slot.TargetQty {
			return fmt.Errorf("ERR_UNFINISHED_SLOT: UPC %s has %d of %d pieces packed",
				slot.UPC, slot.PackedQty, slot.TargetQty)
		}
	}

	minWeight := s.ExpectedWeightKg - s.ToleranceKg
	maxWeight := s.ExpectedWeightKg + s.ToleranceKg
	if scaleWeightKg < minWeight || scaleWeightKg > maxWeight {
		return fmt.Errorf("ERR_WEIGHT_MISMATCH: Measured %.3f kg outside valid bounds [%.3f - %.3f] kg",
			scaleWeightKg, minWeight, maxWeight)
	}

	s.IsSealed = true
	return nil
}