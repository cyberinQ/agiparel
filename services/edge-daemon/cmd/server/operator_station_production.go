// Author: Sunil Jain, CyberinQ LLC, 2026-10-28.
// F3.9: signed-reader station inbox and non-authoritative handoff guidance.
// Read API never mutates production; the only handoff writer is F3.3's
// independent, worker-signed, transaction-bound movement endpoint.
package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/internal/web"
)

const operatorStationAuthority = "ENROLLED_GATEWAY_REPORT_AND_SIGNED_CUSTODY_NOT_PHYSICAL_CERTIFICATION"

type operatorStationAPI struct {
	server  *EdgeServer
	access  *chapterAccessAPI
	enabled bool
}

type operatorScanEntry struct {
	ScanID     string    `json:"scan_id"`
	PartID     string    `json:"part_id"`
	ReceivedAt time.Time `json:"received_at"`
}
type operatorRouteOption struct {
	RouteID      string `json:"route_id"`
	Action       string `json:"action"`
	StationID    string `json:"station_id"`
	OtherStation string `json:"other_station"`
	Revision     string `json:"revision"`
}
type operatorPartDetail struct {
	ScanID         string                `json:"scan_id"`
	PartID         string                `json:"part_id"`
	GarmentID      string                `json:"garment_id"`
	BundleID       string                `json:"bundle_id"`
	PartCode       string                `json:"part_code"`
	ReceivedAt     time.Time             `json:"received_at"`
	LastAction     string                `json:"last_action"`
	LastStation    string                `json:"last_station"`
	LastRecordedAt *time.Time            `json:"last_recorded_at"`
	Status         string                `json:"status"`
	Blockers       []string              `json:"blockers"`
	Next           []operatorRouteOption `json:"next"`
}
type operatorStationResult struct {
	TenantID    string              `json:"tenant_id"`
	PlantID     string              `json:"plant_id"`
	StationID   string              `json:"station_id"`
	Authority   string              `json:"authority"`
	AsOf        time.Time           `json:"as_of"`
	Scans       []operatorScanEntry `json:"scans"`
	Detail      *operatorPartDetail `json:"detail"`
	Limitations []string            `json:"limitations"`
}

func newOperatorStationAPI(s *EdgeServer) *operatorStationAPI {
	p := &operatorStationAPI{server: s}
	if s == nil || s.db == nil || os.Getenv("AGIPAREL_OPERATOR_STATION_ENABLED") != "true" ||
		(os.Getenv("ENV") != "staging" && os.Getenv("ENV") != "production") ||
		os.Getenv("AGIPAREL_WIP_PRODUCTION_ENABLED") != "true" ||
		os.Getenv("AGIPAREL_GENEALOGY_PRODUCTION_ENABLED") != "true" ||
		os.Getenv("AGIPAREL_CUTTING_RECONCILIATION_ENABLED") != "true" {
		return p
	}
	p.access = newChapterAccessAPI(s)
	p.enabled = p.access.enabled
	return p
}
func operatorStationQuery(q url.Values) (tenant, plant, station, scan string, limit int, errorValue error) {
	for k := range q {
		if k != "tenant_id" && k != "plant_id" && k != "station_id" && k != "scan_id" && k != "limit" {
			errorValue = errors.New("unknown field")
			return
		}
	}
	for _, k := range []string{"tenant_id", "plant_id", "station_id"} {
		if len(q[k]) != 1 || !sewFields(q.Get(k)) {
			errorValue = errors.New("invalid scope")
			return
		}
	}
	tenant, plant, station = q.Get("tenant_id"), q.Get("plant_id"), q.Get("station_id")
	if len(q["scan_id"]) > 1 || (len(q["scan_id"]) == 1 && !sewUUID(q.Get("scan_id"))) {
		errorValue = errors.New("invalid scan")
		return
	}
	scan = q.Get("scan_id")
	limit = 30
	if len(q["limit"]) > 1 {
		errorValue = errors.New("invalid limit")
		return
	}
	if len(q["limit"]) == 1 {
		n, e := strconv.Atoi(q.Get("limit"))
		if e != nil || n < 1 || n > 50 {
			errorValue = errors.New("invalid limit")
			return
		}
		limit = n
	}
	return
}

func (p *operatorStationAPI) page(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "GET required", 405)
		return
	}
	if p == nil || !p.enabled {
		docJSON(w, 503, map[string]string{"error": "ERR_OPERATOR_STATION_DISABLED"})
		return
	}
	content, e := web.GetOperatorStationHTML()
	if e != nil {
		http.Error(w, "View unavailable", 500)
		return
	}
	portalHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodGet {
		_, _ = w.Write(content)
	}
}
func (p *operatorStationAPI) read(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	if p == nil || !p.enabled {
		docJSON(w, 503, map[string]string{"error": "ERR_OPERATOR_STATION_DISABLED"})
		return
	}
	tenant, plant, station, scan, limit, e := operatorStationQuery(r.URL.Query())
	if e != nil {
		docJSON(w, 422, map[string]string{"error": "ERR_OPERATOR_STATION_QUERY"})
		return
	}
	actor, e := (&commissioningAPI{access: p.access}).identify(r, tenant, plant)
	if e != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_OPERATOR_STATION_AUTH"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, e := commissionTx(ctx, p.server.db, tenant)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	defer tx.Rollback()
	readAllowed, e := workforceChapterPermit(ctx, tx, actor, "sewing.read")
	if e != nil {
		sewProdReply(w, e)
		return
	}
	transferAllowed, e := workforceChapterPermit(ctx, tx, actor, "sewing.transfer")
	if e != nil {
		sewProdReply(w, e)
		return
	}
	if !readAllowed && !transferAllowed {
		docJSON(w, 403, map[string]string{"error": "ERR_OPERATOR_STATION_GRANT_REQUIRED"})
		return
	}
	var assigned bool
	e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sew_prod_worker_stations_v10 WHERE tenant_id=$1 AND plant_id=$2 AND station_id=$3 AND worker_subject=$4 AND enabled)`, tenant, plant, station, actor.Subject).Scan(&assigned)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	if !assigned {
		docJSON(w, 403, map[string]string{"error": "ERR_OPERATOR_STATION_WORKER_NOT_ASSIGNED"})
		return
	}
	// Snapshot is bounded, newest first. A missing scan is not evidence that
	// a part is absent; readers or queues can be offline or incomplete.
	rows, e := tx.QueryContext(ctx, `SELECT id::text,part_id::text,received_at FROM wip_gateway_scans_v20
 WHERE tenant_id=$1 AND plant_id=$2 AND station_id=$3 ORDER BY received_at DESC,id DESC LIMIT $4`, tenant, plant, station, limit)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	scans := make([]operatorScanEntry, 0)
	for rows.Next() {
		var v operatorScanEntry
		if e = rows.Scan(&v.ScanID, &v.PartID, &v.ReceivedAt); e != nil {
			break
		}
		scans = append(scans, v)
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		sewProdReply(w, e)
		return
	}
	result := operatorStationResult{TenantID: tenant, PlantID: plant, StationID: station, Authority: operatorStationAuthority,
		AsOf: time.Now().UTC(), Scans: scans,
		Limitations: []string{"Signed gateway scan reports do not independently prove RF presence or station calibration.",
			"This bounded inbox is not an exhaustive inventory or WIP count.",
			"A proposed next handoff is a snapshot; F3.3 independently verifies all conditions at the signed write.",
			"The station view does not establish sewing completion, quality release, payroll credit, shipment or dispatch."}}
	if scan != "" {
		detail, err := p.lookup(ctx, tx, tenant, plant, station, scan, actor.Subject, transferAllowed, time.Now().UTC())
		if errors.Is(err, sql.ErrNoRows) {
			docJSON(w, 404, map[string]string{"error": "ERR_OPERATOR_SCAN_NOT_FOUND"})
			return
		}
		if err != nil {
			sewProdReply(w, err)
			return
		}
		result.Detail = detail
	}
	if e = tx.Commit(); e != nil {
		sewProdReply(w, e)
		return
	}
	docJSON(w, 200, result)
}
func (p *operatorStationAPI) lookup(ctx context.Context, tx *sql.Tx, tenant, plant, station, scan, worker string, transferAllowed bool, now time.Time) (*operatorPartDetail, error) {
	v := &operatorPartDetail{Blockers: []string{}, Next: []operatorRouteOption{}}
	var carrier, binding string
	e := tx.QueryRowContext(ctx, `SELECT s.id::text,s.part_id::text,cp.garment_id::text,cp.bundle_id::text,cp.part_code,
 s.received_at,s.carrier_id::text,s.binding_event_id::text
 FROM wip_gateway_scans_v20 s JOIN gen_cut_parts_v18 cp ON cp.tenant_id=s.tenant_id AND cp.id=s.part_id
 JOIN gen_garments_v18 g ON g.tenant_id=cp.tenant_id AND g.id=cp.garment_id
 WHERE s.tenant_id=$1 AND s.plant_id=$2 AND s.station_id=$3 AND s.id=$4::uuid AND g.plant_id=$2`,
		tenant, plant, station, scan).Scan(&v.ScanID, &v.PartID, &v.GarmentID, &v.BundleID, &v.PartCode, &v.ReceivedAt, &carrier, &binding)
	if e != nil {
		return nil, e
	}
	// A previously valid signed scan can be stale or its carrier can be retired.
	var active, accepted, replaced, consumed bool
	e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM gen_carrier_active_v18 a WHERE a.tenant_id=$1
 AND a.plant_id=$2 AND a.carrier_id=$3::uuid AND a.bound_event_id=$4::uuid
 AND a.target_type='CUT_PART' AND a.target_id=$5::uuid)`, tenant, plant, carrier, binding, v.PartID).Scan(&active)
	if e != nil {
		return nil, e
	}
	e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM cut_part_outcomes_v19 WHERE tenant_id=$1 AND plant_id=$2 AND part_id=$3::uuid AND disposition='ACCEPTED')`, tenant, plant, v.PartID).Scan(&accepted)
	if e != nil {
		return nil, e
	}
	e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM cut_recut_links_v19 WHERE tenant_id=$1 AND plant_id=$2 AND original_part_id=$3::uuid)`, tenant, plant, v.PartID).Scan(&replaced)
	if e != nil {
		return nil, e
	}
	if !transferAllowed {
		v.Blockers = append(v.Blockers, "TRANSFER_PERMISSION_NOT_PROVISIONED")
	}
	if !active {
		v.Blockers = append(v.Blockers, "CARRIER_BINDING_RETIRED_OR_REASSIGNED")
	}
	e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM wip_movements_v20 WHERE tenant_id=$1 AND plant_id=$2 AND scan_id=$3::uuid)`, tenant, plant, scan).Scan(&consumed)
	if e != nil {
		return nil, e
	}
	if consumed {
		v.Blockers = append(v.Blockers, "SCAN_ALREADY_USED_FOR_CUSTODY")
	}
	if !accepted || replaced {
		v.Blockers = append(v.Blockers, "PART_NOT_ELIGIBLE_FOR_WIP")
	}
	age, err := wipScanMaxAge()
	if err != nil {
		return nil, err
	}
	if v.ReceivedAt.After(now.Add(2*time.Minute)) || now.Sub(v.ReceivedAt) > age {
		v.Blockers = append(v.Blockers, "SCAN_TOO_OLD_FOR_HANDOFF")
	}
	var priorRoute, priorWorker string
	var recorded time.Time
	e = tx.QueryRowContext(ctx, `SELECT action,station_id,route_id::text,worker_subject,recorded_at
 FROM wip_movements_v20 WHERE tenant_id=$1 AND plant_id=$2 AND part_id=$3::uuid
 ORDER BY event_no DESC LIMIT 1`, tenant, plant, v.PartID).Scan(&v.LastAction, &v.LastStation, &priorRoute, &priorWorker, &recorded)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return nil, e
	}
	if e == nil {
		v.LastRecordedAt = &recorded
	}
	if errors.Is(e, sql.ErrNoRows) {
		v.LastAction = "NONE"
	}
	var action string
	var routeWhere string
	switch v.LastAction {
	case "NONE":
		action = "DEPART"
		routeWhere = "from_station=$3"
	case "ARRIVE":
		if v.LastStation != station {
			v.Blockers = append(v.Blockers, "LAST_ARRIVAL_AT_DIFFERENT_STATION")
		}
		action = "DEPART"
		routeWhere = "from_station=$3"
	case "DEPART":
		action = "ARRIVE"
		routeWhere = "to_station=$3 AND id=$4::uuid"
		if priorWorker == worker {
			v.Blockers = append(v.Blockers, "INDEPENDENT_RECEIVER_REQUIRED")
		}
	default:
		v.Blockers = append(v.Blockers, "UNRECOGNIZED_MOVEMENT_STATE")
	}
	// Only show actions after every prerequisite passes. The signed writer
	// repeats these checks, including transaction locks, under a fresh snapshot.
	if len(v.Blockers) == 0 && action != "" {
		query := `SELECT id::text,from_station,to_station,revision FROM wip_transfer_routes_v20
 WHERE tenant_id=$1 AND plant_id=$2 AND enabled AND ` + routeWhere + ` ORDER BY revision,id LIMIT 50`
		var rows *sql.Rows
		if action == "ARRIVE" {
			rows, e = tx.QueryContext(ctx, query, tenant, plant, station, priorRoute)
		} else {
			rows, e = tx.QueryContext(ctx, query, tenant, plant, station)
		}
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			var item operatorRouteOption
			var from, to string
			if e = rows.Scan(&item.RouteID, &from, &to, &item.Revision); e != nil {
				break
			}
			item.Action = action
			item.StationID = station
			if action == "DEPART" {
				item.OtherStation = to
			} else {
				item.OtherStation = from
			}
			v.Next = append(v.Next, item)
		}
		if e == nil {
			e = rows.Err()
		}
		rows.Close()
		if e != nil {
			return nil, e
		}
		if len(v.Next) == 0 {
			v.Blockers = append(v.Blockers, "NO_APPROVED_ROUTE_FOR_CURRENT_STATE")
		}
	}
	if len(v.Blockers) > 0 {
		v.Status = "BLOCKED"
	} else {
		v.Status = "READY_FOR_SIGNED_HANDOFF"
	}
	return v, nil
}
func registerOperatorStationRoutes(mux *http.ServeMux, s *EdgeServer) {
	p := newOperatorStationAPI(s)
	mux.HandleFunc("/operator-station", p.page)
	mux.HandleFunc("/api/v1/operator-station/inbox", p.read)
}
