package apierrors

import (
	"errors"
	"net/http"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
)

var (
	ErrListTenants      = errors.New("failed to get tenants")
	ErrTransformTenants = errors.New("failed to transform tenants to API")
	ErrTenantIDInPath   = errors.New("tenant ID in path is not allowed")
)

var tenants = []APIErrors{
	{
		Errors: []error{ErrListTenants},
		ExposedError: cmkapi.DetailedError{
			Code:    "GET_TENANTS",
			Message: "Failed to get tenants",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrTransformTenants},
		ExposedError: cmkapi.DetailedError{
			Code:    "TRANSFORM_TENANTS_TO_API",
			Message: "failed to transform tenants to API",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrListTenants, ErrTenantIDInPath},
		ExposedError: cmkapi.DetailedError{
			Code:    "TENANT_ID_IN_PATH",
			Message: "Tenant ID in path is not allowed",
			Status:  http.StatusBadRequest,
		},
	},
}
