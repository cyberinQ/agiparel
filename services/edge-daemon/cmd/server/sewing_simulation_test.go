package main

import (
	"strings"
	"testing"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/sewing"
)

const exampleLayID = "12345678-1234-1234-1234-123456789abc"

func TestSew09ValidSimulatedReaderOnly(t *testing.T) {
	v := sewScanInput{EventID: "SEW-123456789ab", LayID: exampleLayID, PlantID: "SIM-PLANT-01", LabelCode: "CB08-AAAAAAAAAAAAAAAAAAAAAAAA", StationID: "SEW-ST04", WorkerID: "SIM-OP01"}
	v.Reader.Version = "SEW_READER_V1"
	v.Reader.Source = "UI_SIMULATED"
	v.Reader.ReaderID = "UI-READER-01"
	v.Reader.ClickedAt = time.Now().UTC().Format(time.RFC3339Nano)
	v.Reader.AntennaPort = 1
	if e := validSewScan(v); e != nil {
		t.Fatal(e)
	}
	v.Reader.Source = "GATEWAY_SIGNED"
	if validSewScan(v) == nil {
		t.Fatal("rehearsal endpoint must reject hardware claim")
	}
	v.Reader.Source = "UI_SIMULATED"
	epc := "AABBCCDDEEFF001122334455"
	v.Reader.EPCHex = &epc
	if validSewScan(v) == nil {
		t.Fatal("simulated scan cannot claim real observed EPC")
	}
	v.Reader.EPCHex = nil
	rssi := -60.0
	v.Reader.RSSIDBm = &rssi
	if validSewScan(v) == nil {
		t.Fatal("simulated scan cannot claim a measured RSSI")
	}
}
func TestSew09GroupConservationAndErrors(t *testing.T) {
	parts := []sewGroupPart{{part: "FRONT", shade: "BLUE", size: "M", start: 1, end: 12, mult: 2}, {part: "BACK", shade: "BLUE", size: "M", start: 1, end: 12, mult: 1}}
	ids, e := sewCheckGroup(sewing.BundleToSingle, parts, []string{"FRONT", "BACK"}, exampleLayID, 1, 1)
	if e != nil || len(ids) != 12 {
		t.Fatalf("12 garments expected %v %v", ids, e)
	}
	if ids[0] == ids[11] || !strings.Contains(ids[0], exampleLayID) {
		t.Fatal("serial lineage must be unique and source-derived")
	}
	cases := []struct {
		name     string
		parts    []sewGroupPart
		required []string
		mode     sewing.ExecutionMode
	}{
		{"missing", parts[:1], []string{"FRONT", "BACK"}, sewing.BundleToSingle},
		{"wrong shade", []sewGroupPart{parts[0], {part: "BACK", shade: "RED", size: "M", start: 1, end: 12, mult: 1}}, []string{"FRONT", "BACK"}, sewing.BundleToSingle},
		{"wrong size", []sewGroupPart{parts[0], {part: "BACK", shade: "BLUE", size: "L", start: 1, end: 12, mult: 1}}, []string{"FRONT", "BACK"}, sewing.BundleToSingle},
		{"wrong ply", []sewGroupPart{parts[0], {part: "BACK", shade: "BLUE", size: "M", start: 2, end: 12, mult: 1}}, []string{"FRONT", "BACK"}, sewing.BundleToSingle},
		{"duplicate part", []sewGroupPart{parts[0], parts[0]}, []string{"FRONT", "BACK"}, sewing.BundleToSingle},
		{"single invalid", parts[:1], []string{"FRONT"}, sewing.SingleToSingle},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, e := sewCheckGroup(tc.mode, tc.parts, tc.required, exampleLayID, 1, 1); e == nil {
				t.Fatal("should fail closed")
			}
		})
	}
}
func TestSew09OperationContractValidation(t *testing.T) {
	base := sewOpRequest{LayID: exampleLayID, PlantID: "SIM-PLANT-01", OperationCode: "JOIN_PANEL", Mode: sewing.BundleToSingle, RequiredParts: []string{"FRONT", "BACK"}, MachineClass: "LOCKSTITCH", SAMSeconds: 36}
	if e := validSewOp(base); e != nil {
		t.Fatal(e)
	}
	base.RequiredParts = []string{"FRONT", "FRONT"}
	if validSewOp(base) == nil {
		t.Fatal("duplicate part accepted")
	}
	base.RequiredParts = []string{"FRONT", "BACK"}
	base.Predecessors = []string{"JOIN_PANEL"}
	if validSewOp(base) == nil {
		t.Fatal("self prerequisite accepted")
	}
}
func TestSew09CommandValidationAndReplayHash(t *testing.T) {
	c := sewSimCommand{CommandID: "SEW-123456789012", Action: "CLAIM", LayID: exampleLayID, PlantID: "SIM-PLANT-01", OperationID: exampleLayID, MarkerPosition: 1, PlyGroup: 1, StationID: "SEW-ST04", WorkerID: "SIM-OP01", MachineClass: "LOCKSTITCH"}
	if e := validSewCommand(c); e != nil {
		t.Fatal(e)
	}
	copy := c
	if sewDigest(c) != sewDigest(copy) {
		t.Fatal("stable replay hash")
	}
	copy.WorkerID = "SIM-OP02"
	if sewDigest(c) == sewDigest(copy) {
		t.Fatal("different actor with reused command id must conflict")
	}
	c.Action = "COMPLETE"
	if validSewCommand(c) == nil {
		t.Fatal("complete must reference an existing cycle")
	}
	c.OperationID = ""
	c.LayID = ""
	c.MarkerPosition = 0
	c.PlyGroup = 0
	c.MachineClass = ""
	c.CycleID = exampleLayID
	if e := validSewCommand(c); e != nil {
		t.Fatal(e)
	}
	c.Action = "ISSUE"
	if validSewCommand(c) == nil {
		t.Fatal("issue must have detail")
	}
}
func TestSew09OvernightCannotEarnContinuousTime(t *testing.T) {
	start := time.Date(2026, 10, 26, 23, 55, 0, 0, time.UTC)
	if _, e := sewElapsed(start, start.Add(8*time.Hour)); e == nil {
		t.Fatal("8 hour overnight continuous run accepted")
	}
	if ms, e := sewElapsed(start, start.Add(2*time.Minute)); e != nil || ms != 120000 {
		t.Fatalf("active interval %d %v", ms, e)
	}
	if _, e := sewElapsed(start, start.Add(-time.Second)); e == nil {
		t.Fatal("clock reversal accepted")
	}
}
