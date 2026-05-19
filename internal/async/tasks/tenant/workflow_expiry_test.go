package tasks_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"

	tasks "github.com/openkcm/cmk/internal/async/tasks/tenant"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/authz"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	wfMechanism "github.com/openkcm/cmk/internal/workflow"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

var (
	ErrWorkflowNotFound = errors.New("workflow not found")
	ErrMockGetWorkflows = errors.New("forced GetWorkflows error")
	ErrMockTransition   = errors.New("forced TransitionWorkflow error")
	ErrMockCanExpire    = errors.New("forced WorkflowCanExpire error")
)

type WorkflowExpiryMock struct {
	repo          repo.Repo
	getErr        error
	transitionErr error
	canExpireErr  error
	canExpire     bool
	authzLoader   *authz_loader.AuthzLoader[authz.RepoResourceTypeName,
		authz.RepoAction]
}

func (s *WorkflowExpiryMock) GetWorkflows(ctx context.Context,
	params repo.QueryMapper,
) ([]*model.Workflow, int, error) {
	if s.getErr != nil {
		return nil, 0, s.getErr
	}

	query := params.GetQuery(ctx)
	return repo.ListAndCount(ctx, s.repo, params.GetPagination(), model.Workflow{}, query)
}

func (s *WorkflowExpiryMock) ExpireWorkflow(ctx context.Context,
	workflowID uuid.UUID,
) (*model.Workflow, error) {
	if s.authzLoader != nil {
		// We test for unauthz in this case
		err := s.authzLoader.LoadAllowList(ctx)
		if err != nil {
			return nil, err
		}

		_, err = authz.CheckAuthz(ctx, s.authzLoader.AuthzHandler,
			authz.RepoResourceTypeCertificate, authz.RepoActionDelete)
		return nil, err
	}

	if s.transitionErr != nil {
		return nil, s.transitionErr
	}

	workflows, _, _ := s.GetWorkflows(ctx, manager.WorkflowFilter{})
	for _, wf := range workflows {
		if wf.ID == workflowID {
			wf.State = string(wfMechanism.StateExpired)

			_, err := s.repo.Patch(ctx, wf, *repo.NewQuery())
			if err != nil {
				return nil, err
			}

			return wf, nil
		}
	}
	return nil, ErrWorkflowNotFound
}

func (s *WorkflowExpiryMock) WorkflowCanExpire(
	_ context.Context,
	_ *model.Workflow,
) (bool, error) {
	if s.canExpireErr != nil {
		return false, s.canExpireErr
	}
	return s.canExpire, nil
}

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
		t.Context(), r, nil, clientsFactory, nil, svcRegistry, cfg, keyConfigManager, userManager)

	keyManager := manager.NewKeyManager(r, svcRegistry, tenantConfigManager, keyConfigManager,
		userManager, certManager, nil, cmkAuditor)
	wm := manager.NewWorkflowManager(r, svcRegistry, keyManager, keyConfigManager, systemManager,
		groupManager, userManager, nil, tenantConfigManager, cfg)

	return wm, r, tenants[0]
}

func TestWorkflowExpiresAction(t *testing.T) {
	t.Run("expires workflow in WaitApproval state", func(t *testing.T) {
		wm, r, tenantID := setupWorkflowExpiry(t)
		ctx, err := cmkcontext.InjectInternalUserData(
			cmkcontext.CreateTenantContext(t.Context(), tenantID),
			constants.InternalTaskWorkflowExpirationRole)
		assert.NoError(t, err)

		testutils.CreateTestEntities(ctx, t, r,
			testutils.NewWorkflow(func(w *model.Workflow) {
				w.State = wfMechanism.StateWaitApproval.String()
				w.ExpiryDate = ptr.PointTo(time.Now().AddDate(0, 0, -1))
			}),
		)

		processor := tasks.NewWorkflowExpiryProcessor(wm, r)
		err = processor.ProcessTask(ctx, asynq.NewTask(config.TypeWorkflowExpire, nil))
		assert.NoError(t, err)

		wfs, _, err := wm.GetWorkflows(ctx, manager.WorkflowFilter{})
		assert.NoError(t, err)
		assert.Len(t, wfs, 1)
		assert.Equal(t, wfMechanism.StateExpired.String(), wfs[0].State)
	})

	t.Run("expires workflow in WaitConfirmation state", func(t *testing.T) {
		wm, r, tenantID := setupWorkflowExpiry(t)
		ctx, err := cmkcontext.InjectInternalUserData(
			cmkcontext.CreateTenantContext(t.Context(), tenantID),
			constants.InternalTaskWorkflowExpirationRole)
		assert.NoError(t, err)

		testutils.CreateTestEntities(ctx, t, r,
			testutils.NewWorkflow(func(w *model.Workflow) {
				w.State = wfMechanism.StateWaitConfirmation.String()
				w.ExpiryDate = ptr.PointTo(time.Now().AddDate(0, 0, -1))
			}),
		)

		processor := tasks.NewWorkflowExpiryProcessor(wm, r)
		err = processor.ProcessTask(ctx, asynq.NewTask(config.TypeWorkflowExpire, nil))
		assert.NoError(t, err)

		wfs, _, err := wm.GetWorkflows(ctx, manager.WorkflowFilter{})
		assert.NoError(t, err)
		assert.Len(t, wfs, 1)
		assert.Equal(t, wfMechanism.StateExpired.String(), wfs[0].State)
	})

	t.Run("expires workflow in Executing state", func(t *testing.T) {
		wm, r, tenantID := setupWorkflowExpiry(t)
		ctx, err := cmkcontext.InjectInternalUserData(
			cmkcontext.CreateTenantContext(t.Context(), tenantID),
			constants.InternalTaskWorkflowExpirationRole)
		assert.NoError(t, err)

		testutils.CreateTestEntities(ctx, t, r,
			testutils.NewWorkflow(func(w *model.Workflow) {
				w.State = wfMechanism.StateExecuting.String()
				w.ExpiryDate = ptr.PointTo(time.Now().AddDate(0, 0, -1))
			}),
		)

		processor := tasks.NewWorkflowExpiryProcessor(wm, r)
		err = processor.ProcessTask(ctx, asynq.NewTask(config.TypeWorkflowExpire, nil))
		assert.NoError(t, err)

		wfs, _, err := wm.GetWorkflows(ctx, manager.WorkflowFilter{})
		assert.NoError(t, err)
		assert.Len(t, wfs, 1)
		assert.Equal(t, wfMechanism.StateExpired.String(), wfs[0].State)
	})

	t.Run("skips workflows with future expiry date", func(t *testing.T) {
		wm, r, tenantID := setupWorkflowExpiry(t)
		ctx, err := cmkcontext.InjectInternalUserData(
			cmkcontext.CreateTenantContext(t.Context(), tenantID),
			constants.InternalTaskWorkflowExpirationRole)
		assert.NoError(t, err)

		testutils.CreateTestEntities(ctx, t, r,
			testutils.NewWorkflow(func(w *model.Workflow) {
				w.State = wfMechanism.StateWaitApproval.String()
				w.ExpiryDate = ptr.PointTo(time.Now().AddDate(0, 0, 1))
			}),
		)

		processor := tasks.NewWorkflowExpiryProcessor(wm, r)
		err = processor.ProcessTask(ctx, asynq.NewTask(config.TypeWorkflowExpire, nil))
		assert.NoError(t, err)

		wfs, _, err := wm.GetWorkflows(ctx, manager.WorkflowFilter{})
		assert.NoError(t, err)
		assert.Len(t, wfs, 1)
		assert.Equal(t, wfMechanism.StateWaitApproval.String(), wfs[0].State)
	})

	t.Run("skips workflows with no expiry date", func(t *testing.T) {
		wm, r, tenantID := setupWorkflowExpiry(t)
		ctx, err := cmkcontext.InjectInternalUserData(
			cmkcontext.CreateTenantContext(t.Context(), tenantID),
			constants.InternalTaskWorkflowExpirationRole)
		assert.NoError(t, err)

		testutils.CreateTestEntities(ctx, t, r,
			testutils.NewWorkflow(func(w *model.Workflow) {
				w.State = wfMechanism.StateWaitApproval.String()
				w.ExpiryDate = nil
			}),
		)

		processor := tasks.NewWorkflowExpiryProcessor(wm, r)
		err = processor.ProcessTask(ctx, asynq.NewTask(config.TypeWorkflowExpire, nil))
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
				ctx, err := cmkcontext.InjectInternalUserData(
					cmkcontext.CreateTenantContext(t.Context(), tenantID),
					constants.InternalTaskWorkflowExpirationRole)
				assert.NoError(t, err)

				testutils.CreateTestEntities(ctx, t, r,
					testutils.NewWorkflow(func(w *model.Workflow) {
						w.State = state.String()
						w.ExpiryDate = ptr.PointTo(time.Now().AddDate(0, 0, -1))
					}),
				)

				processor := tasks.NewWorkflowExpiryProcessor(wm, r)
				err = processor.ProcessTask(ctx, asynq.NewTask(config.TypeWorkflowExpire, nil))
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
		ctx, err := cmkcontext.InjectInternalUserData(
			cmkcontext.CreateTenantContext(t.Context(), tenantID),
			constants.InternalTaskWorkflowExpirationRole)
		assert.NoError(t, err)

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
		err = processor.ProcessTask(ctx, asynq.NewTask(config.TypeWorkflowExpire, nil))
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
		ctx, err := cmkcontext.InjectInternalUserData(
			cmkcontext.CreateTenantContext(t.Context(), tenantID),
			constants.InternalTaskWorkflowExpirationRole)
		assert.NoError(t, err)

		processor := tasks.NewWorkflowExpiryProcessor(wm, r)
		err = processor.ProcessTask(ctx, asynq.NewTask(config.TypeWorkflowExpire, nil))
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

	t.Run("Should log on unauthorized processing", func(t *testing.T) {
		db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{}, testutils.WithGenerateTenants(1))
		r := sql.NewRepository(db)

		tenantID := tenants[0]
		ctx, err := cmkcontext.InjectInternalUserData(
			cmkcontext.CreateTenantContext(t.Context(), tenantID),
			constants.InternalTaskWorkflowExpirationRole)
		assert.NoError(t, err)

		testutils.CreateTestEntities(ctx, t, r,
			testutils.NewWorkflow(func(w *model.Workflow) {
				w.State = wfMechanism.StateWaitApproval.String()
				w.ExpiryDate = ptr.PointTo(time.Now().AddDate(0, 0, -1))
			}),
		)

		authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(), r, &config.Config{})
		authzRepo := authz_repo.NewAuthzRepo(r, authzRepoLoader)

		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		expirer := &WorkflowExpiryMock{
			repo:        authzRepo,
			canExpire:   true,
			authzLoader: authzRepoLoader,
		}
		processor := tasks.NewWorkflowExpiryProcessor(expirer, authzRepo)

		err = processor.ProcessTask(ctx, asynq.NewTask(config.TypeWorkflowExpire, nil))
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "Failed to expire workflow")
		assert.Contains(t, buf.String(), "authorization decision error")
	})
}
