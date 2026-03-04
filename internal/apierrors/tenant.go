package apierrors

import (
	"errors"
	"net/http"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/manager"
)

var (
	ErrListTenants      = errors.New("failed to get tenants")
	ErrTransformTenants = errors.New("failed to transform tenants to API")
	ErrTenantIDInPath   = errors.New("tenant ID in path is not allowed")
)

var tenants = []errs.ExposedErrors[*APIError]{
	{
		InternalErrorChain: []error{manager.ErrTenantNotAllowed},
		ExposedError: &APIError{
			Code:    "NO_TENANT_ACCESS",
			Message: "User has no permission to access tenant",
			Status:  http.StatusForbidden,
		},
	},
	{
		InternalErrorChain: []error{ErrListTenants},
		ExposedError: &APIError{
			Code:    "GET_TENANTS",
			Message: "Failed to get tenants",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{ErrTransformTenants},
		ExposedError: &APIError{
			Code:    "TRANSFORM_TENANTS_TO_API",
			Message: "failed to transform tenants to API",
			Status:  http.StatusInternalServerError,
		},
	},
}
