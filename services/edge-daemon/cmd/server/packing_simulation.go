// Author: CyberInQ / Sunil Jain. Batch 11A-B: carton and dispatch rehearsal.
// All events are UI-simulated; no physical RFID, QA release, or legal dispatch.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/internal/web"
)

const packSimAuthority = "REHEARSAL_ONLY_NOT_SHIPMENT_AUTHORIZATION"

type packSimRequest struct {
	EventID  string      `json:"event_id"`
	Action   string      `json:"action"`
	Carton   string      `json:"carton_code"`
	Manifest string      `json:"manifest_code"`
	LayID    string      `json:"lay_id"`
	Plant    string      `json:"plant_id"`
	Serial   string      `json:"serial"`
	Actor    string      `json:"actor"`
	Station  string      `json:"station_id"`
	Reader   qcSimReader `json:"reader"`
}

func packValid(v packSimRequest) error {
	if !sewCommandID.MatchString(v.EventID) || !sewFields(v.Actor, v.Station) || !qcValidReader(qcSimReader{Version: "QC_READER_V1", Source: v.Reader.Source, ReaderID: v.Reader.ReaderID, AntennaPort: v.Reader.AntennaPort, ClickedAt: v.Reader.ClickedAt, EPCHex: v.Reader.EPCHex, RSSIDBm: v.Reader.RSSIDBm}) || v.Reader.Version != "PACK_READER_V1" {
		return sewFail(422, "ERR_PACK_SIM_READER")
	}
	switch v.Action {
	case "OPEN_CARTON":
		if !sewFields(v.Carton, v.Plant) || !sewUUID(v.LayID) || v.Manifest != "" || v.Serial != "" {
			return sewFail(422, "ERR_PACK_SIM_FIELDS")
		}
	case "ADD_GARMENT", "REMOVE_GARMENT":
		if !sewFields(v.Carton) || !strings.HasPrefix(v.Serial, "SIM-G-") || len(v.Serial) > 128 || v.Manifest != "" || v.LayID != "" || v.Plant != "" {
			return sewFail(422, "ERR_PACK_SIM_FIELDS")
		}
	case "SEAL_CARTON":
		if !sewFields(v.Carton) || v.Manifest != "" || v.Serial != "" || v.LayID != "" || v.Plant != "" {
			return sewFail(422, "ERR_PACK_SIM_FIELDS")
		}
	case "OPEN_MANIFEST":
		if !sewFields(v.Manifest, v.Plant) || v.Carton != "" || v.Serial != "" || v.LayID != "" {
			return sewFail(422, "ERR_PACK_SIM_FIELDS")
		}
	case "LOAD_CARTON":
		if !sewFields(v.Manifest, v.Carton) || v.Serial != "" || v.LayID != "" || v.Plant != "" {
			return sewFail(422, "ERR_PACK_SIM_FIELDS")
		}
	case "DISPATCH_REHEARSAL":
		if !sewFields(v.Manifest) || v.Carton != "" || v.Serial != "" || v.LayID != "" || v.Plant != "" {
			return sewFail(422, "ERR_PACK_SIM_FIELDS")
		}
	default:
		return sewFail(422, "ERR_PACK_SIM_ACTION")
	}
	return nil
}
func (s *EdgeServer) packTenant(w http.ResponseWriter) (string, bool) {
	if os.Getenv("ENV") != "development" {
		docJSON(w, 403, map[string]string{"error": "ERR_PACK_SIM_DEVELOPMENT_ONLY"})
		return "", false
	}
	return s.sewTenant(w)
}
func (s *EdgeServer) handlePackSimEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "POST required", 405)
		return
	}
	tenant, ok := s.packTenant(w)
	if !ok {
		return
	}
	var v packSimRequest
	if !sewStrictJSON(w, r, &v) {
		return
	}
	if e := packValid(v); e != nil {
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
	if e = sewTxLock(ctx, tx, tenant, "PACK_EVENT:"+v.EventID); e != nil {
		sewReplyError(w, e)
		return
	}
	hash := sewDigest(v)
	var prior string
	e = tx.QueryRowContext(ctx, `SELECT payload_sha256 FROM pack_sim_events_v11 WHERE tenant_id=$1 AND event_id=$2`, tenant, v.EventID).Scan(&prior)
	if e == nil {
		if prior != hash {
			sewReplyError(w, sewFail(409, "ERR_PACK_SIM_EVENT_COLLISION"))
			return
		}
		docJSON(w, 200, map[string]any{"replayed": true, "event_id": v.EventID, "authority": packSimAuthority})
		return
	}
	if e != sql.ErrNoRows {
		sewReplyError(w, e)
		return
	}
	// Serialize all carton/manifest operations per tenant, avoiding cross-lock ordering.
	if e = sewTxLock(ctx, tx, tenant, "PACK_GLOBAL"); e != nil {
		sewReplyError(w, e)
		return
	}
	var res sql.Result
	switch v.Action {
	case "OPEN_CARTON":
		var exists bool
		e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM qc_sim_routes_v10 WHERE tenant_id=$1 AND lay_draft_id=$2::uuid AND plant_id=$3)`, tenant, v.LayID, v.Plant).Scan(&exists)
		if e == nil && !exists {
			e = sewFail(409, "ERR_PACK_SIM_SOURCE_LAY")
		}
		if e == nil {
			_, e = tx.ExecContext(ctx, `INSERT INTO pack_sim_cartons_v11(tenant_id,carton_code,lay_draft_id,plant_id) VALUES($1,$2,$3::uuid,$4)`, tenant, v.Carton, v.LayID, v.Plant)
		}
	case "ADD_GARMENT":
		// Database trigger independently verifies QC pass, full finishing, buyer binding and same lay/plant.
		var buyer string
		e = tx.QueryRowContext(ctx, `SELECT buyer_code FROM qc_sim_buyer_bindings_v10 WHERE tenant_id=$1 AND serial=$2`, tenant, v.Serial).Scan(&buyer)
		if e == sql.ErrNoRows {
			e = sewFail(409, "ERR_PACK_SIM_NO_BUYER_BIND")
		}
		if e == nil {
			_, e = tx.ExecContext(ctx, `INSERT INTO pack_sim_items_v11(tenant_id,serial,carton_code,buyer_code) VALUES($1,$2,$3,$4)`, tenant, v.Serial, v.Carton, buyer)
		}
	case "REMOVE_GARMENT":
		res, e = tx.ExecContext(ctx, `DELETE FROM pack_sim_items_v11 i USING pack_sim_cartons_v11 c WHERE i.tenant_id=$1 AND i.serial=$2 AND i.carton_code=$3 AND c.tenant_id=i.tenant_id AND c.carton_code=i.carton_code AND c.state='OPEN'`, tenant, v.Serial, v.Carton)
		if e == nil {
			var n int64
			n, e = res.RowsAffected()
			if e == nil && n != 1 {
				e = sewFail(409, "ERR_PACK_SIM_NOT_REMOVABLE")
			}
		}
	case "SEAL_CARTON":
		var n int
		e = tx.QueryRowContext(ctx, `SELECT count(*)::int FROM pack_sim_items_v11 WHERE tenant_id=$1 AND carton_code=$2`, tenant, v.Carton).Scan(&n)
		if e == nil && n == 0 {
			e = sewFail(409, "ERR_PACK_SIM_EMPTY_CARTON")
		}
		if e == nil {
			res, e = tx.ExecContext(ctx, `UPDATE pack_sim_cartons_v11 SET state='SEALED',sealed_at=now() WHERE tenant_id=$1 AND carton_code=$2 AND state='OPEN'`, tenant, v.Carton)
		}
		if e == nil {
			var count int64
			count, e = res.RowsAffected()
			if e == nil && count != 1 {
				e = sewFail(409, "ERR_PACK_SIM_NOT_OPEN")
			}
		}
	case "OPEN_MANIFEST":
		_, e = tx.ExecContext(ctx, `INSERT INTO pack_sim_manifests_v11(tenant_id,manifest_code,plant_id) VALUES($1,$2,$3)`, tenant, v.Manifest, v.Plant)
	case "LOAD_CARTON":
		var n int
		e = tx.QueryRowContext(ctx, `SELECT count(*)::int FROM pack_sim_items_v11 WHERE tenant_id=$1 AND carton_code=$2`, tenant, v.Carton).Scan(&n)
		if e == nil && n == 0 {
			e = sewFail(409, "ERR_PACK_SIM_EMPTY_CARTON")
		}
		if e == nil {
			_, e = tx.ExecContext(ctx, `INSERT INTO pack_sim_manifest_cartons_v11(tenant_id,manifest_code,carton_code,item_count) VALUES($1,$2,$3,$4)`, tenant, v.Manifest, v.Carton, n)
		}
	case "DISPATCH_REHEARSAL":
		var cartons, items int
		e = tx.QueryRowContext(ctx, `SELECT count(*)::int,COALESCE(sum(item_count),0)::int FROM pack_sim_manifest_cartons_v11 WHERE tenant_id=$1 AND manifest_code=$2`, tenant, v.Manifest).Scan(&cartons, &items)
		if e == nil && (cartons == 0 || items == 0) {
			e = sewFail(409, "ERR_PACK_SIM_EMPTY_MANIFEST")
		}
		if e == nil {
			res, e = tx.ExecContext(ctx, `UPDATE pack_sim_manifests_v11 SET state='DISPATCH_REHEARSAL',dispatched_at=now() WHERE tenant_id=$1 AND manifest_code=$2 AND state='DRAFT'`, tenant, v.Manifest)
		}
		if e == nil {
			var count int64
			count, e = res.RowsAffected()
			if e == nil && count != 1 {
				e = sewFail(409, "ERR_PACK_SIM_NOT_DRAFT")
			}
		}
	}
	if e != nil {
		sewReplyError(w, e)
		return
	}
	reader, _ := json.Marshal(v.Reader)
	_, e = tx.ExecContext(ctx, `INSERT INTO pack_sim_events_v11(tenant_id,event_id,payload_sha256,action,carton_code,manifest_code,serial,actor,station_id,reader_payload) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb)`, tenant, v.EventID, hash, v.Action, v.Carton, v.Manifest, v.Serial, v.Actor, v.Station, string(reader))
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		sewReplyError(w, e)
		return
	}
	docJSON(w, 201, map[string]any{"event_id": v.EventID, "action": v.Action, "replayed": false, "authority": packSimAuthority})
}
func (s *EdgeServer) handlePackSimCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "GET required", 405)
		return
	}
	tenant, ok := s.packTenant(w)
	if !ok {
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
	candidates := []map[string]any{}
	cartons := []map[string]any{}
	manifests := []map[string]any{}
	rows, e := tx.QueryContext(ctx, `SELECT q.serial,b.buyer_code,q.lay_draft_id::text,q.plant_id,q.color_code,q.size_code,COALESCE(i.carton_code,'') FROM qc_sim_units_v10 q JOIN qc_sim_buyer_bindings_v10 b ON b.tenant_id=q.tenant_id AND b.serial=q.serial JOIN qc_sim_routes_v10 rt ON rt.tenant_id=q.tenant_id AND rt.lay_draft_id=q.lay_draft_id AND rt.plant_id=q.plant_id LEFT JOIN pack_sim_items_v11 i ON i.tenant_id=q.tenant_id AND i.serial=q.serial WHERE q.tenant_id=$1 AND q.qc_state='ACCEPTED' AND q.finishing_count=cardinality(rt.steps) ORDER BY q.serial LIMIT 2000`, tenant)
	if e != nil {
		sewReplyError(w, e)
		return
	}
	for rows.Next() {
		var serial, buyer, lay, plant, color, size, carton string
		e = rows.Scan(&serial, &buyer, &lay, &plant, &color, &size, &carton)
		if e != nil {
			break
		}
		candidates = append(candidates, map[string]any{"serial": serial, "buyer_code": buyer, "lay_id": lay, "plant_id": plant, "color": color, "size": size, "carton": carton})
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		sewReplyError(w, e)
		return
	}
	rows, e = tx.QueryContext(ctx, `SELECT c.carton_code,c.lay_draft_id::text,c.plant_id,c.state,count(i.serial)::int,COALESCE(m.manifest_code,'') FROM pack_sim_cartons_v11 c LEFT JOIN pack_sim_items_v11 i ON i.tenant_id=c.tenant_id AND i.carton_code=c.carton_code LEFT JOIN pack_sim_manifest_cartons_v11 m ON m.tenant_id=c.tenant_id AND m.carton_code=c.carton_code WHERE c.tenant_id=$1 GROUP BY c.carton_code,c.lay_draft_id,c.plant_id,c.state,m.manifest_code ORDER BY c.created_at DESC LIMIT 500`, tenant)
	if e != nil {
		sewReplyError(w, e)
		return
	}
	for rows.Next() {
		var carton, lay, plant, state, manifest string
		var n int
		e = rows.Scan(&carton, &lay, &plant, &state, &n, &manifest)
		if e != nil {
			break
		}
		cartons = append(cartons, map[string]any{"carton": carton, "lay_id": lay, "plant_id": plant, "state": state, "items": n, "manifest": manifest})
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		sewReplyError(w, e)
		return
	}
	rows, e = tx.QueryContext(ctx, `SELECT m.manifest_code,m.plant_id,m.state,count(c.carton_code)::int,COALESCE(sum(c.item_count),0)::int FROM pack_sim_manifests_v11 m LEFT JOIN pack_sim_manifest_cartons_v11 c ON c.tenant_id=m.tenant_id AND c.manifest_code=m.manifest_code WHERE m.tenant_id=$1 GROUP BY m.manifest_code,m.plant_id,m.state ORDER BY m.created_at DESC LIMIT 500`, tenant)
	if e != nil {
		sewReplyError(w, e)
		return
	}
	for rows.Next() {
		var manifest, plant, state string
		var c, n int
		e = rows.Scan(&manifest, &plant, &state, &c, &n)
		if e != nil {
			break
		}
		manifests = append(manifests, map[string]any{"manifest": manifest, "plant_id": plant, "state": state, "cartons": c, "garments": n})
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		sewReplyError(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		sewReplyError(w, e)
		return
	}
	docJSON(w, 200, map[string]any{"candidates": candidates, "cartons": cartons, "manifests": manifests, "authority": packSimAuthority})
}
func servePackSimulation(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "GET required", 405)
		return
	}
	if os.Getenv("ENV") != "development" {
		http.Error(w, "Development simulation only", 403)
		return
	}
	b, e := web.GetPackSimulationHTML()
	if e != nil {
		http.Error(w, "Not found", 404)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}
