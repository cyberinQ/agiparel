// Author: Sunil Jain, 2026-11-12.
// Serve protected worker instructions and answer chapter how-to questions from reviewed text.
package main

import (
	"encoding/json"
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/sqlguard"
	"net/http"
	"strconv"
	"strings"

	"github.com/cyberinq/agiparel/services/edge-daemon/internal/agie/interviewer"
	"github.com/cyberinq/agiparel/services/edge-daemon/internal/workerhelp"
)

func (s *EdgeServer) handleAgieManual(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if raw := r.URL.Query().Get("chapter"); raw != "" {
		index, err := strconv.Atoi(raw)
		if err != nil {
			http.Error(w, "Choose a chapter from 1 to 10", http.StatusBadRequest)
			return
		}
		ch, ok := workerhelp.Get(index)
		if !ok {
			http.Error(w, "Choose a chapter from 1 to 10", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"chapter": ch, "source": "reviewed_worker_manual", "version": workerhelp.All().Version})
		return
	}
	_ = json.NewEncoder(w).Encode(workerhelp.All())
}

func lastAgieQuestion(messages []interviewer.LLMMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return strings.TrimSpace(messages[i].Content)
		}
	}
	return ""
}

// handleAgieCapabilities probes only read-only tooling. A successful probe is
// NOT proof of access to every factory table or a functioning model.
func (s *EdgeServer) handleAgieCapabilities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	result := map[string]interface{}{
		"manual_chapters":        len(workerhelp.All().Chapters),
		"catalog_probe":          false,
		"read_only_select_probe": false,
		"model_tool_calling":     "not_tested",
		"access_scope":           "database_app_role_and_tenant; not per-worker authorization",
	}
	if s.db != nil {
		if tenant, err := intakeTenant(); err == nil {
			inspector := sqlguard.NewCatalogInspector(s.db, tenant)
			_, catalogErr := inspector.InspectCatalog(r.Context(), "")
			result["catalog_probe"] = catalogErr == nil
			output, queryErr := inspector.ExecuteSafeSQL(r.Context(), "SELECT 1 AS probe")
			result["read_only_select_probe"] = queryErr == nil && strings.Contains(output, "probe")
		}
	}
	_ = json.NewEncoder(w).Encode(result)
}
