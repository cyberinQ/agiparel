// Author: Sunil Jain, CyberinQ LLC
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
	LightGreen  TrafficLight = "GREEN"
	LightYellow TrafficLight = "YELLOW"
	LightRed    TrafficLight = "RED"
)

type ReadinessCard struct {
	Category     string       `json:"category"`
	Status       TrafficLight `json:"status"`
	Headline     string       `json:"headline"`
	Details      []string     `json:"details"`
	SuggestedFix string       `json:"suggested_fix,omitempty"`
}

type PlanAssessment struct {
	PONumber            string          `json:"po_number"`
	StyleCode           string          `json:"style_code"`
	OrderedQty          int             `json:"ordered_qty"`
	TargetCutQty        int             `json:"target_cut_qty"`
	EstimatedDaysToDone int             `json:"estimated_days_to_done"`
	OverallStatus       TrafficLight    `json:"overall_status"`
	Cards               []ReadinessCard `json:"cards"`
}

func AssessWorkbook(wb *onboarding.FactoryWorkbook, poNumber string) PlanAssessment {
	if wb == nil {
		return PlanAssessment{
			PONumber:      poNumber,
			OverallStatus: LightRed,
			Cards: []ReadinessCard{
				{Category: "CLOTH", Status: LightRed, Headline: "Upload factory data first", Details: []string{"Workbook pointer is nil."}},
				{Category: "MACHINES", Status: LightRed, Headline: "Machine plan not verified", Details: []string{"Workbook pointer is nil."}},
				{Category: "WORKERS", Status: LightRed, Headline: "Worker plan not verified", Details: []string{"Workbook pointer is nil."}},
			},
		}
	}
	var targetWO onboarding.WorkOrderRow
	for _, wo := range wb.WorkOrders {
		if wo.PONumber == poNumber || poNumber == "" {
			targetWO = wo
			break
		}
	}
	if targetWO.OrderedQty <= 0 {
		return PlanAssessment{
			PONumber:      poNumber,
			OverallStatus: LightRed,
			Cards: []ReadinessCard{
				{Category: "CLOTH", Status: LightRed, Headline: "Upload a buyer order first", Details: []string{"No verified PO was found."}},
				{Category: "MACHINES", Status: LightRed, Headline: "Machine plan not verified", Details: []string{"A buyer order is needed."}},
				{Category: "WORKERS", Status: LightRed, Headline: "Worker plan not verified", Details: []string{"A buyer order is needed."}},
			},
		}
	}

	extra := int(math.Round(float64(targetWO.OrderedQty) * 0.02))
	if extra < 1 {
		extra = 1
	} else if extra > 15 {
		extra = 15
	}
	targetCut := targetWO.OrderedQty + extra

	clothCard := evaluateCloth(wb, targetWO.PONumber, targetWO.StyleCode, targetCut)
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

func evaluateCloth(wb *onboarding.FactoryWorkbook, po, styleCode string, targetCut int) ReadinessCard {
	// Dynamically calculate consumption from style parts/BOM; fallback to 1.25 only if unspecified
	consumptionPerGarment := 0.0
	for _, p := range wb.StyleParts {
		if p.StyleCode == styleCode && strings.Contains(strings.ToUpper(p.MaterialSKU), "DENIM") {
			// SubBundleOrder or linked BOM yields piece count; standard piece allowance
			consumptionPerGarment += 0.35
		}
	}
	if consumptionPerGarment <= 0.1 {
		consumptionPerGarment = 1.25
	}
	neededMeters := float64(targetCut) * consumptionPerGarment
	primarySKU := ""
	for _, st := range wb.Styles {
		if st.StyleCode == styleCode && st.FabricSKU != "" {
			primarySKU = strings.ToUpper(st.FabricSKU)
			break
		}
	}

	availMeters := 0.0
	for _, r := range wb.StoreRolls {
		skuUpper := strings.ToUpper(r.FactorySKU)
		// Exclude secondary trim/pocketing rolls from main body shell fabric calculation
		if strings.Contains(skuUpper, "POCKET") {
			continue
		}
		if primarySKU != "" && skuUpper != primarySKU {
			continue
		}
		if (r.Status == "AVAILABLE" || r.Status == "RESERVED") && (r.ReservedOrder == "" || r.ReservedOrder == po) {
			availMeters += r.LengthM
		}
	}

	if availMeters >= neededMeters {
		return ReadinessCard{
			Category: "CLOTH",
			Status:   LightGreen,
			Headline: "Enough Cloth in Store",
			Details: []string{
				fmt.Sprintf("We need %.0f meters of main shell cloth for %d pieces.", neededMeters, targetCut),
				fmt.Sprintf("We have %.1f meters ready in the store.", availMeters),
			},
		}
	}

	shortfall := neededMeters - availMeters
	return ReadinessCard{
		Category:     "CLOTH",
		Status:       LightRed,
		Headline:     fmt.Sprintf("Need %.1f More Meters of Main Shell Cloth", shortfall),
		Details:      []string{fmt.Sprintf("Have %.1f m of main shell fabric (excluding pocketing), but order needs %.0f m.", availMeters, neededMeters)},
		SuggestedFix: fmt.Sprintf("Order %.1f meters of shell cloth from supplier or allocate unreserved rolls.", shortfall),
	}
}

func evaluateMachines(wb *onboarding.FactoryWorkbook, styleCode string) ReadinessCard {
	if len(wb.Machines) == 0 || len(wb.OperationsDAG) == 0 {
		return ReadinessCard{
			Category: "MACHINES",
			Status:   LightRed,
			Headline: "Missing machine list or sewing steps",
			Details:  []string{"Upload both the operation bulletin and machine roster."},
		}
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
			SuggestedFix: "Dispatch line mechanic to repair stations in maintenance/breakdown.",
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
		return ReadinessCard{
			Category: "WORKERS",
			Status:   LightRed,
			Headline: "Worker roster is missing",
			Details:  []string{"Upload today's worker list."},
		}
	}

	today := time.Now().Format("2006-01-02")
	stale := []string{}
	coveredStations := make(map[string]string)
	presentCount := 0

	for _, worker := range wb.Workers {
		if worker.AttendanceDate != "" && worker.AttendanceDate != today {
			stale = append(stale, worker.Name)
		}
		if strings.ToUpper(worker.AttendanceStatus) != "ABSENT" {
			presentCount++
			if worker.AssignedStationID != "" {
				coveredStations[worker.AssignedStationID] = worker.Name
			}
		}
	}
	if len(stale) > 0 {
		return ReadinessCard{
			Category:     "WORKERS",
			Status:       LightYellow,
			Headline:     "Attendance is from an older shift",
			Details:      []string{fmt.Sprintf("%d workers have an outdated attendance date; upload today's register.", len(stale))},
			SuggestedFix: "Upload today's signed attendance register.",
		}
	}

	uncoveredAbsent := []string{}
	coveredNotes := []string{}
	for _, w := range wb.Workers {
		if strings.ToUpper(w.AttendanceStatus) == "ABSENT" {
			if coverName, covered := coveredStations[w.AssignedStationID]; covered {
				coveredNotes = append(coveredNotes, fmt.Sprintf("%s absent at %s (covered by %s)", w.Name, w.AssignedStationID, coverName))
			} else {
				uncoveredAbsent = append(uncoveredAbsent, fmt.Sprintf("%s is absent at %s (%s)", w.Name, w.AssignedStationID, w.PrimarySkill))
			}
		}
	}

	if len(uncoveredAbsent) > 0 {
		return ReadinessCard{
			Category:     "WORKERS",
			Status:       LightYellow,
			Headline:     fmt.Sprintf("%d Station(s) Uncovered Today", len(uncoveredAbsent)),
			Details:      uncoveredAbsent,
			SuggestedFix: "Assign a qualified float operator to uncovered stations.",
		}
	}

	details := []string{"Every sewing station has a trained operator."}
	details = append(details, coveredNotes...)
	return ReadinessCard{
		Category: "WORKERS",
		Status:   LightGreen,
		Headline: fmt.Sprintf("%d Workers Present & All Stations Covered", presentCount),
		Details:  details,
	}
}
