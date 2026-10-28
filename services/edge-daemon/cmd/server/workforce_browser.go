// Author: Sunil Jain, 2026-10-28.
// F1.2: opt-in, confidential OIDC code+PKCE login with PostgreSQL one-use states,
// revocable opaque server sessions and CSRF-protected logout. No production write authority.
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/workforceauth"
)

const oidcBindCookie = "__Host-agiparel-login"
const workforceCookie = "__Host-agiparel-session"

type workforceBrowser struct {
	db                                       *sql.DB
	verifier                                 *workforceauth.Verifier
	issuer, clientID, clientSecret, callback string
	authorizeURL, tokenURL                   string
	httpClient                               *http.Client
	enabled                                  bool
}

type sessionIdentity struct {
	Issuer, Subject string
	csrfHash        []byte
}

func safeEndpoint(raw string) bool {
	u, e := url.Parse(raw)
	return e == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.Fragment == "" && u.RawQuery == "" && u.Opaque == "" && u.Port() != "0" && !strings.ContainsAny(raw, " \r\n")
}
func newWorkforceBrowser(s *EdgeServer) *workforceBrowser {
	b := &workforceBrowser{}
	if os.Getenv("AGIPAREL_WORKFORCE_BROWSER_ENABLED") != "true" {
		return b
	}
	if s == nil || s.db == nil || (os.Getenv("ENV") != "production" && os.Getenv("ENV") != "staging") || os.Getenv("AGIPAREL_WORKFORCE_OIDC_ENABLED") != "true" {
		return b
	}
	issuer := os.Getenv("AGIPAREL_WORKFORCE_OIDC_ISSUER")
	clientID := os.Getenv("AGIPAREL_WORKFORCE_OIDC_CLIENT_ID")
	authorize := os.Getenv("AGIPAREL_WORKFORCE_OIDC_AUTHORIZATION_ENDPOINT")
	token := os.Getenv("AGIPAREL_WORKFORCE_OIDC_TOKEN_ENDPOINT")
	callback := os.Getenv("AGIPAREL_WORKFORCE_OIDC_CALLBACK_URL")
	secretPath := os.Getenv("AGIPAREL_WORKFORCE_OIDC_CLIENT_SECRET_FILE")
	if !safeEndpoint(authorize) || !safeEndpoint(token) || !safeEndpoint(callback) || len(clientID) < 3 || len(clientID) > 256 || secretPath == "" {
		return b
	}
	cb, e := url.Parse(callback)
	if e != nil || cb.Path != "/auth/callback" {
		return b
	}
	st, e := os.Stat(secretPath)
	if e != nil || st.Size() < 16 || st.Size() > 4096 {
		return b
	}
	raw, e := os.ReadFile(secretPath)
	if e != nil {
		return b
	}
	secret := strings.TrimSpace(string(raw))
	if len(secret) < 16 {
		return b
	}
	v, e := workforceauth.New(issuer, clientID, os.Getenv("AGIPAREL_WORKFORCE_OIDC_JWKS_FILE"))
	if e != nil {
		return b
	}
	b.db, b.verifier, b.issuer, b.clientID, b.clientSecret = s.db, v, issuer, clientID, secret
	b.authorizeURL, b.tokenURL, b.callback = authorize, token, callback
	b.httpClient = &http.Client{Timeout: 8 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	b.enabled = true
	return b
}
func (b *workforceBrowser) available(w http.ResponseWriter) bool {
	if b == nil || !b.enabled {
		docJSON(w, 503, map[string]string{"error": "ERR_WORKFORCE_BROWSER_DISABLED"})
		return false
	}
	return true
}
func randomToken() (string, error) {
	raw := make([]byte, 32)
	if _, e := rand.Read(raw); e != nil {
		return "", e
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
func hashToken(s string) []byte { sum := sha256.Sum256([]byte(s)); return sum[:] }
func checkOpaqueToken(s string) bool {
	if len(s) != 43 {
		return false
	}
	b, e := base64.RawURLEncoding.DecodeString(s)
	return e == nil && len(b) == 32 && base64.RawURLEncoding.EncodeToString(b) == s
}
func clearSecureCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
}
func (b *workforceBrowser) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	if !b.available(w) {
		return
	}
	state, e := randomToken()
	if e != nil {
		http.Error(w, "Login unavailable", 500)
		return
	}
	bind, e := randomToken()
	if e != nil {
		http.Error(w, "Login unavailable", 500)
		return
	}
	nonce, e := randomToken()
	if e != nil {
		http.Error(w, "Login unavailable", 500)
		return
	}
	verifier, e := randomToken()
	if e != nil {
		http.Error(w, "Login unavailable", 500)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if _, e = b.db.ExecContext(ctx, `SELECT workforce_oidc_begin_v16($1,$2,$3,$4)`, hashToken(state), hashToken(bind), nonce, verifier); e != nil {
		http.Error(w, "Login unavailable", 503)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: oidcBindCookie, Value: bind, Path: "/", MaxAge: 300, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
	challenge := sha256.Sum256([]byte(verifier))
	u, _ := url.Parse(b.authorizeURL)
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", b.clientID)
	q.Set("redirect_uri", b.callback)
	q.Set("scope", "openid")
	q.Set("state", state)
	q.Set("nonce", nonce)
	q.Set("code_challenge", base64.RawURLEncoding.EncodeToString(challenge[:]))
	q.Set("code_challenge_method", "S256")
	u.RawQuery = q.Encode()
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	http.Redirect(w, r, u.String(), http.StatusFound)
}
func (b *workforceBrowser) callbackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	if !b.available(w) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	cookie, e := r.Cookie(oidcBindCookie)
	clearSecureCookie(w, oidcBindCookie)
	state := r.URL.Query()["state"]
	code := r.URL.Query()["code"]
	if e != nil || !checkOpaqueToken(cookie.Value) || len(state) != 1 || !checkOpaqueToken(state[0]) || len(code) != 1 || len(code[0]) < 6 || len(code[0]) > 4096 || r.URL.Query().Get("error") != "" {
		http.Error(w, "Invalid login response", 401)
		return
	}
	var nonce, pkce string
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	err := b.db.QueryRowContext(ctx, `SELECT nonce,pkce_verifier FROM workforce_oidc_consume_v16($1,$2)`, hashToken(state[0]), hashToken(cookie.Value)).Scan(&nonce, &pkce)
	if err != nil {
		http.Error(w, "Expired or reused login", 401)
		return
	}
	form := url.Values{"grant_type": {"authorization_code"}, "code": {code[0]}, "redirect_uri": {b.callback}, "code_verifier": {pkce}}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, b.tokenURL, strings.NewReader(form.Encode()))
	if e != nil {
		http.Error(w, "Login unavailable", 503)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(url.QueryEscape(b.clientID), url.QueryEscape(b.clientSecret))
	resp, e := b.httpClient.Do(req)
	if e != nil {
		http.Error(w, "Identity provider unavailable", 503)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "Identity verification failed", 401)
		return
	}
	var tokens struct {
		IDToken string `json:"id_token"`
	}
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 32769))
	if readErr != nil || len(body) > 32768 || json.Unmarshal(body, &tokens) != nil || len(tokens.IDToken) > 8192 {
		http.Error(w, "Identity verification failed", 401)
		return
	}
	identity, e := b.verifier.VerifyIDToken(tokens.IDToken, nonce)
	if e != nil {
		http.Error(w, "Identity verification failed", 401)
		return
	}
	session, e := randomToken()
	if e != nil {
		http.Error(w, "Login unavailable", 500)
		return
	}
	csrf, e := randomToken()
	if e != nil {
		http.Error(w, "Login unavailable", 500)
		return
	}
	if _, e = b.db.ExecContext(ctx, `SELECT workforce_session_create_v16($1,$2,$3,$4)`, hashToken(session), hashToken(csrf), identity.Issuer, identity.Subject); e != nil {
		http.Error(w, "Login unavailable", 503)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: workforceCookie, Value: session, Path: "/", MaxAge: 8 * 3600, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/intelligence", http.StatusSeeOther)
}
func (b *workforceBrowser) identity(r *http.Request) (sessionIdentity, error) {
	if b == nil || !b.enabled {
		return sessionIdentity{}, errors.New("ERR_SESSION_DISABLED")
	}
	c, e := r.Cookie(workforceCookie)
	if e != nil || !checkOpaqueToken(c.Value) {
		return sessionIdentity{}, errors.New("ERR_SESSION_MISSING")
	}
	var id sessionIdentity
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	e = b.db.QueryRowContext(ctx, `SELECT idp_issuer,idp_subject,csrf_hash FROM workforce_session_get_v16($1)`, hashToken(c.Value)).Scan(&id.Issuer, &id.Subject, &id.csrfHash)
	if e != nil || id.Issuer != b.issuer || len(id.Subject) == 0 || len(id.Subject) > 128 {
		return sessionIdentity{}, errors.New("ERR_SESSION_INVALID")
	}
	return id, nil
}
func (b *workforceBrowser) session(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	if !b.available(w) {
		return
	}
	id, e := b.identity(r)
	w.Header().Set("Cache-Control", "no-store")
	if e != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_SESSION_UNAUTHENTICATED"})
		return
	}
	// CSRF tokens cannot be recovered from a hash. Supply a separate, signed
	// double-submit cookie? Instead issue a fresh CSRF token through a DB rotation
	// routine in the session endpoint (see rotateCSRF below).
	csrf, e := b.rotateCSRF(r)
	if e != nil {
		docJSON(w, 503, map[string]string{"error": "ERR_SESSION_CSRF"})
		return
	}
	docJSON(w, 200, map[string]any{"authenticated": true, "issuer": id.Issuer, "subject": id.Subject, "csrf": csrf})
}
func (b *workforceBrowser) rotateCSRF(r *http.Request) (string, error) {
	c, e := r.Cookie(workforceCookie)
	if e != nil || !checkOpaqueToken(c.Value) {
		return "", errors.New("invalid session")
	}
	csrf, e := randomToken()
	if e != nil {
		return "", e
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	// Approved single-session rotation is an opaque security-definer operation.
	var ok bool
	e = b.db.QueryRowContext(ctx, `SELECT workforce_session_csrf_rotate_v16($1,$2)`, hashToken(c.Value), hashToken(csrf)).Scan(&ok)
	if e != nil || !ok {
		return "", errors.New("session invalid")
	}
	return csrf, nil
}
func (b *workforceBrowser) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", 405)
		return
	}
	if !b.available(w) {
		return
	}
	origin := r.Header.Get("Origin")
	u, e := url.Parse(origin)
	cb, _ := url.Parse(b.callback)
	if e != nil || origin == "" || u.Scheme != "https" || u.Host != cb.Host || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		http.Error(w, "CSRF origin rejected", 403)
		return
	}
	id, e := b.identity(r)
	if e != nil {
		http.Error(w, "Session missing", 401)
		return
	}
	token := r.Header.Get("X-Agiparel-CSRF")
	if !checkOpaqueToken(token) || subtle.ConstantTimeCompare(hashToken(token), id.csrfHash) != 1 {
		http.Error(w, "CSRF proof required", 403)
		return
	}
	c, _ := r.Cookie(workforceCookie)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if _, e = b.db.ExecContext(ctx, `SELECT workforce_session_revoke_v16($1)`, hashToken(c.Value)); e != nil {
		http.Error(w, "Logout unavailable", 503)
		return
	}
	clearSecureCookie(w, workforceCookie)
	w.Header().Set("Cache-Control", "no-store")
	docJSON(w, 200, map[string]bool{"logged_out": true})
}
func registerWorkforceBrowserRoutes(mux *http.ServeMux, b *workforceBrowser) {
	mux.HandleFunc("/auth/login", b.login)
	mux.HandleFunc("/auth/callback", b.callbackHandler)
	mux.HandleFunc("/auth/logout", b.logout)
	mux.HandleFunc("/api/v1/workforce/session", b.session)
}
