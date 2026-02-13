package cmk

import (
	"context"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform"
	"github.com/openkcm/cmk/internal/api/transform/tenant"
	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/utils/ptr"
)

func (c *APIController) GetTenants(
	ctx context.Context,
	request cmkapi.GetTenantsRequestObject,
) (cmkapi.GetTenantsResponseObject, error) {
	pagination := repo.Pagination{
		Skip:  ptr.GetIntOrDefault(request.Params.Skip, constants.DefaultSkip),
		Top:   ptr.GetIntOrDefault(request.Params.Top, constants.DefaultTop),
		Count: ptr.GetSafeDeref(request.Params.Count),
	}

	currentTenant, err := c.Manager.Tenant.GetTenant(ctx)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrListTenants, err)
	}

	tenants, total, err := c.Manager.Tenant.ListTenantInfo(ctx, ptr.PointTo(currentTenant.IssuerURL), pagination)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrListTenants, err)
	}

	values, err := transform.ToList(
		tenants,
		tenant.ToAPI,
	)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrTransformTenants, err)
	}

	response := cmkapi.TenantList{
		Value: values,
	}

	if pagination.Count {
		response.Count = ptr.PointTo(total)
	}

	return cmkapi.GetTenants200JSONResponse(response), nil
}

func (c *APIController) GetTenantInfo(
	ctx context.Context,
	_ cmkapi.GetTenantInfoRequestObject,
) (cmkapi.GetTenantInfoResponseObject, error) {
	currentTenant, err := c.Manager.Tenant.GetTenant(ctx)
	if err != nil {
		return nil, err
	}

	tenantAPI, err := tenant.ToAPI(*currentTenant)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrTransformTenants, err)
	}

	return cmkapi.GetTenantInfo200JSONResponse(*tenantAPI), nil
}
