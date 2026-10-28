// Author: Sunil Jain, 2026-10-27
// Verify explicit per-field review, canonical edits and fail-closed manager approval.
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFactoryApprovalValidation(t *testing.T) {
	cases := []struct {
		name, typ, field string
		attrs            map[string]string
		ok               bool
		key              string
	}{
		{"edited machine", "MACHINE", "", map[string]string{"machine_id": "MC-4", "machine_type": "Overlock", "status_note": "Floor 2"}, true, "MC-4"},
		{"edited skill", "SKILL", "", map[string]string{"worker_id": "W-1", "skill": "Seam sewing"}, true, "W-1:Seam sewing"},
		{"typed rate", "RATE", "", map[string]string{"cost_type": "Labor", "currency": "INR", "rate": "650.00", "rate_basis": "DAY"}, true, "Labor:INR"},
		{"rate without basis", "RATE", "", map[string]string{"cost_type": "Labor", "currency": "INR", "rate": "650.00"}, false, ""},
		{"material observation", "MATERIAL", "", map[string]string{"material_code": "FAB-9", "quantity": "12.5", "unit": "METER"}, true, "FAB-9:METER"},
		{"invalid stock", "MATERIAL", "", map[string]string{"material_code": "FAB-9", "quantity": "-12", "unit": "METER"}, false, ""},
		{"field corrected", "FIELD", "machines.sewing_count", map[string]string{"value": "05"}, true, "machines.sewing_count"},
		{"invalid day", "FIELD", "calendar.workdays_week", map[string]string{"value": "9"}, false, ""},
		{"unknown type", "PAYROLL", "", map[string]string{"rate": "5"}, false, ""},
		{"unexpected master field", "MACHINE", "", map[string]string{"machine_id": "M-3", "machine_type": "Lockstitch", "approved_inventory": "999"}, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key, normalized, err := validatedApproval(tc.typ, tc.field, tc.attrs)
			if (err == nil) != tc.ok {
				t.Fatalf("expected success %v, got %q", tc.ok, err)
			}
			if tc.ok && (key != tc.key || len(normalized) == 0) {
				t.Fatalf("key/attrs invalid %q %+v", key, normalized)
			}
		})
	}
}
func TestFactoryApprovalAllFieldsRequired(t *testing.T) {
	attrs := map[string]string{"machine_id": "M-8", "machine_type": "Overlock"}
	for _, tc := range []struct {
		checks []string
		valid  bool
	}{{[]string{"machine_id", "machine_type"}, true}, {[]string{"machine_id"}, false}, {[]string{"machine_id", "machine_id"}, false}, {[]string{"machine_id", "machine_type", "extra"}, false}} {
		if got := checkedEveryField(attrs, tc.checks); got != tc.valid {
			t.Fatalf("checked %#v got %v", tc.checks, got)
		}
	}
}
func TestManagerSubmitRequiresExplicitDevIdentity(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_OPERATOR_USER", "dev")
	t.Setenv("AGIPAREL_OPERATOR_PASSWORD", "safe-dev-pass")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/factory-onboarding/approvals/submit", strings.NewReader("{}"))
	req.SetBasicAuth("dev", "safe-dev-pass")
	if _, ok := developmentReviewActor(req); ok {
		t.Fatal("missing manager configuration must fail closed")
	}
	t.Setenv("AGIPAREL_REVIEW_MANAGER_USER", "dev")
	if _, ok := developmentReviewActor(req); !ok {
		t.Fatal("explicit development manager rejected")
	}
	req.SetBasicAuth("dev", "wrong")
	if _, ok := developmentReviewActor(req); ok {
		t.Fatal("incorrect password accepted")
	}
	req.SetBasicAuth("dev", "safe-dev-pass")
	t.Setenv("ENV", "production")
	if _, ok := developmentReviewActor(req); ok {
		t.Fatal("production must fail closed without workforce identity")
	}
}
func TestUnauthorizedManagerSubmissionCannotWrite(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_REVIEW_MANAGER_USER", "")
	server := &EdgeServer{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/factory-onboarding/approvals/submit", strings.NewReader("{}"))
	result := httptest.NewRecorder()
	server.handleFactoryApprovalSubmit(result, req)
	if result.Code != http.StatusForbidden {
		t.Fatalf("status %d, expected fail closed", result.Code)
	}
}
