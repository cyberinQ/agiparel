// Author: Sunil Jain, 2026-10-25
// Read-only audit of legacy machine records in PostgreSQL or a factory workbook.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/masterdata"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	input := flag.String("workbook", "", "Path to a local factory_workbook.json; omit to read PostgreSQL")
	tenant := flag.String("tenant", "TENANT_DEFAULT", "Tenant for read-only PostgreSQL audit")
	flag.Parse()
	var machines []masterdata.Machine
	var err error
	if *input != "" {
		machines, err = fromWorkbook(*input)
	} else {
		machines, err = fromDB(*tenant)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "machine audit:", err)
		os.Exit(1)
	}
	report := masterdata.AuditMachineRoster(machines)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// Findings are advisory, not a process failure. No database writes occur.
}
func fromWorkbook(path string) ([]masterdata.Machine, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var wb struct {
		Machines []struct {
			MachineID string `json:"machine_id"`
			LineID    string `json:"line_id"`
			StationID string `json:"station_id"`
			Name      string `json:"name"`
			Class     string `json:"canonical_class"`
			Capacity  int    `json:"capacity_uph"`
			Status    string `json:"status"`
		} `json:"machines"`
	}
	if err := json.NewDecoder(file).Decode(&wb); err != nil {
		return nil, err
	}
	out := make([]masterdata.Machine, 0, len(wb.Machines))
	for _, m := range wb.Machines {
		out = append(out, masterdata.Machine{ID: m.MachineID, Line: m.LineID, Station: m.StationID, Name: m.Name, Class: m.Class, CapacityUPH: m.Capacity, Status: m.Status})
	}
	return out, nil
}
func fromDB(tenant string) ([]masterdata.Machine, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL is not set (or supply -workbook)")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "SELECT set_config('app.current_tenant_id', $1, true)", tenant); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,line_id,station_id,name,canonical_class,rated_capacity_uph,status FROM machine_roster WHERE tenant_id=$1 ORDER BY id`, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []masterdata.Machine{}
	for rows.Next() {
		var m masterdata.Machine
		if err = rows.Scan(&m.ID, &m.Line, &m.Station, &m.Name, &m.Class, &m.CapacityUPH, &m.Status); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
