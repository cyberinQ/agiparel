// Author: Sunil Jain, 2026-10-25
// Check that unverified brownfield inputs remain manager questions.
package planning

import (
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/onboarding"
	"testing"
)

func TestReadinessInterviewRequiresEvidence(t *testing.T) {
	x := BuildReadinessInterview(nil, "PO1")
	if x.ReadyForApproval || len(x.Questions) != 1 {
		t.Fatalf("nil workbook should remain draft: %+v", x)
	}
	wb := &onboarding.FactoryWorkbook{WorkOrders: []onboarding.WorkOrderRow{{PONumber: "PO1", OrderedQty: 100, StyleCode: "STYLE1"}}}
	x = BuildReadinessInterview(wb, "PO1")
	if x.ReadyForApproval || len(x.Questions) < 6 {
		t.Fatalf("incomplete facts must not be considered approved: %+v", x)
	}
}
