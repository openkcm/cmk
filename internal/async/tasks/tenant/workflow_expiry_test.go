package tasks_test

import (
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"

	tasks "github.com/openkcm/cmk/internal/async/tasks/tenant"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	wfMechanism "github.com/openkcm/cmk/internal/workflow"
	"github.com/openkcm/cmk/utils/ptr"
)

func setupWorkflowExpiry(t *testing.T) (*manager.WorkflowManager, repo.Repo, string) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{}, testutils.WithGenerateTenants(1))
	r := sql.NewRepository(db)

	cfg := &config.Config{}
	svcRegistry := testutils.NewTestPlugins()

	certManager := manager.NewCertificateManager(t.Context(), r, svcRegistry, cfg)
	tenantConfigManager := manager.NewTenantConfigManager(r, svcRegistry, nil)
	cmkAuditor := auditor.New(t.Context(), cfg)
	userManager := manager.NewUserManager(r, cmkAuditor)
	tagManager := manager.NewTagManager(r)
	keyConfigManager := manager.NewKeyConfigManager(r, certManager, userManager, tagManager, cmkAuditor, cfg)
	groupManager := manager.NewGroupManager(r, svcRegistry, userManager)

	clientsFactory, err := clients.NewFactory(cfg.Services)
	assert.NoError(t, err)
	systemManager := manager.NewSystemManager(
		t.Context(), r, clientsFactory, nil, svcRegistry, cfg, keyConfigManager, userManager)

	keyManager := manager.NewKeyManager(r, svcRegistry, tenantConfigManager, keyConfigManager,
		userManager, certManager, nil, cmkAuditor)
	wm := manager.NewWorkflowManager(r, svcRegistry, keyManager, keyConfigManager, systemManager,
		groupManager, userManager, nil, tenantConfigManager, cfg)

	return wm, r, tenants[0]
}

func TestWorkflowExpiresAction(t *testing.T) {
	t.Run("expires workflow in WaitApproval state", func(t *testing.T) {
		wm, r, tenantID := setupWorkflowExpiry(t)
		ctx := testutils.InjectClientDataIntoContext(
			testutils.CreateCtxWithTenant(tenantID),
			wfMechanism.SystemUserID,
			nil,
		)

		testutils.CreateTestEntities(ctx, t, r,
			testutils.NewWorkflow(func(w *model.Workflow) {
				w.State = wfMechanism.StateWaitApproval.String()
				w.ExpiryDate = ptr.PointTo(time.Now().AddDate(0, 0, -1))
			}),
		)

		processor := tasks.NewWorkflowExpiryProcessor(wm, r)
		err := processor.ProcessTask(ctx, asynq.NewTask(config.TypeWorkflowExpire, nil))
		assert.NoError(t, err)

		wfs, _, err := wm.GetWorkflows(ctx, manager.WorkflowFilter{})
		assert.NoError(t, err)
		assert.Len(t, wfs, 1)
		assert.Equal(t, wfMechanism.StateExpired.String(), wfs[0].State)
	})

	t.Run("expires workflow in WaitConfirmation state", func(t *testing.T) {
		wm, r, tenantID := setupWorkflowExpiry(t)
		ctx := testutils.InjectClientDataIntoContext(
			testutils.CreateCtxWithTenant(tenantID),
			wfMechanism.SystemUserID,
			nil,
		)

		testutils.CreateTestEntities(ctx, t, r,
			testutils.NewWorkflow(func(w *model.Workflow) {
				w.State = wfMechanism.StateWaitConfirmation.String()
				w.ExpiryDate = ptr.PointTo(time.Now().AddDate(0, 0, -1))
			}),
		)

		processor := tasks.NewWorkflowExpiryProcessor(wm, r)
		err := processor.ProcessTask(ctx, asynq.NewTask(config.TypeWorkflowExpire, nil))
		assert.NoError(t, err)

		wfs, _, err := wm.GetWorkflows(ctx, manager.WorkflowFilter{})
		assert.NoError(t, err)
		assert.Len(t, wfs, 1)
		assert.Equal(t, wfMechanism.StateExpired.String(), wfs[0].State)
	})

	t.Run("expires workflow in Executing state", func(t *testing.T) {
		wm, r, tenantID := setupWorkflowExpiry(t)
		ctx := testutils.InjectClientDataIntoContext(
			testutils.CreateCtxWithTenant(tenantID),
			wfMechanism.SystemUserID,
			nil,
		)

		testutils.CreateTestEntities(ctx, t, r,
			testutils.NewWorkflow(func(w *model.Workflow) {
				w.State = wfMechanism.StateExecuting.String()
				w.ExpiryDate = ptr.PointTo(time.Now().AddDate(0, 0, -1))
			}),
		)

		processor := tasks.NewWorkflowExpiryProcessor(wm, r)
		err := processor.ProcessTask(ctx, asynq.NewTask(config.TypeWorkflowExpire, nil))
		assert.NoError(t, err)

		wfs, _, err := wm.GetWorkflows(ctx, manager.WorkflowFilter{})
		assert.NoError(t, err)
		assert.Len(t, wfs, 1)
		assert.Equal(t, wfMechanism.StateExpired.String(), wfs[0].State)
	})

	t.Run("skips workflows with future expiry date", func(t *testing.T) {
		wm, r, tenantID := setupWorkflowExpiry(t)
		ctx := testutils.InjectClientDataIntoContext(
			testutils.CreateCtxWithTenant(tenantID),
			wfMechanism.SystemUserID,
			nil,
		)

		testutils.CreateTestEntities(ctx, t, r,
			testutils.NewWorkflow(func(w *model.Workflow) {
				w.State = wfMechanism.StateWaitApproval.String()
				w.ExpiryDate = ptr.PointTo(time.Now().AddDate(0, 0, 1))
			}),
		)

		processor := tasks.NewWorkflowExpiryProcessor(wm, r)
		err := processor.ProcessTask(ctx, asynq.NewTask(config.TypeWorkflowExpire, nil))
		assert.NoError(t, err)

		wfs, _, err := wm.GetWorkflows(ctx, manager.WorkflowFilter{})
		assert.NoError(t, err)
		assert.Len(t, wfs, 1)
		assert.Equal(t, wfMechanism.StateWaitApproval.String(), wfs[0].State)
	})

	t.Run("skips workflows with no expiry date", func(t *testing.T) {
		wm, r, tenantID := setupWorkflowExpiry(t)
		ctx := testutils.InjectClientDataIntoContext(
			testutils.CreateCtxWithTenant(tenantID),
			wfMechanism.SystemUserID,
			nil,
		)

		testutils.CreateTestEntities(ctx, t, r,
			testutils.NewWorkflow(func(w *model.Workflow) {
				w.State = wfMechanism.StateWaitApproval.String()
				w.ExpiryDate = nil
			}),
		)

		processor := tasks.NewWorkflowExpiryProcessor(wm, r)
		err := processor.ProcessTask(ctx, asynq.NewTask(config.TypeWorkflowExpire, nil))
		assert.NoError(t, err)

		wfs, _, err := wm.GetWorkflows(ctx, manager.WorkflowFilter{})
		assert.NoError(t, err)
		assert.Len(t, wfs, 1)
		assert.Equal(t, wfMechanism.StateWaitApproval.String(), wfs[0].State)
	})

	t.Run("skips terminal state workflows with past expiry date", func(t *testing.T) {
		terminalStates := []wfMechanism.State{
			wfMechanism.StateExpired,
			wfMechanism.StateRevoked,
			wfMechanism.StateRejected,
			wfMechanism.StateSuccessful,
			wfMechanism.StateFailed,
		}

		for _, state := range terminalStates {
			t.Run(state.String(), func(t *testing.T) {
				wm, r, tenantID := setupWorkflowExpiry(t)
				ctx := testutils.InjectClientDataIntoContext(
					testutils.CreateCtxWithTenant(tenantID),
					wfMechanism.SystemUserID,
					nil,
				)

				testutils.CreateTestEntities(ctx, t, r,
					testutils.NewWorkflow(func(w *model.Workflow) {
						w.State = state.String()
						w.ExpiryDate = ptr.PointTo(time.Now().AddDate(0, 0, -1))
					}),
				)

				processor := tasks.NewWorkflowExpiryProcessor(wm, r)
				err := processor.ProcessTask(ctx, asynq.NewTask(config.TypeWorkflowExpire, nil))
				assert.NoError(t, err)

				wfs, _, err := wm.GetWorkflows(ctx, manager.WorkflowFilter{})
				assert.NoError(t, err)
				assert.Len(t, wfs, 1)
				assert.Equal(t, state.String(), wfs[0].State, "terminal workflow state should not change")
			})
		}
	})

	t.Run("only expires past-due workflows when mixed with future ones", func(t *testing.T) {
		wm, r, tenantID := setupWorkflowExpiry(t)
		ctx := testutils.InjectClientDataIntoContext(
			testutils.CreateCtxWithTenant(tenantID),
			wfMechanism.SystemUserID,
			nil,
		)

		testutils.CreateTestEntities(ctx, t, r,
			testutils.NewWorkflow(func(w *model.Workflow) {
				w.State = wfMechanism.StateWaitApproval.String()
				w.ExpiryDate = ptr.PointTo(time.Now().AddDate(0, 0, -1))
			}),
			testutils.NewWorkflow(func(w *model.Workflow) {
				w.State = wfMechanism.StateWaitApproval.String()
				w.ExpiryDate = ptr.PointTo(time.Now().AddDate(0, 0, 1))
			}),
			testutils.NewWorkflow(func(w *model.Workflow) {
				w.State = wfMechanism.StateInitial.String()
				w.ExpiryDate = nil
			}),
		)

		processor := tasks.NewWorkflowExpiryProcessor(wm, r)
		err := processor.ProcessTask(ctx, asynq.NewTask(config.TypeWorkflowExpire, nil))
		assert.NoError(t, err)

		wfs, _, err := wm.GetWorkflows(ctx, manager.WorkflowFilter{})
		assert.NoError(t, err)
		assert.Len(t, wfs, 3)

		stateCounts := map[string]int{}
		for _, wf := range wfs {
			stateCounts[wf.State]++
		}
		assert.Equal(t, 1, stateCounts[wfMechanism.StateExpired.String()])
		assert.Equal(t, 1, stateCounts[wfMechanism.StateWaitApproval.String()])
		assert.Equal(t, 1, stateCounts[wfMechanism.StateInitial.String()])
	})

	t.Run("no-op when there are no workflows", func(t *testing.T) {
		wm, r, tenantID := setupWorkflowExpiry(t)
		ctx := testutils.InjectClientDataIntoContext(
			testutils.CreateCtxWithTenant(tenantID),
			wfMechanism.SystemUserID,
			nil,
		)

		processor := tasks.NewWorkflowExpiryProcessor(wm, r)
		err := processor.ProcessTask(ctx, asynq.NewTask(config.TypeWorkflowExpire, nil))
		assert.NoError(t, err)
	})

	t.Run("task type is correct", func(t *testing.T) {
		wm, r, _ := setupWorkflowExpiry(t)
		processor := tasks.NewWorkflowExpiryProcessor(wm, r)
		assert.Equal(t, config.TypeWorkflowExpire, processor.TaskType())
	})

	t.Run("fan out func is set", func(t *testing.T) {
		wm, r, _ := setupWorkflowExpiry(t)
		processor := tasks.NewWorkflowExpiryProcessor(wm, r)
		assert.NotNil(t, processor.FanOutFunc())
	})

	t.Run("tenant query is empty", func(t *testing.T) {
		wm, r, _ := setupWorkflowExpiry(t)
		processor := tasks.NewWorkflowExpiryProcessor(wm, r)
		assert.Equal(t, repo.NewQuery(), processor.TenantQuery())
	})
}
