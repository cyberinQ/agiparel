package web

import (
 "strings"
 "testing"
)

func TestBuyerTechPackReviewRequiresHumanConsumptionAndEvidence(t *testing.T) {
 page, err := GetBuyerReviewHTML()
 if err != nil {t.Fatal(err)}
 for _, text := range []string{`id="techpack-workbench"`, `id="techpack-candidates"`, `0.000`, `a verified rate and unit`, `/documents`, `PROPOSED · HOLD`} {
   if !strings.Contains(string(page),text) {t.Errorf("missing buyer review safety disclosure or control %q",text)}
 }
 script,err:=staticFS.ReadFile("static/buyer_review.js")
 if err != nil {t.Fatal(err)}
 for _,text:=range []string{`techpack_material_hints`, `renderTechPackHints`, `per_garment:''`, `uom:''`, `source_ordinal:hint.source_ordinal`, `textContent`} {
  if !strings.Contains(string(script),text){t.Errorf("missing BOM suggestion contract %q",text)}
 }
}
