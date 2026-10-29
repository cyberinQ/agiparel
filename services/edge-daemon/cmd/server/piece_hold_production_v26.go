// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// F3.16A: one permanent garment/ply can be held while unrelated WIP proceeds.
// Signed human claims are not physical defect certification or sewing credit.
package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/attestation"
)

const pieceHoldAuthority = "SIGNED_PIECE_HOLD_NO_SEWING_OR_CUSTODY_CREDIT"

type pieceHoldCommand struct {
	CommandID    string `json:"command_id"`
	PlantID      string `json:"plant_id"`
	GarmentID    string `json:"garment_id"`
	StationID    string `json:"station_id"`
	Action       string `json:"action"`
	Reason       string `json:"reason"`
	HoldID       string `json:"hold_id"`
	ScanID       string `json:"scan_id"`
	ReviewAction string `json:"review_action"`
	Note         string `json:"note"`
}
type pieceHoldRequest struct {
	Command pieceHoldCommand `json:"command"`
}
type pieceHoldAPI struct {
	wip     *wipAPI
	access  *chapterAccessAPI
	enabled bool
}

func newPieceHoldAPI(s *EdgeServer) *pieceHoldAPI {
	p := &pieceHoldAPI{}
	if s == nil || s.db == nil || os.Getenv("AGIPAREL_PIECE_HOLDS_ENABLED") != "true" ||
		(os.Getenv("ENV") != "production" && os.Getenv("ENV") != "staging") {
		return p
	}
	p.wip = newWIPAPI(s)
	if !p.wip.enabled {
		return p
	}
	p.access = newChapterAccessAPI(s)
	p.enabled = p.access.enabled
	return p
}
func (p *pieceHoldAPI) ready(w http.ResponseWriter) bool {
	if p == nil || !p.enabled {
		docJSON(w, 503, map[string]string{"error": "ERR_F316_DISABLED"})
		return false
	}
	return true
}
func pieceHoldValid(c pieceHoldCommand) bool {
	if !sewCommandID.MatchString(c.CommandID) || !sewFields(c.PlantID, c.StationID) || !sewUUID(c.GarmentID) ||
		len(strings.TrimSpace(c.Note)) < 12 || len(c.Note) > 500 || strings.ContainsRune(c.Note, 0) {
		return false
	}
	switch c.Action {
	case "HOLD":
		if c.HoldID != "" || c.ScanID != "" || c.ReviewAction != "" {
			return false
		}
		switch c.Reason {
		case "SEWING_DEFECT", "MISSING_COMPONENT", "OIL_STAIN", "NEEDLE_DAMAGE", "REWORK_REQUIRED":
			return true
		}
	case "REVIEW":
		return c.Reason == "" && sewUUID(c.HoldID) && sewUUID(c.ScanID) && c.ReviewAction == "SUPERVISOR_REVIEWED_FOR_WIP"
	}
	return false
}
func pieceHoldPermission(action string) (string, string) {
	if action == "HOLD" {
		return "sewing.piece.hold", "sewing_operator"
	}
	return "sewing.piece.review", "sewing_supervisor"
}
func pieceHoldChallenge(c pieceHoldCommand) string { return "WIP_PIECE_V26:" + sewDigest(c) }
func (p *pieceHoldAPI) challenge(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var req pieceHoldRequest
	if !sewProdStrictJSON(w, r, &req) {
		return
	}
	if !pieceHoldValid(req.Command) {
		docJSON(w, 422, map[string]string{"error": "ERR_F316_FIELDS"})
		return
	}
	permission, role := pieceHoldPermission(req.Command.Action)
	docJSON(w, 200, map[string]any{"challenge": pieceHoldChallenge(req.Command), "required_action": permission,
		"required_role": role, "required_auth": "webauthn_or_fido2", "authority": pieceHoldAuthority,
		"sewing_credit": false, "custody_changed": false})
}
func (p *pieceHoldAPI) event(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var req pieceHoldRequest
	if !sewProdStrictJSON(w, r, &req) {
		return
	}
	c := req.Command
	if !pieceHoldValid(c) {
		docJSON(w, 422, map[string]string{"error": "ERR_F316_FIELDS"})
		return
	}
	token, err := bearer(r)
	if err != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_F316_SIGNED_BEARER_REQUIRED"})
		return
	}
	permission, role := pieceHoldPermission(c.Action)
	a, err := p.wip.gen.sewing.verifier.Verify(token, permission, pieceHoldChallenge(c), role, true)
	if err != nil || a.PlantID != c.PlantID || !validChapterActor(a) || a.JWTID == "" || len(a.JWTID) > 255 {
		docJSON(w, 401, map[string]string{"error": "ERR_F316_SIGNED_ASSERTION"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	id, replay, err := p.persist(ctx, c, a, token, permission)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	status := 201
	if replay {
		status = 200
	}
	docJSON(w, status, map[string]any{"id": id, "replayed": replay, "action": c.Action,
		"authority": pieceHoldAuthority, "custody_changed": false, "sewing_credit": false,
		"quality_release": false, "physical_repair_certified": false})
}
func (p *pieceHoldAPI) persist(ctx context.Context, c pieceHoldCommand, a attestation.Claims, token, permission string) (string, bool, error) {
	tx, err := p.wip.gen.sewing.server.docTransaction(ctx, a.TenantID)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback()
	if err = workforceChapterRequire(ctx, tx, a, permission); err != nil {
		return "", false, err
	}
	if err = sewTxLock(ctx, tx, a.TenantID, "F316:PIECE:"+c.GarmentID); err != nil {
		return "", false, err
	}
	digest := sewDigest(c)
	var id, oldHash, oldActor string
	if c.Action == "HOLD" {
		err = tx.QueryRowContext(ctx, `SELECT id::text,command_sha256,reporter_subject FROM wip_piece_holds_v26
   WHERE tenant_id=$1 AND command_id=$2`, a.TenantID, c.CommandID).Scan(&id, &oldHash, &oldActor)
	} else {
		err = tx.QueryRowContext(ctx, `SELECT id::text,command_sha256,supervisor_subject FROM wip_piece_reviews_v26
   WHERE tenant_id=$1 AND command_id=$2`, a.TenantID, c.CommandID).Scan(&id, &oldHash, &oldActor)
	}
	if err == nil {
		if oldHash != digest || oldActor != a.Subject {
			return "", false, sewFail(409, "ERR_F316_COMMAND_REPLAY_CONFLICT")
		}
		return id, true, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, err
	}
	if c.Action == "HOLD" {
		err = tx.QueryRowContext(ctx, `INSERT INTO wip_piece_holds_v26
   (tenant_id,plant_id,garment_id,station_id,reason,note,command_id,command_sha256,reporter_subject,reporter_proof_id,assertion_sha256)
   VALUES($1,$2,$3::uuid,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id::text`,
			a.TenantID, c.PlantID, c.GarmentID, c.StationID, c.Reason, c.Note, c.CommandID, digest, a.Subject, a.JWTID, sewProdHashString(token)).Scan(&id)
	} else {
		err = tx.QueryRowContext(ctx, `INSERT INTO wip_piece_reviews_v26
   (tenant_id,plant_id,garment_id,station_id,hold_id,scan_id,review_action,note,command_id,command_sha256,supervisor_subject,supervisor_proof_id,assertion_sha256)
   VALUES($1,$2,$3::uuid,$4,$5::uuid,$6::uuid,$7,$8,$9,$10,$11,$12,$13) RETURNING id::text`,
			a.TenantID, c.PlantID, c.GarmentID, c.StationID, c.HoldID, c.ScanID, c.ReviewAction, c.Note, c.CommandID, digest, a.Subject, a.JWTID, sewProdHashString(token)).Scan(&id)
	}
	if err != nil {
		return "", false, sewFail(409, "ERR_F316_HOLD_OR_PROOF_CONFLICT")
	}
	return id, false, tx.Commit()
}

type pieceHoldQuery struct {
	TenantID, PlantID, LayID string
	Position, Group          int
}

func parsePieceHoldQuery(q url.Values) (pieceHoldQuery, error) {
	var out pieceHoldQuery
	for k := range q {
		if k != "tenant_id" && k != "plant_id" && k != "lay_draft_id" && k != "marker_position" && k != "ply_group" {
			return out, errors.New("unknown query")
		}
	}
	for _, k := range []string{"tenant_id", "plant_id", "lay_draft_id", "marker_position", "ply_group"} {
		if len(q[k]) != 1 {
			return out, errors.New("missing or duplicate query")
		}
	}
	out.TenantID = q.Get("tenant_id")
	out.PlantID = q.Get("plant_id")
	out.LayID = q.Get("lay_draft_id")
	if !sewFields(out.TenantID, out.PlantID) || !sewUUID(out.LayID) {
		return out, errors.New("invalid scope")
	}
	var err error
	out.Position, err = strconv.Atoi(q.Get("marker_position"))
	if err != nil || out.Position < 1 || out.Position > 80 {
		return out, errors.New("invalid position")
	}
	out.Group, err = strconv.Atoi(q.Get("ply_group"))
	if err != nil || out.Group < 1 || out.Group > 210 {
		return out, errors.New("invalid group")
	}
	return out, nil
}
func (p *pieceHoldAPI) groups(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	if !p.ready(w) {
		return
	}
	q, err := parsePieceHoldQuery(r.URL.Query())
	if err != nil {
		docJSON(w, 422, map[string]string{"error": "ERR_F316_QUERY"})
		return
	}
	actor, err := (&commissioningAPI{access: p.access}).identify(r, q.TenantID, q.PlantID)
	if err != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_F316_AUTH"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, err := p.wip.gen.sewing.server.docTransaction(ctx, q.TenantID)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	defer tx.Rollback()
	allowed := false
	for _, grant := range []string{"sewing.read", "sewing.piece.hold", "sewing.piece.review"} {
		ok, e := workforceChapterPermit(ctx, tx, actor, grant)
		if e != nil {
			sewProdReply(w, e)
			return
		}
		allowed = allowed || ok
	}
	if !allowed {
		docJSON(w, 403, map[string]string{"error": "ERR_F316_GRANT_REQUIRED"})
		return
	}
	var assigned bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sew_prod_worker_stations_v10 w WHERE
  w.tenant_id=$1 AND w.plant_id=$2 AND w.worker_subject=$3 AND w.enabled)`, q.TenantID, q.PlantID, actor.Subject).Scan(&assigned)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	if !assigned {
		docJSON(w, 403, map[string]string{"error": "ERR_F316_WORKER_NOT_ASSIGNED"})
		return
	}
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT ply_start,ply_end FROM cut_bundle_drafts_v8
  WHERE tenant_id=$1 AND lay_draft_id=$2::uuid AND marker_position=$3 AND ply_group=$4 ORDER BY ply_start,ply_end`,
		q.TenantID, q.LayID, q.Position, q.Group)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	type rng struct{ start, end int }
	ranges := make([]rng, 0, 2)
	for rows.Next() {
		var x rng
		if err = rows.Scan(&x.start, &x.end); err != nil {
			break
		}
		ranges = append(ranges, x)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		sewProdReply(w, err)
		return
	}
	if len(ranges) != 1 {
		docJSON(w, 409, map[string]string{"error": "ERR_F316_MISSING_OR_AMBIGUOUS_PLY_GROUP"})
		return
	}
	rg := ranges[0]
	items := make([]map[string]any, 0)
	rs, err := tx.QueryContext(ctx, `SELECT g.id::text,g.ply_number,h.id::text,h.reason
   FROM gen_garments_v18 g
   LEFT JOIN LATERAL (SELECT hold.id,hold.reason FROM wip_piece_holds_v26 hold
     WHERE hold.tenant_id=g.tenant_id AND hold.garment_id=g.id
     AND NOT EXISTS(SELECT 1 FROM wip_piece_reviews_v26 review
       WHERE review.tenant_id=hold.tenant_id AND review.hold_id=hold.id)
     LIMIT 1) h ON TRUE
   WHERE g.tenant_id=$1 AND g.plant_id=$2 AND g.lay_draft_id=$3::uuid
    AND g.marker_position=$4 AND g.ply_number BETWEEN $5 AND $6
   ORDER BY g.ply_number LIMIT 210`, q.TenantID, q.PlantID, q.LayID, q.Position, rg.start, rg.end)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	holds := 0
	for rs.Next() {
		var id string
		var ply int
		var hid, reason sql.NullString
		if err = rs.Scan(&id, &ply, &hid, &reason); err != nil {
			break
		}
		entry := map[string]any{"garment_id": id, "ply_number": ply, "held": hid.Valid}
		if hid.Valid {
			entry["hold_id"] = hid.String
			entry["reason"] = reason.String
			holds++
		}
		items = append(items, entry)
	}
	if err == nil {
		err = rs.Err()
	}
	rs.Close()
	if err != nil {
		sewProdReply(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		sewProdReply(w, err)
		return
	}
	docJSON(w, 200, map[string]any{"tenant_id": q.TenantID, "plant_id": q.PlantID, "lay_draft_id": q.LayID,
		"marker_position": q.Position, "ply_group": q.Group, "nominal_ply_count": rg.end - rg.start + 1,
		"reserved_garment_identities": len(items), "held_garment_identities": holds,
		"unheld_reserved_identities": len(items) - holds, "garments": items,
		"authority":               "IDENTITY_RESERVATIONS_AND_SIGNED_HOLDS_NOT_PHYSICAL_WIP_COUNT",
		"physical_units_verified": false, "sewing_credit": false, "movement_inferred": false})
}
func registerPieceHoldRoutes(mux *http.ServeMux, s *EdgeServer) {
	p := newPieceHoldAPI(s)
	mux.HandleFunc("/api/v1/wip/pieces/challenge", p.challenge)
	mux.HandleFunc("/api/v1/wip/pieces/events", p.event)
	mux.HandleFunc("/api/v1/wip/pieces/groups", p.groups)
}
