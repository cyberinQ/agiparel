// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func validReviewV33(kind string) factoryReleaseCommandV33 {
	c := factoryReleaseCommandV33{Kind: kind, PlantID: "PLANT-01", CommandID: "factory-review-0033", Note: "Independently documented factory review without external custody authority"}
	if kind == "SAFETY_CLEARANCE" {
		c.IncidentID = "11111111-1111-4111-8111-111111111111"
		c.FollowupPassID = "22222222-2222-4222-8222-222222222222"
		c.GarmentID = "33333333-3333-4333-8333-333333333333"
		c.AccountedFragments = 2
		c.EvidenceDocumentID = "DOCUMENT-EVIDENCE-01"
		c.EvidenceSHA256 = strings.Repeat("a", 64)
	} else {
		c.PlanID = "44444444-4444-4444-8444-444444444444"
		c.Decision = "INTERNAL_RECOMMEND_ACCEPT"
		c.PassCount = 20
		c.MinorCount = 2
	}
	return c
}
func TestFactoryReleaseV33IndependentActionsAndPayload(t *testing.T) {
	for _, tc := range []struct{ kind, action, path string }{
		{"SAFETY_CLEARANCE", "quality.needle.clear", "/api/v1/quality/safety/clearances"},
		{"AQL_REVIEW", "quality.aql.review", "/api/v1/quality/aql/reviews"},
	} {
		c := validReviewV33(tc.kind)
		action, role, ok := factoryReleasePolicyV33(c)
		if !ok || action != tc.action || role != "quality_inspector" {
			t.Fatalf("invalid action %q %q", action, role)
		}
		if !factoryReleasePathV33(tc.path, tc.kind) {
			t.Fatal("correct route blocked")
		}
		for _, wrong := range []string{"/api/v1/quality/other", "/api/v1/quality/aql/reviews", "/api/v1/quality/safety/clearances"} {
			if wrong != tc.path && factoryReleasePathV33(wrong, tc.kind) {
				t.Fatal("cross-workflow route accepted")
			}
		}
		before := factoryReleaseChallengeV33(c)
		c.Note += " tampered"
		if before == factoryReleaseChallengeV33(c) {
			t.Fatal("challenge unbound")
		}
	}
}
func TestFactoryReleaseV33InvalidFields(t *testing.T) {
	c := validReviewV33("SAFETY_CLEARANCE")
	c.PlanID = "forged"
	if _, _, ok := factoryReleasePolicyV33(c); ok {
		t.Fatal("safety accepted forged AQL")
	}
	c = validReviewV33("AQL_REVIEW")
	c.MinorCount = -1
	if _, _, ok := factoryReleasePolicyV33(c); ok {
		t.Fatal("negative count accepted")
	}
	c = validReviewV33("AQL_REVIEW")
	c.Decision = "BUYER_ACCEPTED"
	if _, _, ok := factoryReleasePolicyV33(c); ok {
		t.Fatal("claimed buyer acceptance")
	}
}
func TestFactoryReleaseV33Disabled(t *testing.T) {
	t.Setenv("AGIPAREL_FACTORY_RELEASE_V33_ENABLED", "false")
	p := newFactoryReleaseAPIv33(&EdgeServer{})
	if p.enabled {
		t.Fatal("unconfigured release enabled")
	}
	for i, endpoint := range []func(http.ResponseWriter, *http.Request){p.challenge, p.event, p.dockReadiness} {
		w := httptest.NewRecorder()
		method := http.MethodPost
		if i == 2 {
			method = http.MethodGet
		}
		r := httptest.NewRequest(method, "/api/v1/packing/dock/readiness", nil)
		endpoint(w, r)
		if w.Code < 400 {
			t.Fatalf("disabled request succeeded: %d", w.Code)
		}
	}
}
