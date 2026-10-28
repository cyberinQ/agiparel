// Author: Sunil Jain, 2026-10-28.
// F1.3 invariant tests: canonical chapter actions, scope, and strict denial.
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/attestation"
)

func TestWorkforceChapterCatalogCoversAllTen(t *testing.T) {
	seen := map[int]bool{}
	for key, policy := range chapterPermissions {
		if policy.Chapter < 1 || policy.Chapter > 10 || policy.Authority == "" || key == "" {
			t.Fatalf("invalid entry %q: %+v", key, policy)
		}
		seen[policy.Chapter] = true
	}
	for n := 1; n <= 10; n++ {
		if !seen[n] {
			t.Fatalf("chapter %d missing", n)
		}
	}
	for permission, chapter := range map[string]int{"sewing.complete": 6, "quality.inspect": 7, "quality.repair": 7, "quality.read": 7, "packing.stage": 8, "packing.read": 8, "intelligence.read": 9} {
		if chapterPermissions[permission].Chapter != chapter {
			t.Fatalf("wrong route permission: %s", permission)
		}
	}
}
func TestWorkforceChapterActorRequiresScopedTrustedIdentity(t *testing.T) {
	good := attestation.Claims{Issuer: "https://idp.example.test", Subject: "worker_1", TenantID: "tenant_1", PlantID: "plant_1"}
	if !validChapterActor(good) {
		t.Fatal("rejected correct actor")
	}
	for _, bad := range []attestation.Claims{
		{Issuer: "", Subject: good.Subject, TenantID: good.TenantID, PlantID: good.PlantID},
		{Issuer: "http://fake.local", Subject: good.Subject, TenantID: good.TenantID, PlantID: good.PlantID},
		{Issuer: good.Issuer, Subject: "", TenantID: good.TenantID, PlantID: good.PlantID},
		{Issuer: good.Issuer, Subject: good.Subject, TenantID: "", PlantID: good.PlantID},
		{Issuer: good.Issuer, Subject: good.Subject, TenantID: good.TenantID, PlantID: "bad\nplant"},
	} {
		if validChapterActor(bad) {
			t.Fatalf("accepted untrusted actor %+v", bad)
		}
	}
}
func TestWorkforceChapterRouteFailsClosedInDevelopment(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_WORKFORCE_OIDC_ENABLED", "true")
	api := newChapterAccessAPI(nil)
	if api.enabled {
		t.Fatal("enabled without production prerequisites")
	}
	w := httptest.NewRecorder()
	api.report(w, httptest.NewRequest(http.MethodGet, "/api/v1/workforce/chapter-access?tenant_id=t_1&plant_id=p_1", nil))
	if w.Code != 503 || !strings.Contains(w.Body.String(), "ERR_WORKFORCE_CHAPTER_DISABLED") {
		t.Fatalf("unexpected %d %s", w.Code, w.Body.String())
	}
}
func TestWorkforceChapterReadinessDoesNotEnableLegacyRoutes(t *testing.T) {
	for _, n := range []int{1, 2, 3, 4, 5, 10} {
		on, mode := chapterProductionStatus(n)
		if on || mode != "NO_PRODUCTION_ROUTE_AVAILABLE" {
			t.Fatalf("chapter %d wrongly enabled", n)
		}
	}
	t.Setenv("AGIPAREL_SEWING_PRODUCTION_ENABLED", "true")
	on, mode := chapterProductionStatus(6)
	if !on || mode != "SIGNED_TRANSACTION_PROOF_REQUIRED" {
		t.Fatal("signed operation weakened")
	}
}

func TestWorkforceChapterAccessRequestRejectsAmbiguousOrUnboundScopes(t *testing.T) {
	api := &chapterAccessAPI{enabled: true}
	for _, tc := range []struct {
		url    string
		status int
	}{
		{"/api/v1/workforce/chapter-access?tenant_id=t_1", 422},
		{"/api/v1/workforce/chapter-access?tenant_id=t_1&plant_id=p_1&plant_id=p_2", 422},
		{"/api/v1/workforce/chapter-access?tenant_id=t_1&tenant_id=t_2&plant_id=p_1", 422},
		{"/api/v1/workforce/chapter-access?tenant_id=t_1&plant_id=p_1", 401},
	} {
		w := httptest.NewRecorder()
		api.report(w, httptest.NewRequest(http.MethodGet, tc.url, nil))
		if w.Code != tc.status {
			t.Fatalf("%s returned %d expected %d", tc.url, w.Code, tc.status)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("missing no-store")
		}
	}
}
func TestWorkforceChapterUnknownActionFailsWithoutDatabaseAccess(t *testing.T) {
	actor := attestation.Claims{Issuer: "https://idp.example.test", Subject: "worker_1", TenantID: "tenant_1", PlantID: "plant_1"}
	allowed, e := workforceChapterPermit(context.Background(), nil, actor, "dispatch.release")
	if allowed || e == nil {
		t.Fatal("unregistered action did not fail closed")
	}
}
