// Author: CyberinQ, 2026-10-26 — Read-only public demonstration and operator boundary.
package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/cyberinq/agiparel/services/edge-daemon/internal/web"
)

// operatorAccessGate is a temporary, single-operator access barrier for the
// brownfield development server. This is NOT workforce identity, MFA, tenant
// authorization, or approval of any real production material transaction.
// The public demo must never invoke privileged APIs or disclose real factory data.
func operatorAccessGate(next http.Handler, user, password string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/", "/demo", "/demo-assets/portal.css", "/demo-assets/portal.js", "/healthz":
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// Fail closed if the deployment forgot to supply real credentials.
		if user == "" || len(password) < 4 {
			http.Error(w, "Operator access is not configured", http.StatusServiceUnavailable)
			return
		}
		suppliedUser, suppliedPassword, ok := r.BasicAuth()
		if !ok || !sameSecret(user, suppliedUser) || !sameSecret(password, suppliedPassword) {
			w.Header().Set("WWW-Authenticate", `Basic realm="Agiparel Factory Operations", charset="UTF-8"`)
			http.Error(w, "Operator authentication required", http.StatusUnauthorized)
			return
		}
		// Browsers can resend Basic credentials to a malicious form. Reject
		// cross-site writes before reaching the legacy operational handlers.
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				http.Error(w, "Cross-site operation rejected", http.StatusForbidden)
				return
			}
			if origin := r.Header.Get("Origin"); origin != "" {
				u, err := url.Parse(origin)
				if err != nil || !strings.EqualFold(u.Host, r.Host) || (u.Scheme != "https" && u.Scheme != "http") {
					http.Error(w, "Cross-site operation rejected", http.StatusForbidden)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func sameSecret(expected, supplied string) bool {
	a := sha256.Sum256([]byte(expected))
	b := sha256.Sum256([]byte(supplied))
	return subtle.ConstantTimeCompare(a[:], b[:]) == 1
}

func portalHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'self'; script-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
}

func servePublicPortal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	portalHeaders(w)
	data, err := web.GetPortalHTML()
	if err != nil {
		http.Error(w, "Portal unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write(data)
	}
}

func servePublicPortalAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	portalHeaders(w)
	var data []byte
	var err error
	switch r.URL.Path {
	case "/demo-assets/portal.css":
		data, err = web.GetPortalCSS()
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case "/demo-assets/portal.js":
		data, err = web.GetPortalJS()
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Asset unavailable", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write(data)
	}
}

func registerPublicPortal(mux *http.ServeMux) {
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		servePublicPortal(w, r)
	})
	mux.HandleFunc("/demo", servePublicPortal)
	mux.HandleFunc("/demo-assets/portal.css", servePublicPortalAsset)
	mux.HandleFunc("/demo-assets/portal.js", servePublicPortalAsset)
}

func configuredOperatorGate(next http.Handler) http.Handler {
	// Dedicated ingress uses enrolled Ed25519 reader keys or strong, scoped IdP
	// assertions. A Basic Auth challenge would prevent hardware clients while
	// providing no identity assurance. Never exempt generic /api paths.
	secureIngress := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/sewing/production/observations", "/api/v1/sewing/production/completions", "/api/v1/sewing/production/completions/challenge",
			"/api/v1/quality/garments/challenge", "/api/v1/quality/garments/events", "/api/v1/quality/garments/history", "/api/v1/quality/garments/specification",
			"/api/v1/quality/production/events/challenge", "/api/v1/quality/production/events", "/api/v1/quality/production/catalog",
			"/api/v1/packing/units/challenge", "/api/v1/packing/units/events", "/api/v1/packing/manifests/challenge", "/api/v1/packing/manifests/events", "/api/v1/packing/genealogy", "/api/v1/packing/specification",
			"/api/v1/packing/production/holds/challenge", "/api/v1/packing/production/holds", "/api/v1/packing/production/readiness",
			"/api/v1/intelligence/production/report",
			"/api/v1/genealogy/units/challenge", "/api/v1/genealogy/units",
			"/api/v1/genealogy/carriers/challenge", "/api/v1/genealogy/carriers/events", "/api/v1/genealogy/carriers/history",
			"/api/v1/cutting/production/reconciliation/challenge", "/api/v1/cutting/production/reconciliation/events", "/api/v1/cutting/production/reconciliation/balance", "/api/v1/cutting/production/reconciliation/defects",
			"/api/v1/jobwork/parts/outward/challenge", "/api/v1/jobwork/parts/outward", "/api/v1/jobwork/parts/inward/challenge", "/api/v1/jobwork/parts/inward", "/api/v1/jobwork/parts/status", "/api/v1/jobwork/reconciliation", "/api/v1/jobwork/discrepancies/challenge", "/api/v1/jobwork/discrepancies", "/api/v1/jobwork/vendor-ack/challenge", "/api/v1/jobwork/vendor-ack", "/api/v1/fabric/shrinkage/challenge", "/api/v1/fabric/shrinkage/observations", "/api/v1/fabric/shrinkage/readiness", "/api/v1/jobwork/dispositions/challenge", "/api/v1/jobwork/dispositions",
			"/api/v1/factory/evidence/challenge", "/api/v1/factory/evidence/events", "/api/v1/quality/aql/sampling", "/api/v1/quality/needle/incidents",
			"/api/v1/quality/safety/clearances/challenge", "/api/v1/quality/safety/clearances", "/api/v1/quality/aql/reviews/challenge", "/api/v1/quality/aql/reviews", "/api/v1/packing/dock/readiness",
			"/api/v1/packing/dock/scans", "/api/v1/packing/dock/verification", "/api/v1/packing/dock/external-assertions", "/api/v1/packing/dock/external-verification", "/api/v1/packing/dock/diagnostics", "/api/v1/packing/dock/seals", "/api/v1/packing/dock/seals/challenge",
			"/api/v1/packing/dock/carrier-receipts", "/api/v1/packing/dock/carrier-receipts/challenge", "/api/v1/packing/dock/asn-drafts", "/api/v1/packing/dock/asn-drafts/challenge",
			"/api/v1/wip/pieces/challenge", "/api/v1/wip/pieces/events", "/api/v1/wip/pieces/groups",
			"/api/v1/sewing/pieces/completions/challenge", "/api/v1/sewing/pieces/completions",
			"/api/v1/wip/exceptions/challenge", "/api/v1/wip/exceptions/events", "/api/v1/wip/exceptions/cases",
			"/api/v1/wip/production/scans", "/api/v1/wip/production/movements/challenge", "/api/v1/wip/production/movements", "/api/v1/wip/production/history", "/api/v1/wip/production/metrics",
			"/auth/login", "/auth/callback", "/auth/logout", "/api/v1/workforce/session", "/api/v1/workforce/chapter-access", "/api/v1/commissioning/stations", "/api/v1/operator-station/inbox":
			next.ServeHTTP(w, r)
		case "/commissioning", "/static/factory_commissioning.js", "/operator-station", "/static/factory_operator_station.js", "/static/factory_wip_exceptions.js", "/intelligence", "/static/ux_unified.css", "/static/chapter_workflow.css",
			"/static/factory_intelligence.css", "/static/factory_intelligence_production.js":
			if r.Method == http.MethodGet || r.Method == http.MethodHead {
				next.ServeHTTP(w, r)
				return
			}
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		default:
			operatorAccessGate(next, "", "").ServeHTTP(w, r)
		}
	})
	if os.Getenv("ENV") != "development" {
		return secureIngress
	}
	// A shared Basic credential is acceptable only for a disposable DEV HMI.
	// Production must use a tenant-scoped workforce identity provider instead.
	return operatorAccessGate(next, os.Getenv("AGIPAREL_OPERATOR_USER"), os.Getenv("AGIPAREL_OPERATOR_PASSWORD"))
}
