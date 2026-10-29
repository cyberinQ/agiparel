// Author: Sunil Jain, CyberinQ LLC
package planning

import (
	"fmt"
	"math"
	"sort"
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
	}
	maxAllowedExtra := int(math.Max(15.0, float64(targetWO.OrderedQty)*0.03))
	if maxAllowedExtra > 500 {
		maxAllowedExtra = 500
	}
	if extra > maxAllowedExtra {
		extra = maxAllowedExtra
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
	if len(wb.Lines) > 0 {
		matchedLine := false
		for _, ln := range wb.Lines {
			if strings.EqualFold(ln.LineID, targetWO.LineID) && ln.TargetDailyCapacity > 0 {
				dailyCap = ln.TargetDailyCapacity
				matchedLine = true
				break
			}
		}
		if !matchedLine && wb.Lines[0].TargetDailyCapacity > 0 {
			dailyCap = wb.Lines[0].TargetDailyCapacity
		}
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

// evaluateCloth compares only explicit style-specific material quantities and
// workbook-reported stock. A workbook is not an approved marker or a live WMS;
// sufficiency is therefore provisional, never an authoritative GREEN release.
func evaluateCloth(wb *onboarding.FactoryWorkbook, po, styleCode string, targetCut int) ReadinessCard {
	expected := make(map[string]float64)
	uomBySKU := make(map[string]string)
	for _, b := range wb.BOMRows {
		if !strings.EqualFold(strings.TrimSpace(b.StyleCode), strings.TrimSpace(styleCode)) || !strings.EqualFold(b.Category, "FABRIC") {
			continue
		}
		sku := strings.ToUpper(strings.TrimSpace(b.MaterialSKU))
		unit := strings.ToUpper(strings.TrimSpace(b.UOM))
		if sku == "" || math.IsNaN(b.QtyPerGarment) || math.IsInf(b.QtyPerGarment, 0) || b.QtyPerGarment <= 0 ||
			(unit != "M" && unit != "METER" && unit != "METRE") {
			return ReadinessCard{Category: "CLOTH", Status: LightYellow, Headline: "UNKNOWN — fabric BOM needs review",
				Details:      []string{"Fabric BOM has missing, invalid or non-meter quantities. Do not infer meters from cut-part counts."},
				SuggestedFix: "Provide a validated fabric BOM or approved marker yield with explicit meter units."}
		}
		if old, ok := uomBySKU[sku]; ok && old != unit && (old != "M" && old != "METER" && old != "METRE") {
			return ReadinessCard{Category: "CLOTH", Status: LightYellow, Headline: "UNKNOWN — conflicting BOM units", SuggestedFix: "Resolve material units."}
		}
		uomBySKU[sku] = unit
		expected[sku] += b.QtyPerGarment * float64(targetCut)
	}
	// A partial fabric BOM must not silently omit the style's declared shell SKU.
	primary := ""
	for _, st := range wb.Styles {
		if strings.EqualFold(strings.TrimSpace(st.StyleCode), strings.TrimSpace(styleCode)) {
			primary = strings.ToUpper(strings.TrimSpace(st.FabricSKU))
			break
		}
	}
	if primary != "" && len(expected) > 0 {
		if _, ok := expected[primary]; !ok {
			return ReadinessCard{Category: "CLOTH", Status: LightYellow,
				Headline:     "UNKNOWN — shell fabric missing from BOM",
				Details:      []string{fmt.Sprintf("Style declares shell SKU %s, but the BOM omits its consumption.", primary)},
				SuggestedFix: "Reconcile the style shell SKU with an approved BOM/marker."}
		}
	}
	if len(expected) == 0 {
		return ReadinessCard{Category: "CLOTH", Status: LightYellow, Headline: "UNKNOWN — fabric requirement not measured",
			Details:      []string{"No explicit fabric BOM or approved marker yield for this style. Style part counts do not determine fabric length."},
			SuggestedFix: "Load the style's fabric BOM or approved marker consumption; do not procure from fixed estimates."}
	}
	// Stock lengths are reported by the workbook, not physically remeasured or reserved here.
	available := make(map[string]float64)
	for _, r := range wb.StoreRolls {
		sku := strings.ToUpper(strings.TrimSpace(r.FactorySKU))
		if _, wanted := expected[sku]; !wanted {
			continue
		}
		state := strings.ToUpper(strings.TrimSpace(r.Status))
		if state != "AVAILABLE" && state != "RESERVED" {
			continue
		}
		owner := strings.TrimSpace(r.ReservedOrder)
		if owner != "" && owner != po {
			continue
		}
		if state == "RESERVED" && owner == "" {
			continue
		} // ambiguous reservation
		if math.IsNaN(r.LengthM) || math.IsInf(r.LengthM, 0) || r.LengthM <= 0 {
			return ReadinessCard{Category: "CLOTH", Status: LightYellow, Headline: "UNKNOWN — invalid reported roll balance",
				SuggestedFix: "Reconcile the roll's remaining length and owner before planning."}
		}
		available[sku] += r.LengthM
	}
	keys := make([]string, 0, len(expected))
	for k := range expected {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	details := []string{"WORKBOOK-DECLARED ONLY: quantities are not approved marker yields or live verified stock."}
	deficit := false
	for _, sku := range keys {
		required, have := expected[sku], available[sku]
		details = append(details, fmt.Sprintf("%s: declared need %.3f m for %d pieces; reported eligible stock %.3f m.", sku, required, targetCut, have))
		if have+1e-9 < required {
			deficit = true
		}
	}
	if deficit {
		return ReadinessCard{Category: "CLOTH", Status: LightRed, Headline: "Reported fabric deficit — verify before ordering",
			Details: details, SuggestedFix: "Validate approved marker consumption and reconcile actual roll balances; only then reserve or procure."}
	}
	return ReadinessCard{Category: "CLOTH", Status: LightYellow, Headline: "Reported stock covers declared BOM — confirmation required",
		Details: details, SuggestedFix: "Approve marker/consumption and reconcile live roll reservations before granting cutting release."}
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
		} else if status == "ACTIVE" || status == "IDLE" {
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
