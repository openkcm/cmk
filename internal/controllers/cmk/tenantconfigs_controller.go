package cmk

import (
	"context"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/api/transform/tenantconfigs"
	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/errs"
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

func (c *APIController) GetTenantWorkflowConfiguration(
	ctx context.Context,
	_ cmkapi.GetTenantWorkflowConfigurationRequestObject,
) (cmkapi.GetTenantWorkflowConfigurationResponseObject, error) {
	workflowConfig, err := c.Manager.TenantConfigs.GetWorkflowConfig(ctx)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrGetWorkflowConfig, err)
	}

	apiConfig := tenantconfigs.WorkflowConfigToAPI(workflowConfig)
	return cmkapi.GetTenantWorkflowConfiguration200JSONResponse(*apiConfig), nil
}

func (c *APIController) UpdateTenantWorkflowConfiguration(
	ctx context.Context,
	request cmkapi.UpdateTenantWorkflowConfigurationRequestObject,
) (cmkapi.UpdateTenantWorkflowConfigurationResponseObject, error) {
	savedConfig, err := c.Manager.TenantConfigs.UpdateWorkflowConfig(ctx, request.Body)
	if err != nil {
		return nil, errs.Wrap(apierrors.ErrSetWorkflowConfig, err)
	}

	apiConfig := tenantconfigs.WorkflowConfigToAPI(savedConfig)
	return cmkapi.UpdateTenantWorkflowConfiguration200JSONResponse(*apiConfig), nil
}
