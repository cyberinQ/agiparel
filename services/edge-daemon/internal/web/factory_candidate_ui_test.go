package web

import (
 "strings"
 "testing"
)
func TestFactoryCandidateTriageIsFindableAndReviewOnly(t *testing.T){
 html,err:=staticFS.ReadFile("static/factory_setup.html");if err!=nil{t.Fatal(err)}
 body:=string(html)
 for _,needle:=range []string{"factory_candidates.js","candidate-source","candidate-items","register-items","/documents","/static/workspace_nav.js"}{
  if !strings.Contains(body,needle){t.Errorf("factory setup missing %q",needle)}
 }
 js,err:=staticFS.ReadFile("static/factory_candidates.js");if err!=nil{t.Fatal(err)}
 code:=string(js)
 for _,needle:=range []string{"/api/v1/factory-onboarding/candidates/scan","/api/v1/factory-onboarding/candidates/review","ACCEPT_PROVISIONAL","/documents?document=","createElement","textContent"}{
  if !strings.Contains(code,needle){t.Errorf("review UI missing %q",needle)}
 }
 for _,unsafe:=range []string{"innerHTML","/api/v1/stores/issue","/api/v1/material/issue"}{
  if strings.Contains(code,unsafe){t.Errorf("review UI includes unsafe path %q",unsafe)}
 }
 doc,err:=staticFS.ReadFile("static/documents.js");if err!=nil{t.Fatal(err)}
 if !strings.Contains(string(doc),"source-segment-"){t.Fatal("missing exact evidence deep links")}
}
