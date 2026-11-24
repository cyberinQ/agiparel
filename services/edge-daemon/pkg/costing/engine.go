// Author: Sunil Jain, CyberinQ LLC (Generated 2026-11-24)
// Deterministic Costing with Planning Preview engine (Costing PRD v0.5, FR-1..FR-15, D1..D15, V1..V14).
// CPM = Total Factory Expense / Total Paid Work Minutes (D3/D3a locked).
// CM = Total OB SAM * CPM (no GSD or efficiency multipliers).
// Zero Amber/Red override: any non-Green check requires a joint re-plan record (D5).
package costing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Verdict string

const (
	VerdictGreen Verdict = "GREEN"
	VerdictAmber Verdict = "AMBER"
	VerdictRed   Verdict = "RED"
)

var validUOMs = map[string]bool{
	"YD": true, "M": true, "PCS": true, "CONE": true, "KG": true, "SET": true,
}

var numericTextRE = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`)

type OrderMatrixLine struct {
	Colour string `json:"colour"`
	Size   string `json:"size"`
	UPC    string `json:"upc"`
	Qty    int    `json:"qty"`
}

type OrderMatrixInput struct {
	BuyerID                   string            `json:"buyer_id"`
	StyleID                   string            `json:"style_id"`
	PORef                     string            `json:"po_ref"`
	Currency                  string            `json:"currency"`
	OrderDate                 time.Time         `json:"order_date"`
	RequestedShipDate         time.Time         `json:"requested_ship_date"`
	TentativeConfirmationDate time.Time         `json:"tentative_confirmation_date"`
	QuoteValidUntil           time.Time         `json:"quote_valid_until"`
	StatedGrandTotal          *int              `json:"stated_grand_total,omitempty"`
	StatedColourTotals        map[string]int    `json:"stated_colour_totals,omitempty"`
	Lines                     []OrderMatrixLine `json:"lines"`
	ExistingBuyerOpenUPCs     map[string]string `json:"existing_buyer_open_upcs,omitempty"` // upc -> po_ref
}

type OrderMatrixSummary struct {
	TotalQty     int            `json:"total_qty"`
	ColourTotals map[string]int `json:"colour_totals"`
}

// ValidateOrderMatrix enforces V1, V2, V3, V5 at save/import time.
func ValidateOrderMatrix(in OrderMatrixInput, now time.Time) (OrderMatrixSummary, error) {
	var out OrderMatrixSummary
	if strings.TrimSpace(in.BuyerID) == "" || strings.TrimSpace(in.StyleID) == "" || strings.TrimSpace(in.PORef) == "" {
		return out, errors.New("ERR_V1_MISSING_HEADER_KEYS")
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	ord := time.Date(in.OrderDate.Year(), in.OrderDate.Month(), in.OrderDate.Day(), 0, 0, 0, 0, time.UTC)
	ship := time.Date(in.RequestedShipDate.Year(), in.RequestedShipDate.Month(), in.RequestedShipDate.Day(), 0, 0, 0, 0, time.UTC)
	if ord.After(today) {
		return out, errors.New("ERR_V5_ORDER_DATE_IN_FUTURE")
	}
	if !ship.After(ord) {
		return out, errors.New("ERR_V5_SHIP_DATE_MUST_FOLLOW_ORDER_DATE")
	}
	if ship.After(ord.AddDate(1, 6, 0)) {
		return out, errors.New("ERR_V5_SHIP_DATE_EXCEEDS_18_MONTHS")
	}
	if len(in.Lines) == 0 {
		return out, errors.New("ERR_V2_EMPTY_ORDER_MATRIX")
	}

	out.ColourTotals = make(map[string]int)
	seenCS := make(map[string]bool)
	seenUPC := make(map[string]bool)

	for _, l := range in.Lines {
		c := strings.ToUpper(strings.TrimSpace(l.Colour))
		s := strings.ToUpper(strings.TrimSpace(l.Size))
		u := strings.TrimSpace(l.UPC)
		if c == "" || s == "" {
			return out, errors.New("ERR_V2_BLANK_COLOUR_OR_SIZE")
		}
		if l.Qty <= 0 {
			return out, errors.New("ERR_V2_NON_POSITIVE_LINE_QTY")
		}
		if u == "" {
			return out, fmt.Errorf("ERR_V3_MISSING_UPC_FOR_%s_%s", c, s)
		}
		csKey := c + ":" + s
		if seenCS[csKey] {
			return out, fmt.Errorf("ERR_V2_DUPLICATE_COLOUR_SIZE_%s_%s", c, s)
		}
		seenCS[csKey] = true

		if seenUPC[u] {
			return out, fmt.Errorf("ERR_V3_DUPLICATE_UPC_IN_PO_%s", u)
		}
		seenUPC[u] = true

		if otherPO, conflict := in.ExistingBuyerOpenUPCs[u]; conflict && otherPO != in.PORef {
			return out, fmt.Errorf("ERR_V3_UPC_%s_ALREADY_USED_ON_OPEN_PO_%s", u, otherPO)
		}

		out.ColourTotals[c] += l.Qty
		out.TotalQty += l.Qty
	}

	// V2: Reject imports whose stated colour subtotals or grand total disagree with summed rows.
	for c, stated := range in.StatedColourTotals {
		normC := strings.ToUpper(strings.TrimSpace(c))
		actual := out.ColourTotals[normC]
		if stated != actual {
			return out, fmt.Errorf("ERR_V2_COLOUR_TOTAL_MISMATCH_%s: stated=%d actual=%d", normC, stated, actual)
		}
	}
	if in.StatedGrandTotal != nil && *in.StatedGrandTotal != out.TotalQty {
		return out, fmt.Errorf("ERR_V2_GRAND_TOTAL_MISMATCH: stated=%d actual=%d", *in.StatedGrandTotal, out.TotalQty)
	}
	return out, nil
}

type BOMLineInput struct {
	LineNo               int       `json:"line_no"`
	ItemID               string    `json:"item_id"`
	Category             string    `json:"category"`
	Colour               string    `json:"colour,omitempty"`
	SupplierID           string    `json:"supplier_id"`
	RawConsumption       string    `json:"raw_consumption"` // validated against V4 (no text like "1.5 yd /pc")
	UOM                  string    `json:"uom"`
	PurchaseUnitMultiple float64   `json:"purchase_unit_multiple"`
	WastagePct           float64   `json:"wastage_pct"`
	UnitPrice            float64   `json:"unit_price"`
	PriceCurrency        string    `json:"price_currency"`
	AvailUnreservedStock float64   `json:"avail_unreserved_stock"`
	StockSnapshotAt      time.Time `json:"stock_snapshot_at"`
	LeadTimeDays         int       `json:"lead_time_days"`
	OpenCostingsCount    int       `json:"open_costings_count"` // C2 transparency
}

type BOMLineComputed struct {
	BOMLineInput
	Consumption       float64 `json:"consumption"`
	GrossQty          float64 `json:"gross_qty"`
	NetQty            float64 `json:"net_qty"`
	LineCostPerPc     float64 `json:"line_cost_per_pc"`
	TotalMaterialCost float64 `json:"total_material_cost"`
}

// ComputeBOMLine implements §8.1 deterministic BOM gross, net (Available-Unreserved only, rounded up
// to supplier purchase unit), and gross-based material cost per piece.
func ComputeBOMLine(in BOMLineInput, orderQty int) (BOMLineComputed, error) {
	var out BOMLineComputed
	if orderQty <= 0 {
		return out, errors.New("ERR_BOM_INVALID_ORDER_QTY")
	}
	raw := strings.TrimSpace(in.RawConsumption)
	if !numericTextRE.MatchString(raw) {
		return out, fmt.Errorf("ERR_V4_NON_NUMERIC_CONSUMPTION_LINE_%d: %q", in.LineNo, in.RawConsumption)
	}
	var cons float64
	if _, err := fmt.Sscanf(raw, "%f", &cons); err != nil || cons <= 0 {
		return out, fmt.Errorf("ERR_V4_INVALID_CONSUMPTION_LINE_%d", in.LineNo)
	}
	uom := strings.ToUpper(strings.TrimSpace(in.UOM))
	if !validUOMs[uom] {
		return out, fmt.Errorf("ERR_V4_UNKNOWN_UOM_LINE_%d: %q", in.LineNo, in.UOM)
	}
	if in.WastagePct < 0 || in.WastagePct > 100 || in.UnitPrice < 0 {
		return out, fmt.Errorf("ERR_V4_OUT_OF_RANGE_WASTAGE_OR_PRICE_LINE_%d", in.LineNo)
	}
	if in.AvailUnreservedStock < 0 {
		return out, fmt.Errorf("ERR_V6_NEGATIVE_AVAILABLE_STOCK_LINE_%d", in.LineNo)
	}
	if in.StockSnapshotAt.IsZero() {
		return out, fmt.Errorf("ERR_V6_MISSING_STOCK_SNAPSHOT_TIMESTAMP_LINE_%d", in.LineNo)
	}

	mult := in.PurchaseUnitMultiple
	if mult <= 0 {
		mult = 1.0
	}
	factor := 1.0 + (in.WastagePct / 100.0)
	gross := cons * float64(orderQty) * factor
	rawNet := math.Max(0, gross-in.AvailUnreservedStock)
	// Round net requirement UP to supplier's purchase unit (§8.1)
	net := math.Ceil((rawNet-1e-9)/mult) * mult
	if net < 0 {
		net = 0
	}
	costPerPc := cons * factor * in.UnitPrice

	out.BOMLineInput = in
	out.UOM = uom
	out.Consumption = cons
	out.GrossQty = round4(gross)
	out.NetQty = round4(net)
	out.LineCostPerPc = round6(costPerPc)
	out.TotalMaterialCost = round4(costPerPc * float64(orderQty))
	return out, nil
}

// ComputeCPM implements locked D3/D3a:
// CPM = Total Factory Expense / Total Paid Work Minutes (never produced minutes).
func ComputeCPM(totalFactoryExpense, totalPaidWorkMinutes float64) (float64, error) {
	if totalFactoryExpense < 0 {
		return 0, errors.New("ERR_D3_NEGATIVE_FACTORY_EXPENSE")
	}
	if totalPaidWorkMinutes <= 0 {
		return 0, errors.New("ERR_D3A_PAID_WORK_MINUTES_MUST_BE_POSITIVE")
	}
	return round6(totalFactoryExpense / totalPaidWorkMinutes), nil
}

type OBOperation struct {
	OperationID   string  `json:"operation_id"`
	Sequence      int     `json:"sequence"`
	MachineTypeID string  `json:"machine_type_id"`
	SkillGrade    string  `json:"skill_grade"`
	SAM           float64 `json:"sam"`
	IsCritical    bool    `json:"is_critical"`
}

type CostLine struct {
	CostType    string  `json:"cost_type"`
	Basis       string  `json:"basis"` // RATE or MANUAL
	RateRef     string  `json:"rate_ref,omitempty"`
	AmountPerPc float64 `json:"amount_per_pc"`
	Note        string  `json:"note"`
}

type PriceSummary struct {
	TotalQty       int     `json:"total_qty"`
	MaterialPerPc  float64 `json:"material_per_pc"`
	CMPerPc        float64 `json:"cm_per_pc"`
	OtherPerPc     float64 `json:"other_per_pc"`
	OverheadPerPc  float64 `json:"overhead_per_pc"`
	MarginPct      float64 `json:"margin_pct"`
	FOBPerPc       float64 `json:"fob_per_pc"`
	OrderValue     float64 `json:"order_value"`
	BaseFOBPerPc   float64 `json:"base_fob_per_pc"`
	BaseOrderValue float64 `json:"base_order_value"`
	FXRate         float64 `json:"fx_rate"`
}

// BuildPriceSummary enforces FR-3, FR-4, V1, V8 (no GSD/efficiency factor; manual lines require note).
func BuildPriceSummary(
	orderQty int,
	bomLines []BOMLineComputed,
	obOps []OBOperation,
	cpm float64,
	costLines []CostLine,
	marginPct float64,
	fxRate float64,
) (PriceSummary, float64, error) {
	var out PriceSummary
	if orderQty <= 0 || len(obOps) == 0 || cpm <= 0 || fxRate <= 0 {
		return out, 0, errors.New("ERR_PRICE_MISSING_CORE_INPUTS")
	}
	if marginPct >= 100 {
		return out, 0, errors.New("ERR_PRICE_INVALID_MARGIN_PCT")
	}

	var matPc float64
	for _, b := range bomLines {
		matPc += b.LineCostPerPc
	}

	var totalSAM float64
	for _, op := range obOps {
		if op.SAM <= 0 {
			return out, 0, fmt.Errorf("ERR_FR3_INVALID_SAM_OP_%s", op.OperationID)
		}
		totalSAM += op.SAM
	}
	// FR-3 / §8.2: CM per piece = Total OB SAM * CPM (no efficiency multiplier)
	cmPc := totalSAM * cpm

	var otherPc, overheadPc float64
	for _, cl := range costLines {
		if strings.ToUpper(cl.Basis) == "MANUAL" && len(strings.TrimSpace(cl.Note)) < 5 {
			return out, 0, fmt.Errorf("ERR_V8_MANUAL_COST_LINE_NOTE_REQUIRED_%s", cl.CostType)
		}
		if strings.ToUpper(cl.CostType) == "OVERHEAD" {
			overheadPc += cl.AmountPerPc
		} else if strings.ToUpper(cl.CostType) != "MARGIN" {
			otherPc += cl.AmountPerPc
		}
	}

	baseCost := matPc + cmPc + otherPc + overheadPc
	fobPc := baseCost / (1.0 - (marginPct / 100.0))
	orderVal := fobPc * float64(orderQty)

	out = PriceSummary{
		TotalQty:       orderQty,
		MaterialPerPc:  round6(matPc),
		CMPerPc:        round6(cmPc),
		OtherPerPc:     round6(otherPc),
		OverheadPerPc:  round6(overheadPc),
		MarginPct:      round4(marginPct),
		FOBPerPc:       round6(fobPc),
		OrderValue:     round4(orderVal),
		BaseFOBPerPc:   round6(fobPc * fxRate),
		BaseOrderValue: round4(orderVal * fxRate),
		FXRate:         fxRate,
	}
	return out, round4(totalSAM), nil
}

// ComputeCutAllowance implements §8.3: Cut_{c,s} = q_{c,s} + min(maxExtra, max(minExtra, ceil(pct * q_{c,s}))).
// Default tenant config: 2%, min 1, max 15 (e.g. 400 -> 408, 750 -> 765, 850 -> 865).
func ComputeCutAllowance(orderQty int, allowancePct float64, minExtra, maxExtra int) int {
	if orderQty <= 0 {
		return 0
	}
	extra := int(math.Ceil(float64(orderQty) * allowancePct))
	if extra < minExtra {
		extra = minExtra
	}
	if maxExtra > 0 && extra > maxExtra {
		extra = maxExtra
	}
	return orderQty + extra
}

type PreviewInput struct {
	OrderDate                time.Time
	RequestedShipDate        time.Time
	TransitHandoverDays      int
	AuditLoadBufferDays      int
	FinishingLagDays         int
	MaterialBufferDays       int
	AmberSlackThresholdDays  int // D5 default: 7
	MinutesPerOperatorPerDay int // e.g. 480
	PeakTargetPcsPerDay      int
	MachineSparePct          float64
	OrderQty                 int
	BOMLines                 []BOMLineComputed
	OBOperations             []OBOperation
	OwnedMachinesByType      map[string]int // working machines not under breakdown
	BlockedMachinesByType    map[string]int // blocked by machine_no in active Assigned OBs
	PoolOperatorsByGrade     map[string]int // qualified operators in factory pool
	BlockedOperatorsByGrade  map[string]int // blocked by operator_id in active Assigned OBs
	DailyRampPcs             []int          // D1: agreed ramp e.g. [600, 1200, 2000, ...]
	DailyAvailableMinutes    int
	DailyCommittedMinutes    int
	DailyHeldMinutes         int
}

type PreviewResult struct {
	InputSnapshotHash         string    `json:"input_snapshot_hash"`
	Verdict                   Verdict   `json:"verdict"`
	Reasons                   []string  `json:"reasons"`
	LatestPlanCutDate         time.Time `json:"latest_plan_cut_date"`
	SewingStartDate           time.Time `json:"sewing_start_date"`
	SewingEndDate             time.Time `json:"sewing_end_date"`
	FinishingEndDate          time.Time `json:"finishing_end_date"`
	EarliestFeasibleExFactory time.Time `json:"earliest_feasible_ex_factory"`
	SlackDays                 int       `json:"slack_days"`
	BindingMaterialSKU        string    `json:"binding_material_sku"`
}

// RunPlanningPreview implements FR-5, FR-6, FR-7, FR-8 and §8.4-§8.6 deterministically.
func RunPlanningPreview(in PreviewInput) (PreviewResult, error) {
	var res PreviewResult
	if in.OrderQty <= 0 || in.MinutesPerOperatorPerDay <= 0 || in.PeakTargetPcsPerDay <= 0 {
		return res, errors.New("ERR_PREVIEW_INVALID_PARAMETERS")
	}
	snapBytes, _ := json.Marshal(in)
	sum := sha256.Sum256(snapBytes)
	res.InputSnapshotHash = hex.EncodeToString(sum[:])

	worst := VerdictGreen
	escalate := func(v Verdict, reason string) {
		if reason != "" {
			res.Reasons = append(res.Reasons, reason)
		}
		if v == VerdictRed {
			worst = VerdictRed
		} else if v == VerdictAmber && worst == VerdictGreen {
			worst = VerdictAmber
		}
	}

	// §8.6 Backward schedule from requested ship date
	exFactoryReq := in.RequestedShipDate.AddDate(0, 0, -in.TransitHandoverDays)
	finEnd := exFactoryReq.AddDate(0, 0, -in.AuditLoadBufferDays)
	sewEnd := finEnd.AddDate(0, 0, -in.FinishingLagDays)

	// Calculate sewing days from D1 agreed ramp
	sewDays := 0
	rem := in.OrderQty
	for i := 0; rem > 0; i++ {
		dayOut := in.PeakTargetPcsPerDay
		if i < len(in.DailyRampPcs) && in.DailyRampPcs[i] > 0 {
			dayOut = in.DailyRampPcs[i]
		} else if len(in.DailyRampPcs) > 0 {
			dayOut = in.DailyRampPcs[len(in.DailyRampPcs)-1]
		}
		rem -= dayOut
		sewDays++
		if sewDays > 365 {
			break
		}
	}
	if sewDays < 1 {
		sewDays = 1
	}
	sewStart := sewEnd.AddDate(0, 0, -(sewDays - 1))
	latestCut := sewStart.AddDate(0, 0, -1)
	res.LatestPlanCutDate = latestCut
	res.SewingStartDate = sewStart
	res.SewingEndDate = sewEnd
	res.FinishingEndDate = finEnd

	// FR-5: Material feasibility preview
	neededBy := latestCut.AddDate(0, 0, -in.MaterialBufferDays)
	var latestInHouse time.Time
	for _, b := range in.BOMLines {
		if b.NetQty <= 0 {
			continue
		}
		inHouse := in.OrderDate.AddDate(0, 0, b.LeadTimeDays)
		if latestInHouse.IsZero() || inHouse.After(latestInHouse) {
			latestInHouse = inHouse
			res.BindingMaterialSKU = b.ItemID
		}
		slack := int(neededBy.Sub(inHouse).Hours() / 24)
		if inHouse.After(neededBy) {
			escalate(VerdictRed, fmt.Sprintf("FR-5 Material %s arrives %s after needed-by %s", b.ItemID, inHouse.Format("2006-01-02"), neededBy.Format("2006-01-02")))
		} else if slack < in.AmberSlackThresholdDays {
			escalate(VerdictAmber, fmt.Sprintf("FR-5 Material %s slack %d days < threshold %d", b.ItemID, slack, in.AmberSlackThresholdDays))
		}
	}

	// FR-6 & FR-7 (§8.4): Machine & Skill requirements vs unblocked pool
	machReq := make(map[string]int)
	skillReq := make(map[string]int)
	skillCritical := make(map[string]bool)
	var totalSAM float64

	for _, op := range in.OBOperations {
		totalSAM += op.SAM
		n := int(math.Ceil((op.SAM * float64(in.PeakTargetPcsPerDay)) / float64(in.MinutesPerOperatorPerDay)))
		if n < 1 {
			n = 1
		}
		machReq[op.MachineTypeID] += n
		skillReq[op.SkillGrade] += n
		if op.IsCritical {
			skillCritical[op.SkillGrade] = true
		}
	}

	machTypes := make([]string, 0, len(machReq))
	for mt := range machReq {
		machTypes = append(machTypes, mt)
	}
	sort.Strings(machTypes)
	for _, mt := range machTypes {
		req := int(math.Ceil(float64(machReq[mt]) * (1.0 + in.MachineSparePct)))
		owned, ok := in.OwnedMachinesByType[mt]
		if !ok || owned == 0 {
			escalate(VerdictRed, fmt.Sprintf("FR-6 Unowned machine type %s (need %d)", mt, req))
			continue
		}
		avail := owned - in.BlockedMachinesByType[mt]
		if avail < req {
			escalate(VerdictAmber, fmt.Sprintf("FR-6 Machine shortage for %s: need %d, unblocked available %d", mt, req, avail))
		}
	}

	grades := make([]string, 0, len(skillReq))
	for g := range skillReq {
		grades = append(grades, g)
	}
	sort.Strings(grades)
	for _, g := range grades {
		req := skillReq[g]
		avail := in.PoolOperatorsByGrade[g] - in.BlockedOperatorsByGrade[g]
		if avail < req {
			if skillCritical[g] {
				escalate(VerdictRed, fmt.Sprintf("FR-7 Critical skill shortage %s: need %d, available %d", g, req, avail))
			} else {
				escalate(VerdictAmber, fmt.Sprintf("FR-7 Skill shortage %s: need %d, available %d", g, req, avail))
			}
		}
	}

	// FR-8: Capacity & earliest feasible ex-factory
	freeMinPerDay := in.DailyAvailableMinutes - in.DailyCommittedMinutes - in.DailyHeldMinutes
	peakMinPerDay := int(math.Ceil(float64(in.PeakTargetPcsPerDay) * totalSAM))
	if freeMinPerDay < peakMinPerDay {
		escalate(VerdictRed, fmt.Sprintf("FR-8 Line capacity deficit: need %d min/day, free %d min/day", peakMinPerDay, freeMinPerDay))
	}

	earliestCut := in.OrderDate.AddDate(0, 0, 1)
	if !latestInHouse.IsZero() && latestInHouse.AddDate(0, 0, in.MaterialBufferDays).After(earliestCut) {
		earliestCut = latestInHouse.AddDate(0, 0, in.MaterialBufferDays)
	}
	earliestSewStart := earliestCut.AddDate(0, 0, 1)
	earliestSewEnd := earliestSewStart.AddDate(0, 0, sewDays-1)
	earliestFinEnd := earliestSewEnd.AddDate(0, 0, in.FinishingLagDays)
	res.EarliestFeasibleExFactory = earliestFinEnd.AddDate(0, 0, in.AuditLoadBufferDays)
	res.SlackDays = int(exFactoryReq.Sub(res.EarliestFeasibleExFactory).Hours() / 24)

	if res.SlackDays < 0 {
		escalate(VerdictRed, fmt.Sprintf("FR-8 Ship date missed by %d days (earliest ex-factory %s vs requested %s)", -res.SlackDays, res.EarliestFeasibleExFactory.Format("2006-01-02"), exFactoryReq.Format("2006-01-02")))
	} else if res.SlackDays < in.AmberSlackThresholdDays {
		escalate(VerdictAmber, fmt.Sprintf("FR-8 Schedule slack %d days < %d day threshold", res.SlackDays, in.AmberSlackThresholdDays))
	}

	res.Verdict = worst
	return res, nil
}

// ValidateSubmitAndConfirmGate enforces V7 and D5:
// Submit/Confirm is blocked if preview is stale, OB is not RELEASED, or verdict is AMBER/RED without a joint ReplanRecord.
func ValidateSubmitAndConfirmGate(previewStale bool, obState string, verdict Verdict, hasJointReplan bool, replanReason string) error {
	if previewStale {
		return errors.New("ERR_V7_PREVIEW_IS_STALE_RECOMPUTE_FIRST")
	}
	if strings.ToUpper(strings.TrimSpace(obState)) != "RELEASED" {
		return errors.New("ERR_V7_OB_VERSION_MUST_BE_RELEASED")
	}
	if verdict == VerdictAmber || verdict == VerdictRed {
		if !hasJointReplan || len(strings.TrimSpace(replanReason)) < 20 {
			return errors.New("ERR_D5_NO_AMBER_RED_OVERRIDE_JOINT_REPLAN_REQUIRED")
		}
	}
	return nil
}

func round4(v float64) float64 { return math.Round(v*10000) / 10000 }
func round6(v float64) float64 { return math.Round(v*1000000) / 1000000 }
