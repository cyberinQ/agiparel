// Author: Sunil Jain, 2026-10-25
// Read-only machine-master reconciliation; no duplicate is merged without physical verification.
package masterdata

import (
	"fmt"
	"sort"
	"strings"
)

type Machine struct {
	ID          string `json:"machine_id"`
	Line        string `json:"line_id"`
	Station     string `json:"station_id"`
	Name        string `json:"name"`
	Class       string `json:"canonical_class"`
	CapacityUPH int    `json:"rated_capacity_uph"`
	Status      string `json:"status"`
}

type Finding struct {
	Code       string   `json:"code"`
	Severity   string   `json:"severity"`
	MachineIDs []string `json:"machine_ids"`
	Message    string   `json:"message"`
	Action     string   `json:"action"`
}

type Report struct {
	Rows            int       `json:"rows"`
	UniqueRecordIDs int       `json:"unique_record_ids"`
	ReviewRequired  bool      `json:"review_required"`
	Findings        []Finding `json:"findings"`
	Notes           []string  `json:"notes"`
}

func canonical(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}

// AuditMachineRoster checks possible duplicate identities and ambiguous capacity.
// Matching station positions do NOT establish duplicate physical assets.
func AuditMachineRoster(machines []Machine) Report {
	out := Report{Rows: len(machines), Findings: []Finding{}, Notes: []string{
		"Suspected duplicates are review-only; neither record is deleted or disabled.",
		"Machine counts and rated UPH are not verified garment production capacity.",
		"A physical serial/asset ID is required before consolidating records.",
	}}
	byID := map[string][]int{}
	byStation := map[string][]int{}
	byNameLocation := map[string][]int{}
	for i, m := range machines {
		id := canonical(m.ID)
		if id == "" {
			out.Findings = append(out.Findings, Finding{"MISSING_ASSET_ID", "BLOCK", []string{m.ID}, "Machine record has no asset ID.", "Assign a verified physical asset ID."})
		} else {
			byID[id] = append(byID[id], i)
		}
		if canonical(m.Line) == "" || canonical(m.Station) == "" {
			out.Findings = append(out.Findings, Finding{"MISSING_LOCATION", "REVIEW", []string{m.ID}, "Machine has no complete line/station location.", "Confirm the current physical location."})
		} else {
			loc := canonical(m.Line) + "|" + canonical(m.Station)
			byStation[loc] = append(byStation[loc], i)
			if canonical(m.Name) != "" {
				byNameLocation[loc+"|"+canonical(m.Name)] = append(byNameLocation[loc+"|"+canonical(m.Name)], i)
			}
		}
		if m.CapacityUPH <= 0 || m.CapacityUPH > 600 {
			out.Findings = append(out.Findings, Finding{"INVALID_RATED_CAPACITY", "BLOCK", []string{m.ID}, fmt.Sprintf("Rated machine UPH (%d) is outside realistic floor bounds [1-600].", m.CapacityUPH), "Confirm capacity and its unit; do not substitute a default."})
		}
		if canonical(m.Class) == "" {
			out.Findings = append(out.Findings, Finding{"MISSING_MACHINE_CLASS", "REVIEW", []string{m.ID}, "Machine class is missing.", "Confirm machine type before matching operations."})
		}
	}
	out.UniqueRecordIDs = len(byID)
	for _, ids := range byID {
		if len(ids) > 1 {
			out.Findings = append(out.Findings, Finding{"DUPLICATE_RECORD_ID", "BLOCK", idsFor(machines, ids), "Two or more records share the same machine ID.", "Resolve record identity before import."})
		}
	}
	for _, ids := range byNameLocation {
		if len(ids) > 1 {
			out.Findings = append(out.Findings, Finding{"SAME_NAME_AT_STATION", "REVIEW", idsFor(machines, ids), "Several machine records have the same name and station.", "Check labels and physical asset serials; decide whether distinct equipment exists."})
		}
	}
	for _, ids := range byStation {
		if len(ids) > 1 {
			out.Findings = append(out.Findings, Finding{"MULTIPLE_ASSETS_AT_STATION", "REVIEW", idsFor(machines, ids), fmt.Sprintf("%d machine records share a station; this may be valid.", len(ids)), "Verify whether the station contains multiple physical assets."})
		}
	}
	sort.Slice(out.Findings, func(i, j int) bool {
		a, b := out.Findings[i], out.Findings[j]
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return strings.Join(a.MachineIDs, ",") < strings.Join(b.MachineIDs, ",")
	})
	out.ReviewRequired = len(out.Findings) > 0
	return out
}

func idsFor(m []Machine, idx []int) []string {
	out := make([]string, 0, len(idx))
	for _, i := range idx {
		out = append(out, m[i].ID)
	}
	sort.Strings(out)
	return out
}
