// Author: Sunil Jain, 2026-11-12.
// Verify a model can read the complete manual and cannot use it to invent live production.
package interviewer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestManualToolDoesNotRequireDatabase(t *testing.T) {
	steps := 0
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		steps++
		var req LLMRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		hasManual := false
		for _, tool := range req.Tools {
			if tool.Function.Name == "read_worker_manual" {
				hasManual = true
			}
		}
		if !hasManual {
			t.Error("model has no worker manual tool")
		}
		w.Header().Set("Content-Type", "application/json")
		if steps == 1 {
			_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"","tool_calls":[{"id":"call-1","type":"function","function":{"name":"read_worker_manual","arguments":{"chapter":5}}}]}}`))
			return
		}
		found := false
		for _, msg := range req.Messages {
			if msg.Role == "tool" && strings.Contains(msg.Content, "Save lay draft") {
				found = true
			}
		}
		if !found {
			t.Error("manual content was not passed to model")
		}
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"Chapter 05: Save lay draft. A draft is not a physical release."}}`))
	}))
	defer model.Close()
	t.Setenv("CLOUD_S27_ENDPOINT", "")
	t.Setenv("S27_API_KEY", "")
	t.Setenv("OLLAMA_HOST", model.URL)
	engine := NewInterviewerEngine()
	answer, err := engine.Chat(context.Background(), nil, []LLMMessage{{Role: "user", Content: "How do I plan a cut?"}})
	if err != nil || !strings.Contains(answer, "Save lay draft") {
		t.Fatalf("manual not available without DB: %v %q", err, answer)
	}
	if steps != 2 {
		t.Fatalf("expected tool call and answer, got %d requests", steps)
	}
}

func TestManualToolCannotVerifyLiveNumbers(t *testing.T) {
	steps := 0
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		steps++
		w.Header().Set("Content-Type", "application/json")
		if steps == 1 {
			_, _ = w.Write([]byte(`{"message":{"role":"assistant","tool_calls":[{"id":"call-1","type":"function","function":{"name":"read_worker_manual","arguments":{"chapter":5}}}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"401 garments were signed released today."}}`))
	}))
	defer model.Close()
	t.Setenv("CLOUD_S27_ENDPOINT", "")
	t.Setenv("S27_API_KEY", "")
	t.Setenv("OLLAMA_HOST", model.URL)
	answer, err := NewInterviewerEngine().Chat(context.Background(), nil, []LLMMessage{{Role: "user", Content: "How many garments were signed released today?"}})
	if err != nil || strings.Contains(answer, "401") || !strings.Contains(answer, "cannot verify") {
		t.Fatalf("unguarded live number: %v %q", err, answer)
	}
}
