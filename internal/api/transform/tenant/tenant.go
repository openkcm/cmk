package tenant

import (
	"strings"

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

	roleStr := strings.TrimPrefix(string(tenant.Role), "ROLE_")
	role := cmkapi.TenantRole(roleStr)
	apiTenant := &cmkapi.Tenant{
		Id:     &tenant.ID,
		Region: tenant.Region,
		Name:   tenant.SchemaName,
		Role:   &role,
	}

	return apiTenant, nil
}
