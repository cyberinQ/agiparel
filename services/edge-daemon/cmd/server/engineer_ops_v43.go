// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// Engineer Ops is an opt-in, independently authenticated, read-only control plane.
// Full host, code, database and security administration stays on controlled infrastructure.
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/internal/web"
)

func isEngineerOpsPath(path string) bool {
	switch path {
	case "/engineer-ops", "/engineer-ops/assets/ops.css", "/engineer-ops/assets/ops.js", "/api/v1/engineer-ops/overview":
		return true
	}
	return false
}

func isolateEngineerOps(operational, mux http.Handler) http.Handler {
	admin := engineerOpsGate(mux)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isEngineerOpsPath(r.URL.Path) {
			admin.ServeHTTP(w, r)
			return
		}
		operational.ServeHTTP(w, r)
	})
}

// This control plane does not inherit DEV Basic credentials or workforce permissions.
// A production reverse proxy must strip inbound X-Forwarded-* values, set HTTPS itself,
// and keep the app listening on a private interface before TRUST_PROXY_TLS is enabled.
func engineerOpsGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, private")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'self'; script-src 'self'; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Engineer Ops is read-only", http.StatusMethodNotAllowed)
			return
		}
		if os.Getenv("AGIPAREL_ENGINEER_OPS_ENABLED") != "true" {
			http.NotFound(w, r)
			return
		}
		user := os.Getenv("AGIPAREL_ENGINEER_OPS_USER")
		pass := os.Getenv("AGIPAREL_ENGINEER_OPS_PASSWORD")
		if strings.TrimSpace(user) == "" || len(pass) < 24 {
			http.Error(w, "Engineer Ops provisioning incomplete", http.StatusServiceUnavailable)
			return
		}
		host := r.Host
		if i := strings.LastIndex(host, ":"); i >= 0 && !strings.Contains(host, "]") {
			host = host[:i]
		}
		localHost := (host == "127.0.0.1" || host == "localhost" || host == "[::1]") && os.Getenv("ENV") == "development"
		trustedHTTPS := os.Getenv("AGIPAREL_ENGINEER_OPS_TRUST_PROXY_TLS") == "true" && r.Header.Get("X-Forwarded-Proto") == "https"
		if r.TLS == nil && !trustedHTTPS && !localHost {
			http.Error(w, "Engineer Ops requires HTTPS. If HTTPS terminates at a proxy, configure its sanitized X-Forwarded-Proto header and enable trusted proxy TLS in the application.", http.StatusForbidden)
			return
		}
		givenUser, givenPass, ok := r.BasicAuth()
		if !ok || !sameSecret(user, givenUser) || !sameSecret(pass, givenPass) {
			w.Header().Set("WWW-Authenticate", `Basic realm="Agiparel Engineer Ops", charset="UTF-8"`)
			http.Error(w, "Engineer Ops credentials required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type engineerOpsTable struct {
	Schema     string `json:"schema"`
	Name       string `json:"name"`
	RLSEnabled bool   `json:"rls_enabled"`
	RLSForced  bool   `json:"rls_forced"`
}
type engineerOpsDatabase struct {
	Status string `json:"status"`
	Name   string `json:"name,omitempty"`
	Role   string `json:"role,omitempty"`
}
type engineerOpsOverview struct {
	Release         string              `json:"release"`
	Environment     string              `json:"environment"`
	CheckedAt       string              `json:"checked_at"`
	ServiceStatus   string              `json:"service_status"`
	Database        engineerOpsDatabase `json:"database"`
	Modules         []string            `json:"modules"`
	Tables          []engineerOpsTable  `json:"tables"`
	TablesTruncated bool                `json:"tables_truncated"`
}

func registerEngineerOpsRoutes(mux *http.ServeMux, server *EdgeServer) {
	mux.HandleFunc("/engineer-ops", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := web.GetEngineerOpsHTML()
		if err != nil {
			http.Error(w, "Unavailable", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.Method == http.MethodGet {
			_, _ = w.Write(body)
		}
	})
	mux.HandleFunc("/engineer-ops/assets/ops.css", func(w http.ResponseWriter, r *http.Request) {
		body, err := web.GetEngineerOpsCSS()
		if err != nil {
			http.Error(w, "Unavailable", 500)
			return
		}
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		if r.Method == http.MethodGet {
			_, _ = w.Write(body)
		}
	})
	mux.HandleFunc("/engineer-ops/assets/ops.js", func(w http.ResponseWriter, r *http.Request) {
		body, err := web.GetEngineerOpsJS()
		if err != nil {
			http.Error(w, "Unavailable", 500)
			return
		}
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		if r.Method == http.MethodGet {
			_, _ = w.Write(body)
		}
	})
	mux.HandleFunc("/api/v1/engineer-ops/overview", func(w http.ResponseWriter, r *http.Request) {
		out := engineerOpsOverview{
			Release: os.Getenv("AGIPAREL_RELEASE"), Environment: os.Getenv("ENV"),
			CheckedAt: time.Now().UTC().Format(time.RFC3339), ServiceStatus: "UP",
			Database: engineerOpsDatabase{Status: "unavailable"}, Modules: []string{}, Tables: []engineerOpsTable{},
		}
		if out.Release == "" {
			out.Release = "v43 (revision not provisioned)"
		}
		if server.modules != nil {
			for m, on := range server.modules.enabled {
				if on {
					out.Modules = append(out.Modules, m)
				}
			}
			sort.Strings(out.Modules)
		}
		if server.db != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			defer cancel()
			// Application-role catalog only. No privileged cross-tenant reads, arbitrary SQL or password disclosure.
			var dbName, dbRole string
			if err := server.db.QueryRowContext(ctx, "SELECT current_database(),current_user").Scan(&dbName, &dbRole); err == nil {
				out.Database = engineerOpsDatabase{Status: "connected", Name: dbName, Role: dbRole}
				rows, err := server.db.QueryContext(ctx, `SELECT n.nspname,c.relname,c.relrowsecurity,c.relforcerowsecurity FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE c.relkind IN ('r','p') AND n.nspname IN ('public','s01_identity','s02_device') ORDER BY n.nspname,c.relname LIMIT 201`)
				if err == nil {
					defer rows.Close()
					for rows.Next() {
						var t engineerOpsTable
						if rows.Scan(&t.Schema, &t.Name, &t.RLSEnabled, &t.RLSForced) != nil {
							break
						}
						if len(out.Tables) == 200 {
							out.TablesTruncated = true
							break
						}
						out.Tables = append(out.Tables, t)
					}
				}
			}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(out)
	})
}
