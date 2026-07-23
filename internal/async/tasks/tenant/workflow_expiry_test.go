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
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
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
	authzLoader   *authz_loader.AuthzLoader[authz.RepoResourceType,
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
		err := s.authzLoader.LoadTenantAllowedActions(ctx)
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
			wf.State = model.WorkflowStateExpired

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

	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{}, testutils.WithGenerateTenants(1))
	r := sql.NewRepository(db)

	cfg := &config.Config{
		Database: dbCfg,
	}
	svcRegistry := testutils.NewTestPlugins()

	eventFactory, err := eventprocessor.NewEventFactory(t.Context(), cfg, r)
	assert.NoError(t, err)

	certManager := manager.NewCertificateManager(t.Context(), r, svcRegistry, cfg)
	tenantConfigManager := manager.NewTenantConfigManager(r, svcRegistry, nil, certManager)
	cmkAuditor := auditor.New(t.Context(), cfg)
	userManager := manager.NewUserManager(r, cmkAuditor)
	tagManager := manager.NewTagManager(r)
	keyConfigManager := manager.NewKeyConfigManager(r, certManager, userManager, tagManager, cmkAuditor, eventFactory, cfg)
	groupManager := manager.NewGroupManager(r, svcRegistry, userManager)

	clientsFactory, err := clients.NewFactory(cfg.Services)
	assert.NoError(t, err)
	systemManager := manager.NewSystemManager(
		t.Context(), r, nil, clientsFactory, nil, svcRegistry, cfg, keyConfigManager, userManager,
	)

	keyManager := manager.NewKeyManager(r, svcRegistry, tenantConfigManager, keyConfigManager,
		userManager, certManager, nil, cmkAuditor)
	wm := manager.NewWorkflowManager(r, svcRegistry, keyManager, keyConfigManager, systemManager,
		groupManager, userManager, nil, tenantConfigManager, cfg)

	return wm, r, tenants[0]
}

func newExpiryContext(t *testing.T, tenantID string) context.Context {
	t.Helper()
	ctx, err := cmkcontext.InjectInternalUserData(
		cmkcontext.CreateTenantContext(t.Context(), tenantID),
		constants.InternalTaskWorkflowExpirationRole,
	)
	assert.NoError(t, err)
	return ctx
}

func runExpiryProcessor(t *testing.T, wm *manager.WorkflowManager, r repo.Repo, ctx context.Context) error {
	t.Helper()
	processor := tasks.NewWorkflowExpiryProcessor(wm, r)
	return processor.ProcessTask(ctx, asynq.NewTask(config.TypeWorkflowExpire, nil))
}

func TestWorkflowExpiresAction(t *testing.T) {
	// States that should be transitioned to Expired when past due.
	for _, state := range []model.WorkflowState{
		model.WorkflowStateWaitApproval,
		model.WorkflowStateWaitConfirmation,
		model.WorkflowStateExecuting,
	} {
		t.Run(state.String(), func(t *testing.T) {
			wm, r, tenantID := setupWorkflowExpiry(t)
			ctx := newExpiryContext(t, tenantID)

			testutils.CreateTestEntities(
				ctx, t, r,
				testutils.NewWorkflow(func(w *model.Workflow) {
					w.State = state
					w.ExpiryDate = ptr.PointTo(time.Now().AddDate(0, 0, -1))
				}),
			)

			assert.NoError(t, runExpiryProcessor(t, wm, r, ctx))

			wfs, _, err := wm.GetWorkflows(ctx, manager.WorkflowFilter{})
			assert.NoError(t, err)
			assert.Len(t, wfs, 1)
			assert.Equal(t, model.WorkflowStateExpired, wfs[0].State)
		})
	}

	t.Run("future expiry skipped", func(t *testing.T) {
		wm, r, tenantID := setupWorkflowExpiry(t)
		ctx := newExpiryContext(t, tenantID)

		testutils.CreateTestEntities(
			ctx, t, r,
			testutils.NewWorkflow(func(w *model.Workflow) {
				w.State = model.WorkflowStateWaitApproval
				w.ExpiryDate = ptr.PointTo(time.Now().AddDate(0, 0, 1))
			}),
		)

		assert.NoError(t, runExpiryProcessor(t, wm, r, ctx))

		wfs, _, err := wm.GetWorkflows(ctx, manager.WorkflowFilter{})
		assert.NoError(t, err)
		assert.Len(t, wfs, 1)
		assert.Equal(t, model.WorkflowStateWaitApproval, wfs[0].State)
	})

	t.Run("no expiry skipped", func(t *testing.T) {
		wm, r, tenantID := setupWorkflowExpiry(t)
		ctx := newExpiryContext(t, tenantID)

		testutils.CreateTestEntities(
			ctx, t, r,
			testutils.NewWorkflow(func(w *model.Workflow) {
				w.State = model.WorkflowStateWaitApproval
				w.ExpiryDate = nil
			}),
		)

		assert.NoError(t, runExpiryProcessor(t, wm, r, ctx))

		wfs, _, err := wm.GetWorkflows(ctx, manager.WorkflowFilter{})
		assert.NoError(t, err)
		assert.Len(t, wfs, 1)
		assert.Equal(t, model.WorkflowStateWaitApproval, wfs[0].State)
	})

	t.Run("terminal states skipped", func(t *testing.T) {
		for _, state := range []model.WorkflowState{
			model.WorkflowStateExpired,
			model.WorkflowStateRevoked,
			model.WorkflowStateRejected,
			model.WorkflowStateSuccessful,
			model.WorkflowStateFailed,
		} {
			t.Run(state.String(), func(t *testing.T) {
				wm, r, tenantID := setupWorkflowExpiry(t)
				ctx := newExpiryContext(t, tenantID)

				testutils.CreateTestEntities(
					ctx, t, r,
					testutils.NewWorkflow(func(w *model.Workflow) {
						w.State = state
						w.ExpiryDate = ptr.PointTo(time.Now().AddDate(0, 0, -1))
					}),
				)

				assert.NoError(t, runExpiryProcessor(t, wm, r, ctx))

				wfs, _, err := wm.GetWorkflows(ctx, manager.WorkflowFilter{})
				assert.NoError(t, err)
				assert.Len(t, wfs, 1)
				assert.Equal(t, state, wfs[0].State, "terminal workflow state should not change")
			})
		}
	})

	t.Run("mixed expiry dates", func(t *testing.T) {
		wm, r, tenantID := setupWorkflowExpiry(t)
		ctx := newExpiryContext(t, tenantID)

		testutils.CreateTestEntities(
			ctx, t, r,
			testutils.NewWorkflow(func(w *model.Workflow) {
				w.State = model.WorkflowStateWaitApproval
				w.ExpiryDate = ptr.PointTo(time.Now().AddDate(0, 0, -1))
			}),
			testutils.NewWorkflow(func(w *model.Workflow) {
				w.State = model.WorkflowStateWaitApproval
				w.ExpiryDate = ptr.PointTo(time.Now().AddDate(0, 0, 1))
			}),
			testutils.NewWorkflow(func(w *model.Workflow) {
				w.State = model.WorkflowStateInitial
				w.ExpiryDate = nil
			}),
		)

		assert.NoError(t, runExpiryProcessor(t, wm, r, ctx))

		wfs, _, err := wm.GetWorkflows(ctx, manager.WorkflowFilter{})
		assert.NoError(t, err)
		assert.Len(t, wfs, 3)

		stateCounts := map[model.WorkflowState]int{}
		for _, wf := range wfs {
			stateCounts[wf.State]++
		}
		assert.Equal(t, 1, stateCounts[model.WorkflowStateExpired])
		assert.Equal(t, 1, stateCounts[model.WorkflowStateWaitApproval])
		assert.Equal(t, 1, stateCounts[model.WorkflowStateInitial])
	})

	t.Run("no workflows", func(t *testing.T) {
		wm, r, tenantID := setupWorkflowExpiry(t)
		ctx := newExpiryContext(t, tenantID)
		assert.NoError(t, runExpiryProcessor(t, wm, r, ctx))
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

	t.Run("logs on unauthorized", func(t *testing.T) {
		db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{}, testutils.WithGenerateTenants(1))
		r := sql.NewRepository(db)

		ctx := newExpiryContext(t, tenants[0])

		testutils.CreateTestEntities(
			ctx, t, r,
			testutils.NewWorkflow(func(w *model.Workflow) {
				w.State = model.WorkflowStateWaitApproval
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

		err := processor.ProcessTask(ctx, asynq.NewTask(config.TypeWorkflowExpire, nil))
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "Failed to expire workflow")
		assert.Contains(t, buf.String(), "authorization decision error")
	})
}
