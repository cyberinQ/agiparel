// Author: Sunil Jain, 2026-10-28.
// Test real aggregator over deterministic database/sql rows, including empty
// reporting periods, tenant+plant parameters and independently missing KPIs.
package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

type factoryTestDriver struct{}
type factoryTestConn struct{}
type factoryTestTx struct{}
type factoryTestRows struct {
	cols  []string
	data  [][]driver.Value
	index int
}

func (factoryTestDriver) Open(string) (driver.Conn, error) { return factoryTestConn{}, nil }
func (factoryTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("no prepared statements")
}
func (factoryTestConn) Close() error              { return nil }
func (factoryTestConn) Begin() (driver.Tx, error) { return factoryTestTx{}, nil }
func (factoryTestTx) Commit() error               { return nil }
func (factoryTestTx) Rollback() error             { return nil }
func (r *factoryTestRows) Columns() []string      { return r.cols }
func (r *factoryTestRows) Close() error           { return nil }
func (r *factoryTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.data) {
		return io.EOF
	}
	copy(dest, r.data[r.index])
	r.index++
	return nil
}
func (factoryTestConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if len(args) != 4 || args[0].Value != "tenant_A" || args[1].Value != "plant_A" ||
		!strings.Contains(query, "tenant_id=$1") || !strings.Contains(query, "plant_id=$2") ||
		!strings.Contains(query, "$3") || !strings.Contains(query, "$4") {
		return nil, errors.New("missing bounded tenant/plant/time scope")
	}
	if _, ok := args[2].Value.(time.Time); !ok {
		return nil, errors.New("missing start time")
	}
	if _, ok := args[3].Value.(time.Time); !ok {
		return nil, errors.New("missing end time")
	}
	var vals [][]driver.Value
	var cols []string
	switch {
	case strings.Contains(query, "FROM cut_part_outcomes_v19"):
		cols = []string{"accepted", "rejected"}
		vals = [][]driver.Value{{int64(8), int64(2)}}
	case strings.Contains(query, "FROM qc_garment_events_v21"):
		cols = []string{"passed", "held", "released"}
		vals = [][]driver.Value{{int64(4), int64(1), int64(3)}}
	case strings.Contains(query, "FROM cut_consumption_reconciliations_v19"):
		cols = []string{"used", "trim", "damaged"}
		vals = [][]driver.Value{{int64(9000), int64(1000), int64(2000)}}
	case strings.Contains(query, "FROM wip_movements_v20") && strings.Contains(query, "JOIN wip_transfer_routes_v20"):
		cols = []string{"id", "from", "to", "count", "p50", "p95"}
		vals = [][]driver.Value{{"route123", "SEW-1", "FINISH-1", int64(3), float64(10), float64(30)}}
	case strings.Contains(query, "FROM (SELECT extract(epoch"):
		cols = []string{"count", "p50", "p95"}
		vals = [][]driver.Value{{int64(3), float64(10), float64(30)}}
	case strings.Contains(query, "FROM pack_carton_events_v22"):
		cols = []string{"added", "sealed"}
		vals = [][]driver.Value{{int64(3), int64(1)}}
	case strings.Contains(query, "FROM pack_manifest_events_v22"):
		cols = []string{"staged"}
		vals = [][]driver.Value{{int64(0)}}
	default:
		return nil, errors.New("unexpected SQL query")
	}
	return &factoryTestRows{cols: cols, data: vals}, nil
}

var factoryDriverOnce sync.Once

func TestF36FactoryEvidenceSourceQueries(t *testing.T) {
	factoryDriverOnce.Do(func() { sql.Register("factorytest36", factoryTestDriver{}) })
	db, e := sql.Open("factorytest36", "")
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	tx, e := db.BeginTx(context.Background(), nil)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	r, e := loadFactoryEvidence(context.Background(), tx, "tenant_A", "plant_A", time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC))
	if e != nil {
		t.Fatal(e)
	}
	if r.ProductionRelease || r.Authority != factoryEvidenceAuthority || len(r.Metrics) != 18 || len(r.SlowestRoutes) != 1 {
		t.Fatalf("invalid report: %+v", r)
	}
	metrics := map[string]factoryEvidenceMetric{}
	for _, m := range r.Metrics {
		metrics[m.ID] = m
	}
	for id, want := range map[string]int64{"cut_parts_accepted": 8, "cut_parts_rejected": 2, "cut_part_acceptance_pct": 8000,
		"garment_first_pass_yield_pct": 8000, "garments_quality_released": 3, "classified_fabric_use_pct": 7500,
		"wip_completed_handoffs": 3, "wip_handoff_p95_ms": 30000, "cartons_sealed": 1, "manifests_staged": 0} {
		v := metrics[id].Value
		if v == nil || *v != want {
			t.Errorf("%s: got %+v expected %d", id, metrics[id], want)
		}
	}
	for _, id := range []string{"oee_pct", "actual_fabric_utilization_pct", "labor_efficiency_pct", "physically_dispatched_garments"} {
		if metrics[id].Value != nil || metrics[id].Status != "NOT_MEASURED" {
			t.Errorf("fabricated metric %s: %+v", id, metrics[id])
		}
	}
	if r.SlowestRoutes[0].P95Milliseconds != 30000 {
		t.Fatalf("route report: %+v", r.SlowestRoutes)
	}
}
