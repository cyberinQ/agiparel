package onboarding

import (
	"context"
	"strings"
	"testing"
)

func TestLegacyProvisioningNeverRuns(t *testing.T) {
	if err := ApplyToDatabase(context.Background(), nil, &FactoryWorkbook{TenantID: "dev"}); err == nil || !strings.Contains(err.Error(), "RETIRED") {
		t.Fatalf("legacy factory write still enabled: %v", err)
	}
}
func TestInvalidLegacyFieldsAreNotInvented(t *testing.T) {
	fixtures := []string{
		"FACTORY_PROFILE,dev,Factory,Unit 1,8,20\nMACHINE_ROSTER,M-01,L,ST,M,CLASS,broken,ACTIVE\n",
		"FACTORY_PROFILE,dev,Factory,Unit 1,8,20\nWORKER_ROSTER,W-01,E,Worker,SEW,A,missing,PRESENT,L,ST\n",
		"FACTORY_PROFILE,dev,Factory,Unit 1,8,20\nSTORE_ROLLS,ROLL-01,FABRIC,missing,148,A,BIN,AVAILABLE,,0\n",
		"FACTORY_PROFILE,dev,Factory,Unit 1,8,20\nWORK_ORDERS,WO-1,PO-1,ST-1,L,W-1,600,broken,{},PROPOSED\n",
	}
	for _, csv := range fixtures {
		if _, err := ParseWorkbookCSV(strings.NewReader(csv)); err == nil {
			t.Fatalf("invalid field accepted: %q", csv)
		}
	}
}
