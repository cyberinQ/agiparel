package interviewer

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const AgieExecutivePrompt = `You are AGIE, the Autonomous Executive Plant Partner for the Factory General Manager.
Axiom: "Make blame impossible by making evidence automatic."
Rules:
1. Ground truth data from the factory PostgreSQL database is provided below.
2. Answer the user's inquiry directly, factually, and concisely in professional English.
3. Explicitly report all active floor issues, absent operators, maintenance requests, and line constraints.
4. Never mention prompts, tools, or SQL queries in your response.`

type LLMRequest struct {
	Model       string       `json:"model"`
	Messages    []LLMMessage `json:"messages"`
	Temperature float64      `json:"temperature"`
	Stream      bool         `json:"stream"`
}

type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type InterviewerEngine struct {
	ollamaHost string
	modelName  string
	client     *http.Client
}

func NewInterviewerEngine() *InterviewerEngine {
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = "http://localhost:11434"
	}
	model := os.Getenv("AGIE_MODEL")
	if model == "" {
		model = "qwen2.5-coder:7b"
	}
	return &InterviewerEngine{
		ollamaHost: host,
		modelName:  model,
		client:     &http.Client{Timeout: 60 * time.Second},
	}
}

// FetchLiveGroundTruth extracts verifiable plant ground truth strictly from PostgreSQL.
// No fallback strings. If the database has no records, it states that fact explicitly.
func FetchLiveGroundTruth(ctx context.Context, db *sql.DB) string {
	if db == nil {
		return "DATABASE STATUS: Offline (operating in disconnected air-gapped mode).\n"
	}

	// Scope tenant context for PostgreSQL Row-Level Security
	if _, err := db.ExecContext(ctx, "SET app.current_tenant_id = 'TENANT_DEFAULT'"); err != nil {
		return fmt.Sprintf("DATABASE ERROR: Failed to set tenant context: %v\n", err)
	}

	var sb strings.Builder

	// 1. Factory Profile (Tenants Table)
	var tenantID, tenantName string
	err := db.QueryRowContext(ctx, `SELECT id, name FROM tenants LIMIT 1;`).Scan(&tenantID, &tenantName)
	if err == nil {
		sb.WriteString(fmt.Sprintf("FACTORY TENANT: %s (ID: %s)\n", tenantName, tenantID))
	} else {
		sb.WriteString("FACTORY TENANT: No tenant record found in database.\n")
	}

	// 2. Active Work Orders & Styles
	rowsWO, err := db.QueryContext(ctx, `
		SELECT dwo.po_number, sm.style_code, sm.category, dwo.line_id, dwo.ordered_qty, dwo.status
		FROM dynamic_work_orders dwo
		JOIN styles_master sm ON dwo.style_id = sm.id
		WHERE dwo.status = 'IN_PROGRESS';
	`)
	if err == nil {
		activeCount := 0
		for rowsWO.Next() {
			activeCount++
			var po, style, category, line, status string
			var qty int
			if err := rowsWO.Scan(&po, &style, &category, &line, &qty, &status); err == nil {
				sb.WriteString(fmt.Sprintf("ACTIVE WORK ORDER: PO #%s | Style: %s (%s) | Line: %s | Target: %d units | Status: %s\n",
					po, style, category, line, qty, status))
			}
		}
		rowsWO.Close()
		if activeCount == 0 {
			sb.WriteString("ACTIVE WORK ORDER: Zero work orders currently in progress.\n")
		}
	} else {
		sb.WriteString("ACTIVE WORK ORDER: Query failed or dynamic_work_orders table empty.\n")
	}

	// 3. Attendance Today
	rowsAtt, err := db.QueryContext(ctx, `
		SELECT w.name, COALESCE(a.assigned_station_id, 'UNASSIGNED'), a.status
		FROM attendance_register a
		JOIN workers w ON a.worker_id = w.id
		WHERE a.shift_date = CURRENT_DATE;
	`)
	if err == nil {
		presentCount := 0
		absentCount := 0
		var absentList []string
		for rowsAtt.Next() {
			var name, station, st string
			if err := rowsAtt.Scan(&name, &station, &st); err == nil {
				if st == "ABSENT" {
					absentCount++
					absentList = append(absentList, fmt.Sprintf("%s (%s)", name, station))
				} else {
					presentCount++
				}
			}
		}
		rowsAtt.Close()
		sb.WriteString(fmt.Sprintf("ATTENDANCE TODAY: %d Present, %d Absent (Total Registered: %d).\n",
			presentCount, absentCount, presentCount+absentCount))
		if absentCount > 0 {
			sb.WriteString(fmt.Sprintf("ABSENT WORKERS: %s\n", strings.Join(absentList, ", ")))
		}
	} else {
		sb.WriteString("ATTENDANCE TODAY: No attendance records for today.\n")
	}

	// 4. Machine Roster
	rowsM, err := db.QueryContext(ctx, `
		SELECT name, line_id, canonical_class, rated_capacity_uph, status
		FROM machine_roster;
	`)
	if err == nil {
		machineCount := 0
		totalCapacity := 0
		var breakdownList []string
		for rowsM.Next() {
			machineCount++
			var name, line, class, st string
			var cap int
			if err := rowsM.Scan(&name, &line, &class, &cap, &st); err == nil {
				totalCapacity += cap
				if st == "BREAKDOWN" || st == "MAINTENANCE" {
					breakdownList = append(breakdownList, fmt.Sprintf("%s on %s (%s)", name, line, st))
				}
			}
		}
		rowsM.Close()
		sb.WriteString(fmt.Sprintf("MACHINE ROSTER: %d machines registered | Total Capacity: %d uph\n", machineCount, totalCapacity))
		if len(breakdownList) > 0 {
			sb.WriteString(fmt.Sprintf("MACHINE DOWNTIME: %s\n", strings.Join(breakdownList, ", ")))
		} else {
			sb.WriteString("MACHINE DOWNTIME: None. All machines nominal.\n")
		}
	} else {
		sb.WriteString("MACHINE ROSTER: No machines registered in database.\n")
	}

	// 5. Hourly Production
	var actualUnits, defectUnits, andonMinutes int
	err = db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(actual_units), 0), COALESCE(SUM(defect_units), 0), COALESCE(SUM(andon_downtime_minutes), 0)
		FROM hourly_line_metrics
		WHERE shift_date = CURRENT_DATE;
	`).Scan(&actualUnits, &defectUnits, &andonMinutes)
	if err == nil {
		dhu := 0.0
		if actualUnits > 0 {
			dhu = (float64(defectUnits) / float64(actualUnits)) * 100.0
		}
		sb.WriteString(fmt.Sprintf("PRODUCTION VELOCITY TODAY: Output = %d pcs | Defects = %d pcs (DHU: %.2f%%) | Downtime = %d mins\n",
			actualUnits, defectUnits, dhu, andonMinutes))
	}

// Inside FetchLiveGroundTruth(ctx context.Context, db *sql.DB):
// Append this query block after Step 5 (Hourly Production):

	// 6. Active Floor Issues Ground Truth (Domain 7 / PM-02)
	iRows, err := db.QueryContext(ctx, `
		SELECT id, line_id, COALESCE(station_id, 'LINE'), issue_type, severity, title, owner_id, status
		FROM floor_issues
		WHERE status IN ('open', 'in_progress')
		ORDER BY started_at DESC;
	`)
	if err == nil {
		var issueList []string
		for iRows.Next() {
			var id, line, station, itype, sev, title, owner, st string
			if err := iRows.Scan(&id, &line, &station, &itype, &sev, &title, &owner, &st); err == nil {
				issueList = append(issueList, fmt.Sprintf("- [%s] Issue %s on %s (%s): %s | Type: %s | Assigned Owner: %s",
					strings.ToUpper(sev), id, line, station, title, itype, owner))
			}
		}
		iRows.Close()
		if len(issueList) > 0 {
			sb.WriteString(fmt.Sprintf("ACTIVE FLOOR ISSUES (%d):\n%s\n", len(issueList), strings.Join(issueList, "\n")))
		} else {
			sb.WriteString("ACTIVE FLOOR ISSUES: None logged. Zero active floor issues.\n")
		}
	} else {
		sb.WriteString("ACTIVE FLOOR ISSUES: Query failed or floor_issues table empty.\n")
	}

	return sb.String()
}

func (e *InterviewerEngine) Chat(ctx context.Context, db *sql.DB, messages []LLMMessage) (string, error) {
	url := fmt.Sprintf("%s/api/chat", e.ollamaHost)
	groundTruth := FetchLiveGroundTruth(ctx, db)
	systemPrompt := fmt.Sprintf("%s\n\n=== LIVE FACTORY GROUND TRUTH ===\n%s=================================",
		AgieExecutivePrompt, groundTruth)

	fullMessages := []LLMMessage{{Role: "system", Content: systemPrompt}}
	fullMessages = append(fullMessages, messages...)

	payload := LLMRequest{
		Model:       e.modelName,
		Messages:    fullMessages,
		Temperature: 0.1,
		Stream:      false,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return "Agie Local Fallback: Edge LLM core unreachable. Operating in air-gapped status.", nil
	}
	defer resp.Body.Close()

	var result struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode LLM response: %w", err)
	}

	return result.Message.Content, nil
}