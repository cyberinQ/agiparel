package interviewer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type LLMRequest struct {
	Model       string       `json:"model"`
	Messages    []LLMMessage `json:"messages"`
	Temperature float64      `json:"temperature"`
}

type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type LLMResponse struct {
	Message LLMMessage `json:"message"`
}

type InterviewerEngine struct {
	ollamaHost string
	modelName  string
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
	}
}

func (e *InterviewerEngine) QueryAgie(systemPrompt, userQuery string) (string, error) {
	url := fmt.Sprintf("%s/api/chat", e.ollamaHost)
	
	payload := LLMRequest{
		Model: e.modelName,
		Messages: []LLMMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userQuery},
		},
		Temperature: 0.2,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Sprintf("Agie (Air-Gapped Fallback): Acknowledged. Setting up workflow parameters for: %s", userQuery), nil
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