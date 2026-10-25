// Author: Sunil Jain, 2026-10-25
// Read-only manager questions for brownfield order readiness. No plan is published.
package planning

import (
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/onboarding"
	"strings"
)

type ManagerQuestion struct {
	ID          string `json:"id"`
	Question    string `json:"question"`
	Why         string `json:"why"`
	RequiredFor string `json:"required_for"`
}

type ReadinessInterview struct {
	PONumber         string            `json:"po_number"`
	ReadyForApproval bool              `json:"ready_for_approval"`
	Questions        []ManagerQuestion `json:"questions"`
	Notes            []string          `json:"notes"`
}

// BuildReadinessInterview is additive to AssessWorkbook and does not alter
// legacy values, reserve inventory, approve documents or authorize cutting.
func BuildReadinessInterview(wb *onboarding.FactoryWorkbook, poNumber string) ReadinessInterview {
	out := ReadinessInterview{PONumber: poNumber, Questions: []ManagerQuestion{}, Notes: []string{"Draft only. Confirm all source facts before making a commitment."}}
	add := func(id, q, why, forWhat string) {
		out.Questions = append(out.Questions, ManagerQuestion{ID: id, Question: q, Why: why, RequiredFor: forWhat})
	}
	if wb == nil {
		add("FACTORY_DATA", "Please upload today's factory information.", "No inventory or capacity records were supplied.", "planning")
		return out
	}
	var wo *onboarding.WorkOrderRow
	for i := range wb.WorkOrders {
		if wb.WorkOrders[i].PONumber == poNumber {
			wo = &wb.WorkOrders[i]
			break
		}
	}
	if wo == nil || wo.OrderedQty <= 0 {
		add("BUYER_ORDER", "Please upload or confirm the buyer PO and order quantity.", "The buyer order is missing or incomplete.", "order")
		return out
	}
	out.PONumber = wo.PONumber
	if strings.TrimSpace(wo.StyleCode) == "" {
		add("STYLE", "Which approved style and tech pack apply to this order?", "The garment design is not linked.", "engineering")
	}
	styleFound := false
	fabric := ""
	for _, s := range wb.Styles {
		if s.StyleCode == wo.StyleCode {
			styleFound = true
			fabric = strings.TrimSpace(s.FabricSKU)
			break
		}
	}
	if !styleFound {
		add("TECH_PACK", "Can you confirm the approved tech pack and BOM?", "No matching approved style information is in this snapshot.", "engineering")
	}
	if fabric == "" {
		add("SHELL_FABRIC", "What is the shell fabric code for this style?", "Fabric matching would otherwise be a guess.", "material")
	}
	if len(wb.OperationsDAG) == 0 {
		add("OPERATIONS", "Please upload the current operation bulletin.", "The required sewing steps are unknown.", "engineering")
	}
	if len(wb.StoreRolls) == 0 {
		add("ROLL_STOCK", "Can you share today's fabric roll list, widths, shades and usable lengths?", "We cannot confirm cloth availability.", "cutting")
	}
	if len(wb.Machines) == 0 {
		add("MACHINES", "Which machines are available for this work?", "Machine capacity is not yet known.", "capacity")
	}
	if len(wb.Workers) == 0 {
		add("WORKERS", "Who is working today, and which jobs can they do?", "Available skills are unknown.", "capacity")
	}
	if len(wb.Lines) == 0 {
		add("LINES", "Which lines and shifts can take this order?", "A real completion date needs line capacity.", "schedule")
	}
	add("MARKER", "Please provide the approved Gerber marker options and fabric width for this style.", "The current workbook does not carry verified per-ply marker yield.", "cutting")
	add("CUT_APPROVAL", "Has the cutting master approved shade mixing, lay limits and any over-cut allowance?", "These cannot be assumed from a general estimate.", "cutting")
	// Explicitly not ready: source approvals and marker confirmation are outside
	// the workbook schema. A later approved-facts service may close these questions.
	return out
}
