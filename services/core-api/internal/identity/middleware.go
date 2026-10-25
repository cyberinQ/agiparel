// Author: Sunil Jain, CyberinQ LLC
package identity

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
)

type ctxKey string

const TenantContextKey ctxKey = "tenant_id"

func TenantFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(TenantContextKey).(string); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return "TENANT_DEFAULT"
}

func TenantMiddleware(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
			if tenantID == "" {
				http.Error(w, "ERR_MISSING_TENANT: X-Tenant-ID header is required", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), TenantContextKey, tenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
