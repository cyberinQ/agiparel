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
3. If no defects, downtime, or breakdowns are recorded, state that operations are currently nominal.
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

// FetchLiveGroundTruth runs direct parameterized queries across our core domains in < 5ms.
func FetchLiveGroundTruth(ctx context.Context, db *sql.DB) string {
	if db == nil {
		return "Database connection unavailable (operating in air-gapped offline mode)."
	}

	// Set tenant context for Row-Level Security
	_, _ = db.ExecContext(ctx, "SET app.current_tenant_id = 'TENANT_DEFAULT'")

	var sb strings.Builder

	// 1. Work Orders Ground Truth
	var po, style, line, status string
	var orderedQty int
	err := db.QueryRowContext(ctx, `
		SELECT dwo.po_number, sm.style_code, dwo.line_id, dwo.ordered_qty, dwo.status
		FROM dynamic_work_orders dwo
		JOIN styles_master sm ON dwo.style_id = sm.id
		WHERE dwo.status = 'IN_PROGRESS'
		LIMIT 1;
	`).Scan(&po, &style, &line, &orderedQty, &status)
	if err == nil {
		sb.WriteString(fmt.Sprintf("ACTIVE WORK ORDER: PO #%s | Style: %s | Line: %s | Target Qty: %d units | Status: %s\n", po, style, line, orderedQty, status))
	} else {
		sb.WriteString("ACTIVE WORK ORDER: None currently active.\n")
	}

	// 2. Attendance & Absenteeism Ground Truth
	rows, err := db.QueryContext(ctx, `
		SELECT w.name, COALESCE(a.assigned_station_id, 'UNASSIGNED'), a.status
		FROM attendance_register a
		JOIN workers w ON a.worker_id = w.id
		WHERE a.shift_date = CURRENT_DATE;
	`)
	if err == nil {
		presentCount := 0
		absentCount := 0
		var absentDetails []string
		for rows.Next() {
			var name, station, st string
			if err := rows.Scan(&name, &station, &st); err == nil {
				if st == "ABSENT" {
					absentCount++
					absentDetails = append(absentDetails, fmt.Sprintf("%s (Station: %s)", name, station))
				} else {
					presentCount++
				}
			}
		}
		rows.Close()
		sb.WriteString(fmt.Sprintf("ATTENDANCE TODAY: %d Present, %d Absent.\n", presentCount, absentCount))
		if absentCount > 0 {
			sb.WriteString(fmt.Sprintf("ABSENT WORKERS: %s\n", strings.Join(absentDetails, ", ")))
		} else {
			sb.WriteString("ABSENT WORKERS: None. Full attendance on floor.\n")
		}
	}

	// 3. Machines & Maintenance Ground Truth
	mRows, err := db.QueryContext(ctx, `
		SELECT name, line_id, station_id, canonical_class, rated_capacity_uph, status
		FROM machine_roster;
	`)
	if err == nil {
		totalCapacity := 0
		breakdowns := 0
		var breakdownDetails []string
		for mRows.Next() {
			var name, lineID, stationID, class, st string
			var cap int
			if err := mRows.Scan(&name, &lineID, &stationID, &class, &cap, &st); err == nil {
				totalCapacity += cap
				if st == "BREAKDOWN" || st == "MAINTENANCE" {
					breakdowns++
					breakdownDetails = append(breakdownDetails, fmt.Sprintf("%s on %s (%s)", name, lineID, st))
				}
			}
		}
		mRows.Close()
		sb.WriteString(fmt.Sprintf("MACHINE CAPACITY: Total Rated Capacity = %d pcs/hr.\n", totalCapacity))
		if breakdowns > 0 {
			sb.WriteString(fmt.Sprintf("MACHINE ISSUES: %d machines down: %s\n", breakdowns, strings.Join(breakdownDetails, ", ")))
		} else {
			sb.WriteString("MACHINE ISSUES: Zero breakdowns or maintenance holds. All machines nominal.\n")
		}
	}

	// 4. Hourly Production Metrics Ground Truth
	var actualUnits, defectUnits, andonMinutes int
	err = db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(actual_units), 0), COALESCE(SUM(defect_units), 0), COALESCE(SUM(andon_downtime_minutes), 0)
		FROM hourly_line_metrics
		WHERE shift_date = CURRENT_DATE;
	`).Scan(&actualUnits, &defectUnits, &andonMinutes)
	if err == nil {
		dhu := 0.0
		if actualUnits > 0 {
			dhu = (float64(defectUnits) / float64(actualUnits)) * 100
		}
		sb.WriteString(fmt.Sprintf("PRODUCTION VELOCITY TODAY: Actual Output = %d pcs | Defects = %d pcs (DHU: %.2f%%) | Downtime = %d mins\n",
			actualUnits, defectUnits, dhu, andonMinutes))
	}

	return sb.String()
}

// Chat performs direct-inferred executive reasoning backed by live ground truth.
func (e *InterviewerEngine) Chat(ctx context.Context, db *sql.DB, messages []LLMMessage) (string, error) {
	url := fmt.Sprintf("%s/api/chat", e.ollamaHost)

	// Fetch live ground truth directly from the database in < 5ms
	groundTruth := FetchLiveGroundTruth(ctx, db)

	systemPrompt := fmt.Sprintf("%s\n\n=== LIVE FACTORY GROUND TRUTH ===\n%s=================================", AgieExecutivePrompt, groundTruth)

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