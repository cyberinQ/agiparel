// Author: Sunil Jain, 2026-10-25 — Regression tests for AGIE executive question routing.
package main

import (
	"strings"
	"testing"

	"github.com/cyberinq/agiparel/services/edge-daemon/internal/agie/interviewer"
)

func TestOperationalIntent(t *testing.T) {
	tests := []struct{ text, want string }{
		{"how many workers present todaY", "attendance_today"},
		{"How many employees are present?", "attendance_today"},
		{"who is present and who is absent today", "attendance_today"},
		{"Who is absent today?", "attendance_today"},
		{"Who's late this shift?", "attendance_today"},
		{"Who is on leave today?", "attendance_today"},
		{"who attended the meeting today?", ""},
		{"which po is in progress?", "po_in_progress"},
		{"Any active purchase orders?", "po_in_progress"},
		{"Please explain machine bottlenecks", ""},
	}
	for _, tt := range tests {
		got := operationalIntent([]interviewer.LLMMessage{{Role: "user", Content: tt.text}})
		if got != tt.want {
			t.Errorf("intent %q: got %q; want %q", tt.text, got, tt.want)
		}
	}
}

func TestIntakeFailureSummary(t *testing.T) {
	tests := []struct{ input, contains string }{
		{"[ERROR] Extracting scan.pdf: OCR unavailable\n[FATAL] 1 documents failed extraction", "scan.pdf"},
		{"[FATAL] PostgreSQL transaction rolled back: permission denied", "PostgreSQL"},
		{"garbled subprocess output", "container logs"},
	}
	for _, tt := range tests {
		got := intakeFailureSummary(tt.input)
		if !strings.Contains(got, tt.contains) {
			t.Errorf("summary %q does not contain %q", got, tt.contains)
		}
	}
}

// Author: Sunil Jain, 2026-10-25 — Regression: capacity is a ground-truth intent, not a generic LLM request.
func TestFactoryCapacityIntent(t *testing.T) {
	cases := []string{"what's our total capacity", "What is the factory capacity?", "how many pieces can we produce per day?"}
	for _, question := range cases {
		got := operationalIntent([]interviewer.LLMMessage{{Role: "user", Content: question}})
		if got != "factory_capacity" {
			t.Fatalf("%q: got %q", question, got)
		}
	}
}
