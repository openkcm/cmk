package cmk

import (
	"context"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	"github.com/openkcm/cmk-core/internal/api/transform"
	"github.com/openkcm/cmk-core/internal/api/transform/tenant"
	"github.com/openkcm/cmk-core/internal/apierrors"
	"github.com/openkcm/cmk-core/internal/constants"
	"github.com/openkcm/cmk-core/internal/errs"
	cmkcontext "github.com/openkcm/cmk-core/utils/context"
	"github.com/openkcm/cmk-core/utils/ptr"
)

const (
	SysPath = "sys" // Since tenants list endpoint is not specific to any tenant, we use "sys" as a placeholder.
)

func (c *APIController) GetTenants(
	ctx context.Context,
	request cmkapi.GetTenantsRequestObject,
) (cmkapi.GetTenantsResponseObject, error) {
	tenantID, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrListTenants, err)
	}

	if tenantID != SysPath {
		return nil, errs.Wrap(apierrors.ErrListTenants, apierrors.ErrTenantIDInPath)
	}

	skip := ptr.GetIntOrDefault(request.Params.Skip, constants.DefaultSkip)
	top := ptr.GetIntOrDefault(request.Params.Top, constants.DefaultTop)

	tenants, total, err := c.Manager.Tenant.ListTenantInfo(ctx, skip, top)
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
