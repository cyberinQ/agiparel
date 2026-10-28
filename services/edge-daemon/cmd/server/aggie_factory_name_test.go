// Author: Sunil Jain, 2026-10-28
// Regression: tenant-specific factory name and non-hallucinated, actionable chat failures.
package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cyberinq/agiparel/services/edge-daemon/internal/agie/interviewer"
)

type agieNameTestDriver struct{ data *agieNameTestData }
type agieNameTestData struct {
	name, state, tenant string
	absent              bool
	broken              bool
}
type agieNameTestConn struct{ data *agieNameTestData }
type agieNameTestTx struct{}
type agieNameTestRows struct {
	data *agieNameTestData
	done bool
}

func (d agieNameTestDriver) Open(string) (driver.Conn, error) {
	return &agieNameTestConn{data: d.data}, nil
}
func (c *agieNameTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (c *agieNameTestConn) Close() error              { return nil }
func (c *agieNameTestConn) Begin() (driver.Tx, error) { return agieNameTestTx{}, nil }
func (c *agieNameTestConn) BeginTx(_ context.Context, _ driver.TxOptions) (driver.Tx, error) {
	return agieNameTestTx{}, nil
}
func (agieNameTestTx) Commit() error   { return nil }
func (agieNameTestTx) Rollback() error { return nil }
func (c *agieNameTestConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	if !strings.Contains(q, "set_config('app.current_tenant_id'") || len(args) != 1 {
		return nil, errors.New("missing tenant isolation")
	}
	c.data.tenant = fmt.Sprint(args[0].Value)
	return driver.RowsAffected(1), nil
}
func (c *agieNameTestConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	if !strings.Contains(q, "factory_fact_events_v65b") || !strings.Contains(q, "field_key='factory.name'") || len(args) != 1 || fmt.Sprint(args[0].Value) != c.data.tenant {
		return nil, errors.New("query must be tenant-scoped and query only factory name")
	}
	if c.data.broken {
		return nil, errors.New("record store offline")
	}
	return &agieNameTestRows{data: c.data}, nil
}
func (r *agieNameTestRows) Columns() []string { return []string{"value_text", "review_state"} }
func (r *agieNameTestRows) Close() error      { return nil }
func (r *agieNameTestRows) Next(dst []driver.Value) error {
	if r.done || r.data.absent {
		return io.EOF
	}
	r.done = true
	dst[0] = r.data.name
	dst[1] = r.data.state
	return nil
}

func TestAggieFactoryNameFromLatestTenantFact(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_OPERATOR_TENANT", "dev")
	for i, tt := range []struct {
		name, state    string
		absent, broken bool
		contains       string
		wantErr        bool
	}{
		{"Agiparel Unit 09", "CONFIRMED", false, false, `"Agiparel Unit 09"`, false},
		{"Agiparel Unit 09", "PROVISIONAL", false, false, "still needs checking", false},
		{"", "", true, false, "do not have your factory name", false},
		{"", "", false, true, "", true},
	} {
		data := &agieNameTestData{name: tt.name, state: tt.state, absent: tt.absent, broken: tt.broken}
		id := fmt.Sprintf("agie-factory-name-test-%d", i)
		sql.Register(id, agieNameTestDriver{data: data})
		db, err := sql.Open(id, "")
		if err != nil {
			t.Fatal(err)
		}
		got, err := groundedOperationalReply(context.Background(), db, "factory_name")
		db.Close()
		if (err != nil) != tt.wantErr {
			t.Errorf("case %d unexpected error %v", i, err)
		}
		if !tt.wantErr && !strings.Contains(got, tt.contains) {
			t.Errorf("case %d: got %q, expected %q", i, got, tt.contains)
		}
		if data.tenant != "dev" {
			t.Errorf("case %d: wrong tenant %q", i, data.tenant)
		}
	}
}

func TestAggieFactoryNameIntent(t *testing.T) {
	for _, question := range []string{"what's my factory name", "What is the name of our factory?", "What is my factory called?"} {
		if got := operationalIntent([]interviewer.LLMMessage{{Role: "user", Content: question}}); got != "factory_name" {
			t.Errorf("%q: got %q", question, got)
		}
	}
	if got := operationalIntent([]interviewer.LLMMessage{{Role: "user", Content: "what is the name of the fabric?"}}); got != "" {
		t.Errorf("should not intercept unrelated question: %q", got)
	}
}

func TestAggieServiceErrorIsStructuredAndPrivate(t *testing.T) {
	for _, code := range []string{"model_unavailable", "records_unavailable"} {
		r := httptest.NewRecorder()
		writeAgieServiceError(r, code, "Please try again.")
		if r.Code != http.StatusServiceUnavailable {
			t.Errorf("status=%d", r.Code)
		}
		if r.Header().Get("Cache-Control") != "no-store" {
			t.Error("missing private no-store header")
		}
		var body map[string]string
		if err := json.Unmarshal(r.Body.Bytes(), &body); err != nil || body["code"] != code {
			t.Errorf("bad JSON %q %v", r.Body.String(), err)
		}
	}
}
