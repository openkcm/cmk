package apierrors

import (
	"errors"
	"net/http"

	"github.com/openkcm/cmk/internal/api/cmkapi"
)

var (
	ErrListTenants      = errors.New("failed to get tenants")
	ErrGetTenantInfo    = errors.New("failed to get tenant info")
	ErrTransformTenants = errors.New("failed to transform tenants to API")
	ErrTenantIDInPath   = errors.New("tenant ID in path is not allowed")
	ErrTenantNotAllowed = errors.New("user has no permission to access tenant")
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
		Errors: []error{ErrTenantNotAllowed},
		ExposedError: cmkapi.DetailedError{
			Code:    "NO_TENANT_ACCESS",
			Message: "User has no permission to access tenant",
			Status:  http.StatusForbidden,
		},
	},
}
