// Author: Sunil Jain, CyberInQ. Batch 10A/B acceptance-contract tests.
package main

import (
	"strings"
	"testing"
	"time"
)

func qcExample(action string) qcSimEventRequest {
	return qcSimEventRequest{
		EventID: "QC-12345678901234", LayID: exampleLayID, PlantID: "SIM-PLANT-01",
		Serial: "SIM-G-" + exampleLayID + "-1-1", Action: action,
		Actor: "QA-INSPECTOR-01", StationID: "QC-STATION-01",
		Reader: qcSimReader{Version: "QC_READER_V1", Source: "UI_SIMULATED", ReaderID: "UI-QC-READER-01", AntennaPort: 1, ClickedAt: time.Now().UTC().Format(time.RFC3339Nano)},
	}
}
func TestQC10ReaderProvenanceAndInputContract(t *testing.T) {
	r := qcExample("INSPECT_PASS")
	if e := qcValidRequest(r); e != nil {
		t.Fatal(e)
	}
	r.Reader.Source = "HARDWARE_SIGNED"
	if qcValidRequest(r) == nil {
		t.Fatal("physical RFID claim must be rejected")
	}
	r.Reader.Source = "UI_SIMULATED"
	tag := "AABBCCDDEEFF"
	r.Reader.EPCHex = &tag
	if qcValidRequest(r) == nil {
		t.Fatal("fabricated EPC must be rejected")
	}
	r.Reader.EPCHex = nil
	strength := -41.5
	r.Reader.RSSIDBm = &strength
	if qcValidRequest(r) == nil {
		t.Fatal("fabricated RSSI must be rejected")
	}
	r.Reader.RSSIDBm = nil
	r.Reader.Version = "WRONG_READER"
	if qcValidRequest(r) == nil {
		t.Fatal("reader contract mismatch must reject")
	}
	r.Reader.Version = "QC_READER_V1"
	r.Action = "INSPECT_FAIL"
	r.DefectCode = "BROKEN_STITCH"
	if e := qcValidRequest(r); e != nil {
		t.Fatal(e)
	}
	r.DefectCode = ""
	if qcValidRequest(r) == nil {
		t.Fatal("failed inspection requires defect")
	}
	r = qcExample("BIND_BUYER")
	r.BuyerCode = "BUYER-000001"
	r.StyleCode = "STYLE-123"
	r.SizeCode = "XL"
	if e := qcValidRequest(r); e != nil {
		t.Fatal(e)
	}
	r.BuyerCode = "bad code with space"
	if qcValidRequest(r) == nil {
		t.Fatal("buyer serial must be constrained")
	}
}
func TestQC10PartialHoldAndIndependentRepair(t *testing.T) {
	cases := []struct {
		state, action, actor, fail, repair string
		done, total                        int
		next                               string
		allowed                            bool
	}{
		{"PENDING", "INSPECT_FAIL", "QA-01", "", "", 0, 2, "HOLD", true},
		{"PENDING", "INSPECT_PASS", "QA-01", "", "", 0, 2, "ACCEPTED", true},
		{"HOLD", "REPAIR_COMPLETE", "TECH-02", "QA-01", "", 0, 2, "REINSPECT", true},
		{"HOLD", "REPAIR_COMPLETE", "QA-01", "QA-01", "", 0, 2, "", false},
		{"REINSPECT", "REINSPECT_PASS", "QA-03", "QA-01", "TECH-02", 0, 2, "ACCEPTED", true},
		{"REINSPECT", "REINSPECT_PASS", "TECH-02", "QA-01", "TECH-02", 0, 2, "", false},
		{"REINSPECT", "REINSPECT_PASS", "QA-01", "QA-01", "TECH-02", 0, 2, "", false},
		{"REINSPECT", "REINSPECT_FAIL", "QA-03", "QA-01", "TECH-02", 0, 2, "HOLD", true},
		{"HOLD", "INSPECT_PASS", "QA-04", "QA-01", "", 0, 2, "", false},
		{"PENDING", "FINISH_STEP", "QA-01", "", "", 0, 2, "", false},
		{"ACCEPTED", "FINISH_STEP", "FIN-01", "", "", 0, 2, "ACCEPTED", true},
		{"ACCEPTED", "FINISH_STEP", "FIN-01", "", "", 2, 2, "", false},
		{"ACCEPTED", "BIND_BUYER", "FIN-01", "", "", 1, 2, "", false},
		{"ACCEPTED", "BIND_BUYER", "FIN-01", "", "", 2, 2, "ACCEPTED", true},
	}
	for _, v := range cases {
		t.Run(strings.Join([]string{v.state, v.action, v.actor}, "_"), func(t *testing.T) {
			next, e := qcCheckTransition(v.state, v.action, v.actor, v.fail, v.repair, v.done, v.total)
			if (e == nil) != v.allowed || next != v.next {
				t.Fatalf("got %s %v; want %s allowed=%v", next, e, v.next, v.allowed)
			}
		})
	}
}
func TestQC10RouteCatalogIsBoundedAndImmutableSelection(t *testing.T) {
	for key, expected := range map[string]int{"WOVEN_STANDARD": 2, "WASH_FINISH": 4, "HANGER_FINISH": 3, "KNIT_FOLD": 3} {
		got, e := qcSteps(key)
		if e != nil || len(got) != expected {
			t.Fatalf("%s route %v %v", key, got, e)
		}
		got[0] = "MUTATE"
		other, _ := qcSteps(key)
		if other[0] == "MUTATE" {
			t.Fatal("route shared mutable slice")
		}
	}
	if _, e := qcSteps("ANY_ROUTE"); e == nil {
		t.Fatal("unknown route allowed")
	}
}
func TestQC10ReplayDigestBindsActorAndEvidence(t *testing.T) {
	a := qcExample("INSPECT_PASS")
	b := a
	if sewDigest(a) != sewDigest(b) {
		t.Fatal("identical command should replay")
	}
	b.Actor = "QA-INSPECTOR-02"
	if sewDigest(a) == sewDigest(b) {
		t.Fatal("actor change should conflict")
	}
	b = a
	b.Reader.ReaderID = "UI-QC-READER-99"
	if sewDigest(a) == sewDigest(b) {
		t.Fatal("reader change should conflict")
	}
}
