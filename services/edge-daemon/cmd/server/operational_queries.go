// Author: Sunil Jain, 2026-10-25 — Tenant-scoped attendance, PO and capacity answers without AI hallucination.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/internal/agie/interviewer"
)

// A simple direct SQL route is safer and faster than asking a language model to
// invent a query for high-volume, unambiguous operational facts.
func operationalIntent(messages []interviewer.LLMMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "user" {
			continue
		}
		q := strings.ToLower(strings.TrimSpace(messages[i].Content))
		switch {
		case (strings.Contains(q, "list") || strings.Contains(q, "show") || strings.Contains(q, "who are") || strings.Contains(q, "what are")) &&
			(strings.Contains(q, "worker") || strings.Contains(q, "employee") || strings.Contains(q, "staff")) &&
			(strings.Contains(q, "machine") || strings.Contains(q, "equipment")):
			return "workers_and_machines"
		case strings.Contains(q, "capacity") || strings.Contains(q, "how many pieces can") || strings.Contains(q, "daily output capability"):
			return "factory_capacity"
		case (strings.Contains(q, "present") || strings.Contains(q, "attendance")) && (strings.Contains(q, "worker") || strings.Contains(q, "employee") || strings.Contains(q, "staff")):
			return "attendance_today"
		case (strings.Contains(q, "po") || strings.Contains(q, "purchase order") || strings.Contains(q, "work order")) && (strings.Contains(q, "progress") || strings.Contains(q, "running") || strings.Contains(q, "active")):
			return "po_in_progress"
		}
		break
	}
	return ""
}

func groundedOperationalReply(ctx context.Context, db *sql.DB, intent string) (string, error) {
	if db == nil {
		return "", fmt.Errorf("PostgreSQL is not configured")
	}
	queryCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	tx, err := db.BeginTx(queryCtx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(queryCtx, `SELECT set_config('app.current_tenant_id', $1, true)`, "TENANT_DEFAULT"); err != nil {
		return "", err
	}

	switch intent {
	case "workers_and_machines":
		workers := make([]string, 0, 100)
		rows, err := tx.QueryContext(queryCtx, `SELECT name, employee_code, primary_skill FROM workers WHERE tenant_id=$1 ORDER BY name, id LIMIT 100`, "TENANT_DEFAULT")
		if err != nil {
			return "", err
		}
		for rows.Next() {
			var name, code, skill string
			if err := rows.Scan(&name, &code, &skill); err != nil {
				rows.Close()
				return "", err
			}
			workers = append(workers, fmt.Sprintf("%s (%s, %s)", name, code, skill))
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return "", err
		}
		rows.Close()

		machines := make([]string, 0, 100)
		rows, err = tx.QueryContext(queryCtx, `SELECT name, line_id, station_id, status FROM machine_roster WHERE tenant_id=$1 ORDER BY line_id, station_id, id LIMIT 100`, "TENANT_DEFAULT")
		if err != nil {
			return "", err
		}
		for rows.Next() {
			var name, line, station, status string
			if err := rows.Scan(&name, &line, &station, &status); err != nil {
				rows.Close()
				return "", err
			}
			machines = append(machines, fmt.Sprintf("%s (%s, %s, %s)", name, line, station, status))
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return "", err
		}
		rows.Close()

		reply := fmt.Sprintf("Factory records: %d workers and %d machines are listed below (up to 100 each).", len(workers), len(machines))
		if len(workers) == 0 {
			reply += " No workers are recorded."
		} else {
			reply += "\nWorkers:\n- " + strings.Join(workers, "\n- ")
		}
		if len(machines) == 0 {
			reply += " No machines are recorded."
		} else {
			reply += "\nMachines:\n- " + strings.Join(machines, "\n- ")
		}
		return reply, nil
	case "factory_capacity":
		// These are independently verifiable resource counts. Summing rated machine
		// UPH would be incorrect: garment operations form a sequential bottlenecked line.
		var totalMachines, activeMachines, machineLines int
		err = tx.QueryRowContext(queryCtx, `
            SELECT COUNT(*), COUNT(*) FILTER (WHERE status='ACTIVE'), COUNT(DISTINCT line_id)
            FROM machine_roster WHERE tenant_id=$1`, "TENANT_DEFAULT").Scan(&totalMachines, &activeMachines, &machineLines)
		if err != nil {
			return "", err
		}

		var attendanceRecords, presentWorkers, lateWorkers int
		err = tx.QueryRowContext(queryCtx, `
            SELECT COUNT(*), COUNT(*) FILTER (WHERE status='PRESENT'),
                   COUNT(*) FILTER (WHERE status='LATE')
            FROM attendance_register
            WHERE tenant_id=$1 AND shift_date=(CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Kolkata')::date`, "TENANT_DEFAULT").Scan(&attendanceRecords, &presentWorkers, &lateWorkers)
		if err != nil {
			return "", err
		}

		var reportingWindows, observedUnits, targetUnits, reportingLines int
		err = tx.QueryRowContext(queryCtx, `
            SELECT COUNT(*), COALESCE(SUM(actual_units),0), COALESCE(SUM(target_units),0), COUNT(DISTINCT line_id)
            FROM hourly_line_metrics
            WHERE tenant_id=$1 AND shift_date=(CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Kolkata')::date`, "TENANT_DEFAULT").Scan(&reportingWindows, &observedUnits, &targetUnits, &reportingLines)
		if err != nil {
			return "", err
		}

		reply := fmt.Sprintf("Factory resources: %d machines (%d marked ACTIVE) across %d lines.", totalMachines, activeMachines, machineLines)
		if attendanceRecords == 0 {
			reply += " Today's verified worker attendance has not been recorded."
		} else {
			reply += fmt.Sprintf(" Today's attendance: %d PRESENT and %d LATE out of %d records.", presentWorkers, lateWorkers, attendanceRecords)
		}
		if reportingWindows == 0 {
			reply += " No hourly production results have been recorded today."
		} else {
			reply += fmt.Sprintf(" Today's recorded production: %d units against %d planned units over %d hourly line windows on %d lines.", observedUnits, targetUnits, reportingWindows, reportingLines)
		}
		reply += " I cannot verify total achievable pieces/day from these records: it requires validated operation SAM, bottleneck/line balance, actual attendance and production efficiency. Machine rated hourly capacities cannot be added together."
		return reply, nil
	case "attendance_today":
		var shiftDay time.Time
		var present, late, absent, leave, recorded int
		err = tx.QueryRowContext(queryCtx, `
            SELECT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Kolkata')::date,
                   COUNT(*) FILTER (WHERE status='PRESENT'),
                   COUNT(*) FILTER (WHERE status='LATE'),
                   COUNT(*) FILTER (WHERE status='ABSENT'),
                   COUNT(*) FILTER (WHERE status='ON_LEAVE'),
                   COUNT(*)
            FROM attendance_register
            WHERE tenant_id=$1 AND shift_date=(CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Kolkata')::date`, "TENANT_DEFAULT").Scan(&shiftDay, &present, &late, &absent, &leave, &recorded)
		if err != nil {
			return "", err
		}
		if recorded == 0 {
			var mostRecent sql.NullTime
			if err := tx.QueryRowContext(queryCtx, `SELECT MAX(shift_date) FROM attendance_register WHERE tenant_id=$1`, "TENANT_DEFAULT").Scan(&mostRecent); err != nil {
				return "", err
			}
			if mostRecent.Valid {
				return fmt.Sprintf("No attendance has been recorded for today (%s, factory local time). The latest attendance record is for %s. I cannot confirm how many workers are present today.", shiftDay.Format("2 Jan 2006"), mostRecent.Time.Format("2 Jan 2006")), nil
			}
			return fmt.Sprintf("No verified attendance records are available for today (%s, factory local time).", shiftDay.Format("2 Jan 2006")), nil
		}
		return fmt.Sprintf("Today (%s, factory local time): %d workers marked PRESENT, %d LATE, %d ABSENT, %d ON_LEAVE, from %d attendance records. These are recorded statuses, not live physical presence.", shiftDay.Format("2 Jan 2006"), present, late, absent, leave, recorded), nil
	case "po_in_progress":
		type order struct {
			po, status string
			qty        int
		}
		orders := []order{}
		rows, err := tx.QueryContext(queryCtx, `SELECT po_number,status,ordered_qty FROM dynamic_work_orders WHERE tenant_id=$1 AND status='IN_PROGRESS' ORDER BY started_at DESC LIMIT 5`, "TENANT_DEFAULT")
		if err != nil {
			return "", err
		}
		for rows.Next() {
			var o order
			if err = rows.Scan(&o.po, &o.status, &o.qty); err != nil {
				rows.Close()
				return "", err
			}
			orders = append(orders, o)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return "", err
		}
		if len(orders) > 0 {
			parts := make([]string, 0, len(orders))
			for _, o := range orders {
				parts = append(parts, fmt.Sprintf("%s (%d pieces)", o.po, o.qty))
			}
			return "PO(s) marked IN_PROGRESS: " + strings.Join(parts, ", ") + ".", nil
		}
		rows, err = tx.QueryContext(queryCtx, `SELECT po_number,status,ordered_qty FROM dynamic_work_orders WHERE tenant_id=$1 AND status IN ('PROPOSED','PLANNED','LOCKED','HOLD') ORDER BY started_at DESC LIMIT 5`, "TENANT_DEFAULT")
		if err != nil {
			return "", err
		}
		for rows.Next() {
			var o order
			if err = rows.Scan(&o.po, &o.status, &o.qty); err != nil {
				rows.Close()
				return "", err
			}
			orders = append(orders, o)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return "", err
		}
		if len(orders) == 0 {
			return "No purchase order is marked IN_PROGRESS, and I found no pending work orders.", nil
		}
		parts := make([]string, 0, len(orders))
		for _, o := range orders {
			parts = append(parts, fmt.Sprintf("%s (%s, %d pieces)", o.po, o.status, o.qty))
		}
		return "No PO is currently marked IN_PROGRESS. Other recorded orders: " + strings.Join(parts, ", ") + ".", nil
	}
	return "", fmt.Errorf("unsupported operational question")
}
