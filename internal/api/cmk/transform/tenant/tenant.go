package tenant

import (
	"strings"

	cmkapi "github.com/openkcm/cmk/internal/api/cmk/generated"
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
		Id:   &tenant.ID,
		Role: &role,
		Name: tenant.Name,
	}

	return apiTenant, nil
}
