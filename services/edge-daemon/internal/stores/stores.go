package stores

import (
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/stores"
)

type RollStock = stores.RollStock
type IssuanceRecord = stores.IssuanceRecord
type StoresEngine = stores.StoresEngine

func NewStoresEngine() *StoresEngine {
	return stores.NewStoresEngine()
}

func CalculateNetIndent(sku string, bomQty, unreservedQty float64) stores.IndentResult {
	res, _ := stores.EvaluateIndent(stores.IndentRequest{
		SKU:            sku,
		BOMRequiredQty: bomQty,
		UnreservedQty:  unreservedQty,
	})
	return stores.IndentResult{
		SKU:              res.SKU,
		BOMRequiredQty:   res.BOMRequiredQty,
		UnreservedQty:    res.UnreservedQty,
		NetIndentQty:     res.NetIndentQty,
		RequiresOverride: res.RequiresOverride,
	}
}
