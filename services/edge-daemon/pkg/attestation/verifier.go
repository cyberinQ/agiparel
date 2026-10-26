// Author: Sunil Jain, CyberinQ LLC, 2026-10-26.
// Verify external-IdP Ed25519 assertions. This package NEVER mints identities.
package attestation

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Claims are the signed identity and exact action/transaction attestation.
// The trusted identity provider must authenticate the human at signing time.
// For a physical custody signature, challenge is MaterialIssueChallenge(...).
type Claims struct {
	Issuer      string   `json:"iss"`
	Audience    string   `json:"aud"`
	Subject     string   `json:"sub"`
	TenantID    string   `json:"tenant_id"`
	PlantID     string   `json:"plant_id"`
	Action      string   `json:"action"`
	Challenge   string   `json:"challenge,omitempty"`
	JWTID       string   `json:"jti"`
	IssuedAt    int64    `json:"iat"`
	NotBefore   int64    `json:"nbf"`
	ExpiresAt   int64    `json:"exp"`
	Roles       []string `json:"roles"`
	AuthMethods []string `json:"amr"`
}

type Verifier struct {
	PublicKeys  map[string]ed25519.PublicKey
	Issuer      string
	Audience    string
	Now         func() time.Time
	MaxLifetime time.Duration
	ClockSkew   time.Duration
}

type header struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
	KeyID     string `json:"kid"`
}

func includes(ss []string, desired string) bool {
	for _, s := range ss {
		if s == desired {
			return true
		}
	}
	return false
}

// Verify is strict: only EdDSA, pinned key-id, exact issuer/audience/action,
// bounded freshness and trusted role assertions. Headers and client IDs are not principals.
func (v *Verifier) Verify(token, action, challenge, role string, requireStrongAuth bool) (Claims, error) {
	var c Claims
	if v == nil || v.Issuer == "" || v.Audience == "" || len(v.PublicKeys) == 0 {
		return c, errors.New("ERR_IDP_NOT_CONFIGURED")
	}
	if len(token) > 8192 || len(token) < 60 {
		return c, errors.New("ERR_INVALID_ASSERTION")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return c, errors.New("ERR_INVALID_ASSERTION")
	}
	hRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return c, errors.New("ERR_BAD_JWS_HEADER")
	}
	var h header
	if err = json.Unmarshal(hRaw, &h); err != nil || h.Algorithm != "EdDSA" || h.Type != "JWT" {
		return c, errors.New("ERR_JWS_ALGORITHM")
	}
	key := v.PublicKeys[h.KeyID]
	if len(key) != ed25519.PublicKeySize {
		return c, errors.New("ERR_UNKNOWN_SIGNING_KEY")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(sig) != ed25519.SignatureSize {
		return c, errors.New("ERR_JWS_SIGNATURE")
	}
	if !ed25519.Verify(key, []byte(parts[0]+"."+parts[1]), sig) {
		return c, errors.New("ERR_JWS_SIGNATURE")
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(body) > 4096 {
		return c, errors.New("ERR_JWS_CLAIMS")
	}
	if err = json.Unmarshal(body, &c); err != nil {
		return Claims{}, errors.New("ERR_JWS_CLAIMS")
	}
	if c.Issuer != v.Issuer || c.Audience != v.Audience || c.Action != action || c.Challenge != challenge ||
		strings.TrimSpace(c.Subject) == "" || strings.TrimSpace(c.TenantID) == "" || strings.TrimSpace(c.PlantID) == "" || strings.TrimSpace(c.JWTID) == "" {
		return Claims{}, errors.New("ERR_ASSERTION_SCOPE")
	}
	if role != "" && !includes(c.Roles, role) {
		return Claims{}, errors.New("ERR_ASSERTION_ROLE")
	}
	now := time.Now().UTC()
	if v.Now != nil {
		now = v.Now().UTC()
	}
	skew := v.ClockSkew
	if skew == 0 {
		skew = 15 * time.Second
	}
	max := v.MaxLifetime
	if max == 0 {
		max = 5 * time.Minute
	}
	nowS := now.Unix()
	skewS := int64(skew.Seconds())
	if c.ExpiresAt <= nowS-skewS || c.NotBefore > nowS+skewS || c.IssuedAt > nowS+skewS ||
		c.IssuedAt <= 0 || c.ExpiresAt <= c.IssuedAt || c.ExpiresAt-c.IssuedAt > int64(max.Seconds()) ||
		c.NotBefore < c.IssuedAt-skewS {
		return Claims{}, errors.New("ERR_ASSERTION_TIME")
	}
	if requireStrongAuth && !(includes(c.AuthMethods, "webauthn") || includes(c.AuthMethods, "fido2")) {
		return Claims{}, errors.New("ERR_STRONG_AUTH_REQUIRED")
	}
	return c, nil
}

// DecodePublicKey accepts a base64url-encoded, externally provisioned Ed25519 public key.
func DecodePublicKey(value string) (ed25519.PublicKey, error) {
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil || len(b) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("ERR_IDP_KEY_INVALID: expected 32-byte raw Ed25519 public key")
	}
	return ed25519.PublicKey(b), nil
}

// VerifyChallengeBound verifies the token-signed challenge when the expected
// digest is held in the database and therefore not available to the router.
// The signed challenge is ALWAYS rechecked by the repository before committing.
func (v *Verifier) VerifyChallengeBound(token, action, role string, strong bool) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, errors.New("ERR_INVALID_ASSERTION")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(raw) > 4096 {
		return Claims{}, errors.New("ERR_JWS_CLAIMS")
	}
	var untrusted Claims
	if err = json.Unmarshal(raw, &untrusted); err != nil || untrusted.Challenge == "" {
		return Claims{}, errors.New("ERR_CHALLENGE_REQUIRED")
	}
	return v.Verify(token, action, untrusted.Challenge, role, strong)
}
