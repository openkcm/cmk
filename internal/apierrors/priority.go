package apierrors

import (
	"net/http"

	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	"github.tools.sap/kms/cmk/internal/repo"
)

const (
	TenantNotFound = "TENANT_NOT_FOUND"
)

var highPrio = []APIErrors{
	{
		Errors: []error{repo.ErrTenantNotFound},
		ExposedError: cmkapi.DetailedError{
			Code:    TenantNotFound,
			Message: "Tenant does not exist",
			Status:  http.StatusNotFound,
		},
	},
}
