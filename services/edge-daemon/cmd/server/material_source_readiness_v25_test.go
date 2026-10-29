// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// Deterministic database/sql test: do not infer buyer approval from a proposed
// intake or inflate observed consumption from unverified spread reports.
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
)

type sourceTestDriver struct{ mode string }
type sourceTestConn struct{ mode string }
type sourceTestTx struct{}
type sourceTestRows struct {
	cols   []string
	values [][]driver.Value
	i      int
}

func (d sourceTestDriver) Open(string) (driver.Conn, error) { return sourceTestConn{mode: d.mode}, nil }
func (c sourceTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("no statement preparation")
}
func (c sourceTestConn) Close() error              { return nil }
func (c sourceTestConn) Begin() (driver.Tx, error) { return sourceTestTx{}, nil }
func (sourceTestTx) Commit() error                 { return nil }
func (sourceTestTx) Rollback() error               { return nil }
func (r *sourceTestRows) Columns() []string        { return r.cols }
func (r *sourceTestRows) Close() error             { return nil }
func (r *sourceTestRows) Next(dest []driver.Value) error {
	if r.i >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.i])
	r.i++
	return nil
}
func (c sourceTestConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	var cols []string
	var vals [][]driver.Value
	if len(args) < 2 || args[0].Value != "TEN-25" {
		return nil, errors.New("tenant not scoped")
	}
	switch {
	case strings.Contains(q, "FROM dynamic_work_orders w"):
		if len(args) != 3 || args[1].Value != "PLANT-25" || args[2].Value != "WO-25" || !strings.Contains(q, "w.plant_id=$2") {
			return nil, errors.New("work order not plant scoped")
		}
		cols = []string{"po", "style", "qty"}
		vals = [][]driver.Value{{"PO-25", "STYLE-25", int64(12)}}
	case strings.Contains(q, "SELECT count(*) FROM order_intakes_v5"):
		if len(args) != 3 || args[1].Value != "PO-25" || args[2].Value != "STYLE-25" {
			return nil, errors.New("source intake not order scoped")
		}
		n := int64(1)
		if c.mode == "ambiguous" {
			n = 2
		}
		if c.mode == "missing" {
			n = 0
		}
		cols = []string{"count"}
		vals = [][]driver.Value{{n}}
	case strings.Contains(q, "SELECT id,source_sha256,status,ordered_qty"):
		if len(args) != 3 || args[1].Value != "PO-25" || args[2].Value != "STYLE-25" {
			return nil, errors.New("proposal not source scoped")
		}
		qty := int64(12)
		if c.mode == "mismatch" {
			qty = 9
		}
		cols = []string{"id", "sha", "status", "qty"}
		vals = [][]driver.Value{{"intake-25", strings.Repeat("a", 64), "PROPOSED", qty}}
	case strings.Contains(q, "FROM order_intake_bom_v5"):
		if len(args) != 3 || args[1].Value != "intake-25" || args[2].Value != int64(12) {
			return nil, errors.New("BOM not source scoped")
		}
		cols = []string{"sku", "category", "uom", "per", "qty"}
		vals = [][]driver.Value{{"FAB-A", "FABRIC", "M", int64(1750), int64(21000)}, {"ZIP-A", "TRIM", "PCS", int64(1000), int64(12000)}}
	case strings.Contains(q, "FROM cut_sewing_releases_v8"):
		if len(args) != 4 || args[1].Value != "PLANT-25" || args[2].Value != "PO-25" || args[3].Value != "STYLE-25" || !strings.Contains(q, "r.plant_id=$2") {
			return nil, errors.New("releases not plant scoped")
		}
		cols = []string{"count", "consumption"}
		vals = [][]driver.Value{{int64(2), int64(30000)}}
	default:
		return nil, errors.New("unexpected SQL read: " + q)
	}
	return &sourceTestRows{cols: cols, values: vals}, nil
}

var sourceDriverOnce sync.Once

func TestF315BSourceEvidenceProposedIsNeverProductionAuthority(t *testing.T) {
	sourceDriverOnce.Do(func() {
		for _, mode := range []string{"valid", "ambiguous", "missing", "mismatch"} {
			sql.Register("f315b_"+mode, sourceTestDriver{mode})
		}
	})
	for _, tc := range []struct {
		mode, state string
		rows        int
	}{
		{"valid", "PROPOSED_BUYER_BOM_NOT_APPROVED", 2},
		{"ambiguous", "AMBIGUOUS_PROPOSED_INTAKES", 0},
		{"missing", "NO_SOURCE_INTAKE", 0},
		{"mismatch", "PROPOSED_BOM_ORDER_QUANTITY_MISMATCH", 2},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			db, err := sql.Open("f315b_"+tc.mode, "")
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			tx, err := db.BeginTx(context.Background(), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			got, err := loadSourceMaterialEvidence(context.Background(), tx, "TEN-25", "PLANT-25", "WO-25")
			if err != nil {
				t.Fatal(err)
			}
			if got.ProductionRelease || got.SourceState != tc.state || len(got.Lines) != tc.rows || got.SignedReleaseCount != 2 || got.SignedAttributedConsumptionMM != 30000 {
				t.Fatalf("incorrect authority or source: %+v", got)
			}
			if tc.mode == "valid" && (got.Lines[0].DeclaredOrderRequirementMilli != 21000 || got.Lines[1].DeclaredOrderRequirementMilli != 12000) {
				t.Fatalf("BOM units: %+v", got.Lines)
			}
		})
	}
}
