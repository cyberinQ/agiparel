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

// Batch 06: restricted surplus-first, source-backed material planning scenarios.
func GetMaterialPlanningHTML() ([]byte, error) {
	return staticFS.ReadFile("static/material_planning.html")
}

// Gate 06.5-A: original document library (restricted operator screen).
func GetDocumentsHTML() ([]byte, error) { return staticFS.ReadFile("static/documents.html") }

// Gate 06.5-B1: factory discovery before buyer order intake.
func GetFactorySetupHTML() ([]byte, error) { return staticFS.ReadFile("static/factory_setup.html") }

// Gate 06.5-C1: original-document-to-proposed-order review.
func GetBuyerReviewHTML() ([]byte, error) { return staticFS.ReadFile("static/buyer_review.html") }

// Batch 07A: restricted stores observation / roll identity workspace.
func GetStoresHTML() ([]byte, error) { return staticFS.ReadFile("static/stores.html") }

// E09 browser-based station scanner emulation (read-only embedded assets).
func GetSewSimulationHTML() ([]byte, error) { return staticFS.ReadFile("static/sewing_execution.html") }

func GetQCSimulationHTML() ([]byte, error) { return staticFS.ReadFile("static/quality_execution.html") }

func GetPackSimulationHTML() ([]byte, error) {
	return staticFS.ReadFile("static/packing_execution.html")
}

// Batch 12A/B: restricted factory intelligence rehearsal.
func GetIntelSimulationHTML() ([]byte, error) {
	return staticFS.ReadFile("static/factory_intelligence.html")
}

// Author: Sunil Jain, 2026-10-28. Restricted production evidence HMI.
func GetIntelProductionHTML() ([]byte, error) {
	return staticFS.ReadFile("static/factory_intelligence_production.html")
}

// Author: Sunil Jain, CyberinQ LLC, 2026-10-28.
func GetCommissioningHTML() ([]byte, error) {
	return staticFS.ReadFile("static/factory_commissioning.html")
}

// Author: Sunil Jain, CyberinQ LLC, 2026-10-28. F3.9 operator station.
func GetOperatorStationHTML() ([]byte, error) {
	return staticFS.ReadFile("static/factory_operator_station.html")
}

// v43: Dedicated authenticated engineer-ops UI. Never served by the public assets handler.
func GetEngineerOpsHTML() ([]byte, error) { return staticFS.ReadFile("static/engineer_ops.html") }
func GetEngineerOpsCSS() ([]byte, error)  { return staticFS.ReadFile("static/engineer_ops.css") }
func GetEngineerOpsJS() ([]byte, error)   { return staticFS.ReadFile("static/engineer_ops.js") }
