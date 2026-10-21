package cutting

import "github.com/cyberinq/agiparel/services/edge-daemon/pkg/cutting"

type CutPlanTarget = cutting.CutPlanTarget
type SequentialBundleSpec = cutting.SequentialBundleSpec
type LayRollRecord = cutting.LayRollRecord

func CalculateCutTarget(orderID, sizeVariant string, orderQty int) (CutPlanTarget, error) {
	return cutting.CalculateCutTarget(orderID, sizeVariant, orderQty)
}

func EvaluateEndBitDisposition(rollID, orderID string, cutNo, layStart, layEnd int, endBitM float64) (*LayRollRecord, error) {
	return cutting.EvaluateEndBitDisposition(rollID, orderID, cutNo, layStart, layEnd, endBitM)
}