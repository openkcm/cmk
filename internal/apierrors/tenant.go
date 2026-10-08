package apierrors

import (
	"errors"
	"net/http"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/manager"
)

var ErrTransformTenants = errors.New("failed to transform tenants to API")

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
		InternalErrorChain: []error{manager.ErrListTenants},
		ExposedError: &APIError{
			Code:    "NO_TENANT_ACCESS",
			Message: "User has no permission to access tenant",
			Status:  http.StatusForbidden,
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
