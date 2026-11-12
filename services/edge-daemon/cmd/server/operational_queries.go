// Author: Sunil Jain, 2026-11-12 — Tenant-scoped, source-backed operational counts and identity answers.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/internal/agie/interviewer"
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/aggieintent"
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
		case strings.Contains(q, "factory") && (strings.Contains(q, "name") || strings.Contains(q, "called")) &&
			(strings.Contains(q, "what") || strings.Contains(q, "tell") || strings.Contains(q, "show") || strings.Contains(q, "know")):
			return "factory_name"
		case (strings.Contains(q, "list") || strings.Contains(q, "show") || strings.Contains(q, "who are") || strings.Contains(q, "what are")) &&
			(strings.Contains(q, "worker") || strings.Contains(q, "employee") || strings.Contains(q, "staff")) &&
			(strings.Contains(q, "machine") || strings.Contains(q, "equipment")):
			return "workers_and_machines"
		case strings.Contains(q, "capacity") || strings.Contains(q, "how many pieces can") || strings.Contains(q, "daily output capability"):
			return "factory_capacity"
		case isAttendanceQuestion(q):
			return "attendance_today"
		case (strings.Contains(q, "po") || strings.Contains(q, "purchase order") || strings.Contains(q, "work order")) && (strings.Contains(q, "progress") || strings.Contains(q, "running") || strings.Contains(q, "active")):
			return "po_in_progress"
		}
		break
	}
	return ""
}

// operationalIntents keeps deterministic numerical questions out of the LLM.
// Multiple counts are answered separately; never give an invented aggregate.
func operationalIntents(messages []interviewer.LLMMessage) []string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "user" {
			continue
		}
		if exact := aggieintent.Resolve(messages[i].Content); len(exact) > 0 {
			return exact
		}
		break
	}
	if legacy := operationalIntent(messages); legacy != "" {
		return []string{legacy}
	}
	return nil
}

// Factory time zone is explicit when configured; legacy installations used India time.
// The response labels the fallback rather than silently guessing a new customer's zone.
func aggieFactoryTimeZone() (string, string, error) {
	zone := strings.TrimSpace(os.Getenv("AGIPAREL_FACTORY_TIMEZONE"))
	note := "configured factory time"
	if zone == "" {
		zone = "Asia/Kolkata"
		note = "legacy default Asia/Kolkata (ask the administrator to set the factory time zone)"
	}
	if _, err := time.LoadLocation(zone); err != nil {
		return "", "", fmt.Errorf("invalid factory time zone: %w", err)
	}
	return zone, note, nil
}

// isAttendanceQuestion routes everyday workforce questions directly to verified
// attendance records rather than the slower, optional language model. In particular,
// "who is present and who is absent today" need not mention "workers" explicitly.
// Questions about unrelated people being present must not be intercepted.
func isAttendanceQuestion(q string) bool {
	workforce := strings.Contains(q, "worker") || strings.Contains(q, "employee") ||
		strings.Contains(q, "staff") || strings.Contains(q, "attendance") || strings.Contains(q, "shift")
	who := strings.Contains(q, "who is") || strings.Contains(q, "who's") ||
		strings.Contains(q, "who are") || strings.Contains(q, "who was") || strings.Contains(q, "who has")
	status := strings.Contains(q, "present") || strings.Contains(q, "absent") ||
		strings.Contains(q, "on leave") || strings.Contains(q, "late") || strings.Contains(q, "checked in")
	today := strings.Contains(q, "today") || strings.Contains(q, "this morning") ||
		strings.Contains(q, "this shift") || strings.Contains(q, "right now")
	return (workforce && status) || (who && status && today)
}

func groundedOperationalReply(ctx context.Context, db *sql.DB, intent string) (string, error) {
	if db == nil {
		return "", fmt.Errorf("PostgreSQL is not configured")
	}
	tenant, err := intakeTenant()
	if err != nil {
		return "", fmt.Errorf("factory tenant is not configured: %w", err)
	}
	queryCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	tx, err := db.BeginTx(queryCtx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(queryCtx, `SELECT set_config('app.current_tenant_id', $1, true)`, tenant); err != nil {
		return "", err
	}

	switch intent {
	case aggieintent.AttendanceToday:
		zone, zoneNote, e := aggieFactoryTimeZone()
		if e != nil {
			return "I cannot check today's attendance until the factory time zone is fixed.", nil
		}
		var date time.Time
		var recorded, present, late, absent, onLeave int64
		err = tx.QueryRowContext(queryCtx, `SELECT (CURRENT_TIMESTAMP AT TIME ZONE $2)::date,
          COUNT(*), COUNT(*) FILTER (WHERE status='PRESENT'),
          COUNT(*) FILTER (WHERE status='LATE'), COUNT(*) FILTER (WHERE status='ABSENT'),
          COUNT(*) FILTER (WHERE status='ON_LEAVE')
          FROM attendance_register WHERE tenant_id=$1
          AND shift_date=(CURRENT_TIMESTAMP AT TIME ZONE $2)::date`, tenant, zone).
			Scan(&date, &recorded, &present, &late, &absent, &onLeave)
		if err != nil {
			return "", err
		}
		if recorded == 0 {
			return fmt.Sprintf("For %s (%s), no attendance entries were recorded. I cannot say how many workers are at the factory today. Open the attendance register.", date.Format("2 Jan 2006"), zoneNote), nil
		}
		return fmt.Sprintf("For %s (%s): %d workers marked present or late (%d present, %d late). %d marked absent, %d on leave, across %d attendance entries. This is recorded attendance, not proof of who is physically inside the factory.", date.Format("2 Jan 2006"), zoneNote, present+late, present, late, absent, onLeave, recorded), nil
	case aggieintent.RegisteredWorkers:
		var count int64
		err = tx.QueryRowContext(queryCtx, `SELECT COUNT(*) FROM workers WHERE tenant_id=$1`, tenant).Scan(&count)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d workers are listed in this factory's worker register. This is not today's attendance; unregistered workers are not included.", count), nil
	case aggieintent.RegisteredMachines:
		var total, active, lines int64
		err = tx.QueryRowContext(queryCtx, `SELECT COUNT(*),
			COUNT(*) FILTER (WHERE status='ACTIVE'), COUNT(DISTINCT line_id)
			FROM machine_roster WHERE tenant_id=$1`, tenant).Scan(&total, &active, &lines)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Machine register: %d machines recorded; %d marked ACTIVE across %d lines. ACTIVE is a recorded machine status, not proof a machine is running, free to use, or safe to operate.", total, active, lines), nil
	case aggieintent.MachineStatus:
		// A roster status is NOT a live equipment-availability or safety signal.
		var total, active int64
		err = tx.QueryRowContext(queryCtx, `SELECT COUNT(*), COUNT(*) FILTER (WHERE status='ACTIVE')
			FROM machine_roster WHERE tenant_id=$1`, tenant).Scan(&total, &active)
		if err != nil {
			return "", err
		}
		if total == 0 {
			return "No machines are recorded in the machine register. I cannot determine which machines are open or working.", nil
		}
		rows, e := tx.QueryContext(queryCtx, `SELECT id, name, line_id, station_id, status
			FROM machine_roster WHERE tenant_id=$1
			ORDER BY CASE WHEN status='ACTIVE' THEN 0 ELSE 1 END, line_id, station_id, id LIMIT 21`, tenant)
		if e != nil {
			return "", e
		}
		items := make([]string, 0, 20)
		truncated := false
		for rows.Next() {
			var id, name, line, station, status string
			if e = rows.Scan(&id, &name, &line, &station, &status); e != nil {
				rows.Close()
				return "", e
			}
			if len(items) == 20 {
				truncated = true
				break
			}
			items = append(items, fmt.Sprintf("%s (%s, line %s, station %s): %s", name, id, line, station, status))
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return "", e
		}
		reply := fmt.Sprintf("Machine register: %d machines, %d marked ACTIVE. These are recorded status labels, not verified live availability or machine safety.", total, active)
		if len(items) > 0 {
			reply += "\nRecorded machines:\n- " + strings.Join(items, "\n- ")
		}
		if truncated {
			reply += "\nOnly the first 20 machines are shown; ask the supervisor for the full verified roster."
		}
		return reply + "\nI cannot confirm which machines are physically running, open, idle, or safe to operate from this register.", nil
	case aggieintent.InventoryRolls:
		var total, eligible, reserved, onHold int64
		err = tx.QueryRowContext(queryCtx, `SELECT COUNT(*),
          COUNT(*) FILTER (WHERE status='AVAILABLE' AND inspection_status='PASS' AND remaining_length_m > 0),
          COUNT(*) FILTER (WHERE status='RESERVED' AND remaining_length_m > 0),
          COUNT(*) FILTER (WHERE status='QUARANTINE' OR inspection_status IS DISTINCT FROM 'PASS')
          FROM store_rolls WHERE tenant_id=$1`, tenant).Scan(&total, &eligible, &reserved, &onHold)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Fabric stock book: %d rolls recorded. %d have passed inspection, are marked AVAILABLE, and have fabric left; %d are reserved; %d are on hold or not inspection-passed. These categories may overlap. Roll-check forms alone do not create stock, and this count is not a reservation for any order.", total, eligible, reserved, onHold), nil
	case "factory_name":
		// The latest Chapter 01 revision is the only authoritative source for
		// this question. Provisional names are never presented as confirmed.
		var name, state string
		err := tx.QueryRowContext(queryCtx, `SELECT value_text, review_state
          FROM factory_fact_events_v65b
          WHERE tenant_id=$1 AND field_key='factory.name'
          ORDER BY revision DESC LIMIT 1`, tenant).Scan(&name, &state)
		if err == sql.ErrNoRows {
			return "I do not have your factory name recorded yet. You can add it in Chapter 01 — Factory Setup.", nil
		}
		if err != nil {
			return "", err
		}
		if state == "CONFIRMED" {
			return fmt.Sprintf("Chapter 01 confirms your factory name as %q.", name), nil
		}
		return fmt.Sprintf("Chapter 01 lists your factory name as %q, but it still needs checking.", name), nil
	case "workers_and_machines":
		workers := make([]string, 0, 100)
		rows, err := tx.QueryContext(queryCtx, `SELECT name, employee_code, primary_skill FROM workers WHERE tenant_id=$1 ORDER BY name, id LIMIT 100`, tenant)
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
		rows, err = tx.QueryContext(queryCtx, `SELECT name, line_id, station_id, status FROM machine_roster WHERE tenant_id=$1 ORDER BY line_id, station_id, id LIMIT 100`, tenant)
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
            FROM machine_roster WHERE tenant_id=$1`, tenant).Scan(&totalMachines, &activeMachines, &machineLines)
		if err != nil {
			return "", err
		}

		var attendanceRecords, presentWorkers, lateWorkers int
		err = tx.QueryRowContext(queryCtx, `
            SELECT COUNT(*), COUNT(*) FILTER (WHERE status='PRESENT'),
                   COUNT(*) FILTER (WHERE status='LATE')
            FROM attendance_register
            WHERE tenant_id=$1 AND shift_date=(CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Kolkata')::date`, tenant).Scan(&attendanceRecords, &presentWorkers, &lateWorkers)
		if err != nil {
			return "", err
		}

		var reportingWindows, observedUnits, targetUnits, reportingLines int
		err = tx.QueryRowContext(queryCtx, `
            SELECT COUNT(*), COALESCE(SUM(actual_units),0), COALESCE(SUM(target_units),0), COUNT(DISTINCT line_id)
            FROM hourly_line_metrics
            WHERE tenant_id=$1 AND shift_date=(CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Kolkata')::date`, tenant).Scan(&reportingWindows, &observedUnits, &targetUnits, &reportingLines)
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
            WHERE tenant_id=$1 AND shift_date=(CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Kolkata')::date`, tenant).Scan(&shiftDay, &present, &late, &absent, &leave, &recorded)
		if err != nil {
			return "", err
		}
		if recorded == 0 {
			var mostRecent sql.NullTime
			if err := tx.QueryRowContext(queryCtx, `SELECT MAX(shift_date) FROM attendance_register WHERE tenant_id=$1`, tenant).Scan(&mostRecent); err != nil {
				return "", err
			}
			if mostRecent.Valid {
				return fmt.Sprintf("No attendance has been recorded for today (%s, factory local time). The latest attendance record is for %s. I cannot confirm how many workers are present today.", shiftDay.Format("2 Jan 2006"), mostRecent.Time.Format("2 Jan 2006")), nil
			}
			return fmt.Sprintf("No verified attendance records are available for today (%s, factory local time).", shiftDay.Format("2 Jan 2006")), nil
		}
		// Return named groups when the caller asks WHO, not just aggregate counts.
		// Limit the number of names so a large factory cannot generate unbounded replies.
		groups := map[string][]string{
			"PRESENT": {}, "LATE": {}, "ABSENT": {}, "ON_LEAVE": {},
		}
		rows, err := tx.QueryContext(queryCtx, `
            SELECT a.status, COALESCE(w.name, a.worker_id), a.worker_id
            FROM attendance_register a
            LEFT JOIN workers w ON w.id=a.worker_id AND w.tenant_id=a.tenant_id
            WHERE a.tenant_id=$1 AND a.shift_date=(CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Kolkata')::date
            ORDER BY a.status, COALESCE(w.name, a.worker_id), a.worker_id
            LIMIT 51`, tenant)
		if err != nil {
			return "", err
		}
		seen := 0
		for rows.Next() {
			var status, name, id string
			if err := rows.Scan(&status, &name, &id); err != nil {
				rows.Close()
				return "", err
			}
			if _, ok := groups[status]; ok && seen < 50 {
				groups[status] = append(groups[status], fmt.Sprintf("%s (%s)", name, id))
			}
			seen++
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return "", err
		}
		rows.Close()
		reply := fmt.Sprintf("Today (%s, factory local time): %d PRESENT, %d LATE, %d ABSENT, %d ON_LEAVE out of %d recorded workers.",
			shiftDay.Format("2 Jan 2006"), present, late, absent, leave, recorded)
		for _, entry := range []struct{ status, heading string }{
			{"PRESENT", "Present"}, {"LATE", "Late"}, {"ABSENT", "Absent"}, {"ON_LEAVE", "On leave"},
		} {
			if len(groups[entry.status]) > 0 {
				reply += "\n" + entry.heading + ": " + strings.Join(groups[entry.status], ", ") + "."
			}
		}
		if recorded > 50 {
			reply += "\nOnly the first 50 names are shown; totals include all recorded workers."
		}
		return reply + "\nThese are recorded attendance statuses, not live physical presence. Workers without an attendance entry are not assumed absent.", nil
	case "po_in_progress":
		type order struct {
			po, status string
			qty        int
		}
		orders := []order{}
		rows, err := tx.QueryContext(queryCtx, `SELECT po_number,status,ordered_qty FROM dynamic_work_orders WHERE tenant_id=$1 AND status='IN_PROGRESS' ORDER BY started_at DESC LIMIT 5`, tenant)
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
		rows, err = tx.QueryContext(queryCtx, `SELECT po_number,status,ordered_qty FROM dynamic_work_orders WHERE tenant_id=$1 AND status IN ('PROPOSED','PLANNED','LOCKED','HOLD') ORDER BY started_at DESC LIMIT 5`, tenant)
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
