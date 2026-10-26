package main

import (
	"strings"
	"testing"
)

func TestDevProvisionMustBeExplicit(t *testing.T) {
	t.Setenv("AGIPAREL_DEV_PROVISION", "")
	t.Setenv("AGIPAREL_ENVIRONMENT", "development")
	if requireDev() == nil {
		t.Fatal("missing explicit development opt-in accepted")
	}
	t.Setenv("AGIPAREL_DEV_PROVISION", "I_UNDERSTAND_FAKE_DATA")
	t.Setenv("AGIPAREL_ENVIRONMENT", "production")
	if requireDev() == nil {
		t.Fatal("production provision accepted")
	}
	t.Setenv("AGIPAREL_ENVIRONMENT", "development")
	if err := requireDev(); err != nil {
		t.Fatal(err)
	}
}
func TestInvalidInputsFailBeforeConnecting(t *testing.T) {
	t.Setenv("AGIPAREL_DEV_PROVISION", "I_UNDERSTAND_FAKE_DATA")
	t.Setenv("AGIPAREL_ENVIRONMENT", "development")
	t.Setenv("AGIPAREL_ADMIN_DATABASE_URL", "postgres://nonexistent:invalid@127.0.0.1:1/none")
	for _, a := range [][]string{
		{"scope", "--tenant", "X", "--approved-by", "CEO", "--order", "PO", "--plant", "P", "--holder", "H", "--lease-duration", "48h"},
		{"scope", "--tenant", "X", "--approved-by", "CEO", "--order", "PO", "--plant", "P"},
		{"enroll-ceo", "--tenant", "X", "--approved-by", "admin"},
	} {
		if err := run(a); err == nil || !strings.Contains(err.Error(), "required") && !strings.Contains(err.Error(), "duration") {
			t.Fatalf("expected input validation for %v, got %v", a, err)
		}
	}
}
