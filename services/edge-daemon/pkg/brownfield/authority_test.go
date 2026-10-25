// Author: Sunil Jain, 2026-10-25
// Prove brownfield default-deny behavior without touching live records.
package brownfield

import "testing"

func TestAuthorityFailClosed(t *testing.T) {
	s := Scope{TenantID: "T", PlantID: "P", Domain: "inventory", ObjectID: "roll-1"}
	for _, mode := range []Mode{Observe, Advise} {
		if CanCommit(Grant{Scope: s, Mode: mode}, s, "agiparel").Allowed {
			t.Fatalf("%s permitted a write", mode)
		}
	}
	g := Grant{Scope: s, Mode: Delegated, WriterID: "agiparel", ApprovedBy: "manager"}
	if err := ValidateGrant(g); err != nil {
		t.Fatal(err)
	}
	if !CanCommit(g, s, "agiparel").Allowed {
		t.Fatal("approved scope rejected")
	}
	if CanCommit(g, Scope{TenantID: "T", PlantID: "OTHER", Domain: "inventory", ObjectID: "roll-1"}, "agiparel").Allowed {
		t.Fatal("cross-plant write approved")
	}
	if CanCommit(g, s, "legacy").Allowed {
		t.Fatal("wrong writer approved")
	}
	if CanCommit(Grant{Scope: s, Mode: Authority, WriterID: "agiparel"}, s, "agiparel").Allowed {
		t.Fatal("missing approval permitted")
	}
}
