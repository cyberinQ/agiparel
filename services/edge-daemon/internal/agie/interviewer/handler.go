package interviewer

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type CloudAgieRequest struct {
	TenantID string       `json:"tenant_id"`
	PlantID  string       `json:"plant_id"`
	Messages []LLMMessage `json:"messages"`
}

type CloudAgieResponse struct {
	Reply string `json:"reply"`
}

type InterviewerEngine struct {
	cloudEndpoint string
	client        *http.Client
}

func NewInterviewerEngine() *InterviewerEngine {
	endpoint := os.Getenv("CLOUD_S27_ENDPOINT")
	if endpoint == "" {
		endpoint = "https://api.agiparel.cloud/v1/s27/copilot/chat"
	}
	return &InterviewerEngine{
		cloudEndpoint: endpoint,
		client:        &http.Client{Timeout: 15 * time.Second},
	}
}

func (e *InterviewerEngine) Chat(ctx context.Context, db *sql.DB, messages []LLMMessage, tenantID ...string) (string, error) {
	activeTenant := "TENANT_DEFAULT"
	if len(tenantID) > 0 && tenantID[0] != "" {
		activeTenant = tenantID[0]
	}

	payload := CloudAgieRequest{
		TenantID: activeTenant,
		PlantID:  os.Getenv("PLANT_ID"),
		Messages: messages,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, e.cloudEndpoint, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("S27_API_KEY")))

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return "Agie Local Fallback: Cloud S27 core unreachable. Operating in air-gapped status.", nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cloud S27 API returned HTTP %d", resp.StatusCode)
	}

	var result CloudAgieResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode S27 response: %w", err)
	}

	return result.Reply, nil
}