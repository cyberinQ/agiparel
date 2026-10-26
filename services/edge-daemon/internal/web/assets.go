package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/*
var staticFS embed.FS

func StaticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}

func GetSEW01HTML() ([]byte, error) {
	return staticFS.ReadFile("static/sew01.html")
}

func GetPlantViewHTML() ([]byte, error) {
	return staticFS.ReadFile("static/plant_view.html")
}

func GetOnboardingHTML() ([]byte, error) {
	return staticFS.ReadFile("static/onboarding.html")
}

func GetCuttingViewHTML() ([]byte, error) {
	return staticFS.ReadFile("static/cutting_view.html")
}

// Batch 04: isolated public assets. The existing StaticHandler remains private
// behind the operator gate, so embedded HMI HTML is never exposed as demo assets.
func GetPortalHTML() ([]byte, error) { return staticFS.ReadFile("static/portal.html") }
func GetPortalCSS() ([]byte, error)  { return staticFS.ReadFile("static/portal.css") }
func GetPortalJS() ([]byte, error)   { return staticFS.ReadFile("static/portal.js") }

// Batch 05: restricted, source-backed buyer PO/BOM staging workspace.
func GetOrderIntakeHTML() ([]byte, error) { return staticFS.ReadFile("static/order_intake.html") }
