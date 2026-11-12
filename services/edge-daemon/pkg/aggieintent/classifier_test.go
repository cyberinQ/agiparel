// Author: Sunil Jain, 2026-11-12.
// Ensure multiple factory counts are routed deterministically and without guesswork.
package aggieintent

import (
	"reflect"
	"testing"
)

func TestCounts(t *testing.T) {
	cases := []struct {
		q    string
		want []string
	}{
		{"how many workers are in the factory today? how many rolls in inventory?", []string{AttendanceToday, InventoryRolls}},
		{"How many employees are present today?", []string{AttendanceToday}},
		{"how many workers are registered in the factory?", []string{RegisteredWorkers}},
		{"How many fabric rolls in stock?", []string{InventoryRolls}},
		{"How many rolls do we have and how many staff today?", []string{InventoryRolls, AttendanceToday}},
		{"How many workers are absent today?", []string{AttendanceToday}},
		{"How many sweaters are in inventory?", nil},
		{"How do I check workers?", nil},
		{"What is the worker manual?", nil},
		{"show my rolls", nil},
	}
	for _, tt := range cases {
		if got := Counts(tt.q); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Counts(%q) = %v, want %v", tt.q, got, tt.want)
		}
	}
}
