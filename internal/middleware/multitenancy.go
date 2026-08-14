package middleware

import (
	"context"
	"net/http"

	"github.com/openkcm/cmk/internal/handlers"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

const (
	// TenantPathParamName is the name of the path parameter used to extract the tenant ID.
	TenantPathParamName = "tenant"
)

// InjectMultiTenancy returns a middleware that extracts the tenant ID from the request path
// parameter and stores it in the request context.
func InjectMultiTenancy() func(http.Handler) http.Handler {
	handleError := handlers.ResponseErrorHandlerFunc()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenant := r.PathValue(TenantPathParamName)
			if tenant == "" {
				handleError(w, r, cmkcontext.ErrTenantInvalid)
				return
			}

			ctx := context.WithValue(r.Context(), cmkcontext.TenantKey, tenant)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
