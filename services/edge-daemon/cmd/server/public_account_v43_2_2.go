// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// Public account entry pages. Actual registration/login remains with a configured OIDC provider.
package main

import (
	"html"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func safeHostedRegistrationURL(raw, issuer string) bool {
	if !safeEndpoint(raw) || !safeEndpoint(issuer) {
		return false
	}
	target, err := url.Parse(raw)
	if err != nil {
		return false
	}
	source, err := url.Parse(issuer)
	if err != nil {
		return false
	}
	return target.Scheme == "https" && strings.EqualFold(target.Host, source.Host) && target.User == nil && target.Fragment == "" && strings.HasPrefix(target.Path, "/if/flow/")
}

func servePublicAccountPage(w http.ResponseWriter, r *http.Request, page string, b *workforceBrowser) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// A configured provider is the only place that accepts credentials or registration data.
	ready := b != nil && b.enabled
	if page == "register" && ready {
		registration := os.Getenv("AGIPAREL_IDENTITY_V44_REGISTRATION_URL")
		if safeHostedRegistrationURL(registration, b.issuer) {
			http.Redirect(w, r, registration, http.StatusSeeOther)
			return
		}
	}
	title, lead := "Sign in", "Sign in with your Agiparel account to explore the factory chapters."
	notice := "Account sign-in is being configured. The existing factory demo may still request temporary operator access. Do not enter passwords on this page."
	actions := `<a href="/register">Create an account</a><a href="/">Factory Home</a>`
	if page == "register" {
		title, lead = "Create an account", "One account for Agiparel factory chapters and your authorized factory access."
		notice = "Self-registration is being configured. An account cannot be created here until the secure registration service is connected."
		actions = `<a href="/login">Sign in instead</a><a href="/">Factory Home</a>`
		if ready {
			notice = "Registration is awaiting its verified enrollment flow. Existing accounts can still sign in."
		}
	} else if ready {
		notice = "Your password is entered only at the configured identity provider, not on this page."
		actions = `<a class="primary" href="/auth/login">Continue securely</a><a href="/register">Create an account</a>`
	}
	portalHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodHead {
		return
	}
	content := `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>` + html.EscapeString(title) + ` | Agiparel</title><link rel="stylesheet" href="/demo-assets/portal.css"></head><body>` +
		`<header class="site-top"><a href="/" class="brand"><span class="brand-mark" aria-hidden="true"></span>agiparel</a><nav class="site-links" aria-label="Main navigation"><a class="site-link" href="/about">About</a><a class="site-link" href="/login">Sign in</a></nav></header>` +
		`<main class="account-main"><section class="account-panel account-intro"><p class="eyebrow">AGIPAREL ACCOUNT</p><h1>` + html.EscapeString(title) + `</h1><p>` + html.EscapeString(lead) + `</p><div class="account-status" role="status">` + html.EscapeString(notice) + `</div><nav class="account-actions" aria-label="Account actions">` + actions + `</nav></section></main>` +
		`<footer><strong>agiparel</strong><span>Empowered by CyberinQ.</span><span>Public home · No customer records</span><a href="/engineer-ops">Engineer Ops</a></footer></body></html>`
	_, _ = w.Write([]byte(content))
}

func registerPublicAccountRoutes(mux *http.ServeMux, b *workforceBrowser) {
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) { servePublicAccountPage(w, r, "login", b) })
	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) { servePublicAccountPage(w, r, "register", b) })
}
