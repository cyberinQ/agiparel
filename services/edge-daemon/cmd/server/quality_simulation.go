// Author: Sunil Jain, CyberInQ, 2026-10-27.
// Batch 10A/B - E10. Per-garment QC holds, independent repair inspection,
// finishing-route execution, and simulated buyer identity with true sewing
// source lineage. Never a physical RF observation or shipping authorization.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/internal/web"
)

const qcSimEvidence = "UI_SIMULATED_NO_RF_OBSERVATION"
const qcSimAuthority = "REHEARSAL_ONLY_NOT_QA_RELEASE"

var qcBuyerCode = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{3,95}$`)

var qcProfiles = map[string][]string{
	"WOVEN_STANDARD": {"TRIM", "PRESS"},
	"WASH_FINISH":    {"WASH", "DRY", "PRESS", "TRIM"},
	"HANGER_FINISH":  {"TRIM", "PRESS", "HANGER"},
	"KNIT_FOLD":      {"TRIM", "PRESS", "FOLD"},
}

type qcSimReader struct {
	Version     string   `json:"version"`
	Source      string   `json:"source"`
	ReaderID    string   `json:"reader_id"`
	AntennaPort int      `json:"antenna_port"`
	ClickedAt   string   `json:"clicked_at"`
	EPCHex      *string  `json:"epc_hex"`
	RSSIDBm     *float64 `json:"rssi_dbm"`
}

type qcSimEventRequest struct {
	EventID    string      `json:"event_id"`
	LayID      string      `json:"lay_id"`
	PlantID    string      `json:"plant_id"`
	Serial     string      `json:"serial"`
	Action     string      `json:"action"`
	Actor      string      `json:"actor"`
	StationID  string      `json:"station_id"`
	DefectCode string      `json:"defect_code"`
	Step       string      `json:"step"`
	BuyerCode  string      `json:"buyer_code"`
	StyleCode  string      `json:"style_code"`
	SizeCode   string      `json:"size_code"`
	Reader     qcSimReader `json:"reader"`
}

type qcSimRouteRequest struct {
	LayID   string `json:"lay_id"`
	PlantID string `json:"plant_id"`
	Profile string `json:"profile"`
}

func qcValidReader(v qcSimReader) bool {
	if v.Version != "QC_READER_V1" || v.Source != "UI_SIMULATED" || !sewFields(v.ReaderID) || v.AntennaPort < 1 || v.AntennaPort > 64 || v.EPCHex != nil || v.RSSIDBm != nil {
		return false
	}
	t, e := time.Parse(time.RFC3339Nano, v.ClickedAt)
	return e == nil && t.Before(time.Now().Add(5*time.Minute)) && t.After(time.Now().Add(-24*time.Hour))
}
func qcValidRequest(v qcSimEventRequest) error {
	if !sewCommandID.MatchString(v.EventID) || !sewUUID(v.LayID) || !sewFields(v.PlantID, v.Actor, v.StationID) || !strings.HasPrefix(v.Serial, "SIM-G-") || len(v.Serial) > 128 || !qcValidReader(v.Reader) {
		return sewFail(422, "ERR_QC_SIM_FIELDS_OR_READER")
	}
	switch v.Action {
	case "ENROLL", "INSPECT_PASS", "INSPECT_FAIL", "REPAIR_COMPLETE", "REINSPECT_PASS", "REINSPECT_FAIL", "FINISH_STEP", "BIND_BUYER":
	default:
		return sewFail(422, "ERR_QC_SIM_ACTION")
	}
	if v.Action == "INSPECT_FAIL" || v.Action == "REINSPECT_FAIL" {
		if !sewFields(v.DefectCode) {
			return sewFail(422, "ERR_QC_SIM_DEFECT_REQUIRED")
		}
	} else if v.DefectCode != "" {
		return sewFail(422, "ERR_QC_SIM_UNEXPECTED_DEFECT")
	}
	if v.Action == "FINISH_STEP" {
		if !sewFields(v.Step) {
			return sewFail(422, "ERR_QC_SIM_FINISH_STEP")
		}
	} else if v.Step != "" {
		return sewFail(422, "ERR_QC_SIM_UNEXPECTED_STEP")
	}
	if v.Action == "BIND_BUYER" {
		if !qcBuyerCode.MatchString(v.BuyerCode) || !sewFields(v.StyleCode, v.SizeCode) {
			return sewFail(422, "ERR_QC_SIM_BUYER_IDENTITY")
		}
	} else if v.BuyerCode != "" || v.StyleCode != "" || v.SizeCode != "" {
		return sewFail(422, "ERR_QC_SIM_UNEXPECTED_BUYER")
	}
	return nil
}
func qcCheckTransition(state, action, actor, failInspector, repairActor string, finished, routeLen int) (string, error) {
	switch action {
	case "INSPECT_PASS":
		if state == "PENDING" {
			return "ACCEPTED", nil
		}
	case "INSPECT_FAIL":
		if state == "PENDING" {
			return "HOLD", nil
		}
	case "REPAIR_COMPLETE":
		if state == "HOLD" && actor != failInspector {
			return "REINSPECT", nil
		}
	case "REINSPECT_PASS", "REINSPECT_FAIL":
		if state == "REINSPECT" && actor != repairActor && actor != failInspector {
			if action == "REINSPECT_PASS" {
				return "ACCEPTED", nil
			}
			return "HOLD", nil
		}
	case "FINISH_STEP":
		if state == "ACCEPTED" && routeLen > 0 && finished < routeLen {
			return "ACCEPTED", nil
		}
	case "BIND_BUYER":
		if state == "ACCEPTED" && routeLen > 0 && finished == routeLen {
			return "ACCEPTED", nil
		}
	}
	return "", sewFail(409, "ERR_QC_SIM_STATE_OR_INDEPENDENT_CHECK")
}
func (s *EdgeServer) qcTenant(w http.ResponseWriter) (string, bool) {
	if os.Getenv("ENV") != "development" {
		docJSON(w, 403, map[string]string{"error": "ERR_QC_SIM_DEVELOPMENT_ONLY"})
		return "", false
	}
	return s.sewTenant(w)
}
func qcReplay(ctx context.Context, tx *sql.Tx, tenant, eventID, hash string) (bool, error) {
	var previous string
	e := tx.QueryRowContext(ctx, `SELECT payload_sha256 FROM qc_sim_events_v10 WHERE tenant_id=$1 AND event_id=$2`, tenant, eventID).Scan(&previous)
	if e == sql.ErrNoRows {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	if previous != hash {
		return false, sewFail(409, "ERR_QC_SIM_EVENT_ID_COLLISION")
	}
	return true, nil
}
func (s *EdgeServer) handleQCSimCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "GET required", 405)
		return
	}
	tenant, ok := s.qcTenant(w)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, e := s.docTransaction(ctx, tenant)
	if e != nil {
		sewReplyError(w, e)
		return
	}
	defer tx.Rollback()
	lays := []map[string]any{}
	rows, e := tx.QueryContext(ctx, `SELECT d.id::text,d.intake_id,d.cut_no,
      (SELECT count(DISTINCT serial) FROM sew_sim_credits_v9 c CROSS JOIN LATERAL unnest(c.serials) serial WHERE c.tenant_id=d.tenant_id AND c.lay_draft_id=d.id)::int
      FROM cut_lay_drafts_v8 d WHERE d.tenant_id=$1 ORDER BY d.created_at DESC LIMIT 100`, tenant)
	if e != nil {
		sewReplyError(w, e)
		return
	}
	for rows.Next() {
		var lay, intake string
		var cut, n int
		if e = rows.Scan(&lay, &intake, &cut, &n); e != nil {
			break
		}
		lays = append(lays, map[string]any{"lay_id": lay, "intake_id": intake, "cut": cut, "simulated_sewing_serials": n})
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		sewReplyError(w, e)
		return
	}
	lay := r.URL.Query().Get("lay_id")
	plant := r.URL.Query().Get("plant_id")
	units := []map[string]any{}
	candidates := []map[string]any{}
	var route any = nil
	if lay != "" || plant != "" {
		if !sewUUID(lay) || !sewFields(plant) {
			docJSON(w, 422, map[string]string{"error": "ERR_QC_SIM_LAY_PLANT"})
			return
		}
		rows, e = tx.QueryContext(ctx, `SELECT DISTINCT s.serial,p.color_code,p.size_code,c.marker_position,
          EXISTS(SELECT 1 FROM qc_sim_units_v10 q WHERE q.tenant_id=c.tenant_id AND q.serial=s.serial)
          FROM sew_sim_credits_v9 c JOIN sew_sim_cycles_v9 cy ON cy.tenant_id=c.tenant_id AND cy.id=c.cycle_id
          JOIN cut_lay_positions_v8 p ON p.tenant_id=c.tenant_id AND p.lay_draft_id=c.lay_draft_id AND p.marker_position=c.marker_position
          CROSS JOIN LATERAL unnest(c.serials) AS s(serial)
          WHERE c.tenant_id=$1 AND c.lay_draft_id=$2::uuid AND cy.plant_id=$3 ORDER BY s.serial LIMIT 1500`, tenant, lay, plant)
		if e != nil {
			sewReplyError(w, e)
			return
		}
		for rows.Next() {
			var serial, color, size string
			var pos int
			var registered bool
			if e = rows.Scan(&serial, &color, &size, &pos, &registered); e != nil {
				break
			}
			candidates = append(candidates, map[string]any{"serial": serial, "color": color, "size": size, "position": pos, "registered": registered})
		}
		if e == nil {
			e = rows.Err()
		}
		rows.Close()
		if e != nil {
			sewReplyError(w, e)
			return
		}
		rows, e = tx.QueryContext(ctx, `SELECT q.serial,q.color_code,q.size_code,q.qc_state,q.last_defect,q.fail_inspector,q.repair_actor,q.finishing_count,q.revision,
          COALESCE(b.buyer_code,'') FROM qc_sim_units_v10 q LEFT JOIN qc_sim_buyer_bindings_v10 b ON b.tenant_id=q.tenant_id AND b.serial=q.serial
          WHERE q.tenant_id=$1 AND q.lay_draft_id=$2::uuid AND q.plant_id=$3 ORDER BY q.serial LIMIT 1500`, tenant, lay, plant)
		if e != nil {
			sewReplyError(w, e)
			return
		}
		for rows.Next() {
			var serial, color, size, state, defect, failed, repair, buyer string
			var done, revision int
			if e = rows.Scan(&serial, &color, &size, &state, &defect, &failed, &repair, &done, &revision, &buyer); e != nil {
				break
			}
			units = append(units, map[string]any{"serial": serial, "color": color, "size": size, "qc_state": state, "defect": defect, "fail_inspector": failed, "repair_actor": repair, "finished_steps": done, "revision": revision, "buyer_code": buyer})
		}
		if e == nil {
			e = rows.Err()
		}
		rows.Close()
		if e != nil {
			sewReplyError(w, e)
			return
		}
		var profile string
		var steps []byte
		e = tx.QueryRowContext(ctx, `SELECT profile,to_json(steps) FROM qc_sim_routes_v10 WHERE tenant_id=$1 AND lay_draft_id=$2::uuid AND plant_id=$3`, tenant, lay, plant).Scan(&profile, &steps)
		if e != nil && e != sql.ErrNoRows {
			sewReplyError(w, e)
			return
		}
		if e == nil {
			var list []string
			if e = json.Unmarshal(steps, &list); e != nil {
				sewReplyError(w, e)
				return
			}
			route = map[string]any{"profile": profile, "steps": list, "authority": "UNAPPROVED_SIMULATION_ROUTE"}
		}
	}
	if e = tx.Commit(); e != nil {
		sewReplyError(w, e)
		return
	}
	docJSON(w, 200, map[string]any{"lays": lays, "candidates": candidates, "units": units, "route": route, "evidence": qcSimEvidence, "authority": qcSimAuthority})
}
func (s *EdgeServer) handleQCSimRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "POST required", 405)
		return
	}
	tenant, ok := s.qcTenant(w)
	if !ok {
		return
	}
	var v qcSimRouteRequest
	if !sewStrictJSON(w, r, &v) {
		return
	}
	steps, exists := qcProfiles[v.Profile]
	if !sewUUID(v.LayID) || !sewFields(v.PlantID) || !exists {
		docJSON(w, 422, map[string]string{"error": "ERR_QC_SIM_ROUTE"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, e := s.docTransaction(ctx, tenant)
	if e != nil {
		sewReplyError(w, e)
		return
	}
	defer tx.Rollback()
	if e = sewTxLock(ctx, tx, tenant, "QC_ROUTE:"+v.LayID+":"+v.PlantID); e != nil {
		sewReplyError(w, e)
		return
	}
	var existing string
	e = tx.QueryRowContext(ctx, `SELECT profile FROM qc_sim_routes_v10 WHERE tenant_id=$1 AND lay_draft_id=$2::uuid AND plant_id=$3`, tenant, v.LayID, v.PlantID).Scan(&existing)
	if e != nil && e != sql.ErrNoRows {
		sewReplyError(w, e)
		return
	}
	reused := e == nil
	if reused && existing != v.Profile {
		docJSON(w, 409, map[string]string{"error": "ERR_QC_SIM_ROUTE_IMMUTABLE"})
		return
	}
	if !reused {
		var sewingExists bool
		if e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sew_sim_credits_v9 c JOIN sew_sim_cycles_v9 cy ON cy.tenant_id=c.tenant_id AND cy.id=c.cycle_id WHERE c.tenant_id=$1 AND c.lay_draft_id=$2::uuid AND cy.plant_id=$3)`, tenant, v.LayID, v.PlantID).Scan(&sewingExists); e != nil {
			sewReplyError(w, e)
			return
		}
		if !sewingExists {
			docJSON(w, 409, map[string]string{"error": "ERR_QC_SIM_NO_SEWING_SOURCE"})
			return
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO qc_sim_routes_v10(tenant_id,lay_draft_id,plant_id,profile,steps) VALUES($1,$2::uuid,$3,$4,$5::text[])`, tenant, v.LayID, v.PlantID, v.Profile, pgTextArray(steps))
		if e != nil {
			sewReplyError(w, e)
			return
		}
	}
	if e = tx.Commit(); e != nil {
		sewReplyError(w, e)
		return
	}
	docJSON(w, 200, map[string]any{"profile": v.Profile, "steps": steps, "replayed": reused, "authority": "UNAPPROVED_SIMULATION_ROUTE"})
}

type qcUnit struct {
	serial, lay, plant, color, size, state, failInspector, repairActor, defect string
	pos, done, revision                                                        int
}

func qcGetUnit(ctx context.Context, tx *sql.Tx, tenant, serial string) (qcUnit, error) {
	var u qcUnit
	e := tx.QueryRowContext(ctx, `SELECT serial,lay_draft_id::text,plant_id,color_code,size_code,qc_state,fail_inspector,repair_actor,last_defect,marker_position,finishing_count,revision FROM qc_sim_units_v10 WHERE tenant_id=$1 AND serial=$2 FOR UPDATE`, tenant, serial).Scan(&u.serial, &u.lay, &u.plant, &u.color, &u.size, &u.state, &u.failInspector, &u.repairActor, &u.defect, &u.pos, &u.done, &u.revision)
	return u, e
}
func qcRecordEvent(ctx context.Context, tx *sql.Tx, tenant string, v qcSimEventRequest, hash, previous, next, detail string) error {
	reader, _ := json.Marshal(v.Reader)
	_, e := tx.ExecContext(ctx, `INSERT INTO qc_sim_events_v10(tenant_id,event_id,payload_sha256,serial,action,actor,station_id,detail,reader_payload,prior_state,next_state)
      VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10,$11)`, tenant, v.EventID, hash, v.Serial, v.Action, v.Actor, v.StationID, detail, string(reader), previous, next)
	return e
}
func (s *EdgeServer) handleQCSimEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "POST required", 405)
		return
	}
	tenant, ok := s.qcTenant(w)
	if !ok {
		return
	}
	var v qcSimEventRequest
	if !sewStrictJSON(w, r, &v) {
		return
	}
	if e := qcValidRequest(v); e != nil {
		sewReplyError(w, e)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	tx, e := s.docTransaction(ctx, tenant)
	if e != nil {
		sewReplyError(w, e)
		return
	}
	defer tx.Rollback()
	// Lock by tenant+event ID before checking for replay, and by source serial
	// before changing any unit. Two concurrent events cannot both win.
	if e = sewTxLock(ctx, tx, tenant, "QC_EVENT:"+v.EventID); e != nil {
		sewReplyError(w, e)
		return
	}
	hash := sewDigest(v)
	replay, e := qcReplay(ctx, tx, tenant, v.EventID, hash)
	if e != nil {
		sewReplyError(w, e)
		return
	}
	if replay {
		docJSON(w, 200, map[string]any{"replayed": true, "event_id": v.EventID, "authority": qcSimAuthority})
		return
	}
	if e = sewTxLock(ctx, tx, tenant, "QC_SERIAL:"+v.Serial); e != nil {
		sewReplyError(w, e)
		return
	}
	if v.Action == "ENROLL" {
		// The only permitted source is an existing, completed simulated sewing
		// cycle; no browser-supplied serial, color, size or count is trusted.
		var color, size string
		var pos int
		e = tx.QueryRowContext(ctx, `SELECT p.color_code,p.size_code,c.marker_position
          FROM sew_sim_credits_v9 c JOIN sew_sim_cycles_v9 cy ON cy.tenant_id=c.tenant_id AND cy.id=c.cycle_id
          JOIN cut_lay_positions_v8 p ON p.tenant_id=c.tenant_id AND p.lay_draft_id=c.lay_draft_id AND p.marker_position=c.marker_position
          WHERE c.tenant_id=$1 AND c.lay_draft_id=$2::uuid AND cy.plant_id=$3 AND $4=ANY(c.serials) LIMIT 1`, tenant, v.LayID, v.PlantID, v.Serial).Scan(&color, &size, &pos)
		if e == sql.ErrNoRows {
			sewReplyError(w, sewFail(409, "ERR_QC_SIM_UNKNOWN_SEWING_SERIAL"))
			return
		}
		if e != nil {
			sewReplyError(w, e)
			return
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO qc_sim_units_v10(tenant_id,serial,lay_draft_id,plant_id,marker_position,color_code,size_code) VALUES($1,$2,$3::uuid,$4,$5,$6,$7)`, tenant, v.Serial, v.LayID, v.PlantID, pos, color, size)
		if e != nil {
			sewReplyError(w, sewFail(409, "ERR_QC_SIM_ALREADY_ENROLLED_OR_DB"))
			return
		}
		if e = qcRecordEvent(ctx, tx, tenant, v, hash, "SOURCE_SIM_SEW", "PENDING", ""); e != nil {
			sewReplyError(w, e)
			return
		}
		if e = tx.Commit(); e != nil {
			sewReplyError(w, e)
			return
		}
		docJSON(w, 201, map[string]any{"serial": v.Serial, "qc_state": "PENDING", "authority": qcSimAuthority})
		return
	}
	u, e := qcGetUnit(ctx, tx, tenant, v.Serial)
	if e == sql.ErrNoRows {
		sewReplyError(w, sewFail(404, "ERR_QC_SIM_UNIT_NOT_ENROLLED"))
		return
	}
	if e != nil {
		sewReplyError(w, e)
		return
	}
	if u.lay != v.LayID || u.plant != v.PlantID {
		sewReplyError(w, sewFail(409, "ERR_QC_SIM_LINEAGE_MISMATCH"))
		return
	}
	var alreadyBound bool
	if e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM qc_sim_buyer_bindings_v10 WHERE tenant_id=$1 AND serial=$2)`, tenant, v.Serial).Scan(&alreadyBound); e != nil {
		sewReplyError(w, e)
		return
	}
	if alreadyBound {
		sewReplyError(w, sewFail(409, "ERR_QC_SIM_BUYER_ALREADY_BOUND"))
		return
	}
	var profile string
	var stepsJSON []byte
	e = tx.QueryRowContext(ctx, `SELECT profile,to_json(steps) FROM qc_sim_routes_v10 WHERE tenant_id=$1 AND lay_draft_id=$2::uuid AND plant_id=$3`, tenant, v.LayID, v.PlantID).Scan(&profile, &stepsJSON)
	if e != nil && e != sql.ErrNoRows {
		sewReplyError(w, e)
		return
	}
	steps := []string{}
	if e == nil {
		if e = json.Unmarshal(stepsJSON, &steps); e != nil {
			sewReplyError(w, e)
			return
		}
	}
	next, e := qcCheckTransition(u.state, v.Action, v.Actor, u.failInspector, u.repairActor, u.done, len(steps))
	if e != nil {
		sewReplyError(w, e)
		return
	}
	if v.Action == "FINISH_STEP" && v.Step != steps[u.done] {
		sewReplyError(w, sewFail(409, "ERR_QC_SIM_WRONG_FINISH_ORDER"))
		return
	}
	if v.Action == "BIND_BUYER" {
		var style string
		var skuMatch bool
		e = tx.QueryRowContext(ctx, `SELECT o.style_code,
          EXISTS(SELECT 1 FROM order_intake_skus_v5 sk WHERE sk.tenant_id=o.tenant_id AND sk.intake_id=o.id AND sk.color_code=$3 AND sk.size_code=$4)
          FROM cut_lay_drafts_v8 d JOIN order_intakes_v5 o ON o.tenant_id=d.tenant_id AND o.id=d.intake_id
          WHERE d.tenant_id=$1 AND d.id=$2::uuid`, tenant, v.LayID, u.color, u.size).Scan(&style, &skuMatch)
		if e != nil {
			sewReplyError(w, e)
			return
		}
		if !skuMatch || style != v.StyleCode || u.size != v.SizeCode {
			sewReplyError(w, sewFail(409, "ERR_QC_SIM_BUYER_STYLE_SIZE_MISMATCH"))
			return
		}
		if e = qcRecordEvent(ctx, tx, tenant, v, hash, u.state, u.state, v.BuyerCode); e != nil {
			sewReplyError(w, e)
			return
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO qc_sim_buyer_bindings_v10(tenant_id,serial,buyer_code,style_code,size_code,event_id) VALUES($1,$2,$3,$4,$5,$6)`, tenant, v.Serial, v.BuyerCode, style, u.size, v.EventID)
		if e != nil {
			sewReplyError(w, sewFail(409, "ERR_QC_SIM_DUPLICATE_BUYER_SERIAL"))
			return
		}
	} else {
		defect := u.defect
		failure := u.failInspector
		repair := u.repairActor
		nextCount := u.done
		inspector := ""
		switch v.Action {
		case "INSPECT_PASS":
			inspector = v.Actor
			defect = ""
		case "INSPECT_FAIL", "REINSPECT_FAIL":
			inspector = v.Actor
			failure = v.Actor
			repair = ""
			defect = v.DefectCode
		case "REPAIR_COMPLETE":
			repair = v.Actor
		case "REINSPECT_PASS":
			inspector = v.Actor
			defect = ""
		case "FINISH_STEP":
			nextCount++
		}
		_, e = tx.ExecContext(ctx, `UPDATE qc_sim_units_v10 SET qc_state=$3,finishing_count=$4,last_defect=$5,fail_inspector=$6,repair_actor=$7,
          last_inspector=CASE WHEN $8<>'' THEN $8 ELSE last_inspector END,revision=revision+1,updated_at=now() WHERE tenant_id=$1 AND serial=$2`, tenant, v.Serial, next, nextCount, defect, failure, repair, inspector)
		if e != nil {
			sewReplyError(w, e)
			return
		}
		detail := v.DefectCode
		if v.Action == "FINISH_STEP" {
			detail = v.Step
		}
		if e = qcRecordEvent(ctx, tx, tenant, v, hash, u.state, next, detail); e != nil {
			sewReplyError(w, e)
			return
		}
	}
	if e = tx.Commit(); e != nil {
		sewReplyError(w, e)
		return
	}
	docJSON(w, 200, map[string]any{"serial": v.Serial, "action": v.Action, "previous_state": u.state, "qc_state": next, "finished_steps": u.done + map[bool]int{true: 1, false: 0}[v.Action == "FINISH_STEP"], "replayed": false, "authority": qcSimAuthority, "rfid": qcSimEvidence})
}
func serveQCSimulation(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("ENV") != "development" {
		http.Error(w, "Simulation unavailable outside development", 403)
		return
	}
	data, e := web.GetQCSimulationHTML()
	if e != nil {
		http.Error(w, "QC simulation not installed", 404)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

// Used by acceptance tests to verify the approved *simulation* route catalog.
func qcSteps(profile string) ([]string, error) {
	v, ok := qcProfiles[profile]
	if !ok {
		return nil, fmt.Errorf("unknown finishing profile")
	}
	return append([]string{}, v...), nil
}
