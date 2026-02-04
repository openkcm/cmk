package tenant

import (
	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/utils/sanitise"
)

// ToAPI transforms a system model to an API system.
func ToAPI(tenant model.Tenant) (*cmkapi.Tenant, error) {
	err := sanitise.Sanitize(&tenant)
	if err != nil {
		return nil, err
	}

	apiTenant := &cmkapi.Tenant{
		Id:     &tenant.ID,
		Region: tenant.Region,
		Name:   tenant.SchemaName,
	}

	return apiTenant, nil
}
