// Author: Sunil Jain, 2026-11-12.
// Classify exact worker and fabric-roll count questions without an LLM or raw SQL generation.
package aggieintent

import "strings"

const (
	AttendanceToday   = "attendance_today_count"
	RegisteredWorkers = "registered_workers_count"
	InventoryRolls    = "roll_ledger_count"
)

// Counts recognizes only simple quantity questions, preserving the question order.
// It is deliberately narrow; ambiguous operational requests still use governed tools.
func Counts(question string) []string {
	q := strings.ToLower(strings.TrimSpace(question))
	if q == "" {
		return nil
	}
	isQuantity := strings.Contains(q, "how many") || strings.Contains(q, "number of") ||
		strings.Contains(q, "count of") || strings.Contains(q, "count the") ||
		strings.Contains(q, "total number")
	if !isQuantity {
		return nil
	}
	workerAt := firstPosition(q, "workers", "worker", "employees", "employee", "staff", "people working")
	rollAt := firstPosition(q, "fabric rolls", "rolls", "inventory rolls")
	if workerAt < 0 && rollAt < 0 {
		return nil
	}
	workerIntent := RegisteredWorkers
	if strings.Contains(q, "today") || strings.Contains(q, "this shift") ||
		strings.Contains(q, "right now") || strings.Contains(q, "present") ||
		strings.Contains(q, "on duty") || strings.Contains(q, "absent") {
		workerIntent = AttendanceToday
	}
	if rollAt < 0 {
		return []string{workerIntent}
	}
	if workerAt < 0 {
		return []string{InventoryRolls}
	}
	if workerAt < rollAt {
		return []string{workerIntent, InventoryRolls}
	}
	return []string{InventoryRolls, workerIntent}
}

func firstPosition(q string, terms ...string) int {
	pos := -1
	for _, term := range terms {
		if n := strings.Index(q, term); n >= 0 && (pos < 0 || n < pos) {
			pos = n
		}
	}
	return pos
}
