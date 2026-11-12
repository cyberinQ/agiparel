// Author: Sunil Jain, 2026-11-12.
// Route independent factory facts by the CURRENT question's entity; never reuse a previous turn's entity.
package aggieintent

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	AttendanceToday    = "attendance_today_count"
	RegisteredWorkers  = "registered_workers_count"
	InventoryRolls     = "roll_ledger_count"
	RegisteredMachines = "machine_roster_count"
	MachineStatus      = "machine_roster_status"
)

// Counts is retained for v50 callers; Resolve also recognizes machine-status questions.
func Counts(question string) []string { return Resolve(question) }

// Resolve returns only independently supported, read-only intents in question order.
// No conversation history enters the classifier, preventing roll/machine topic leakage.
func Resolve(question string) []string {
	question = strings.ToLower(strings.TrimSpace(question))
	if question == "" {
		return nil
	}
	parts := strings.FieldsFunc(question, func(r rune) bool {
		return r == '?' || r == '!' || r == ';' || r == '\n' || r == '.'
	})
	var result []string
	for _, part := range parts {
		result = append(result, resolveClause(strings.TrimSpace(part))...)
	}
	return result
}

func resolveClause(q string) []string {
	if q == "" {
		return nil
	}
	isQuantity := hasAny(q, "how many", "number of", "count of", "count the", "total number", "machine count", "machines count")
	workerAt := firstPosition(q, "workers", "worker", "employees", "employee", "staff", "people working")
	rollAt := firstPosition(q, "fabric rolls", "rolls", "roll", "inventory rolls")
	machineAt := firstPosition(q, "machines", "machine", "equipment")
	machineStatus := machineAt >= 0 && (hasAny(q,
		"which machine", "what machine", "show machine", "list machine", "machine status", "machines status", "equipment status") ||
		hasAny(q, "running", "working", "available", "open", "idle", "active", "offline", "online", "down", "broken", "in use", "ready", "operating"))
	if !isQuantity && !machineStatus {
		return nil
	}

	type located struct {
		at     int
		intent string
	}
	intents := make([]located, 0, 3)
	if isQuantity && workerAt >= 0 {
		intent := RegisteredWorkers
		if hasAny(q, "today", "this shift", "right now", "present", "on duty", "absent") {
			intent = AttendanceToday
		}
		intents = append(intents, located{workerAt, intent})
	}
	if isQuantity && rollAt >= 0 {
		intents = append(intents, located{rollAt, InventoryRolls})
	}
	if machineAt >= 0 && (isQuantity || machineStatus) {
		intent := RegisteredMachines
		if machineStatus {
			intent = MachineStatus
		}
		intents = append(intents, located{machineAt, intent})
	}
	if len(intents) == 0 {
		return nil
	}
	for i := 1; i < len(intents); i++ {
		for j := i; j > 0 && intents[j].at < intents[j-1].at; j-- {
			intents[j], intents[j-1] = intents[j-1], intents[j]
		}
	}
	result := make([]string, 0, len(intents))
	for _, item := range intents {
		result = append(result, item.intent)
	}
	return result
}

func hasAny(q string, terms ...string) bool {
	for _, term := range terms {
		if firstPosition(q, term) >= 0 {
			return true
		}
	}
	return false
}

// firstPosition requires word/phrase boundaries so "roll" cannot match "payroll"
// and "machine" cannot match "machinery" or unrelated longer words.
func firstPosition(q string, terms ...string) int {
	earliest := -1
	for _, term := range terms {
		from := 0
		for from < len(q) {
			ix := strings.Index(q[from:], term)
			if ix < 0 {
				break
			}
			at := from + ix
			end := at + len(term)
			before := at == 0
			if !before {
				r, _ := utf8.DecodeLastRuneInString(q[:at])
				before = !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
			}
			after := end == len(q)
			if !after {
				r, _ := utf8.DecodeRuneInString(q[end:])
				after = !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
			}
			if before && after {
				if earliest < 0 || at < earliest {
					earliest = at
				}
				break
			}
			from = at + 1
		}
	}
	return earliest
}
