package tasks_test

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"

	tasks "github.com/openkcm/cmk/internal/async/tasks/tenant"
	"github.com/openkcm/cmk/internal/authz"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	wfMechanism "github.com/openkcm/cmk/internal/workflow"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	contextUtils "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

var (
	ErrWorkflowNotFound = errors.New("workflow not found")
	ErrTransitionFailed = errors.New("workflow transition failed")
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

func (s *WorkflowExpiryMock) TransitionWorkflow(ctx context.Context,
	workflowID uuid.UUID,
	transition wfMechanism.Transition,
) (*model.Workflow, error) {
	if s.authzLoader != nil {
		//We test for unauthz in this case
		s.authzLoader.LoadAllowList(ctx)
		_, err := authz.CheckAuthz(ctx, s.authzLoader.AuthzHandler,
			authz.RepoResourceTypeCertificate, authz.RepoActionDelete)
		return nil, err

	}

	if s.transitionErr != nil {
		return nil, s.transitionErr
	}

	workflows, _, _ := s.GetWorkflows(ctx, manager.WorkflowFilter{})
	for _, wf := range workflows {
		if wf.ID == workflowID {
			if transition != wfMechanism.TransitionExpire {
				return nil, ErrTransitionFailed
			}

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

func TestWorkflowExpiresAction(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{}, testutils.WithGenerateTenants(1))
	r := sql.NewRepository(db)

	tenantID := tenants[0]
	ctx := contextUtils.CreateTenantContext(t.Context(), tenantID)

	// Thisn role isn't need for the task processing but to GetWorkFlows in the test code
	ctx, err := cmkcontext.InjectInternalClientData(ctx,
		constants.InternalTaskWorkflowExpirationRole)
	assert.NoError(t, err)

	// Two workflows not yet expired (future expiry date), one past expiry date in an expirable state.
	testutils.CreateTestEntities(ctx, t, r,
		testutils.NewWorkflow(
			func(workflow *model.Workflow) {
				workflow.ID = uuid.New()
				workflow.ExpiryDate = ptr.PointTo(time.Now().AddDate(0, 0, 1))
			},
		),
		testutils.NewWorkflow(
			func(workflow *model.Workflow) {
				workflow.ID = uuid.New()
				workflow.ExpiryDate = ptr.PointTo(time.Now().AddDate(0, 0, 1))
			},
		),
		testutils.NewWorkflow(
			func(workflow *model.Workflow) {
				workflow.ID = uuid.New()
				workflow.State = wfMechanism.StateWaitApproval.String()
				workflow.ExpiryDate = ptr.PointTo(time.Now().AddDate(0, 0, -1))
			},
		),
	)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(),
		r, &config.Config{})

	authzRepo := authz_repo.NewAuthzRepo(r, authzRepoLoader)

	task := asynq.NewTask(config.TypeWorkflowExpire, nil)

	t.Run("Sets Expired", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		expirer := &WorkflowExpiryMock{
			repo:      authzRepo,
			canExpire: true,
		}
		processor := tasks.NewWorkflowExpiryProcessor(expirer, authzRepo)

		err := processor.ProcessTask(ctx, task)
		assert.NoError(t, err)

		wfs, _, err := expirer.GetWorkflows(ctx, manager.WorkflowFilter{})
		assert.NoError(t, err)
		assert.NotContains(t, strings.ToLower(buf.String()), "error")
		assert.Len(t, wfs, 3)

		initStates := 0
		expStates := 0
		for _, wf := range wfs {
			if wf.State == string(wfMechanism.StateInitial) {
				initStates++
			} else if wf.State == string(wfMechanism.StateExpired) {
				expStates++
			}
		}
		assert.Equal(t, 2, initStates)
		assert.Equal(t, 1, expStates)
	})

	t.Run("GetWorkflows fails", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		expirer := &WorkflowExpiryMock{repo: authzRepo, getErr: ErrMockGetWorkflows}
		processor := tasks.NewWorkflowExpiryProcessor(expirer, authzRepo)

		err := processor.ProcessTask(ctx, task)
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "Error during workflow expiry batch processing")
		assert.Contains(t, buf.String(), "forced GetWorkflows error")
	})

	t.Run("Transition fails", func(t *testing.T) {
		// Transition errors are logged and skipped per-workflow; ProcessTask still succeeds.
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		expirer := &WorkflowExpiryMock{
			repo:          authzRepo,
			transitionErr: ErrMockTransition,
			canExpire:     true,
		}
		processor := tasks.NewWorkflowExpiryProcessor(expirer, r)

		err := processor.ProcessTask(ctx, task)
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "forced TransitionWorkflow error")
	})

	t.Run("WorkflowCanExpire fails", func(t *testing.T) {
		// Per-workflow expiry check errors are logged and skipped; ProcessTask still succeeds.
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		expirer := &WorkflowExpiryMock{
			repo:         r,
			canExpireErr: ErrMockCanExpire,
		}
		processor := tasks.NewWorkflowExpiryProcessor(expirer, r)

		err := processor.ProcessTask(ctx, task)
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "forced WorkflowCanExpire error")
	})

	t.Run("Skips workflow when CanExpire is false", func(t *testing.T) {
		// canExpire false — expired workflows should be skipped without error.
		expirer := &WorkflowExpiryMock{
			repo:      r,
			canExpire: false,
		}
		processor := tasks.NewWorkflowExpiryProcessor(expirer, r)

		err := processor.ProcessTask(ctx, task)
		assert.NoError(t, err)
	})

	t.Run("Should log on unauthorized processing", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		expirer := &WorkflowExpiryMock{
			repo:        authzRepo,
			canExpire:   true,
			authzLoader: authzRepoLoader,
		}
		processor := tasks.NewWorkflowExpiryProcessor(expirer, authzRepo)

		err := processor.ProcessTask(ctx, task)
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "Failed to expire workflow")
		assert.Contains(t, buf.String(), "authorization decision error")
	})
}
