// Batch 08B. Exact-mm lay evidence, immutable material-issue attribution,
// and SHA-256 challenges for externally authenticated RFID witnesses.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

type cutIssueAllocation struct {
	IssueID     string `json:"issue_id"`
	AllocatedMM int64  `json:"allocated_mm"`
}
type cutSpreadInput struct {
	DraftID                string               `json:"draft_id"`
	RequestKey             string               `json:"request_key"`
	ActualPlies            int                  `json:"actual_plies"`
	MeasuredMarkerLengthMM int64                `json:"measured_marker_length_mm"`
	ConsumptionMM          int64                `json:"reported_consumption_mm"`
	Allocations            []cutIssueAllocation `json:"issue_allocations"`
	Note                   string               `json:"evidence_note"`
}
type cutSpreadReceipt struct {
	ReportID               string               `json:"report_id"`
	DraftID                string               `json:"draft_id"`
	RequestKey             string               `json:"request_key"`
	ActualPlies            int                  `json:"actual_plies"`
	MeasuredMarkerLengthMM int64                `json:"measured_marker_length_mm"`
	ConsumptionMM          int64                `json:"reported_consumption_mm"`
	SnapshotSHA            string               `json:"draft_snapshot_sha256"`
	State                  string               `json:"state"`
	CreatedAt              string               `json:"created_at"`
	Allocations            []cutIssueAllocation `json:"issue_allocations"`
	ScannedBundles         int                  `json:"signed_bundle_witnesses"`
	TotalBundles           int                  `json:"draft_bundle_count"`
}
type cutEligibleIssue struct {
	IssueID                   string `json:"issue_id"`
	RollID                    string `json:"roll_id"`
	IssuedMM                  int64  `json:"issued_mm"`
	AlreadyAttributedMM       int64  `json:"already_attributed_mm"`
	AvailableForAttributionMM int64  `json:"available_for_attribution_mm"`
}

func validateCutSpread(v cutSpreadInput) error {
	if !cutDraftUUID.MatchString(v.DraftID) || !storesRequestKey.MatchString(v.RequestKey) {
		return errors.New("ERR_CUT_SPREAD_ID_OR_REQUEST_KEY")
	}
	if v.ActualPlies < 1 || v.ActualPlies > 210 || v.MeasuredMarkerLengthMM < 100 || v.MeasuredMarkerLengthMM > 100000000 || v.ConsumptionMM <= 0 {
		return errors.New("ERR_CUT_SPREAD_MEASUREMENTS")
	}
	if v.ConsumptionMM < v.MeasuredMarkerLengthMM*int64(v.ActualPlies) {
		return errors.New("ERR_CUT_SPREAD_CONSUMPTION_UNDER_MARKER_LENGTH")
	}
	if len(strings.TrimSpace(v.Note)) < 12 || len(v.Note) > 1000 || strings.ContainsRune(v.Note, 0) {
		return errors.New("ERR_CUT_SPREAD_EVIDENCE_NOTE")
	}
	if len(v.Allocations) < 1 || len(v.Allocations) > 20 {
		return errors.New("ERR_CUT_SPREAD_ALLOCATION_COUNT")
	}
	var total int64
	seen := make(map[string]bool)
	for _, a := range v.Allocations {
		if !cutDraftUUID.MatchString(a.IssueID) || a.AllocatedMM < 1 || a.AllocatedMM > v.ConsumptionMM || seen[a.IssueID] {
			return errors.New("ERR_CUT_SPREAD_ISSUE_ALLOCATION")
		}
		seen[a.IssueID] = true
		if total > v.ConsumptionMM-a.AllocatedMM {
			return errors.New("ERR_CUT_SPREAD_ALLOCATION_TOTAL")
		}
		total += a.AllocatedMM
	}
	if total != v.ConsumptionMM {
		return errors.New("ERR_CUT_SPREAD_ALLOCATION_TOTAL")
	}
	return nil
}
func normalizedCutSpread(v cutSpreadInput) cutSpreadInput {
	v.Allocations = append([]cutIssueAllocation(nil), v.Allocations...)
	sort.Slice(v.Allocations, func(i, j int) bool { return v.Allocations[i].IssueID < v.Allocations[j].IssueID })
	return v
}
func cutSpreadInputHash(v cutSpreadInput) string {
	b, _ := json.Marshal(normalizedCutSpread(v))
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// Signed challenge binds a real, immutable spread report and *exact* label/EPC
// to the tenant, plant, and witness identity. No RFID reader is authenticated
// solely by this challenge or by operator-supplied reader IDs.
type cutScanChallengeFields struct {
	Version     string `json:"version"`
	TenantID    string `json:"tenant_id"`
	PlantID     string `json:"plant_id"`
	SpreadID    string `json:"spread_report_id"`
	DraftID     string `json:"draft_id"`
	SnapshotSHA string `json:"draft_snapshot_sha256"`
	LabelCode   string `json:"label_code"`
	EPCHex      string `json:"epc_hex"`
}

func cutScanChallenge(v cutScanChallengeFields) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func canonicalEPCHex(s string) (string, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if len(s) < 24 || len(s) > 96 || len(s)%2 != 0 {
		return "", errors.New("ERR_CUT_EPC_FORMAT")
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'A' || c > 'F') {
			return "", errors.New("ERR_CUT_EPC_FORMAT")
		}
	}
	return s, nil
}
