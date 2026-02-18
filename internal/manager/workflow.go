package manager

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/samber/oops"

	idmv1 "github.com/openkcm/plugin-sdk/proto/plugin/identity_management/v1"

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

type Workflow interface {
	CheckWorkflow(ctx context.Context, workflow *model.Workflow) (WorkflowStatus, error)
	GetWorkflows(ctx context.Context, params repo.QueryMapper) ([]*model.Workflow, int, error)
	CreateWorkflow(ctx context.Context, workflow *model.Workflow) (*model.Workflow, error)
	GetWorkflowByID(ctx context.Context, workflowID uuid.UUID) (*model.Workflow, error)
	ListWorkflowApprovers(
		ctx context.Context,
		id uuid.UUID,
		decisionMade bool,
		pagination repo.Pagination,
	) ([]*model.WorkflowApprover, int, error)
	GetWorkflowAvailableTransitions(ctx context.Context, workflow *model.Workflow) ([]wf.Transition, error)
	GetWorkflowApprovalSummary(ctx context.Context, workflow *model.Workflow) (*wf.ApprovalSummary, error)
	TransitionWorkflow(
		ctx context.Context,
		workflowID uuid.UUID,
		transition wf.Transition,
	) (*model.Workflow, error)
	WorkflowConfig(ctx context.Context) (*model.WorkflowConfig, error)
	IsWorkflowRequired(ctx context.Context) (bool, error)
	CleanupTerminalWorkflows(ctx context.Context) error
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
	cfg                     *config.Config
}

func NewWorkflowManager(
	repository repo.Repo,
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
	State                  string
	ArtifactType           string
	ArtifactID             uuid.UUID
	ArtifactName           string
	ParametersResourceName string
	ActionType             string
	Skip                   int
	Top                    int
	Count                  bool
}

var _ repo.QueryMapper = (*WorkflowFilter)(nil) // Assert interface impl

func NewWorkflowFilterFromOData(queryMapper odata.QueryOdataMapper) (*WorkflowFilter, error) {
	skipPtr, topPtr, countPtr := queryMapper.GetPaging()
	skip := ptr.GetIntOrDefault(skipPtr, constants.DefaultSkip)
	top := ptr.GetIntOrDefault(topPtr, constants.DefaultTop)
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

	return &WorkflowFilter{
		State:                  state,
		ArtifactType:           artifactType,
		ArtifactID:             artifactID,
		ArtifactName:           artifactName,
		ParametersResourceName: parametersResourceName,
		ActionType:             actionType,
		Skip:                   skip,
		Top:                    top,
		Count:                  count,
	}, nil
}

var approverJoinCond = repo.JoinCondition{
	Table:     &model.Workflow{},
	Field:     repo.IDField,
	JoinField: fmt.Sprintf("%s_%s", repo.WorkflowField, repo.IDField),
	JoinTable: &model.WorkflowApprover{},
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
		val = w.State
	case repo.ArtifactTypeField:
		val = w.ArtifactType
	case repo.ActionTypeField:
		val = w.ActionType
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
	workflow.State = wf.StateInitial.String()

	status, err := w.CheckWorkflow(ctx, workflow)
	if err != nil {
		return nil, err
	}
	if status.Exists {
		return nil, ErrOngoingWorkflowExist
	}
	if status.ErrDetails != nil {
		return nil, err
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

func (w *WorkflowManager) GetWorkflowByID(ctx context.Context, workflowID uuid.UUID) (*model.Workflow, error) {
	query := repo.NewQuery()
	ck := repo.NewCompositeKey()
	ck = ck.Where(repo.IDField, workflowID)
	query = query.Where(repo.NewCompositeKeyGroup(ck))

	workflows, _, err := w.getWorkflows(ctx, repo.Pagination{}, query)
	if err != nil {
		return nil, err
	}

	if len(workflows) == 0 {
		return nil, errs.Wrap(ErrWorkflowNotAllowed, err)
	}

	return workflows[0], nil
}

// ListWorkflowApprovers retrieves a paginated list of approvers for a given workflow ID.
// Returns a slice of WorkflowApprover, the total count, and an error if any occurs.
func (w *WorkflowManager) ListWorkflowApprovers(
	ctx context.Context,
	id uuid.UUID,
	decisionMade bool,
	pagination repo.Pagination,
) ([]*model.WorkflowApprover, int, error) {
	_, err := w.GetWorkflowByID(ctx, id)
	if err != nil {
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

	approverValues := make([]model.WorkflowApprover, len(approvers))
	for i, approver := range approvers {
		if approver != nil {
			approverValues[i] = *approver
		}
	}

	approverUserNames := wf.GetApproverUserNames(approverValues)

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
	clientData, err := cmkContext.ExtractClientData(ctx)
	if err != nil {
		return nil, err
	}

	userID := clientData.Identifier

	workflowLifecycle, err := w.getWorkflowLifecycle(ctx, workflow, userID)
	if err != nil {
		return nil, err
	}

	// For now assume that only business users can perform transitions.
	// When other types of users are supported, this logic will need to be updated.
	transitions := workflowLifecycle.AvailableBusinessUserTransitions(ctx)

	return transitions, nil
}

func (w *WorkflowManager) GetWorkflowApprovalSummary(
	ctx context.Context,
	workflow *model.Workflow,
) (*wf.ApprovalSummary, error) {
	workflowLifecycle, err := w.getWorkflowLifecycle(ctx, workflow, wf.SystemUserID) // Use system user for summary
	if err != nil {
		return nil, err
	}

	summary, err := workflowLifecycle.GetApprovalSummary(ctx)
	if err != nil {
		return nil, err
	}

	return summary, nil
}

func (w *WorkflowManager) TransitionWorkflow(
	ctx context.Context,
	workflowID uuid.UUID,
	transition wf.Transition,
) (*model.Workflow, error) {
	clientData, err := cmkContext.ExtractClientData(ctx)
	if err != nil {
		return nil, err
	}

	userID := clientData.Identifier

	workflow := &model.Workflow{ID: workflowID}

	_, err = w.repo.First(ctx, workflow, *repo.NewQuery().Preload(repo.Preload{"Approvers"}))
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

	recipients := wf.GetNotificationRecipients(*workflow, transition)

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
		Where(repo.StateField, wf.TerminalStates).
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

// transformCheckWorkflowError checks the returned error from validate
// If it's an error created by invalid action set it in status and don't return an error
// Otherwise throw an error that will create a non 2xx HTTP Code
func transformCheckWorkflowError(status WorkflowStatus, err error) (WorkflowStatus, error) {
	if err == nil {
		return status, nil
	}

	if errors.Is(err, ErrConnectSystemNoPrimaryKey) || errors.Is(err, ErrCheckOngoingWorkflow) {
		status.ErrDetails = err
		status.CanCreate = false
		return status, nil
	}
	return status, errs.Wrap(ErrCheckWorkflow, err)
}

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
	return WorkflowStatus{
		Enabled:   enabled,
		Exists:    exists,
		Valid:     isValid,
		CanCreate: !exists && isValid,
	}, nil
}

func (w *WorkflowManager) validateWorkflow(ctx context.Context, workflow *model.Workflow) (bool, error) {
	keyConfigs, err := w.getKeyConfigurationsFromArtifact(ctx, workflow)
	if err != nil {
		return false, err
	}

	if w.isSystemConnect(workflow) {
		for _, kc := range keyConfigs {
			if !ptr.IsNotNilUUID(kc.PrimaryKeyID) {
				return false, ErrConnectSystemNoPrimaryKey
			}

			key := &model.Key{ID: *kc.PrimaryKeyID}
			_, err := w.repo.First(ctx, key, *repo.NewQuery())
			if err != nil {
				return false, err
			}

			if key.State != string(cmkapi.KeyStateENABLED) {
				return false, ErrConnectSystemNoPrimaryKey
			}
		}
	}

	return true, nil
}

func (w *WorkflowManager) isSystemConnect(workflow *model.Workflow) bool {
	return workflow.ArtifactType == wf.ArtifactTypeSystem.String() &&
		(workflow.ActionType == string(wf.ActionTypeLink) || workflow.ActionType == string(wf.ActionTypeSwitch))
}

// getWorkflows retrieves workflows based on the provided query,
// applying access control checks.
// This must not be used in conjunction with preloading approvers.
func (w *WorkflowManager) getWorkflows(
	ctx context.Context,
	pagination repo.Pagination,
	query *repo.Query,
) ([]*model.Workflow, int, error) {
	isGroupFiltered, err := w.userManager.NeedsGroupFiltering(ctx, authz.ActionRead, authz.ResourceTypeWorkFlow)
	if err != nil {
		return nil, 0, errs.Wrap(ErrGetWorkflowDB, err)
	}

	if isGroupFiltered {
		iamIdentifier, err := cmkContext.ExtractClientDataIdentifier(ctx)
		if err != nil {
			return nil, 0, errs.Wrap(ErrGetWorkflowDB, err)
		}

		query = query.Join(
			repo.LeftJoin,
			approverJoinCond,
		)
		orCK := repo.NewCompositeKey().
			Where(repo.InitiatorIDField, iamIdentifier).
			Where(repo.UserIDField, iamIdentifier)
		orCK.IsStrict = false

		query = query.Where(repo.NewCompositeKeyGroup(orCK))
	}

	query = query.SetLimit(pagination.Top).SetOffset(pagination.Skip)

	workflows := []*model.Workflow{}

	err = w.repo.List(ctx, model.Workflow{}, &workflows, *query.GroupBy(repo.IDField))
	if err != nil {
		return nil, 0, errs.Wrap(ErrGetWorkflowDB, err)
	}

	count, err := w.repo.Count(
		ctx,
		&model.Workflow{},
		*query.Select(repo.NewSelectField(repo.IDField, repo.QueryFunction{
			Function: repo.CountFunc,
			Distinct: true,
		})),
	)
	if err != nil {
		return nil, 0, errs.Wrap(ErrGetWorkflowDB, err)
	}

	return workflows, count, nil
}

// addApprovers adds the specified approvers to the workflow
// and transitions the workflow to the next state.
func (w *WorkflowManager) getWorkflowLifecycle(
	ctx context.Context,
	workflow *model.Workflow,
	userID string,
) (*wf.Lifecycle, error) {
	workflowConfig, err := w.WorkflowConfig(ctx)
	if err != nil {
		return nil, oops.Join(ErrGetWorkflowConfig, err)
	}

	workflowLifecycle := wf.NewLifecycle(
		workflow, w.keyManager, w.keyConfigurationManager, w.systemManager, w.repo, userID,
		workflowConfig.MinimumApprovals,
	)

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

		// Associate approver groups with the workflow
		groupIDs := make([]uuid.UUID, len(groups))
		for i, group := range groups {
			groupIDs[i] = group.ID
		}
		bytes, err := json.Marshal(groupIDs)
		if err != nil {
			return errs.Wrap(ErrAddApproverGroupsDB, err)
		}

		workflow.ApproverGroupIDs = bytes
		_, err = w.repo.Patch(ctx, workflow, *repo.NewQuery())
		if err != nil {
			return errs.Wrap(ErrAddApproverGroupsDB, err)
		}

		// Then, apply the transition to next state
		err = workflowLifecycle.ApplyTransition(ctx, wf.TransitionCreate)
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
		Where(repo.StateField, wf.NonTerminalStates)

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
func (w *WorkflowManager) applyTransition(
	ctx context.Context,
	userID string,
	workflow *model.Workflow,
	transition wf.Transition,
) error {
	err := w.repo.Transaction(ctx, func(ctx context.Context) error {
		workflowLifecycle, err := w.getWorkflowLifecycle(ctx, workflow, userID)
		if err != nil {
			return err
		}

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
	approverID string,
	approved bool,
) error {
	approver := &model.WorkflowApprover{}

	err := w.repo.Transaction(ctx, func(ctx context.Context) error {
		ck := repo.NewCompositeKey().
			Where(fmt.Sprintf("%s_%s", repo.UserField, repo.IDField), approverID).
			Where(fmt.Sprintf("%s_%s", repo.WorkflowField, repo.IDField), workflowID)

		_, err := w.repo.First(ctx, approver, *repo.NewQuery().
			Where(repo.NewCompositeKeyGroup(ck)))
		if err != nil {
			return errs.Wrap(wf.ErrCheckApproverDecision, err)
		}

		approver.Approved = sql.NullBool{Bool: approved, Valid: true}

		_, err = w.repo.Patch(ctx, approver, *repo.NewQuery())
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
	case wf.ArtifactTypeKeyConfiguration.String(), wf.ArtifactTypeSystem.String(), wf.ArtifactTypeKey.String():
		_, err := w.getKeyConfigurationsFromArtifact(ctx, workflow)
		if errors.Is(err, ErrKeyConfigurationNotAllowed) || errors.Is(err, repo.ErrNotFound) {
			return false, nil
		}

		if err != nil {
			return false, err
		}

		userinfo, err := w.userManager.GetUserInfo(ctx)
		if err != nil {
			return false, err
		}

		if userinfo.Role == string(constants.TenantAuditorRole) {
			return false, nil
		}

		return true, nil
	}
	return false, errs.Wrapf(ErrGetKeyConfigFromArtifact,
		"unsupported artifact type: "+workflow.ArtifactType)
}

func (w *WorkflowManager) getKeyConfigurationsFromArtifact(
	ctx context.Context,
	workflow *model.Workflow,
) ([]*model.KeyConfiguration, error) {
	var keyConfigs []*model.KeyConfiguration

	switch workflow.ArtifactType {
	case wf.ArtifactTypeKeyConfiguration.String():
		keyConfig, err := w.keyConfigurationManager.GetKeyConfigurationByID(ctx, workflow.ArtifactID)
		if err != nil {
			return nil, errs.Wrap(ErrGetKeyConfigFromArtifact, err)
		}

		keyConfigs = append(keyConfigs, keyConfig)

	case wf.ArtifactTypeSystem.String():
		keyConfigsFromSystems, err := w.getKeyConfigFromSystem(ctx, workflow)
		if err != nil {
			return nil, err
		}

		keyConfigs = append(keyConfigs, keyConfigsFromSystems...)

	case wf.ArtifactTypeKey.String():
		keyConfig, err := w.getKeyConfigFromKey(ctx, workflow)
		if err != nil {
			return nil, err
		}

		keyConfigs = append(keyConfigs, keyConfig)

	default:
		return nil, errs.Wrapf(ErrGetKeyConfigFromArtifact,
			"unsupported artifact type: "+workflow.ArtifactType)
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
	case wf.ActionTypeUnlink.String(), wf.ActionTypeSwitch.String():
		system, err := w.systemManager.GetSystemByID(ctx, workflow.ArtifactID)
		if err != nil {
			return nil, errs.Wrap(ErrGetKeyConfigFromArtifact, err)
		}

		keyConfig, err := w.keyConfigurationManager.GetKeyConfigurationByID(ctx, *system.KeyConfigurationID)
		if err != nil {
			return nil, errs.Wrap(ErrGetKeyConfigFromArtifact, err)
		}

		keyConfigs = append(keyConfigs, keyConfig)
	}

	// If action type is LINK or SWITCH, we need to get the target key configuration from parameters
	switch workflow.ActionType {
	case wf.ActionTypeLink.String(), wf.ActionTypeSwitch.String():
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
	idmClient, err := w.groupManager.GetIdentityManagementPlugin()
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

		authCtx, err := cmkContext.ExtractClientDataAuthContext(ctx)
		if err != nil {
			return nil, nil, errs.Wrap(ErrAutoAssignApprover, err)
		}

		idmGroup, err := idmClient.GetGroup(ctx, &idmv1.GetGroupRequest{
			GroupName:   group.IAMIdentifier,
			AuthContext: &idmv1.AuthContext{Data: authCtx},
		})
		if err != nil {
			return nil, nil, errs.Wrap(ErrAutoAssignApprover, err)
		}

		groupUsers, err := idmClient.GetUsersForGroup(ctx, &idmv1.GetUsersForGroupRequest{
			GroupId:     idmGroup.GetGroup().GetId(),
			AuthContext: &idmv1.AuthContext{Data: authCtx},
		})
		if err != nil {
			return nil, nil, errs.Wrap(ErrAutoAssignApprover, err)
		}

		for _, user := range groupUsers.GetUsers() {
			userID := user.GetId()

			if userID == workflow.InitiatorID {
				continue // Skip initiator
			}

			approverMap[userID] = model.WorkflowApprover{
				UserID:   userID,
				UserName: user.GetEmail(),
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
			slog.String("task_id", info.ID),
			slog.String("workflow_id", workflow.ID.String()))
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

	n, err := notifier.New(w.cfg)
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

	task, err := n.Workflow().CreateTask(data, recipients)
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
	case wf.ArtifactTypeKey.String():
		key, err := w.keyManager.Get(ctx, workflow.ArtifactID)
		if err != nil {
			return err
		}
		workflow.ArtifactName = ptr.PointTo(key.Name)

	case wf.ArtifactTypeKeyConfiguration.String():
		keyConfig, err := w.keyConfigurationManager.GetKeyConfigurationByID(ctx, workflow.ArtifactID)
		if err != nil {
			return err
		}
		workflow.ArtifactName = ptr.PointTo(keyConfig.Name)

	case wf.ArtifactTypeSystem.String():
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

func (w *WorkflowManager) populateParametersResource(
	ctx context.Context,
	workflow *model.Workflow,
) error {
	switch workflow.ArtifactType {
	case wf.ArtifactTypeKeyConfiguration.String():
		if workflow.ActionType == wf.ActionTypeUpdatePrimary.String() {
			key, err := w.keyManager.Get(ctx, workflow.ArtifactID)
			if err != nil {
				return err
			}

			workflow.ParametersResourceType = ptr.PointTo(wf.ParametersResourceTypeKey.String())
			workflow.ParametersResourceName = ptr.PointTo(key.Name)
		}

	case wf.ArtifactTypeSystem.String():
		switch workflow.ActionType {
		case wf.ActionTypeLink.String(), wf.ActionTypeSwitch.String():
			keyConfigID, err := uuid.Parse(workflow.Parameters)
			if err != nil {
				return err
			}

			keyConfig, err := w.keyConfigurationManager.GetKeyConfigurationByID(ctx, keyConfigID)
			if err != nil {
				return err
			}

			workflow.ParametersResourceType = ptr.PointTo(wf.ParametersResourceTypeKeyConfiguration.String())
			workflow.ParametersResourceName = ptr.PointTo(keyConfig.Name)
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
