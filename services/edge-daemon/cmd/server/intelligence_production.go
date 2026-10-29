// Author: Sunil Jain, 2026-10-28.
// Chapter 09: authorized, scope-bounded, read-only evidence counts for real
// production *groups*. Never infer unit, labor, profitability or shipment KPIs.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/attestation"
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/workforceauth"
)

const intelProdAuthority = "AUTHENTICATED_GROUP_EVENTS_NOT_GARMENT_RELEASE"
const intelProdChallenge = "INTELLIGENCE_PROD_REPORT_V1"

type intelProdAPI struct {
	server    *EdgeServer
	sewing    *sewProdAPI
	workforce *workforceauth.Verifier
	browser   *workforceBrowser
	enabled   bool
	reason    string
}

type intelProdMetric struct {
	ID      string `json:"id"`
	Value   *int64 `json:"value"`
	Unit    string `json:"unit"`
	Source  string `json:"source"`
	Status  string `json:"status"`
	Meaning string `json:"meaning"`
}

type intelProdReport struct {
	Authority         string                 `json:"authority"`
	TenantID          string                 `json:"tenant_id"`
	PlantID           string                 `json:"plant_id"`
	Since             time.Time              `json:"since"`
	Until             time.Time              `json:"until_exclusive"`
	AsOf              time.Time              `json:"as_of"`
	Basis             string                 `json:"basis"`
	Metrics           []intelProdMetric      `json:"metrics"`
	Limitations       []string               `json:"limitations"`
	ProductionRelease bool                   `json:"production_release"`
	FactoryEvidence   *factoryEvidenceReport `json:"factory_evidence"`
}

func newIntelProdAPI(s *EdgeServer) *intelProdAPI {
	p := &intelProdAPI{server: s, reason: "ERR_INTEL_PROD_DISABLED"}
	if s != nil {
		p.browser = s.browser
	}
	if os.Getenv("AGIPAREL_INTELLIGENCE_PRODUCTION_ENABLED") != "true" {
		return p
	}
	if os.Getenv("ENV") != "production" && os.Getenv("ENV") != "staging" {
		p.reason = "ERR_INTEL_PROD_ENVIRONMENT"
		return p
	}
	p.sewing = newSewProdAPI(s)
	if !p.sewing.enabled {
		p.reason = "ERR_INTEL_PROD_SEWING_PREREQUISITE"
		return p
	}
	if os.Getenv("AGIPAREL_WORKFORCE_OIDC_ENABLED") == "true" {
		var err error
		p.workforce, err = workforceauth.New(os.Getenv("AGIPAREL_WORKFORCE_OIDC_ISSUER"),
			os.Getenv("AGIPAREL_WORKFORCE_OIDC_AUDIENCE"), os.Getenv("AGIPAREL_WORKFORCE_OIDC_JWKS_FILE"))
		if err != nil {
			p.reason = "ERR_INTEL_PROD_WORKFORCE_CONFIG"
			return p // No pilot-token fallback when OIDC mode is enabled but misconfigured.
		}
	}
	p.enabled = true
	return p
}
func (p *intelProdAPI) ready(w http.ResponseWriter) bool {
	if !p.enabled {
		docJSON(w, 503, map[string]string{"error": p.reason})
		return false
	}
	return true
}
func intelProdMaxDays() int {
	days := 31
	if s := os.Getenv("AGIPAREL_INTELLIGENCE_MAX_WINDOW_DAYS"); s != "" {
		if n, e := strconv.Atoi(s); e == nil && n >= 1 && n <= 365 {
			days = n
		}
	}
	return days
}
func intelProdWindow(q map[string][]string, maxDays int, now time.Time) (time.Time, time.Time, error) {
	// RFC3339 instants are unambiguous across shifts and daylight saving.
	sinceRaw, untilRaw := q["since"], q["until"]
	if len(sinceRaw) != 1 || len(untilRaw) != 1 {
		return time.Time{}, time.Time{}, sewProdInvalid("ERR_INTEL_PROD_WINDOW_REQUIRED")
	}
	from, e := time.Parse(time.RFC3339, sinceRaw[0])
	if e != nil {
		return time.Time{}, time.Time{}, sewProdInvalid("ERR_INTEL_PROD_WINDOW")
	}
	to, e := time.Parse(time.RFC3339, untilRaw[0])
	if e != nil {
		return time.Time{}, time.Time{}, sewProdInvalid("ERR_INTEL_PROD_WINDOW")
	}
	from = from.UTC()
	to = to.UTC()
	if !from.Before(to) || to.Sub(from) > time.Duration(maxDays)*24*time.Hour || to.After(now.Add(time.Minute)) {
		return time.Time{}, time.Time{}, sewProdInvalid("ERR_INTEL_PROD_WINDOW")
	}
	return from, to, nil
}
func intelProdMetricOf(id, source, meaning string, n *int64) intelProdMetric {
	m := intelProdMetric{ID: id, Value: n, Unit: "groups", Source: source, Meaning: meaning, Status: "NOT_MEASURED"}
	if n != nil {
		m.Status = "OBSERVED_GROUP_EVENT"
	}
	return m
}
func intelProdMetrics(total, reviewed, hold, reinspect, pending, staged, stale int64) []intelProdMetric {
	known := func(v int64) *int64 { return &v }
	return []intelProdMetric{
		intelProdMetricOf("sewing_completion_groups", "sew_prod_completions_v10", "Worker-attested sewing operation groups, not individual garments or accepted output", known(total)),
		intelProdMetricOf("qc_current_reviewed_groups", "qc_prod_group_events_v11", "Groups whose latest QC decision is REVIEWED, not garment-level release", known(reviewed)),
		intelProdMetricOf("qc_current_hold_groups", "qc_prod_group_events_v11", "Groups whose latest QC decision is HOLD", known(hold)),
		intelProdMetricOf("qc_current_reinspection_groups", "qc_prod_group_events_v11", "Groups awaiting independent reinspection", known(reinspect)),
		intelProdMetricOf("qc_unreviewed_groups", "qc_prod_group_events_v11", "Groups with no QC event yet", known(pending)),
		intelProdMetricOf("packing_current_staging_holds", "pack_prod_holds_v12", "Latest QC-reviewed revision has a staging HOLD; no cartons released", known(staged)),
		intelProdMetricOf("packing_stale_staging_holds", "pack_prod_holds_v12", "Earlier staging HOLD no longer matches the latest QC review", known(stale)),
		intelProdMetricOf("verified_garments", "individual_garment_genealogy", "Group cohorts cannot establish independently physically verified finished garments; see signed unit-event section", nil),
		intelProdMetricOf("dispatched_garments", "verified_carrier_handoff", "No approved production dispatch evidence exists", nil),
		intelProdMetricOf("first_pass_yield", "immutable_unit_first_inspection", "Group reviews do not establish garment-level first-pass yield", nil),
		intelProdMetricOf("labor_efficiency", "approved_sam_and_paid_time", "No verified comparable labor time and earned minutes", nil),
		intelProdMetricOf("order_margin", "approved_actual_cost_and_revenue", "No reconciled actual-cost and commercial basis", nil),
	}
}

// identifyReporter never accepts browser-supplied roles or unsigned proxy identity.
// Standard OIDC access tokens are allowed only for read-only reporting.
// All production mutations continue to require challenge-bound attestations.
func (p *intelProdAPI) identifyReporter(token string, r *http.Request) (attestation.Claims, bool, error) {
	if p.workforce == nil {
		c, err := p.sewing.verifier.Verify(token, "intelligence.read", intelProdChallenge, "factory_manager", true)
		return c, false, err
	}
	identity, err := p.workforce.Verify(token)
	if err != nil {
		return attestation.Claims{}, true, err
	}
	tenant := r.URL.Query()["tenant_id"]
	plant := r.URL.Query()["plant_id"]
	if len(tenant) != 1 || len(plant) != 1 || !sewFields(tenant[0], plant[0]) {
		return attestation.Claims{}, true, sewProdInvalid("ERR_WORKFORCE_SCOPE_REQUIRED")
	}
	return attestation.Claims{Issuer: identity.Issuer, Subject: identity.Subject, TenantID: tenant[0], PlantID: plant[0]}, true, nil
}

// This check runs inside the SAME RLS-scoped report transaction as all metrics.
// The preexisting reader assignment remains an independent, mandatory second gate.
// Kept as the chapter-09 compatibility contract; the shared policy also
// requires workforce_identity_grants_v15 and the exact registered action.
func workforceReportGrant(ctx context.Context, tx *sql.Tx, actor attestation.Claims) (bool, error) {
	return workforceChapterPermit(ctx, tx, actor, "intelligence.read")
}

func (p *intelProdAPI) report(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", 405)
		return
	}
	if !p.ready(w) {
		return
	}
	now := time.Now().UTC()
	from, to, e := intelProdWindow(r.URL.Query(), intelProdMaxDays(), now)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	var actor attestation.Claims
	var enterprise bool
	if r.Header.Get("Authorization") == "" && p.browser != nil && p.browser.enabled && p.workforce != nil {
		var session sessionIdentity
		session, e = p.browser.identity(r)
		if e == nil {
			tenant, plant := r.URL.Query()["tenant_id"], r.URL.Query()["plant_id"]
			if len(tenant) != 1 || len(plant) != 1 || !sewFields(tenant[0], plant[0]) {
				e = sewProdInvalid("ERR_WORKFORCE_SCOPE_REQUIRED")
			} else {
				actor = attestation.Claims{Issuer: session.Issuer, Subject: session.Subject, TenantID: tenant[0], PlantID: plant[0]}
				enterprise = true
			}
		}
	} else {
		var token string
		token, e = bearer(r)
		if e == nil {
			actor, enterprise, e = p.identifyReporter(token, r)
		}
	}
	if e != nil {
		docJSON(w, 401, map[string]string{"error": "ERR_INTEL_PROD_ASSERTION"})
		return
	}
	if !sewFields(actor.TenantID, actor.PlantID) || actor.Subject == "" || len(actor.Subject) > 128 {
		docJSON(w, 403, map[string]string{"error": "ERR_INTEL_PROD_SCOPE"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	tx, e := p.server.intelTx(ctx, actor.TenantID)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	defer tx.Rollback()
	// A workforce grant is now mandatory for BOTH enterprise OIDC and signed
	// pilot assertions. Session/role claims never grant permission by themselves.
	if enterprise { // Enterprise and signed pilot actors use the same grant authority.
		if p.workforce == nil {
			docJSON(w, 503, map[string]string{"error": "ERR_WORKFORCE_CONFIG"})
			return
		}
	}
	granted, err := workforceReportGrant(ctx, tx, actor)
	if err != nil {
		sewProdReply(w, err)
		return
	}
	if !granted {
		docJSON(w, 403, map[string]string{"error": "ERR_WORKFORCE_GRANT_REQUIRED"})
		return
	}
	var allowed bool
	e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM intelligence_prod_readers_v13
       WHERE tenant_id=$1 AND plant_id=$2 AND worker_subject=$3 AND enabled)`, actor.TenantID, actor.PlantID, actor.Subject).Scan(&allowed)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	if !allowed {
		docJSON(w, 403, map[string]string{"error": "ERR_INTEL_PROD_READER_ASSIGNMENT"})
		return
	}
	// Cohort is the set of completion GROUPS created during the selected period.
	// QC and staging are *latest known state at report time*, not a historical
	// period-end state. Snapshot isolation prevents fractured totals.
	var total, reviewed, hold, reinspect, pending, staged, stale int64
	e = tx.QueryRowContext(ctx, `SELECT count(*)::bigint,
        count(*) FILTER (WHERE x.qc_state='REVIEWED')::bigint,
        count(*) FILTER (WHERE x.qc_state='HOLD')::bigint,
        count(*) FILTER (WHERE x.qc_state='REINSPECT')::bigint,
        count(*) FILTER (WHERE x.qc_state='PENDING')::bigint,
        count(*) FILTER (WHERE x.qc_state='REVIEWED' AND x.hold_revision=x.qc_revision)::bigint,
        count(*) FILTER (WHERE x.hold_revision IS NOT NULL AND (x.hold_revision<>x.qc_revision OR x.qc_state<>'REVIEWED'))::bigint
      FROM (
       SELECT COALESCE(q.next_state,'PENDING') AS qc_state,COALESCE(q.revision,0) AS qc_revision,
              h.qc_revision AS hold_revision
       FROM sew_prod_completions_v10 c
       LEFT JOIN LATERAL (SELECT next_state,revision FROM qc_prod_group_events_v11 q
         WHERE q.tenant_id=c.tenant_id AND q.completion_id=c.id ORDER BY revision DESC LIMIT 1) q ON true
       LEFT JOIN LATERAL (SELECT qc_revision FROM pack_prod_holds_v12 h
         WHERE h.tenant_id=c.tenant_id AND h.completion_id=c.id ORDER BY qc_revision DESC LIMIT 1) h ON true
       WHERE c.tenant_id=$1 AND c.plant_id=$2 AND c.created_at >= $3 AND c.created_at < $4
      ) x`, actor.TenantID, actor.PlantID, from, to).Scan(&total, &reviewed, &hold, &reinspect, &pending, &staged, &stale)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	// F3.6: all independently sourced unit metrics are read under the same
	// repeatable-read, tenant-bound report transaction and permission check.
	factory, e := loadFactoryEvidence(ctx, tx, actor.TenantID, actor.PlantID, from, to)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	var asOf time.Time
	if e = tx.QueryRowContext(ctx, `SELECT transaction_timestamp()`).Scan(&asOf); e != nil {
		sewProdReply(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		sewProdReply(w, e)
		return
	}
	report := intelProdReport{
		Authority: intelProdAuthority, TenantID: actor.TenantID, PlantID: actor.PlantID,
		Since: from, Until: to, AsOf: asOf, Basis: "Completion groups created during [since,until); latest QC and staging state as of database snapshot",
		Metrics:         intelProdMetrics(total, reviewed, hold, reinspect, pending, staged, stale),
		FactoryEvidence: &factory,
		Limitations: []string{
			"A sewing completion records a worker-attested operation GROUP, not finished garment units or actual hourly output.",
			"Reader gateway signatures attest an enrolled gateway's report, not independent proof of physical RFID observation.",
			"QC state and staging holds are latest current states at the report snapshot, not historical period-end balances.",
			"The legacy group metric block does not measure individual garments; see separate F3.6 signed unit-event metrics for QC, packing and first-pass yield.",
			"No labor efficiency, cost, margin or buyer OTIF is computed without authorized source systems.",
		}, ProductionRelease: false,
	}
	w.Header().Set("Cache-Control", "no-store")
	docJSON(w, 200, report)
}

func registerIntelProdRoutes(mux *http.ServeMux, s *EdgeServer) {
	p := newIntelProdAPI(s)
	if !p.enabled {
		fmt.Printf("[intelligence-production-v13] disabled (fail closed): %s\n", p.reason)
	}
	mux.HandleFunc("/api/v1/intelligence/production/report", p.report)
}
