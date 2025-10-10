package tenant

import (
	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/model"
)

// ToAPI transforms a system model to an API system.
func ToAPI(tenant model.Tenant) (*cmkapi.Tenant, error) {
	apiTenant := &cmkapi.Tenant{
		Id:     &tenant.ID,
		Region: tenant.Region,
		Name:   tenant.SchemaName,
	}

	return apiTenant, nil
}
