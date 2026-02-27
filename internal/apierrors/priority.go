package apierrors

import (
	"net/http"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/repo"
)

const (
	TenantNotFound = "TENANT_NOT_FOUND"
)

var highPrio = []errs.ExposedErrors[*APIError]{
	{
		InternalErrorChain: []error{repo.ErrTenantNotFound},
		ExposedError: &APIError{
			Code:    TenantNotFound,
			Message: "Tenant does not exist",
			Status:  http.StatusNotFound,
		},
	},
}
