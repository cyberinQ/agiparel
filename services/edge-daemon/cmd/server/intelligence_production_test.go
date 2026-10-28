// Author: Sunil Jain, 2026-10-28.
// Chapter 09 production evidence: authorization, time scope and unsupported metric safeguards.
package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestIntelProdWindow(t *testing.T) {
	now := time.Date(2026, 10, 28, 20, 0, 0, 0, time.UTC)
	q := url.Values{"since": {"2026-10-27T10:00:00Z"}, "until": {"2026-10-28T10:00:00Z"}}
	start, end, err := intelProdWindow(q, 31, now)
	if err != nil || end.Sub(start) != 24*time.Hour {
		t.Fatalf("valid bounded period: %v", err)
	}
	bad := []url.Values{
		{"since": {"2026-10-27T10:00:00Z"}},
		{"since": {"2026-10-27T10:00:00Z"}, "until": {"2026-10-28T10:00:00Z", "2026-10-28T10:00:00Z"}},
		{"since": {"2026-10-27T10:00:00Z"}, "until": {"2026-10-26T10:00:00Z"}},
		{"since": {"2026-09-01T10:00:00Z"}, "until": {"2026-10-28T10:00:00Z"}},
		{"since": {"2026-10-28T10:00:00Z"}, "until": {"2026-10-29T10:00:00Z"}},
		{"since": {"2026-10-27"}, "until": {"2026-10-28T10:00:00Z"}},
	}
	for _, query := range bad {
		if _, _, e := intelProdWindow(query, 31, now); e == nil {
			t.Fatalf("accepted unsafe window %v", query)
		}
	}
}
func TestIntelProdMetricsNeverInventGarments(t *testing.T) {
	metrics := intelProdMetrics(7, 3, 2, 1, 1, 1, 1)
	found := map[string]intelProdMetric{}
	for _, m := range metrics {
		found[m.ID] = m
	}
	for _, id := range []string{"verified_garments", "dispatched_garments", "first_pass_yield", "labor_efficiency", "order_margin"} {
		m, ok := found[id]
		if !ok || m.Value != nil || m.Status != "NOT_MEASURED" {
			t.Fatalf("fabricated production KPI: %+v", m)
		}
	}
	if m := found["qc_current_hold_groups"]; m.Value == nil || *m.Value != 2 || m.Unit != "groups" {
		t.Fatalf("wrong group basis: %+v", m)
	}
}
func TestIntelProdGateAndStrongAuth(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_INTELLIGENCE_PRODUCTION_ENABLED", "true")
	p := newIntelProdAPI(nil)
	if p.enabled || p.reason != "ERR_INTEL_PROD_ENVIRONMENT" {
		t.Fatalf("unsafe development production gate: %+v", p)
	}
	r := httptest.NewRequest(http.MethodGet, "/api/v1/intelligence/production/report?since=2026-10-27T00:00:00Z&until=2026-10-28T00:00:00Z", nil)
	w := httptest.NewRecorder()
	p.report(w, r)
	if w.Code != 503 || !strings.Contains(w.Body.String(), "ERR_INTEL_PROD_ENVIRONMENT") {
		t.Fatalf("gate: %d %s", w.Code, w.Body.String())
	}
}
func TestIntelProductionOperatorGate(t *testing.T) {
	t.Setenv("ENV", "production")
	h := configuredOperatorGate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	for _, path := range []string{"/intelligence", "/static/factory_intelligence_production.js", "/static/factory_intelligence.css", "/api/v1/intelligence/production/report"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 204 {
			t.Errorf("real IdP-protected read endpoint blocked by Basic gate: %s %d", path, w.Code)
		}
	}
	for _, path := range []string{"/api/v1/intelligence/sim/report", "/static/sewing_execution.js", "/api/v1/intelligence/production/unknown"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 503 {
			t.Errorf("unapproved production path bypassed: %s %d", path, w.Code)
		}
	}
}
