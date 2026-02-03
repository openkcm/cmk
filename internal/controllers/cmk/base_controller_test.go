package cmk_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/repo"
)

const (
	totalRecordCount = 21
	providerTest     = "TEST"
)

func disableWorkflow(t *testing.T, ctx context.Context, r repo.Repo) {
	t.Helper()

	tenantConfigManager := manager.NewTenantConfigManager(r, nil)

	workflowConfig, err := tenantConfigManager.GetWorkflowConfig(ctx)
	require.NoError(t, err)

	workflowConfig.Enabled = false

	_, err = tenantConfigManager.SetWorkflowConfig(ctx, workflowConfig)
	require.NoError(t, err)
}
