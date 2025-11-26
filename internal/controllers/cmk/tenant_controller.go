package cmk

import (
	"context"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform"
	"github.com/openkcm/cmk/internal/api/transform/tenant"
	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

func (c *APIController) GetTenants(
	ctx context.Context,
	request cmkapi.GetTenantsRequestObject,
) (cmkapi.GetTenantsResponseObject, error) {
	skip := ptr.GetIntOrDefault(request.Params.Skip, constants.DefaultSkip)
	top := ptr.GetIntOrDefault(request.Params.Top, constants.DefaultTop)

	currentTenant, err := c.Manager.Tenant.GetTenant(ctx)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrListTenants, err)
	}

	tenants, total, err := c.Manager.Tenant.ListTenantInfo(ctx, ptr.PointTo(currentTenant.IssuerURL), skip, top)
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

	if ptr.GetSafeDeref(request.Params.Count) {
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
		return nil, errs.Wrap(apierrors.ErrGetTenantInfo, err)
	}

	iamIdentifiers, err := cmkcontext.ExtractClientDataGroups(ctx)
	if err != nil {
		return nil, apierrors.ErrTenantNotAllowed
	}

	accessible, err := c.Manager.Group.CheckTenantHasAnyIAMGroups(ctx, iamIdentifiers)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrGetTenantInfo, err)
	}

	if !accessible {
		return nil, apierrors.ErrTenantNotAllowed
	}

	tenantAPI, err := tenant.ToAPI(*currentTenant)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrTransformTenants, err)
	}

	return cmkapi.GetTenantInfo200JSONResponse(*tenantAPI), nil
}
