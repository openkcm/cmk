package manager

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	"github.com/openkcm/cmk-core/internal/config"
	"github.com/openkcm/cmk-core/internal/constants"
	"github.com/openkcm/cmk-core/internal/errs"
	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/internal/repo"
	wf "github.com/openkcm/cmk-core/internal/workflow"
	"github.com/openkcm/cmk-core/utils/ptr"
)

var ErrWorkflowApproverDecision = errors.New("workflow approver decision")

type Workflow interface {
	GetWorkflows(ctx context.Context, filter WorkflowFilter) ([]*model.Workflow, int, error)
	CreateWorkflow(ctx context.Context, workflow *model.Workflow) (*model.Workflow, error)
	GetWorkflowsByID(ctx context.Context, workflowID uuid.UUID) (*model.Workflow, error)
	AddWorkflowApprovers(
		ctx context.Context,
		workflowID uuid.UUID,
		userID uuid.UUID,
		approvers []cmkapi.WorkflowApprover,
	) (*model.Workflow, error)
	ListWorkflowApprovers(
		ctx context.Context,
		id uuid.UUID,
		skip int,
		top int,
	) ([]*model.WorkflowApprover, int, error)
	TransitionWorkflow(
		ctx context.Context,
		userID uuid.UUID,
		workflowID uuid.UUID,
		transition wf.Transition,
	) (*model.Workflow, error)
	NewWorkflowFilter(request cmkapi.GetWorkflowsRequestObject) WorkflowFilter
}

type WorkflowManager struct {
	repo                    repo.Repo
	keyManager              wf.KeyActions
	keyConfigurationManager wf.KeyConfigurationActions
	systemManager           wf.SystemActions
	workflowsConfig         *config.Workflows
}

type WorkflowFilter struct {
	State        string
	ArtifactType string
	ArtifactID   uuid.UUID
	ActionType   string
	UserID       uuid.UUID
	Skip         int
	Top          int
}

func NewWorkflowManager(
	repository repo.Repo,
	keyManager *KeyManager,
	keyConfigurationManager *KeyConfigManager,
	systemManager *SystemManager,
	workflowsConfig *config.Workflows,
) *WorkflowManager {
	return &WorkflowManager{
		repo:                    repository,
		keyManager:              keyManager,
		keyConfigurationManager: keyConfigurationManager,
		systemManager:           systemManager,
		workflowsConfig:         workflowsConfig,
	}
}

func (w WorkflowFilter) ApplyToQuery(query *repo.Query) *repo.Query {
	ck := repo.NewCompositeKey()

	if w.State != "" {
		ck = ck.Where(repo.StateField, w.State)
	}

	if w.ArtifactType != "" {
		ck = ck.Where(repo.ArtifactTypeField, w.ArtifactType)
	}

	if w.ArtifactID != uuid.Nil {
		ck = ck.Where(repo.ArtifactIDField, w.ArtifactID)
	}

	if w.ActionType != "" {
		ck = ck.Where(repo.ActionTypeField, w.ActionType)
	}

	if len(ck.Conds) > 0 {
		query = query.Where(repo.NewCompositeKeyGroup(ck))
	}

	if w.UserID != uuid.Nil {
		joinCond := repo.JoinCondition{
			Table:     &model.Workflow{},
			Field:     repo.IDField,
			JoinField: fmt.Sprintf("%s_%s", repo.WorkflowField, repo.IDField),
			JoinTable: &model.WorkflowApprover{},
		}
		query = query.Join(
			repo.LeftJoin,
			joinCond,
		)
		orCK := repo.NewCompositeKey()
		orCK.IsStrict = false
		orCK.Where(repo.InitiatorIDField, w.UserID)
		orCK.Where(fmt.Sprintf("%s_%s", repo.UserField, repo.IDField), w.UserID)

		query = query.Where(repo.NewCompositeKeyGroup(orCK))
	}

	return query
}

func (w *WorkflowManager) NewWorkflowFilter(request cmkapi.GetWorkflowsRequestObject) WorkflowFilter {
	var state string
	if request.Params.State != nil {
		state = strings.ToUpper(string(*request.Params.State))
	}

	var artifactType string
	if request.Params.ArtifactType != nil {
		artifactType = strings.ToUpper(string(*request.Params.ArtifactType))
	}

	var actionType string
	if request.Params.ActionType != nil {
		actionType = strings.ToUpper(string(*request.Params.ActionType))
	}

	return WorkflowFilter{
		State:        state,
		ArtifactType: artifactType,
		ArtifactID:   ptr.GetSafeDeref(request.Params.ArtifactID),
		ActionType:   actionType,
		UserID:       ptr.GetSafeDeref(request.Params.UserID),
		Skip:         ptr.GetIntOrDefault(request.Params.Skip, constants.DefaultSkip),
		Top:          ptr.GetIntOrDefault(request.Params.Top, constants.DefaultTop),
	}
}

func (w *WorkflowManager) GetWorkflows(
	ctx context.Context,
	filter WorkflowFilter,
) ([]*model.Workflow, int, error) {
	workflows := []*model.Workflow{}

	query := repo.NewQuery().SetLimit(filter.Top).SetOffset(filter.Skip)
	query = filter.ApplyToQuery(query)

	count, err := w.repo.List(ctx, model.Workflow{}, &workflows, *query)
	if err != nil {
		return nil, 0, errs.Wrap(ErrGetWorkflowDB, err)
	}

	return workflows, count, nil
}

func (w *WorkflowManager) CreateWorkflow(
	ctx context.Context,
	workflow *model.Workflow,
) (*model.Workflow, error) {
	workflow.State = wf.StateInitial.String()

	exist, err := w.checkOngoingWorkflowForArtifact(ctx, workflow)
	if err != nil {
		return nil, err
	} else if exist {
		return nil, ErrOngoingWorkflowExist
	}

	err = w.repo.Create(ctx, workflow)
	if err != nil {
		return nil, errs.Wrap(ErrCreateWorkflowDB, err)
	}

	return workflow, nil
}

func (w *WorkflowManager) GetWorkflowsByID(ctx context.Context, workflowID uuid.UUID) (*model.Workflow, error) {
	workflow := &model.Workflow{ID: workflowID}

	_, err := w.repo.First(ctx, workflow, *repo.NewQuery().Preload(repo.Preload{"Approvers"}))
	if err != nil {
		return nil, errs.Wrap(ErrGetWorkflowDB, err)
	}

	return workflow, nil
}

// ListWorkflowApprovers retrieves a paginated list of approvers for a given workflow ID.
// Returns a slice of WorkflowApprover, the total count, and an error if any occurs.
func (w *WorkflowManager) ListWorkflowApprovers(
	ctx context.Context,
	id uuid.UUID,
	skip int,
	top int,
) ([]*model.WorkflowApprover, int, error) {
	workflows := &model.Workflow{}

	ck := repo.NewCompositeKey().
		Where(repo.IDField, id)

	_, err := w.repo.First(
		ctx,
		workflows,
		*repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)),
	)
	if err != nil {
		return nil, 0, errs.Wrap(ErrGetWorkflowDB, err)
	}

	var approvers []*model.WorkflowApprover

	ck = repo.NewCompositeKey().
		Where(fmt.Sprintf("%s_%s", repo.WorkflowField, repo.IDField), id)

	count, err := w.repo.List(
		ctx,
		model.WorkflowApprover{},
		&approvers,
		*repo.NewQuery().
			Where(repo.NewCompositeKeyGroup(ck)).
			SetLimit(top).SetOffset(skip),
	)
	if err != nil {
		return nil, 0, errs.Wrap(wf.ErrListApprovers, err)
	}

	return approvers, count, nil
}

func (w *WorkflowManager) AddWorkflowApprovers(
	ctx context.Context,
	workflowID uuid.UUID,
	userID uuid.UUID,
	approvers []cmkapi.WorkflowApprover,
) (*model.Workflow, error) {
	workflow := &model.Workflow{ID: workflowID}

	_, err := w.repo.First(ctx, workflow, *repo.NewQuery())
	if err != nil {
		return nil, errs.Wrap(ErrGetWorkflowDB, err)
	}

	if workflow.State != wf.StateInitial.String() {
		return nil, ErrWorkflowNotInitial
	}

	if workflow.InitiatorID != userID {
		return nil, errs.Wrap(ErrValidateActor,
			wf.NewInvalidEventActorError(userID, "initiator"))
	}

	err = w.addApprovers(ctx, userID, workflow, approvers)
	if err != nil {
		return nil, errs.Wrap(ErrAddApproversDB, err)
	}

	return workflow, nil
}

func (w *WorkflowManager) TransitionWorkflow(
	ctx context.Context,
	userID uuid.UUID,
	workflowID uuid.UUID,
	transition wf.Transition,
) (*model.Workflow, error) {
	workflow := &model.Workflow{ID: workflowID}

	_, err := w.repo.First(ctx, workflow, *repo.NewQuery())
	if err != nil {
		return nil, errs.Wrap(ErrGetWorkflowDB, err)
	}

	err = w.applyTransition(
		ctx,
		userID,
		workflow,
		transition,
	)
	if err != nil {
		return nil, err
	}

	return workflow, nil
}

// addApprovers adds the specified approvers to the workflow
// and transitions the workflow to the next state.
// This is wrapped in a transaction to ensure that DB state is consistent
func (w *WorkflowManager) addApprovers(
	ctx context.Context,
	userID uuid.UUID,
	workflow *model.Workflow,
	approvers []cmkapi.WorkflowApprover,
) error {
	err := w.repo.Transaction(ctx, func(ctx context.Context, r repo.Repo) error {
		workflowLifecycle := wf.NewLifecycle(
			workflow,
			w.keyManager,
			w.keyConfigurationManager,
			w.systemManager,
			r,
			userID,
			w.workflowsConfig.MinimumApprovals,
		)

		// Add each approver to the workflow
		for _, approver := range approvers {
			approver := model.WorkflowApprover{
				WorkflowID: workflow.ID,
				UserID:     approver.Id,
			}

			_, err := r.First(ctx, workflow, *repo.NewQuery())
			if err != nil {
				return errs.Wrap(ErrGetWorkflowDB, err)
			}

			err = r.Create(ctx, &approver)
			if err != nil {
				return errs.Wrap(ErrAddApproversDB, err)
			}
		}

		// Then, apply the transition to next state
		err := workflowLifecycle.ApplyTransition(ctx, wf.TransitionCreate)
		if err != nil {
			return errs.Wrap(ErrApplyTransition, err)
		}

		return nil
	})
	if err != nil {
		return errs.Wrap(ErrInDBTransaction, err)
	}

	return nil
}

func (w *WorkflowManager) checkOngoingWorkflowForArtifact(
	ctx context.Context,
	workflow *model.Workflow,
) (bool, error) {
	workflows := []*model.Workflow{}

	ck := repo.NewCompositeKey().
		Where(fmt.Sprintf("%s_%s", repo.ArtifactField, repo.TypeField), workflow.ArtifactType).
		Where(fmt.Sprintf("%s_%s", repo.ArtifactField, repo.IDField), workflow.ArtifactID).
		Where(repo.StateField, wf.NonTerminalStates)

	count, err := w.repo.List(ctx, model.Workflow{}, &workflows, *repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)))
	if err != nil {
		return false, errs.Wrap(ErrCheckOngoingWorkflow, err)
	}

	return count > 0, nil
}

// updateApproverDecisionAndApplyTransition updates the approver
// decision and applies the transition to the wf.
// This is wrapped in a transaction to ensure that DB state is
// consistent in case of errors.
func (w *WorkflowManager) applyTransition(
	ctx context.Context,
	userID uuid.UUID,
	workflow *model.Workflow,
	transition wf.Transition,
) error {
	err := w.repo.Transaction(ctx, func(ctx context.Context, r repo.Repo) error {
		workflowLifecycle := wf.NewLifecycle(
			workflow, w.keyManager, w.keyConfigurationManager, w.systemManager, r, userID,
			w.workflowsConfig.MinimumApprovals,
		)

		validateErr := workflowLifecycle.ValidateActor(ctx, transition)
		if validateErr != nil {
			return errs.Wrap(ErrValidateActor, validateErr)
		}

		var txErr error

		switch transition {
		case wf.TransitionApprove:
			txErr = w.updateApproverDecision(ctx, workflow.ID, userID, true)
		case wf.TransitionReject:
			txErr = w.updateApproverDecision(ctx, workflow.ID, userID, false)
		case wf.TransitionCreate, wf.TransitionExpire,
			wf.TransitionExecute, wf.TransitionFail:
			txErr = ErrWorkflowCannotTransitionDB
		case wf.TransitionConfirm, wf.TransitionRevoke:
			txErr = nil
		}

		if txErr != nil {
			return txErr
		}

		transitionErr := workflowLifecycle.ApplyTransition(ctx, transition)
		if transitionErr != nil {
			return errs.Wrap(ErrApplyTransition, transitionErr)
		}

		return nil
	})
	if err != nil {
		return errs.Wrap(ErrInDBTransaction, err)
	}

	return nil
}

// UpdateApproverDecision updates the decision of an approver on a wfMechanism.
func (w *WorkflowManager) updateApproverDecision(
	ctx context.Context,
	workflowID uuid.UUID,
	approverID uuid.UUID,
	approved bool,
) error {
	approver := &model.WorkflowApprover{}

	err := w.repo.Transaction(ctx, func(ctx context.Context, r repo.Repo) error {
		ck := repo.NewCompositeKey().
			Where(fmt.Sprintf("%s_%s", repo.UserField, repo.IDField), approverID).
			Where(fmt.Sprintf("%s_%s", repo.WorkflowField, repo.IDField), workflowID)

		_, err := r.First(ctx, approver, *repo.NewQuery().
			Where(repo.NewCompositeKeyGroup(ck)))
		if err != nil {
			return errs.Wrap(wf.ErrCheckApproverDecision, err)
		}

		approver.Approved = sql.NullBool{Bool: approved, Valid: true}

		_, err = r.Patch(ctx, approver, *repo.NewQuery())
		if err != nil {
			return errs.Wrap(ErrUpdateApproverDecision, err)
		}

		return nil
	})
	if err != nil {
		return errs.Wrap(ErrWorkflowApproverDecision, err)
	}

	return nil
}
