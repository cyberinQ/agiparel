package web

import (
	"strings"
	"testing"
)

func TestStoresAdmissionVisibleButNotApprovedByDev(t *testing.T) {
	b, e := GetStoresHTML()
	if e != nil {
		t.Fatal(e)
	}
	for _, s := range []string{"/static/stores_admission.js", "id=\"roll-admission-workbench\"", "id=\"admission-queue\"", "id=\"admission-identity\"", "two separate external-IdP signatures"} {
		if !strings.Contains(string(b), s) {
			t.Errorf("missing screen element %q", s)
		}
	}
	js, e := staticFS.ReadFile("static/stores_admission.js")
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(js), "/api/v1/stores/admissions") {
		t.Fatal("missing admission readiness API")
	}
	for _, s := range []string{"method:'POST'", "/api/v1/material/roll-admissions',"} {
		if strings.Contains(string(js), s) {
			t.Fatalf("the operator browser must not authorize signed stock: %s", s)
		}
	}
}
