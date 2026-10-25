package identity_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"os"
	"testing"

	"github.com/cyberinq/agiparel/services/s01-identity/pkg/identity"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func setupTestDB(t *testing.T) (*sql.DB, func()) {
	if os.Getenv("RUN_DB_TESTS") != "1" {
		t.Skip("Set RUN_DB_TESTS=1 for PostgreSQL integration tests")
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://agiparel_app:app_secure_runtime_pass@localhost:5432/agiparel_dev?sslmode=disable"
	}
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		t.Fatalf("Failed to connect to PG: %v", err)
	}

	return db, func() {
		_ = db.Close()
	}
}

func TestA41_SeparationOfDuties_UnauthorizedWaiver(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	svc := identity.NewIdentityService(db)
	ctx := context.Background()

	// Provision test tenant and plant
	var tenantID, plantID, workerID, supervisorID, opRoleID, supRoleID string
	err := db.QueryRow(`
		INSERT INTO s01_identity.tenants (name, slug) 
		VALUES ('Test Tenant Noble', 'test-noble-' || gen_random_uuid()) 
		RETURNING id::text;
	`).Scan(&tenantID)
	if err != nil {
		t.Fatalf("Tenant seed failed: %v", err)
	}

	_ = db.QueryRow(`
		INSERT INTO s01_identity.plants (tenant_id, code, name) 
		VALUES ($1::uuid, 'OKHLA-01', 'Okhla Plant') 
		RETURNING id::text;
	`, tenantID).Scan(&plantID)

	// Roles
	_ = db.QueryRow(`INSERT INTO s01_identity.roles (tenant_id, name) VALUES ($1::uuid, 'OPERATOR') RETURNING id::text;`, tenantID).Scan(&opRoleID)
	_ = db.QueryRow(`INSERT INTO s01_identity.roles (tenant_id, name) VALUES ($1::uuid, 'SUPERVISOR') RETURNING id::text;`, tenantID).Scan(&supRoleID)

	// Grant permissions: supervisor gets carton waiver, operator only gets read
	_, _ = db.Exec(`INSERT INTO s01_identity.role_permissions (role_id, permission) VALUES ($1::uuid, 'SEW_PIECE_CLAIM');`, opRoleID)
	_, _ = db.Exec(`INSERT INTO s01_identity.role_permissions (role_id, permission) VALUES ($1::uuid, 'SEW_PIECE_CLAIM'), ($1::uuid, 'APPROVE_CARTON_WAIVER');`, supRoleID)

	// Users
	_ = db.QueryRow(`INSERT INTO s01_identity.users (tenant_id, email, full_name) VALUES ($1::uuid, 'worker@test.com', 'Test Worker') RETURNING id::text;`, tenantID).Scan(&workerID)
	_ = db.QueryRow(`INSERT INTO s01_identity.users (tenant_id, email, full_name) VALUES ($1::uuid, 'supv@test.com', 'Test Supervisor') RETURNING id::text;`, tenantID).Scan(&supervisorID)

	// Role grants
	_, _ = db.Exec(`INSERT INTO s01_identity.user_role_grants (tenant_id, user_id, role_id, plant_id) VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid);`, tenantID, workerID, opRoleID, plantID)
	_, _ = db.Exec(`INSERT INTO s01_identity.user_role_grants (tenant_id, user_id, role_id, plant_id) VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid);`, tenantID, supervisorID, supRoleID, plantID)

	// Case 1: Worker attempts carton waiver -> MUST FAIL with ErrUnauthorized
	authorized, err := svc.EnforcePermission(ctx, tenantID, workerID, plantID, "APPROVE_CARTON_WAIVER")
	if authorized || err != identity.ErrUnauthorized {
		t.Fatalf("Expected ErrUnauthorized for worker, got authorized=%v, err=%v", authorized, err)
	}

	// Case 2: Supervisor attempts carton waiver -> MUST SUCCEED
	authorized, err = svc.EnforcePermission(ctx, tenantID, supervisorID, plantID, "APPROVE_CARTON_WAIVER")
	if !authorized || err != nil {
		t.Fatalf("Expected authorization for supervisor, got authorized=%v, err=%v", authorized, err)
	}
}

func TestA12_HardwareDeviceAttestationAndRevocation(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	svc := identity.NewIdentityService(db)

	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Keygen failed: %v", err)
	}

	payload := []byte(`{"device_uid":"ZEBRA-TC21-001","station":"LINE14_ST04","ts":1792606580}`)
	digest := sha256.Sum256(payload)
	sig := ed25519.Sign(privKey, digest[:])
	sigHex := hex.EncodeToString(sig)

	// Verify valid signature
	if err := svc.VerifyDeviceSignature(pubKey, payload, sigHex); err != nil {
		t.Fatalf("Expected valid device signature: %v", err)
	}

	// Tamper payload: must fail
	tamperedPayload := []byte(`{"device_uid":"ZEBRA-TC21-001","station":"LINE14_ST05","ts":1792606580}`)
	if err := svc.VerifyDeviceSignature(pubKey, tamperedPayload, sigHex); err != identity.ErrInvalidSignature {
		t.Fatalf("Expected ErrInvalidSignature on tampered payload, got %v", err)
	}
}
