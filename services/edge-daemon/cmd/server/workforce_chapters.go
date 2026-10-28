// Author: Sunil Jain, 2026-10-28.
// F1.3: shared explicit-action, tenant/plant-scoped workforce authorization.
// Existing independent station/role assignments and signed transactional proofs
// are additional requirements; a role or session never authorizes a write alone.
package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/attestation"
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/workforceauth"
)

type chapterPermission struct {
	Chapter   int
	Authority string
}

// Exact mapping is mirrored in migration 0036; unknown permissions fail closed.
var chapterPermissions = map[string]chapterPermission{
	"factory.read":      {1, "READ"},
	"factory.manage":    {1, "SIGNED_WRITE"},
	"orders.read":       {2, "READ"},
	"orders.manage":     {2, "SIGNED_WRITE"},
	"planning.read":     {3, "READ"},
	"planning.approve":  {3, "SIGNED_WRITE"},
	"stores.read":       {4, "READ"},
	"stores.move":       {4, "SIGNED_WRITE"},
	"cutting.read":      {5, "READ"},
	"cutting.release":   {5, "SIGNED_WRITE"},
	"sewing.read":       {6, "SIGNED_READ"},
	"sewing.complete":   {6, "SIGNED_WRITE"},
	"sewing.transfer":   {6, "SIGNED_WRITE"},
	"quality.read":      {7, "SIGNED_READ"},
	"quality.inspect":   {7, "SIGNED_WRITE"},
	"quality.repair":    {7, "SIGNED_WRITE"},
	"packing.read":      {8, "SIGNED_READ"},
	"packing.stage":     {8, "SIGNED_WRITE"},
	"intelligence.read": {9, "READ"},
	"documents.read":    {10, "READ"},
	"documents.manage":  {10, "SIGNED_WRITE"},
}

func validChapterActor(a attestation.Claims) bool {
	return sewFields(a.TenantID, a.PlantID) && len(a.Subject) > 0 && len(a.Subject) <= 128 &&
		len(a.Issuer) >= 9 && len(a.Issuer) <= 512 && len(a.Issuer) >= len("https://") && a.Issuer[:len("https://")] == "https://"
}

// Must execute inside a transaction that has already set app.current_tenant_id.
// Both an independently provisioned grant and an exact catalog action must exist.
func workforceChapterPermit(ctx context.Context, tx *sql.Tx, a attestation.Claims, permission string) (bool, error) {
	policy, ok := chapterPermissions[permission]
	if !ok || !validChapterActor(a) {
		return false, errors.New("ERR_WORKFORCE_CHAPTER_POLICY")
	}
	var allowed bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(
 SELECT 1 FROM workforce_identity_grants_v15 g
 JOIN workforce_chapter_actions_v17 p ON p.permission=g.permission AND p.chapter_id=$6 AND p.authority=$7
 WHERE g.tenant_id=$1 AND g.plant_id=$2 AND g.idp_issuer=$3 AND g.idp_subject=$4
 AND g.permission=$5 AND g.enabled)`,
		a.TenantID, a.PlantID, a.Issuer, a.Subject, permission, policy.Chapter, policy.Authority).Scan(&allowed)
	return allowed, err
}

// This check is deliberately placed before idempotent replay results: revoking
// access immediately prevents a worker from reading or retrying old commands.
func workforceChapterRequire(ctx context.Context, tx *sql.Tx, a attestation.Claims, permission string) error {
	allowed, err := workforceChapterPermit(ctx, tx, a, permission)
	if err != nil {
		return err
	}
	if !allowed {
		return sewFail(http.StatusForbidden, "ERR_WORKFORCE_CHAPTER_GRANT_REQUIRED")
	}
	return nil
}

type chapterAccessEntry struct {
	Chapter            int      `json:"chapter"`
	Permissions        []string `json:"granted_permissions"`
	ProductionAPI      bool     `json:"production_api_enabled"`
	Authorization      string   `json:"authorization"`
	NotProductionReady bool     `json:"not_production_ready"`
}
type chapterAccessAPI struct {
	server   *EdgeServer
	browser  *workforceBrowser
	verifier *workforceauth.Verifier
	enabled  bool
}

func newChapterAccessAPI(s *EdgeServer) *chapterAccessAPI {
	p := &chapterAccessAPI{server: s}
	if s == nil || s.db == nil || (os.Getenv("ENV") != "production" && os.Getenv("ENV") != "staging") || os.Getenv("AGIPAREL_WORKFORCE_OIDC_ENABLED") != "true" {
		return p
	}
	v, err := workforceauth.New(os.Getenv("AGIPAREL_WORKFORCE_OIDC_ISSUER"), os.Getenv("AGIPAREL_WORKFORCE_OIDC_AUDIENCE"), os.Getenv("AGIPAREL_WORKFORCE_OIDC_JWKS_FILE"))
	if err != nil {
		return p
	}
	p.verifier = v
	p.browser = s.browser
	p.enabled = true
	return p
}
func chapterProductionStatus(ch int) (bool, string) {
	switch ch {
	case 6:
		return os.Getenv("AGIPAREL_SEWING_PRODUCTION_ENABLED") == "true", "SIGNED_TRANSACTION_PROOF_REQUIRED"
	case 7:
		return os.Getenv("AGIPAREL_QUALITY_PRODUCTION_ENABLED") == "true", "SIGNED_TRANSACTION_PROOF_REQUIRED"
	case 8:
		return os.Getenv("AGIPAREL_PACKING_PRODUCTION_ENABLED") == "true", "SIGNED_TRANSACTION_PROOF_REQUIRED"
	case 9:
		return os.Getenv("AGIPAREL_INTELLIGENCE_PRODUCTION_ENABLED") == "true", "READ_ONLY_OIDC_AND_REPORT_ASSIGNMENT_REQUIRED"
	default:
		return false, "NO_PRODUCTION_ROUTE_AVAILABLE"
	}
}
func (p *chapterAccessAPI) report(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	if !p.enabled {
		docJSON(w, 503, map[string]string{"error": "ERR_WORKFORCE_CHAPTER_DISABLED"})
		return
	}
	q := r.URL.Query()
	tenants, plants := q["tenant_id"], q["plant_id"]
	if len(tenants) != 1 || len(plants) != 1 || !sewFields(tenants[0], plants[0]) {
		docJSON(w, 422, map[string]string{"error": "ERR_WORKFORCE_SCOPE_REQUIRED"})
		return
	}
	a := attestation.Claims{TenantID: tenants[0], PlantID: plants[0]}
	if r.Header.Get("Authorization") != "" {
		token, e := bearer(r)
		if e != nil {
			docJSON(w, 401, map[string]string{"error": "ERR_WORKFORCE_AUTH"})
			return
		}
		identity, e := p.verifier.Verify(token)
		if e != nil {
			docJSON(w, 401, map[string]string{"error": "ERR_WORKFORCE_AUTH"})
			return
		}
		a.Issuer, a.Subject = identity.Issuer, identity.Subject
	} else if p.browser != nil && p.browser.enabled {
		identity, e := p.browser.identity(r)
		if e != nil {
			docJSON(w, 401, map[string]string{"error": "ERR_WORKFORCE_AUTH"})
			return
		}
		a.Issuer, a.Subject = identity.Issuer, identity.Subject
	} else {
		docJSON(w, 401, map[string]string{"error": "ERR_WORKFORCE_AUTH"})
		return
	}
	if !validChapterActor(a) {
		docJSON(w, 403, map[string]string{"error": "ERR_WORKFORCE_SCOPE"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	tx, e := p.server.docTransaction(ctx, a.TenantID)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, `SELECT c.chapter_id,c.permission FROM workforce_identity_grants_v15 g
 JOIN workforce_chapter_actions_v17 c ON c.permission=g.permission
 WHERE g.tenant_id=$1 AND g.plant_id=$2 AND g.idp_issuer=$3 AND g.idp_subject=$4 AND g.enabled
 ORDER BY c.chapter_id,c.permission`, a.TenantID, a.PlantID, a.Issuer, a.Subject)
	if e != nil {
		sewProdReply(w, e)
		return
	}
	grants := make(map[int][]string)
	for rows.Next() {
		var ch int
		var permission string
		if e = rows.Scan(&ch, &permission); e != nil {
			break
		}
		grants[ch] = append(grants[ch], permission)
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
	entries := make([]chapterAccessEntry, 0, 10)
	for ch := 1; ch <= 10; ch++ {
		enabled, mode := chapterProductionStatus(ch)
		permissions := grants[ch]
		if permissions == nil {
			permissions = []string{}
		}
		sort.Strings(permissions)
		entries = append(entries, chapterAccessEntry{Chapter: ch, Permissions: permissions, ProductionAPI: enabled, Authorization: mode, NotProductionReady: ch <= 5 || ch == 10})
	}
	docJSON(w, 200, map[string]any{"tenant_id": a.TenantID, "plant_id": a.PlantID, "chapters": entries,
		"authority":   "PROVISIONED_ACTION_ENTITLEMENTS_ONLY_NOT_PHYSICAL_OR_TRANSACTION_APPROVAL",
		"limitations": []string{"Chapters 01-05 and 10 have no approved production route.", "Chapters 06-08 require independent signed transaction proofs and station/role assignments.", "Chapter 09 additionally requires its independent production-report reader assignment.", "A grant is not permission to ship, pay wages, or authorize inventory."}})
}
func registerWorkforceChapterRoutes(mux *http.ServeMux, s *EdgeServer) {
	p := newChapterAccessAPI(s)
	mux.HandleFunc("/api/v1/workforce/chapter-access", p.report)
}
