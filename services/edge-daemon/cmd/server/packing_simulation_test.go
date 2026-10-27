package main

import (
	"testing"
	"time"
)

func packFixture(action string) packSimRequest {
	return packSimRequest{EventID: "PACK-0123456789AB", Action: action, Carton: "CARTON_001", Manifest: "MANIFEST_001", LayID: "11111111-1111-4111-8111-111111111111", Plant: "PLANT_01", Serial: "SIM-G-123", Actor: "OPERATOR_01", Station: "STATION_01", Reader: qcSimReader{Version: "PACK_READER_V1", Source: "UI_SIMULATED", ReaderID: "UI_READER_01", AntennaPort: 1, ClickedAt: time.Now().UTC().Format(time.RFC3339Nano)}}
}
func TestPackSimulationPayloadRules(t *testing.T) {
	cases := []struct {
		action string
		adjust func(*packSimRequest)
	}{
		{"OPEN_CARTON", func(v *packSimRequest) { v.Manifest = ""; v.Serial = "" }},
		{"ADD_GARMENT", func(v *packSimRequest) { v.Manifest = ""; v.LayID = ""; v.Plant = "" }},
		{"REMOVE_GARMENT", func(v *packSimRequest) { v.Manifest = ""; v.LayID = ""; v.Plant = "" }},
		{"SEAL_CARTON", func(v *packSimRequest) { v.Manifest = ""; v.Serial = ""; v.LayID = ""; v.Plant = "" }},
		{"OPEN_MANIFEST", func(v *packSimRequest) { v.Carton = ""; v.Serial = ""; v.LayID = "" }},
		{"LOAD_CARTON", func(v *packSimRequest) { v.Serial = ""; v.LayID = ""; v.Plant = "" }},
		{"DISPATCH_REHEARSAL", func(v *packSimRequest) { v.Carton = ""; v.Serial = ""; v.LayID = ""; v.Plant = "" }},
	}
	for _, c := range cases {
		t.Run(c.action, func(t *testing.T) {
			v := packFixture(c.action)
			c.adjust(&v)
			if err := packValid(v); err != nil {
				t.Fatalf("valid action rejected: %v", err)
			}
			v.Reader.EPCHex = new(string)
			if err := packValid(v); err == nil {
				t.Fatal("fabricated physical EPC accepted")
			}
		})
	}
}
func TestPackRejectUnknownAndReaderSpoof(t *testing.T) {
	v := packFixture("OPEN_CARTON")
	v.Manifest = ""
	v.Serial = ""
	v.Action = "DISPATCH"
	if packValid(v) == nil {
		t.Fatal("unknown action accepted")
	}
	v.Action = "OPEN_CARTON"
	v.Reader.Source = "HARDWARE"
	if packValid(v) == nil {
		t.Fatal("hardware spoof accepted")
	}
	v.Reader.Source = "UI_SIMULATED"
	v.Reader.RSSIDBm = new(float64)
	if packValid(v) == nil {
		t.Fatal("RSSI spoof accepted")
	}
}
