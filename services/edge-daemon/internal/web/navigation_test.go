package web

import (
	"strings"
	"testing"
)

// Gate 06.5 UX: every operator page must offer a consistent way home and
// every generalized business-document upload must enter the source library.
func TestRestrictedPagesLoadSharedNavigation(t *testing.T) {
	for _, page := range []string{
		"factory_setup.html", "documents.html", "order_intake.html", "material_planning.html",
		"plant_view.html", "onboarding.html", "cutting_view.html", "sew01.html",
	} {
		t.Run(page, func(t *testing.T) {
			raw, err := staticFS.ReadFile("static/" + page)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(raw), `src="/static/workspace_nav.js"`) {
				t.Fatalf("%s is missing the shared navigation", page)
			}
		})
	}
	raw, err := staticFS.ReadFile("static/workspace_nav.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"'/'", "'/factory-setup'", "'/documents'", "'/order-intake'", "'/material-planning'", "'/planning'", "'/plant'", "'/cutting'", "'/sew01'", "'/onboarding'"} {
		if !strings.Contains(string(raw), path) {
			t.Errorf("navigation missing %s", path)
		}
	}
}

func TestGeneralUploadsUseCentralDocumentLibrary(t *testing.T) {
	for _, page := range []string{"plant_view.html", "onboarding.html"} {
		raw, err := staticFS.ReadFile("static/" + page)
		if err != nil {
			t.Fatal(err)
		}
		body := string(raw)
		if !strings.Contains(body, `href="/documents"`) {
			t.Errorf("%s missing library link", page)
		}
		if strings.Contains(body, `onchange="uploadCustomerDocuments(this)"`) {
			t.Errorf("%s still exposes its old general uploader", page)
		}
		if page == "onboarding.html" && !strings.Contains(body, "The old CSV-to-factory importer has been retired.") {
			t.Errorf("%s must clearly state that the legacy importer is retired", page)
		}
	}
	// Specialist CSV inputs are not removed before Gate 06.5-C normalizes PDFs.
	for _, page := range []string{"order_intake.html", "material_planning.html"} {
		raw, err := staticFS.ReadFile("static/" + page)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), `.csv`) || !strings.Contains(string(raw), `href="/documents"`) {
			t.Errorf("%s must retain CSV functionality and point to central intake", page)
		}
	}
}
