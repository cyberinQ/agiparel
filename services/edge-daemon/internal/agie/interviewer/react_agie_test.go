// Author: Sunil Jain, CyberinQ LLC
package interviewer_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cyberinq/agiparel/services/edge-daemon/internal/agie/interviewer"
)

func TestReActAgie_FailClosedWithoutVerifiedSQL(t *testing.T) {
	mockOllama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"All 14 lines are running at 100% efficiency."}}`))
	}))
	defer mockOllama.Close()

	t.Setenv("CLOUD_S27_ENDPOINT", "")
	t.Setenv("S27_API_KEY", "")
	t.Setenv("OLLAMA_HOST", mockOllama.URL)

	engine := interviewer.NewInterviewerEngine()
	reply, err := engine.Chat(context.Background(), nil, []interviewer.LLMMessage{
		{Role: "user", Content: "What is our line efficiency right now?"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(reply, "100% efficiency") {
		t.Fatalf("expected unverified hallucination to be blocked, got: %s", reply)
	}
	if !strings.Contains(reply, "cannot verify") {
		t.Fatalf("expected fail-closed verification message, got: %s", reply)
	}
}
