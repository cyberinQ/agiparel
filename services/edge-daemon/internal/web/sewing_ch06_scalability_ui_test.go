// Author: Sunil Jain, 2026-10-28
// Ensure scalable Chapter 06 changes preserve the shared visual shell.
package web

import (
	"strings"
	"testing"
)

func TestSew06ScalableStationFlowRetainsSharedVisualContract(t *testing.T) {
	page, err := staticFS.ReadFile("static/sewing_execution.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(page)
	for _, required := range []string{`data-ag-phase="sewing"`, `id="moreLays"`, `id="moreGroups"`, `id="historyScope"`, `id="dropPending"`, `id="identityHelp"`, `id="station"`, `id="worker"`, `src="/static/workspace_nav.js"`, `/static/ux_unified.css`, `/static/chapter_workflow.css`} {
		if !strings.Contains(html, required) {
			t.Errorf("missing contract %s", required)
		}
	}
	for _, unsafe := range []string{`value="SIM-PLANT-01"`, `value="SEW-ST04"`, `value="SIM-OP01"`, `value="LOCKSTITCH"`, `value="36"`} {
		if strings.Contains(html, unsafe) {
			t.Errorf("factory-specific default remains %s", unsafe)
		}
	}
	js, err := staticFS.ReadFile("static/sewing_execution.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"groups_only=1", "group_cursor", "lay_cursor", "bundle_position", "cycles_has_more", "active_conflict", "sameActor", "state.generation"} {
		if !strings.Contains(string(js), required) {
			t.Errorf("scalable UI contract missing %s", required)
		}
	}
}
