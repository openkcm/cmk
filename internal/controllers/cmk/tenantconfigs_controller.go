package cmk

import (
	"context"

	"github.tools.sap/kms/cmk/internal/api/cmkapi"
	"github.tools.sap/kms/cmk/internal/api/transform/tenantconfigs"
	"github.tools.sap/kms/cmk/internal/apierrors"
	"github.tools.sap/kms/cmk/internal/errs"
)

func (c *APIController) GetTenantKeystores(
	_ context.Context,
	_ cmkapi.GetTenantKeystoresRequestObject,
) (cmkapi.GetTenantKeystoresResponseObject, error) {
	dbKeystore, err := c.Manager.TenantConfigs.GetTenantsKeystores()
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrGetDefaultKeystore, err)
	}

	apiDefaultKeystore, err := tenantconfigs.ToAPI(dbKeystore)
	if err != nil {
		return nil, err
	}

	return cmkapi.GetTenantKeystores200JSONResponse(*apiDefaultKeystore), nil
}
