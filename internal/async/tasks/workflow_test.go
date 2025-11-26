package tasks_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/async/tasks"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	wfMechanism "github.com/openkcm/cmk/internal/workflow"
	asyncUtils "github.com/openkcm/cmk/utils/async"
	contextUtils "github.com/openkcm/cmk/utils/context"
)

const (
	GoodWorkflowID  = "00000000-0000-0000-0000-000000000000"
	PanicWorkflowID = "11111111-1111-1111-1111-111111111111"
	BadWorkflowID   = "22222222-2222-2222-2222-222222222222"
)

var ErrBadWorkflow = errors.New("bad workflow error")

type WorkflowAssignMock struct{}

func (s *WorkflowAssignMock) GetCertificatesForRotation(
	_ context.Context,
) ([]*model.Certificate, int, error) {
	return []*model.Certificate{}, 0, nil
}

func (s *WorkflowAssignMock) AutoAssignApprovers(
	_ context.Context,
	workflowID uuid.UUID,
) (*model.Workflow, error) {
	switch workflowID.String() {
	case PanicWorkflowID:
		panic("simulated panic")
	case BadWorkflowID:
		return nil, ErrBadWorkflow
	}

	return &model.Workflow{ID: workflowID}, nil
}

func TestWorkflowAssignAction(t *testing.T) {
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
				workflow.ID = uuid.MustParse(GoodWorkflowID)
			},
		),
		testutils.NewWorkflow(
			func(workflow *model.Workflow) {
				workflow.ID = uuid.MustParse(BadWorkflowID)
			},
		),
		testutils.NewWorkflow(
			func(workflow *model.Workflow) {
				workflow.ID = uuid.MustParse(PanicWorkflowID)
			},
		),
	)

	assigner := tasks.NewWorkflowProcessor(&WorkflowAssignMock{}, r)

	t.Run("No task data", func(t *testing.T) {
		assert.Panics(t, func() {
			_ = assigner.ProcessTask(ctx, nil)
		}, "Expected panic on nil task data")
	})

	t.Run("Wrong task payload data", func(t *testing.T) {
		task := asynq.NewTask(config.TypeWorkflowAutoAssign, []byte("invalid-uuid"))
		err := assigner.ProcessTask(ctx, task)
		assert.ErrorIs(t, err, tasks.ErrRunningTask, "Failed to parse task payload")
	})

	t.Run("Wrong workflow ID in payload data", func(t *testing.T) {
		payload := asyncUtils.NewTaskPayload(ctx, []byte("1234"))
		data, err := payload.ToBytes()
		assert.NoError(t, err)

		task := asynq.NewTask(config.TypeWorkflowAutoAssign, data)
		err = assigner.ProcessTask(ctx, task)
		assert.ErrorIs(t, err, tasks.ErrRunningTask, "Failed to parse task payload data")
	})

	t.Run("Bad workflow causing error", func(t *testing.T) {
		payload := asyncUtils.NewTaskPayload(ctx, []byte(BadWorkflowID))
		data, err := payload.ToBytes()
		assert.NoError(t, err)

		task := asynq.NewTask(config.TypeWorkflowAutoAssign, data)
		err = assigner.ProcessTask(ctx, task)
		assert.ErrorIs(t, err, tasks.ErrRunningTask, ErrBadWorkflow)

		ck := repo.NewCompositeKey().Where(repo.IDField, BadWorkflowID)
		updatedWorkflow := &model.Workflow{}

		_, err = r.First(ctx, updatedWorkflow, *repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)))
		assert.NoError(t, err)

		assert.Equal(t, wfMechanism.StateFailed.String(), updatedWorkflow.State)
		assert.Equal(t, "bad workflow error", updatedWorkflow.FailureReason)
	})

	t.Run("Panic during processing", func(t *testing.T) {
		payload := asyncUtils.NewTaskPayload(ctx, []byte(PanicWorkflowID))
		data, err := payload.ToBytes()
		assert.NoError(t, err)

		task := asynq.NewTask(config.TypeWorkflowAutoAssign, data)

		assert.NotPanics(t, func() {
			err := assigner.ProcessTask(ctx, task)
			assert.NoError(t, err)
		}, "Expected panic to be recovered")

		ck := repo.NewCompositeKey().Where(repo.IDField, PanicWorkflowID)
		updatedWorkflow := &model.Workflow{}

		_, err = r.First(ctx, updatedWorkflow, *repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)))
		assert.NoError(t, err)

		assert.Equal(t, wfMechanism.StateFailed.String(), updatedWorkflow.State)
		assert.Equal(t, "internal error when assigning approvers", updatedWorkflow.FailureReason)
	})

	t.Run("Successful processing", func(t *testing.T) {
		payload := asyncUtils.NewTaskPayload(ctx, []byte(GoodWorkflowID))
		data, err := payload.ToBytes()
		assert.NoError(t, err)

		task := asynq.NewTask(config.TypeWorkflowAutoAssign, data)
		err = assigner.ProcessTask(ctx, task)
		assert.NoError(t, err, "Expected no error on good workflow")
	})
}
