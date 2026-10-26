// Author: Sunil Jain, 2026-10-25
// Read-only manager questions for the existing factory onboarding screen.
// Never approves a plan, changes inventory, or commits a customer document.
package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/planning"
)

func (s *EdgeServer) handlePlanningReadinessQuestions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	po := strings.TrimSpace(r.URL.Query().Get("po"))
	if len(po) > 128 {
		http.Error(w, "PO is too long", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(planning.BuildReadinessInterview(s.loadCumulativeWorkbook(), po))
}
