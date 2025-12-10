package tasks_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"

	"github.tools.sap/kms/cmk/internal/async/tasks"
	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/manager"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/repo"
	"github.tools.sap/kms/cmk/internal/repo/sql"
	"github.tools.sap/kms/cmk/internal/testutils"
	wfMechanism "github.tools.sap/kms/cmk/internal/workflow"
	contextUtils "github.tools.sap/kms/cmk/utils/context"
)

var ErrWorkflowNotFound = errors.New("workflow not found")

type WorkflowExpiryMock struct {
	repo repo.Repo
}

func (s *WorkflowExpiryMock) GetWorkflows(ctx context.Context,
	params repo.QueryMapper) ([]*model.Workflow, int, error) {
	workflows := []*model.Workflow{}

	count, err := s.repo.List(ctx, model.Workflow{}, &workflows, *params.GetQuery())
	if err != nil {
		return nil, 0, err
	}

	return workflows, count, nil
}

func (s *WorkflowExpiryMock) TransitionWorkflow(ctx context.Context, userID uuid.UUID,
	workflowID uuid.UUID, transition wfMechanism.Transition) (*model.Workflow, error) {
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

func TestWorkflowExpiresAction(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		Models: []driver.TenantTabler{
			model.Workflow{},
			model.WorkflowApprover{},
		},
	}, testutils.WithGenerateTenants(1))
	r := sql.NewRepository(db)

	tenantID := tenants[0]
	ctx := contextUtils.CreateTenantContext(t.Context(), tenantID)

	testutils.CreateTestEntities(ctx, t, r,
		testutils.NewWorkflow(
			func(workflow *model.Workflow) {
				workflow.ID = uuid.New()
				workflow.ExpiryDate = time.Now().AddDate(0, 0, 1)
			},
		),
		testutils.NewWorkflow(
			func(workflow *model.Workflow) {
				workflow.ID = uuid.New()
				workflow.ExpiryDate = time.Now().AddDate(0, 0, 1)
			},
		),
		testutils.NewWorkflow(
			func(workflow *model.Workflow) {
				workflow.ID = uuid.New()
				workflow.ExpiryDate = time.Now().AddDate(0, 0, -1)
			},
		),
	)

	expirer := &WorkflowExpiryMock{r}
	processor := tasks.NewWorkflowExpiryProcessor(expirer, r)

	t.Run("Sets Expired", func(t *testing.T) {
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
}
