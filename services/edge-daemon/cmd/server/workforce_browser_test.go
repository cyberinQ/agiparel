// Author: Sunil Jain, 2026-10-28.
// Fail-closed SSO access, endpoint validation, cookie security and CSRF basics.
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBrowserSSOFailClosed(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("AGIPAREL_WORKFORCE_BROWSER_ENABLED", "true")
	b := newWorkforceBrowser(&EdgeServer{})
	if b.enabled {
		t.Fatal("browser auth enabled in development")
	}
	for _, handler := range []func(http.ResponseWriter, *http.Request){b.login, b.callbackHandler, b.logout, b.session} {
		w := httptest.NewRecorder()
		handler(w, httptest.NewRequest("GET", "https://edge.example.test/auth/login", nil))
		if w.Code != 503 && w.Code != 405 {
			t.Fatalf("disabled endpoint returned %d", w.Code)
		}
	}
}
func TestBrowserEndpointAndTokenValidation(t *testing.T) {
	for _, v := range []string{"https://idp.example.test/oauth2/authorize", "https://edge.example.test/auth/callback"} {
		if !safeEndpoint(v) {
			t.Fatal("valid HTTPS URL rejected", v)
		}
	}
	for _, v := range []string{"http://idp.example.test/authorize", "https://evil.example/path?redirect=foo", "https://user:pass@idp.example/authorize", "https://idp.example/authorize#frag", "https://idp.example/authorize\r\nX-Injection:1"} {
		if safeEndpoint(v) {
			t.Fatal("unsafe OAuth endpoint accepted", v)
		}
	}
	token, e := randomToken()
	if e != nil {
		t.Fatal(e)
	}
	if !checkOpaqueToken(token) || checkOpaqueToken(token+"=") || checkOpaqueToken(strings.Repeat("a", 43)) {
		t.Fatal("invalid opaque token bounds")
	}
	if string(hashToken(token)) == string(hashToken(token+"x")) {
		t.Fatal("broken session hashing")
	}
}
func TestLogoutRequiresCorrectOriginAndCSRF(t *testing.T) {
	b := &workforceBrowser{enabled: true, callback: "https://edge.example.test/auth/callback"}
	for _, origin := range []string{"", "https://attacker.example.test", "http://edge.example.test"} {
		req := httptest.NewRequest(http.MethodPost, "https://edge.example.test/auth/logout", nil)
		req.Header.Set("Origin", origin)
		rec := httptest.NewRecorder()
		b.logout(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("accepted origin %q: %d", origin, rec.Code)
		}
	}
}
