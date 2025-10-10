package middleware

import (
	"errors"
	"net/http"

	multitenancyMiddleware "github.com/bartventer/gorm-multitenancy/middleware/nethttp/v8"
)

const (
	// TenantPathParamName is the name of the path parameter used to extract the tenant ID.
	TenantPathParamName = "tenant"
)

// ErrTenantNotFound is returned when the tenant ID is not found in the request.
var ErrTenantNotFound = errors.New("tenant not found as path parameter")

// InjectMultiTenancy returns a middleware function that handles multi-tenancy based on path parameter.
func InjectMultiTenancy() func(http.Handler) http.Handler {
	WithTenantConfig := multitenancyMiddleware.DefaultWithTenantConfig
	WithTenantConfig.TenantGetters = []func(r *http.Request) (string, error){
		func(r *http.Request) (string, error) {
			tenant := r.PathValue(TenantPathParamName)
			if tenant == "" {
				return "", ErrTenantNotFound
			}

			return tenant, nil
		},
	}

	return multitenancyMiddleware.WithTenant(WithTenantConfig)
}
