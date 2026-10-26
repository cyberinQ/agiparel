package web

import (
	"strings"
	"testing"
)

func TestFactoryReferenceCatalogUX(t *testing.T) {
	samples := map[string][]string{
		"static/factory_setup.html":   {`/static/factory_reference.js`, `id="factory-references"`, `id="reference-ready"`, `id="reference-current"`, `id="reference-history-toggle"`, `href="/documents"`},
		"static/factory_reference.js": {`/api/v1/factory-onboarding/reference-masters`, `PUBLISH`, `WITHDRAW`, `draft_revision`, `expected_revision`, `acknowledged`, `rate_basis`, `/documents?document=`},
	}
	for path, wants := range samples {
		raw, err := staticFS.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		body := string(raw)
		for _, want := range wants {
			if !strings.Contains(body, want) {
				t.Errorf("%s lacks %q", path, want)
			}
		}
		if strings.HasSuffix(path, ".js") && strings.Contains(body, "innerHTML") {
			t.Errorf("unsafe innerHTML in %s", path)
		}
	}
}
