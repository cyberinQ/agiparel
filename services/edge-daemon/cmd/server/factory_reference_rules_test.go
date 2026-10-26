package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReferenceMasterFactsAreTypedAndKeyBound(t *testing.T) {
	tests := []struct{ kind, key, raw, basis string }{
		{"MACHINE", "MC-101", `{"machine_id":"MC-101","machine_type":"Lockstitch","status_note":"Working"}`, ""},
		{"SKILL", "W-02:Topstitch", `{"worker_id":"W-02","skill":"Topstitch","level_note":"Trained"}`, ""},
		{"RATE", "Machine hourly:USD", `{"cost_type":"Machine hourly","rate":"7.5000","currency":"USD"}`, "HOUR"},
	}
	for _, tt := range tests {
		got, err := parseReferenceFacts(tt.kind, tt.key, []byte(tt.raw), tt.basis)
		if err != nil {
			t.Errorf("%s unexpected: %v", tt.kind, err)
		}
		if got.Type != tt.kind || got.Key != tt.key {
			t.Errorf("wrong data %+v", got)
		}
	}
}
func TestReferenceFactsNeverInventStockOrCosts(t *testing.T) {
	bad := []struct{ kind, key, raw, basis string }{
		{"MATERIAL", "FAB-A:M", `{"material_code":"FAB-A","quantity":"9999","unit":"M"}`, ""},
		{"MACHINE", "MC-2", `{"machine_id":"MC-1","machine_type":"Overlock"}`, ""},
		{"MACHINE", "MC-1", `{"machine_id":"MC-1","machine_type":""}`, ""},
		{"SKILL", "W-7:Hem", `{"worker_id":"W-9","skill":"Hem"}`, ""},
		{"SKILL", "W-7:Hem", `{"worker_id":"W-7","skill":99}`, ""},
		{"RATE", "Labor:USD", `{"cost_type":"Labor","rate":"-5","currency":"USD"}`, "HOUR"},
		{"RATE", "Labor:USD", `{"cost_type":"Labor","rate":"1.23456","currency":"USD"}`, "HOUR"},
		{"RATE", "Labor:USD", `{"cost_type":"Labor","rate":"=SUM(A1:A5)","currency":"USD"}`, "HOUR"},
		{"RATE", "Labor:USD", `{"cost_type":"Labor","rate":"5","currency":"USD"}`, ""},
		{"RATE", "Labor:USD", `{"cost_type":"Labor","rate":"5","currency":"EUR"}`, "HOUR"},
		{"MACHINE", "MC-1", `{"machine_id":"MC-1","machine_type":"Lockstitch"}`, "HOUR"},
		{"MACHINE", "MC-1", `[]`, ""},
	}
	for _, tt := range bad {
		if _, err := parseReferenceFacts(tt.kind, tt.key, []byte(tt.raw), tt.basis); err == nil {
			t.Errorf("unsafe %s/%s accepted: %s", tt.kind, tt.key, tt.raw)
		}
	}
}
func TestReferenceActionsRequireExplicitReview(t *testing.T) {
	id := "FC-0123456789abcdef0123456789abcdef"
	ok := func(action, typ, key, basis string, rev, draft int, note string) error {
		return validReferenceAction(action, typ, key, id, rev, draft, note, basis)
	}
	if err := ok("PUBLISH", "MACHINE", "MC-1", "", 0, 1, "Machine plate inspected"); err != nil {
		t.Fatal(err)
	}
	if err := ok("WITHDRAW", "RATE", "Labor:USD", "HOUR", 3, 2, "Rate no longer applicable"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []struct {
		action, typ, key, basis string
		rev, draft              int
		note                    string
	}{
		{"APPROVE_PRODUCTION", "MACHINE", "MC-1", "", 0, 1, "Explained source record"},
		{"PUBLISH", "MATERIAL", "FAB-1:M", "", 0, 1, "Inspected warehouse roll"},
		{"PUBLISH", "MACHINE", " MC-1", "", 0, 1, "Inspected machine label"},
		{"PUBLISH", "RATE", "Labor:USD", "", 0, 1, "Rate quoted by manager"},
		{"PUBLISH", "MACHINE", "MC-1", "", -1, 1, "Inspected machine label"},
		{"PUBLISH", "MACHINE", "MC-1", "", 0, 0, "Inspected machine label"},
		{"PUBLISH", "MACHINE", "MC-1", "", 0, 1, "short"},
		{"PUBLISH", "MACHINE", "MC-1", "", 0, 1, strings.Repeat("x", 1001)},
	} {
		if err := ok(bad.action, bad.typ, bad.key, bad.basis, bad.rev, bad.draft, bad.note); err == nil {
			t.Errorf("accepted %+v", bad)
		}
	}
	if json.Valid([]byte(`{"entity_type":"RATE","rate_basis":"HOUR"}`)) == false {
		t.Fatal("broken test fixture")
	}
}
