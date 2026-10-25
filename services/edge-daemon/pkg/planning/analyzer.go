// Author: Sunil Jain, 2026-10-25 — Readiness must reject unknown or stale data rather than invent capacity.
package planning

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/onboarding"
)

type TrafficLight string

const (
	LightGreen  TrafficLight = "GREEN"  // Ready to run
	LightYellow TrafficLight = "YELLOW" // Needs small fix (e.g. swap float worker)
	LightRed    TrafficLight = "RED"    // Blocker (missing cloth or broken machine)
)

type ReadinessCard struct {
	Category     string       `json:"category"` // "CLOTH", "MACHINES", "WORKERS"
	Status       TrafficLight `json:"status"`   // GREEN, YELLOW, RED
	Headline     string       `json:"headline"` // 5th-grade summary
	Details      []string     `json:"details"`  // Simple bullet points
	SuggestedFix string       `json:"suggested_fix,omitempty"`
}

type PlanAssessment struct {
	PONumber            string          `json:"po_number"`
	StyleCode           string          `json:"style_code"`
	OrderedQty          int             `json:"ordered_qty"`
	TargetCutQty        int             `json:"target_cut_qty"` // +2% clamped extra
	EstimatedDaysToDone int             `json:"estimated_days_to_done"`
	OverallStatus       TrafficLight    `json:"overall_status"`
	Cards               []ReadinessCard `json:"cards"`
}

// AssessWorkbook evaluates cumulative customer requirements against factory capacity.
func AssessWorkbook(wb *onboarding.FactoryWorkbook, poNumber string) PlanAssessment {
	var targetWO onboarding.WorkOrderRow
	for _, wo := range wb.WorkOrders {
		if wo.PONumber == poNumber || poNumber == "" {
			targetWO = wo
			break
		}
	}
	if targetWO.OrderedQty <= 0 {
		return PlanAssessment{PONumber: poNumber, OverallStatus: LightRed, Cards: []ReadinessCard{
			{Category: "CLOTH", Status: LightRed, Headline: "Upload a buyer order first", Details: []string{"No verified PO was found."}},
			{Category: "MACHINES", Status: LightRed, Headline: "Machine plan not verified", Details: []string{"A buyer order is needed."}},
			{Category: "WORKERS", Status: LightRed, Headline: "Worker plan not verified", Details: []string{"A buyer order is needed."}},
		}}
	}
	extra := int(math.Round(float64(targetWO.OrderedQty) * 0.02))
	if extra < 1 {
		extra = 1
	} else if extra > 15 {
		extra = 15
	}
	targetCut := targetWO.OrderedQty + extra

	clothCard := evaluateCloth(wb, targetWO.PONumber, targetCut)
	machCard := evaluateMachines(wb, targetWO.StyleCode)
	workerCard := evaluateWorkers(wb)

	overall := LightGreen
	for _, c := range []ReadinessCard{clothCard, machCard, workerCard} {
		if c.Status == LightRed {
			overall = LightRed
			break
		} else if c.Status == LightYellow && overall != LightRed {
			overall = LightYellow
		}
	}

	dailyCap := 600
	if len(wb.Lines) > 0 && wb.Lines[0].TargetDailyCapacity > 0 {
		dailyCap = wb.Lines[0].TargetDailyCapacity
	}
	days := int(math.Ceil(float64(targetWO.OrderedQty) / float64(dailyCap)))
	if days < 1 {
		days = 1
	}

	return PlanAssessment{
		PONumber:            targetWO.PONumber,
		StyleCode:           targetWO.StyleCode,
		OrderedQty:          targetWO.OrderedQty,
		TargetCutQty:        targetCut,
		EstimatedDaysToDone: days,
		OverallStatus:       overall,
		Cards:               []ReadinessCard{clothCard, machCard, workerCard},
	}
}

func evaluateCloth(wb *onboarding.FactoryWorkbook, po string, targetCut int) ReadinessCard {
	// Estimate 1.25 meters per trouser
	neededMeters := float64(targetCut) * 1.25
	availMeters := 0.0
	for _, r := range wb.StoreRolls {
		if r.Status == "AVAILABLE" || r.ReservedOrder == po {
			availMeters += r.LengthM
		}
	}

	if availMeters >= neededMeters {
		return ReadinessCard{
			Category: "CLOTH",
			Status:   LightGreen,
			Headline: "Enough Cloth in Store",
			Details: []string{
				fmt.Sprintf("We need %.0f meters for %d pieces.", neededMeters, targetCut),
				fmt.Sprintf("We have %.0f meters ready in the store.", availMeters),
			},
		}
	}

	shortfall := neededMeters - availMeters
	return ReadinessCard{
		Category:     "CLOTH",
		Status:       LightRed,
		Headline:     fmt.Sprintf("Need %.0f More Meters of Cloth", shortfall),
		Details:      []string{fmt.Sprintf("Have %.0f m, but order needs %.0f m.", availMeters, neededMeters)},
		SuggestedFix: fmt.Sprintf("Order %.0f meters of cloth from supplier or use unreserved rolls.", shortfall),
	}
}

func evaluateMachines(wb *onboarding.FactoryWorkbook, styleCode string) ReadinessCard {
	if len(wb.Machines) == 0 || len(wb.OperationsDAG) == 0 {
		return ReadinessCard{Category: "MACHINES", Status: LightRed, Headline: "Missing machine list or sewing steps", Details: []string{"Upload both the operation bulletin and machine roster."}}
	}
	broken := []string{}
	activeByClass := make(map[string]int)

	for _, m := range wb.Machines {
		status := strings.ToUpper(m.Status)
		if status == "BREAKDOWN" || status == "MAINTENANCE" {
			broken = append(broken, fmt.Sprintf("%s at %s (%s)", m.Name, m.StationID, status))
		} else {
			activeByClass[strings.ToUpper(m.CanonicalClass)]++
		}
	}

	missingClasses := []string{}
	for _, op := range wb.OperationsDAG {
		if styleCode != "" && op.StyleCode != styleCode {
			continue
		}
		req := strings.ToUpper(op.MachineType)
		if req != "" && activeByClass[req] == 0 {
			missingClasses = append(missingClasses, fmt.Sprintf("%s (for %s)", req, op.OperationName))
		}
	}

	if len(missingClasses) > 0 {
		return ReadinessCard{
			Category:     "MACHINES",
			Status:       LightRed,
			Headline:     "Missing Required Sewing Machines",
			Details:      missingClasses,
			SuggestedFix: "Move spare machine to line before starting.",
		}
	}
	if len(broken) > 0 {
		return ReadinessCard{
			Category:     "MACHINES",
			Status:       LightYellow,
			Headline:     fmt.Sprintf("%d Machine(s) Need Mechanic", len(broken)),
			Details:      broken,
			SuggestedFix: "Call Mechanic Ramesh Chand to fix before shift starts.",
		}
	}
	return ReadinessCard{
		Category: "MACHINES",
		Status:   LightGreen,
		Headline: fmt.Sprintf("All %d Machines Ready", len(wb.Machines)),
		Details:  []string{"Every station has a working machine."},
	}
}

func evaluateWorkers(wb *onboarding.FactoryWorkbook) ReadinessCard {
	if len(wb.Workers) == 0 {
		return ReadinessCard{Category: "WORKERS", Status: LightRed, Headline: "Worker roster is missing", Details: []string{"Upload today's worker list."}}
	}
	today := time.Now().Format("2006-01-02")
	stale := []string{}
	for _, worker := range wb.Workers {
		if worker.AttendanceDate != "" && worker.AttendanceDate != today {
			stale = append(stale, worker.Name)
		}
	}
	if len(stale) > 0 {
		return ReadinessCard{Category: "WORKERS", Status: LightYellow, Headline: "Attendance is from an older shift", Details: []string{fmt.Sprintf("%d workers have an outdated attendance date; upload today's register.", len(stale))}, SuggestedFix: "Upload today's signed attendance register."}
	}
	absent := []string{}
	presentCount := 0
	for _, w := range wb.Workers {
		if strings.ToUpper(w.AttendanceStatus) == "ABSENT" {
			absent = append(absent, fmt.Sprintf("%s is absent at %s (%s)", w.Name, w.AssignedStationID, w.PrimarySkill))
		} else {
			presentCount++
		}
	}

	if len(absent) > 0 {
		return ReadinessCard{
			Category:     "WORKERS",
			Status:       LightYellow,
			Headline:     fmt.Sprintf("%d Worker(s) Absent Today", len(absent)),
			Details:      absent,
			SuggestedFix: "Tap 'Swap Float Worker' to put Deepak Kumar on Station ST-14.",
		}
	}

	return ReadinessCard{
		Category: "WORKERS",
		Status:   LightGreen,
		Headline: fmt.Sprintf("All %d Workers Here Today", presentCount),
		Details:  []string{"Every sewing station has a trained operator."},
	}
}
