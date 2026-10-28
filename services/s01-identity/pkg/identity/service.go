// Author: Sunil Jain, CyberinQ LLC
package identity

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
)

var (
	ErrUnauthorized     = errors.New("ERR_UNAUTHORIZED: operation denied by tenant policy")
	ErrDeviceRevoked    = errors.New("ERR_DEVICE_REVOKED: hardware unit is untrusted or revoked")
	ErrInvalidSignature = errors.New("ERR_INVALID_SIGNATURE: ed25519 payload attestation failed")
)

type UserGrant struct {
	UserID      string   `json:"user_id"`
	Email       string   `json:"email"`
	TenantID    string   `json:"tenant_id"`
	PlantID     string   `json:"plant_id"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
}

type DeviceEnrollment struct {
	DeviceUID       string `json:"device_uid"`
	DeviceType      string `json:"device_type"`
	PlantID         string `json:"plant_id"`
	CertFingerprint string `json:"cert_fingerprint"`
	PublicKeyPEM    string `json:"public_key_pem"`
}

type IdentityService struct {
	db *sql.DB
}

func NewIdentityService(db *sql.DB) *IdentityService {
	return &IdentityService{db: db}
}

func (s *IdentityService) ExecuteWithTenant(ctx context.Context, tenantID string, fn func(tx *sql.Tx) error) error {
	if s.db == nil {
		return errors.New("ERR_DB_NIL: database connection is nil")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("tx_begin: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "SELECT set_config('app.current_tenant_id', $1, true)", tenantID); err != nil {
		return fmt.Errorf("set_tenant: %w", err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *IdentityService) VerifyDeviceSignature(pubKey ed25519.PublicKey, rawPayload []byte, signatureHex string) error {
	if len(pubKey) != ed25519.PublicKeySize {
		return ErrInvalidSignature
	}
	sigBytes, err := hex.DecodeString(signatureHex)
	if err != nil {
		return fmt.Errorf("decode_signature: %w", err)
	}
	if len(sigBytes) != ed25519.SignatureSize {
		return ErrInvalidSignature
	}
	digest := sha256.Sum256(rawPayload)
	if !ed25519.Verify(pubKey, digest[:], sigBytes) {
		return ErrInvalidSignature
	}
	return nil
}

func (s *IdentityService) EnforcePermission(ctx context.Context, tenantID, userID, plantID, requiredPermission string) (bool, error) {
	var hasPerm bool
	err := s.ExecuteWithTenant(ctx, tenantID, func(tx *sql.Tx) error {
		query := `
SELECT EXISTS (
	SELECT 1 FROM s01_identity.user_role_grants urg
	JOIN s01_identity.users u ON u.id = urg.user_id AND u.tenant_id = urg.tenant_id
	JOIN s01_identity.role_permissions rp ON urg.role_id = rp.role_id
	WHERE urg.user_id::text = $1
		AND urg.tenant_id::text = $2
		AND u.is_active = true
		AND (urg.plant_id::text = NULLIF($3, '') OR urg.plant_id IS NULL)
		AND rp.permission = $4
	);`
		return tx.QueryRowContext(ctx, query, userID, tenantID, plantID, requiredPermission).Scan(&hasPerm)
	})
	if err != nil {
		return false, err
	}
	if !hasPerm {
		return false, ErrUnauthorized
	}
	return true, nil
}
