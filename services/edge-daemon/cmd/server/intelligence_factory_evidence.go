// Author: Sunil Jain, 2026-10-28.
// F3.6: evidence-scoped factory metrics derived from append-only F3.1–F3.5
// records. These are signed attestations, not calibrated physical measurements.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"
)

const factoryEvidenceAuthority = "SIGNED_UNIT_AND_CUSTODY_EVENTS_NOT_PHYSICAL_CERTIFICATION"

type factoryEvidenceMetric struct {
	ID          string `json:"id"`
	Value       *int64 `json:"value"`
	Unit        string `json:"unit"`
	Status      string `json:"status"`
	Source      string `json:"source"`
	Meaning     string `json:"meaning"`
	Numerator   *int64 `json:"numerator,omitempty"`
	Denominator *int64 `json:"denominator,omitempty"`
}

type factoryRouteTiming struct {
	RouteID           string `json:"route_id"`
	FromStation       string `json:"from_station"`
	ToStation         string `json:"to_station"`
	CompletedHandoffs int64  `json:"completed_handoffs"`
	P50Milliseconds   int64  `json:"p50_milliseconds"`
	P95Milliseconds   int64  `json:"p95_milliseconds"`
	Evidence          string `json:"evidence"`
}

type factoryEvidenceReport struct {
	Authority         string                  `json:"authority"`
	Basis             string                  `json:"basis"`
	Metrics           []factoryEvidenceMetric `json:"metrics"`
	SlowestRoutes     []factoryRouteTiming    `json:"slowest_routes"`
	Limitations       []string                `json:"limitations"`
	ProductionRelease bool                    `json:"production_release"`
}

func factoryCount(id, source, meaning, unit, status string, value int64) factoryEvidenceMetric {
	return factoryEvidenceMetric{ID: id, Value: &value, Unit: unit, Status: status, Source: source, Meaning: meaning}
}
func factoryAbsent(id, source, meaning, unit, status string) factoryEvidenceMetric {
	return factoryEvidenceMetric{ID: id, Value: nil, Unit: unit, Status: status, Source: source, Meaning: meaning}
}

// Percentages are integer basis points, avoiding float precision ambiguity in
// source-of-truth JSON. A zero denominator is unknown, never an invented 0%.
func factoryRate(id, source, meaning string, numerator, denominator int64) factoryEvidenceMetric {
	if denominator == 0 {
		m := factoryAbsent(id, source, meaning, "basis_points", "NO_DENOMINATOR")
		m.Numerator, m.Denominator = &numerator, &denominator
		return m
	}
	// All inputs are nonnegative counts or millimetres from constrained columns.
	// Float multiplication avoids int64 overflow on aggregated real-world data.
	value := int64(math.Round(float64(numerator) / float64(denominator) * 10000))
	m := factoryCount(id, source, meaning, "basis_points", "DERIVED_SIGNED_ATTESTATIONS", value)
	m.Numerator, m.Denominator = &numerator, &denominator
	return m
}
func factoryDuration(id, source, meaning string, value sql.NullFloat64) factoryEvidenceMetric {
	if !value.Valid || math.IsNaN(value.Float64) || math.IsInf(value.Float64, 0) || value.Float64 < 0 {
		return factoryAbsent(id, source, meaning, "milliseconds", "NO_HANDOFF_PAIRS")
	}
	if value.Float64 > float64(math.MaxInt64)/1000 {
		return factoryAbsent(id, source, meaning, "milliseconds", "INVALID_INTERVAL")
	}
	return factoryCount(id, source, meaning, "milliseconds", "DERIVED_RECORDED_HANDOFF", int64(math.Round(value.Float64*1000)))
}

func loadFactoryEvidence(ctx context.Context, tx *sql.Tx, tenant, plant string, from, to time.Time) (factoryEvidenceReport, error) {
	report := factoryEvidenceReport{
		Authority:     factoryEvidenceAuthority,
		Basis:         "Independent event cohorts by database timestamp in [since,until), in one repeatable-read tenant snapshot. Counts from different stages are not one common garment cohort.",
		SlowestRoutes: []factoryRouteTiming{},
		Limitations: []string{
			"Cut-part and garment inspection outcomes are signed human attestations, not autonomous camera or scale verification.",
			"First-pass yield uses initial garment inspections within this period; it is not group QC and does not include uninspected garments.",
			"WIP intervals are database-recorded departure-to-arrival handoff intervals, not machine cycle times or independently measured travel time.",
			"Carton weights are human attested. A staged dispatch manifest is not carrier acceptance or shipping evidence.",
			"Counts by event timestamp use different cohorts. Do not interpret ratios between stages as order conversion or loss.",
			"OEE, actual fabric utilization, labor efficiency, financial margin and dispatched units remain unmeasured without their respective source systems.",
		},
		ProductionRelease: false,
	}
	query := func(sqlText string, dest ...any) error {
		return tx.QueryRowContext(ctx, sqlText, tenant, plant, from, to).Scan(dest...)
	}
	var cutAccepted, cutRejected int64
	if err := query(`SELECT count(*) FILTER (WHERE disposition='ACCEPTED')::bigint,
      count(*) FILTER (WHERE disposition='REJECTED')::bigint
      FROM cut_part_outcomes_v19 WHERE tenant_id=$1 AND plant_id=$2 AND created_at >= $3 AND created_at < $4`,
		&cutAccepted, &cutRejected); err != nil {
		return report, fmt.Errorf("F36 cut outcomes: %w", err)
	}
	report.Metrics = append(report.Metrics,
		factoryCount("cut_parts_accepted", "cut_part_outcomes_v19", "Independently dual-signed accepted cut-part dispositions in period", "parts", "SIGNED_HUMAN_EVIDENCE", cutAccepted),
		factoryCount("cut_parts_rejected", "cut_part_outcomes_v19", "Independently dual-signed rejected cut-part dispositions in period", "parts", "SIGNED_HUMAN_EVIDENCE", cutRejected),
		factoryRate("cut_part_acceptance_pct", "cut_part_outcomes_v19", "Accepted / (accepted + rejected) cut-part decisions within period", cutAccepted, cutAccepted+cutRejected))

	var initialPass, initialHold, released int64
	if err := query(`SELECT count(*) FILTER (WHERE action='INSPECT_PASS')::bigint,
      count(*) FILTER (WHERE action='INSPECT_HOLD')::bigint,
      count(*) FILTER (WHERE action='QUALITY_RELEASE')::bigint
      FROM qc_garment_events_v21 WHERE tenant_id=$1 AND plant_id=$2 AND recorded_at >= $3 AND recorded_at < $4`,
		&initialPass, &initialHold, &released); err != nil {
		return report, fmt.Errorf("F36 garment QC: %w", err)
	}
	report.Metrics = append(report.Metrics,
		factoryCount("garments_initially_inspected", "qc_garment_events_v21", "Distinct first inspection events; excludes reinspection", "garments", "SIGNED_HUMAN_EVIDENCE", initialPass+initialHold),
		factoryRate("garment_first_pass_yield_pct", "qc_garment_events_v21", "INSPECT_PASS / (INSPECT_PASS + INSPECT_HOLD); first inspections only", initialPass, initialPass+initialHold),
		factoryCount("garments_quality_released", "qc_garment_events_v21", "Quality-manager release attestations; not physical shipment", "garments", "SIGNED_HUMAN_EVIDENCE", released))

	var used, trim, damaged int64
	if err := query(`SELECT coalesce(sum(used_part_mm),0)::bigint,
      coalesce(sum(trim_waste_mm),0)::bigint, coalesce(sum(damaged_mm),0)::bigint
      FROM cut_consumption_reconciliations_v19 WHERE tenant_id=$1 AND plant_id=$2 AND created_at >= $3 AND created_at < $4`,
		&used, &trim, &damaged); err != nil {
		return report, fmt.Errorf("F36 consumption: %w", err)
	}
	report.Metrics = append(report.Metrics,
		factoryCount("reported_cutting_consumption_mm", "cut_consumption_reconciliations_v19", "Dual-attested fabric length classified as used, trim waste or damaged; not physical metrology", "millimeters", "SIGNED_HUMAN_EVIDENCE", used+trim+damaged),
		factoryRate("classified_fabric_use_pct", "cut_consumption_reconciliations_v19", "Reported used length / all reported classified consumed length; NOT CAD marker area efficiency", used, used+trim+damaged))

	var handoffs int64
	var median, p95 sql.NullFloat64
	if err := query(`SELECT count(*)::bigint,
      percentile_cont(0.5) WITHIN GROUP (ORDER BY d.elapsed_seconds),
      percentile_cont(0.95) WITHIN GROUP (ORDER BY d.elapsed_seconds)
      FROM (SELECT extract(epoch FROM (a.recorded_at - dep.recorded_at))::float8 AS elapsed_seconds
       FROM wip_movements_v20 a JOIN wip_movements_v20 dep
         ON dep.tenant_id=a.tenant_id AND dep.part_id=a.part_id
         AND dep.event_no=a.event_no-1 AND dep.action='DEPART' AND dep.route_id=a.route_id
       WHERE a.tenant_id=$1 AND a.plant_id=$2 AND a.action='ARRIVE'
         AND a.recorded_at >= $3 AND a.recorded_at < $4
         AND a.recorded_at >= dep.recorded_at) d`, &handoffs, &median, &p95); err != nil {
		return report, fmt.Errorf("F36 WIP: %w", err)
	}
	report.Metrics = append(report.Metrics,
		factoryCount("wip_completed_handoffs", "wip_movements_v20", "Matched signed DEPART/ARRIVE custody pairs, filtered by arrival time", "handoffs", "SIGNED_GATEWAY_AND_HUMAN_EVIDENCE", handoffs),
		factoryDuration("wip_handoff_p50_ms", "wip_movements_v20", "Median recorded custody handoff interval", median),
		factoryDuration("wip_handoff_p95_ms", "wip_movements_v20", "95th percentile recorded custody handoff interval", p95))

	// Bounded slowest-route diagnostic; aggregation is over all matched pairs
	// before LIMIT. Does not suppress the overall handoff counts or percentile.
	rows, err := tx.QueryContext(ctx, `SELECT r.id::text,r.from_station,r.to_station,
       count(*)::bigint,
       percentile_cont(0.5) WITHIN GROUP (ORDER BY extract(epoch FROM (a.recorded_at-dep.recorded_at)))::float8,
       percentile_cont(0.95) WITHIN GROUP (ORDER BY extract(epoch FROM (a.recorded_at-dep.recorded_at)))::float8
      FROM wip_movements_v20 a JOIN wip_movements_v20 dep
       ON dep.tenant_id=a.tenant_id AND dep.part_id=a.part_id
       AND dep.event_no=a.event_no-1 AND dep.action='DEPART' AND dep.route_id=a.route_id
      JOIN wip_transfer_routes_v20 r ON r.tenant_id=a.tenant_id AND r.id=a.route_id AND r.plant_id=a.plant_id
      WHERE a.tenant_id=$1 AND a.plant_id=$2 AND a.action='ARRIVE'
       AND a.recorded_at >= $3 AND a.recorded_at < $4 AND a.recorded_at >= dep.recorded_at
      GROUP BY r.id,r.from_station,r.to_station
      ORDER BY 6 DESC,r.id LIMIT 10`, tenant, plant, from, to)
	if err != nil {
		return report, fmt.Errorf("F36 route timing: %w", err)
	}
	for rows.Next() {
		var rt factoryRouteTiming
		var m, q sql.NullFloat64
		if err = rows.Scan(&rt.RouteID, &rt.FromStation, &rt.ToStation, &rt.CompletedHandoffs, &m, &q); err != nil {
			break
		}
		if !m.Valid || !q.Valid || m.Float64 < 0 || q.Float64 < 0 || q.Float64 > float64(math.MaxInt64)/1000 {
			err = fmt.Errorf("F36 invalid route interval")
			break
		}
		rt.P50Milliseconds = int64(math.Round(m.Float64 * 1000))
		rt.P95Milliseconds = int64(math.Round(q.Float64 * 1000))
		rt.Evidence = "SIGNED_CUSTODY_HANDOFF_INTERVAL_NOT_MACHINE_CYCLE_TIME"
		report.SlowestRoutes = append(report.SlowestRoutes, rt)
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return report, fmt.Errorf("F36 route rows: %w", err)
	}

	var added, sealed, staged int64
	if err = query(`SELECT count(*) FILTER (WHERE action='ADD')::bigint,
        count(*) FILTER (WHERE action='SEAL')::bigint
       FROM pack_carton_events_v22 WHERE tenant_id=$1 AND plant_id=$2 AND recorded_at >= $3 AND recorded_at < $4`, &added, &sealed); err != nil {
		return report, fmt.Errorf("F36 cartons: %w", err)
	}
	if err = query(`SELECT count(*)::bigint FROM pack_manifest_events_v22
       WHERE tenant_id=$1 AND plant_id=$2 AND action='STAGE' AND recorded_at >= $3 AND recorded_at < $4`, &staged); err != nil {
		return report, fmt.Errorf("F36 staged manifests: %w", err)
	}
	report.Metrics = append(report.Metrics,
		factoryCount("garments_added_to_cartons", "pack_carton_events_v22", "Unique garment ADD attestations; may not yet be sealed", "garments", "SIGNED_HUMAN_EVIDENCE", added),
		factoryCount("cartons_sealed", "pack_carton_events_v22", "Independently signed seals with reported weight in approved tolerance; scale calibration unverified", "cartons", "SIGNED_HUMAN_EVIDENCE", sealed),
		factoryCount("manifests_staged", "pack_manifest_events_v22", "Signed staging events, not verified dock or carrier handover", "manifests", "STAGED_NOT_SHIPPED", staged))
	for _, unknown := range []struct{ id, meaning, unit, source string }{
		{"actual_fabric_utilization_pct", "No CAD component area and measured consumed fabric area", "basis_points", "approved_CAD_and_measured_fabric_area"},
		{"oee_pct", "No machine runtime, planned production time, ideal cycles and independently qualified output", "basis_points", "machine_telemetry_and_approved_cycle_standard"},
		{"labor_efficiency_pct", "No validated SAM and labor attendance/cost evidence", "basis_points", "approved_OB_SAM_and_paid_minutes"},
		{"physically_dispatched_garments", "No independent dock or carrier handover evidence", "garments", "verified_carrier_handover"},
	} {
		report.Metrics = append(report.Metrics, factoryAbsent(unknown.id, unknown.source, unknown.meaning, unknown.unit, "NOT_MEASURED"))
	}
	return report, nil
}
