package interviewer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

const ArchitectPrompt = `You are AGIE-ARCHITECT, Lead IT Setup Agent for Agiparel OS.
Interview the user to gather factory topology (factory name, unit, line, machine types, capacity).
Invariants:
1. Canonical machine types: SNLS, DNLS, 3-OL, 4-OL, 5-OL, KANSAI_FLATLOCK, BUTTON_HOLE, BARTACK, END_CUTTER.
2. If capacity < 10 or > 2000 pcs/hr, challenge the value.
3. Once all facts are gathered, present an itemized summary and ask: "Please reply CONFIRM to apply changes."`

const AdvisorPromptTemplate = `You are AGIE-ADVISOR, Executive Strategic Partner for the Factory GM powered by Qwen.
You have direct, real-time access to the plant's live operational telemetry and configuration database.

=== CURRENT LIVE FACTORY GROUND TRUTH ===
%s
=========================================

INSTRUCTIONS FOR GM QUERIES:
1. Always answer directly and truthfully using the ground-truth data above.
2. If the GM asks about the work order, supervisor, attendance, capacity, defects, or pacing, pull the answer directly from the facts above.
3. Never claim you do not have access to real-time work orders, supervisors, or metrics; cite the values from the ground-truth block above.
4. Keep answers concise, factual, and actionable.
5. Unless the GM explicitly asks to "make a note", "remember this", or "take action", conversations are transient.`

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
		client:     &http.Client{Timeout: 90 * time.Second},
	}
}

func (e *InterviewerEngine) Chat(mode string, messages []LLMMessage, plantContext string) (string, error) {
	url := fmt.Sprintf("%s/api/chat", e.ollamaHost)

	var sysPrompt string
	if mode == "ADVISOR" {
		sysPrompt = fmt.Sprintf(AdvisorPromptTemplate, plantContext)
	} else {
		sysPrompt = ArchitectPrompt
	}

	fullMessages := append([]LLMMessage{{Role: "system", Content: sysPrompt}}, messages...)

	payload := LLMRequest{
		Model:       e.modelName,
		Messages:    fullMessages,
		Temperature: 0.2,
		Stream:      false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	resp, err := e.client.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return "Agie Local Fallback: Edge LLM core unreachable. Ensure Ollama is running.", nil
	}
	defer resp.Body.Close()

	var result struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Message.Content, nil
}