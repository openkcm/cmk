package apierrors

import (
	"net/http"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/repo"
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
