// Batch 08B: FIDO2-backed *human witnessed* bundle tag claims. A signed human
// assertion is NOT hardware reader attestation. No automatic production release.
package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/attestation"
)

type cutScanInput struct {
	DraftID            string `json:"draft_id"`
	LabelCode          string `json:"label_code"`
	EPCHex             string `json:"epc_hex"`
	WitnessAttestation string `json:"witness_attestation"`
}

func validateCutScanInput(v cutScanInput) error {
	if !cutDraftUUID.MatchString(v.DraftID) || len(v.LabelCode) != 29 || !strings.HasPrefix(v.LabelCode, "CB08-") {
		return errors.New("ERR_CUT_SCAN_TARGET")
	}
	for _, c := range v.LabelCode[5:] {
		if (c < '0' || c > '9') && (c < 'A' || c > 'F') {
			return errors.New("ERR_CUT_SCAN_TARGET")
		}
	}
	_, e := canonicalEPCHex(v.EPCHex)
	return e
}
func (a *materialAPI) cutScanChallengeRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", 405)
		return
	}
	if !a.ready(w) {
		return
	}
	actor, e := a.authorize(r, "cut.bundle.scan.prepare", "cutting_reader", true)
	if e != nil {
		materialError(w, e)
		return
	}
	v := cutScanInput{DraftID: r.URL.Query().Get("draft_id"), LabelCode: r.URL.Query().Get("label_code"), EPCHex: r.URL.Query().Get("epc_hex")}
	if e = validateCutScanInput(v); e != nil {
		materialError(w, e)
		return
	}
	v.EPCHex, _ = canonicalEPCHex(v.EPCHex)
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	tx, e := a.repository.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		materialError(w, e)
		return
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id',$1,true)`, actor.TenantID); e != nil {
		materialError(w, e)
		return
	}
	challenge, e := cutScanChallengeFromDB(ctx, tx, actor, v)
	if e != nil {
		materialScanError(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		materialError(w, e)
		return
	}
	respondMaterial(w, 200, map[string]any{"challenge": challenge, "action": "cut.bundle.scan.witness", "role": "cutting_reader", "draft_id": v.DraftID, "label_code": v.LabelCode, "epc_hex": v.EPCHex, "proof": "EdDSA JWT + strong FIDO2/WebAuthn authentication", "hardware_reader_attested": false})
}
func cutScanChallengeFromDB(ctx context.Context, tx *sql.Tx, actor attestation.Claims, v cutScanInput) (string, error) {
	var spreadID, snapshotHash, dbLabel string
	// The store issue roll's registered plant must match the real IdP assertion.
	// Every allocated issue must belong to the same plant.
	e := tx.QueryRowContext(ctx, `SELECT s.id::text,s.draft_snapshot_sha256,b.label_code
 FROM cut_spread_reports_v8 s JOIN cut_bundle_drafts_v8 b ON b.tenant_id=s.tenant_id AND b.lay_draft_id=s.lay_draft_id
 WHERE s.tenant_id=$1 AND s.lay_draft_id=$2::uuid AND b.label_code=$3`, actor.TenantID, v.DraftID, v.LabelCode).Scan(&spreadID, &snapshotHash, &dbLabel)
	if errors.Is(e, sql.ErrNoRows) {
		return "", errors.New("ERR_CUT_SCAN_SPREAD_OR_LABEL_MISSING")
	}
	if e != nil {
		return "", e
	}
	var issueCount, foreignPlant int
	e = tx.QueryRowContext(ctx, `SELECT count(*)::int,count(*) FILTER (WHERE sr.plant_id<>$3)::int
 FROM cut_spread_issue_allocations_v8 al JOIN material_issues_v5 mi ON mi.tenant_id=al.tenant_id AND mi.id=al.issue_id
 JOIN store_rolls sr ON sr.tenant_id=mi.tenant_id AND sr.id=mi.roll_id WHERE al.tenant_id=$1 AND al.spread_report_id=$2::uuid`, actor.TenantID, spreadID, actor.PlantID).Scan(&issueCount, &foreignPlant)
	if e != nil {
		return "", e
	}
	if issueCount == 0 || foreignPlant != 0 {
		return "", errors.New("ERR_CUT_SCAN_PLANT_MISMATCH")
	}
	fields := cutScanChallengeFields{Version: "CUT08B_SIGNED_SCAN_V1", TenantID: actor.TenantID, PlantID: actor.PlantID, SpreadID: spreadID, DraftID: v.DraftID, SnapshotSHA: snapshotHash, LabelCode: v.LabelCode, EPCHex: v.EPCHex}
	return cutScanChallenge(fields), nil
}
func materialScanError(w http.ResponseWriter, e error) {
	if strings.HasPrefix(e.Error(), "ERR_CUT_") {
		respondMaterial(w, 409, map[string]string{"error": e.Error()})
		return
	}
	materialError(w, e)
}
func (a *materialAPI) cutSignedScanRoute(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !a.ready(w) {
		return
	}
	actor, e := a.authorize(r, "cut.bundle.scan.submit", "cutting_reader", true)
	if e != nil {
		materialError(w, e)
		return
	}
	var v cutScanInput
	if e = decodeMaterialJSON(w, r, &v); e != nil {
		http.Error(w, "Invalid scan JSON", 400)
		return
	}
	if e = validateCutScanInput(v); e != nil {
		materialScanError(w, e)
		return
	}
	if v.WitnessAttestation == "" {
		materialScanError(w, errors.New("ERR_CUT_SCAN_WITNESS_REQUIRED"))
		return
	}
	v.EPCHex, _ = canonicalEPCHex(v.EPCHex)
	ctx, cancel := context.WithTimeout(r.Context(), 18*time.Second)
	defer cancel()
	receipt, e := a.persistCutSignedScan(ctx, actor, v)
	if e != nil {
		materialScanError(w, e)
		return
	}
	respondMaterial(w, 201, receipt)
}
func (a *materialAPI) persistCutSignedScan(ctx context.Context, actor attestation.Claims, v cutScanInput) (map[string]any, error) {
	tx, e := a.repository.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id',$1,true)`, actor.TenantID); e != nil {
		return nil, e
	}
	// Lock prevents a conflicting claim on this tenant while re-checking the
	// immutable spread/issue identity and performing the unique EPC write.
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1),hashtext($2))`, actor.TenantID, "CUT08B:SCAN_WITNESSES"); e != nil {
		return nil, e
	}
	challenge, e := cutScanChallengeFromDB(ctx, tx, actor, v)
	if e != nil {
		return nil, e
	}
	witness, e := a.verifier.Verify(v.WitnessAttestation, "cut.bundle.scan.witness", challenge, "cutting_reader", true)
	if e != nil {
		return nil, e
	}
	if witness.Subject != actor.Subject || witness.TenantID != actor.TenantID || witness.PlantID != actor.PlantID || witness.JWTID == "" {
		return nil, errors.New("ERR_CUT_SCAN_WITNESS_SCOPE")
	}
	tokenHash := sha256.Sum256([]byte(v.WitnessAttestation))
	var id string
	var when time.Time
	e = tx.QueryRowContext(ctx, `INSERT INTO cut_bundle_scan_witnesses_v8
 (tenant_id,plant_id,spread_report_id,lay_draft_id,label_code,epc_hex,scanner_subject,scanner_proof_id,scanner_assertion_hash,signed_challenge_hash)
 SELECT $1,$2,s.id,$3::uuid,$4,$5,$6,$7,$8,$9 FROM cut_spread_reports_v8 s
 WHERE s.tenant_id=$1 AND s.lay_draft_id=$3::uuid
 RETURNING id::text,created_at`, actor.TenantID, actor.PlantID, v.DraftID, v.LabelCode, v.EPCHex, witness.Subject, witness.JWTID, hex.EncodeToString(tokenHash[:]), challenge).Scan(&id, &when)
	if errors.Is(e, sql.ErrNoRows) {
		return nil, errors.New("ERR_CUT_SCAN_SPREAD_MISSING")
	}
	if e != nil {
		return nil, e
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return map[string]any{"id": id, "created_at": when.UTC().Format(time.RFC3339Nano), "draft_id": v.DraftID, "label_code": v.LabelCode, "epc_hex": v.EPCHex, "witness_subject": witness.Subject, "state": "SIGNED_HUMAN_WITNESS_NOT_HARDWARE_ATTESTED", "production_released": false}, nil
}
