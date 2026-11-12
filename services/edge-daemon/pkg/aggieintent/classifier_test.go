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

// Author: Sunil Jain, 2026-11-12.
// A fresh user turn must never inherit the previous roll question's entity.
func TestMachineIntentsAndEntityIsolation(t *testing.T) {
	cases := []struct {
		question string
		want     []string
	}{
		{"How many machines?", []string{RegisteredMachines}},
		{"how many machines do we have?", []string{RegisteredMachines}},
		{"number of machines", []string{RegisteredMachines}},
		{"Which machines are open?", []string{MachineStatus}},
		{"which machines are running today?", []string{MachineStatus}},
		{"Show machine status", []string{MachineStatus}},
		{"How many machines are available?", []string{MachineStatus}},
		{"How many rolls in inventory? How many machines?", []string{InventoryRolls, RegisteredMachines}},
		{"How many machines and workers are registered?", []string{RegisteredMachines, RegisteredWorkers}},
		{"How many machines, workers today, and rolls?", []string{RegisteredMachines, AttendanceToday, InventoryRolls}},
		{"How many rolls of fabric?", []string{InventoryRolls}},
		{"How many payroll entries?", nil},
		{"How many machinery patents?", nil},
		{"Which people are on leave?", nil},
		{"How many sewing operations?", nil},
		{"How many machines? Which doors are open?", []string{RegisteredMachines}},
		{"How many workers are registered? How many rolls are available today?", []string{RegisteredWorkers, InventoryRolls}},
	}
	for _, tc := range cases {
		if got := Resolve(tc.question); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Resolve(%q)=%v want=%v", tc.question, got, tc.want)
		}
	}
	turns := []struct {
		question string
		entity   string
	}{
		{"How many rolls?", InventoryRolls},
		{"How many machines?", RegisteredMachines},
		{"How many workers are registered?", RegisteredWorkers},
		{"Which machines are open?", MachineStatus},
		{"How many rolls?", InventoryRolls},
	}
	for _, turn := range turns {
		got := Resolve(turn.question)
		if len(got) != 1 || got[0] != turn.entity {
			t.Fatalf("entity leak: %q produced %v, want %s", turn.question, got, turn.entity)
		}
	}
}
