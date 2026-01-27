package apierrors

import (
	"errors"
	"net/http"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/manager"
)

var (
	ErrListTenants      = errors.New("failed to get tenants")
	ErrTransformTenants = errors.New("failed to transform tenants to API")
	ErrTenantIDInPath   = errors.New("tenant ID in path is not allowed")
)

var tenants = []APIErrors{
	{
		Errors: []error{manager.ErrTenantNotAllowed},
		ExposedError: cmkapi.DetailedError{
			Code:    "NO_TENANT_ACCESS",
			Message: "User has no permission to access tenant",
			Status:  http.StatusForbidden,
		},
	},
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
}
