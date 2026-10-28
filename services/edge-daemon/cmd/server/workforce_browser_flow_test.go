// Author: Sunil Jain, 2026-10-28.
// Full F1.2 protocol loop against a fake, TLS-protected IdP and an in-memory SQL driver.
// Does not substitute for staging PostgreSQL/RLS integration tests.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/workforceauth"
)

type fakeSessionDB struct {
	sync.Mutex
	state, browser  []byte
	nonce, pkce     string
	consumed        bool
	session, csrf   []byte
	issuer, subject string
	revoked         bool
}
type fakeSQLConnector struct{ store *fakeSessionDB }

func (c fakeSQLConnector) Connect(context.Context) (driver.Conn, error) {
	return &fakeSQLConn{c.store}, nil
}
func (c fakeSQLConnector) Driver() driver.Driver { return fakeSQLDriver{c.store} }

type fakeSQLDriver struct{ store *fakeSessionDB }

func (d fakeSQLDriver) Open(string) (driver.Conn, error) { return &fakeSQLConn{d.store}, nil }

type fakeSQLConn struct{ store *fakeSessionDB }

func (c *fakeSQLConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepared statements disabled")
}
func (c *fakeSQLConn) Close() error              { return nil }
func (c *fakeSQLConn) Begin() (driver.Tx, error) { return nil, errors.New("transactions disabled") }
func (c *fakeSQLConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	s := c.store
	s.Lock()
	defer s.Unlock()
	switch {
	case strings.Contains(q, "workforce_oidc_begin_v16"):
		s.state = append([]byte(nil), args[0].Value.([]byte)...)
		s.browser = append([]byte(nil), args[1].Value.([]byte)...)
		s.nonce = args[2].Value.(string)
		s.pkce = args[3].Value.(string)
		return driver.RowsAffected(1), nil
	case strings.Contains(q, "workforce_session_create_v16"):
		s.session = append([]byte(nil), args[0].Value.([]byte)...)
		s.csrf = append([]byte(nil), args[1].Value.([]byte)...)
		s.issuer = args[2].Value.(string)
		s.subject = args[3].Value.(string)
		return driver.RowsAffected(1), nil
	case strings.Contains(q, "workforce_session_revoke_v16"):
		s.revoked = true
		return driver.RowsAffected(1), nil
	}
	return nil, fmt.Errorf("unexpected SQL exec %s", q)
}

type fakeRows struct {
	cols []string
	vals [][]driver.Value
}

func (r *fakeRows) Columns() []string { return r.cols }
func (r *fakeRows) Close() error      { return nil }
func (r *fakeRows) Next(dest []driver.Value) error {
	if len(r.vals) == 0 {
		return io.EOF
	}
	copy(dest, r.vals[0])
	r.vals = r.vals[1:]
	return nil
}
func (c *fakeSQLConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	s := c.store
	s.Lock()
	defer s.Unlock()
	switch {
	case strings.Contains(q, "workforce_oidc_consume_v16"):
		r := &fakeRows{cols: []string{"nonce", "pkce_verifier"}}
		if !s.consumed && string(args[0].Value.([]byte)) == string(s.state) && string(args[1].Value.([]byte)) == string(s.browser) {
			r.vals = [][]driver.Value{{s.nonce, s.pkce}}
			s.consumed = true
		}
		return r, nil
	case strings.Contains(q, "workforce_session_get_v16"):
		r := &fakeRows{cols: []string{"idp_issuer", "idp_subject", "csrf_hash"}}
		if !s.revoked && string(args[0].Value.([]byte)) == string(s.session) {
			r.vals = [][]driver.Value{{s.issuer, s.subject, s.csrf}}
		}
		return r, nil
	case strings.Contains(q, "workforce_session_csrf_rotate_v16"):
		ok := !s.revoked && string(args[0].Value.([]byte)) == string(s.session)
		if ok {
			s.csrf = append([]byte(nil), args[1].Value.([]byte)...)
		}
		return &fakeRows{cols: []string{"ok"}, vals: [][]driver.Value{{ok}}}, nil
	}
	return nil, fmt.Errorf("unexpected SQL query %s", q)
}
func TestBrowserOIDCFullRoundTrip(t *testing.T) {
	pub, priv, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	jwks, _ := json.Marshal(map[string]any{"keys": []any{map[string]any{"kty": "OKP", "kid": "ssokey", "alg": "EdDSA", "use": "sig", "crv": "Ed25519", "x": base64.RawURLEncoding.EncodeToString(pub)}}})
	path := filepath.Join(t.TempDir(), "jwks.json")
	if e = os.WriteFile(path, jwks, 0600); e != nil {
		t.Fatal(e)
	}
	verifier, e := workforceauth.New("https://idp.example.test", "factory-browser", path)
	if e != nil {
		t.Fatal(e)
	}
	store := &fakeSessionDB{}
	db := sql.OpenDB(fakeSQLConnector{store})
	defer db.Close()
	db.SetMaxOpenConns(1)
	var expectedNonce string
	var expectedChallenge string
	fakeIDP := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" || r.Method != http.MethodPost {
			t.Error("wrong IdP endpoint")
			http.Error(w, "bad endpoint", 400)
			return
		}
		if e := r.ParseForm(); e != nil {
			t.Error(e)
			return
		}
		if r.PostForm.Get("code_verifier") == "" || r.PostForm.Get("grant_type") != "authorization_code" {
			t.Error("missing PKCE")
			return
		}
		h := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
		if base64.RawURLEncoding.EncodeToString(h[:]) != expectedChallenge {
			t.Error("PKCE challenge mismatch")
			return
		}
		now := time.Now().UTC()
		header, _ := json.Marshal(map[string]string{"kid": "ssokey", "alg": "EdDSA", "typ": "JWT"})
		claims, _ := json.Marshal(map[string]any{"iss": "https://idp.example.test", "aud": "factory-browser", "sub": "worker_7", "nonce": expectedNonce, "amr": []string{"mfa"}, "iat": now.Unix(), "exp": now.Add(5 * time.Minute).Unix()})
		signed := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
		token := signed + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(priv, []byte(signed)))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"id_token": token})
	}))
	defer fakeIDP.Close()
	b := &workforceBrowser{db: db, verifier: verifier, issuer: "https://idp.example.test", clientID: "factory-browser", clientSecret: "confidential-secret-value", callback: "https://edge.example.test/auth/callback", authorizeURL: "https://idp.example.test/authorize", tokenURL: fakeIDP.URL + "/token", httpClient: fakeIDP.Client(), enabled: true}
	login := httptest.NewRecorder()
	b.login(login, httptest.NewRequest("GET", "https://edge.example.test/auth/login", nil))
	if login.Code != 302 {
		t.Fatal("login not redirected", login.Code, login.Body.String())
	}
	authURL, e := url.Parse(login.Header().Get("Location"))
	if e != nil {
		t.Fatal(e)
	}
	expectedNonce = authURL.Query().Get("nonce")
	expectedChallenge = authURL.Query().Get("code_challenge")
	if authURL.Query().Get("code_challenge_method") != "S256" || len(expectedNonce) < 32 {
		t.Fatal("missing PKCE or nonce")
	}
	var binding *http.Cookie
	for _, c := range login.Result().Cookies() {
		if c.Name == oidcBindCookie {
			binding = c
		}
	}
	if binding == nil || !binding.Secure || !binding.HttpOnly {
		t.Fatal("insecure login browser binder")
	}
	cb := httptest.NewRequest("GET", "https://edge.example.test/auth/callback?state="+url.QueryEscape(authURL.Query().Get("state"))+"&code=valid-authorization-code", nil)
	cb.AddCookie(binding)
	callback := httptest.NewRecorder()
	b.callbackHandler(callback, cb)
	if callback.Code != 303 {
		t.Fatal("callback failed", callback.Code, callback.Body.String())
	}
	var sessionCookie *http.Cookie
	for _, c := range callback.Result().Cookies() {
		if c.Name == workforceCookie && c.MaxAge > 0 {
			sessionCookie = c
		}
	}
	if sessionCookie == nil || !sessionCookie.HttpOnly || !sessionCookie.Secure || sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Fatal("missing secure session cookie")
	}
	duplicate := httptest.NewRecorder()
	b.callbackHandler(duplicate, cb)
	if duplicate.Code != 401 {
		t.Fatal("authorization state replay succeeded")
	}
	sessionReq := httptest.NewRequest("GET", "https://edge.example.test/api/v1/workforce/session", nil)
	sessionReq.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	b.session(rec, sessionReq)
	if rec.Code != 200 {
		t.Fatal("session lookup failed", rec.Code, rec.Body.String())
	}
	var detail struct {
		Authenticated bool   `json:"authenticated"`
		CSRF          string `json:"csrf"`
		Subject       string `json:"subject"`
	}
	if e = json.Unmarshal(rec.Body.Bytes(), &detail); e != nil || !detail.Authenticated || detail.Subject != "worker_7" || !checkOpaqueToken(detail.CSRF) {
		t.Fatal("bad authenticated session", e)
	}
	logoutReq := httptest.NewRequest("POST", "https://edge.example.test/auth/logout", nil)
	logoutReq.AddCookie(sessionCookie)
	logoutReq.Header.Set("Origin", "https://edge.example.test")
	bad := httptest.NewRecorder()
	b.logout(bad, logoutReq)
	if bad.Code != 403 {
		t.Fatal("logout without CSRF allowed")
	}
	logoutReq.Header.Set("X-Agiparel-CSRF", detail.CSRF)
	good := httptest.NewRecorder()
	b.logout(good, logoutReq)
	if good.Code != 200 {
		t.Fatal("logout failed", good.Code, good.Body.String())
	}
	after := httptest.NewRecorder()
	b.session(after, sessionReq)
	if after.Code != 401 {
		t.Fatal("revoked session remains usable")
	}
}
