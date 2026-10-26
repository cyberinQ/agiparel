// Author: Sunil Jain, 2026-10-25
// Ensure the machine audit flags ambiguity without erasing legitimate shared-station equipment.
package masterdata

import "testing"

func TestAuditEmpty(t *testing.T) {
	r := AuditMachineRoster(nil)
	if r.ReviewRequired || r.Rows != 0 {
		t.Fatalf("unexpected findings: %+v", r)
	}
}
func TestAuditSharedStationIsReviewOnly(t *testing.T) {
	r := AuditMachineRoster([]Machine{{ID: "m1", Line: "14", Station: "ST-02", Name: "Juki", Class: "SNLS", CapacityUPH: 100}, {ID: "m2", Line: "14", Station: "ST-02", Name: "Brother", Class: "SNLS", CapacityUPH: 120}})
	if !r.ReviewRequired || r.UniqueRecordIDs != 2 {
		t.Fatalf("unexpected result: %+v", r)
	}
	for _, f := range r.Findings {
		if f.Severity == "BLOCK" {
			t.Fatalf("shared station is not proof of duplicate: %+v", f)
		}
	}
}
func TestAuditDuplicateAndIncomplete(t *testing.T) {
	r := AuditMachineRoster([]Machine{{ID: "A", Line: "L", Station: "S", Name: "Overlock", Class: "OL", CapacityUPH: 100}, {ID: "A", Line: "L", Station: "S", Name: "Overlock", Class: "OL", CapacityUPH: 0}})
	hasID, hasCapacity := false, false
	for _, f := range r.Findings {
		if f.Code == "DUPLICATE_RECORD_ID" {
			hasID = true
		}
		if f.Code == "INVALID_RATED_CAPACITY" {
			hasCapacity = true
		}
	}
	if !hasID || !hasCapacity {
		t.Fatalf("expected both findings: %+v", r)
	}
}
func TestAuditSameNameDifferentStationsNotSuspect(t *testing.T) {
	r := AuditMachineRoster([]Machine{{ID: "A", Line: "L", Station: "S1", Name: "Juki", Class: "SNLS", CapacityUPH: 100}, {ID: "B", Line: "L", Station: "S2", Name: "Juki", Class: "SNLS", CapacityUPH: 100}})
	if r.ReviewRequired {
		t.Fatalf("same model at distinct stations is normal: %+v", r)
	}
}
