// Author: Sunil Jain, 2026-10-25 — Verify local Ollama routing and fail-closed ground-truth behavior.
package interviewer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatDoesNotInventWithoutVerifiedTool(t *testing.T) {
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("incorrect model API path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"17 workers are here today"}}`))
	}))
	defer model.Close()
	t.Setenv("CLOUD_S27_ENDPOINT", "")
	t.Setenv("S27_API_KEY", "")
	t.Setenv("OLLAMA_HOST", model.URL)
	engine := NewInterviewerEngine()
	answer, err := engine.Chat(context.Background(), nil, []LLMMessage{{Role: "user", Content: "how many workers are present?"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(answer, "17 workers") {
		t.Fatalf("unverified LLM answer leaked: %s", answer)
	}
	if !strings.Contains(answer, "cannot verify") {
		t.Fatalf("expected explicit unverified status, got %s", answer)
	}
}

func TestChatModelMissingIsAnError(t *testing.T) {
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "model not found", http.StatusNotFound) }))
	defer model.Close()
	t.Setenv("CLOUD_S27_ENDPOINT", "")
	t.Setenv("S27_API_KEY", "")
	t.Setenv("OLLAMA_HOST", model.URL)
	engine := NewInterviewerEngine()
	_, err := engine.Chat(context.Background(), nil, []LLMMessage{{Role: "user", Content: "report bottlenecks"}})
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected HTTP 404 model error, got %v", err)
	}
}
