package tasks_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"

	tasks "github.com/openkcm/cmk/internal/async/tasks/tenant"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	wfMechanism "github.com/openkcm/cmk/internal/workflow"
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

	t.Run("Sets Expired", func(t *testing.T) {
		expirer := &WorkflowExpiryMock{
			repo:      r,
			canExpire: true,
		}
		processor := tasks.NewWorkflowExpiryProcessor(expirer, r)

		task := asynq.NewTask(config.TypeWorkflowExpire, nil)
		err := processor.ProcessTask(ctx, task)
		assert.NoError(t, err)

		wfs, _, _ := expirer.GetWorkflows(ctx, manager.WorkflowFilter{})
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
		expirer := &WorkflowExpiryMock{repo: r, getErr: ErrMockGetWorkflows}
		processor := tasks.NewWorkflowExpiryProcessor(expirer, r)

		task := asynq.NewTask(config.TypeWorkflowExpire, nil)
		err := processor.ProcessTask(ctx, task)
		assert.Error(t, err)
	})

	t.Run("Transition fails", func(t *testing.T) {
		// Transition errors are logged and skipped per-workflow; ProcessTask still succeeds.
		expirer := &WorkflowExpiryMock{
			repo:          r,
			transitionErr: ErrMockTransition,
			canExpire:     true,
		}
		processor := tasks.NewWorkflowExpiryProcessor(expirer, r)

		task := asynq.NewTask(config.TypeWorkflowExpire, nil)
		err := processor.ProcessTask(ctx, task)
		assert.NoError(t, err)
	})

	t.Run("WorkflowCanExpire fails", func(t *testing.T) {
		// Per-workflow expiry check errors are logged and skipped; ProcessTask still succeeds.
		expirer := &WorkflowExpiryMock{
			repo:         r,
			canExpireErr: ErrMockCanExpire,
		}
		processor := tasks.NewWorkflowExpiryProcessor(expirer, r)

		task := asynq.NewTask(config.TypeWorkflowExpire, nil)
		err := processor.ProcessTask(ctx, task)
		assert.NoError(t, err)
	})

	t.Run("Skips workflow when CanExpire is false", func(t *testing.T) {
		// canExpire false — expired workflows should be skipped without error.
		expirer := &WorkflowExpiryMock{
			repo:      r,
			canExpire: false,
		}
		processor := tasks.NewWorkflowExpiryProcessor(expirer, r)

		task := asynq.NewTask(config.TypeWorkflowExpire, nil)
		err := processor.ProcessTask(ctx, task)
		assert.NoError(t, err)
	})
}
