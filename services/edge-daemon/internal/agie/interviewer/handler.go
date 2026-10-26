// Author: Sunil Jain, CyberinQ LLC
package interviewer

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/sqlguard"
)

const AgieExecutivePrompt = `You are Agie, a friendly factory assistant.
STRICT RULES:
1. You MUST use the 'run_safe_sql' tool first to query the database before answering.
2. NEVER invent data. If the tool returns empty, say "I do not have that information right now."
3. Speak simply at a 5th-grade level. No technical jargon.
4. NEVER mention "SQL", "database", "tools", or "inference". Just give the final answer naturally.`

type LLMMessage struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

type LLMRequest struct {
	Model       string           `json:"model"`
	Messages    []LLMMessage     `json:"messages"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	Temperature float64          `json:"temperature"`
	Stream      bool             `json:"stream"`
}

type InterviewerEngine struct {
	cloudEndpoint string
	ollamaHost    string
	modelName     string
	client        *http.Client
}

func NewInterviewerEngine() *InterviewerEngine {
	endpoint := strings.TrimSpace(os.Getenv("CLOUD_S27_ENDPOINT"))
	oHost := strings.TrimSpace(os.Getenv("OLLAMA_HOST"))
	if oHost == "" {
		oHost = "http://host.docker.internal:11434"
	}
	model := strings.TrimSpace(os.Getenv("AGIE_MODEL"))
	if model == "" {
		model = "qwen3-coder:30b"
	}
	// Larger locally hosted models can take longer than 30 seconds to load and answer.
	// Keep this configurable without changing the server's public API.
	timeoutSeconds := 180
	if raw := strings.TrimSpace(os.Getenv("AGIE_HTTP_TIMEOUT_SECONDS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 10 && n <= 600 {
			timeoutSeconds = n
		}
	}
	return &InterviewerEngine{
		cloudEndpoint: endpoint,
		ollamaHost:    oHost,
		modelName:     model,
		client:        &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second},
	}
}

func (e *InterviewerEngine) Chat(ctx context.Context, db *sql.DB, messages []LLMMessage, tenantID ...string) (string, error) {
	fullMessages := []LLMMessage{{Role: "system", Content: AgieExecutivePrompt}}
	fullMessages = append(fullMessages, messages...)

	inspector := sqlguard.NewCatalogInspector(db, tenantID...)
	verifiedSQL := false

	for step := 0; step < 4; step++ {
		reqPayload := LLMRequest{
			Model:       e.modelName,
			Messages:    fullMessages,
			Tools:       AgieToolDefinitions,
			Temperature: 0.0,
			Stream:      false,
		}
		bodyBytes, _ := json.Marshal(reqPayload)

		var resp *http.Response
		var err error

		if e.cloudEndpoint != "" && os.Getenv("S27_API_KEY") != "" {
			httpReq, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, e.cloudEndpoint, bytes.NewReader(bodyBytes))
			if reqErr == nil {
				httpReq.Header.Set("Content-Type", "application/json")
				httpReq.Header.Set("Authorization", "Bearer "+os.Getenv("S27_API_KEY"))
				resp, err = e.client.Do(httpReq)
				if resp != nil && resp.StatusCode != http.StatusOK {
					resp.Body.Close()
					resp = nil
				}
			}
		}

		if resp == nil {
			oHTTPReq, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/chat", e.ollamaHost), bytes.NewReader(bodyBytes))
			if reqErr != nil {
				return "", reqErr
			}
			oHTTPReq.Header.Set("Content-Type", "application/json")
			resp, err = e.client.Do(oHTTPReq)
			if err != nil {
				return "", fmt.Errorf("Ollama at %s is unreachable: %w", e.ollamaHost, err)
			}
			if resp.StatusCode != http.StatusOK {
				status := resp.StatusCode
				resp.Body.Close()
				return "", fmt.Errorf("AI service returned HTTP %d (check installed model %s)", status, e.modelName)
			}
		}

		var result struct {
			Message LLMMessage `json:"message"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return "I had trouble understanding the response from my systems.", nil
		}
		resp.Body.Close()

		if len(result.Message.ToolCalls) == 0 {
			if !verifiedSQL {
				return "I cannot verify that answer from factory records right now.", nil
			}
			if strings.TrimSpace(result.Message.Content) == "" {
				return "I do not have enough verified information to answer.", nil
			}
			return result.Message.Content, nil
		}

		fullMessages = append(fullMessages, result.Message)

		for _, tc := range result.Message.ToolCalls {
			var toolOut string
			switch tc.Function.Name {
			case "inspect_db_catalog":
				var args struct {
					TableName string `json:"table_name"`
				}
				_ = json.Unmarshal(tc.Function.Arguments, &args)
				out, err := inspector.InspectCatalog(ctx, args.TableName)
				if err != nil {
					toolOut = fmt.Sprintf(`{"error": %q}`, err.Error())
				} else {
					toolOut = out
				}
			case "run_safe_sql":
				var args struct {
					Query string `json:"query"`
				}
				_ = json.Unmarshal(tc.Function.Arguments, &args)
				fixedQuery := strings.ReplaceAll(args.Query, "active_floor_issues", "floor_issues")
				out, err := inspector.ExecuteSafeSQL(ctx, fixedQuery)
				if err != nil {
					toolOut = fmt.Sprintf(`{"error": %q}`, err.Error())
				} else {
					toolOut = out
					verifiedSQL = true
				}
			default:
				toolOut = `{"error": "unknown tool"}`
			}

			fullMessages = append(fullMessages, LLMMessage{
				Role:    "tool",
				Content: toolOut,
			})
		}
	}

	return "I had to think for too long and could not find the answer.", nil
}
