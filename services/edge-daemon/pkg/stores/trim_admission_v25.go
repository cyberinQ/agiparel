// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// F3.14B: two independently authenticated persons admit a NEW trim lot only
// against an approved, source-hashed document; never adjust an existing lot.
package stores

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var trimAdmissionID = regexp.MustCompile(`^[A-Za-z0-9_-]{12,80}$`)
var trimAdmissionSHA = regexp.MustCompile(`^[0-9a-f]{64}$`)
var trimAdmissionUOM = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,31}$`)

type TrimAdmissionVerifier interface {
	VerifyTrimAdmissionWitness(context.Context, WitnessProof, string) error
}

type TrimAdmissionCommand struct {
	TenantID, PlantID string       `json:"-"`
	StockID           string       `json:"stock_id"`
	TrimCode          string       `json:"trim_code"`
	Description       string       `json:"description"`
	SupplierLot       string       `json:"supplier_lot"`
	ColorCode         string       `json:"color_code"`
	SizeCode          string       `json:"size_code"`
	UOM               string       `json:"uom"`
	QuantityMilli     int64        `json:"quantity_milli"`
	BinLocation       string       `json:"bin_location"`
	SourceDocumentID  string       `json:"source_document_id"`
	SourceSHA256      string       `json:"source_sha256"`
	CommandID         string       `json:"command_id"`
	Manager           WitnessProof `json:"-"`
	Inspector         WitnessProof `json:"-"`
}

type TrimAdmissionReceipt struct {
	ID              string `json:"id"`
	StockID         string `json:"stock_id"`
	QuantityMilli   int64  `json:"quantity_milli"`
	ChallengeSHA256 string `json:"challenge_sha256"`
	Replayed        bool   `json:"replayed"`
	Authority       string `json:"authority"`
}

func ValidTrimAdmission(c TrimAdmissionCommand) bool {
	if !nonblank(c.TenantID, c.PlantID, c.StockID, c.TrimCode, c.Description, c.SupplierLot, c.UOM, c.BinLocation, c.SourceDocumentID, c.SourceSHA256, c.CommandID) ||
		c.QuantityMilli < 1 || c.QuantityMilli > 999999999999 ||
		!trimAdmissionID.MatchString(c.CommandID) || !trimAdmissionSHA.MatchString(c.SourceSHA256) || !trimAdmissionUOM.MatchString(c.UOM) {
		return false
	}
	for _, entry := range []struct {
		v   string
		max int
	}{{c.TenantID, 64}, {c.PlantID, 64}, {c.StockID, 64}, {c.TrimCode, 64}, {c.Description, 255}, {c.SupplierLot, 128}, {c.ColorCode, 64}, {c.SizeCode, 32}, {c.BinLocation, 128}, {c.SourceDocumentID, 128}} {
		if len(entry.v) > entry.max || strings.ContainsAny(entry.v, "\r\n\x00") {
			return false
		}
	}
	return len(strings.TrimSpace(c.BinLocation)) >= 2
}

func TrimAdmissionChallenge(c TrimAdmissionCommand) string {
	raw := fmt.Sprintf("agiparel.trim-admit.v25\n%q\n%q\n%q\n%q\n%q\n%q\n%q\n%q\n%q\n%d\n%q\n%q\n%q\n%q",
		c.TenantID, c.PlantID, c.StockID, c.TrimCode, c.Description, c.SupplierLot, c.ColorCode, c.SizeCode, c.UOM, c.QuantityMilli, c.BinLocation, c.SourceDocumentID, c.SourceSHA256, c.CommandID)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func VerifyTrimAdmissionWitnesses(ctx context.Context, verifier TrimAdmissionVerifier, c TrimAdmissionCommand) error {
	if !ValidTrimAdmission(c) {
		return errors.New("ERR_F314B_FIELDS")
	}
	if verifier == nil {
		return errors.New("ERR_F314B_VERIFIER_REQUIRED")
	}
	a, b := c.Manager, c.Inspector
	if !nonblank(a.Subject, a.ProofID, a.Assertion, a.IDPIssuer, b.Subject, b.ProofID, b.Assertion, b.IDPIssuer) ||
		a.Role != "stores_manager" || b.Role != "stores_inspector" ||
		a.Subject == b.Subject || a.ProofID == b.ProofID ||
		a.TenantID != c.TenantID || b.TenantID != c.TenantID ||
		a.PlantID != c.PlantID || b.PlantID != c.PlantID ||
		len(a.Subject) > 64 || len(b.Subject) > 64 || len(a.ProofID) > 255 || len(b.ProofID) > 255 ||
		len(a.IDPIssuer) > 255 || len(b.IDPIssuer) > 255 {
		return errors.New("ERR_F314B_INDEPENDENT_WITNESSES")
	}
	challenge := TrimAdmissionChallenge(c)
	if err := verifier.VerifyTrimAdmissionWitness(ctx, a, challenge); err != nil {
		return fmt.Errorf("ERR_F314B_MANAGER_PROOF: %w", err)
	}
	if err := verifier.VerifyTrimAdmissionWitness(ctx, b, challenge); err != nil {
		return fmt.Errorf("ERR_F314B_INSPECTOR_PROOF: %w", err)
	}
	return nil
}

func (r MaterialRepository) AdmitTrim(ctx context.Context, c TrimAdmissionCommand) (TrimAdmissionReceipt, error) {
	out := TrimAdmissionReceipt{}
	if err := VerifyTrimAdmissionWitnesses(ctx, r.TrimAdmissionVerifier, c); err != nil {
		return out, err
	}
	tx, err := r.begin(ctx, c.TenantID)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	// The SQL trigger independently verifies these same grants and source status.
	for _, p := range []struct {
		w    WitnessProof
		perm string
	}{{c.Manager, "stores.trims.admit"}, {c.Inspector, "stores.trims.inspect"}} {
		var allowed bool
		err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM workforce_identity_grants_v15 g JOIN workforce_chapter_actions_v17 a
     ON a.permission=g.permission AND a.chapter_id=4 AND a.authority='SIGNED_WRITE'
     WHERE g.tenant_id=$1 AND g.plant_id=$2 AND g.idp_subject=$3 AND g.idp_issuer=$4 AND g.permission=$5 AND g.enabled)`,
			c.TenantID, c.PlantID, p.w.Subject, p.w.IDPIssuer, p.perm).Scan(&allowed)
		if err != nil {
			return out, err
		}
		if !allowed {
			return out, errors.New("ERR_F314B_GRANT_REQUIRED")
		}
	}
	var stock, plant, code, descr, lot, color, size, uom, bin, document, source, command, mgr, ins, mgrProof, insProof, challenge string
	var quantity int64
	err = tx.QueryRowContext(ctx, `SELECT id::text,stock_id,plant_id,trim_code,description,supplier_lot,color_code,size_code,uom,
 ROUND(received_qty*1000)::bigint,bin_location,source_document_id,source_sha256,command_id,
 manager_subject,inspector_subject,manager_proof_id,inspector_proof_id,challenge_sha256
 FROM trim_stock_admissions_v25 WHERE tenant_id=$1 AND command_id=$2`, c.TenantID, c.CommandID).Scan(
		&out.ID, &stock, &plant, &code, &descr, &lot, &color, &size, &uom, &quantity, &bin, &document, &source, &command, &mgr, &ins, &mgrProof, &insProof, &challenge)
	if err == nil {
		if stock != c.StockID || plant != c.PlantID || code != c.TrimCode || descr != c.Description || lot != c.SupplierLot || color != c.ColorCode || size != c.SizeCode || uom != c.UOM || quantity != c.QuantityMilli || bin != c.BinLocation || document != c.SourceDocumentID || source != c.SourceSHA256 || command != c.CommandID || mgr != c.Manager.Subject || ins != c.Inspector.Subject || mgrProof != c.Manager.ProofID || insProof != c.Inspector.ProofID || challenge != TrimAdmissionChallenge(c) {
			return out, errors.New("ERR_F314B_COMMAND_CONFLICT")
		}
		out.StockID, out.QuantityMilli, out.ChallengeSHA256, out.Replayed, out.Authority = c.StockID, c.QuantityMilli, challenge, true, "SIGNED_ADMISSION_DOCUMENT_NOT_CALIBRATED_MEASUREMENT"
		return out, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO trim_stock_admissions_v25(
 tenant_id,plant_id,stock_id,trim_code,description,supplier_lot,color_code,size_code,uom,received_qty,bin_location,
 source_document_id,source_sha256,command_id,manager_subject,inspector_subject,manager_issuer,inspector_issuer,
 manager_proof_id,inspector_proof_id,manager_assertion_sha256,inspector_assertion_sha256,challenge_sha256)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::numeric/1000,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23) RETURNING id::text`,
		c.TenantID, c.PlantID, c.StockID, c.TrimCode, c.Description, c.SupplierLot, c.ColorCode, c.SizeCode, c.UOM, c.QuantityMilli, c.BinLocation, c.SourceDocumentID, c.SourceSHA256, c.CommandID,
		c.Manager.Subject, c.Inspector.Subject, c.Manager.IDPIssuer, c.Inspector.IDPIssuer, c.Manager.ProofID, c.Inspector.ProofID, trimHash(c.Manager.Assertion), trimHash(c.Inspector.Assertion), TrimAdmissionChallenge(c)).Scan(&out.ID)
	if err != nil {
		if strings.Contains(err.Error(), "ERR_F314B_") || strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			return out, errors.New("ERR_F314B_STOCK_SOURCE_OR_PROOF_CONFLICT")
		}
		return out, fmt.Errorf("trim admission database write: %w", err)
	}
	out.StockID, out.QuantityMilli, out.ChallengeSHA256, out.Authority = c.StockID, c.QuantityMilli, TrimAdmissionChallenge(c), "SIGNED_ADMISSION_DOCUMENT_NOT_CALIBRATED_MEASUREMENT"
	return out, tx.Commit()
}
