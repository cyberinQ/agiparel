// Author: Sunil Jain, 2026-11-11 — Sewing-driven scheduling, garment-family integrity and change-control gates.
package sop2

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

type SewingNeed struct {
	ID              string `json:"id"`
	IntakeID        string `json:"intake_id"`
	Style           string `json:"style"`
	SKU             string `json:"sku"`
	LineID          string `json:"line_id"`
	SewingStart     string `json:"sewing_start"` // YYYY-MM-DD (factory-local)
	DeliveryDate    string `json:"delivery_date"`
	LeadDays        int    `json:"lead_days"`
	NeedPieces      int64  `json:"need_pieces"`
	LineStock       int64  `json:"line_stock"`
	MinimumStock    int64  `json:"minimum_stock"`
	RemainingPO     int64  `json:"remaining_po"`
	MinutesPerPiece int64  `json:"minutes_per_piece"`
	Frozen          bool   `json:"frozen"`
}

type CapacityDay struct {
	Date             string `json:"date"`
	TableID          string `json:"table_id"`
	AvailableMinutes int64  `json:"available_minutes"`
}

type ScheduledCut struct {
	NeedID       string `json:"need_id"`
	IntakeID     string `json:"intake_id"`
	SKU          string `json:"sku"`
	LineID       string `json:"line_id"`
	Date         string `json:"date"`
	CutReadyDate string `json:"cut_ready_date"`
	TableID      string `json:"table_id"`
	Pieces       int64  `json:"pieces"`
	Minutes      int64  `json:"minutes"`
	Priority     string `json:"priority"`
}

type ScheduleResult struct {
	Proposed  []ScheduledCut   `json:"proposed"`
	Shortages map[string]int64 `json:"shortages"`
	Alerts    []string         `json:"alerts"`
	Frozen    bool             `json:"frozen"`
}

func parseDay(raw string) (time.Time, error) {
	t, e := time.Parse("2006-01-02", raw)
	if e != nil || t.Format("2006-01-02") != raw {
		return time.Time{}, errors.New("INVALID_FACTORY_DATE")
	}
	return t, nil
}

// SuggestDailyCut is a deterministic capacity-limited advisory schedule; it
// never modifies frozen sewing plans or fabric/stock ledgers.
func SuggestDailyCut(needs []SewingNeed, days []CapacityDay) (ScheduleResult, error) {
	out := ScheduleResult{Proposed: []ScheduledCut{}, Shortages: map[string]int64{}, Alerts: []string{}, Frozen: false}
	if len(needs) == 0 || len(needs) > 500 || len(days) == 0 || len(days) > 90 {
		return out, errors.New("SCHEDULE_SIZE_INVALID")
	}
	for i := range needs {
		n := &needs[i]
		if n.ID == "" || n.IntakeID == "" || n.LineID == "" || n.SKU == "" || !n.Frozen || n.NeedPieces < 0 || n.LineStock < 0 || n.MinimumStock < 0 || n.RemainingPO < 0 || n.MinutesPerPiece <= 0 || n.LeadDays < 0 || n.LeadDays > 365 {
			return out, errors.New("SEWING_PLAN_NOT_FROZEN_OR_INVALID")
		}
		if _, e := parseDay(n.SewingStart); e != nil {
			return out, e
		}
		if _, e := parseDay(n.DeliveryDate); e != nil {
			return out, e
		}
		if n.LineStock < n.MinimumStock {
			out.Alerts = append(out.Alerts, "LINE_STOCK_BELOW_MINIMUM:"+n.ID)
		}
	}
	sort.Slice(needs, func(i, j int) bool {
		a, b := needs[i], needs[j]
		// Priority: lines below their stock buffer, deadlines, largest balances.
		aShort, bShort := a.LineStock < a.MinimumStock, b.LineStock < b.MinimumStock
		if aShort != bShort {
			return aShort
		}
		aStart, _ := parseDay(a.SewingStart)
		bStart, _ := parseDay(b.SewingStart)
		aReady := aStart.AddDate(0, 0, -a.LeadDays)
		bReady := bStart.AddDate(0, 0, -b.LeadDays)
		if !aReady.Equal(bReady) {
			return aReady.Before(bReady)
		}
		if a.DeliveryDate != b.DeliveryDate {
			return a.DeliveryDate < b.DeliveryDate
		}
		if a.RemainingPO != b.RemainingPO {
			return a.RemainingPO > b.RemainingPO
		}
		return a.ID < b.ID
	})
	sort.Slice(days, func(i, j int) bool {
		if days[i].Date == days[j].Date {
			return days[i].TableID < days[j].TableID
		}
		return days[i].Date < days[j].Date
	})
	for _, d := range days {
		if d.TableID == "" || d.AvailableMinutes < 0 {
			return out, errors.New("CUTTING_CAPACITY_INVALID")
		}
		if _, e := parseDay(d.Date); e != nil {
			return out, e
		}
	}
	available := make([]int64, len(days))
	for i, d := range days {
		available[i] = d.AvailableMinutes
	}
	for _, n := range needs {
		desired := n.NeedPieces - n.LineStock + n.MinimumStock
		if desired < 0 {
			desired = 0
		}
		if desired > n.RemainingPO {
			desired = n.RemainingPO
		}
		start, _ := parseDay(n.SewingStart)
		ready := start.AddDate(0, 0, -n.LeadDays).Format("2006-01-02")
		for i, d := range days {
			if desired == 0 || d.Date > ready {
				continue
			}
			pieces := available[i] / n.MinutesPerPiece
			if pieces > desired {
				pieces = desired
			}
			if pieces <= 0 {
				continue
			}
			priority := "NORMAL"
			if n.LineStock < n.MinimumStock {
				priority = "LINE_AT_RISK"
			} else if d.Date == ready {
				priority = "DUE_TODAY"
			}
			out.Proposed = append(out.Proposed, ScheduledCut{NeedID: n.ID, IntakeID: n.IntakeID, SKU: n.SKU, LineID: n.LineID, Date: d.Date, CutReadyDate: ready, TableID: d.TableID, Pieces: pieces, Minutes: pieces * n.MinutesPerPiece, Priority: priority})
			available[i] -= pieces * n.MinutesPerPiece
			desired -= pieces
		}
		if desired > 0 {
			out.Shortages[n.ID] = desired
			out.Alerts = append(out.Alerts, fmt.Sprintf("FROZEN_SEWING_CUT_SHORTAGE:%s:%d", n.ID, desired))
		}
	}
	return out, nil
}

type SubBundle struct {
	Label            string `json:"label"`
	FamilyID         string `json:"family_id"`
	PartCode         string `json:"part_code"`
	PlyStart         int    `json:"ply_start"`
	PlyEnd           int    `json:"ply_end"`
	PanelStart       int64  `json:"panel_start"`
	PanelEnd         int64  `json:"panel_end"`
	PiecesPerGarment int64  `json:"pieces_per_garment"`
	NFCUID           string `json:"nfc_uid"`
	Bound            bool   `json:"bound"`
	// A single lay ply can have multiple parts, but all those parts must
	// reference exactly the same source roll for each garment/ply.
	SourceRollByPly map[int]string `json:"source_roll_by_ply"`
}

type MarriagePair struct {
	Left  string `json:"left"`
	Right string `json:"right"`
}

type FamilyAudit struct {
	Complete          bool     `json:"complete"`
	ValidMarriage     bool     `json:"valid_marriage"`
	Missing           []string `json:"missing"`
	Problems          []string `json:"problems"`
	Scanned           int      `json:"scanned"`
	Expected          int      `json:"expected"`
	AuthorizedRelease bool     `json:"authorized_release"`
}

// VerifyFamily supports paper+NFC data checks and complete sub-bundle presence.
// It deliberately cannot authorize release: only the existing signed EPC reader
// and two-person release transaction has that authority.
func VerifyFamily(expected []SubBundle, pairs []MarriagePair, scannedLabels []string) (FamilyAudit, error) {
	out := FamilyAudit{Missing: []string{}, Problems: []string{}, Expected: len(expected), AuthorizedRelease: false}
	if len(expected) == 0 || len(expected) > 640 {
		return out, errors.New("FAMILY_SIZE_INVALID")
	}
	family := expected[0].FamilyID
	labels := map[string]SubBundle{}
	seenTags := map[string]bool{}
	scans := map[string]bool{}
	plies := map[int]string{}
	for _, b := range expected {
		if b.FamilyID == "" || b.FamilyID != family || b.Label == "" || b.PartCode == "" || b.PlyStart < 1 || b.PlyEnd < b.PlyStart || b.PanelEnd < b.PanelStart || b.PanelEnd-b.PanelStart+1 != int64(b.PlyEnd-b.PlyStart+1) || b.PiecesPerGarment < 1 || labels[b.Label].Label != "" {
			return out, errors.New("SUB_BUNDLE_IDENTITY_INVALID")
		}
		if !b.Bound || b.NFCUID == "" || seenTags[b.NFCUID] {
			out.Problems = append(out.Problems, "NFC_TAG_NOT_UNIQUELY_BOUND:"+b.Label)
		}
		seenTags[b.NFCUID] = true
		for p := b.PlyStart; p <= b.PlyEnd; p++ {
			id := b.SourceRollByPly[p]
			if id == "" || (plies[p] != "" && plies[p] != id) {
				out.Problems = append(out.Problems, fmt.Sprintf("GARMENT_PLY_ROLL_MISMATCH:%s:%d", b.Label, p))
			}
			if id != "" {
				plies[p] = id
			}
		}
		labels[b.Label] = b
	}
	for _, scan := range scannedLabels {
		if labels[scan].Label == "" || scans[scan] {
			out.Problems = append(out.Problems, "UNKNOWN_OR_DUPLICATE_SCAN:"+scan)
			continue
		}
		scans[scan] = true
		out.Scanned++
	}
	for _, b := range expected {
		if !scans[b.Label] {
			out.Missing = append(out.Missing, b.Label)
		}
	}
	required := map[string]bool{}
	valid := true
	for _, pair := range pairs {
		if pair.Left == pair.Right || labels[pair.Left].Label == "" || labels[pair.Right].Label == "" {
			valid = false
			out.Problems = append(out.Problems, "MARRIAGE_PAIR_INVALID")
			continue
		}
		a, b := pair.Left, pair.Right
		if a > b {
			a, b = b, a
		}
		key := a + "|" + b
		if required[key] {
			valid = false
			out.Problems = append(out.Problems, "DUPLICATE_MARRIAGE_PAIR")
		}
		required[key] = true
	}
	// The expected marriage mapping must be an explicit connected graph:
	// every part of a family is a member of one traceable marriage group.
	if len(expected) > 1 {
		adjacency := map[string][]string{}
		for _, pair := range pairs {
			adjacency[pair.Left] = append(adjacency[pair.Left], pair.Right)
			adjacency[pair.Right] = append(adjacency[pair.Right], pair.Left)
		}
		seen := map[string]bool{}
		stack := []string{expected[0].Label}
		for len(stack) > 0 {
			s := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if seen[s] {
				continue
			}
			seen[s] = true
			stack = append(stack, adjacency[s]...)
		}
		if len(seen) != len(expected) {
			valid = false
			out.Problems = append(out.Problems, "MARRIAGE_GRAPH_DISCONNECTED")
		}
	}
	out.ValidMarriage = valid
	out.Complete = valid && len(out.Missing) == 0 && len(out.Problems) == 0
	return out, nil
}

type ChangeControl struct {
	ResourceID           string          `json:"resource_id"`
	ResourceType         string          `json:"resource_type"`
	OldVersion           int             `json:"old_version"`
	NewVersion           int             `json:"new_version"`
	ReasonCode           string          `json:"reason_code"`
	AffectedStakeholders []string        `json:"affected_stakeholders"`
	ApprovedBy           map[string]bool `json:"approved_by"`
}

func EvaluateChange(c ChangeControl) (bool, []string, error) {
	if c.ResourceID == "" || c.ReasonCode == "" || c.OldVersion < 1 || c.NewVersion != c.OldVersion+1 || len(c.AffectedStakeholders) == 0 {
		return false, nil, errors.New("CHANGE_REQUEST_INVALID")
	}
	switch c.ResourceType {
	case "PO_TARGET", "STYLE_LEAD", "DAILY_CUT", "PANEL_NUMBERING", "SEWING_PLAN":
	default:
		return false, nil, errors.New("CHANGE_RESOURCE_UNKNOWN")
	}
	seen := map[string]bool{}
	missing := []string{}
	for _, name := range c.AffectedStakeholders {
		if name == "" || seen[name] {
			return false, nil, errors.New("CHANGE_STAKEHOLDER_INVALID")
		}
		seen[name] = true
		if !c.ApprovedBy[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return len(missing) == 0, missing, nil
}
