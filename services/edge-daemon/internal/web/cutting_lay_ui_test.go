package web

import (
	"strings"
	"testing"
)

func TestCuttingWorkspaceBatch08ADoesNotPromoteDemo(t *testing.T) {
	b, e := GetCuttingViewHTML()
	if e != nil {
		t.Fatal(e)
	}
	body := string(b)
	for _, phrase := range []string{"Cutting & Bundle Genealogy", "PROPOSED / HOLD", "NOT RELEASED", "/static/cutting_lay.js", "/static/workspace_nav.js", "not RFID EPCs"} {
		if !strings.Contains(body, phrase) {
			t.Errorf("cutting workspace missing %q", phrase)
		}
	}
	for _, leak := range []string{"ROLL-007 (59m Denim)", "simulateCameraFlaw()", "Pants from This Roll"} {
		if strings.Contains(body, leak) {
			t.Errorf("demo claim persisted %q", leak)
		}
	}
	script, e := staticFS.ReadFile("static/cutting_lay.js")
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(script), "/api/v1/cutting/lay-drafts") {
		t.Fatal("cutting draft API not wired")
	}
}
