package manager

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/samber/oops"

	idmv1 "github.com/openkcm/plugin-sdk/proto/plugin/identity_management/v1"

	"github.tools.sap/kms/cmk/internal/async"
	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/constants"
	"github.tools.sap/kms/cmk/internal/errs"
	"github.tools.sap/kms/cmk/internal/log"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/notifier"
	wn "github.tools.sap/kms/cmk/internal/notifier/workflow"
	"github.tools.sap/kms/cmk/internal/repo"
	wf "github.tools.sap/kms/cmk/internal/workflow"
	asyncUtils "github.tools.sap/kms/cmk/utils/async"
	cmkContext "github.tools.sap/kms/cmk/utils/context"
	"github.tools.sap/kms/cmk/utils/odata"
	"github.tools.sap/kms/cmk/utils/ptr"
)

type WorkflowStatus struct {
	Enabled bool
	Exists  bool
}

var ErrWorkflowApproverDecision = errors.New("workflow approver decision")

type Workflow interface {
	CheckWorkflow(ctx context.Context, workflow *model.Workflow) (WorkflowStatus, error)
	GetWorkflows(ctx context.Context, params repo.QueryMapper) ([]*model.Workflow, int, error)
	CreateWorkflow(ctx context.Context, workflow *model.Workflow) (*model.Workflow, error)
	GetWorkflowsByID(ctx context.Context, workflowID uuid.UUID) (*model.Workflow, error)
	ListWorkflowApprovers(
		ctx context.Context,
		id uuid.UUID,
		skip int,
		top int,
	) ([]*model.WorkflowApprover, int, error)
	TransitionWorkflow(
		ctx context.Context,
		workflowID uuid.UUID,
		transition wf.Transition,
	) (*model.Workflow, error)
	WorkflowConfig(ctx context.Context) (*model.WorkflowConfig, error)
	IsWorkflowEnabled(ctx context.Context) bool
	CleanupTerminalWorkflows(ctx context.Context) error
}

type WorkflowManager struct {
	repo                    repo.Repo
	keyManager              *KeyManager
	keyConfigurationManager *KeyConfigManager
	systemManager           *SystemManager
	groupManager            *GroupManager
	asyncClient             async.Client
	tenantConfigManager     *TenantConfigManager
}

func NewWorkflowManager(
	repository repo.Repo,
	keyManager *KeyManager,
	keyConfigurationManager *KeyConfigManager,
	systemManager *SystemManager,
	groupManager *GroupManager,
	asyncClient async.Client,
	tenantConfigManager *TenantConfigManager,
) *WorkflowManager {
	return &WorkflowManager{
		repo:                    repository,
		keyManager:              keyManager,
		keyConfigurationManager: keyConfigurationManager,
		systemManager:           systemManager,
		groupManager:            groupManager,
		asyncClient:             asyncClient,
		tenantConfigManager:     tenantConfigManager,
	}
}

type WorkflowFilter struct {
	State        string
	ArtifactType string
	ArtifactID   uuid.UUID
	ActionType   string
	UserID       string
	Skip         int
	Top          int
}

var _ repo.QueryMapper = (*WorkflowFilter)(nil) // Assert interface impl

func NewWorkflowFilterFromOData(queryMapper odata.QueryOdataMapper) (*WorkflowFilter, error) {
	skipPtr, topPtr := queryMapper.GetPaging()
	skip := ptr.GetIntOrDefault(skipPtr, constants.DefaultSkip)
	top := ptr.GetIntOrDefault(topPtr, constants.DefaultTop)

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

	actionType, err := queryMapper.GetString(repo.ActionTypeField)
	if err != nil {
		return nil, err
	}

	userID, err := queryMapper.GetString(repo.UserIDField)
	if err != nil {
		return nil, err
	}

	return &WorkflowFilter{
		State:        state,
		ArtifactType: artifactType,
		ArtifactID:   artifactID,
		ActionType:   actionType,
		UserID:       userID,
		Skip:         skip,
		Top:          top,
	}, nil
}

func (w WorkflowFilter) GetQuery() *repo.Query {
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

	if w.ActionType != "" {
		ck = ck.Where(repo.ActionTypeField, w.ActionType)
	}

	if w.UserID != "" {
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
		orCK := repo.NewCompositeKey().
			Where(repo.InitiatorIDField, w.UserID).
			Where(repo.UserIDField, w.UserID)
		orCK.IsStrict = false

		query = query.Where(repo.NewCompositeKeyGroup(orCK))
	}

	if len(ck.Conds) > 0 {
		query = query.Where(repo.NewCompositeKeyGroup(ck))
	}

	return query.SetLimit(w.Top).SetOffset(w.Skip)
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

func (w WorkflowFilter) GetString(field repo.QueryField) (string, error) {
	var val string

	switch field {
	case repo.StateField:
		val = w.State
	case repo.ArtifactTypeField:
		val = w.ArtifactType
	case repo.ActionTypeField:
		val = w.ActionType
	case repo.UserIDField:
		val = w.UserID
	default:
		return "", ErrIncompatibleQueryField
	}

	return val, nil
}

func (w *WorkflowManager) GetWorkflows(
	ctx context.Context,
	params repo.QueryMapper,
) ([]*model.Workflow, int, error) {
	workflows := []*model.Workflow{}

	query := *params.GetQuery()

	count, err := w.repo.List(ctx, model.Workflow{}, &workflows, query)
	if err != nil {
		return nil, 0, errs.Wrap(ErrGetWorkflowDB, err)
	}

	return workflows, count, nil
}

func (w *WorkflowManager) WorkflowConfig(ctx context.Context) (*model.WorkflowConfig, error) {
	workflowConfig, err := w.tenantConfigManager.GetWorkflowConfig(ctx)
	if err != nil {
		return nil, oops.Join(ErrGetWorkflowConfig, err)
	}

	return workflowConfig, nil
}

func (w *WorkflowManager) CheckWorkflow(ctx context.Context,
	workflow *model.Workflow,
) (WorkflowStatus, error) {
	workflowConfig, err := w.WorkflowConfig(ctx)
	if err != nil {
		return WorkflowStatus{}, err
	}

	enable := workflowConfig.Enabled

	if !enable {
		return WorkflowStatus{
			Enabled: false,
			Exists:  false,
		}, nil
	}

	isActive := repo.NewCompositeKey().
		Where(repo.StateField, wf.StateInitial.String()).
		Where(repo.StateField, wf.StateWaitApproval.String()).
		Where(repo.StateField, wf.StateWaitConfirmation.String()).
		Where(repo.StateField, wf.StateExecuting.String())
	isActive.IsStrict = false

	baseCondition := repo.NewCompositeKey().Where(
		repo.ArtifactIDField, workflow.ArtifactID)

	switch workflow.ActionType {
	case wf.ActionTypeUpdatePrimary.String(), wf.ActionTypeLink.String(),
		wf.ActionTypeSwitch.String():
		baseCondition = baseCondition.Where(repo.ParametersField, workflow.Parameters)
	default:
		// empty
	}

	exist, err := w.repo.First(
		ctx,
		&model.Workflow{},
		*repo.NewQuery().Where(repo.NewCompositeKeyGroup(baseCondition)).Where(repo.NewCompositeKeyGroup(isActive)),
	)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return WorkflowStatus{
			Enabled: enable,
			Exists:  false,
		}, oops.Join(ErrCheckWorkflow, err)
	}

	return WorkflowStatus{
		Enabled: enable,
		Exists:  exist,
	}, nil
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

	err = w.repo.Transaction(ctx, func(ctx context.Context, r repo.Repo) error {
		err = r.Create(ctx, workflow)
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

	approvers, err := w.getApproversFromKeyConfigs(ctx, keyConfigs)
	if err != nil {
		return nil, err
	}

	err = w.addApprovers(ctx, workflow.InitiatorID, workflow, approvers)
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

func (w *WorkflowManager) IsWorkflowEnabled(ctx context.Context) bool {
	workflowConfig, err := w.WorkflowConfig(ctx)
	if err != nil {
		log.Error(ctx, "Failed to get workflow config", err)
		return false
	}

	return workflowConfig.Enabled
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
			return w.repo.Transaction(ctx, func(ctx context.Context, r repo.Repo) error {
				for _, workflow := range workflows {
					_, err := r.Delete(ctx, &model.Workflow{ID: workflow.ID}, *repo.NewQuery())
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

// addApprovers adds the specified approvers to the workflow
// and transitions the workflow to the next state.
// This is wrapped in a transaction to ensure that DB state is consistent
func (w *WorkflowManager) addApprovers(
	ctx context.Context,
	userID string,
	workflow *model.Workflow,
	approvers []*model.WorkflowApprover,
) error {
	err := w.repo.Transaction(ctx, func(ctx context.Context, r repo.Repo) error {
		workflowConfig, err := w.WorkflowConfig(ctx)
		if err != nil {
			return oops.Join(ErrGetWorkflowConfig, err)
		}

		workflowLifecycle := wf.NewLifecycle(
			workflow,
			w.keyManager,
			w.keyConfigurationManager,
			w.systemManager,
			r,
			userID,
			workflowConfig.MinimumApprovals,
		)

		// Add each approver to the workflow
		for _, approver := range approvers {
			if approver.UserID == workflow.InitiatorID {
				// Skip adding the initiator as an approver
				continue
			}

			approver.WorkflowID = workflow.ID

			_, err := r.First(ctx, workflow, *repo.NewQuery())
			if err != nil {
				return errs.Wrap(ErrGetWorkflowDB, err)
			}

			err = r.Set(ctx, approver)
			if err != nil {
				return errs.Wrap(ErrAddApproversDB, err)
			}
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
	userID string,
	workflow *model.Workflow,
	transition wf.Transition,
) error {
	err := w.repo.Transaction(ctx, func(ctx context.Context, r repo.Repo) error {
		workflowConfig, err := w.WorkflowConfig(ctx)
		if err != nil {
			return oops.Join(ErrGetWorkflowConfig, err)
		}

		workflowLifecycle := wf.NewLifecycle(
			workflow, w.keyManager, w.keyConfigurationManager, w.systemManager, r, userID,
			workflowConfig.MinimumApprovals,
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
	approverID string,
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

func (w *WorkflowManager) getApproversFromKeyConfigs(
	ctx context.Context,
	keyConfigs []*model.KeyConfiguration,
) ([]*model.WorkflowApprover, error) {
	idmClient, err := w.groupManager.GetIdentityManagementPlugin()
	if err != nil {
		return nil, err
	}

	// Use a map to avoid duplicate approvers
	approverMap := make(map[string]model.WorkflowApprover)

	for _, keyConfig := range keyConfigs {
		group, err := w.groupManager.GetGroupByID(ctx, keyConfig.AdminGroupID)
		if err != nil {
			return nil, errs.Wrap(ErrAutoAssignApprover, err)
		}

		authCtx, err := cmkContext.ExtractClientDataAuthContext(ctx)
		if err != nil {
			return nil, errs.Wrap(ErrAutoAssignApprover, err)
		}

		idmGroup, err := idmClient.GetGroup(ctx, &idmv1.GetGroupRequest{
			GroupName:   group.IAMIdentifier,
			AuthContext: &idmv1.AuthContext{Data: authCtx},
		})
		if err != nil {
			return nil, errs.Wrap(ErrAutoAssignApprover, err)
		}

		groupUsers, err := idmClient.GetUsersForGroup(ctx, &idmv1.GetUsersForGroupRequest{
			GroupId:     idmGroup.GetGroup().GetId(),
			AuthContext: &idmv1.AuthContext{Data: authCtx},
		})
		if err != nil {
			return nil, errs.Wrap(ErrAutoAssignApprover, err)
		}

		for _, user := range groupUsers.GetUsers() {
			userID := user.GetId()

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

	return approvers, nil
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

	n, err := notifier.New()
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

	_, err = w.asyncClient.Enqueue(task)
	if err != nil {
		log.Error(ctx, "Enqueue workflow transition task failed", err)
		return err
	}

	return nil
}
