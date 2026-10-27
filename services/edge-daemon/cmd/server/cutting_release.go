// Independently signed release from Cutting to Sewing. The development operator
// may inspect evidence but cannot release bundles.
package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/attestation"
)

type cutReleaseEvidence struct {
	Version       string            `json:"version"`
	TenantID      string            `json:"tenant_id"`
	PlantID       string            `json:"plant_id"`
	SpreadID      string            `json:"spread_id"`
	DraftID       string            `json:"draft_id"`
	SnapshotSHA   string            `json:"snapshot_sha256"`
	ConsumptionMM int64             `json:"consumption_mm"`
	BundleCount   int               `json:"bundle_count"`
	Entries       []cutReleaseEntry `json:"entries"`
	Issues        []cutReleaseIssue `json:"issues"`
}
type cutReleaseIssue struct {
	IssueID     string `json:"issue_id"`
	RollID      string `json:"roll_id"`
	AllocatedMM int64  `json:"allocated_mm"`
}
type cutReleaseEntry struct {
	LabelCode      string `json:"label_code"`
	EPCHex         string `json:"epc_hex"`
	HumanProofID   string `json:"human_proof_id"`
	ReaderGateway  string `json:"reader_gateway"`
	ReaderEventID  string `json:"reader_event_id"`
	ReaderEventSHA string `json:"reader_event_sha256"`
}
type cutReleaseState struct {
	SpreadID        string   `json:"spread_id"`
	DraftID         string   `json:"draft_id"`
	Expected        int      `json:"expected_bundles"`
	HumanWitnessed  int      `json:"human_witnessed_bundles"`
	GatewayReported int      `json:"gateway_reported_bundles"`
	Matched         int      `json:"matched_bundles"`
	PlantMatched    bool     `json:"plant_matched"`
	IssuanceBacked  bool     `json:"issuance_backed"`
	Released        bool     `json:"released"`
	Ready           bool     `json:"ready_for_signed_release"`
	Reasons         []string `json:"reasons"`
	EvidenceSHA     string   `json:"evidence_sha256,omitempty"`
}

func cutEvidenceHash(v cutReleaseEvidence) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func loadCutReleaseEvidence(ctx context.Context, tx *sql.Tx, tenant, plant, spreadID string) (cutReleaseEvidence, cutReleaseState, error) {
	var ev cutReleaseEvidence
	var st cutReleaseState
	ev.Version = "CUT_SEWING_RELEASE_V1"
	ev.TenantID = tenant
	ev.PlantID = plant
	ev.SpreadID = spreadID
	ev.Entries = []cutReleaseEntry{}
	ev.Issues = []cutReleaseIssue{}
	st.SpreadID = spreadID
	st.Reasons = []string{}
	var count int
	e := tx.QueryRowContext(ctx, `SELECT s.lay_draft_id::text,s.draft_snapshot_sha256,s.reported_consumption_mm,
 (SELECT count(*)::int FROM cut_bundle_drafts_v8 b WHERE b.tenant_id=s.tenant_id AND b.lay_draft_id=s.lay_draft_id),
 (SELECT count(*)::int FROM cut_sewing_releases_v8 x WHERE x.tenant_id=s.tenant_id AND x.spread_report_id=s.id)
 FROM cut_spread_reports_v8 s WHERE s.tenant_id=$1 AND s.id=$2::uuid`, tenant, spreadID).Scan(&ev.DraftID, &ev.SnapshotSHA, &ev.ConsumptionMM, &ev.BundleCount, &count)
	if e != nil {
		return ev, st, e
	}
	st.DraftID = ev.DraftID
	st.Expected = ev.BundleCount
	st.Released = count != 0
	e = tx.QueryRowContext(ctx, `SELECT count(*)::int FROM cut_bundle_scan_witnesses_v8 WHERE tenant_id=$1 AND spread_report_id=$2::uuid AND plant_id=$3`, tenant, spreadID, plant).Scan(&st.HumanWitnessed)
	if e != nil {
		return ev, st, e
	}
	e = tx.QueryRowContext(ctx, `SELECT count(*)::int FROM cut_reader_events_v8 WHERE tenant_id=$1 AND spread_report_id=$2::uuid AND plant_id=$3`, tenant, spreadID, plant).Scan(&st.GatewayReported)
	if e != nil {
		return ev, st, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT b.label_code,w.epc_hex,w.scanner_proof_id,g.gateway_id,g.event_id,g.event_sha256
 FROM cut_bundle_drafts_v8 b
 JOIN cut_bundle_scan_witnesses_v8 w ON w.tenant_id=b.tenant_id AND w.lay_draft_id=b.lay_draft_id AND w.label_code=b.label_code AND w.plant_id=$3
 JOIN cut_reader_events_v8 g ON g.tenant_id=w.tenant_id AND g.spread_report_id=w.spread_report_id AND g.label_code=w.label_code AND g.epc_hex=w.epc_hex AND g.plant_id=$3
 JOIN cut_reader_gateways_v8 device ON device.tenant_id=g.tenant_id AND device.gateway_id=g.gateway_id AND device.enabled AND device.plant_id=$3
 WHERE b.tenant_id=$1 AND b.lay_draft_id=$2::uuid AND w.spread_report_id=$4::uuid
 ORDER BY b.label_code`, tenant, ev.DraftID, plant, spreadID)
	if e != nil {
		return ev, st, e
	}
	for rows.Next() {
		var entry cutReleaseEntry
		if e = rows.Scan(&entry.LabelCode, &entry.EPCHex, &entry.HumanProofID, &entry.ReaderGateway, &entry.ReaderEventID, &entry.ReaderEventSHA); e != nil {
			break
		}
		ev.Entries = append(ev.Entries, entry)
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		return ev, st, e
	}
	st.Matched = len(ev.Entries)
	// All allocated issues must belong to this plant, and attribution total
	// must match the reported measured millimetres.
	var issueCount, foreign int
	var attributed int64
	e = tx.QueryRowContext(ctx, `SELECT count(*)::int,COALESCE(sum(a.allocated_mm),0)::bigint,
 count(*) FILTER(WHERE sr.plant_id<>$3)::int
 FROM cut_spread_issue_allocations_v8 a JOIN material_issues_v5 i ON i.tenant_id=a.tenant_id AND i.id=a.issue_id
 JOIN store_rolls sr ON sr.tenant_id=i.tenant_id AND sr.id=i.roll_id
 WHERE a.tenant_id=$1 AND a.spread_report_id=$2::uuid`, tenant, spreadID, plant).Scan(&issueCount, &attributed, &foreign)
	if e != nil {
		return ev, st, e
	}
	issueRows, e := tx.QueryContext(ctx, `SELECT a.issue_id::text,i.roll_id,a.allocated_mm FROM cut_spread_issue_allocations_v8 a JOIN material_issues_v5 i ON i.tenant_id=a.tenant_id AND i.id=a.issue_id WHERE a.tenant_id=$1 AND a.spread_report_id=$2::uuid ORDER BY a.issue_id::text`, tenant, spreadID)
	if e != nil {
		return ev, st, e
	}
	for issueRows.Next() {
		var item cutReleaseIssue
		if e = issueRows.Scan(&item.IssueID, &item.RollID, &item.AllocatedMM); e != nil {
			break
		}
		ev.Issues = append(ev.Issues, item)
	}
	if e == nil {
		e = issueRows.Err()
	}
	issueRows.Close()
	if e != nil {
		return ev, st, e
	}
	st.IssuanceBacked = issueCount > 0 && attributed == ev.ConsumptionMM
	st.PlantMatched = foreign == 0
	if !st.IssuanceBacked {
		st.Reasons = append(st.Reasons, "SIGNED_ISSUANCE_COVERAGE_INCOMPLETE")
	}
	if !st.PlantMatched {
		st.Reasons = append(st.Reasons, "PLANT_SCOPE_MISMATCH")
	}
	if st.Expected == 0 || st.HumanWitnessed != st.Expected {
		st.Reasons = append(st.Reasons, "HUMAN_EPC_WITNESSES_INCOMPLETE")
	}
	if st.Expected == 0 || st.GatewayReported != st.Expected {
		st.Reasons = append(st.Reasons, "ENROLLED_READER_EVENTS_INCOMPLETE")
	}
	if st.Expected == 0 || st.Matched != st.Expected {
		st.Reasons = append(st.Reasons, "HUMAN_AND_READER_EPCS_DISAGREE")
	}
	if st.Released {
		st.Reasons = append(st.Reasons, "ALREADY_RELEASED")
	}
	st.Ready = len(st.Reasons) == 0
	if st.Ready {
		st.EvidenceSHA = cutEvidenceHash(ev)
	}
	return ev, st, nil
}

// Customer workbench: reads counts only. This is not a signature endpoint.
func (s *EdgeServer) handleCuttingReleaseStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", 405)
		return
	}
	tenant, e := intakeTenant()
	if e != nil || s.db == nil {
		docJSON(w, 503, map[string]string{"error": "ERR_CUT_RELEASE_DB"})
		return
	}
	draft := r.URL.Query().Get("draft_id")
	if !cutDraftUUID.MatchString(draft) {
		docJSON(w, 400, map[string]string{"error": "ERR_CUT_DRAFT_ID"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	tx, e := s.docTransaction(ctx, tenant)
	if e != nil {
		http.Error(w, "Unable to read release", 503)
		return
	}
	defer tx.Rollback()
	var spread string
	e = tx.QueryRowContext(ctx, `SELECT id::text FROM cut_spread_reports_v8 WHERE tenant_id=$1 AND lay_draft_id=$2::uuid`, tenant, draft).Scan(&spread)
	if errors.Is(e, sql.ErrNoRows) {
		docJSON(w, 200, map[string]any{"state": "NO_SPREAD_EVIDENCE", "released": false, "ready_for_signed_release": false, "identity_enabled": false})
		return
	}
	if e != nil {
		http.Error(w, "Unable to read release", 503)
		return
	}
	// The dev view has no trusted plant context; display evidence counts by
	// spread, but do not claim readiness until a signed plant-scoped request.
	var expected, human, gateway, matched, released int
	e = tx.QueryRowContext(ctx, `SELECT
 (SELECT count(*) FROM cut_bundle_drafts_v8 b WHERE b.tenant_id=s.tenant_id AND b.lay_draft_id=s.lay_draft_id),
 (SELECT count(*) FROM cut_bundle_scan_witnesses_v8 w WHERE w.tenant_id=s.tenant_id AND w.spread_report_id=s.id),
 (SELECT count(*) FROM cut_reader_events_v8 g WHERE g.tenant_id=s.tenant_id AND g.spread_report_id=s.id),
 (SELECT count(*) FROM cut_reader_events_v8 g JOIN cut_bundle_scan_witnesses_v8 w ON w.tenant_id=g.tenant_id AND w.spread_report_id=g.spread_report_id AND w.label_code=g.label_code AND w.epc_hex=g.epc_hex WHERE g.tenant_id=s.tenant_id AND g.spread_report_id=s.id),
 (SELECT count(*) FROM cut_sewing_releases_v8 x WHERE x.tenant_id=s.tenant_id AND x.spread_report_id=s.id)
 FROM cut_spread_reports_v8 s WHERE s.tenant_id=$1 AND s.id=$2::uuid`, tenant, spread).Scan(&expected, &human, &gateway, &matched, &released)
	if e != nil {
		http.Error(w, "Unable to read release", 503)
		return
	}
	if e = tx.Commit(); e != nil {
		http.Error(w, "Unable to read release", 503)
		return
	}
	docJSON(w, 200, map[string]any{"spread_id": spread, "draft_id": draft, "expected_bundles": expected, "human_witnessed_bundles": human, "gateway_reported_bundles": gateway, "matched_bundles": matched, "released": released > 0, "ready_for_signed_release": false, "read_only": "Plant-scoped readiness and two-person approval require external identity"})
}

type cutReleaseInput struct {
	SpreadID              string `json:"spread_id"`
	SupervisorAttestation string `json:"supervisor_attestation"`
	QualityAttestation    string `json:"quality_attestation"`
}

func (a *materialAPI) cutReleaseChallengeRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", 405)
		return
	}
	if !a.ready(w) {
		return
	}
	actor, e := a.authorize(r, "cut.release.prepare", "cutting_supervisor", true)
	if e != nil {
		materialError(w, e)
		return
	}
	id := r.URL.Query().Get("spread_id")
	if !cutDraftUUID.MatchString(id) {
		materialScanError(w, errors.New("ERR_CUT_RELEASE_ID"))
		return
	}
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
	_, status, e := loadCutReleaseEvidence(ctx, tx, actor.TenantID, actor.PlantID, id)
	if errors.Is(e, sql.ErrNoRows) {
		materialScanError(w, errors.New("ERR_CUT_RELEASE_SPREAD_MISSING"))
		return
	}
	if e != nil {
		materialError(w, e)
		return
	}
	if !status.Ready {
		respondMaterial(w, 409, map[string]any{"error": "ERR_CUT_RELEASE_NOT_READY", "status": status})
		return
	}
	if e = tx.Commit(); e != nil {
		materialError(w, e)
		return
	}
	respondMaterial(w, 200, map[string]any{"challenge": status.EvidenceSHA, "spread_id": id, "bundle_count": status.Expected, "action": "cut.release.witness", "supervisor_role": "cutting_supervisor", "quality_role": "quality_inspector", "proof": "Strong external EdDSA assertion (FIDO2/WebAuthn)", "note": "Gateway provenance is not independent RF-physics certification"})
}
func (a *materialAPI) cutReleaseRoute(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !a.ready(w) {
		return
	}
	actor, e := a.authorize(r, "cut.release.submit", "cutting_supervisor", true)
	if e != nil {
		materialError(w, e)
		return
	}
	var input cutReleaseInput
	if e = decodeMaterialJSON(w, r, &input); e != nil {
		http.Error(w, "Bad JSON", 400)
		return
	}
	if !cutDraftUUID.MatchString(input.SpreadID) || input.SupervisorAttestation == "" || input.QualityAttestation == "" {
		materialScanError(w, errors.New("ERR_CUT_RELEASE_INPUT"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 18*time.Second)
	defer cancel()
	receipt, e := a.saveCutRelease(ctx, actor, input)
	if e != nil {
		materialScanError(w, e)
		return
	}
	respondMaterial(w, 201, receipt)
}
func (a *materialAPI) saveCutRelease(ctx context.Context, actor attestation.Claims, v cutReleaseInput) (map[string]any, error) {
	tx, e := a.repository.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id',$1,true)`, actor.TenantID); e != nil {
		return nil, e
	}
	// Serialize against gateway writes while snapshotting the complete set.
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1),hashtext($2))`, actor.TenantID, "CUT08C:READER_EVENTS"); e != nil {
		return nil, e
	}
	_, st, e := loadCutReleaseEvidence(ctx, tx, actor.TenantID, actor.PlantID, v.SpreadID)
	if errors.Is(e, sql.ErrNoRows) {
		return nil, errors.New("ERR_CUT_RELEASE_SPREAD_MISSING")
	}
	if e != nil {
		return nil, e
	}
	if !st.Ready {
		return nil, errors.New("ERR_CUT_RELEASE_NOT_READY")
	}
	sup, e := a.verifier.Verify(v.SupervisorAttestation, "cut.release.witness", st.EvidenceSHA, "cutting_supervisor", true)
	if e != nil {
		return nil, e
	}
	qc, e := a.verifier.Verify(v.QualityAttestation, "cut.release.witness", st.EvidenceSHA, "quality_inspector", true)
	if e != nil {
		return nil, e
	}
	if sup.TenantID != actor.TenantID || qc.TenantID != actor.TenantID || sup.PlantID != actor.PlantID || qc.PlantID != actor.PlantID || sup.Subject != actor.Subject || qc.Subject == sup.Subject || sup.JWTID == qc.JWTID {
		return nil, errors.New("ERR_CUT_RELEASE_INDEPENDENT_WITNESSES_REQUIRED")
	}
	sHash := sha256.Sum256([]byte(v.SupervisorAttestation))
	qHash := sha256.Sum256([]byte(v.QualityAttestation))
	var id string
	var created time.Time
	e = tx.QueryRowContext(ctx, `INSERT INTO cut_sewing_releases_v8
 (tenant_id,plant_id,spread_report_id,lay_draft_id,evidence_sha256,supervisor_subject,quality_subject,supervisor_proof_id,quality_proof_id,supervisor_assertion_sha256,quality_assertion_sha256,bundle_count)
 VALUES($1,$2,$3::uuid,$4::uuid,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id::text,created_at`, actor.TenantID, actor.PlantID, v.SpreadID, st.DraftID, st.EvidenceSHA, sup.Subject, qc.Subject, sup.JWTID, qc.JWTID, hex.EncodeToString(sHash[:]), hex.EncodeToString(qHash[:]), st.Expected).Scan(&id, &created)
	if e != nil {
		return nil, e
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return map[string]any{"id": id, "spread_id": v.SpreadID, "draft_id": st.DraftID, "bundle_count": st.Expected, "released_at": created.UTC().Format(time.RFC3339Nano), "state": "RELEASED_TO_SEWING_SIGNED", "evidence_sha256": st.EvidenceSHA}, nil
}
