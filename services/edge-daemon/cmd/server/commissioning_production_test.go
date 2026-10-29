// Author: Sunil Jain, CyberinQ LLC, 2026-10-28.
package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestCommissioningQueryAndPagination(t *testing.T) {
	valid := url.Values{"tenant_id": {"tenantA"}, "plant_id": {"Plant_1"}, "limit": {"100"}}
	tenant, plant, cursor, limit, e := commissionPage(valid)
	if e != nil || tenant != "tenantA" || plant != "Plant_1" || cursor != "" || limit != 100 {
		t.Fatalf("valid page: %s %s %s %d %v", tenant, plant, cursor, limit, e)
	}
	for _, q := range []url.Values{
		{"tenant_id": {"tenantA"}},
		{"tenant_id": {"tenantA", "tenantB"}, "plant_id": {"Plant_1"}},
		{"tenant_id": {"tenantA"}, "plant_id": {"Plant_1"}, "limit": {"101"}},
		{"tenant_id": {"tenantA"}, "plant_id": {"Plant_1"}, "limit": {"-1"}},
		{"tenant_id": {"tenantA"}, "plant_id": {"Plant_1"}, "cursor": {"bad/notvalid"}},
		{"tenant_id": {"tenantA"}, "plant_id": {"Plant_1"}, "permission": {"sewing.transfer"}},
	} {
		if _, _, _, _, e = commissionPage(q); e == nil {
			t.Errorf("accepted %v", q)
		}
	}
}
func TestCommissioningStatusDoesNotCertify(t *testing.T) {
	s := commissioningStation{EnabledGateways: 1, TotalGateways: 2, AssignedWorkers: 1, EnabledInboundRoutes: 1}
	commissionClassify(&s)
	if s.Status != "CONFIGURED_NOT_PHYSICALLY_CERTIFIED" || len(s.Blockers) != 0 {
		t.Fatal(s)
	}
	s.EnabledGateways = 0
	s.AssignedWorkers = 0
	s.EnabledInboundRoutes = 0
	commissionClassify(&s)
	if s.Status != "CONFIGURATION_INCOMPLETE" || len(s.Blockers) != 3 {
		t.Fatal(s)
	}
}
func TestCommissioningFailsClosedInDev(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_COMMISSIONING_READ_ENABLED", "true")
	p := newCommissioningAPI(nil)
	for _, method := range []string{"GET", "POST"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(method, "/api/v1/commissioning/stations?tenant_id=t1&plant_id=p1", nil)
		p.report(w, r)
		if method == "GET" && (w.Code != 503 || !strings.Contains(w.Body.String(), "ERR_COMMISSIONING_DISABLED")) {
			t.Fatalf("gate: %d %s", w.Code, w.Body.String())
		}
		if method == "POST" && w.Code != 405 {
			t.Fatalf("method: %d", w.Code)
		}
	}
}
func TestCommissioningProductionIngress(t *testing.T) {
	t.Setenv("ENV", "production")
	h := configuredOperatorGate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	for _, endpoint := range []string{"/commissioning", "/static/factory_commissioning.js", "/api/v1/commissioning/stations"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", endpoint, nil))
		if w.Code != 204 {
			t.Errorf("blocked enterprise secured route %s: %d", endpoint, w.Code)
		}
	}
	for _, endpoint := range []string{"/api/v1/commissioning/create-gateway", "/api/v1/commissioning/stations/delete"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", endpoint, nil))
		if w.Code != 503 {
			t.Errorf("unexpected privilege bypass %s: %d", endpoint, w.Code)
		}
	}
}
