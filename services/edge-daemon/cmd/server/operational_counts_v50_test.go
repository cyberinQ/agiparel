// Author: Sunil Jain, 2026-11-12.
// Test two live factory counts with a read-only, tenant-scoped SQL driver; no LLM required.
package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type v50Driver struct{}
type v50Conn struct{}
type v50Tx struct{}
type v50Rows struct {
	columns []string
	data    [][]driver.Value
	pos     int
}

func (v50Driver) Open(name string) (driver.Conn, error) { return v50Conn{}, nil }
func (v50Conn) Prepare(q string) (driver.Stmt, error)   { return nil, driver.ErrSkip }
func (v50Conn) Close() error                            { return nil }
func (v50Conn) Begin() (driver.Tx, error)               { return v50Tx{}, nil }
func (v50Conn) BeginTx(_ context.Context, opt driver.TxOptions) (driver.Tx, error) {
	if !opt.ReadOnly {
		return nil, driver.ErrBadConn
	}
	return v50Tx{}, nil
}
func (v50Tx) Commit() error   { return nil }
func (v50Tx) Rollback() error { return nil }
func (v50Conn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	if !strings.Contains(q, "set_config('app.current_tenant_id'") || len(args) != 1 || args[0].Value != "dev" {
		return nil, driver.ErrBadConn
	}
	return driver.RowsAffected(1), nil
}
func (v50Conn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	if len(args) == 0 || args[0].Value != "dev" {
		return nil, driver.ErrBadConn
	}
	switch {
	case strings.Contains(q, "FROM attendance_register"):
		if len(args) != 2 || args[1].Value != "Asia/Kolkata" {
			return nil, driver.ErrBadConn
		}
		return &v50Rows{columns: []string{"date", "records", "present", "late", "absent", "leave"}, data: [][]driver.Value{{time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC), int64(5), int64(2), int64(1), int64(1), int64(1)}}}, nil
	case strings.Contains(q, "FROM machine_roster") && strings.Contains(q, "COUNT(DISTINCT"):
		return &v50Rows{columns: []string{"total", "active", "lines"}, data: [][]driver.Value{{int64(7), int64(4), int64(2)}}}, nil
	case strings.Contains(q, "FROM machine_roster") && strings.Contains(q, "COUNT(*)"):
		return &v50Rows{columns: []string{"total", "active"}, data: [][]driver.Value{{int64(7), int64(4)}}}, nil
	case strings.Contains(q, "FROM machine_roster") && strings.Contains(q, "SELECT id, name"):
		return &v50Rows{columns: []string{"id", "name", "line", "station", "status"}, data: [][]driver.Value{
			{"M-01", "Juki lockstitch", "L1", "S1", "ACTIVE"},
			{"M-02", "Overlock", "L1", "S2", "MAINTENANCE"},
		}}, nil
	case strings.Contains(q, "FROM store_rolls"):
		return &v50Rows{columns: []string{"total", "eligible", "reserved", "hold"}, data: [][]driver.Value{{int64(8), int64(4), int64(2), int64(2)}}}, nil
	default:
		return nil, driver.ErrBadConn
	}
}
func (r *v50Rows) Columns() []string { return r.columns }
func (r *v50Rows) Close() error      { return nil }
func (r *v50Rows) Next(dest []driver.Value) error {
	if r.pos >= len(r.data) {
		return io.EOF
	}
	copy(dest, r.data[r.pos])
	r.pos++
	return nil
}
func TestAggieCombinedCountsFromReadOnlyFactorySource(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_OPERATOR_TENANT", "dev")
	t.Setenv("AGIPAREL_FACTORY_TIMEZONE", "Asia/Kolkata")
	var once sync.Once
	once.Do(func() { sql.Register("agiparel_v50_fake", v50Driver{}) })
	db, e := sql.Open("agiparel_v50_fake", "")
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	server := &EdgeServer{db: db}
	body := `{"messages":[{"role":"user","content":"how many workers are in the factory today? how many rolls in inventory?"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agie/chat", strings.NewReader(body))
	w := httptest.NewRecorder()
	server.handleAgieChat(w, req)
	if w.Code != 200 {
		t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
	}
	var data map[string]string
	if e = json.Unmarshal(w.Body.Bytes(), &data); e != nil {
		t.Fatal(e)
	}
	if data["source"] != "live_postgresql" {
		t.Fatalf("wrong provenance: %v", data)
	}
	for _, part := range []string{"3 workers marked present or late", "8 rolls recorded", "4 have passed inspection", "2 are reserved"} {
		if !strings.Contains(data["reply"], part) {
			t.Errorf("missing %q in %q", part, data["reply"])
		}
	}
}

// Author: Sunil Jain, 2026-11-12.
// Verify the actual chat handler never reuses a previous roll query for a machine question.
func TestAggieMachineStatusAndMultiTurnIsolationV501(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_OPERATOR_TENANT", "dev")
	t.Setenv("AGIPAREL_FACTORY_TIMEZONE", "Asia/Kolkata")
	sql.Register("agiparel_v501_machine_fake", v50Driver{})
	db, err := sql.Open("agiparel_v501_machine_fake", "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := &EdgeServer{db: db}
	ask := func(history []map[string]string) string {
		t.Helper()
		input, err := json.Marshal(map[string]interface{}{"messages": history})
		if err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		server.handleAgieChat(rec, httptest.NewRequest(http.MethodPost,
			"/api/v1/agie/chat", strings.NewReader(string(input))))
		if rec.Code != http.StatusOK {
			t.Fatalf("HTTP %d: %s", rec.Code, rec.Body.String())
		}
		var result map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result["source"] != "live_postgresql" {
			t.Fatalf("not grounded: %v", result)
		}
		return result["reply"]
	}
	history := []map[string]string{{"role": "user", "content": "How many rolls in inventory?"}}
	rolls := ask(history)
	if !strings.Contains(rolls, "8 rolls recorded") {
		t.Fatal(rolls)
	}
	history = append(history,
		map[string]string{"role": "assistant", "content": rolls},
		map[string]string{"role": "user", "content": "How many machines?"})
	machines := ask(history)
	if !strings.Contains(machines, "7 machines recorded") || strings.Contains(strings.ToLower(machines), "roll") {
		t.Fatalf("cross-entity answer: %q", machines)
	}
	history = append(history,
		map[string]string{"role": "assistant", "content": machines},
		map[string]string{"role": "user", "content": "Which machines are open?"})
	statuses := ask(history)
	if !strings.Contains(statuses, "Juki lockstitch") ||
		!strings.Contains(statuses, "not verified live availability") ||
		strings.Contains(strings.ToLower(statuses), "roll") {
		t.Fatalf("machine statuses missing or contaminated: %q", statuses)
	}
	combined := ask([]map[string]string{{"role": "user", "content": "How many machines and rolls?"}})
	if !strings.Contains(combined, "7 machines recorded") || !strings.Contains(combined, "8 rolls recorded") {
		t.Fatalf("combined machine and roll entities lost: %q", combined)
	}
}
