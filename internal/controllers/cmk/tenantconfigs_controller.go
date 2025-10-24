package cmk

import (
	"context"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform/tenantconfigs"
	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/errs"
)

func (c *APIController) GetTenantsKeystores(
	_ context.Context,
	_ cmkapi.GetTenantsKeystoresRequestObject,
) (cmkapi.GetTenantsKeystoresResponseObject, error) {
	dbKeystore, err := c.Manager.TenantConfigs.GetTenantsKeystores()
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrGetDefaultKeystore, err)
	}

	apiDefaultKeystore := tenantconfigs.ToAPI(dbKeystore)

	return cmkapi.GetTenantsKeystores200JSONResponse(*apiDefaultKeystore), nil
}
