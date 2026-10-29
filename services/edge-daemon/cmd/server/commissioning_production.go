// Author: Sunil Jain, CyberinQ LLC, 2026-10-28.
// F3.8: source-backed, read-only station commissioning visibility.
// No reader enrollment, new identity authority or physical certification is granted.
package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/internal/web"
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/attestation"
)

const commissioningAuthority = "ENROLLMENT_AND_SIGNED_REPORTS_NOT_PHYSICAL_CERTIFICATION"

type commissioningAPI struct {
	server  *EdgeServer
	access  *chapterAccessAPI
	enabled bool
}
type commissioningStation struct {
	StationID             string     `json:"station_id"`
	EnabledGateways       int64      `json:"enabled_gateways"`
	TotalGateways         int64      `json:"total_gateways"`
	AssignedWorkers       int64      `json:"assigned_workers"`
	EnabledInboundRoutes  int64      `json:"enabled_inbound_routes"`
	EnabledOutboundRoutes int64      `json:"enabled_outbound_routes"`
	LatestSignedScan      *time.Time `json:"last_signed_scan_received_at"`
	Status                string     `json:"status"`
	Blockers              []string   `json:"blockers"`
}
type commissioningResponse struct {
	TenantID    string                 `json:"tenant_id"`
	PlantID     string                 `json:"plant_id"`
	Authority   string                 `json:"authority"`
	AsOf        time.Time              `json:"as_of"`
	Stations    []commissioningStation `json:"stations"`
	NextStation string                 `json:"next_station"`
	Limitations []string               `json:"limitations"`
}

func newCommissioningAPI(s *EdgeServer) *commissioningAPI {
	p := &commissioningAPI{server: s}
	if s == nil || s.db == nil || os.Getenv("AGIPAREL_COMMISSIONING_READ_ENABLED") != "true" || (os.Getenv("ENV") != "staging" && os.Getenv("ENV") != "production") {
		return p
	}
	p.access = newChapterAccessAPI(s)
	p.enabled = p.access.enabled
	return p
}
func commissionPage(q map[string][]string) (string, string, string, int, error) {
	one := func(k string) (string, error) {
		v := q[k]
		if len(v) != 1 {
			return "", errors.New("missing or duplicate scope")
		}
		return v[0], nil
	}
	tenant, e := one("tenant_id")
	if e != nil {
		return "", "", "", 0, e
	}
	plant, e := one("plant_id")
	if e != nil {
		return "", "", "", 0, e
	}
	if !sewFields(tenant, plant) {
		return "", "", "", 0, errors.New("invalid scope")
	}
	cursor := ""
	if x, ok := q["cursor"]; ok {
		if len(x) != 1 || !sewFields(x[0]) {
			return "", "", "", 0, errors.New("invalid cursor")
		}
		cursor = x[0]
	}
	limit := 50
	if x, ok := q["limit"]; ok {
		if len(x) != 1 {
			return "", "", "", 0, errors.New("invalid limit")
		}
		n, e := strconv.Atoi(x[0])
		if e != nil || n < 1 || n > 100 {
			return "", "", "", 0, errors.New("invalid limit")
		}
		limit = n
	}
	for key := range q {
		if key != "tenant_id" && key != "plant_id" && key != "cursor" && key != "limit" {
			return "", "", "", 0, errors.New("unknown query field")
		}
	}
	return tenant, plant, cursor, limit, nil
}
func commissionClassify(s *commissioningStation) {
	s.Blockers = []string{}
	if s.EnabledGateways == 0 {
		s.Blockers = append(s.Blockers, "NO_ENABLED_ENROLLED_GATEWAY")
	}
	if s.AssignedWorkers == 0 {
		s.Blockers = append(s.Blockers, "NO_ENABLED_WORKER_STATION_ASSIGNMENT")
	}
	if s.EnabledInboundRoutes+s.EnabledOutboundRoutes == 0 {
		s.Blockers = append(s.Blockers, "NO_ENABLED_TRANSFER_ROUTE")
	}
	if len(s.Blockers) > 0 {
		s.Status = "CONFIGURATION_INCOMPLETE"
		return
	}
	s.Status = "CONFIGURED_NOT_PHYSICALLY_CERTIFIED"
}
func (p *commissioningAPI) identify(r *http.Request, tenant, plant string) (attestation.Claims, error) {
	a := attestation.Claims{TenantID: tenant, PlantID: plant}
	if r.Header.Get("Authorization") != "" {
		token, e := bearer(r)
		if e != nil {
			return a, e
		}
		id, e := p.access.verifier.Verify(token)
		if e != nil {
			return a, e
		}
		a.Issuer, a.Subject = id.Issuer, id.Subject
	} else if p.access.browser != nil && p.access.browser.enabled {
		id, e := p.access.browser.identity(r)
		if e != nil {
			return a, e
		}
		a.Issuer, a.Subject = id.Issuer, id.Subject
	} else {
		return a, errors.New("ERR_COMMISSION_AUTH_REQUIRED")
	}
	if !validChapterActor(a) {
		return a, errors.New("ERR_COMMISSION_INVALID_IDENTITY")
	}
	return a, nil
}

// A single repeatable-read snapshot avoids combining enrollment states from
// different moments. No production INSERT, UPDATE, DELETE or session mutation.
func commissionTx(ctx context.Context, db *sql.DB, tenant string) (*sql.Tx, error) {
	tx, e := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if e != nil {
		return nil, e
	}
	// Force the restricted application role even if the connection pool was
	// accidentally configured with a more privileged login.
	if _, e = tx.ExecContext(ctx, `SET LOCAL ROLE agiparel_app`); e != nil {
		tx.Rollback()
		return nil, e
	}
	if _, e = tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id',$1,true)`, tenant); e != nil {
		tx.Rollback()
		return nil, e
	}
	return tx, nil
}

const commissioningStationsSQL = `WITH
 gw AS (SELECT station_id,count(*)::bigint total,count(*) FILTER(WHERE enabled)::bigint active
  FROM sew_prod_gateways_v10 WHERE tenant_id=$1 AND plant_id=$2 GROUP BY station_id),
 workers AS (SELECT station_id,count(*) FILTER(WHERE enabled)::bigint active
  FROM sew_prod_worker_stations_v10 WHERE tenant_id=$1 AND plant_id=$2 GROUP BY station_id),
 inbound AS (SELECT to_station station_id,count(*) FILTER(WHERE enabled)::bigint active
  FROM wip_transfer_routes_v20 WHERE tenant_id=$1 AND plant_id=$2 GROUP BY to_station),
 outbound AS (SELECT from_station station_id,count(*) FILTER(WHERE enabled)::bigint active
  FROM wip_transfer_routes_v20 WHERE tenant_id=$1 AND plant_id=$2 GROUP BY from_station),
 scans AS (SELECT station_id,max(received_at) last_received FROM wip_gateway_scans_v20
  WHERE tenant_id=$1 AND plant_id=$2 GROUP BY station_id),
 ids AS (SELECT station_id FROM gw UNION SELECT station_id FROM workers
        UNION SELECT station_id FROM inbound UNION SELECT station_id FROM outbound)
 SELECT ids.station_id,COALESCE(gw.active,0),COALESCE(gw.total,0),
 COALESCE(workers.active,0),COALESCE(inbound.active,0),COALESCE(outbound.active,0),scans.last_received
 FROM ids LEFT JOIN gw USING(station_id) LEFT JOIN workers USING(station_id)
 LEFT JOIN inbound USING(station_id) LEFT JOIN outbound USING(station_id)
 LEFT JOIN scans USING(station_id) WHERE ids.station_id COLLATE "C" > $3 COLLATE "C"
 ORDER BY ids.station_id COLLATE "C" LIMIT $4`

func (p *commissioningAPI) report(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	if p == nil || !p.enabled {
		docJSON(w, 503, map[string]string{"error": "ERR_COMMISSIONING_DISABLED"})
		return
	}
	tenant, plant, cursor, limit, e := commissionPage(r.URL.Query())
	if e != nil {
		docJSON(w, 422, map[string]string{"error": "ERR_COMMISSIONING_SCOPE_OR_PAGE"})
		return
	}
	actor, e := p.identify(r, tenant, plant)
	if e != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_COMMISSIONING_AUTH"})
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
	allowed, e := workforceChapterPermit(ctx, tx, actor, "sewing.read")
	if e != nil {
		sewProdReply(w, e)
		return
	}
	if !allowed {
		allowed, e = workforceChapterPermit(ctx, tx, actor, "sewing.transfer")
		if e != nil {
			sewProdReply(w, e)
			return
		}
	}
	if !allowed {
		docJSON(w, 403, map[string]string{"error": "ERR_COMMISSIONING_GRANT_REQUIRED"})
		return
	}
	rows, e := tx.QueryContext(ctx, commissioningStationsSQL, tenant, plant, cursor, limit+1)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	stations := make([]commissioningStation, 0, limit)
	next := ""
	for rows.Next() {
		var s commissioningStation
		var scanned sql.NullTime
		if e = rows.Scan(&s.StationID, &s.EnabledGateways, &s.TotalGateways, &s.AssignedWorkers, &s.EnabledInboundRoutes, &s.EnabledOutboundRoutes, &scanned); e != nil {
			break
		}
		if len(stations) == limit {
			next = stations[len(stations)-1].StationID
			break
		}
		if scanned.Valid {
			t := scanned.Time.UTC()
			s.LatestSignedScan = &t
		}
		commissionClassify(&s)
		stations = append(stations, s)
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		sewProdReply(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		sewProdReply(w, e)
		return
	}
	docJSON(w, 200, commissioningResponse{TenantID: tenant, PlantID: plant, Authority: commissioningAuthority,
		AsOf: time.Now().UTC(), Stations: stations, NextStation: next,
		Limitations: []string{"Enrollment and approved routes are configuration evidence, not reader calibration or physical commissioning.",
			"Gateway scan timestamps are signed gateway reports received by the server, not independently verified physical presence.",
			"An assigned worker is a provisioned count, not proof of operator availability or skill.",
			"Missing stations may indicate absent provisioning; zero rows must not be interpreted as a fully commissioned plant.",
			"This read-only screen cannot authorize production, QC, inventory, wages, carton shipment or dispatch."}})
}
func (p *commissioningAPI) page(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "GET required", 405)
		return
	}
	if p == nil || !p.enabled {
		docJSON(w, 503, map[string]string{"error": "ERR_COMMISSIONING_DISABLED"})
		return
	}
	content, e := web.GetCommissioningHTML()
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
func registerCommissioningRoutes(mux *http.ServeMux, s *EdgeServer) {
	p := newCommissioningAPI(s)
	mux.HandleFunc("/commissioning", p.page)
	mux.HandleFunc("/api/v1/commissioning/stations", p.report)
}
