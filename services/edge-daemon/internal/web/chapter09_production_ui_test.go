// Author: Sunil Jain, 2026-10-28.
// Preserve the report visual system while isolating production evidence from rehearsal and secrets.
package web

import (
	"strings"
	"testing"
)

func TestChapter09ProductionUIContracts(t *testing.T) {
	b, err := GetIntelProductionHTML()
	if err != nil {
		t.Fatal(err)
	}
	page := string(b)
	for _, required := range []string{`data-ag-phase="reports"`, `/static/ux_unified.css`, `/static/chapter_workflow.css`, `/static/factory_intelligence.css`, `factory_intelligence_production.js`, `id="token"`, `id="status"`, `id="measures"`, `not released garments`, `not garment output`} {
		if !strings.Contains(page, required) {
			t.Errorf("missing production UI contract: %s", required)
		}
	}
	for _, bad := range []string{`/api/v1/intelligence/sim/`, `id="capture"`, `id="ask-aggie"`, `Asia/Kolkata`, `John Doe`} {
		if strings.Contains(page, bad) {
			t.Errorf("simulation/control leak: %s", bad)
		}
	}
	js, err := staticFS.ReadFile("static/factory_intelligence_production.js")
	if err != nil {
		t.Fatal(err)
	}
	// F1.2: Browser SSO must send its same-origin session cookie, while explicit
	// bearer-token requests must omit cookies. Assert the mode split rather than
	// requiring the pre-SSO unconditional credentials:'omit' literal.
	for _, required := range []string{
		`AUTHENTICATED_GROUP_EVENTS_NOT_GARMENT_RELEASE`,
		`NOT_MEASURED`, `textContent`, `replaceChildren`,
		`headers:mode==='session'?{}:{Authorization:'Bearer '+token}`,
		`credentials:mode==='session'?'same-origin':'omit'`,
		`fetch('/api/v1/workforce/session',{credentials:'same-origin'`,
		`fetch('/auth/logout',{method:'POST',credentials:'same-origin'`,
		`'X-Agiparel-CSRF':csrf`,
	} {
		if !strings.Contains(string(js), required) {
			t.Errorf("missing secure rendering: %s", required)
		}
	}
	for _, bad := range []string{`localStorage`, `sessionStorage`, `innerHTML`, `eval(`} {
		if strings.Contains(string(js), bad) {
			t.Errorf("unsafe browser storage/rendering: %s", bad)
		}
	}
}
