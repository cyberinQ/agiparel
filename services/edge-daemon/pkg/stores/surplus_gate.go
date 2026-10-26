// Author: Sunil Jain, 2026-10-26 — Compute strict, evidence-based surplus eligibility before purchase indent approval.
package stores

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// This planner NEVER issues a purchase order or reserves physical stock.
// Its result must be revalidated and atomically reserved by the authoritative
// inventory transaction before any indent is approved.
type SurplusPolicy struct {
	MaxStockAgeDays int `json:"max_stock_age_days"`
	// Expressed as integer parts-per-million; 30000 means 3%. Zero = exact.
	GSMTolerancePPM    int                 `json:"gsm_tolerance_ppm"`
	ApprovedShadeBands map[string][]string `json:"approved_shade_bands"`
}

type BOMMaterialNeed struct {
	TenantID              string `json:"tenant_id"`
	BuyerID               string `json:"buyer_id"`
	OrderID               string `json:"order_id"`
	FactoryItemID         string `json:"factory_item_id"`
	Color                 string `json:"color"`
	Construction          string `json:"construction"`
	GSM                   int    `json:"gsm"`
	RequiredUsableWidthMM int    `json:"required_usable_width_mm"`
	ShadeBand             string `json:"shade_band"`
	RequiredLengthMM      int64  `json:"required_length_mm"`
}

type AvailableRollSegment struct {
	SegmentID             string    `json:"segment_id"`
	TenantID              string    `json:"tenant_id"`
	FactoryItemID         string    `json:"factory_item_id"`
	Color                 string    `json:"color"`
	Construction          string    `json:"construction"`
	GSM                   int       `json:"gsm"`
	UsableWidthMM         int       `json:"usable_width_mm"`
	ShadeBand             string    `json:"shade_band"`
	StockReceivedAt       time.Time `json:"stock_received_at"`
	QualityStatus         string    `json:"quality_status"` // PASS, HOLD, REJECTED
	OwnerType             string    `json:"owner_type"`     // FACTORY or BUYER
	OwnerBuyerID          string    `json:"owner_buyer_id,omitempty"`
	ApprovedBuyerIDs      []string  `json:"approved_buyer_ids"` // explicit buyer approvals for factory-owned material
	ReservationOrderID    string    `json:"reservation_order_id,omitempty"`
	AvailableUnreservedMM int64     `json:"available_unreserved_mm"`
	SourceEvidenceID      string    `json:"source_evidence_id"`
}

type SurplusAllocation struct {
	SegmentID  string `json:"segment_id"`
	AllocateMM int64  `json:"allocate_mm"`
	EvidenceID string `json:"evidence_id"`
}
type SurplusReject struct {
	SegmentID string   `json:"segment_id"`
	Reasons   []string `json:"reasons"`
}
type SurplusPlan struct {
	OrderID     string              `json:"order_id"`
	RequiredMM  int64               `json:"required_mm"`
	EligibleMM  int64               `json:"eligible_mm"`
	NetIndentMM int64               `json:"net_indent_mm"`
	Allocations []SurplusAllocation `json:"allocations"`
	Rejected    []SurplusReject     `json:"rejected"`
	// No authoritative indent may be created from this uncommitted snapshot.
	AdvisoryOnly bool `json:"advisory_only"`
}

func has(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// EvaluateSurplusGate preserves all six dimensions and emits explicit reasons.
// Unknown owner, age, quality, or source are blockers, not implicit passes.
func EvaluateSurplusGate(now time.Time, policy SurplusPolicy, need BOMMaterialNeed, candidates []AvailableRollSegment) (SurplusPlan, error) {
	plan := SurplusPlan{OrderID: need.OrderID, RequiredMM: need.RequiredLengthMM, NetIndentMM: need.RequiredLengthMM, AdvisoryOnly: true,
		Allocations: []SurplusAllocation{}, Rejected: []SurplusReject{}}
	if strings.TrimSpace(need.TenantID) == "" || strings.TrimSpace(need.OrderID) == "" || strings.TrimSpace(need.BuyerID) == "" ||
		strings.TrimSpace(need.FactoryItemID) == "" || strings.TrimSpace(need.Color) == "" || strings.TrimSpace(need.Construction) == "" ||
		strings.TrimSpace(need.ShadeBand) == "" || need.GSM <= 0 || need.RequiredUsableWidthMM <= 0 || need.RequiredLengthMM <= 0 ||
		policy.MaxStockAgeDays <= 0 || policy.GSMTolerancePPM < 0 || policy.GSMTolerancePPM > 1000000 || now.IsZero() {
		return SurplusPlan{}, errors.New("ERR_PRE_PO_INPUT: Missing material, buyer, quantity or approved eligibility policy")
	}
	// Do not reorder the caller's stock snapshot.
	candidates = append([]AvailableRollSegment(nil), candidates...)
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].SegmentID < candidates[j].SegmentID })
	seen := make(map[string]bool)
	for _, stock := range candidates {
		if stock.SegmentID == "" || seen[stock.SegmentID] {
			return SurplusPlan{}, fmt.Errorf("ERR_STOCK_IDENTITY: Blank or duplicate segment %q", stock.SegmentID)
		}
		seen[stock.SegmentID] = true
		var reasons []string
		if stock.TenantID != need.TenantID {
			reasons = append(reasons, "TENANT_MISMATCH")
		}
		if stock.FactoryItemID != need.FactoryItemID || stock.Color != need.Color || stock.Construction != need.Construction {
			reasons = append(reasons, "SPEC_MISMATCH")
		}
		if stock.GSM <= 0 || abs64(int64(stock.GSM-need.GSM))*1000000 > int64(need.GSM)*int64(policy.GSMTolerancePPM) {
			reasons = append(reasons, "GSM_MISMATCH")
		}
		if stock.UsableWidthMM < need.RequiredUsableWidthMM {
			reasons = append(reasons, "WIDTH_TOO_NARROW")
		}
		if stock.ShadeBand != need.ShadeBand && !has(policy.ApprovedShadeBands[need.ShadeBand], stock.ShadeBand) {
			reasons = append(reasons, "SHADE_NOT_APPROVED")
		}
		if stock.QualityStatus != "PASS" {
			reasons = append(reasons, "QUALITY_NOT_PASS")
		}
		if stock.StockReceivedAt.IsZero() || stock.StockReceivedAt.After(now) || now.Sub(stock.StockReceivedAt) > time.Duration(policy.MaxStockAgeDays)*24*time.Hour {
			reasons = append(reasons, "STOCK_AGE_UNVERIFIED")
		}
		if stock.OwnerType == "BUYER" && stock.OwnerBuyerID != need.BuyerID {
			reasons = append(reasons, "BUYER_OWNERSHIP_BLOCK")
		} else if stock.OwnerType == "FACTORY" && !has(stock.ApprovedBuyerIDs, need.BuyerID) {
			reasons = append(reasons, "BUYER_APPROVAL_MISSING")
		} else if stock.OwnerType != "BUYER" && stock.OwnerType != "FACTORY" {
			reasons = append(reasons, "OWNERSHIP_UNKNOWN")
		}
		if stock.ReservationOrderID != "" {
			reasons = append(reasons, "ALREADY_RESERVED")
		}
		if stock.AvailableUnreservedMM <= 0 {
			reasons = append(reasons, "NO_AVAILABLE_STOCK")
		}
		if strings.TrimSpace(stock.SourceEvidenceID) == "" {
			reasons = append(reasons, "EVIDENCE_MISSING")
		}
		if len(reasons) > 0 {
			plan.Rejected = append(plan.Rejected, SurplusReject{SegmentID: stock.SegmentID, Reasons: reasons})
			continue
		}
		remain := need.RequiredLengthMM - plan.EligibleMM
		if remain <= 0 {
			break
		}
		qty := stock.AvailableUnreservedMM
		if qty > remain {
			qty = remain
		}
		plan.Allocations = append(plan.Allocations, SurplusAllocation{SegmentID: stock.SegmentID, AllocateMM: qty, EvidenceID: stock.SourceEvidenceID})
		plan.EligibleMM += qty
	}
	plan.NetIndentMM = need.RequiredLengthMM - plan.EligibleMM
	return plan, nil
}
