// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// F3.14: independently signed, serializable trims inventory issuance.
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

type TrimIssueCommand struct {
	TenantID      string       `json:"-"`
	PlantID       string       `json:"-"`
	TrimStockID   string       `json:"trim_stock_id"`
	OrderID       string       `json:"order_id"`
	LineID        string       `json:"line_id"`
	QuantityMilli int64        `json:"quantity_milli"` // thousandths of the stock UOM, never a floating-point number
	CommandID     string       `json:"command_id"`
	Issuer        WitnessProof `json:"-"`
	Recipient     WitnessProof `json:"-"`
}
type TrimIssueReceipt struct {
	ID              string `json:"id"`
	TrimStockID     string `json:"trim_stock_id"`
	OrderID         string `json:"order_id"`
	QuantityMilli   int64  `json:"quantity_milli"`
	ChallengeSHA256 string `json:"challenge_sha256"`
	Replayed        bool   `json:"replayed"`
	Authority       string `json:"authority"`
}

func ValidTrimIssue(c TrimIssueCommand) bool {
	if !nonblank(c.TenantID, c.PlantID, c.TrimStockID, c.OrderID, c.LineID, c.CommandID) || c.QuantityMilli <= 0 || c.QuantityMilli > 999999999999 || len(c.CommandID) < 12 || len(c.CommandID) > 80 {
		return false
	}
	for _, ch := range c.CommandID {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-') {
			return false
		}
	}
	return len(c.TenantID) <= 64 && len(c.PlantID) <= 64 && len(c.TrimStockID) <= 64 && len(c.OrderID) <= 64 && len(c.LineID) <= 64
}
func TrimIssueChallenge(c TrimIssueCommand) string {
	raw := fmt.Sprintf("agiparel.trim-issue.v24\n%q\n%q\n%q\n%q\n%q\n%d\n%q", c.TenantID, c.PlantID, c.TrimStockID, c.OrderID, c.LineID, c.QuantityMilli, c.CommandID)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
func trimHash(s string) string { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }

func VerifyTrimWitnesses(ctx context.Context, v MaterialWitnessVerifier, c TrimIssueCommand) error {
	if !ValidTrimIssue(c) {
		return errors.New("ERR_F314_TRIM_FIELDS")
	}
	if v == nil {
		return errors.New("ERR_F314_VERIFIER_REQUIRED")
	}
	a, b := c.Issuer, c.Recipient
	if !nonblank(a.Subject, b.Subject, a.ProofID, b.ProofID, a.Assertion, b.Assertion, a.IDPIssuer, b.IDPIssuer) ||
		a.Role != "stores_issuer" || b.Role != "stores_recipient" ||
		len(a.Subject) > 64 || len(b.Subject) > 64 || len(a.ProofID) > 255 || len(b.ProofID) > 255 ||
		a.Subject == b.Subject || a.ProofID == b.ProofID ||
		a.TenantID != c.TenantID || b.TenantID != c.TenantID || a.PlantID != c.PlantID || b.PlantID != c.PlantID {
		return errors.New("ERR_F314_INDEPENDENT_WITNESSES")
	}
	challenge := TrimIssueChallenge(c)
	if err := v.VerifyMaterialWitness(ctx, a, challenge); err != nil {
		return fmt.Errorf("ERR_F314_ISSUER_PROOF: %w", err)
	}
	if err := v.VerifyMaterialWitness(ctx, b, challenge); err != nil {
		return fmt.Errorf("ERR_F314_RECIPIENT_PROOF: %w", err)
	}
	return nil
}
func (r MaterialRepository) IssueTrim(ctx context.Context, c TrimIssueCommand) (TrimIssueReceipt, error) {
	out := TrimIssueReceipt{}
	if err := VerifyTrimWitnesses(ctx, r.WitnessVerifier, c); err != nil {
		return out, err
	}
	tx, err := r.begin(ctx, c.TenantID)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	// Both independently provisioned grants must exist, in addition to signed roles.
	for _, item := range []struct {
		proof      WitnessProof
		permission string
	}{{c.Issuer, "stores.trims.issue"}, {c.Recipient, "stores.trims.receive"}} {
		var allowed bool
		err = tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM workforce_identity_grants_v15 g JOIN workforce_chapter_actions_v17 p
    ON p.permission=g.permission AND p.chapter_id=4 AND p.authority='SIGNED_WRITE'
    WHERE g.tenant_id=$1 AND g.plant_id=$2 AND g.idp_issuer=$3 AND g.idp_subject=$4 AND g.permission=$5 AND g.enabled)`,
			c.TenantID, c.PlantID, item.proof.IDPIssuer, item.proof.Subject, item.permission).Scan(&allowed)
		if err != nil {
			return out, err
		}
		if !allowed {
			return out, errors.New("ERR_F314_WORKFORCE_GRANT_REQUIRED")
		}
	}
	var priorQty int64
	var stock, order, line, plant, issuer, recipient, issuerProof, recipientProof, hash string
	err = tx.QueryRowContext(ctx, `SELECT id::text,trim_stock_id,order_id,line_id,plant_id,
 ROUND(issued_qty*1000)::bigint,issuer_id,recipient_id,issuer_proof_id,recipient_proof_id,challenge_sha256
 FROM trim_issuances_v13 WHERE tenant_id=$1 AND command_id=$2`, c.TenantID, c.CommandID).
		Scan(&out.ID, &stock, &order, &line, &plant, &priorQty, &issuer, &recipient, &issuerProof, &recipientProof, &hash)
	if err == nil {
		if stock != c.TrimStockID || order != c.OrderID || line != c.LineID || plant != c.PlantID ||
			priorQty != c.QuantityMilli || issuer != c.Issuer.Subject || recipient != c.Recipient.Subject ||
			issuerProof != c.Issuer.ProofID || recipientProof != c.Recipient.ProofID || hash != TrimIssueChallenge(c) {
			return TrimIssueReceipt{}, errors.New("ERR_F314_COMMAND_CONFLICT")
		}
		out.TrimStockID, out.OrderID, out.QuantityMilli, out.ChallengeSHA256, out.Replayed, out.Authority = c.TrimStockID, c.OrderID, c.QuantityMilli, hash, true, "SIGNED_TWO_WITNESS_TRIM_ISSUANCE"
		return out, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO trim_issuances_v13
 (tenant_id,plant_id,trim_stock_id,order_id,line_id,issued_qty,issuer_id,recipient_id,command_id,
 issuer_proof_id,recipient_proof_id,challenge_sha256,issuer_assertion_sha256,recipient_assertion_sha256)
 VALUES($1,$2,$3,$4,$5,$6::numeric/1000,$7,$8,$9,$10,$11,$12,$13,$14) RETURNING id::text`,
		c.TenantID, c.PlantID, c.TrimStockID, c.OrderID, c.LineID, c.QuantityMilli, c.Issuer.Subject, c.Recipient.Subject, c.CommandID,
		c.Issuer.ProofID, c.Recipient.ProofID, TrimIssueChallenge(c), trimHash(c.Issuer.Assertion), trimHash(c.Recipient.Assertion)).Scan(&out.ID)
	if err != nil {
		if strings.Contains(err.Error(), "ERR_F314_") || strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			return out, errors.New("ERR_F314_STOCK_OR_PROOF_CONFLICT")
		}
		return out, fmt.Errorf("trim issuance database write: %w", err)
	}
	out.TrimStockID, out.OrderID, out.QuantityMilli, out.ChallengeSHA256, out.Authority = c.TrimStockID, c.OrderID, c.QuantityMilli, TrimIssueChallenge(c), "SIGNED_TWO_WITNESS_TRIM_ISSUANCE"
	return out, tx.Commit()
}

// TrimIssueCommand never accepts a caller-supplied witness identity from the JSON body.
func TrimSignedProof(tenant, plant, issuer, subject, proofID, assertion, role string) WitnessProof {
	return WitnessProof{TenantID: tenant, PlantID: plant, IDPIssuer: strings.TrimSpace(issuer), Subject: subject, ProofID: proofID, Assertion: assertion, Role: role}
}
