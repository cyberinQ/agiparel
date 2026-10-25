package identity

import (
	"context"
	"database/sql"
	"net/http"
)

func TenantMiddleware(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := r.Header.Get("X-Tenant-ID")
			if tenantID == "" {
				http.Error(w, "ERR_MISSING_TENANT: X-Tenant-ID header is required", http.StatusUnauthorized)
				return
			}
			
			// Establish transaction-scoped RLS variable
			ctx := context.WithValue(r.Context(), "tenant_id", tenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}