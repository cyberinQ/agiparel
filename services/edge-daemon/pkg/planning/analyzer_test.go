// Author: Sunil Jain, 2026-10-25 — Prevent green-lighting planning when authoritative inputs are absent.
package planning

import (
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/onboarding"
	"testing"
)

func TestMissingPOStopsPlanning(t *testing.T) {
	a := AssessWorkbook(&onboarding.FactoryWorkbook{}, "MISSING")
	if a.OverallStatus != LightRed || a.OrderedQty != 0 {
		t.Fatalf("unexpected readiness: %+v", a)
	}
}
func TestMissingMachinesAndOperationsStopsPlanning(t *testing.T) {
	w := &onboarding.FactoryWorkbook{WorkOrders: []onboarding.WorkOrderRow{{PONumber: "PO-TEST", OrderedQty: 600}}}
	a := AssessWorkbook(w, "PO-TEST")
	if a.OverallStatus != LightRed || a.Cards[1].Status != LightRed || a.Cards[2].Status != LightRed {
		t.Fatalf("missing inputs not stopped: %+v", a)
	}
}
