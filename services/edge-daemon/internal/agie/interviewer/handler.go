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
	endpoint := os.Getenv("CLOUD_S27_ENDPOINT")
	if endpoint == "" {
		endpoint = "https://api.agiparel.cloud/v1/s27/copilot/chat"
	}
	oHost := os.Getenv("OLLAMA_HOST")
	if oHost == "" {
		oHost = "http://host.docker.internal:11434"
	}
	model := os.Getenv("AGIE_MODEL")
	if model == "" {
		model = "qwen2.5-coder:7b"
	}
	return &InterviewerEngine{
		cloudEndpoint: endpoint,
		ollamaHost:    oHost,
		modelName:     model,
		client:        &http.Client{Timeout: 60 * time.Second},
	}
}

func (e *InterviewerEngine) Chat(ctx context.Context, db *sql.DB, messages []LLMMessage, tenantID ...string) (string, error) {
	fullMessages := []LLMMessage{{Role: "system", Content: AgieExecutivePrompt}}
	fullMessages = append(fullMessages, messages...)
	inspector := sqlguard.NewCatalogInspector(db)

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
		
		httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, e.cloudEndpoint, bytes.NewBuffer(bodyBytes))
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("S27_API_KEY")))
		
		resp, err = e.client.Do(httpReq)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil { resp.Body.Close() }
			oHttpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/chat", e.ollamaHost), bytes.NewBuffer(bodyBytes))
			oHttpReq.Header.Set("Content-Type", "application/json")
			resp, err = e.client.Do(oHttpReq)
			if err != nil {
				return "I am offline and cannot reach my systems right now.", nil
			}
		}

		var result struct {
			Message LLMMessage `json:"message"`
		}
		
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			if resp != nil { resp.Body.Close() }
			return "I had trouble understanding the response from my systems.", nil
		}
		if resp != nil { resp.Body.Close() }

		if len(result.Message.ToolCalls) == 0 {
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
