package tasks_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/async/tasks"
	"github.com/openkcm/cmk/internal/authz"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	asyncUtils "github.com/openkcm/cmk/utils/async"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

const (
	GoodWorkflowID                = "00000000-0000-0000-0000-000000000000"
	PanicWorkflowID               = "11111111-1111-1111-1111-111111111111"
	BadWorkflowID                 = "22222222-2222-2222-2222-222222222222"
	SystemUnderWorkflowWorkflowID = "33333333-3333-3333-3333-333333333333"
)

var ErrBadWorkflow = errors.New("bad workflow error")

var allowedWorkflowTestActions = []authz.RepoAction{
	authz.RepoActionFirst,
	authz.RepoActionUpdate,
}

var allowedWorkflowApproversTestActions = []authz.RepoAction{
	authz.RepoActionCreate,
	authz.RepoActionDelete,
}

type WorkflowAssignMock struct {
	authzLoader *authz_loader.AuthzLoader[authz.RepoResourceType,
		authz.RepoAction]
	repo repo.Repo
}

func (s *WorkflowAssignMock) AutoAssignApprovers(
	ctx context.Context,
	workflowID uuid.UUID,
) (*model.Workflow, error) {
	// We only test a subset of the permissions
	for _, testAction := range allowedWorkflowTestActions {
		isAllowed, err := authz.CheckAuthz(ctx, s.authzLoader.AuthzHandler,
			authz.RepoResourceTypeWorkflow, testAction)
		if err != nil {
			return nil, err
		}
		if !isAllowed {
			return nil, authz.ErrAuthzDecision
		}
	}
	for _, testAction := range allowedWorkflowApproversTestActions {
		isAllowed, err := authz.CheckAuthz(ctx, s.authzLoader.AuthzHandler,
			authz.RepoResourceTypeWorkflowApprover, testAction)
		if err != nil {
			return nil, err
		}
		if !isAllowed {
			return nil, authz.ErrAuthzDecision
		}
	}

	switch workflowID.String() {
	case PanicWorkflowID:
		panic("simulated panic")
	case BadWorkflowID:
		return nil, ErrBadWorkflow
	case SystemUnderWorkflowWorkflowID:
		return nil, ErrBadWorkflow
	}

	return &model.Workflow{ID: workflowID}, nil
}

func (s *WorkflowAssignMock) HandleTerminalWorkflow(ctx context.Context, workflow *model.Workflow) error {
	switch workflow.ArtifactType {
	case model.WorkflowArtifactTypeSystem:
		system := &model.System{
			ID:            workflow.ArtifactID,
			UnderWorkflow: false,
		}
		_, err := s.repo.Patch(ctx, system, *repo.NewQuery().Update(repo.UnderWorkflowField))
		if err != nil {
			return err
		}
	default:
		// empty
	}
	return nil
}

type WorkflowAssignMockUnauthz struct {
	authzLoader *authz_loader.AuthzLoader[authz.RepoResourceType,
		authz.RepoAction]
}

func (s *WorkflowAssignMockUnauthz) AutoAssignApprovers(
	ctx context.Context,
	workflowID uuid.UUID,
) (*model.Workflow, error) {
	_, err := authz.CheckAuthz(ctx, s.authzLoader.AuthzHandler,
		authz.RepoResourceTypeWorkflow, authz.RepoActionList)
	if err != nil {
		return nil, err
	}

	return &model.Workflow{ID: workflowID}, nil
}

func (s *WorkflowAssignMockUnauthz) HandleTerminalWorkflow(ctx context.Context, workflow *model.Workflow) error {
	return nil
}

func TestWorkflowAssignAction(t *testing.T) {
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{}, testutils.WithGenerateTenants(1))
	rawRepo := sql.NewRepository(db)

	tenantID := tenants[0]
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenantID)

	testutils.CreateTestEntities(
		ctx, t, rawRepo,
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

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(),
		rawRepo, &config.Config{})

	authzRepo := authz_repo.NewAuthzRepo(rawRepo, authzRepoLoader)

	assigner := tasks.NewWorkflowProcessor(
		&WorkflowAssignMock{authzLoader: authzRepoLoader, repo: authzRepo}, authzRepo,
	)

	unauthzAssigner := tasks.NewWorkflowProcessor(
		&WorkflowAssignMockUnauthz{authzLoader: authzRepoLoader}, authzRepo,
	)

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

	// We need to interact with the DB in the test. Use this role for convenience,
	// since it includes the necessary permissions.
	ctx, err := cmkcontext.InjectInternalUserData(ctx,
		constants.InternalTaskWorkflowApproversRole)
	assert.NoError(t, err)

	t.Run("Bad workflow causing error", func(t *testing.T) {
		payload := asyncUtils.NewTaskPayload(ctx, []byte(BadWorkflowID))
		data, err := payload.ToBytes()
		assert.NoError(t, err)

		task := asynq.NewTask(config.TypeWorkflowAutoAssign, data)
		err = assigner.ProcessTask(ctx, task)
		assert.ErrorIs(t, err, tasks.ErrRunningTask, ErrBadWorkflow)

		ck := repo.NewCompositeKey().Where(repo.IDField, BadWorkflowID)
		updatedWorkflow := &model.Workflow{}

		_, err = authzRepo.First(ctx, updatedWorkflow,
			*repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)))
		assert.NoError(t, err)

		assert.Equal(t, model.WorkflowStateFailed, updatedWorkflow.State)
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

		_, err = authzRepo.First(ctx, updatedWorkflow,
			*repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)))
		assert.NoError(t, err)

		assert.Equal(t, model.WorkflowStateFailed, updatedWorkflow.State)
		assert.Equal(t, "internal error when assigning approvers", updatedWorkflow.FailureReason)
	})

	t.Run("Unauthorized processing", func(t *testing.T) {
		payload := asyncUtils.NewTaskPayload(ctx, []byte(GoodWorkflowID))
		data, err := payload.ToBytes()
		assert.NoError(t, err)

		task := asynq.NewTask(config.TypeWorkflowAutoAssign, data)
		err = unauthzAssigner.ProcessTask(ctx, task)
		assert.ErrorIs(t, err, authz.ErrAuthorizationDenied)
	})

	t.Run("Successful processing", func(t *testing.T) {
		payload := asyncUtils.NewTaskPayload(ctx, []byte(GoodWorkflowID))
		data, err := payload.ToBytes()
		assert.NoError(t, err)

		task := asynq.NewTask(config.TypeWorkflowAutoAssign, data)
		err = assigner.ProcessTask(ctx, task)
		assert.NoError(t, err, "Expected no error on good workflow")
	})

	t.Run("System underWorkflow set to false on failure", func(t *testing.T) {
		system := testutils.NewSystem(func(s *model.System) {
			s.UnderWorkflow = true
		})

		workflowID, err := uuid.Parse(SystemUnderWorkflowWorkflowID)
		assert.NoError(t, err)

		workflow := testutils.NewWorkflow(func(w *model.Workflow) {
			w.ArtifactType = model.WorkflowArtifactTypeSystem
			w.ArtifactID = system.ID
			w.ID = workflowID
		})

		testutils.CreateTestEntities(
			ctx,
			t,
			rawRepo,
			system,
			workflow,
		)

		// Trigger workflow failure by using BadWorkflowID logic
		// We need to modify the mock to handle this new workflow ID
		payload := asyncUtils.NewTaskPayload(ctx, []byte(workflow.ID.String()))
		data, err := payload.ToBytes()
		assert.NoError(t, err)

		task := asynq.NewTask(config.TypeWorkflowAutoAssign, data)

		// Create a special assigner that will fail for this workflow
		failingAssigner := tasks.NewWorkflowProcessor(&WorkflowAssignMock{authzLoader: authzRepoLoader, repo: rawRepo}, rawRepo)

		err = failingAssigner.ProcessTask(ctx, task)
		assert.ErrorIs(t, err, tasks.ErrRunningTask, ErrBadWorkflow)

		// Verify system.underWorkflow is now false
		updatedSystem := &model.System{ID: system.ID}
		_, err = rawRepo.First(ctx, updatedSystem, *repo.NewQuery())
		assert.NoError(t, err)
		assert.False(t, updatedSystem.UnderWorkflow, "Expected system.underWorkflow to be false after workflow failure")

		// Verify workflow is in failed state
		updatedWorkflow := &model.Workflow{ID: workflow.ID}
		_, err = rawRepo.First(ctx, updatedWorkflow, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Equal(t, model.WorkflowStateFailed, updatedWorkflow.State)
	})
}
