// Author: Sunil Jain, 2026-10-28.
// Verify externally issued, audience-scoped JWT access tokens against provisioned
// OIDC JWKS. No identity minting, proxy-header trust, or client-supplied role grants.
package workforceauth

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/url"
	"os"
	"strings"
	"time"
)

var errInvalid = errors.New("ERR_WORKFORCE_INVALID_TOKEN")

type Identity struct{ Issuer, Subject string }
type Verifier struct {
	Issuer, Audience string
	keys             map[string]crypto.PublicKey
	Now              func() time.Time
}
type jwks struct {
	Keys []struct {
		Kty string `json:"kty"`
		Kid string `json:"kid"`
		Alg string `json:"alg"`
		Use string `json:"use"`
		N   string `json:"n"`
		E   string `json:"e"`
		Crv string `json:"crv"`
		X   string `json:"x"`
	} `json:"keys"`
}
type tokenHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	Typ string `json:"typ"`
}
type claims struct {
	Iss string          `json:"iss"`
	Sub string          `json:"sub"`
	Aud json.RawMessage `json:"aud"`
	Exp int64           `json:"exp"`
	Nbf int64           `json:"nbf"`
	Iat int64           `json:"iat"`
	AMR []string        `json:"amr"`
}

// New loads an approved JWKS snapshot. Rotate by deploying a new file and
// restarting the service; fetching an attacker-chosen jku/x5u is forbidden.
func New(issuer, audience, path string) (*Verifier, error) {
	parsed, parseErr := url.Parse(issuer)
	if parseErr != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || strings.ContainsAny(issuer, " \r\n") || audience == "" || len(audience) > 256 || path == "" {
		return nil, errors.New("ERR_WORKFORCE_CONFIG")
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, errors.New("ERR_WORKFORCE_JWKS")
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil || st.Size() > 128*1024 || st.Size() == 0 {
		return nil, errors.New("ERR_WORKFORCE_JWKS")
	}
	var doc jwks
	d := json.NewDecoder(f)
	if e = d.Decode(&doc); e != nil {
		return nil, errors.New("ERR_WORKFORCE_JWKS")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return nil, errors.New("ERR_WORKFORCE_JWKS_TRAILING_DATA")
	}
	if len(doc.Keys) == 0 || len(doc.Keys) > 32 {
		return nil, errors.New("ERR_WORKFORCE_JWKS")
	}
	v := &Verifier{Issuer: issuer, Audience: audience, keys: map[string]crypto.PublicKey{}}
	for _, k := range doc.Keys {
		if k.Kid == "" || len(k.Kid) > 128 || (k.Use != "sig" && k.Use != "") || v.keys[k.Kid] != nil {
			return nil, errors.New("ERR_WORKFORCE_JWKS_KEY")
		}
		switch {
		case k.Kty == "RSA" && (k.Alg == "RS256" || k.Alg == ""):
			n, ee := base64.RawURLEncoding.DecodeString(k.N)
			if ee != nil || len(n) < 256 || len(n) > 1024 {
				return nil, errors.New("ERR_WORKFORCE_JWKS_RSA")
			}
			eBytes, ee := base64.RawURLEncoding.DecodeString(k.E)
			if ee != nil || len(eBytes) == 0 || len(eBytes) > 4 {
				return nil, errors.New("ERR_WORKFORCE_JWKS_RSA")
			}
			exponent := 0
			for _, b := range eBytes {
				exponent = (exponent << 8) | int(b)
			}
			if exponent < 65537 || exponent%2 == 0 {
				return nil, errors.New("ERR_WORKFORCE_JWKS_RSA")
			}
			v.keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exponent}
		case k.Kty == "OKP" && (k.Alg == "EdDSA" || k.Alg == "") && k.Crv == "Ed25519":
			x, ee := base64.RawURLEncoding.DecodeString(k.X)
			if ee != nil || len(x) != ed25519.PublicKeySize {
				return nil, errors.New("ERR_WORKFORCE_JWKS_ED25519")
			}
			v.keys[k.Kid] = ed25519.PublicKey(x)
		default:
			return nil, errors.New("ERR_WORKFORCE_JWKS_ALGORITHM")
		}
	}
	return v, nil
}

func audienceIncludes(raw json.RawMessage, expected string) bool {
	var one string
	if json.Unmarshal(raw, &one) == nil {
		return one == expected
	}
	var many []string
	if json.Unmarshal(raw, &many) != nil || len(many) == 0 || len(many) > 8 {
		return false
	}
	for _, v := range many {
		if v == expected {
			return true
		}
	}
	return false
}

// Verify returns only the immutable IdP subject. Tenant/plant and permissions
// MUST be resolved independently against provisioner-controlled database grants.
func (v *Verifier) Verify(jwt string) (Identity, error) {
	if v == nil || len(v.keys) == 0 || len(jwt) > 8192 || len(jwt) < 60 {
		return Identity{}, errInvalid
	}
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		return Identity{}, errInvalid
	}
	hBytes, e := base64.RawURLEncoding.DecodeString(parts[0])
	if e != nil || len(hBytes) > 1024 {
		return Identity{}, errInvalid
	}
	var h tokenHeader
	if json.Unmarshal(hBytes, &h) != nil || h.Kid == "" || (h.Typ != "JWT" && h.Typ != "at+jwt") {
		return Identity{}, errInvalid
	}
	key := v.keys[h.Kid]
	if key == nil {
		return Identity{}, errInvalid
	}
	signature, e := base64.RawURLEncoding.DecodeString(parts[2])
	if e != nil {
		return Identity{}, errInvalid
	}
	signed := []byte(parts[0] + "." + parts[1])
	digest := sha256.Sum256(signed)
	switch k := key.(type) {
	case *rsa.PublicKey:
		if h.Alg != "RS256" || rsa.VerifyPKCS1v15(k, crypto.SHA256, digest[:], signature) != nil {
			return Identity{}, errInvalid
		}
	case ed25519.PublicKey:
		if h.Alg != "EdDSA" || !ed25519.Verify(k, signed, signature) {
			return Identity{}, errInvalid
		}
	default:
		return Identity{}, errInvalid
	}
	payload, e := base64.RawURLEncoding.DecodeString(parts[1])
	if e != nil || len(payload) > 4096 {
		return Identity{}, errInvalid
	}
	var c claims
	if json.Unmarshal(payload, &c) != nil || c.Iss != v.Issuer || c.Sub == "" || len(c.Sub) > 128 || !audienceIncludes(c.Aud, v.Audience) {
		return Identity{}, errInvalid
	}
	now := time.Now().UTC()
	if v.Now != nil {
		now = v.Now().UTC()
	}
	t := now.Unix()
	if c.Iat <= 0 || c.Iat > t+30 || c.Nbf > t+30 || c.Exp <= t || c.Exp <= c.Iat || c.Exp-c.Iat > 3600 {
		return Identity{}, errInvalid
	}
	// Strong workforce authentication is mandatory for factory management evidence.
	strong := false
	for _, amr := range c.AMR {
		if amr == "mfa" || amr == "webauthn" || amr == "fido2" || amr == "hwk" {
			strong = true
		}
	}
	if !strong {
		return Identity{}, errors.New("ERR_WORKFORCE_STRONG_AUTH_REQUIRED")
	}
	return Identity{Issuer: c.Iss, Subject: c.Sub}, nil
}

// ValidateKey can be used by provisioning diagnostics without disclosing keys.
