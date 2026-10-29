// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// F3.16B: individual worker-attested sewing events from already signed cut-part scans.
// Neither a physical-output certificate nor a QC/payroll/dispatch credit.
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

const sewPieceAuthority = "WORKER_ATTESTED_UNIT_OPERATION_PENDING_QUALITY_NOT_PAYROLL"

type sewPieceCommand struct {
	CommandID string   `json:"command_id"`
	PlantID   string   `json:"plant_id"`
	StationID string   `json:"station_id"`
	GarmentID string   `json:"garment_id"`
	RouteID   string   `json:"route_id"`
	ScanIDs   []string `json:"scan_ids"`
}
type sewPieceRequest struct {
	Command sewPieceCommand `json:"command"`
}
type sewPieceAPI struct {
	sewing  *sewProdAPI
	holds   *pieceHoldAPI
	enabled bool
}

func newSewPieceAPI(s *EdgeServer) *sewPieceAPI {
	p := &sewPieceAPI{}
	if s == nil || s.db == nil || os.Getenv("AGIPAREL_SEW_PIECE_COMPLETIONS_ENABLED") != "true" ||
		(os.Getenv("ENV") != "production" && os.Getenv("ENV") != "staging") {
		return p
	}
	p.sewing = newSewProdAPI(s)
	p.holds = newPieceHoldAPI(s)
	p.enabled = p.sewing.enabled && p.holds.enabled
	return p
}
func (p *sewPieceAPI) ready(w http.ResponseWriter) bool {
	if p == nil || !p.enabled {
		docJSON(w, 503, map[string]string{"error": "ERR_F316B_DISABLED"})
		return false
	}
	return true
}
func sewPieceValid(c sewPieceCommand) bool {
	if !sewCommandID.MatchString(c.CommandID) || !sewFields(c.PlantID, c.StationID) ||
		!sewUUID(c.GarmentID) || !sewUUID(c.RouteID) ||
		len(c.ScanIDs) < 1 || len(c.ScanIDs) > 32 {
		return false
	}
	seen := make(map[string]bool, len(c.ScanIDs))
	for _, id := range c.ScanIDs {
		if !sewUUID(id) {
			return false
		}
		canon := strings.ToLower(id)
		if seen[canon] {
			return false
		}
		seen[canon] = true
	}
	return true
}
func sewPieceChallenge(c sewPieceCommand) string { return "SEW_PIECE_COMPLETE_V27:" + sewDigest(c) }

func (p *sewPieceAPI) challenge(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var req sewPieceRequest
	if !sewProdStrictJSON(w, r, &req) {
		return
	}
	if !sewPieceValid(req.Command) {
		docJSON(w, 422, map[string]string{"error": "ERR_F316B_FIELDS"})
		return
	}
	docJSON(w, 200, map[string]any{
		"challenge":       sewPieceChallenge(req.Command),
		"required_action": "sewing.piece.complete", "required_role": "sewing_operator",
		"required_auth": "webauthn_or_fido2", "evidence_state": sewPieceAuthority,
		"payroll_credit": false, "quality_release": false, "custody_changed": false,
	})
}
func (p *sewPieceAPI) complete(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !p.ready(w) {
		return
	}
	var req sewPieceRequest
	if !sewProdStrictJSON(w, r, &req) {
		return
	}
	c := req.Command
	if !sewPieceValid(c) {
		docJSON(w, 422, map[string]string{"error": "ERR_F316B_FIELDS"})
		return
	}
	token, err := bearer(r)
	if err != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_F316B_SIGNED_BEARER_REQUIRED"})
		return
	}
	actor, err := p.sewing.verifier.Verify(token, "sewing.piece.complete", sewPieceChallenge(c), "sewing_operator", true)
	if err != nil || actor.PlantID != c.PlantID || !validChapterActor(actor) || actor.JWTID == "" || len(actor.JWTID) > 255 {
		docJSON(w, 401, map[string]string{"error": "ERR_F316B_SIGNED_ASSERTION"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	id, replay, err := p.persist(ctx, c, actor, token)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	code := 201
	if replay {
		code = 200
	}
	docJSON(w, code, map[string]any{
		"id": id, "replayed": replay, "garment_id": c.GarmentID, "route_id": c.RouteID,
		"evidence_state": sewPieceAuthority, "worker_attested_operation": true,
		"payroll_credit": false, "quality_release": false, "custody_changed": false,
		"physical_output_certified": false,
	})
}
func (p *sewPieceAPI) persist(ctx context.Context, c sewPieceCommand, actor attestation.Claims, token string) (string, bool, error) {
	tx, err := p.sewing.server.docTransaction(ctx, actor.TenantID)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback()
	if err = workforceChapterRequire(ctx, tx, actor, "sewing.piece.complete"); err != nil {
		return "", false, err
	}
	// Repeat-requests are serialized by command and garment+route. The SQL
	// trigger serializes against whole-group completion and one-piece HOLD.
	if err = sewTxLock(ctx, tx, actor.TenantID, "PIECE_COMMAND:"+c.CommandID); err != nil {
		return "", false, err
	}
	if err = sewTxLock(ctx, tx, actor.TenantID, "PIECE_ROUTE:"+c.RouteID+":"+c.GarmentID); err != nil {
		return "", false, err
	}
	hash := sewDigest(c)
	var id, existingHash, existingWorker string
	err = tx.QueryRowContext(ctx, `SELECT id::text,command_sha256,worker_subject
        FROM sew_piece_completions_v27 WHERE tenant_id=$1 AND command_id=$2`, actor.TenantID, c.CommandID).Scan(&id, &existingHash, &existingWorker)
	if err == nil {
		if existingHash != hash || existingWorker != actor.Subject {
			return "", false, sewFail(409, "ERR_F316B_COMMAND_CONFLICT")
		}
		return id, true, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, err
	}
	var lay, release, station, plant string
	var position int
	err = tx.QueryRowContext(ctx, `SELECT g.lay_draft_id::text,g.release_id::text,g.marker_position,g.plant_id,r.station_id
        FROM gen_garments_v18 g JOIN sew_prod_routes_v10 r
        ON r.tenant_id=g.tenant_id AND r.lay_draft_id=g.lay_draft_id AND r.plant_id=g.plant_id
        WHERE g.tenant_id=$1 AND g.id=$2::uuid AND r.id=$3::uuid AND r.enabled`,
		actor.TenantID, c.GarmentID, c.RouteID).Scan(&lay, &release, &position, &plant, &station)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, sewFail(409, "ERR_F316B_GARMENT_ROUTE_REQUIRED")
	}
	if err != nil {
		return "", false, err
	}
	if plant != c.PlantID || station != c.StationID {
		return "", false, sewFail(403, "ERR_F316B_PLANT_STATION_SCOPE")
	}
	// The trigger independently resolves the exact garment ply group and
	// validates every cut-part scan, predecessor, hold, binding and custody.
	var group int
	err = tx.QueryRowContext(ctx, `SELECT min(b.ply_group) FROM gen_garments_v18 g
      JOIN cut_bundle_drafts_v8 b ON b.tenant_id=g.tenant_id AND b.lay_draft_id=g.lay_draft_id
       AND b.marker_position=g.marker_position AND g.ply_number BETWEEN b.ply_start AND b.ply_end
      WHERE g.tenant_id=$1 AND g.id=$2::uuid`, actor.TenantID, c.GarmentID).Scan(&group)
	if err != nil {
		return "", false, sewFail(409, "ERR_F316B_GROUP_NOT_FOUND")
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO sew_piece_completions_v27
      (tenant_id,command_id,command_sha256,garment_id,lay_draft_id,release_id,route_id,plant_id,station_id,
       marker_position,ply_group,scan_ids,worker_subject,worker_proof_id,assertion_sha256)
      VALUES($1,$2,$3,$4::uuid,$5::uuid,$6::uuid,$7::uuid,$8,$9,$10,$11,$12::uuid[],$13,$14,$15)
      RETURNING id::text`, actor.TenantID, c.CommandID, hash, c.GarmentID, lay, release, c.RouteID, c.PlantID, c.StationID,
		position, group, pgUUIDArray(c.ScanIDs), actor.Subject, actor.JWTID, sewProdHashString(token)).Scan(&id)
	if err != nil {
		return "", false, sewPieceAdmissionError(err)
	}
	return id, false, tx.Commit()
}

func registerSewPieceRoutes(mux *http.ServeMux, s *EdgeServer) {
	p := newSewPieceAPI(s)
	mux.HandleFunc("/api/v1/sewing/pieces/completions/challenge", p.challenge)
	mux.HandleFunc("/api/v1/sewing/pieces/completions", p.complete)
}

// Use SQLSTATE instead of treating infrastructure failures as stock/evidence conflicts.
// Detailed PostgreSQL errors are never exposed to the caller.
func sewPieceAdmissionError(err error) error {
	var state interface{ SQLState() string }
	if errors.As(err, &state) {
		switch state.SQLState() {
		case "23505", "23503", "23514", "P0001":
			return sewFail(409, "ERR_F316B_EVIDENCE_OR_CONCURRENT_CONFLICT")
		}
	}
	return err
}
