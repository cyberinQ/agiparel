// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublicAccountAndAboutAccessibleWithoutOperatorPassword(t *testing.T) {
	mux := http.NewServeMux()
	registerPublicPortal(mux)
	registerPublicAccountRoutes(mux, nil)
	gate := operatorAccessGate(mux, "", "")
	for _, tc := range []struct{ path, contains string }{
		{"/", "Every garment"},
		{"/about", "Where we are today."},
		{"/login", "Account sign-in is being configured."},
		{"/register", "Self-registration is being configured."},
	} {
		req := httptest.NewRequest(http.MethodGet, "http://example.test"+tc.path, nil)
		rec := httptest.NewRecorder()
		gate.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), tc.contains) {
			t.Errorf("%s: code %d body: %.100s", tc.path, rec.Code, rec.Body.String())
		}
		if rec.Header().Get("WWW-Authenticate") != "" {
			t.Errorf("%s challenged user with basic authentication", tc.path)
		}
		if rec.Header().Get("X-Frame-Options") != "DENY" {
			t.Errorf("%s missing frame protection", tc.path)
		}
	}
	for _, route := range []string{"/about", "/login", "/register"} {
		req := httptest.NewRequest(http.MethodPost, "http://example.test"+route, nil)
		rec := httptest.NewRecorder()
		gate.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST %s returned %d", route, rec.Code)
		}
		req = httptest.NewRequest(http.MethodHead, "http://example.test"+route, nil)
		rec = httptest.NewRecorder()
		gate.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
			t.Errorf("HEAD %s: %d len=%d", route, rec.Code, rec.Body.Len())
		}
	}
	req := httptest.NewRequest(http.MethodGet, "http://example.test/stores", nil)
	rec := httptest.NewRecorder()
	gate.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("private API/page gate loosened: %d", rec.Code)
	}
}

func TestPublicAccountNeverAcceptsUntrustedEnrollmentRedirect(t *testing.T) {
	issuer := "https://auth.agiparel.empowerai.us/application/o/agiparel/"
	good := "https://auth.agiparel.empowerai.us/if/flow/enrollment/"
	if !safeHostedRegistrationURL(good, issuer) {
		t.Fatal("valid trusted flow rejected")
	}
	for _, bad := range []string{
		"http://auth.agiparel.empowerai.us/if/flow/enrollment/",
		"https://evil.example/if/flow/enrollment/",
		"https://auth.agiparel.empowerai.us.evil.example/if/flow/enrollment/",
		"https://auth.agiparel.empowerai.us/evil/flow/enrollment/",
		"https://auth.agiparel.empowerai.us/if/flow/enrollment/#fragment",
	} {
		if safeHostedRegistrationURL(bad, issuer) {
			t.Errorf("unsafe identity redirect permitted: %s", bad)
		}
	}
}

func TestPublicAccountConfiguredOIDCUsesProvider(t *testing.T) {
	b := &workforceBrowser{enabled: true, issuer: "https://auth.agiparel.empowerai.us/application/o/agiparel/"}
	t.Setenv("AGIPAREL_IDENTITY_V44_REGISTRATION_URL", "https://auth.agiparel.empowerai.us/if/flow/enrollment/")
	mux := http.NewServeMux()
	registerPublicAccountRoutes(mux, b)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "https://agiparel.empowerai.us/register", nil))
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "https://auth.agiparel.empowerai.us/if/flow/enrollment/" {
		t.Fatalf("register status=%d redirect=%s", w.Code, w.Header().Get("Location"))
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "https://agiparel.empowerai.us/login", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `href="/auth/login"`) {
		t.Fatal("login missing OIDC action")
	}
	t.Setenv("AGIPAREL_IDENTITY_V44_REGISTRATION_URL", "https://evil.example/if/flow/enrollment/")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "https://agiparel.empowerai.us/register", nil))
	if w.Code != http.StatusOK || w.Header().Get("Location") != "" {
		t.Fatal("untrusted enrollment URL was followed")
	}
}
