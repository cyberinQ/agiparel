// Batch 08A: deterministic classic-bundle proposals. The units represented
// here are operator-reported/planned; NONE are verified cutting or RFID scans.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

type cutPosition struct {
	Position  int    `json:"position"`
	ColorCode string `json:"color_code"`
	SizeCode  string `json:"size_code"`
}
type cutPart struct {
	PartCode         string `json:"part_code"`
	PiecesPerGarment int    `json:"pieces_per_garment"`
}
type cutDraftInput struct {
	IntakeID           string        `json:"intake_id"`
	CutNo              int           `json:"cut_no"`
	ExpectedRevision   int           `json:"expected_revision"`
	RequestKey         string        `json:"request_key"`
	MarkerCode         string        `json:"marker_code"`
	FabricSKU          string        `json:"fabric_sku"`
	ShadeBand          string        `json:"shade_band"`
	MarkerLengthMM     int64         `json:"marker_length_mm"`
	UsableWidthMM      int           `json:"usable_width_mm"`
	ReportedPlies      int           `json:"reported_plies"`
	BundlePlyGroupSize int           `json:"bundle_ply_group_size"`
	Positions          []cutPosition `json:"positions"`
	Parts              []cutPart     `json:"parts"`
	Note               string        `json:"note"`
}
type cutBundle struct {
	LabelCode        string `json:"label_code"`
	Position         int    `json:"position"`
	ColorCode        string `json:"color_code"`
	SizeCode         string `json:"size_code"`
	PartCode         string `json:"part_code"`
	PiecesPerGarment int    `json:"pieces_per_garment"`
	PlyGroup         int    `json:"ply_group"`
	PlyStart         int    `json:"ply_start"`
	PlyEnd           int    `json:"ply_end"`
	PieceCount       int    `json:"piece_count"`
	RFIDState        string `json:"rfid_state"`
}
type cutDraftReport struct {
	DraftID            string      `json:"draft_id"`
	IntakeID           string      `json:"intake_id"`
	PONumber           string      `json:"po_number"`
	CutNo              int         `json:"cut_no"`
	Revision           int         `json:"revision"`
	MarkerCode         string      `json:"marker_code"`
	FabricSKU          string      `json:"fabric_sku"`
	ShadeBand          string      `json:"shade_band"`
	MarkerLengthMM     int64       `json:"marker_length_mm"`
	UsableWidthMM      int         `json:"usable_width_mm"`
	ReportedPlies      int         `json:"reported_plies"`
	BundlePlyGroupSize int         `json:"bundle_ply_group_size"`
	PlannedGarments    int         `json:"planned_garments"`
	PlannedParts       int         `json:"planned_parts"`
	PositionCount      int         `json:"position_count"`
	BundleCount        int         `json:"bundle_count"`
	SourceOrderSHA256  string      `json:"source_order_sha256"`
	SnapshotSHA256     string      `json:"snapshot_sha256"`
	State              string      `json:"state"`
	Authority          string      `json:"authority"`
	IssuedLengthMM     int64       `json:"verified_issued_length_mm"`
	VerifiedIssueCount int         `json:"verified_issue_count"`
	Warning            string      `json:"warning"`
	Note               string      `json:"note"`
	CreatedAt          string      `json:"created_at,omitempty"`
	Bundles            []cutBundle `json:"bundles"`
}

func validateCutDraft(v cutDraftInput) error {
	if !passportID.MatchString(v.IntakeID) || len(v.IntakeID) > 40 {
		return errors.New("intake_id: choose a staged buyer order")
	}
	if v.CutNo < 1 || v.CutNo > 100000 || v.ExpectedRevision < 0 || v.ExpectedRevision > 10000 {
		return errors.New("cut_no or expected_revision out of range")
	}
	if !storesRequestKey.MatchString(v.RequestKey) {
		return errors.New("request_key: 8–128 letters/numbers/_/-")
	}
	if !passportID.MatchString(v.MarkerCode) || !passportCode.MatchString(v.FabricSKU) {
		return errors.New("marker_code and fabric_sku must be valid identifiers")
	}
	if strings.TrimSpace(v.ShadeBand) == "" || len(v.ShadeBand) > 32 || strings.ContainsAny(v.ShadeBand, "\x00\r\n") {
		return errors.New("shade_band required, maximum 32")
	}
	if v.MarkerLengthMM < 100 || v.MarkerLengthMM > 100000000 || v.UsableWidthMM < 100 || v.UsableWidthMM > 4000 {
		return errors.New("marker length or width outside supported range")
	}
	if v.ReportedPlies < 1 || v.ReportedPlies > 210 {
		return errors.New("reported_plies: 1–210")
	}
	if v.BundlePlyGroupSize < 1 || v.BundlePlyGroupSize > 210 {
		return errors.New("bundle_ply_group_size: 1–210")
	}
	if len(strings.TrimSpace(v.Note)) < 12 || len(v.Note) > 1000 || strings.ContainsRune(v.Note, 0) {
		return errors.New("note: 12–1000 characters of operator evidence")
	}
	if len(v.Positions) == 0 || len(v.Positions) > 80 || len(v.Parts) == 0 || len(v.Parts) > 24 || len(v.Positions)*len(v.Parts)*((v.ReportedPlies+v.BundlePlyGroupSize-1)/v.BundlePlyGroupSize) > 640 {
		return errors.New("1–80 positions, 1–24 parts, maximum 640 sub-bundles")
	}
	seenPos := make(map[int]bool)
	for _, p := range v.Positions {
		if p.Position < 1 || p.Position > 80 || seenPos[p.Position] || !passportCode.MatchString(p.ColorCode) || len(p.ColorCode) > 64 || !passportCode.MatchString(p.SizeCode) || len(p.SizeCode) > 32 {
			return errors.New("positions: unique marker positions, known color and size codes required")
		}
		seenPos[p.Position] = true
	}
	for n := 1; n <= len(v.Positions); n++ {
		if !seenPos[n] {
			return errors.New("marker positions must be contiguous from 1")
		}
	}
	seenParts := make(map[string]bool)
	for _, p := range v.Parts {
		if !passportID.MatchString(p.PartCode) || seenParts[p.PartCode] || p.PiecesPerGarment < 1 || p.PiecesPerGarment > 8 {
			return errors.New("parts: unique code and 1–8 pieces per garment")
		}
		seenParts[p.PartCode] = true
	}
	return nil
}
func cutInputHash(v cutDraftInput) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func cutLabel(tenant, intake string, cut, rev, pos, group int, part string) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("CUT08A|%s|%s|%d|%d|%d|%d|%s", tenant, intake, cut, rev, pos, group, part)))
	return "CB08-" + strings.ToUpper(hex.EncodeToString(h[:12]))
}
func buildCutBundles(tenant string, v cutDraftInput, revision int) ([]cutBundle, error) {
	if e := validateCutDraft(v); e != nil {
		return nil, e
	}
	positions := append([]cutPosition(nil), v.Positions...)
	parts := append([]cutPart(nil), v.Parts...)
	sort.Slice(positions, func(i, j int) bool { return positions[i].Position < positions[j].Position })
	sort.Slice(parts, func(i, j int) bool { return parts[i].PartCode < parts[j].PartCode })
	groups := (v.ReportedPlies + v.BundlePlyGroupSize - 1) / v.BundlePlyGroupSize
	out := make([]cutBundle, 0, len(positions)*len(parts)*groups)
	for _, position := range positions {
		for _, part := range parts {
			for group := 1; group <= groups; group++ {
				first := (group-1)*v.BundlePlyGroupSize + 1
				last := group * v.BundlePlyGroupSize
				if last > v.ReportedPlies {
					last = v.ReportedPlies
				}
				out = append(out, cutBundle{
					LabelCode: cutLabel(tenant, v.IntakeID, v.CutNo, revision, position.Position, group, part.PartCode),
					Position:  position.Position, ColorCode: position.ColorCode, SizeCode: position.SizeCode,
					PartCode: part.PartCode, PiecesPerGarment: part.PiecesPerGarment, PlyGroup: group,
					PlyStart: first, PlyEnd: last, PieceCount: (last - first + 1) * part.PiecesPerGarment,
					RFIDState: "UNBOUND_DRAFT_LABEL",
				})
			}
		}
	}
	return out, nil
}
func cutCounts(positions []cutPosition, plies int) map[string]int64 {
	totals := make(map[string]int64)
	for _, p := range positions {
		totals[p.ColorCode+"\x00"+p.SizeCode] += int64(plies)
	}
	return totals
}

// Compare historical latest-revision counts to the actual source SKU matrix.
// Caller serializes all writes for one (tenant,intake) using an advisory lock.
func checkCutQuantities(allowed, other, proposed map[string]int64) error {
	if len(allowed) == 0 {
		return errors.New("SKU_MATRIX_MISSING")
	}
	for key, q := range proposed {
		max, ok := allowed[key]
		if !ok {
			return errors.New("SKU_NOT_IN_BUYER_ORDER")
		}
		if q < 1 || other[key] < 0 || q > max || other[key] > max-q {
			return errors.New("OVERPLANNED_BUYER_SKU")
		}
	}
	return nil
}

// A second independent cap guards the PO's overall unit count even if source
// SKU lines were imported inconsistently or later systems add overcut rules.
func checkCutTotal(ordered int64, previous, proposed map[string]int64) error {
	if ordered <= 0 {
		return errors.New("INVALID_ORDER_QUANTITY")
	}
	remaining := ordered
	for _, quantity := range previous {
		if quantity < 0 || quantity > remaining {
			return errors.New("OVERPLANNED_BUYER_ORDER")
		}
		remaining -= quantity
	}
	for _, quantity := range proposed {
		if quantity <= 0 || quantity > remaining {
			return errors.New("OVERPLANNED_BUYER_ORDER")
		}
		remaining -= quantity
	}
	return nil
}
