// Author: Sunil Jain / CyberInQ, 2026-10-27.
// E12/E13: tenant-scoped, source-derived, read-only factory intelligence.
// Rehearsal evidence cannot be used for payroll, commercial dispatch or signed closure.
package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/internal/web"
)

const intelSimAuthority = "REHEARSAL_ONLY_NOT_PRODUCTION_CLOSURE"
const intelSimMaxGap = 2 * time.Hour

type intelStage struct {
	Name   string `json:"name"`
	Count  int    `json:"count"`
	Unit   string `json:"unit"`
	Source string `json:"source"`
}
type intelResidual struct {
	From        string `json:"from"`
	To          string `json:"to"`
	Amount      int    `json:"unaccounted"`
	Status      string `json:"status"`
	Explanation string `json:"explanation"`
}
type intelStats struct {
	N        int      `json:"sample_count"`
	Unknown  int      `json:"unknown_count"`
	P50      *float64 `json:"p50_minutes"`
	P90      *float64 `json:"p90_minutes"`
	P95      *float64 `json:"p95_minutes"`
	Mean     *float64 `json:"mean_minutes"`
	Stddev   *float64 `json:"stddev_minutes"`
	CV       *float64 `json:"cv"`
	Rankable bool     `json:"rankable"`
}
type intelOpStats struct {
	Operation   string     `json:"operation"`
	SAMSeconds  float64    `json:"sam_seconds"`
	Stats       intelStats `json:"per_garment_active"`
	IssueEvents int        `json:"issue_events"`
	Signal      string     `json:"signal"`
	Evidence    string     `json:"evidence"`
}
type intelReport struct {
	LayID            string                    `json:"lay_id"`
	IntakeID         string                    `json:"intake_id"`
	FactoryTimezone  string                    `json:"factory_timezone"`
	GeneratedAt      time.Time                 `json:"generated_at"`
	Authority        string                    `json:"authority"`
	Evidence         string                    `json:"evidence"`
	CutPositions     int                       `json:"cut_marker_positions"`
	CutCandidates    *int                      `json:"cut_bundle_candidate_units"`
	Stages           []intelStage              `json:"stages"`
	Residuals        []intelResidual           `json:"residuals"`
	IntervalStats    map[string]intelStats     `json:"interval_stats"`
	ExclusionReasons map[string]map[string]int `json:"interval_exclusions"`
	OperationStats   []intelOpStats            `json:"operation_stats"`
	Alerts           []string                  `json:"alerts"`
	Methodology      []string                  `json:"methodology"`
}

func intelQuantile(sorted []float64, p float64) *float64 {
	if len(sorted) == 0 {
		return nil
	}
	pos := p * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	v := sorted[lo] + (sorted[hi]-sorted[lo])*(pos-float64(lo))
	return &v
}
func intelDistribution(raw []float64, unknown int) intelStats {
	out := intelStats{N: len(raw), Unknown: unknown, Rankable: len(raw) >= 5}
	if len(raw) == 0 {
		return out
	}
	v := append([]float64(nil), raw...)
	sort.Float64s(v)
	out.P50 = intelQuantile(v, .50)
	out.P90 = intelQuantile(v, .90)
	out.P95 = intelQuantile(v, .95)
	sum := 0.0
	for _, x := range v {
		sum += x
	}
	mean := sum / float64(len(v))
	variance := 0.0
	for _, x := range v {
		variance += (x - mean) * (x - mean)
	}
	sd := math.Sqrt(variance / float64(len(v)))
	out.Mean = &mean
	out.Stddev = &sd
	if mean > 0 {
		cv := sd / mean
		out.CV = &cv
	}
	return out
}

// Calendar cannot be inferred from clicks. Only short positive server-clock gaps
// wholly inside ONE local calendar day are reported as bounded elapsed time.
// These are NOT shift-adjusted queue or paid worker times.
func intelInterval(start, end sql.NullTime, zone *time.Location) (float64, string) {
	if !start.Valid || !end.Valid {
		return 0, "MISSING_ENDPOINT"
	}
	gap := end.Time.Sub(start.Time)
	if gap < 0 {
		return 0, "REVERSED_TIMESTAMPS"
	}
	a, b := start.Time.In(zone), end.Time.In(zone)
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	if ay != by || am != bm || ad != bd {
		return 0, "CROSSES_FACTORY_DAY"
	}
	if gap > intelSimMaxGap {
		return 0, "LONG_GAP_NEEDS_SHIFT_CALENDAR"
	}
	return gap.Minutes(), ""
}
func intelResidualFor(from, to string, a, b int) intelResidual {
	delta := a - b
	r := intelResidual{From: from, To: to, Amount: delta, Status: "BALANCED", Explanation: "Counts reflect stage state, not verified scrap or actual shipment."}
	if delta < 0 {
		r.Status = "NEGATIVE_INVARIANT"
		r.Explanation = "Downstream exceeds upstream; check source genealogy and duplicates."
	}
	if delta > 0 {
		r.Status = "OPEN_WIP_OR_UNKNOWN_LOSS"
		r.Explanation = "Not proof of scrap; units may be in process, held, removed, or unobserved."
	}
	return r
}
func intelReadCounts(ctx context.Context, tx *sql.Tx, tenant, lay string) (map[string]int, error) {
	counts := map[string]int{}
	queries := []struct{ name, sql string }{
		{"cut_marker_positions", `SELECT count(*)::int FROM cut_lay_positions_v8 WHERE tenant_id=$1 AND lay_draft_id=$2::uuid`},
		{"cut_candidates", `SELECT count(*)::int FROM (SELECT marker_position,ply_group FROM cut_bundle_drafts_v8 WHERE tenant_id=$1 AND lay_draft_id=$2::uuid GROUP BY marker_position,ply_group) g`},
		// All per-stage counts use one unique garment serial, never sum operation credits.
		{"sewn", `SELECT count(DISTINCT ser.serial)::int FROM sew_sim_credits_v9 c JOIN sew_sim_cycles_v9 cy ON cy.tenant_id=c.tenant_id AND cy.id=c.cycle_id AND cy.state='COMPLETED' CROSS JOIN LATERAL unnest(c.serials) AS ser(serial) WHERE c.tenant_id=$1 AND c.lay_draft_id=$2::uuid`},
		{"qc_enrolled", `SELECT count(*)::int FROM qc_sim_units_v10 WHERE tenant_id=$1 AND lay_draft_id=$2::uuid`},
		{"qc_accepted", `SELECT count(*)::int FROM qc_sim_units_v10 WHERE tenant_id=$1 AND lay_draft_id=$2::uuid AND qc_state='ACCEPTED'`},
		{"qc_hold", `SELECT count(*)::int FROM qc_sim_units_v10 WHERE tenant_id=$1 AND lay_draft_id=$2::uuid AND qc_state IN ('HOLD','REINSPECT')`},
		{"qc_pending", `SELECT count(*)::int FROM qc_sim_units_v10 WHERE tenant_id=$1 AND lay_draft_id=$2::uuid AND qc_state='PENDING'`},
		{"qc_rework_events", `SELECT count(*)::int FROM qc_sim_events_v10 e JOIN qc_sim_units_v10 q ON q.tenant_id=e.tenant_id AND q.serial=e.serial WHERE e.tenant_id=$1 AND q.lay_draft_id=$2::uuid AND e.action='REPAIR_COMPLETE'`},
		{"finished", `SELECT count(*)::int FROM qc_sim_units_v10 q JOIN qc_sim_routes_v10 r ON r.tenant_id=q.tenant_id AND r.lay_draft_id=q.lay_draft_id AND r.plant_id=q.plant_id WHERE q.tenant_id=$1 AND q.lay_draft_id=$2::uuid AND q.qc_state='ACCEPTED' AND q.finishing_count=cardinality(r.steps)`},
		{"buyer_bound", `SELECT count(*)::int FROM qc_sim_buyer_bindings_v10 b JOIN qc_sim_units_v10 q ON q.tenant_id=b.tenant_id AND q.serial=b.serial WHERE q.tenant_id=$1 AND q.lay_draft_id=$2::uuid`},
		{"packed", `SELECT count(*)::int FROM pack_sim_items_v11 i JOIN qc_sim_units_v10 q ON q.tenant_id=i.tenant_id AND q.serial=i.serial WHERE q.tenant_id=$1 AND q.lay_draft_id=$2::uuid`},
		{"sealed", `SELECT count(*)::int FROM pack_sim_items_v11 i JOIN pack_sim_cartons_v11 c ON c.tenant_id=i.tenant_id AND c.carton_code=i.carton_code JOIN qc_sim_units_v10 q ON q.tenant_id=i.tenant_id AND q.serial=i.serial WHERE q.tenant_id=$1 AND q.lay_draft_id=$2::uuid AND c.state='SEALED'`},
		{"dispatched", `SELECT count(*)::int FROM pack_sim_items_v11 i JOIN qc_sim_units_v10 q ON q.tenant_id=i.tenant_id AND q.serial=i.serial JOIN pack_sim_manifest_cartons_v11 mc ON mc.tenant_id=i.tenant_id AND mc.carton_code=i.carton_code JOIN pack_sim_manifests_v11 m ON m.tenant_id=mc.tenant_id AND m.manifest_code=mc.manifest_code AND m.state='DISPATCH_REHEARSAL' WHERE q.tenant_id=$1 AND q.lay_draft_id=$2::uuid`},
	}
	for _, q := range queries {
		var n int
		if e := tx.QueryRowContext(ctx, q.sql, tenant, lay).Scan(&n); e != nil {
			return nil, e
		}
		counts[q.name] = n
	}
	return counts, nil
}
func intelOperationMetrics(ctx context.Context, tx *sql.Tx, tenant, lay string) ([]intelOpStats, error) {
	rows, e := tx.QueryContext(ctx, `SELECT op.operation_code,op.sam_seconds::float8,cr.active_ms,cr.garment_count,
       (SELECT count(*)::int FROM sew_sim_events_v9 ev WHERE ev.tenant_id=cy.tenant_id AND ev.cycle_id=cy.id AND ev.action='ISSUE')
       FROM sew_sim_credits_v9 cr JOIN sew_sim_cycles_v9 cy ON cy.tenant_id=cr.tenant_id AND cy.id=cr.cycle_id AND cy.state='COMPLETED'
       JOIN sew_sim_operations_v9 op ON op.tenant_id=cr.tenant_id AND op.id=cr.operation_id
       WHERE cr.tenant_id=$1 AND cr.lay_draft_id=$2::uuid ORDER BY op.operation_code,cy.id`, tenant, lay)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	type collector struct {
		sam     float64
		samples []float64
		issues  int
	}
	v := map[string]*collector{}
	for rows.Next() {
		var op string
		var sam float64
		var active int64
		var garments, issues int
		if e = rows.Scan(&op, &sam, &active, &garments, &issues); e != nil {
			return nil, e
		}
		o, exists := v[op]
		if !exists {
			o = &collector{sam: sam}
			v[op] = o
		}
		if garments > 0 && active >= 0 {
			o.samples = append(o.samples, float64(active)/60000/float64(garments))
		}
		o.issues += issues
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	names := make([]string, 0, len(v))
	for k := range v {
		names = append(names, k)
	}
	sort.Strings(names)
	result := make([]intelOpStats, 0, len(names))
	for _, name := range names {
		o := v[name]
		stats := intelDistribution(o.samples, 0)
		item := intelOpStats{Operation: name, SAMSeconds: o.sam, Stats: stats, IssueEvents: o.issues, Signal: "INSUFFICIENT_SAMPLE", Evidence: "sew_sim_credits_v9 active_ms / garment_count; sew_sim_events_v9 ISSUE; simulation only"}
		if stats.Rankable && stats.P90 != nil {
			item.Signal = "OBSERVED_WITHIN_1_5X_SAM"
			if *stats.P90*60 > 1.5*o.sam {
				item.Signal = "OBSERVED_HIGH_P90_VS_SAM"
			}
		}
		result = append(result, item)
	}
	return result, nil
}
func intelTransitionMetrics(ctx context.Context, tx *sql.Tx, tenant, lay string, zone *time.Location) (map[string]intelStats, map[string]map[string]int, error) {
	// Latest sewing completion for each serial; earliest recorded QC enrollment;
	// buyer binding and first packing scan of garment still in a carton.
	rows, e := tx.QueryContext(ctx, `WITH finished AS (
      SELECT ser.serial,max(cy.finished_at) AS done_at FROM sew_sim_credits_v9 cr
      JOIN sew_sim_cycles_v9 cy ON cy.tenant_id=cr.tenant_id AND cy.id=cr.cycle_id AND cy.state='COMPLETED'
      CROSS JOIN LATERAL unnest(cr.serials) AS ser(serial)
      WHERE cr.tenant_id=$1 AND cr.lay_draft_id=$2::uuid GROUP BY ser.serial
    ), qc_enrolled AS (
      SELECT e.serial,min(e.recorded_at) AS start_at FROM qc_sim_events_v10 e
      JOIN qc_sim_units_v10 u ON u.tenant_id=e.tenant_id AND u.serial=e.serial
      WHERE e.tenant_id=$1 AND u.lay_draft_id=$2::uuid AND e.action='ENROLL' GROUP BY e.serial
    ), packed AS (
      SELECT e.serial,min(e.recorded_at) AS scanned_at FROM pack_sim_events_v11 e
      JOIN pack_sim_items_v11 i ON i.tenant_id=e.tenant_id AND i.serial=e.serial AND i.carton_code=e.carton_code
      JOIN qc_sim_units_v10 u ON u.tenant_id=e.tenant_id AND u.serial=e.serial
      WHERE e.tenant_id=$1 AND u.lay_draft_id=$2::uuid AND e.action='ADD_GARMENT' GROUP BY e.serial
    )
    SELECT f.serial,f.done_at,qe.start_at,b.created_at,p.scanned_at
    FROM finished f LEFT JOIN qc_enrolled qe ON qe.serial=f.serial
    LEFT JOIN qc_sim_buyer_bindings_v10 b ON b.tenant_id=$1 AND b.serial=f.serial
    LEFT JOIN packed p ON p.serial=f.serial`, tenant, lay)
	if e != nil {
		return nil, nil, e
	}
	defer rows.Close()
	sample := map[string][]float64{"sewing_to_qc_enrollment": {}, "buyer_binding_to_carton_scan": {}}
	missing := map[string]map[string]int{"sewing_to_qc_enrollment": {}, "buyer_binding_to_carton_scan": {}}
	for rows.Next() {
		var serial string
		var a, b, c, d sql.NullTime
		if e = rows.Scan(&serial, &a, &b, &c, &d); e != nil {
			return nil, nil, e
		}
		for _, segment := range []struct {
			name     string
			from, to sql.NullTime
		}{
			{"sewing_to_qc_enrollment", a, b}, {"buyer_binding_to_carton_scan", c, d},
		} {
			n, reason := intelInterval(segment.from, segment.to, zone)
			if reason != "" {
				missing[segment.name][reason]++
			} else {
				sample[segment.name] = append(sample[segment.name], n)
			}
		}
	}
	if e = rows.Err(); e != nil {
		return nil, nil, e
	}
	result := map[string]intelStats{}
	for k, s := range sample {
		unknown := 0
		for _, n := range missing[k] {
			unknown += n
		}
		result[k] = intelDistribution(s, unknown)
	}
	return result, missing, nil
}
func (s *EdgeServer) intelReport(ctx context.Context, tx *sql.Tx, tenant, lay, tz string) (intelReport, error) {
	out := intelReport{LayID: lay, FactoryTimezone: tz, GeneratedAt: time.Now().UTC(), Authority: intelSimAuthority,
		Evidence: "DERIVED_FROM_UI_SIMULATED_NO_RF_OBSERVATION",
		Stages:   []intelStage{}, Residuals: []intelResidual{}, OperationStats: []intelOpStats{}, Alerts: []string{},
		Methodology: []string{
			"All garment counts deduplicate simulated garment serials. Cut marker positions are NOT garment quantities.",
			"Cut bundle candidate count is grouped marker-position/ply-group count, NOT verified cut piece quantity; no cut-to-shipped identity is asserted.",
			"Deltas are work-in-progress/unknown residuals, not scrap or financial losses.",
			"Intervals use PostgreSQL server-recorded times, never unverified UI reader clicked_at; negative, missing, >2h and crossing factory-local calendar day are unknown.",
			"Bounded elapsed intervals are not shift-adjusted queue, paid touch, machine utilization or causal waiting-time measurements.",
			"Operation active_ms is reported per garment equivalent of completed simulated cycles; SAM comparison is observational, not causal.",
			"Ranking requires at least five samples. No ERP close, physical dispatch, payroll, staffing decision or signed audit is authorized.",
		}}
	e := tx.QueryRowContext(ctx, `SELECT intake_id FROM cut_lay_drafts_v8 WHERE tenant_id=$1 AND id=$2::uuid`, tenant, lay).Scan(&out.IntakeID)
	if e == sql.ErrNoRows {
		return out, sewFail(404, "ERR_INTEL_LAY_NOT_FOUND")
	}
	if e != nil {
		return out, e
	}
	counts, e := intelReadCounts(ctx, tx, tenant, lay)
	if e != nil {
		return out, e
	}
	out.CutPositions = counts["cut_marker_positions"]
	cutCandidates := counts["cut_candidates"]
	if cutCandidates > 0 {
		out.CutCandidates = &cutCandidates
	}
	stageSpec := []struct{ key, name, unit, source string }{
		{"sewn", "Sewing completed", "simulated garment serials", "sew_sim_credits_v9"},
		{"qc_enrolled", "QC enrolled", "simulated garment serials", "qc_sim_units_v10"},
		{"qc_accepted", "QC accepted", "simulated garment serials", "qc_sim_units_v10"},
		{"qc_hold", "QC held/reinspect", "simulated garment serials", "qc_sim_units_v10"},
		{"qc_pending", "QC pending", "simulated garment serials", "qc_sim_units_v10"},
		{"qc_rework_events", "Repairs performed", "repair events (not units)", "qc_sim_events_v10"},
		{"finished", "Finishing complete", "simulated garment serials", "qc_sim_routes_v10"},
		{"buyer_bound", "Buyer identity bound", "simulated garment serials", "qc_sim_buyer_bindings_v10"},
		{"packed", "Currently packed", "simulated garment serials", "pack_sim_items_v11"},
		{"sealed", "In sealed carton", "simulated garment serials", "pack_sim_cartons_v11"},
		{"dispatched", "Dispatch rehearsal", "simulated garment serials", "pack_sim_manifests_v11"},
	}
	for _, t := range stageSpec {
		out.Stages = append(out.Stages, intelStage{Name: t.name, Count: counts[t.key], Unit: t.unit, Source: t.source})
	}
	for _, p := range [][2]string{{"sewn", "qc_enrolled"}, {"qc_enrolled", "qc_accepted"}, {"qc_accepted", "finished"}, {"finished", "buyer_bound"}, {"buyer_bound", "packed"}, {"packed", "sealed"}, {"sealed", "dispatched"}} {
		// Accepted vs enrolled includes HOLD and PENDING: the residual MUST remain visible.
		r := intelResidualFor(p[0], p[1], counts[p[0]], counts[p[1]])
		out.Residuals = append(out.Residuals, r)
		if r.Status == "NEGATIVE_INVARIANT" {
			out.Alerts = append(out.Alerts, "Count invariant violation: "+p[0]+" -> "+p[1])
		}
	}
	if counts["qc_accepted"]+counts["qc_hold"]+counts["qc_pending"] != counts["qc_enrolled"] {
		out.Alerts = append(out.Alerts, "QC partition mismatch: accepted + held + pending != enrolled")
	}
	if out.CutCandidates == nil {
		out.Alerts = append(out.Alerts, "No cut bundle candidates: cut-to-sewing conservation UNKNOWN")
	}
	ops, e := intelOperationMetrics(ctx, tx, tenant, lay)
	if e != nil {
		return out, e
	}
	out.OperationStats = ops
	for _, op := range ops {
		if op.Signal == "OBSERVED_HIGH_P90_VS_SAM" {
			out.Alerts = append(out.Alerts, "Observed high operation p90 vs SAM: "+op.Operation+" (no causal attribution)")
		}
	}
	tzloc, e := time.LoadLocation(tz)
	if e != nil {
		return out, sewFail(422, "ERR_INTEL_TIMEZONE")
	}
	out.IntervalStats, out.ExclusionReasons, e = intelTransitionMetrics(ctx, tx, tenant, lay, tzloc)
	return out, e
}
func intelTenant(w http.ResponseWriter, s *EdgeServer) (string, bool) {
	if os.Getenv("ENV") != "development" {
		docJSON(w, 403, map[string]string{"error": "ERR_INTEL_DEVELOPMENT_ONLY"})
		return "", false
	}
	return s.sewTenant(w)
}
func intelInputs(lay, tz string) error {
	if !sewUUID(lay) {
		return sewFail(422, "ERR_INTEL_LAY_ID")
	}
	if tz == "" || len(tz) > 64 || strings.Contains(tz, "..") || strings.HasPrefix(tz, "/") {
		return sewFail(422, "ERR_INTEL_TIMEZONE")
	}
	if _, e := time.LoadLocation(tz); e != nil {
		return sewFail(422, "ERR_INTEL_TIMEZONE")
	}
	return nil
}
func (s *EdgeServer) handleIntelSimCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "GET required", 405)
		return
	}
	tenant, ok := intelTenant(w, s)
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
	rows, e := tx.QueryContext(ctx, `SELECT id::text,intake_id,cut_no,revision FROM cut_lay_drafts_v8 WHERE tenant_id=$1 ORDER BY created_at DESC LIMIT 100`, tenant)
	if e != nil {
		sewReplyError(w, e)
		return
	}
	data := []map[string]any{}
	for rows.Next() {
		var id, intake string
		var cut, revision int
		if e = rows.Scan(&id, &intake, &cut, &revision); e != nil {
			break
		}
		data = append(data, map[string]any{"lay_id": id, "intake_id": intake, "cut_no": cut, "revision": revision})
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
	docJSON(w, 200, map[string]any{"lays": data, "authority": intelSimAuthority})
}
func (s *EdgeServer) handleIntelSimReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "GET required", 405)
		return
	}
	tenant, ok := intelTenant(w, s)
	if !ok {
		return
	}
	lay, tz := r.URL.Query().Get("lay_id"), r.URL.Query().Get("timezone")
	if e := intelInputs(lay, tz); e != nil {
		sewReplyError(w, e)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	tx, e := s.intelTx(ctx, tenant)
	if e != nil {
		sewReplyError(w, e)
		return
	}
	defer tx.Rollback()
	report, e := s.intelReport(ctx, tx, tenant, lay, tz)
	if e != nil {
		sewReplyError(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		sewReplyError(w, e)
		return
	}
	docJSON(w, 200, report)
}

// Repeatable-read provides a single consistent cut through all stage tables.
func (s *EdgeServer) intelTx(ctx context.Context, tenant string) (*sql.Tx, error) {
	tx, e := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if e != nil {
		return nil, e
	}
	if _, e = tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id',$1,true)`, tenant); e != nil {
		tx.Rollback()
		return nil, e
	}
	return tx, nil
}

type intelCaptureRequest struct {
	CaptureID string `json:"capture_id"`
	LayID     string `json:"lay_id"`
	Timezone  string `json:"timezone"`
}

func (s *EdgeServer) handleIntelSimCapture(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "POST required", 405)
		return
	}
	tenant, ok := intelTenant(w, s)
	if !ok {
		return
	}
	var req intelCaptureRequest
	if !sewStrictJSON(w, r, &req) {
		return
	}
	if !sewCommandID.MatchString(req.CaptureID) {
		sewReplyError(w, sewFail(422, "ERR_INTEL_CAPTURE_ID"))
		return
	}
	if e := intelInputs(req.LayID, req.Timezone); e != nil {
		sewReplyError(w, e)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 18*time.Second)
	defer cancel()
	tx, e := s.intelTx(ctx, tenant)
	if e != nil {
		sewReplyError(w, e)
		return
	}
	defer tx.Rollback()
	reqHash := sewDigest(struct{ Lay, Timezone string }{req.LayID, req.Timezone})
	var existingHash, storedHash string
	e = tx.QueryRowContext(ctx, `SELECT request_sha256,report_sha256 FROM intel_sim_snapshots_v12 WHERE tenant_id=$1 AND capture_id=$2`, tenant, req.CaptureID).Scan(&existingHash, &storedHash)
	if e == nil {
		if existingHash != reqHash {
			sewReplyError(w, sewFail(409, "ERR_INTEL_CAPTURE_COLLISION"))
			return
		}
		docJSON(w, 200, map[string]any{"capture_id": req.CaptureID, "report_sha256": storedHash, "replayed": true, "authority": intelSimAuthority})
		return
	}
	if e != sql.ErrNoRows {
		sewReplyError(w, e)
		return
	}
	report, e := s.intelReport(ctx, tx, tenant, req.LayID, req.Timezone)
	if e != nil {
		sewReplyError(w, e)
		return
	}
	b, e := json.Marshal(report)
	if e != nil {
		sewReplyError(w, e)
		return
	}
	sum := sha256.Sum256(b)
	digest := hex.EncodeToString(sum[:])
	_, e = tx.ExecContext(ctx, `INSERT INTO intel_sim_snapshots_v12(tenant_id,capture_id,request_sha256,lay_draft_id,factory_timezone,report,report_sha256) VALUES($1,$2,$3,$4::uuid,$5,$6::jsonb,$7)`, tenant, req.CaptureID, reqHash, req.LayID, req.Timezone, string(b), digest)
	if e != nil {
		sewReplyError(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		sewReplyError(w, e)
		return
	}
	docJSON(w, 201, map[string]any{"capture_id": req.CaptureID, "report_sha256": digest, "replayed": false, "authority": intelSimAuthority})
}
func (s *EdgeServer) handleIntelSimSnapshots(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "GET required", 405)
		return
	}
	tenant, ok := intelTenant(w, s)
	if !ok {
		return
	}
	lay := r.URL.Query().Get("lay_id")
	if !sewUUID(lay) {
		sewReplyError(w, sewFail(422, "ERR_INTEL_LAY_ID"))
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
	rows, e := tx.QueryContext(ctx, `SELECT capture_id,report_sha256,factory_timezone,captured_at FROM intel_sim_snapshots_v12 WHERE tenant_id=$1 AND lay_draft_id=$2::uuid ORDER BY captured_at DESC LIMIT 50`, tenant, lay)
	if e != nil {
		sewReplyError(w, e)
		return
	}
	result := []map[string]any{}
	for rows.Next() {
		var id, hash, tz string
		var at time.Time
		if e = rows.Scan(&id, &hash, &tz, &at); e != nil {
			break
		}
		result = append(result, map[string]any{"capture_id": id, "report_sha256": hash, "timezone": tz, "captured_at": at})
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
	docJSON(w, 200, map[string]any{"snapshots": result, "authority": intelSimAuthority})
}
func serveIntelSimulation(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "GET required", 405)
		return
	}
	if os.Getenv("ENV") != "development" {
		http.Error(w, "Development simulation only", 403)
		return
	}
	b, e := web.GetIntelSimulationHTML()
	if e != nil {
		http.Error(w, "Not found", 404)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}
