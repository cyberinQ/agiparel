// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
package onboarding

import (
	"strings"
	"testing"
)

func TestF315ParseFactoryBOMRows(t *testing.T) {
	raw := GenerateTemplateCSV() + "\nSTYLE_BOM,KK524867,DENIM-STRETCH-12OZ,FABRIC,1.875,M,MARKER-REF-17\n"
	wb, err := ParseWorkbookCSV(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(wb.BOMRows) != 1 || wb.BOMRows[0].QtyPerGarment != 1.875 || wb.BOMRows[0].SourceRef != "MARKER-REF-17" {
		t.Fatalf("failed parse %+v", wb.BOMRows)
	}
}
func TestF315RejectsUnboundedBOM(t *testing.T) {
	for _, row := range []string{"STYLE_BOM,ST,SKU,FABRIC,NaN,M", "STYLE_BOM,ST,SKU,FABRIC,Inf,M", "STYLE_BOM,ST,SKU,FABRIC,-1,M", "STYLE_BOM,ST,SKU,OTHER,1,M", "STYLE_BOM,,SKU,FABRIC,1,M"} {
		_, err := ParseWorkbookCSV(strings.NewReader("FACTORY_PROFILE,TENANT,Test,Plant,8,20\n" + row + "\n"))
		if err == nil {
			t.Fatalf("unsafe input accepted %s", row)
		}
	}
}
