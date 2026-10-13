package apierrors

import (
	"net/http"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/utils/odata"
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
	{
		InternalErrorChain: []error{repo.ErrFieldValueTypeMismatch},
		ExposedError: &APIError{
			Code:    "ODATA_MISMATCH_FIELD_TYPE",
			Message: "OData Filter does not match field type",
			Status:  http.StatusBadRequest,
		},
	},
	{
		InternalErrorChain: []error{odata.ErrFilterInvalidValue},
		ExposedError: &APIError{
			Code:    "ODATA_INVALID_FIELD_VALUE",
			Message: "OData Field value is not valid",
			Status:  http.StatusBadRequest,
		},
	},
}
