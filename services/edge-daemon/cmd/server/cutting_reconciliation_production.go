// Author: Sunil Jain, 2026-10-28.
// F3.2: append-only, independently signed material and cut-part reconciliation.
// No device measurement, garment completion, QC release or shipping credit implied.
package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/attestation"
)

const f32ChallengePrefix = "CUT_F32_V1:"
const f32ReadChallenge = "CUT_F32_READ_V1"
const f32HumanEvidence = "DUAL_HUMAN_ATTESTATION_NOT_SENSOR_PROOF"

type f32Command struct {
	CommandID         string `json:"command_id"`
	PlantID           string `json:"plant_id"`
	Action            string `json:"action"`
	ReleaseID         string `json:"release_id"`
	SpreadID          string `json:"spread_id"`
	PartID            string `json:"part_id"`
	ReplacementPartID string `json:"replacement_part_id"`
	UsedPartMM        int64  `json:"used_part_mm"`
	TrimWasteMM       int64  `json:"trim_waste_mm"`
	DamagedMM         int64  `json:"damaged_mm"`
	DefectCode        string `json:"defect_code"`
	Note              string `json:"note"`
}
type f32Request struct {
	Command          f32Command `json:"command"`
	QualityAssertion string     `json:"quality_assertion"`
}
type f32API struct {
	gen     *genProductionAPI
	enabled bool
	reason  string
}

func newF32API(s *EdgeServer) *f32API {
	p := &f32API{reason: "ERR_F32_DISABLED"}
	if os.Getenv("AGIPAREL_CUTTING_RECONCILIATION_ENABLED") != "true" {
		return p
	}
	p.gen = newGenProductionAPI(s)
	if !p.gen.enabled {
		p.reason = "ERR_F32_GENEALOGY_PREREQUISITE"
		return p
	}
	p.enabled = true
	return p
}
func (p *f32API) ready(w http.ResponseWriter) bool {
	if p == nil || !p.enabled {
		reason := "ERR_F32_DISABLED"
		if p != nil {
			reason = p.reason
		}
		docJSON(w, 503, map[string]string{"error": reason})
		return false
	}
	return true
}
func f32Valid(c f32Command) bool {
	if !sewCommandID.MatchString(c.CommandID) || !sewFields(c.PlantID) || !sewUUID(c.ReleaseID) || len(strings.TrimSpace(c.Note)) < 12 || len(c.Note) > 1000 || strings.ContainsRune(c.Note, 0) {
		return false
	}
	switch c.Action {
	case "RECONCILE":
		if !sewUUID(c.SpreadID) || c.PartID != "" || c.ReplacementPartID != "" || c.DefectCode != "" || c.UsedPartMM < 0 || c.TrimWasteMM < 0 || c.DamagedMM < 0 {
			return false
		}
		// No overflow: positive classified terms must fit an ordinary signed bigint.
		if c.UsedPartMM > 0 && c.TrimWasteMM > 9223372036854775807-c.UsedPartMM {
			return false
		}
		return c.DamagedMM <= 9223372036854775807-c.UsedPartMM-c.TrimWasteMM && c.UsedPartMM+c.TrimWasteMM+c.DamagedMM > 0
	case "ACCEPT_PART":
		return sewUUID(c.PartID) && c.SpreadID == "" && c.ReplacementPartID == "" && c.DefectCode == "" && c.UsedPartMM == 0 && c.TrimWasteMM == 0 && c.DamagedMM == 0
	case "REJECT_PART":
		return sewUUID(c.PartID) && c.SpreadID == "" && c.ReplacementPartID == "" && sewFields(c.DefectCode) && len(c.DefectCode) <= 64 && c.UsedPartMM == 0 && c.TrimWasteMM == 0 && c.DamagedMM == 0
	case "LINK_RECUT":
		return sewUUID(c.PartID) && sewUUID(c.ReplacementPartID) && c.PartID != c.ReplacementPartID && c.SpreadID == "" && c.DefectCode == "" && c.UsedPartMM == 0 && c.TrimWasteMM == 0 && c.DamagedMM == 0
	}
	return false
}
func f32Challenge(c f32Command) string { return f32ChallengePrefix + sewDigest(c) }
func (p *f32API) challenge(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var v f32Request
	if !sewProdStrictJSON(w, r, &v) {
		return
	}
	if !f32Valid(v.Command) {
		docJSON(w, 422, map[string]string{"error": "ERR_F32_FIELDS"})
		return
	}
	docJSON(w, 200, map[string]any{"challenge": f32Challenge(v.Command), "supervisor_action": "cutting.release", "quality_action": "quality.inspect", "roles": []string{"cutting_supervisor", "quality_inspector"}, "dual_approval": true, "physical_sensor_verified": false, "production_credit": false})
}
func (p *f32API) claims(w http.ResponseWriter, r *http.Request, cmd f32Command, qa string) (attestation.Claims, attestation.Claims, string, bool) {
	raw, err := bearer(r)
	if err != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_F32_SUPERVISOR_BEARER"})
		return attestation.Claims{}, attestation.Claims{}, "", false
	}
	challenge := f32Challenge(cmd)
	sup, e1 := p.gen.sewing.verifier.Verify(raw, "cutting.release", challenge, "cutting_supervisor", true)
	qual, e2 := p.gen.sewing.verifier.Verify(qa, "quality.inspect", challenge, "quality_inspector", true)
	if e1 != nil || e2 != nil || !sewFields(sup.TenantID, sup.PlantID, qual.TenantID, qual.PlantID) || strings.TrimSpace(sup.Subject) == "" || strings.TrimSpace(qual.Subject) == "" ||
		len(sup.Subject) > 128 || len(qual.Subject) > 128 || len(sup.JWTID) > 255 || len(qual.JWTID) > 255 ||
		sup.TenantID != qual.TenantID || sup.PlantID != qual.PlantID || sup.PlantID != cmd.PlantID || sup.Subject == qual.Subject || sup.JWTID == qual.JWTID || sup.Issuer != qual.Issuer {
		docJSON(w, 401, map[string]string{"error": "ERR_F32_INDEPENDENT_ASSERTIONS_REQUIRED"})
		return attestation.Claims{}, attestation.Claims{}, "", false
	}
	return sup, qual, raw, true
}
func (p *f32API) write(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var v f32Request
	if !sewProdStrictJSON(w, r, &v) {
		return
	}
	c := v.Command
	if !f32Valid(c) {
		docJSON(w, 422, map[string]string{"error": "ERR_F32_FIELDS"})
		return
	}
	sup, qual, supRaw, ok := p.claims(w, r, c, v.QualityAssertion)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	id, replay, err := p.writeTxn(ctx, c, sup, qual, supRaw, v.QualityAssertion)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	status := 201
	if replay {
		status = 200
	}
	docJSON(w, status, map[string]any{"event_id": id, "action": c.Action, "replayed": replay, "evidence": f32HumanEvidence, "sensor_verified": false, "finished_inventory": false, "shipping_release": false, "payroll_credit": false})
}
func (p *f32API) writeTxn(ctx context.Context, c f32Command, sup, qual attestation.Claims, supRaw, qualRaw string) (string, bool, error) {
	tx, err := p.gen.sewing.server.docTransaction(ctx, sup.TenantID)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback()
	if err = workforceChapterRequire(ctx, tx, sup, "cutting.release"); err != nil {
		return "", false, err
	}
	if err = workforceChapterRequire(ctx, tx, qual, "quality.inspect"); err != nil {
		return "", false, err
	}
	// One writer per command; distinct commands for same physical part are serialized
	// by the UNIQUE part_id key, with any constraint failure returned as conflict.
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1),hashtext($2))`, sup.TenantID, "F32:"+c.CommandID); err != nil {
		return "", false, err
	}
	table := ""
	switch c.Action {
	case "RECONCILE":
		table = "cut_consumption_reconciliations_v19"
	case "ACCEPT_PART", "REJECT_PART":
		table = "cut_part_outcomes_v19"
	case "LINK_RECUT":
		table = "cut_recut_links_v19"
	}
	hash := sewDigest(c)
	// Identifiers are constants from an exact switch, never user-supplied SQL.
	var existingID, existingHash, existingSubject, existingQuality string
	err = tx.QueryRowContext(ctx, `SELECT id::text,command_sha256,supervisor_subject,quality_subject FROM `+table+` WHERE tenant_id=$1 AND command_id=$2`, sup.TenantID, c.CommandID).Scan(&existingID, &existingHash, &existingSubject, &existingQuality)
	if err == nil {
		if existingHash != hash || existingSubject != sup.Subject || existingQuality != qual.Subject {
			return "", false, sewFail(409, "ERR_F32_COMMAND_REUSE")
		}
		return existingID, true, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, err
	}
	var id string
	switch c.Action {
	case "RECONCILE":
		err = tx.QueryRowContext(ctx, `INSERT INTO cut_consumption_reconciliations_v19
   (tenant_id,plant_id,release_id,spread_report_id,used_part_mm,trim_waste_mm,damaged_mm,command_id,command_sha256,
    supervisor_subject,quality_subject,supervisor_proof_id,quality_proof_id,supervisor_assertion_sha256,quality_assertion_sha256,note)
   VALUES($1,$2,$3::uuid,$4::uuid,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16) RETURNING id::text`,
			sup.TenantID, c.PlantID, c.ReleaseID, c.SpreadID, c.UsedPartMM, c.TrimWasteMM, c.DamagedMM, c.CommandID, hash, sup.Subject, qual.Subject, sup.JWTID, qual.JWTID, sewProdHashString(supRaw), sewProdHashString(qualRaw), c.Note).Scan(&id)
	case "ACCEPT_PART", "REJECT_PART":
		disposition := "ACCEPTED"
		var defect any
		if c.Action == "REJECT_PART" {
			disposition = "REJECTED"
			defect = c.DefectCode
		}
		err = tx.QueryRowContext(ctx, `INSERT INTO cut_part_outcomes_v19
   (tenant_id,plant_id,part_id,release_id,disposition,defect_code,command_id,command_sha256,
    supervisor_subject,quality_subject,supervisor_proof_id,quality_proof_id,supervisor_assertion_sha256,quality_assertion_sha256,note)
   VALUES($1,$2,$3::uuid,$4::uuid,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) RETURNING id::text`,
			sup.TenantID, c.PlantID, c.PartID, c.ReleaseID, disposition, defect, c.CommandID, hash, sup.Subject, qual.Subject, sup.JWTID, qual.JWTID, sewProdHashString(supRaw), sewProdHashString(qualRaw), c.Note).Scan(&id)
	case "LINK_RECUT":
		err = tx.QueryRowContext(ctx, `INSERT INTO cut_recut_links_v19
   (tenant_id,plant_id,release_id,original_part_id,replacement_part_id,command_id,command_sha256,
    supervisor_subject,quality_subject,supervisor_proof_id,quality_proof_id,supervisor_assertion_sha256,quality_assertion_sha256,note)
   VALUES($1,$2,$3::uuid,$4::uuid,$5::uuid,$6,$7,$8,$9,$10,$11,$12,$13,$14) RETURNING id::text`,
			sup.TenantID, c.PlantID, c.ReleaseID, c.PartID, c.ReplacementPartID, c.CommandID, hash, sup.Subject, qual.Subject, sup.JWTID, qual.JWTID, sewProdHashString(supRaw), sewProdHashString(qualRaw), c.Note).Scan(&id)
	}
	if err != nil {
		return "", false, sewFail(409, "ERR_F32_SOURCE_OR_DUPLICATE_CONFLICT")
	}
	for _, proof := range []struct{ id, actor string }{{sup.JWTID, sup.Subject}, {qual.JWTID, qual.Subject}} {
		if _, err = tx.ExecContext(ctx, `INSERT INTO cut_proof_consumptions_v19(tenant_id,proof_id,entity_type,entity_id,actor_subject)
    VALUES($1,$2,$3,$4::uuid,$5)`, sup.TenantID, proof.id, map[string]string{"RECONCILE": "CONSUMPTION", "ACCEPT_PART": "PART", "REJECT_PART": "PART", "LINK_RECUT": "RECUT"}[c.Action], id, proof.actor); err != nil {
			return "", false, sewFail(409, "ERR_F32_ASSERTION_ALREADY_USED")
		}
	}
	if err = tx.Commit(); err != nil {
		return "", false, sewFail(409, "ERR_F32_COMMIT_CONFLICT")
	}
	return id, false, nil
}
func (p *f32API) balance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	if !p.ready(w) {
		return
	}
	q := r.URL.Query()
	if len(q["release_id"]) != 1 || !sewUUID(q.Get("release_id")) || len(q) > 1 {
		docJSON(w, 422, map[string]string{"error": "ERR_F32_BALANCE_QUERY"})
		return
	}
	a, _, ok := p.gen.actor(w, r, "cutting.read", f32ReadChallenge)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	tx, err := p.gen.sewing.server.docTransaction(ctx, a.TenantID)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	defer tx.Rollback()
	if err = workforceChapterRequire(ctx, tx, a, "cutting.read"); err != nil {
		sewProdReply(w, err)
		return
	}
	var release, spread, lay string
	var consumption int64
	err = tx.QueryRowContext(ctx, `SELECT r.id::text,r.spread_report_id::text,r.lay_draft_id::text,s.reported_consumption_mm FROM cut_sewing_releases_v8 r
 JOIN cut_spread_reports_v8 s ON s.tenant_id=r.tenant_id AND s.id=r.spread_report_id
 WHERE r.tenant_id=$1 AND r.plant_id=$2 AND r.id=$3::uuid`, a.TenantID, a.PlantID, q.Get("release_id")).Scan(&release, &spread, &lay, &consumption)
	if errors.Is(err, sql.ErrNoRows) {
		docJSON(w, 404, map[string]string{"error": "ERR_F32_RELEASE_NOT_FOUND"})
		return
	}
	if err != nil {
		sewProdReply(w, err)
		return
	}
	var used, trim, damaged sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT used_part_mm,trim_waste_mm,damaged_mm FROM cut_consumption_reconciliations_v19 WHERE tenant_id=$1 AND release_id=$2::uuid`, a.TenantID, release).Scan(&used, &trim, &damaged)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		sewProdReply(w, err)
		return
	}
	var expectedPartSlots int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(sum(piece_count),0)::bigint FROM cut_bundle_drafts_v8 WHERE tenant_id=$1 AND lay_draft_id=$2::uuid`, a.TenantID, lay).Scan(&expectedPartSlots); err != nil {
		sewProdReply(w, err)
		return
	}
	var reserved, accepted, rejected, unresolved, recuts, replacementAccepted int64
	err = tx.QueryRowContext(ctx, `SELECT
 (SELECT count(*) FROM gen_cut_parts_v18 p JOIN gen_garments_v18 g ON g.tenant_id=p.tenant_id AND g.id=p.garment_id WHERE g.tenant_id=$1 AND g.release_id=$2::uuid),
 (SELECT count(*) FROM cut_part_outcomes_v19 WHERE tenant_id=$1 AND release_id=$2::uuid AND disposition='ACCEPTED'),
 (SELECT count(*) FROM cut_part_outcomes_v19 WHERE tenant_id=$1 AND release_id=$2::uuid AND disposition='REJECTED'),
 (SELECT count(*) FROM cut_part_outcomes_v19 o WHERE o.tenant_id=$1 AND o.release_id=$2::uuid AND o.disposition='REJECTED' AND NOT EXISTS
  (SELECT 1 FROM cut_recut_links_v19 x WHERE x.tenant_id=o.tenant_id AND x.original_part_id=o.part_id)),
 (SELECT count(*) FROM cut_recut_links_v19 WHERE tenant_id=$1 AND release_id=$2::uuid),
 (SELECT count(*) FROM cut_recut_links_v19 l JOIN cut_part_outcomes_v19 o ON o.tenant_id=l.tenant_id AND o.part_id=l.replacement_part_id AND o.disposition='ACCEPTED' WHERE l.tenant_id=$1 AND l.release_id=$2::uuid)`, a.TenantID, release).Scan(&reserved, &accepted, &rejected, &unresolved, &recuts, &replacementAccepted)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		sewProdReply(w, err)
		return
	}
	var classified any
	if used.Valid {
		classified = map[string]int64{"used_part_mm": used.Int64, "trim_waste_mm": trim.Int64, "damaged_mm": damaged.Int64}
	}
	docJSON(w, 200, map[string]any{"release_id": release, "spread_id": spread, "plant_id": a.PlantID, "signed_issued_consumption_mm": consumption, "human_classification": classified,
		"expected_part_slots_from_released_lay": expectedPartSlots, "reserved_part_identities": reserved, "unreserved_part_slots": expectedPartSlots - reserved, "attested_accepted_parts": accepted, "attested_rejected_parts": rejected, "undecided_reserved_parts": reserved - accepted - rejected,
		"rejected_parts_without_recuts": unresolved, "linked_recut_parts": recuts, "linked_recuts_attested_accepted": replacementAccepted,
		"authority": "INDEPENDENTLY_SIGNED_HUMAN_RECONCILIATION_NOT_MACHINE_VERIFIED", "finished_garments": nil, "sewing_output_credit": false, "inventory_credit": false, "shipment_release": false})
}

// Provisioner-controlled and plant-scoped defect catalog, keyset paginated.
func (p *f32API) defects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	if !p.ready(w) {
		return
	}
	q := r.URL.Query()
	for key := range q {
		if key != "cursor" && key != "limit" {
			docJSON(w, 422, map[string]string{"error": "ERR_F32_DEFECT_QUERY"})
			return
		}
	}
	if len(q["cursor"]) > 1 || len(q["limit"]) > 1 || len(q) > 2 || (q.Get("cursor") != "" && (!sewFields(q.Get("cursor")) || len(q.Get("cursor")) > 64)) {
		docJSON(w, 422, map[string]string{"error": "ERR_F32_DEFECT_QUERY"})
		return
	}
	limit, err := genPageLimit(q.Get("limit"))
	if err != nil {
		sewProdReply(w, err)
		return
	}
	a, _, ok := p.gen.actor(w, r, "cutting.read", f32ReadChallenge)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, err := p.gen.sewing.server.docTransaction(ctx, a.TenantID)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	defer tx.Rollback()
	if err = workforceChapterRequire(ctx, tx, a, "cutting.read"); err != nil {
		sewProdReply(w, err)
		return
	}
	rows, err := tx.QueryContext(ctx, `SELECT code,description FROM cut_defect_codes_v19 WHERE tenant_id=$1 AND plant_id=$2 AND enabled=true AND code>$3 ORDER BY code LIMIT $4`, a.TenantID, a.PlantID, q.Get("cursor"), limit+1)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	type entry struct {
		Code        string `json:"code"`
		Description string `json:"description"`
	}
	items := make([]entry, 0)
	for rows.Next() {
		var item entry
		if e := rows.Scan(&item.Code, &item.Description); e != nil {
			rows.Close()
			sewProdReply(w, e)
			return
		}
		items = append(items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		sewProdReply(w, err)
		return
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	next := ""
	if more {
		next = items[len(items)-1].Code
	}
	if err = tx.Commit(); err != nil {
		sewProdReply(w, err)
		return
	}
	docJSON(w, 200, map[string]any{"plant_id": a.PlantID, "defects": items, "has_more": more, "next_cursor": next, "catalog_authority": "PROVISIONER_CONTROLLED"})
}
func registerF32Routes(mux *http.ServeMux, s *EdgeServer) {
	p := newF32API(s)
	mux.HandleFunc("/api/v1/cutting/production/reconciliation/challenge", p.challenge)
	mux.HandleFunc("/api/v1/cutting/production/reconciliation/events", p.write)
	mux.HandleFunc("/api/v1/cutting/production/reconciliation/balance", p.balance)
	mux.HandleFunc("/api/v1/cutting/production/reconciliation/defects", p.defects)
}
