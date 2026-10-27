package main

import (
	"database/sql"
	"math"
	"testing"
	"time"
)

func TestIntelDistribution(t *testing.T) {
	s := intelDistribution([]float64{1, 2, 3, 4, 5}, 2)
	if s.N != 5 || s.Unknown != 2 || !s.Rankable || s.P50 == nil || *s.P50 != 3 || s.P90 == nil || math.Abs(*s.P90-4.6) > 1e-8 {
		t.Fatalf("bad distribution: %+v", s)
	}
	e := intelDistribution(nil, 3)
	if e.N != 0 || e.Unknown != 3 || e.P50 != nil || e.Rankable {
		t.Fatalf("empty distribution: %+v", e)
	}
}
func TestIntelIntervalRejectsUnknownAndShift(t *testing.T) {
	zone, _ := time.LoadLocation("Asia/Kolkata")
	a := sql.NullTime{Time: time.Date(2026, 10, 27, 9, 0, 0, 0, zone), Valid: true}
	b := sql.NullTime{Time: a.Time.Add(25 * time.Minute), Valid: true}
	n, reason := intelInterval(a, b, zone)
	if reason != "" || n != 25 {
		t.Fatalf("valid gap: %v %s", n, reason)
	}
	_, reason = intelInterval(b, a, zone)
	if reason != "REVERSED_TIMESTAMPS" {
		t.Fatal(reason)
	}
	_, reason = intelInterval(a, sql.NullTime{}, zone)
	if reason != "MISSING_ENDPOINT" {
		t.Fatal(reason)
	}
	_, reason = intelInterval(a, sql.NullTime{Time: a.Time.Add(3 * time.Hour), Valid: true}, zone)
	if reason != "LONG_GAP_NEEDS_SHIFT_CALENDAR" {
		t.Fatal(reason)
	}
	midnight := sql.NullTime{Time: time.Date(2026, 10, 27, 23, 59, 0, 0, zone), Valid: true}
	_, reason = intelInterval(midnight, sql.NullTime{Time: midnight.Time.Add(2 * time.Minute), Valid: true}, zone)
	if reason != "CROSSES_FACTORY_DAY" {
		t.Fatal(reason)
	}
}
func TestIntelResidualNoScrapInference(t *testing.T) {
	if r := intelResidualFor("a", "b", 8, 9); r.Status != "NEGATIVE_INVARIANT" || r.Amount != -1 {
		t.Fatalf("%+v", r)
	}
	if r := intelResidualFor("a", "b", 9, 8); r.Status != "OPEN_WIP_OR_UNKNOWN_LOSS" || r.Amount != 1 {
		t.Fatalf("%+v", r)
	}
	if r := intelResidualFor("a", "b", 9, 9); r.Status != "BALANCED" {
		t.Fatalf("%+v", r)
	}
}
func TestIntelInputs(t *testing.T) {
	const lay = "d3c21b9d-6e78-44c9-9902-c3c4418a44a1"
	if e := intelInputs(lay, "Asia/Kolkata"); e != nil {
		t.Fatal(e)
	}
	for _, tz := range []string{"", "../../etc/passwd", "Mars/Phobos"} {
		if intelInputs(lay, tz) == nil {
			t.Fatal("accepted invalid timezone", tz)
		}
	}
	if intelInputs("bogus", "UTC") == nil {
		t.Fatal("accepted invalid lay")
	}
}
