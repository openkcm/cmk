package manager

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/samber/oops"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/notifier"
	wn "github.com/openkcm/cmk/internal/notifier/workflow"
	serviceapi "github.com/openkcm/cmk/internal/pluginregistry/service/api"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/identitymanagement"
	"github.com/openkcm/cmk/internal/repo"
	wf "github.com/openkcm/cmk/internal/workflow"
	asyncUtils "github.com/openkcm/cmk/utils/async"
	cmkContext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/odata"
	"github.com/openkcm/cmk/utils/ptr"
)

const (
	WorkflowSystemArtifactPropertyKey = "NAME"
)

var (
	ErrWorkflowApproverDecision   = errors.New("workflow approver decision")
	ErrWorkflowNotAllowed         = errors.New("user has no permission to access workflow")
	ErrWorkflowCreationNotAllowed = errors.New("user has no permission to create workflow")
)

type WorkflowStatus struct {
	Enabled    bool
	Exists     bool
	Valid      bool
	CanCreate  bool
	ErrDetails error
}

type WorkflowEligibility struct {
	InsufficientApprovers bool
	InitiatorIneligible   bool
}

type Workflow interface {
	CheckWorkflow(ctx context.Context, workflow *model.Workflow) (WorkflowStatus, error)
	GetWorkflows(ctx context.Context, params repo.QueryMapper) ([]*model.Workflow, int, error)
	CreateWorkflow(ctx context.Context, workflow *model.Workflow) (*model.Workflow, error)
	GetWorkflowByID(ctx context.Context, workflowID uuid.UUID) (*model.Workflow, *WorkflowEligibility, error)
	ListWorkflowApprovers(
		ctx context.Context,
		id uuid.UUID,
		decisionMade bool,
		pagination repo.Pagination,
	) ([]*model.WorkflowApprover, int, error)
	GetWorkflowAvailableTransitions(ctx context.Context, workflow *model.Workflow) ([]wf.Transition, error)
	GetWorkflowApprovalSummary(ctx context.Context, workflow *model.Workflow) (*wf.ApprovalSummary, error)
	GetWorkflowApproverGroups(ctx context.Context, workflow *model.Workflow) ([]*model.Group, error)
	WorkflowCanExpire(ctx context.Context, workflow *model.Workflow) (bool, error)
	TransitionWorkflow(
		ctx context.Context,
		workflowID uuid.UUID,
		transition wf.Transition,
	) (*model.Workflow, error)
	WorkflowConfig(ctx context.Context) (*model.WorkflowConfig, error)
	IsWorkflowRequired(ctx context.Context) (bool, error)
	CleanupTerminalWorkflows(ctx context.Context) error
	HandleTerminalWorkflow(ctx context.Context, workflow *model.Workflow) error
}

type WorkflowManager struct {
	repo                    repo.Repo
	keyManager              *KeyManager
	keyConfigurationManager *KeyConfigManager
	systemManager           *SystemManager
	groupManager            *GroupManager
	userManager             User
	asyncClient             async.Client
	tenantConfigManager     *TenantConfigManager
	svcRegistry             serviceapi.Registry
	cfg                     *config.Config
}

func NewWorkflowManager(
	repository repo.Repo,
	svcRegistry serviceapi.Registry,
	keyManager *KeyManager,
	keyConfigurationManager *KeyConfigManager,
	systemManager *SystemManager,
	groupManager *GroupManager,
	userManager User,
	asyncClient async.Client,
	tenantConfigManager *TenantConfigManager,
	cfg *config.Config,
) *WorkflowManager {
	return &WorkflowManager{
		repo:                    repository,
		svcRegistry:             svcRegistry,
		keyManager:              keyManager,
		keyConfigurationManager: keyConfigurationManager,
		systemManager:           systemManager,
		groupManager:            groupManager,
		userManager:             userManager,
		asyncClient:             asyncClient,
		tenantConfigManager:     tenantConfigManager,
		cfg:                     cfg,
	}
}

type WorkflowFilter struct {
	State                  model.WorkflowState
	ArtifactType           model.WorkflowArtifactType
	ArtifactID             uuid.UUID
	ArtifactName           string
	ParametersResourceName string
	ActionType             model.WorkflowActionType
	Skip                   int
	Top                    int
	Count                  bool
}

var _ repo.QueryMapper = (*WorkflowFilter)(nil) // Assert interface impl

// validateFilterEnums casts raw OData strings to typed workflow enums and
// rejects unknown non-empty values.
func validateFilterEnums(
	state, artifactType, actionType string,
) (model.WorkflowState, model.WorkflowArtifactType, model.WorkflowActionType, error) {
	wfState := model.WorkflowState(state)
	if state != "" && !wfState.Valid() {
		return "", "", "", fmt.Errorf("%w: %q", model.ErrInvalidWorkflowState, state)
	}
	wfArtifactType := model.WorkflowArtifactType(artifactType)
	if artifactType != "" && !wfArtifactType.Valid() {
		return "", "", "", fmt.Errorf("%w: %q", model.ErrInvalidWorkflowArtifactType, artifactType)
	}
	wfActionType := model.WorkflowActionType(actionType)
	if actionType != "" && !wfActionType.Valid() {
		return "", "", "", fmt.Errorf("%w: %q", model.ErrInvalidWorkflowActionType, actionType)
	}
	return wfState, wfArtifactType, wfActionType, nil
}

func NewWorkflowFilterFromOData(queryMapper odata.QueryOdataMapper) (*WorkflowFilter, error) {
	skipPtr, topPtr, countPtr := queryMapper.GetPaging()
	skip := ptr.GetPtrOrDefault(skipPtr, constants.DefaultSkip)
	top := ptr.GetPtrOrDefault(topPtr, constants.DefaultTop)
	count := ptr.GetSafeDeref(countPtr)

	state, err := queryMapper.GetString(repo.StateField)
	if err != nil {
		return nil, err
	}

	artifactType, err := queryMapper.GetString(repo.ArtifactTypeField)
	if err != nil {
		return nil, err
	}

	artifactID, err := queryMapper.GetUUID(repo.ArtifactIDField)
	if err != nil {
		return nil, err
	}

	artifactName, err := queryMapper.GetString(repo.ArtifactNameField)
	if err != nil {
		return nil, err
	}

	actionType, err := queryMapper.GetString(repo.ActionTypeField)
	if err != nil {
		return nil, err
	}

	parametersResourceName, err := queryMapper.GetString(repo.ParamResourceNameField)
	if err != nil {
		return nil, err
	}

	wfState, wfArtifactType, wfActionType, err := validateFilterEnums(state, artifactType, actionType)
	if err != nil {
		return nil, err
	}

	return &WorkflowFilter{
		State:                  wfState,
		ArtifactType:           wfArtifactType,
		ArtifactID:             artifactID,
		ArtifactName:           artifactName,
		ParametersResourceName: parametersResourceName,
		ActionType:             wfActionType,
		Skip:                   skip,
		Top:                    top,
		Count:                  count,
	}, nil
}

func (w WorkflowFilter) GetQuery(_ context.Context) *repo.Query {
	query := repo.NewQuery()

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

	if w.ArtifactName != "" {
		ck = ck.Where(repo.ArtifactNameField, w.ArtifactName)
	}

	if w.ParametersResourceName != "" {
		ck = ck.Where(repo.ParamResourceNameField, w.ParametersResourceName)
	}

	if w.ActionType != "" {
		ck = ck.Where(repo.ActionTypeField, w.ActionType)
	}

	if len(ck.Conds) > 0 {
		query = query.Where(repo.NewCompositeKeyGroup(ck))
	}

	query = query.Order(repo.OrderField{
		Field:     repo.CreatedField,
		Direction: repo.Desc,
	})

	return query
}

func (w WorkflowFilter) GetUUID(field repo.QueryField) (uuid.UUID, error) {
	var id uuid.UUID

	switch field {
	case repo.ArtifactIDField:
		id = w.ArtifactID
	default:
		return uuid.Nil, ErrIncompatibleQueryField
	}

	return id, nil
}

func (w WorkflowFilter) GetPagination() repo.Pagination {
	return repo.Pagination{
		Skip:  w.Skip,
		Top:   w.Top,
		Count: w.Count,
	}
}

func (w WorkflowFilter) GetString(field repo.QueryField) (string, error) {
	var val string

	switch field {
	case repo.StateField:
		val = w.State.String()
	case repo.ArtifactTypeField:
		val = w.ArtifactType.String()
	case repo.ActionTypeField:
		val = w.ActionType.String()
	case repo.ArtifactNameField:
		val = w.ArtifactName
	case repo.ParamResourceNameField:
		val = w.ParametersResourceName
	default:
		return "", ErrIncompatibleQueryField
	}

	return val, nil
}

func (w *WorkflowManager) GetWorkflows(
	ctx context.Context,
	params repo.QueryMapper,
) ([]*model.Workflow, int, error) {
	pagination := params.GetPagination()
	return w.getWorkflows(ctx, pagination, params.GetQuery(ctx))
}

func (w *WorkflowManager) WorkflowConfig(ctx context.Context) (*model.WorkflowConfig, error) {
	workflowConfig, err := w.tenantConfigManager.GetWorkflowConfig(ctx)
	if err != nil {
		return nil, oops.Join(ErrGetWorkflowConfig, err)
	}

	return workflowConfig, nil
}

func (w *WorkflowManager) CheckWorkflow(
	ctx context.Context,
	workflow *model.Workflow,
) (WorkflowStatus, error) {
	workflowConfig, err := w.WorkflowConfig(ctx)
	if err != nil {
		return WorkflowStatus{}, err
	}

	enabled := workflowConfig.Enabled

	allowed, err := w.checkPermissionToCreateWorkflow(ctx, workflow)
	if err != nil {
		return WorkflowStatus{}, errs.Wrap(ErrCheckWorkflow, err)
	}

	if !allowed {
		return WorkflowStatus{},
			errs.Wrap(ErrCheckWorkflow, ErrWorkflowCreationNotAllowed)
	}

	// After this point user is authorised, we can reveal information
	status, err := w.checkWorkflow(ctx, workflow, enabled)
	return transformCheckWorkflowError(status, err)
}

func (w *WorkflowManager) CreateWorkflow(
	ctx context.Context,
	workflow *model.Workflow,
) (*model.Workflow, error) {
	workflow.State = model.WorkflowStateInitial

	// Capture minimum approvals from current tenant configuration as a snapshot
	minimumApprovals := w.getMinimumApprovals(ctx)
	workflow.MinimumApprovalCount = minimumApprovals

	status, err := w.CheckWorkflow(ctx, workflow)
	if err != nil {
		return nil, err
	}
	if status.Exists {
		return nil, ErrOngoingWorkflowExist
	}
	// Return validation errors (including insufficient approvers)
	if status.ErrDetails != nil {
		return nil, status.ErrDetails
	}

	err = w.populateArtifact(ctx, workflow)
	if err != nil {
		return nil, err
	}

	err = w.populateParametersResource(ctx, workflow)
	if err != nil {
		return nil, err
	}

	err = w.repo.Transaction(ctx, func(ctx context.Context) error {
		err = w.repo.Create(ctx, workflow)
		if err != nil {
			return errs.Wrap(ErrCreateWorkflowDB, err)
		}

		err = w.handleNewWorkflow(ctx, workflow)
		if err != nil {
			return errs.Wrap(ErrCreateWorkflowDB, err)
		}

		err = w.createAutoAssignApproversAsyncTask(ctx, *workflow)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, errs.Wrap(ErrInDBTransaction, err)
	}

	return workflow, nil
}

func (w *WorkflowManager) GetWorkflowByID(
	ctx context.Context,
	workflowID uuid.UUID,
) (*model.Workflow, *WorkflowEligibility, error) {
	query := repo.NewQuery()
	ck := repo.NewCompositeKey()
	ck = ck.Where(repo.IDField, workflowID)
	query = query.Where(repo.NewCompositeKeyGroup(ck))

	workflows, _, err := w.getWorkflows(ctx, repo.Pagination{}, query)
	if err != nil {
		return nil, nil, err
	}

	if len(workflows) == 0 {
		return nil, nil, errs.Wrap(ErrWorkflowNotAllowed, err)
	}

	workflow := workflows[0]

	// Single eligibility check for both approvers and initiator (one SCIM call)
	insufficientApprovers, initiatorIneligible, err := w.checkWorkflowEligibility(ctx, workflow)
	if err != nil {
		// Return error so caller can decide how to handle SCIM failures
		return workflow, nil, err
	}

	return workflow, &WorkflowEligibility{
		InsufficientApprovers: insufficientApprovers,
		InitiatorIneligible:   initiatorIneligible,
	}, nil
}

// ListWorkflowApprovers retrieves a paginated list of approvers for a given workflow ID.
// Returns a slice of WorkflowApprover, the total count, and an error if any occurs.
func (w *WorkflowManager) ListWorkflowApprovers(
	ctx context.Context,
	id uuid.UUID,
	decisionMade bool,
	pagination repo.Pagination,
) ([]*model.WorkflowApprover, int, error) {
	// Verify workflow exists
	if _, _, err := w.GetWorkflowByID(ctx, id); err != nil {
		return nil, 0, err
	}

	ck := repo.NewCompositeKey().
		Where(fmt.Sprintf("%s_%s", repo.WorkflowField, repo.IDField), id)

	if decisionMade {
		ck = ck.Where(repo.ApprovedField, repo.NotNull)
	}

	query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck))

	return repo.ListAndCount(ctx, w.repo, pagination, model.WorkflowApprover{}, query)
}

func (w *WorkflowManager) AutoAssignApprovers(
	ctx context.Context,
	workflowID uuid.UUID,
) (*model.Workflow, error) {
	workflow := &model.Workflow{ID: workflowID}

	_, err := w.repo.First(ctx, workflow, *repo.NewQuery())
	if err != nil {
		return nil, errs.Wrap(ErrGetWorkflowDB, err)
	}

	keyConfigs, err := w.getKeyConfigurationsFromArtifact(ctx, workflow)
	if err != nil {
		return nil, err
	}

	approvers, groups, err := w.getApproversAndGroupsFromKeyConfigs(ctx, workflow, keyConfigs)
	if err != nil {
		return nil, err
	}

	err = w.addApproversAndGroupAssociations(ctx, workflow.InitiatorID, workflow, approvers, groups)
	if err != nil {
		return nil, errs.Wrap(ErrAddApproversDB, err)
	}

	idm, err := w.svcRegistry.IdentityManagement()
	if err != nil {
		return nil, err
	}

	approverValues := make([]model.WorkflowApprover, len(approvers))
	for i, approver := range approvers {
		if approver != nil {
			approverValues[i] = *approver
		}
	}

	approverUserNames, err := wf.GetApproverUserNames(ctx, approverValues, idm)
	if err != nil {
		return nil, err
	}

	err = w.createWorkflowTransitionNotificationTask(ctx, *workflow, wf.TransitionCreate, approverUserNames)
	if err != nil {
		log.Error(ctx, "create workflow creation notification task", err)
	}

	return workflow, nil
}

func (w *WorkflowManager) GetWorkflowAvailableTransitions(
	ctx context.Context,
	workflow *model.Workflow,
) ([]wf.Transition, error) {
	userID, err := cmkContext.ExtractUserIdentifier(ctx)
	if err != nil {
		return nil, err
	}

	workflowLifecycle, err := w.getWorkflowLifecycle(ctx, workflow, userID)
	if err != nil {
		return nil, err
	}

	// For now assume that only business users can perform transitions.
	// When other types of users are supported, this logic will need to be updated.
	transitions := workflowLifecycle.AvailableBusinessUserTransitions(ctx)

	return transitions, nil
}

func (w *WorkflowManager) WorkflowCanExpire(
	ctx context.Context,
	workflow *model.Workflow,
) (bool, error) {
	userID, err := cmkContext.ExtractUserIdentifier(ctx)
	if err != nil {
		return false, err
	}

	workflowLifecycle, err := w.getWorkflowLifecycle(ctx, workflow, userID)
	if err != nil {
		return false, err
	}

	return workflowLifecycle.CanExpire(), nil
}

func (w *WorkflowManager) ExpireWorkflow(
	ctx context.Context,
	workflowID uuid.UUID,
) (*model.Workflow, error) {
	workflow := &model.Workflow{ID: workflowID}

	_, err := w.repo.First(ctx, workflow, *repo.NewQuery())
	if err != nil {
		return nil, errs.Wrap(ErrGetWorkflowDB, err)
	}

	err = w.repo.Transaction(ctx, func(ctx context.Context) error {
		userID, err := cmkContext.ExtractUserIdentifier(ctx)
		if err != nil {
			return err
		}

		workflowLifecycle, err := w.getWorkflowLifecycle(ctx, workflow, userID)
		if err != nil {
			return err
		}

		if err = workflowLifecycle.Expire(ctx); err != nil {
			return err
		}

		err = w.HandleTerminalWorkflow(ctx, workflow)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return workflow, nil
}

func (w *WorkflowManager) GetWorkflowApprovalSummary(
	ctx context.Context,
	workflow *model.Workflow,
) (*wf.ApprovalSummary, error) {
	userID, err := cmkContext.ExtractUserIdentifier(ctx)
	if err != nil {
		return nil, err
	}

	workflowLifecycle, err := w.getWorkflowLifecycle(ctx, workflow, userID)
	if err != nil {
		return nil, err
	}

	summary, err := workflowLifecycle.GetApprovalSummary(ctx)
	if err != nil {
		return nil, err
	}

	return summary, nil
}

func (w *WorkflowManager) GetWorkflowApproverGroups(
	ctx context.Context,
	workflow *model.Workflow,
) ([]*model.Group, error) {
	query := repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(
			repo.NewCompositeKey().Where(fmt.Sprintf(
				"%s.%s", model.WorkflowApproverGroup{}.TableName(), repo.WorkflowIDField,
			), workflow.ID),
		),
	).Join(repo.LeftJoin, repo.JoinCondition{
		Table:     model.Group{},
		Field:     repo.IDField,
		JoinTable: model.WorkflowApproverGroup{},
		JoinField: repo.GroupIDField,
	})

	var groups []*model.Group
	err := w.repo.List(ctx, model.Group{}, &groups, *query)
	if err != nil {
		return []*model.Group{}, err
	}

	// Fallback: if no groups found in junction table, try reading from legacy JSONB field
	// This handles the case where async data migration hasn't run yet
	// This should be deleted in future release as well as deleting the column from the table
	if len(groups) == 0 {
		groups, err = w.getApproverGroupsFromLegacyField(ctx, workflow)
		if err != nil {
			return []*model.Group{}, err
		}
	}

	return groups, nil
}

func (w *WorkflowManager) TransitionWorkflow(
	ctx context.Context,
	workflowID uuid.UUID,
	transition wf.Transition,
) (*model.Workflow, error) {
	userID, err := cmkContext.ExtractUserIdentifier(ctx)
	if err != nil {
		return nil, err
	}

	workflow := &model.Workflow{ID: workflowID}

	_, err = w.repo.First(
		ctx,
		workflow,
		*repo.NewQuery().Preload(repo.Preload{"Approvers"}),
	)
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

	idm, err := w.svcRegistry.IdentityManagement()
	if err != nil {
		return nil, err
	}

	recipients, err := wf.GetNotificationRecipients(ctx, *workflow, transition, idm)
	if err != nil {
		return nil, err
	}

	err = w.createWorkflowTransitionNotificationTask(ctx, *workflow, transition, recipients)
	if err != nil {
		log.Error(ctx, "create workflow transition notification task", err)
	}

	return workflow, nil
}

func (w *WorkflowManager) IsWorkflowRequired(ctx context.Context) (bool, error) {
	workflowConfig, err := w.WorkflowConfig(ctx)
	if err != nil {
		return false, err
	}

	return workflowConfig.Enabled, nil
}

func (w *WorkflowManager) CleanupTerminalWorkflows(ctx context.Context) error {
	workflowConfig, err := w.WorkflowConfig(ctx)
	if err != nil {
		return err
	}

	cutoffDate := time.Now().AddDate(0, 0, -workflowConfig.RetentionPeriodDays)
	compositeKey := repo.NewCompositeKey().
		Where(repo.StateField, model.WorkflowTerminalStates).
		Where(repo.CreatedField, cutoffDate, repo.Lt)

	query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(compositeKey))

	// Process workflows in batches to avoid loading all records into memory
	// Use DeleteMode since we're deleting items during processing
	err = repo.ProcessInBatchWithOptions(
		ctx,
		w.repo,
		query,
		repo.DefaultLimit,
		repo.BatchProcessOptions{DeleteMode: true},
		func(workflows []*model.Workflow) error {
			if len(workflows) == 0 {
				return nil
			}

			// Delete workflows in a transaction
			// BeforeDelete hook will automatically delete associated approvers
			return w.repo.Transaction(ctx, func(ctx context.Context) error {
				for _, workflow := range workflows {
					_, err := w.repo.Delete(ctx, &model.Workflow{ID: workflow.ID}, *repo.NewQuery())
					if err != nil {
						return err
					}
				}
				return nil
			})
		},
	)
	if err != nil {
		return err
	}
	return nil
}

// HandleTerminalWorkflow clears the UnderWorkflow flag when a workflow reaches a terminal state
func (w *WorkflowManager) HandleTerminalWorkflow(ctx context.Context, workflow *model.Workflow) error {
	if !slices.Contains(model.WorkflowTerminalStates, workflow.State) {
		return nil
	}

	switch workflow.ArtifactType {
	case model.WorkflowArtifactTypeSystem:
		system := &model.System{
			ID:            workflow.ArtifactID,
			UnderWorkflow: false,
		}
		_, err := w.repo.Patch(ctx, system, *repo.NewQuery().Update(repo.UnderWorkflowField))
		if err != nil {
			return err
		}
	case model.WorkflowArtifactTypeKey:
		key := &model.Key{
			ID:            workflow.ArtifactID,
			UnderWorkflow: false,
		}
		_, err := w.repo.Patch(ctx, key, *repo.NewQuery().Update(repo.UnderWorkflowField))
		if err != nil {
			return err
		}
	default:
		// empty
	}
	return nil
}

func (w *WorkflowManager) getApproverGroupsFromLegacyField(
	ctx context.Context,
	workflow *model.Workflow,
) ([]*model.Group, error) {
	var IDs []uuid.UUID

	if workflow.ApproverGroupIDs == nil {
		return []*model.Group{}, nil
	}

	err := json.Unmarshal(workflow.ApproverGroupIDs, &IDs)
	if err != nil {
		return nil, err
	}

	groups := make([]*model.Group, 0, len(IDs))
	for _, id := range IDs {
		group, err := w.groupManager.GetGroupByID(ctx, id)
		if err != nil {
			log.Warn(ctx, "failed to expand workflow approver group", slog.Any("error", err))

			// Return a placeholder group if the group cannot be found. We can still make use of the ID.
			groups = append(groups, &model.Group{
				ID:   id,
				Name: "NOT_AVAILABLE",
				Role: constants.KeyAdminRole,
			})
			continue
		}

		groups = append(groups, group)
	}

	return groups, nil
}

// validateApproverCount checks if sufficient eligible approvers are available
//
// Returns:
//   - canCreate: true if sufficient approvers exist, false otherwise
//   - err: error details if validation fails
func (w *WorkflowManager) validateApproverCount(
	ctx context.Context,
	workflow *model.Workflow,
	minimumApprovals int,
) (bool, error) {
	// Get key configurations from the workflow artifact
	keyConfigs, err := w.getKeyConfigurationsFromArtifact(ctx, workflow)
	if err != nil {
		return false, errs.Wrap(ErrCheckWorkflow, err)
	}

	approvers, _, err := w.getApproversAndGroupsFromKeyConfigs(ctx, workflow, keyConfigs)
	if err != nil {
		return false, errs.Wrap(ErrCheckWorkflow, err)
	}

	eligibleCount := len(approvers)

	// Validate sufficient approvers exist
	if eligibleCount < minimumApprovals {
		return false, wf.NewInsufficientApproversError(minimumApprovals, eligibleCount)
	}

	return true, nil
}

func (w *WorkflowManager) handleNewWorkflow(ctx context.Context, workflow *model.Workflow) error {
	switch workflow.ArtifactType {
	case model.WorkflowArtifactTypeSystem:
		system := &model.System{
			ID:            workflow.ArtifactID,
			UnderWorkflow: true,
		}
		_, err := w.repo.Patch(ctx, system, *repo.NewQuery())
		if err != nil {
			return err
		}
	case model.WorkflowArtifactTypeKey:
		key := &model.Key{
			ID:            workflow.ArtifactID,
			UnderWorkflow: true,
		}
		_, err := w.repo.Patch(ctx, key, *repo.NewQuery())
		if err != nil {
			return err
		}
	default:
		// empty
	}
	return nil
}

// checkWorkflowEligibility performs eligibility checks for both approvers and initiator with a single SCIM call.
// These checks run regardless of workflow state to provide factual eligibility information to API consumers.
// Returns (insufficientApprovers, initiatorIneligible, error)
func (w *WorkflowManager) checkWorkflowEligibility(
	ctx context.Context,
	workflow *model.Workflow,
) (bool, bool, error) {
	// Get eligible user IDs from IAM
	eligibleUserIDs, err := w.getEligibleUserIDsForWorkflow(ctx, workflow)
	if err != nil {
		return false, false, err
	}

	// If no eligibility restrictions (nil map = no groups configured), return early
	if eligibleUserIDs == nil {
		return false, false, nil
	}

	// Check 1: Insufficient approvers
	insufficientApprovers, err := w.checkInsufficientApprovers(ctx, workflow, eligibleUserIDs)
	if err != nil {
		return false, false, err
	}

	// Check 2: Initiator ineligible
	initiatorIneligible := w.checkInitiatorIneligible(workflow, eligibleUserIDs)

	return insufficientApprovers, initiatorIneligible, nil
}

// getEligibleUserIDsForWorkflow fetches eligible user IDs from IAM groups for the workflow.
// Returns nil map with nil error if workflow has no approver group restrictions.
// Returns nil map with error if IAM query fails.
// Returns populated map with nil error if eligibility restrictions exist.
func (w *WorkflowManager) getEligibleUserIDsForWorkflow(
	ctx context.Context,
	workflow *model.Workflow,
) (map[string]bool, error) {
	groups, err := w.GetWorkflowApproverGroups(ctx, workflow)
	if err != nil {
		return nil, errs.Wrap(ErrCheckWorkflowEligibility, err)
	}

	if len(groups) == 0 {
		return nil, nil //nolint:nilnil // nil map means no restrictions, nil error means success
	}

	// Get identity management plugin
	idm, err := w.groupManager.GetIdentityManagementPlugin()
	if err != nil {
		return nil, errs.Wrap(ErrCheckWorkflowEligibility, err)
	}

	// Get auth context
	authCtx, _ := cmkContext.ExtractBusinessUserDataAuthContext(ctx)
	if authCtx == nil {
		authCtx = map[string]string{}
	}

	// Single SCIM call - get all eligible users from groups
	eligibleUserIDs, err := w.queryGroupMembersFromIAM(ctx, idm, authCtx, groups)
	if err != nil {
		return nil, err
	}

	return eligibleUserIDs, nil
}

// checkInsufficientApprovers checks if there are insufficient eligible approvers
func (w *WorkflowManager) checkInsufficientApprovers(
	ctx context.Context,
	workflow *model.Workflow,
	eligibleUserIDs map[string]bool,
) (bool, error) {
	// Create lifecycle to access business logic
	lifecycle, err := w.getWorkflowLifecycle(ctx, workflow, "")
	if err != nil {
		return false, errs.Wrap(ErrCheckWorkflowEligibility, err)
	}

	// Get all approvers from DB
	allApprovers, err := lifecycle.GetAllApprovers(ctx)
	if err != nil {
		return false, errs.Wrap(ErrCheckWorkflowEligibility, err)
	}

	// Filter approvers by eligibility (using eligibleUserIDs from SCIM)
	var eligibleApprovers []*model.WorkflowApprover
	for _, approver := range allApprovers {
		if eligibleUserIDs[approver.UserID] {
			eligibleApprovers = append(eligibleApprovers, approver)
		}
	}

	return lifecycle.CheckInsufficientApprovers(eligibleApprovers), nil
}

// checkInitiatorIneligible checks if the initiator is no longer eligible to confirm
func (w *WorkflowManager) checkInitiatorIneligible(
	workflow *model.Workflow,
	eligibleUserIDs map[string]bool,
) bool {
	// If initiator is not in eligible set, they can't confirm
	return !eligibleUserIDs[workflow.InitiatorID]
}

func isInvalidAction(err error) bool {
	return errs.IsAnyError(
		err,
		ErrConnectSystemNoPrimaryKey,
		ErrNotAllSystemsConnected,
		ErrAlreadyPrimaryKey,
		ErrUpdateNonBYOKKeyStatus,
		ErrPrimaryKeyDisabled,
		ErrUnsuportedWorkflow,
	)
}

// transformCheckWorkflowError checks the returned error from validate
// If it's an error created by invalid action set it in status and don't return an error
// Otherwise throw an error that will create a non 2xx HTTP Code
func transformCheckWorkflowError(status WorkflowStatus, err error) (WorkflowStatus, error) {
	if !status.CanCreate && status.Exists {
		status.ErrDetails = ErrOngoingWorkflowExist
		return status, nil
	}

	if err == nil {
		return status, nil
	}

	if isInvalidAction(err) {
		status.ErrDetails = err
		status.CanCreate = false
		return status, nil
	}

	return status, errs.Wrap(ErrCheckWorkflow, err)
}

//nolint:funlen
func (w *WorkflowManager) checkWorkflow(ctx context.Context,
	workflow *model.Workflow,
	enabled bool,
) (WorkflowStatus, error) {
	// If workflow is disabled, all others are false
	if !enabled {
		return WorkflowStatus{
			Enabled: false,
		}, nil
	}

	isSupported := w.isSupportedWorkflow(workflow)
	if !isSupported {
		return WorkflowStatus{
			Enabled:   enabled,
			Valid:     false,
			CanCreate: false,
		}, ErrUnsuportedWorkflow
	}

	isValid, err := w.validateWorkflow(ctx, workflow)
	if err != nil {
		return WorkflowStatus{
			Enabled:   enabled,
			Valid:     isValid,
			CanCreate: false,
		}, err
	}

	exists, err := w.checkOngoingWorkflowForArtifact(ctx, workflow)
	if err != nil {
		return WorkflowStatus{
			Enabled:   enabled,
			Exists:    exists,
			Valid:     isValid,
			CanCreate: false,
		}, err
	}

	// If workflow already exists or is invalid, cannot create
	canCreate := !exists && isValid
	if !canCreate {
		return WorkflowStatus{
			Enabled:   enabled,
			Exists:    exists,
			Valid:     isValid,
			CanCreate: false,
		}, nil
	}

	// Validate approver count
	minimumApprovals := w.getMinimumApprovals(ctx)
	canCreate, errDetails := w.validateApproverCount(ctx, workflow, minimumApprovals)
	if errDetails != nil && !errors.Is(errDetails, wf.ErrWorkflowGroupNotSufficientMembers) {
		return WorkflowStatus{
			Enabled:   enabled,
			Exists:    exists,
			Valid:     isValid,
			CanCreate: false,
		}, errDetails
	}

	return WorkflowStatus{
		Enabled:    enabled,
		Exists:     exists,
		Valid:      isValid,
		CanCreate:  canCreate,
		ErrDetails: errDetails,
	}, nil
}

// isSupportedWorkflow checks if the API supports workflow actionType on a certain artifactType
func (w *WorkflowManager) isSupportedWorkflow(workflow *model.Workflow) bool {
	switch workflow.ArtifactType {
	case model.WorkflowArtifactTypeKey:
		switch workflow.ActionType {
		case model.WorkflowActionTypeUpdateState, model.WorkflowActionTypeDelete:
			return true
		default:
			return false
		}
	case model.WorkflowArtifactTypeSystem:
		switch workflow.ActionType {
		case model.WorkflowActionTypeLink, model.WorkflowActionTypeUnlink, model.WorkflowActionTypeSwitch:
			return true
		default:
			return false
		}
	case model.WorkflowArtifactTypeKeyConfiguration:
		if workflow.ActionType == model.WorkflowActionTypeUpdatePrimary {
			return true
		}
		return false
	default:
		return false
	}
}

//nolint:cyclop,gocognit,funlen
func (w *WorkflowManager) validateWorkflow(ctx context.Context, workflow *model.Workflow) (bool, error) {
	// Always returns at least one key configuration
	keyConfigs, err := w.getKeyConfigurationsFromArtifact(ctx, workflow)
	if err != nil {
		return false, err
	}

	switch {
	case w.isSystemConnect(workflow):
		for _, kc := range keyConfigs {
			if !ptr.IsNotNilUUID(kc.PrimaryKeyID) {
				return false, ErrConnectSystemNoPrimaryKey
			}

			key := &model.Key{ID: *kc.PrimaryKeyID}
			_, err := w.repo.First(ctx, key, *repo.NewQuery())
			if err != nil {
				return false, err
			}

			if key.State != cmkapi.KeyStateENABLED {
				return false, ErrConnectSystemNoPrimaryKey
			}
		}
	case w.isPrimaryKeySwitch(workflow):
		targetKeyID, err := uuid.Parse(workflow.Parameters)
		if err != nil {
			return false, err
		}

		if keyConfigs[0].PrimaryKeyID == nil {
			return false, ErrPrimaryKeyDisabled
		}

		targetKey := &model.Key{ID: targetKeyID}
		_, err = w.repo.First(ctx, targetKey, *repo.NewQuery())
		if err != nil {
			return false, err
		}

		if targetKey.State != cmkapi.KeyStateENABLED {
			return false, ErrPrimaryKeyDisabled
		}

		sourceKey := &model.Key{ID: *keyConfigs[0].PrimaryKeyID}
		_, err = w.repo.First(ctx, sourceKey, *repo.NewQuery())
		if err != nil {
			return false, err
		}

		if sourceKey.State != cmkapi.KeyStateENABLED {
			return false, ErrPrimaryKeyDisabled
		}

		// There is always only one keyconfig from KeyConfig ArtifactType
		if ptr.GetSafeDeref(keyConfigs[0].PrimaryKeyID) == targetKey.ID {
			return false, ErrAlreadyPrimaryKey
		}

		query := *repo.NewQuery().
			Where(repo.NewCompositeKeyGroup(repo.NewCompositeKey().
				Where(repo.KeyConfigIDField, workflow.ArtifactID)))

		err = repo.ProcessInBatch(ctx, w.repo, &query, repo.DefaultLimit, func(systems []*model.System) error {
			for _, s := range systems {
				if s.Status != cmkapi.SystemStatusCONNECTED {
					return ErrNotAllSystemsConnected
				}
			}
			return nil
		})
		if err != nil {
			return false, err
		}
	case w.isKeyStateChange(workflow):
		key := &model.Key{ID: workflow.ArtifactID}
		_, err := w.repo.First(ctx, key, *repo.NewQuery())
		if key.KeyType != cmkapi.KeyTypeBYOK {
			return false, ErrUpdateNonBYOKKeyStatus
		}
		if err != nil {
			return false, err
		}
	default:
	}

	return true, nil
}

func (w *WorkflowManager) isKeyStateChange(workflow *model.Workflow) bool {
	return workflow.ArtifactType == model.WorkflowArtifactTypeKey &&
		workflow.ActionType == model.WorkflowActionTypeUpdateState
}

func (w *WorkflowManager) isPrimaryKeySwitch(workflow *model.Workflow) bool {
	return workflow.ArtifactType == model.WorkflowArtifactTypeKeyConfiguration &&
		workflow.ActionType == model.WorkflowActionTypeUpdatePrimary
}

func (w *WorkflowManager) isSystemConnect(workflow *model.Workflow) bool {
	return workflow.ArtifactType == model.WorkflowArtifactTypeSystem &&
		(workflow.ActionType == model.WorkflowActionTypeLink || workflow.ActionType == model.WorkflowActionTypeSwitch)
}

// getWorkflows retrieves workflows based on the provided query,
// applying access control checks.
// This must not be used in conjunction with preloading approvers.
//
//nolint:funlen
func (w *WorkflowManager) getWorkflows(
	ctx context.Context,
	pagination repo.Pagination,
	query *repo.Query,
) ([]*model.Workflow, int, error) {
	isGroupFiltered, err := w.userManager.NeedsGroupFiltering(ctx, authz.APIActionRead, authz.APIResourceTypeWorkFlow)
	if err != nil {
		return nil, 0, errs.Wrap(ErrGetWorkflowDB, err)
	}

	if isGroupFiltered {
		userIAMGroups, err := cmkContext.ExtractBusinessUserDataGroupsString(ctx)
		if err != nil {
			return nil, 0, errs.Wrap(ErrGetWorkflowDB, err)
		}

		query, err = w.filterByInitiatorOrApprover(ctx, query)
		if err != nil {
			return nil, 0, err
		}

		// Only show workflows if the user is currently in an approver group,
		// or if no approver groups have been assigned yet (INITIAL/FAILED states).
		groupFilterCK := repo.NewCompositeKey().
			Where(fmt.Sprintf("%s.%s", model.Group{}.TableName(), repo.IAMIdField), userIAMGroups).
			Where(fmt.Sprintf("%s.%s", model.WorkflowApproverGroup{}.TableName(), repo.WorkflowIDField), repo.Null)
		groupFilterCK.IsStrict = false

		query = query.Join(
			repo.LeftJoin,
			repo.JoinCondition{
				Table:     &model.Workflow{},
				Field:     repo.IDField,
				JoinField: fmt.Sprintf("%s_%s", repo.WorkflowField, repo.IDField),
				JoinTable: &model.WorkflowApproverGroup{},
			},
		).Join(
			repo.LeftJoin,
			repo.JoinCondition{
				Table:     &model.WorkflowApproverGroup{},
				Field:     repo.GroupIDField,
				JoinTable: &model.Group{},
				JoinField: repo.IDField,
			},
		).Where(repo.NewCompositeKeyGroup(groupFilterCK))
	}

	query = query.SetLimit(pagination.Top).SetOffset(pagination.Skip)

	workflows := []*model.Workflow{}

	err = w.repo.List(ctx, model.Workflow{}, &workflows, *query.GroupBy(
		fmt.Sprintf("%s.%s", model.Workflow{}.TableName(), repo.IDField),
	))
	if err != nil {
		return nil, 0, errs.Wrap(ErrGetWorkflowDB, err)
	}

	count, err := w.repo.Count(
		ctx,
		&model.Workflow{},
		*query.Select(repo.NewSelectField(
			fmt.Sprintf("%s.%s", model.Workflow{}.TableName(), repo.IDField),
			repo.QueryFunction{
				Function: repo.CountFunc,
				Distinct: true,
			},
		)),
	)
	if err != nil {
		return nil, 0, errs.Wrap(ErrGetWorkflowDB, err)
	}

	return workflows, count, nil
}

func (w *WorkflowManager) filterByInitiatorOrApprover(ctx context.Context, query *repo.Query) (*repo.Query, error) {
	iamIdentifier, err := cmkContext.ExtractBusinessUserDataIdentifier(ctx)
	if err != nil {
		return nil, err
	}

	orCK := repo.NewCompositeKey().
		Where(repo.InitiatorIDField, iamIdentifier).
		Where(fmt.Sprintf("%s.%s", model.WorkflowApprover{}.TableName(), repo.UserIDField), iamIdentifier)
	orCK.IsStrict = false

	query = query.Join(
		repo.LeftJoin,
		repo.JoinCondition{
			Table:     &model.Workflow{},
			Field:     repo.IDField,
			JoinField: fmt.Sprintf("%s_%s", repo.WorkflowField, repo.IDField),
			JoinTable: &model.WorkflowApprover{},
		},
	).Where(repo.NewCompositeKeyGroup(orCK))

	return query, nil
}

// resolveActorApproverGroupIDs resolves the actor's current group membership against
// the workflow's approver groups. Returns nil if no approver groups are configured,
// or a slice of matching group IDs (empty slice = actor not in any approver group).
func (w *WorkflowManager) resolveActorApproverGroupIDs(
	ctx context.Context,
	workflow *model.Workflow,
) ([]uuid.UUID, error) {
	groups, err := w.GetWorkflowApproverGroups(ctx, workflow)
	if err != nil {
		return nil, err
	}

	if len(groups) == 0 {
		return nil, nil
	}

	userGroupIDs, err := w.resolveUserGroupIDs(ctx)
	if err != nil {
		return nil, err
	}

	var matching []uuid.UUID
	for _, g := range groups {
		if _, ok := userGroupIDs[g.ID]; ok {
			matching = append(matching, g.ID)
		}
	}

	if matching == nil {
		matching = []uuid.UUID{}
	}

	return matching, nil
}

// resolveUserGroupIDs resolves the caller's current IAM groups to internal group UUIDs.
func (w *WorkflowManager) resolveUserGroupIDs(ctx context.Context) (map[uuid.UUID]struct{}, error) {
	userIAMGroups, err := cmkContext.ExtractBusinessUserDataGroupsString(ctx)
	if err != nil {
		return nil, err
	}

	if len(userIAMGroups) == 0 {
		return make(map[uuid.UUID]struct{}), nil
	}

	ck := repo.NewCompositeKey().Where(repo.IAMIdField, userIAMGroups)
	query := repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)).SetLimit(len(userIAMGroups))

	var groups []*model.Group
	err = w.repo.List(ctx, model.Group{}, &groups, *query)
	if err != nil {
		return nil, err
	}

	result := make(map[uuid.UUID]struct{}, len(groups))
	for _, g := range groups {
		result[g.ID] = struct{}{}
	}

	return result, nil
}

// addApprovers adds the specified approvers to the workflow
// and transitions the workflow to the next state.
func (w *WorkflowManager) getWorkflowLifecycle(
	ctx context.Context,
	workflow *model.Workflow,
	userID string,
) (*wf.Lifecycle, error) {
	return w.getWorkflowLifecycleWithEligibility(ctx, workflow, userID, nil)
}

// getWorkflowLifecycleWithEligibility creates a workflow lifecycle with optional eligible approver filtering
func (w *WorkflowManager) getWorkflowLifecycleWithEligibility(
	ctx context.Context,
	workflow *model.Workflow,
	userID string,
	eligibleApproverIDs map[string]bool,
) (*wf.Lifecycle, error) {
	// Use workflow's persisted MinimumApprovalCount if set (snapshot at creation time)
	// Otherwise fallback to current tenant config for backward compatibility with old workflows
	minimumApprovals := workflow.MinimumApprovalCount
	if minimumApprovals == 0 {
		// Fallback for workflows created before this field existed
		workflowConfig, err := w.WorkflowConfig(ctx)
		if err != nil {
			return nil, oops.Join(ErrGetWorkflowConfig, err)
		}
		minimumApprovals = workflowConfig.MinimumApprovals
	}

	workflowLifecycle := wf.NewLifecycle(
		workflow, w.keyManager, w.keyConfigurationManager, w.systemManager, w.repo, userID,
		minimumApprovals,
	)

	// Set eligible approver IDs if provided (for accurate vote counting)
	workflowLifecycle.EligibleApproverIDs = eligibleApproverIDs

	return workflowLifecycle, nil
}

// addApproversAndGroupAssociations adds the specified approvers to the workflow
// and associates the approver groups with the workflow.
// Then, it transitions the workflow to the next state.
// This is wrapped in a transaction to ensure that DB state is consistent
func (w *WorkflowManager) addApproversAndGroupAssociations(
	ctx context.Context,
	userID string,
	workflow *model.Workflow,
	approvers []*model.WorkflowApprover,
	groups []*model.Group,
) error {
	err := w.repo.Transaction(ctx, func(ctx context.Context) error {
		workflowLifecycle, err := w.getWorkflowLifecycle(ctx, workflow, userID)
		if err != nil {
			return err
		}

		// Add each approver to the workflow
		for _, approver := range approvers {
			approver.WorkflowID = workflow.ID

			err = w.repo.Set(ctx, approver)
			if err != nil {
				return errs.Wrap(ErrAddApproversDB, err)
			}
		}

		for _, g := range groups {
			err := w.repo.Set(ctx, model.WorkflowApproverGroup{
				ID:         uuid.New(),
				WorkflowID: workflow.ID,
				GroupID:    g.ID,
			})
			if err != nil {
				return err
			}
		}

		// Then, apply the transition to next state
		err = workflowLifecycle.ValidateAndApplyTransition(ctx, wf.TransitionCreate)
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
	ck := repo.NewCompositeKey().
		Where(fmt.Sprintf("%s_%s", repo.ArtifactField, repo.TypeField), workflow.ArtifactType).
		Where(fmt.Sprintf("%s_%s", repo.ArtifactField, repo.IDField), workflow.ArtifactID).
		Where(repo.StateField, model.WorkflowNonTerminalStates)

	count, err := w.repo.Count(ctx, &model.Workflow{}, *repo.NewQuery().Where(repo.NewCompositeKeyGroup(ck)))
	if err != nil {
		return false, errs.Wrap(ErrCheckOngoingWorkflow, err)
	}

	return count > 0, nil
}

// updateApproverDecisionAndApplyTransition updates the approver
// decision and applies the transition to the wf.
// This is wrapped in a transaction to ensure that DB state is
// consistent in case of errors.
//
//nolint:cyclop
func (w *WorkflowManager) applyTransition(
	ctx context.Context,
	userID string,
	workflow *model.Workflow,
	transition wf.Transition,
) error {
	var capturedEligibleApproverIDs map[string]bool

	err := w.repo.Transaction(ctx, func(ctx context.Context) error {
		// For approve/reject transitions, fetch eligible approvers BEFORE creating lifecycle
		eligibleApproverIDs := w.fetchEligibilityForVote(ctx, workflow, transition)
		capturedEligibleApproverIDs = eligibleApproverIDs

		// Create lifecycle with eligibility filtering (if available)
		workflowLifecycle, err := w.getWorkflowLifecycleWithEligibility(ctx, workflow, userID, eligibleApproverIDs)
		if err != nil {
			return err
		}

		// Resolve actor's approver group membership for the lifecycle guard
		workflowLifecycle.ActorApproverGroupIDs, err = w.resolveActorApproverGroupIDs(ctx, workflow)
		if err != nil {
			return err
		}

		validateErr := workflowLifecycle.ValidateActor(ctx, transition)
		if validateErr != nil {
			return errs.Wrap(ErrValidateActor, validateErr)
		}

		// For approve/reject transitions, fetch approver and check eligibility once
		var approver *model.WorkflowApprover
		if transition == wf.TransitionApprove || transition == wf.TransitionReject {
			var err error
			approver, err = w.fetchAndValidateApprover(ctx, workflow, userID)
			if err != nil {
				return err
			}
		}

		// Update decision in database based on transition type
		txErr := w.recordTransitionInDB(ctx, transition, approver)
		if txErr != nil {
			return txErr
		}

		// Apply the transition - the state machine now uses eligibility-filtered approvers
		transitionErr := workflowLifecycle.ValidateAndApplyTransition(ctx, transition)
		if transitionErr != nil {
			return errs.Wrap(ErrApplyTransition, transitionErr)
		}

		err = w.HandleTerminalWorkflow(ctx, workflow)
		if err != nil {
			return err
		}

		// After vote, check if we should auto-reject due to mathematical impossibility
		w.attemptAutoReject(ctx, workflow, transition, capturedEligibleApproverIDs)

		return nil
	})
	if err != nil {
		return errs.Wrap(ErrInDBTransaction, err)
	}

	return nil
}

// fetchEligibilityForVote fetches eligible approver IDs for approve/reject transitions.
// Returns nil if not a voting transition or if eligibility check fails.
func (w *WorkflowManager) fetchEligibilityForVote(
	ctx context.Context,
	workflow *model.Workflow,
	transition wf.Transition,
) map[string]bool {
	// Only fetch eligibility for approve/reject transitions in WAIT_APPROVAL state
	if workflow.State != model.WorkflowStateWaitApproval {
		return nil
	}
	if transition != wf.TransitionApprove && transition != wf.TransitionReject {
		return nil
	}

	eligibleApproverIDs, err := w.getEligibleUserIDsForWorkflow(ctx, workflow)
	if err != nil {
		// Log warning but don't fail the vote - eligibility check is best-effort
		log.Warn(ctx, "Failed to check approver eligibility for vote",
			slog.Any("error", err), slog.String("workflowID", workflow.ID.String()))
		return nil // Fall back to counting all approvers
	}

	return eligibleApproverIDs
}

// recordTransitionInDB records the transition in the database (for approve/reject).
func (w *WorkflowManager) recordTransitionInDB(
	ctx context.Context,
	transition wf.Transition,
	approver *model.WorkflowApprover,
) error {
	switch transition {
	case wf.TransitionApprove:
		return w.updateApproverDecision(ctx, approver, true)
	case wf.TransitionReject:
		return w.updateApproverDecision(ctx, approver, false)
	case wf.TransitionCreate, wf.TransitionExpire,
		wf.TransitionExecute, wf.TransitionFail:
		return ErrWorkflowCannotTransitionDB
	case wf.TransitionConfirm, wf.TransitionRevoke:
		return nil
	default:
		return nil
	}
}

// attemptAutoReject checks if workflow should auto-reject after a vote.
// This handles cases where approval becomes mathematically impossible.
func (w *WorkflowManager) attemptAutoReject(
	ctx context.Context,
	workflow *model.Workflow,
	transition wf.Transition,
	eligibleApproverIDs map[string]bool,
) {
	// Only check after approve/reject votes in WAIT_APPROVAL state
	if workflow.State != model.WorkflowStateWaitApproval {
		return
	}
	if transition != wf.TransitionApprove && transition != wf.TransitionReject {
		return
	}

	internalLifecycle, err := w.getWorkflowLifecycleWithEligibility(ctx, workflow,
		"", eligibleApproverIDs)
	if err != nil {
		return
	}

	// ApplyTransition skips actor validation — authority is implicit
	// because this is called only after a vote has already been validated.
	rejectErr := internalLifecycle.ApplyTransition(ctx, wf.TransitionReject)
	if rejectErr != nil {
		log.Warn(ctx, "Failed to auto-reject workflow after vote",
			slog.Any("error", rejectErr), slog.String("workflowID", workflow.ID.String()))
	}
}

// fetchAndValidateApprover fetches the approver and validates eligibility for approve/reject transitions.
func (w *WorkflowManager) fetchAndValidateApprover(
	ctx context.Context,
	workflow *model.Workflow,
	userID string,
) (*model.WorkflowApprover, error) {
	ck := repo.NewCompositeKey().
		Where(fmt.Sprintf("%s_%s", repo.UserField, repo.IDField), userID).
		Where(fmt.Sprintf("%s_%s", repo.WorkflowField, repo.IDField), workflow.ID)

	approver := &model.WorkflowApprover{}
	_, err := w.repo.First(ctx, approver, *repo.NewQuery().
		Where(repo.NewCompositeKeyGroup(ck)))
	if err != nil {
		return nil, errs.Wrap(ErrCheckWorkflowEligibility, err)
	}

	// Check eligibility with already-fetched approver
	eligibilityErr := w.checkApproverEligibility(ctx, workflow, userID, approver)
	if eligibilityErr != nil {
		return nil, eligibilityErr
	}

	return approver, nil
}

// updateApproverDecision updates the decision using an already-fetched approver object.
func (w *WorkflowManager) updateApproverDecision(
	ctx context.Context,
	approver *model.WorkflowApprover,
	approved bool,
) error {
	err := w.repo.Transaction(ctx, func(ctx context.Context) error {
		approver.Approved = sql.NullBool{Bool: approved, Valid: true}

		_, err := w.repo.Patch(ctx, approver, *repo.NewQuery())
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

func (w *WorkflowManager) checkPermissionToCreateWorkflow(
	ctx context.Context,
	workflow *model.Workflow,
) (bool, error) {
	switch workflow.ArtifactType {
	case model.WorkflowArtifactTypeKeyConfiguration, model.WorkflowArtifactTypeSystem, model.WorkflowArtifactTypeKey:
		_, err := w.getKeyConfigurationsFromArtifact(ctx, workflow)
		if errors.Is(err, ErrKeyConfigurationNotAllowed) || errors.Is(err, repo.ErrNotFound) {
			return false, nil
		}

		if err != nil {
			return false, err
		}

		userinfo, err := w.userManager.GetBusinessUserInfo(ctx)
		if err != nil {
			return false, err
		}

		if userinfo.Role == string(constants.TenantAuditorRole) {
			return false, nil
		}

		return true, nil
	}
	return false, errs.Wrapf(ErrGetKeyConfigFromArtifact,
		"unsupported artifact type: "+workflow.ArtifactType.String())
}

func (w *WorkflowManager) getKeyConfigurationsFromArtifact(
	ctx context.Context,
	workflow *model.Workflow,
) ([]*model.KeyConfiguration, error) {
	var keyConfigs []*model.KeyConfiguration

	switch workflow.ArtifactType {
	case model.WorkflowArtifactTypeKeyConfiguration:
		keyConfig, err := w.keyConfigurationManager.GetKeyConfigurationByID(ctx, workflow.ArtifactID)
		if err != nil {
			return nil, errs.Wrap(ErrGetKeyConfigFromArtifact, err)
		}

		keyConfigs = append(keyConfigs, keyConfig)

	case model.WorkflowArtifactTypeSystem:
		keyConfigsFromSystems, err := w.getKeyConfigFromSystem(ctx, workflow)
		if err != nil {
			return nil, err
		}

		keyConfigs = append(keyConfigs, keyConfigsFromSystems...)

	case model.WorkflowArtifactTypeKey:
		keyConfig, err := w.getKeyConfigFromKey(ctx, workflow)
		if err != nil {
			return nil, err
		}

		keyConfigs = append(keyConfigs, keyConfig)

	default:
		return nil, errs.Wrapf(ErrGetKeyConfigFromArtifact,
			"unsupported artifact type: "+workflow.ArtifactType.String())
	}

	return keyConfigs, nil
}

func (w *WorkflowManager) getKeyConfigFromSystem(
	ctx context.Context,
	workflow *model.Workflow,
) ([]*model.KeyConfiguration, error) {
	var keyConfigs []*model.KeyConfiguration
	// If action type is UNLINK or SWITCH, we need to get the current key configuration from artifact ID
	switch workflow.ActionType {
	case model.WorkflowActionTypeUnlink, model.WorkflowActionTypeSwitch:
		system, err := w.systemManager.GetSystemByID(ctx, workflow.ArtifactID)
		if err != nil {
			return nil, errs.Wrap(ErrGetKeyConfigFromArtifact, err)
		}

		keyConfig, err := w.keyConfigurationManager.GetKeyConfigurationByID(ctx, *system.KeyConfigurationID)
		if err != nil {
			return nil, errs.Wrap(ErrGetKeyConfigFromArtifact, err)
		}

		keyConfigs = append(keyConfigs, keyConfig)
	default:
		// other action types do not source a key configuration from the artifact
	}

	// If action type is LINK or SWITCH, we need to get the target key configuration from parameters
	switch workflow.ActionType {
	case model.WorkflowActionTypeLink, model.WorkflowActionTypeSwitch:
		keyConfigID, err := uuid.Parse(workflow.Parameters)
		if err != nil {
			return nil, errs.Wrapf(ErrGetKeyConfigFromArtifact,
				fmt.Sprintf("invalid key configuration ID in workflow parameters: %v", err))
		}

		keyConfig, err := w.keyConfigurationManager.GetKeyConfigurationByID(ctx, keyConfigID)
		if err != nil {
			return nil, errs.Wrap(ErrGetKeyConfigFromArtifact, err)
		}

		keyConfigs = append(keyConfigs, keyConfig)
	default:
		// other action types do not source a target key configuration from parameters
	}

	return keyConfigs, nil
}

func (w *WorkflowManager) getKeyConfigFromKey(
	ctx context.Context,
	workflow *model.Workflow,
) (*model.KeyConfiguration, error) {
	key, err := w.keyManager.Get(ctx, workflow.ArtifactID)
	if err != nil {
		return nil, errs.Wrap(ErrGetKeyConfigFromArtifact, err)
	}

	keyConfig, err := w.keyConfigurationManager.GetKeyConfigurationByID(ctx, key.KeyConfigurationID)
	if err != nil {
		return nil, errs.Wrap(ErrGetKeyConfigFromArtifact, err)
	}

	return keyConfig, nil
}

//nolint:cyclop,funlen
func (w *WorkflowManager) getApproversAndGroupsFromKeyConfigs(
	ctx context.Context,
	workflow *model.Workflow,
	keyConfigs []*model.KeyConfiguration,
) ([]*model.WorkflowApprover, []*model.Group, error) {
	idm, err := w.groupManager.GetIdentityManagementPlugin()
	if err != nil {
		return nil, nil, err
	}

	// Use a map to avoid duplicate approvers and groups
	approverMap := make(map[string]model.WorkflowApprover)
	groupMap := make(map[string]model.Group)

	for _, keyConfig := range keyConfigs {
		group := keyConfig.AdminGroup
		if group.ID == uuid.Nil {
			// GetKeyConfigurationByID should have already loaded the admin group
			return nil, nil, errs.Wrapf(ErrAutoAssignApprover, "admin group not loaded for key configuration")
		}

		groupMap[group.IAMIdentifier] = group

		authCtx, _ := cmkContext.ExtractBusinessUserDataAuthContext(ctx)
		if authCtx == nil {
			authCtx = map[string]string{}
		}

		idmGroup, err := idm.GetGroup(ctx, &identitymanagement.GetGroupRequest{
			GroupName:   group.IAMIdentifier,
			AuthContext: identitymanagement.AuthContext{Data: authCtx},
		})
		if err != nil {
			return nil, nil, errs.Wrap(ErrAutoAssignApprover, err)
		}

		groupUsers, err := idm.ListGroupUsers(ctx, &identitymanagement.ListGroupUsersRequest{
			GroupID:     idmGroup.Group.ID,
			AuthContext: identitymanagement.AuthContext{Data: authCtx},
		})
		if err != nil {
			return nil, nil, errs.Wrap(ErrAutoAssignApprover, err)
		}

		for _, user := range groupUsers.Users {
			userID := user.ID

			if userID == workflow.InitiatorID {
				continue // Skip initiator
			}

			approverMap[userID] = model.WorkflowApprover{
				UserID: userID,
			}
		}
	}

	approvers := make([]*model.WorkflowApprover, 0, len(approverMap))
	for _, approver := range approverMap {
		approvers = append(approvers, &approver)
	}

	groups := make([]*model.Group, 0, len(groupMap))
	for _, group := range groupMap {
		groups = append(groups, &group)
	}

	return approvers, groups, nil
}

// getMinimumApprovals retrieves the minimum approval count from tenant config
func (w *WorkflowManager) getMinimumApprovals(ctx context.Context) int {
	config, err := w.tenantConfigManager.GetWorkflowConfig(ctx)
	if err != nil || config == nil {
		return constants.DefaultMinimumApprovalCount // 2
	}
	if config.MinimumApprovals > 0 {
		return config.MinimumApprovals
	}
	return constants.DefaultMinimumApprovalCount
}

// queryGroupMembersFromIAM queries IAM to get all current members across multiple groups.
// Returns a set of user IDs currently in any of the groups. Deleted/non-existent groups
// are skipped (treated as having zero members). Returns error only if IAM queries fail.
func (w *WorkflowManager) queryGroupMembersFromIAM(
	ctx context.Context,
	idm identitymanagement.IdentityManagement,
	authCtx map[string]string,
	groups []*model.Group,
) (map[string]bool, error) {
	eligibleUserIDs := make(map[string]bool)

	for _, group := range groups {
		groupID := group.ID
		// Get group from DB
		group, err := w.groupManager.GetGroupByID(ctx, groupID)
		if err != nil {
			if errs.IsAnyError(err, repo.ErrNotFound, ErrGetGroups) {
				log.Warn(ctx, "skipping deleted/non-existent group in eligibility check",
					slog.String("groupID", groupID.String()))
				continue
			}
			return nil, errs.Wrap(ErrCheckWorkflowEligibility, err)
		}

		// Get group from IAM
		idmGroup, err := idm.GetGroup(ctx, &identitymanagement.GetGroupRequest{
			GroupName:   group.IAMIdentifier,
			AuthContext: identitymanagement.AuthContext{Data: authCtx},
		})
		if err != nil {
			return nil, errs.Wrap(ErrCheckWorkflowEligibility, err)
		}

		// List users in group
		groupUsers, err := idm.ListGroupUsers(ctx, &identitymanagement.ListGroupUsersRequest{
			GroupID:     idmGroup.Group.ID,
			AuthContext: identitymanagement.AuthContext{Data: authCtx},
		})
		if err != nil {
			return nil, errs.Wrap(ErrCheckWorkflowEligibility, err)
		}

		// Add all user IDs to eligible set
		for _, user := range groupUsers.Users {
			eligibleUserIDs[user.ID] = true
		}
	}

	return eligibleUserIDs, nil
}

// checkApproverEligibility validates that an approver is still eligible to vote
func (w *WorkflowManager) checkApproverEligibility(
	ctx context.Context,
	workflow *model.Workflow,
	userID string,
	approver *model.WorkflowApprover,
) error {
	groups, err := w.GetWorkflowApproverGroups(ctx, workflow)
	if err != nil {
		return errs.Wrap(ErrCheckWorkflowEligibility, err)
	}

	if len(groups) == 0 {
		return nil // No group restrictions
	}

	// Get identity management plugin
	idm, err := w.groupManager.GetIdentityManagementPlugin()
	if err != nil {
		return errs.Wrap(ErrCheckWorkflowEligibility, err)
	}

	// Get auth context
	authCtx, _ := cmkContext.ExtractBusinessUserDataAuthContext(ctx)
	if authCtx == nil {
		authCtx = map[string]string{}
	}

	// Check if user is still in any of the groups
	eligibleUserIDs, err := w.queryGroupMembersFromIAM(ctx, idm, authCtx, groups)
	if err != nil {
		return err
	}

	// If user is not eligible
	if !eligibleUserIDs[userID] {
		// If they already voted, their existing vote counts but they can't change it
		if approver.Approved.Valid {
			return wf.NewApproverNoLongerEligibleError(userID)
		}
		// If they haven't voted yet, they can't vote now
		return wf.NewApproverNoLongerEligibleError(userID)
	}

	return nil
}

func (w *WorkflowManager) createAutoAssignApproversAsyncTask(
	ctx context.Context,
	workflow model.Workflow,
) error {
	if w.asyncClient != nil {
		payload := asyncUtils.NewTaskPayload(ctx, []byte(workflow.ID.String()))

		payloadBytes, err := payload.ToBytes()
		if err != nil {
			return errs.Wrap(ErrCreateApproverAssignTask, err)
		}

		task := asynq.NewTask(config.TypeWorkflowAutoAssign, payloadBytes)

		info, err := w.asyncClient.Enqueue(task)
		if err != nil {
			return errs.Wrap(ErrCreateApproverAssignTask, err)
		}

		log.Info(ctx, "Enqueued workflow auto-assign approvers task",
			slog.String("taskId", info.ID),
			slog.String("workflowId", workflow.ID.String()))
	} else {
		log.Warn(ctx, "async client is not initialized, skipping workflow creation task enqueue")
	}

	return nil
}

func (w *WorkflowManager) createWorkflowTransitionNotificationTask(
	ctx context.Context,
	workflow model.Workflow,
	transition wf.Transition,
	recipients []string,
) error {
	if w.asyncClient == nil {
		log.Warn(ctx, "async client is not initialized, skipping workflow transition task enqueue")
		return nil
	}

	tenant, err := repo.GetTenant(ctx, w.repo)
	if err != nil {
		return err
	}

	idm, err := w.svcRegistry.IdentityManagement()
	if err != nil {
		return err
	}

	n, err := notifier.New(w.cfg, idm)
	if err != nil {
		log.Error(ctx, "Create notifier failed", err)
		return nil
	}

	data := wn.NotificationData{
		Tenant:     *tenant,
		Workflow:   workflow,
		Transition: transition,
	}

	if len(recipients) == 0 {
		log.Warn(ctx, "transition recipients is empty, skipping sending notification")
		return nil
	}

	task, err := n.Workflow().CreateTask(ctx, data, recipients)
	if err != nil {
		log.Error(ctx, "Create workflow transition task failed", err)
		return err
	}

	if task == nil {
		log.Info(ctx, "No workflow transition task created, skipping enqueue")
		return nil
	}

	_, err = w.asyncClient.Enqueue(task)
	if err != nil {
		log.Error(ctx, "Enqueue workflow transition task failed", err)
		return err
	}

	return nil
}

func (w *WorkflowManager) populateArtifact(
	ctx context.Context,
	workflow *model.Workflow,
) error {
	switch workflow.ArtifactType {
	case model.WorkflowArtifactTypeKey:
		key, err := w.keyManager.Get(ctx, workflow.ArtifactID)
		if err != nil {
			return err
		}
		workflow.ArtifactName = ptr.PointTo(key.Name)

	case model.WorkflowArtifactTypeKeyConfiguration:
		keyConfig, err := w.keyConfigurationManager.GetKeyConfigurationByID(ctx, workflow.ArtifactID)
		if err != nil {
			return err
		}
		workflow.ArtifactName = ptr.PointTo(keyConfig.Name)

	case model.WorkflowArtifactTypeSystem:
		system, err := w.systemManager.GetSystemByID(ctx, workflow.ArtifactID)
		if err != nil {
			return err
		}
		workflow.ArtifactName = w.getWorkflowSystemArtifactName(system)

	default:
		// empty
	}

	return nil
}

// populateParametersResource resolves the resource info for the workflow
// parameters payload.
//
//nolint:cyclop
func (w *WorkflowManager) populateParametersResource(
	ctx context.Context,
	workflow *model.Workflow,
) error {
	switch workflow.ArtifactType {
	case model.WorkflowArtifactTypeKeyConfiguration:
		if workflow.ActionType == model.WorkflowActionTypeUpdatePrimary {
			keyID, err := uuid.Parse(workflow.Parameters)
			if err != nil {
				return err
			}

			key, err := w.keyManager.Get(ctx, keyID)
			if err != nil {
				return err
			}

			workflow.ParametersResourceType = ptr.PointTo(model.WorkflowParametersResourceTypeKey)
			workflow.ParametersResourceName = ptr.PointTo(key.Name)
		}

	case model.WorkflowArtifactTypeSystem:
		switch workflow.ActionType {
		case model.WorkflowActionTypeLink, model.WorkflowActionTypeSwitch:
			keyConfigID, err := uuid.Parse(workflow.Parameters)
			if err != nil {
				return err
			}

			keyConfig, err := w.keyConfigurationManager.GetKeyConfigurationByID(ctx, keyConfigID)
			if err != nil {
				return err
			}

			workflow.ParametersResourceType = ptr.PointTo(model.WorkflowParametersResourceTypeKeyConfiguration)
			workflow.ParametersResourceName = ptr.PointTo(keyConfig.Name)
		default:
			// other system action types do not populate the parameters resource
		}
	default:
		// empty
	}

	return nil
}

func (w *WorkflowManager) getWorkflowSystemArtifactName(system *model.System) *string {
	var nameFromProperties *string
	// Look for any optional properties that has displayName "Name"
	for propertyName, definition := range w.systemManager.ContextModelsCfg.OptionalProperties {
		if strings.ToUpper(definition.DisplayName) == WorkflowSystemArtifactPropertyKey {
			if val, ok := system.Properties[propertyName]; ok {
				nameFromProperties = ptr.PointTo(val)
			}
			break
		}
	}

	// Set artifact name from properties if found. Fall back to system identifier otherwise.
	if nameFromProperties == nil {
		nameFromProperties = ptr.PointTo(system.Identifier)
	}

	return nameFromProperties
}
