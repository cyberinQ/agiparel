// Author: Sunil Jain, 2026-11-12.
// Opt-in public, browser-memory-only Cutter Mode. This grants no operator API or production authority.
package main

import (
	"net/http"
	"os"
	"strings"

	"github.com/cyberinq/agiparel/services/edge-daemon/internal/web"
)

func cutterPublicEnabled() bool {
	return os.Getenv("AGIPAREL_PUBLIC_CUTTER_DEMO_ENABLED") == "true"
}

func servePublicCutterDemo(w http.ResponseWriter, r *http.Request) {
	if !cutterPublicEnabled() {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data, err := web.GetCutterDemoHTML()
	if err != nil {
		http.Error(w, "Training unavailable", http.StatusServiceUnavailable)
		return
	}
	page := string(data)
	page = strings.Replace(page, `<body>`, `<body data-demo-mode="public">`, 1)
	page = strings.ReplaceAll(page, `/static/cutter_demo.css`, `/demo-assets/cutter.css`)
	page = strings.ReplaceAll(page, `/static/cutter_demo_model.js`, `/demo-assets/cutter-model.js`)
	page = strings.ReplaceAll(page, `/static/cutter_demo.js`, `/demo-assets/cutter.js`)
	page = strings.ReplaceAll(page, `href="/cutting/sop2"`, `href="/"`)
	page = strings.ReplaceAll(page, `Manager control tower`, `AGIPAREL home`)
	page = strings.ReplaceAll(page, `Localhost training only`, `Public browser-only training simulation`)
	portalHeaders(w)
	// This training UI uses inline width styles; scripts remain external and connections are disabled.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'none'; img-src 'self' data:; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write([]byte(page))
	}
}

func servePublicCutterAsset(w http.ResponseWriter, r *http.Request) {
	if !cutterPublicEnabled() {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var data []byte
	var err error
	var typ string
	switch r.URL.Path {
	case "/demo-assets/cutter.css":
		data, err = web.GetCutterDemoCSS()
		typ = "text/css; charset=utf-8"
	case "/demo-assets/cutter-model.js":
		data, err = web.GetCutterDemoModelJS()
		typ = "text/javascript; charset=utf-8"
	case "/demo-assets/cutter.js":
		data, err = web.GetCutterDemoJS()
		typ = "text/javascript; charset=utf-8"
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Training asset unavailable", http.StatusServiceUnavailable)
		return
	}
	portalHeaders(w)
	w.Header().Set("Content-Type", typ)
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write(data)
	}
}
