// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestF310CommandValidationAndSignatureBinding(t *testing.T) {
	base := wipExceptionCommand{CommandID: "REPORT_EXCEPT_012345", PlantID: "plant_01", PartID: "c6e63b80-e150-4364-90be-759755e3ec46", StationID: "sewing_02", Action: "OPEN", IssueType: "LOST_CARRIER", Note: "Reported carrier physically missing"}
	if !exceptionValid(base) {
		t.Fatal("valid report rejected")
	}
	a := exceptionChallenge(base)
	if !strings.HasPrefix(a, "WIP_EXCEPTION_V1:") || len(a) != len("WIP_EXCEPTION_V1:")+64 {
		t.Fatalf("bad challenge %q", a)
	}
	p, r := exceptionPermission("OPEN")
	if p != "sewing.exception.report" || r != "sewing_operator" {
		t.Fatal("incorrect reporting grant")
	}
	p, r = exceptionPermission("CLOSE")
	if p != "sewing.exception.resolve" || r != "sewing_supervisor" {
		t.Fatal("incorrect independent resolution")
	}
	for _, issue := range []string{"UNREADABLE_CARRIER", "MISSING_WIP", "FAILED_SCAN", "MISROUTED_PART", "REJECTED_HANDOFF"} {
		c := base
		c.IssueType = issue
		if !exceptionValid(c) {
			t.Errorf("valid issue rejected: %s", issue)
		}
	}
	changes := []func(*wipExceptionCommand){
		func(c *wipExceptionCommand) { c.IssueType = "AUTO_RESET" },
		func(c *wipExceptionCommand) { c.Note = "too short" },
		func(c *wipExceptionCommand) { c.PartID = "part-1" },
		func(c *wipExceptionCommand) { c.CommandID = "short" },
		func(c *wipExceptionCommand) { c.ScanID = "c6e63b80-e150-4364-90be-759755e3ec46" },
		func(c *wipExceptionCommand) { c.Action = "OVERRIDE" },
	}
	for i, change := range changes {
		c := base
		change(&c)
		if exceptionValid(c) {
			t.Errorf("accepted malformed report %d", i)
		}
	}
	modified := base
	modified.Note = "Reported carrier physically located"
	if exceptionChallenge(modified) == a {
		t.Fatal("command not bound to reason")
	}
	modified = base
	modified.StationID = "other_station"
	if exceptionChallenge(modified) == a {
		t.Fatal("command not bound to station")
	}
	close := base
	close.Action = "CLOSE"
	close.IssueType = ""
	close.CaseID = "157abc90-4632-4ea0-af39-e2d48c85a487"
	close.ScanID = "6fc83ccc-aacd-41db-af3c-b74e13460ed1"
	if !exceptionValid(close) {
		t.Fatal("valid close rejected")
	}
	close.IssueType = "LOST_CARRIER"
	if exceptionValid(close) {
		t.Fatal("close accepted issue mutation")
	}
}
func TestF310QueryAndDevelopmentGate(t *testing.T) {
	q := url.Values{"tenant_id": {"tenant_01"}, "plant_id": {"plant_01"}, "part_id": {"c6e63b80-e150-4364-90be-759755e3ec46"}}
	tenant, plant, part, limit, e := exceptionQuery(q)
	if e != nil || tenant != "tenant_01" || plant != "plant_01" || part == "" || limit != 25 {
		t.Fatal("valid query rejected", e)
	}
	for _, bad := range []url.Values{
		{"tenant_id": {"t"}, "plant_id": {"p"}, "part_id": {"bad"}},
		{"tenant_id": {"t1", "t2"}, "plant_id": {"p1"}, "part_id": {"c6e63b80-e150-4364-90be-759755e3ec46"}},
		{"tenant_id": {"t1"}, "plant_id": {"p1"}, "part_id": {"c6e63b80-e150-4364-90be-759755e3ec46"}, "limit": {"100"}},
		{"tenant_id": {"t1"}, "plant_id": {"p1"}, "part_id": {"c6e63b80-e150-4364-90be-759755e3ec46"}, "admin": {"1"}},
	} {
		if _, _, _, _, err := exceptionQuery(bad); err == nil {
			t.Errorf("accepted unsafe query %v", bad)
		}
	}
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_WIP_EXCEPTIONS_ENABLED", "true")
	p := newWIPExceptionAPI(nil)
	for _, path := range []string{"/api/v1/wip/exceptions/cases", "/api/v1/wip/exceptions/challenge", "/api/v1/wip/exceptions/events"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		p.cases(w, req)
		if w.Code != 503 || !strings.Contains(w.Body.String(), "ERR_F310_DISABLED") {
			t.Fatalf("not disabled %s %d %s", path, w.Code, w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	p.event(w, httptest.NewRequest(http.MethodPost, "/api/v1/wip/exceptions/events", strings.NewReader(`{}`)))
	if w.Code != 503 {
		t.Fatalf("unauthorized write not disabled: %d", w.Code)
	}
}
func TestF310IngressExplicitOnly(t *testing.T) {
	t.Setenv("ENV", "production")
	h := configuredOperatorGate(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
	for _, path := range []string{"/api/v1/wip/exceptions/challenge", "/api/v1/wip/exceptions/events", "/api/v1/wip/exceptions/cases", "/static/factory_wip_exceptions.js"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 204 {
			t.Errorf("blocked explicit ingress %s: %d", path, w.Code)
		}
	}
	for _, path := range []string{"/api/v1/wip/exceptions/force-move", "/api/v1/wip/exceptions/delete"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code == 204 {
			t.Errorf("overbroad exception ingress: %s", path)
		}
	}
}
