// Author: Sunil Jain, CyberinQ LLC, 2026-10-28.
package main

import (
 "net/http"
 "net/http/httptest"
 "net/url"
 "strings"
 "testing"
)

func TestF39OperatorStationQueryStrict(t *testing.T) {
 q:=url.Values{"tenant_id":{"tenantA"},"plant_id":{"Plant_1"},"station_id":{"LINE_02"},"limit":{"30"}}
 tenant,plant,station,scan,limit,e:=operatorStationQuery(q)
 if e!=nil || tenant!="tenantA" || plant!="Plant_1" || station!="LINE_02" || scan!="" || limit!=30 {t.Fatalf("valid query %s %s %s %s %d %v",tenant,plant,station,scan,limit,e)}
 validID:="c6e63b80-e150-4364-90be-759755e3ec46";q.Set("scan_id",validID)
 _,_,_,scan,_,e=operatorStationQuery(q);if e!=nil || scan!=validID {t.Fatal("valid scan rejected",e)}
 for _,bad:=range []url.Values{
  {"tenant_id":{"t1"},"plant_id":{"p1"}},
  {"tenant_id":{"t1"},"plant_id":{"p1"},"station_id":{"s1"},"scan_id":{"not-uuid"}},
  {"tenant_id":{"t1","t2"},"plant_id":{"p1"},"station_id":{"s1"}},
  {"tenant_id":{"t1"},"plant_id":{"p1"},"station_id":{"s1"},"limit":{"51"}},
  {"tenant_id":{"t1"},"plant_id":{"p1"},"station_id":{"s1"},"limit":{"0"}},
  {"tenant_id":{"t1"},"plant_id":{"p1"},"station_id":{"s1"},"admin":{"true"}},
 } {if _,_,_,_,_,err:=operatorStationQuery(bad);err==nil {t.Errorf("accepted invalid query: %v",bad)}}
}
func TestF39FailClosedDevelopment(t *testing.T) {
 t.Setenv("ENV","development");t.Setenv("AGIPAREL_OPERATOR_STATION_ENABLED","true")
 p:=newOperatorStationAPI(nil)
 for _,path:=range []string{"/api/v1/operator-station/inbox?tenant_id=t1&plant_id=p1&station_id=s1","/operator-station"} {
  w:=httptest.NewRecorder();p.read(w,httptest.NewRequest("GET",path,nil))
  if w.Code!=503 || !strings.Contains(w.Body.String(),"ERR_OPERATOR_STATION_DISABLED") {t.Fatal(w.Code,w.Body.String())}
 }
 w:=httptest.NewRecorder();p.read(w,httptest.NewRequest("POST","/api/v1/operator-station/inbox",nil));if w.Code!=405 {t.Fatal(w.Code)}
}
func TestF39ProductionIngressExplicitOnly(t *testing.T) {
 t.Setenv("ENV","production")
 h:=configuredOperatorGate(http.HandlerFunc(func(w http.ResponseWriter,_ *http.Request){w.WriteHeader(204)}))
 for _,path:=range []string{"/operator-station","/static/factory_operator_station.js","/api/v1/operator-station/inbox"} {
  w:=httptest.NewRecorder();h.ServeHTTP(w,httptest.NewRequest("GET",path,nil));if w.Code!=204{t.Errorf("blocked authorized route %s: %d",path,w.Code)}
 }
 for _,path:=range []string{"/api/v1/operator-station/add-scan","/api/v1/operator-station/force-transfer"} {
  w:=httptest.NewRecorder();h.ServeHTTP(w,httptest.NewRequest("GET",path,nil));if w.Code==204 {t.Errorf("overbroad ingress %s",path)}
 }
}
