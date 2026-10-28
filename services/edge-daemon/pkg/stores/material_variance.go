// Author: Sunil Jain, CyberinQ LLC, 2026-10-26.
// CEO-approved additional consumption authorizes ONLY available, same-order stock.
// It never releases another buyer's or another PO's reserved inventory.
package stores

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

type VarianceRequestCommand struct {
	TenantID, PlantID, ReservationID, RollID, OrderID    string
	CutNo                                                int
	AdditionalMM                                         int64
	ActualStory, ReasonCode, RequestedBy, IdempotencyKey string
}

type VarianceReceipt struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	Challenge    string `json:"challenge"`
	OrderID      string `json:"order_id"`
	RollID       string `json:"roll_id"`
	AdditionalMM int64  `json:"additional_mm"`
}

type VarianceApprovalCommand struct {
	TenantID, PlantID, RequestID, CEOSubject, CEOProofID, CEOAssertion string
}

func varianceChallenge(c VarianceRequestCommand, id string) string {
	raw := fmt.Sprintf("agiparel.material-variance.v5\n%q\n%q\n%q\n%q\n%q\n%q\n%d\n%d\n%q\n%q\n%q\n%q",
		c.TenantID, c.PlantID, id, c.ReservationID, c.RollID, c.OrderID, c.CutNo, c.AdditionalMM, c.ActualStory, c.ReasonCode, c.RequestedBy, c.IdempotencyKey)
	d := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(d[:])
}

func isAuthorizedVarianceApprover(ctx context.Context, tx *sql.Tx, tenantID, subject string, deltaMM int64) error {
	var enrolledCEO string
	ceoErr := tx.QueryRowContext(ctx, `SELECT subject FROM material_ceo_principals_v5 WHERE tenant_id=$1 AND active=true`, tenantID).Scan(&enrolledCEO)
	if ceoErr == nil && enrolledCEO == subject {
		return nil
	}
	if deltaMM <= 10000 {
		var isPlantAuth bool
		err := tx.QueryRowContext(ctx, `SELECT EXISTS (
			SELECT 1 FROM s01_identity.user_role_grants urg
			JOIN s01_identity.roles r ON urg.role_id = r.id
			WHERE urg.tenant_id=$1 AND urg.user_id=$2
			  AND r.name IN ('PLANT_MANAGER', 'CUTTING_MASTER', 'FACTORY_GM')
		)`, tenantID, subject).Scan(&isPlantAuth)
		if err == nil && isPlantAuth {
			return nil
		}
		return errors.New("ERR_UNAUTHORIZED_VARIANCE_APPROVER")
	}
	if errors.Is(ceoErr, sql.ErrNoRows) || enrolledCEO != subject {
		return errors.New("ERR_CEO_ONLY_FOR_MAJOR_VARIANCE")
	}
	return ceoErr
}

func (r MaterialRepository) RequestVariance(ctx context.Context, c VarianceRequestCommand) (VarianceReceipt, error) {
	var out VarianceReceipt
	if !nonblank(c.TenantID, c.PlantID, c.ReservationID, c.RollID, c.OrderID, c.ReasonCode, c.RequestedBy, c.IdempotencyKey) ||
		len(strings.TrimSpace(c.ActualStory)) < 20 || c.CutNo <= 0 || c.AdditionalMM <= 0 {
		return out, errors.New("ERR_VARIANCE_INPUT")
	}
	tx, err := r.begin(ctx, c.TenantID)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if err = r.requireAuthorizedScope(ctx, tx, c.TenantID, c.OrderID, c.PlantID); err != nil {
		return out, err
	}
	var prior VarianceRequestCommand
	err = tx.QueryRowContext(ctx, `SELECT id::text,status,challenge_hash,plant_id,reservation_id::text,roll_id,order_id,cut_no,requested_additional_mm,actual_story,reason_code,requested_by
  FROM material_variances_v5 WHERE tenant_id=$1 AND request_key=$2`, c.TenantID, c.IdempotencyKey).Scan(
		&out.ID, &out.Status, &out.Challenge, &prior.PlantID, &prior.ReservationID, &prior.RollID, &prior.OrderID, &prior.CutNo, &prior.AdditionalMM, &prior.ActualStory, &prior.ReasonCode, &prior.RequestedBy)
	if err == nil {
		if prior.PlantID != c.PlantID || prior.ReservationID != c.ReservationID || prior.RollID != c.RollID || prior.OrderID != c.OrderID ||
			prior.CutNo != c.CutNo || prior.AdditionalMM != c.AdditionalMM || prior.ActualStory != c.ActualStory ||
			prior.ReasonCode != c.ReasonCode || prior.RequestedBy != c.RequestedBy {
			return VarianceReceipt{}, errors.New("ERR_IDEMPOTENCY_KEY_REUSE")
		}
		out.OrderID, out.RollID, out.AdditionalMM = c.OrderID, c.RollID, c.AdditionalMM
		return out, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	var roll, order string
	var cut int
	err = tx.QueryRowContext(ctx, `SELECT roll_id,order_id,cut_no FROM material_reservations_v5 WHERE id=$1 AND tenant_id=$2 FOR UPDATE`, c.ReservationID, c.TenantID).Scan(&roll, &order, &cut)
	if errors.Is(err, sql.ErrNoRows) {
		return out, errors.New("ERR_RESERVATION_NOT_FOUND")
	}
	if err != nil {
		return out, err
	}
	if roll != c.RollID || order != c.OrderID || cut != c.CutNo {
		return out, errors.New("ERR_VARIANCE_RESERVATION_MISMATCH")
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO material_variances_v5(tenant_id,plant_id,reservation_id,roll_id,order_id,cut_no,requested_additional_mm,actual_story,reason_code,requested_by,request_key)
   VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id::text`,
		c.TenantID, c.PlantID, c.ReservationID, c.RollID, c.OrderID, c.CutNo, c.AdditionalMM, c.ActualStory, c.ReasonCode, c.RequestedBy, c.IdempotencyKey).Scan(&out.ID)
	if err != nil {
		return out, err
	}
	out.Status = "PENDING_CEO"
	out.OrderID, out.RollID, out.AdditionalMM = c.OrderID, c.RollID, c.AdditionalMM
	out.Challenge = varianceChallenge(c, out.ID)
	_, err = tx.ExecContext(ctx, `UPDATE material_variances_v5 SET challenge_hash=$1 WHERE tenant_id=$2 AND id=$3`, out.Challenge, c.TenantID, out.ID)
	if err != nil {
		return VarianceReceipt{}, err
	}
	if err = insertMaterialOutbox(ctx, tx, c.TenantID, "Variance", out.ID, "ConsumptionVarianceRaised", out); err != nil {
		return VarianceReceipt{}, err
	}
	return out, tx.Commit()
}

// LookupVarianceChallenge returns a server-held immutable digest after authenticated approver review.
func (r MaterialRepository) LookupVarianceChallenge(ctx context.Context, tenant, plant, id, approverSubject string) (string, error) {
	if !nonblank(tenant, plant, id, approverSubject) {
		return "", errors.New("ERR_VARIANCE_INPUT")
	}
	tx, err := r.begin(ctx, tenant)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var challenge string
	var deltaMM int64
	err = tx.QueryRowContext(ctx, `SELECT challenge_hash, requested_additional_mm FROM material_variances_v5 WHERE tenant_id=$1 AND plant_id=$2 AND id=$3 AND status='PENDING_CEO'`, tenant, plant, id).Scan(&challenge, &deltaMM)
	if err != nil {
		return "", err
	}
	if err := isAuthorizedVarianceApprover(ctx, tx, tenant, approverSubject, deltaMM); err != nil {
		return "", err
	}
	return challenge, tx.Commit()
}

// ApproveVariance is called ONLY after trusted IdP verifies the approver's challenge-
// bound WebAuthn/FIDO2 signed assertion, and the exact subject is checked in DB.
// No authorization grants more fabric than physically available.
func (r MaterialRepository) ApproveVariance(ctx context.Context, c VarianceApprovalCommand) (VarianceReceipt, error) {
	var out VarianceReceipt
	if !nonblank(c.TenantID, c.PlantID, c.RequestID, c.CEOSubject, c.CEOProofID, c.CEOAssertion) {
		return out, errors.New("ERR_APPROVER_INPUT")
	}
	tx, err := r.begin(ctx, c.TenantID)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()

	var reservation, order, roll, challenge, status string
	var delta int64
	var cut int
	var existingProof sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT reservation_id::text,order_id,roll_id,cut_no,requested_additional_mm,challenge_hash,status,ceo_proof_id FROM material_variances_v5
   WHERE tenant_id=$1 AND plant_id=$2 AND id=$3 FOR UPDATE`, c.TenantID, c.PlantID, c.RequestID).Scan(&reservation, &order, &roll, &cut, &delta, &challenge, &status, &existingProof)
	if err != nil {
		return out, err
	}
	if err := isAuthorizedVarianceApprover(ctx, tx, c.TenantID, c.CEOSubject, delta); err != nil {
		return out, err
	}

	out = VarianceReceipt{ID: c.RequestID, Status: status, Challenge: challenge, RollID: roll, OrderID: order, AdditionalMM: delta}
	if r.CEOApprovalVerifier == nil {
		return out, errors.New("ERR_CEO_VERIFIER_REQUIRED")
	}
	if err = r.CEOApprovalVerifier.VerifyCEOApproval(ctx, c.TenantID, c.CEOSubject, c.CEOProofID, c.CEOAssertion, challenge); err != nil {
		return out, fmt.Errorf("ERR_CEO_ATTESTATION: %w", err)
	}
	if status == "APPROVED" {
		if existingProof.Valid && existingProof.String == c.CEOProofID {
			return out, tx.Commit()
		}
		return out, errors.New("ERR_VARIANCE_ALREADY_FINAL")
	}
	if status != "PENDING_CEO" {
		return out, errors.New("ERR_VARIANCE_ALREADY_FINAL")
	}
	if err = r.requireAuthorizedScope(ctx, tx, c.TenantID, order, c.PlantID); err != nil {
		return out, err
	}
	var remaining int64
	var state, inspection, rollPlant string
	var owner sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT ROUND(remaining_length_m*1000)::BIGINT,status,inspection_status,reserved_order_id,plant_id FROM store_rolls
   WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, c.TenantID, roll).Scan(&remaining, &state, &inspection, &owner, &rollPlant)
	if err != nil {
		return out, err
	}
	if rollPlant != c.PlantID {
		return out, errors.New("ERR_PLANT_SCOPE_MISMATCH")
	}
	if inspection != "PASS" || state == "QUARANTINE" || state == "DEPLETED" || (owner.Valid && owner.String != order) {
		return out, errors.New("ERR_MATERIAL_NOT_ELIGIBLE")
	}
	var reserved, issued, released int64
	var rowRoll, rowOrder string
	var rowCut int
	err = tx.QueryRowContext(ctx, `SELECT roll_id,order_id,cut_no,reserved_mm,issued_mm,released_mm FROM material_reservations_v5
   WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, c.TenantID, reservation).Scan(&rowRoll, &rowOrder, &rowCut, &reserved, &issued, &released)
	if err != nil {
		return out, err
	}
	if rowRoll != roll || rowOrder != order || rowCut != cut {
		return out, errors.New("ERR_VARIANCE_RESERVATION_MISMATCH")
	}
	var committed int64
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(reserved_mm-issued_mm-released_mm),0)::BIGINT FROM material_reservations_v5
   WHERE tenant_id=$1 AND roll_id=$2`, c.TenantID, roll).Scan(&committed)
	if err != nil {
		return out, err
	}
	if delta > remaining-committed {
		return out, errors.New("ERR_CEO_CANNOT_CREATE_STOCK")
	}
	// Database-enforced over-reservation guard in 0005 triggers on the update.
	_, err = tx.ExecContext(ctx, `UPDATE material_reservations_v5 SET reserved_mm=reserved_mm+$1 WHERE tenant_id=$2 AND id=$3`, delta, c.TenantID, reservation)
	if err != nil {
		return out, err
	}
	sum := sha256.Sum256([]byte(c.CEOAssertion))
	_, err = tx.ExecContext(ctx, `UPDATE material_variances_v5 SET status='APPROVED',ceo_subject=$1,ceo_proof_id=$2,ceo_assertion_hash=$3,approved_at=now()
   WHERE tenant_id=$4 AND id=$5 AND status='PENDING_CEO'`, c.CEOSubject, c.CEOProofID, hex.EncodeToString(sum[:]), c.TenantID, c.RequestID)
	if err != nil {
		return out, err
	}
	out.Status = "APPROVED"
	if err = insertMaterialOutbox(ctx, tx, c.TenantID, "Variance", c.RequestID, "CEOConsumptionVarianceApproved", out); err != nil {
		return VarianceReceipt{}, err
	}
	return out, tx.Commit()
}
