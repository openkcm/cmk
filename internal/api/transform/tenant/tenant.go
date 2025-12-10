package tenant

import (
	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/utils/sanitise"
)

// ToAPI transforms a system model to an API system.
func ToAPI(tenant model.Tenant) (*cmkapi.Tenant, error) {
	err := sanitise.Stringlikes(&tenant)
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
