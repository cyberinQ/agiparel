// Author: Sunil Jain, 2026-10-26 — Enforce tenant-scoped, serializable inventory reservations and authenticated material issuance.
package stores

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// MaterialRepository is a production TRANSACTION boundary, not an HTTP API.
// Caller identity MUST come from an authenticated server-side context. Never
// construct WitnessProof from a request's issuer_id / recipient_id strings.
type MaterialRepository struct {
	DB                    *sql.DB
	WitnessVerifier       MaterialWitnessVerifier
	TrimAdmissionVerifier TrimAdmissionVerifier
	CEOApprovalVerifier   CEOApprovalVerifier
	AuthorityHolder       string // server-configured lease holder, NEVER from HTTP request
}

// CEOApprovalVerifier verifies fresh IdP-issued signed approvals bound to the
// server-held variance challenge. A nil verifier MUST always deny approval.
type CEOApprovalVerifier interface {
	VerifyCEOApproval(ctx context.Context, tenant, subject, proofID, assertion, challenge string) error
}

type MaterialReserveCommand struct {
	TenantID       string
	PlantID        string
	RollID         string
	OrderID        string
	CutNo          int
	LengthMM       int64
	IdempotencyKey string
}

type ReservationReceipt struct {
	ID       string `json:"id"`
	RollID   string `json:"roll_id"`
	OrderID  string `json:"order_id"`
	CutNo    int    `json:"cut_no"`
	LengthMM int64  `json:"length_mm"`
}

// WitnessProof refers to an independently authenticated and cryptographically
// bound session assertion from an upstream identity service.
type WitnessProof struct {
	IDPIssuer string
	PlantID   string
	Role      string
	TenantID  string
	Subject   string
	ProofID   string
	Assertion string
}

// MaterialWitnessVerifier MUST validate assertion signature, verifier audience,
// single-use nonce, freshness and signed challenge. A non-nil verifier alone
// is not proof: deployment must register a production security implementation.
type MaterialWitnessVerifier interface {
	VerifyMaterialWitness(ctx context.Context, proof WitnessProof, challenge string) error
}

type MaterialIssueCommand struct {
	TenantID       string
	PlantID        string
	ReservationID  string
	RollID         string
	OrderID        string
	CutNo          int
	LengthMM       int64
	IdempotencyKey string
	Issuer         WitnessProof
	Recipient      WitnessProof
}

type IssueReceipt struct {
	ID            string `json:"id"`
	RollID        string `json:"roll_id"`
	OrderID       string `json:"order_id"`
	IssuedMM      int64  `json:"issued_mm"`
	ChallengeHash string `json:"challenge_hash"`
}

func nonblank(xs ...string) bool {
	for _, s := range xs {
		if strings.TrimSpace(s) == "" {
			return false
		}
	}
	return true
}

// MaterialIssueChallenge contains ONLY canonical transaction details, not a
// cryptographic assertion itself. Distinct proofs must sign this SAME string.
func MaterialIssueChallenge(c MaterialIssueCommand) string {
	// %q and newline delimiters prevent concatenation ambiguity.
	raw := fmt.Sprintf("agiparel.material-issue.v5\n%q\n%q\n%q\n%q\n%q\n%d\n%d\n%q",
		c.TenantID, c.PlantID, c.ReservationID, c.RollID, c.OrderID, c.CutNo, c.LengthMM, c.IdempotencyKey)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func VerifyMaterialWitnesses(ctx context.Context, verifier MaterialWitnessVerifier, c MaterialIssueCommand) (string, error) {
	if verifier == nil {
		return "", errors.New("ERR_WITNESS_VERIFIER_REQUIRED")
	}
	if !nonblank(c.TenantID, c.ReservationID, c.RollID, c.OrderID, c.IdempotencyKey,
		c.Issuer.Subject, c.Issuer.ProofID, c.Issuer.Assertion,
		c.Recipient.Subject, c.Recipient.ProofID, c.Recipient.Assertion) || c.CutNo <= 0 || c.LengthMM <= 0 {
		return "", errors.New("ERR_ISSUE_INPUT")
	}
	if c.Issuer.TenantID != c.TenantID || c.Recipient.TenantID != c.TenantID ||
		(c.PlantID != "" && (c.Issuer.PlantID != c.PlantID || c.Recipient.PlantID != c.PlantID)) ||
		c.Issuer.Subject == c.Recipient.Subject || c.Issuer.ProofID == c.Recipient.ProofID {
		return "", errors.New("ERR_INDEPENDENT_WITNESS")
	}
	challenge := MaterialIssueChallenge(c)
	if err := verifier.VerifyMaterialWitness(ctx, c.Issuer, challenge); err != nil {
		return "", fmt.Errorf("ERR_ISSUER_ATTESTATION: %w", err)
	}
	if err := verifier.VerifyMaterialWitness(ctx, c.Recipient, challenge); err != nil {
		return "", fmt.Errorf("ERR_RECIPIENT_ATTESTATION: %w", err)
	}
	return challenge, nil
}

func (r MaterialRepository) begin(ctx context.Context, tenant string) (*sql.Tx, error) {
	if r.DB == nil || !nonblank(tenant) {
		return nil, errors.New("ERR_DB_OR_TENANT_MISSING")
	}
	tx, err := r.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, "SELECT set_config('app.current_tenant_id',$1,true)", tenant); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	return tx, nil
}

// Reserve prevents reservation over-commit with both SERIALIZABLE isolation
// and a per-roll row lock, backed by migration 0005's database trigger.
func (r MaterialRepository) Reserve(ctx context.Context, c MaterialReserveCommand) (ReservationReceipt, error) {
	var out ReservationReceipt
	if !nonblank(c.TenantID, c.PlantID, c.RollID, c.OrderID, c.IdempotencyKey) || c.CutNo <= 0 || c.LengthMM <= 0 {
		return out, errors.New("ERR_RESERVATION_INPUT")
	}
	tx, err := r.begin(ctx, c.TenantID)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if err = r.requireAuthorizedScope(ctx, tx, c.TenantID, c.OrderID, c.PlantID); err != nil {
		return out, err
	}
	// Return replayed command exactly, rejecting attempts to change its body.
	err = tx.QueryRowContext(ctx, `SELECT id::text,roll_id,order_id,cut_no,reserved_mm FROM material_reservations_v5
        WHERE tenant_id=$1 AND request_key=$2`, c.TenantID, c.IdempotencyKey).Scan(&out.ID, &out.RollID, &out.OrderID, &out.CutNo, &out.LengthMM)
	if err == nil {
		if out.RollID != c.RollID || out.OrderID != c.OrderID || out.CutNo != c.CutNo || out.LengthMM != c.LengthMM {
			return ReservationReceipt{}, errors.New("ERR_IDEMPOTENCY_KEY_REUSE")
		}
		return out, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ReservationReceipt{}, err
	}
	var remainingMM int64
	var state, inspection, rollPlant string
	var legacyOwner sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT ROUND(remaining_length_m*1000)::BIGINT,status,inspection_status,reserved_order_id,plant_id
        FROM store_rolls WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, c.TenantID, c.RollID).
		Scan(&remainingMM, &state, &inspection, &legacyOwner, &rollPlant)
	if errors.Is(err, sql.ErrNoRows) {
		return out, errors.New("ERR_ROLL_NOT_FOUND")
	}
	if err != nil {
		return out, err
	}
	if rollPlant != c.PlantID {
		return out, errors.New("ERR_PLANT_SCOPE_MISMATCH")
	}
	if inspection != "PASS" || state == "QUARANTINE" || state == "DEPLETED" || state == "ISSUED" {
		return out, errors.New("ERR_ROLL_NOT_RELEASED")
	}
	if legacyOwner.Valid && legacyOwner.String != c.OrderID {
		return out, errors.New("ERR_SACROSANCT_RESERVATION")
	}
	var alreadyCommittedMM int64
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(reserved_mm-issued_mm-released_mm),0)::BIGINT
        FROM material_reservations_v5 WHERE tenant_id=$1 AND roll_id=$2`, c.TenantID, c.RollID).Scan(&alreadyCommittedMM)
	if err != nil {
		return out, err
	}
	if c.LengthMM > remainingMM-alreadyCommittedMM {
		return out, errors.New("ERR_UNRESERVED_LENGTH_INSUFFICIENT")
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO material_reservations_v5 (tenant_id,roll_id,order_id,cut_no,reserved_mm,request_key)
        VALUES ($1,$2,$3,$4,$5,$6) RETURNING id::text`, c.TenantID, c.RollID, c.OrderID, c.CutNo, c.LengthMM, c.IdempotencyKey).Scan(&out.ID)
	if err != nil {
		return ReservationReceipt{}, err
	}
	out.RollID, out.OrderID, out.CutNo, out.LengthMM = c.RollID, c.OrderID, c.CutNo, c.LengthMM
	if err = insertMaterialOutbox(ctx, tx, c.TenantID, "Reservation", out.ID, "MaterialReserved", out); err != nil {
		return ReservationReceipt{}, err
	}
	return out, tx.Commit()
}

// Issue requires two cryptographically verified, distinct authenticated people.
// It atomically commits BOTH the reservation consumption and the stock delta.
func (r MaterialRepository) Issue(ctx context.Context, c MaterialIssueCommand) (IssueReceipt, error) {
	var out IssueReceipt
	if !nonblank(c.PlantID) {
		return out, errors.New("ERR_PLANT_SCOPE_MISSING")
	}
	challenge, err := VerifyMaterialWitnesses(ctx, r.WitnessVerifier, c)
	if err != nil {
		return out, err
	}
	tx, err := r.begin(ctx, c.TenantID)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if err = r.requireAuthorizedScope(ctx, tx, c.TenantID, c.OrderID, c.PlantID); err != nil {
		return out, err
	}
	var priorReservation string
	var priorLength int64
	var issuer, recipient string
	err = tx.QueryRowContext(ctx, `SELECT id::text,reservation_id::text,roll_id,order_id,length_mm,issuer_subject,recipient_subject,signed_challenge_hash
        FROM material_issues_v5 WHERE tenant_id=$1 AND command_key=$2`, c.TenantID, c.IdempotencyKey).
		Scan(&out.ID, &priorReservation, &out.RollID, &out.OrderID, &priorLength, &issuer, &recipient, &out.ChallengeHash)
	if err == nil {
		if priorReservation != c.ReservationID || priorLength != c.LengthMM || out.RollID != c.RollID || out.OrderID != c.OrderID || issuer != c.Issuer.Subject || recipient != c.Recipient.Subject || out.ChallengeHash != challenge {
			return IssueReceipt{}, errors.New("ERR_IDEMPOTENCY_KEY_REUSE")
		}
		out.IssuedMM = priorLength
		return out, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return IssueReceipt{}, err
	}
	var remaining int64
	var status, inspection, rollPlant string
	var legacyOwner sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT ROUND(remaining_length_m*1000)::BIGINT,status,inspection_status,plant_id,reserved_order_id
        FROM store_rolls WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, c.TenantID, c.RollID).Scan(&remaining, &status, &inspection, &rollPlant, &legacyOwner)
	if errors.Is(err, sql.ErrNoRows) {
		return out, errors.New("ERR_ROLL_NOT_FOUND")
	}
	if err != nil {
		return out, err
	}
	if rollPlant != c.PlantID {
		return out, errors.New("ERR_PLANT_SCOPE_MISMATCH")
	}
	if legacyOwner.Valid && legacyOwner.String != c.OrderID {
		return out, errors.New("ERR_SACROSANCT_RESERVATION")
	}
	if inspection != "PASS" || status == "DEPLETED" || status == "QUARANTINE" {
		return out, errors.New("ERR_ROLL_NOT_RELEASED")
	}
	var reserved, issued, released int64
	var roll, order string
	var cut int
	err = tx.QueryRowContext(ctx, `SELECT roll_id,order_id,cut_no,reserved_mm,issued_mm,released_mm
        FROM material_reservations_v5 WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, c.TenantID, c.ReservationID).
		Scan(&roll, &order, &cut, &reserved, &issued, &released)
	if errors.Is(err, sql.ErrNoRows) {
		return out, errors.New("ERR_RESERVATION_NOT_FOUND")
	}
	if err != nil {
		return out, err
	}
	if roll != c.RollID || order != c.OrderID || cut != c.CutNo {
		return out, errors.New("ERR_ISSUE_RESERVATION_MISMATCH")
	}
	if c.LengthMM > remaining || c.LengthMM > reserved-issued-released {
		return out, errors.New("ERR_RESERVED_LENGTH_INSUFFICIENT")
	}
	// A separately authenticated issuer/recipient must sign a fresh challenge;
	// the DB's unique proof IDs prevent reuse across transactions.
	_, err = tx.ExecContext(ctx, `UPDATE material_reservations_v5 SET issued_mm=issued_mm+$1 WHERE id=$2 AND tenant_id=$3`, c.LengthMM, c.ReservationID, c.TenantID)
	if err != nil {
		return out, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE store_rolls SET remaining_length_m=(ROUND(remaining_length_m*1000)-$1)/1000.0,
        status=CASE WHEN ROUND(remaining_length_m*1000)=$1 THEN 'DEPLETED' ELSE 'ISSUED' END,
        aggregate_version=aggregate_version+1
        WHERE id=$2 AND tenant_id=$3`, c.LengthMM, c.RollID, c.TenantID)
	if err != nil {
		return out, err
	}
	issuerHash := sha256.Sum256([]byte(c.Issuer.Assertion))
	recipientHash := sha256.Sum256([]byte(c.Recipient.Assertion))
	err = tx.QueryRowContext(ctx, `INSERT INTO material_issues_v5
        (tenant_id,reservation_id,roll_id,order_id,cut_no,length_mm,command_key,issuer_subject,recipient_subject,issuer_proof_id,recipient_proof_id,issuer_assertion_hash,recipient_assertion_hash,signed_challenge_hash)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) RETURNING id::text`,
		c.TenantID, c.ReservationID, c.RollID, c.OrderID, c.CutNo, c.LengthMM, c.IdempotencyKey,
		c.Issuer.Subject, c.Recipient.Subject, c.Issuer.ProofID, c.Recipient.ProofID,
		hex.EncodeToString(issuerHash[:]), hex.EncodeToString(recipientHash[:]), challenge).Scan(&out.ID)
	if err != nil {
		return IssueReceipt{}, err
	}
	// One UNIQUE proof namespace prevents a proof being reused in either role.
	for _, item := range []struct{ proofID, role string }{
		{c.Issuer.ProofID, "ISSUER"}, {c.Recipient.ProofID, "RECIPIENT"},
	} {
		if _, err = tx.ExecContext(ctx, `INSERT INTO material_used_proofs_v5(tenant_id,proof_id,issue_id,witness_role)
            VALUES ($1,$2,$3,$4)`, c.TenantID, item.proofID, out.ID, item.role); err != nil {
			return IssueReceipt{}, err
		}
	}
	out.RollID, out.OrderID, out.IssuedMM, out.ChallengeHash = c.RollID, c.OrderID, c.LengthMM, challenge
	if err = insertMaterialOutbox(ctx, tx, c.TenantID, "Issue", out.ID, "CustodyTransferWitnessed", out); err != nil {
		return IssueReceipt{}, err
	}
	return out, tx.Commit()
}

func insertMaterialOutbox(ctx context.Context, tx *sql.Tx, tenant, kind, id, event string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO material_outbox_v5(tenant_id,aggregate_kind,aggregate_id,event_kind,payload)
        VALUES ($1,$2,$3,$4,$5)`, tenant, kind, id, event, string(b))
	return err
}
