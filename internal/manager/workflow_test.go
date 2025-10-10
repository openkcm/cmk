package manager_test

import (
	"context"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/grpc/catalog"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/workflow"
)

func SetupWorkflowManager(t *testing.T) (*manager.WorkflowManager,
	repo.Repo, string,
) {
	t.Helper()

	db, tenants := testutils.NewTestDB(t, testutils.TestDBConfig{
		Models: []driver.TenantTabler{
			&model.Workflow{},
			&model.WorkflowApprover{},
		},
	})

	r := sql.NewRepository(db)

	cfg := config.Config{}

	ctlg, err := catalog.New(t.Context(), cfg)
	assert.NoError(t, err)

	tenantConfigManager := manager.NewTenantConfigManager(r, ctlg)
	certManager := manager.NewCertificateManager(t.Context(), r, ctlg, &cfg.Certificates)
	keyConfigManager := manager.NewKeyConfigManager(r, certManager, &cfg)
	cmkAuditor := auditor.New(t.Context(), &cfg)

	clientsFactory, err := clients.NewFactory(cfg.Services)
	assert.NoError(t, err)
	systemManager := manager.NewSystemManager(t.Context(), r, clientsFactory, nil, ctlg, cmkAuditor, &cfg)

	keym := manager.NewKeyManager(r, ctlg, tenantConfigManager, keyConfigManager, certManager, nil, cmkAuditor)
	m := manager.NewWorkflowManager(r, keym, keyConfigManager, systemManager, &cfg.Workflows)

	return m, r, tenants[0]
}

func createTestWorkflow(
	ctx context.Context,
	repo repo.Repo,
	state workflow.State,
	initiatorID uuid.UUID,
	actionType string,
	artifactID uuid.UUID,
	approverID uuid.UUID,
) (*model.Workflow, error) {
	wf := &model.Workflow{
		ID:           uuid.New(),
		State:        string(state),
		InitiatorID:  initiatorID,
		ArtifactType: "KEY",
		ArtifactID:   artifactID,
		ActionType:   actionType,
		Approvers:    []model.WorkflowApprover{{UserID: approverID}},
	}

	err := repo.Create(ctx, wf)
	if err != nil {
		return nil, errs.Wrapf(err, "failed to create test workflow")
	}

	return wf, nil
}

func TestWorkflowManager_CreateWorkflow(t *testing.T) {
	m, repo, tenant := SetupWorkflowManager(t)

	t.Run("Should error on existing workflow", func(t *testing.T) {
		wf, err := createTestWorkflow(testutils.CreateCtxWithTenant(tenant),
			repo, workflow.StateInitial, uuid.New(), "DELETE", uuid.New(), uuid.New())
		assert.NoError(t, err)

		_, err = m.CreateWorkflow(testutils.CreateCtxWithTenant(tenant), wf)
		assert.ErrorIs(t, err, manager.ErrOngoingWorkflowExist)
	})

	t.Run("Should create workflow", func(t *testing.T) {
		expected := &model.Workflow{
			ID:           uuid.New(),
			State:        "INITIAL",
			InitiatorID:  uuid.New(),
			ArtifactType: "KEY",
			ArtifactID:   uuid.New(),
			ActionType:   "DELETE",
			Approvers:    []model.WorkflowApprover{{UserID: uuid.New()}},
		}
		res, err := m.CreateWorkflow(testutils.CreateCtxWithTenant(tenant), expected)
		assert.NoError(t, err)
		assert.Equal(t, expected, res)
	})
}

func TestWorkflowManager_TransitWorkflow(t *testing.T) {
	m, repo, tenant := SetupWorkflowManager(t)

	t.Run("Should error on invalid event actor", func(t *testing.T) {
		wf, err := createTestWorkflow(testutils.CreateCtxWithTenant(tenant), repo,
			workflow.StateInitial, uuid.New(), "DELETE", uuid.New(), uuid.New())
		assert.NoError(t, err)
		_, err = m.TransitionWorkflow(
			testutils.CreateCtxWithTenant(tenant),
			wf.InitiatorID,
			wf.ID,
			workflow.TransitionApprove,
		)
		assert.ErrorIs(t, err, workflow.ErrInvalidEventActor)
	})

	t.Run("Should transit to wait confirmation on approve", func(t *testing.T) {
		wf, err := createTestWorkflow(testutils.CreateCtxWithTenant(tenant), repo,
			workflow.StateWaitApproval, uuid.New(), "DELETE", uuid.New(), uuid.New())
		assert.NoError(t, err)
		res, err := m.TransitionWorkflow(
			testutils.CreateCtxWithTenant(tenant),
			wf.Approvers[0].UserID,
			wf.ID,
			workflow.TransitionApprove,
		)
		assert.NoError(t, err)
		assert.EqualValues(t, workflow.StateWaitConfirmation, res.State)
	})

	t.Run("Should transit to reject on reject", func(t *testing.T) {
		wf, err := createTestWorkflow(testutils.CreateCtxWithTenant(tenant), repo,
			workflow.StateWaitApproval, uuid.New(), "DELETE", uuid.New(), uuid.New())
		assert.NoError(t, err)
		res, err := m.TransitionWorkflow(
			testutils.CreateCtxWithTenant(tenant),
			wf.Approvers[0].UserID,
			wf.ID,
			workflow.TransitionReject,
		)
		assert.NoError(t, err)
		assert.EqualValues(t, workflow.StateRejected, res.State)
	})
}

func TestWorlfowManager_GetWorkflowByID(t *testing.T) {
	m, r, tenant := SetupWorkflowManager(t)
	wf, err := createTestWorkflow(testutils.CreateCtxWithTenant(tenant), r,
		workflow.StateInitial, uuid.New(), "DELETE", uuid.New(), uuid.New())
	assert.NoError(t, err)

	tests := []struct {
		name       string
		workflowID uuid.UUID
		expectErr  bool
		errMessage error
	}{
		{
			name:       "TestWorlfowManager_GetByID_ValidUUID",
			workflowID: wf.ID,
			expectErr:  false,
		},
		{
			name:       "TestWorlfowManager_GetByID_NonExistent",
			workflowID: uuid.New(),
			expectErr:  true,
			errMessage: repo.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retrievedWf, err := m.GetWorkflowsByID(
				testutils.CreateCtxWithTenant(tenant),
				tt.workflowID)
			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, retrievedWf)
				assert.ErrorIs(t, err, tt.errMessage)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, retrievedWf)
				assert.Equal(t, wf.ID, retrievedWf.ID)
				assert.Equal(t, wf.ApproverIDs(), retrievedWf.ApproverIDs())
				assert.NotZero(t, retrievedWf.CreatedAt)
				assert.NotZero(t, retrievedWf.UpdatedAt)
			}
		})
	}
}

func newGetWorkflowsRequest(
	artifactID uuid.UUID,
	state string,
	actionType string,
	artifactType string,
	userID uuid.UUID,
) cmkapi.GetWorkflowsRequestObject {
	params := cmkapi.GetWorkflowsParams{
		ArtifactID: &artifactID,
	}

	if state != "" {
		s := cmkapi.WorkflowStateEnum(state)
		params.State = &s
	}

	if actionType != "" {
		a := cmkapi.WorkflowActionTypeEnum(actionType)
		params.ActionType = &a
	}

	if artifactType != "" {
		at := cmkapi.WorkflowArtifactTypeEnum(artifactType)
		params.ArtifactType = &at
	}

	if userID != uuid.Nil {
		params.UserID = &userID
	}

	return cmkapi.GetWorkflowsRequestObject{Params: params}
}

func TestWorkfowManager_GetWorkflows(t *testing.T) {
	m, repo, tenant := SetupWorkflowManager(t)
	userID := uuid.New()
	allWorkflowUserID := uuid.New()
	artifactID := uuid.New()
	_, err := createTestWorkflow(testutils.CreateCtxWithTenant(tenant), repo,
		workflow.StateInitial, userID, "DELETE", uuid.New(), allWorkflowUserID)
	assert.NoError(t, err)
	_, err = createTestWorkflow(testutils.CreateCtxWithTenant(tenant), repo, workflow.StateInitial,
		allWorkflowUserID, "DELETE", artifactID, uuid.New())
	assert.NoError(t, err)
	_, err = createTestWorkflow(testutils.CreateCtxWithTenant(tenant), repo, workflow.StateRejected,
		allWorkflowUserID, "DELETE", uuid.New(), uuid.New())
	assert.NoError(t, err)
	_, err = createTestWorkflow(testutils.CreateCtxWithTenant(tenant), repo, workflow.StateInitial,
		userID, "UPDATE_STATE", uuid.New(), allWorkflowUserID)
	assert.NoError(t, err)

	tests := []struct {
		name                string
		filter              manager.WorkflowFilter
		expectedCount       int
		expectedState       string
		expectedActionType  string
		expectedArtfactType string
		expectedInitiatorID uuid.UUID
	}{
		{
			name:                "Should get all workflows",
			filter:              manager.WorkflowFilter{},
			expectedCount:       4,
			expectedState:       "",
			expectedActionType:  "",
			expectedArtfactType: "",
			expectedInitiatorID: uuid.Nil,
		},
		{
			name:                "Should get rejected workflows",
			filter:              manager.WorkflowFilter{State: workflow.StateRejected.String()},
			expectedCount:       1,
			expectedState:       workflow.StateRejected.String(),
			expectedActionType:  "",
			expectedArtfactType: "",
			expectedInitiatorID: uuid.Nil,
		},
		{
			name: "Should get initial workflows",
			filter: m.NewWorkflowFilter(newGetWorkflowsRequest(
				uuid.Nil,
				workflow.StateInitial.String(),
				"",
				"",
				uuid.Nil,
			)),
			expectedCount:       3,
			expectedState:       workflow.StateInitial.String(),
			expectedActionType:  "",
			expectedInitiatorID: uuid.Nil,
		},
		{
			name: "Should get action type UPDATE_STATE workflows",
			filter: m.NewWorkflowFilter(newGetWorkflowsRequest(
				uuid.Nil,
				"",
				workflow.ActionTypeUpdateState.String(),
				"",
				uuid.Nil,
			)),
			expectedCount:       1,
			expectedState:       "",
			expectedActionType:  workflow.ActionTypeUpdateState.String(),
			expectedArtfactType: "",
			expectedInitiatorID: uuid.Nil,
		},
		{
			name: "Get workflows for user with 2",
			filter: m.NewWorkflowFilter(newGetWorkflowsRequest(
				uuid.Nil,
				"",
				"",
				"",
				userID,
			)),
			expectedCount:       2,
			expectedState:       "",
			expectedActionType:  "",
			expectedArtfactType: "",
			expectedInitiatorID: userID,
		},
		{
			name: "Get workflows for user with all",
			filter: m.NewWorkflowFilter(newGetWorkflowsRequest(
				uuid.Nil,
				"",
				"",
				"",
				allWorkflowUserID,
			)),
			expectedCount:       4,
			expectedState:       "",
			expectedActionType:  "",
			expectedArtfactType: "",
			expectedInitiatorID: uuid.Nil,
		},
		{
			name: "Get workflows for user with all and state initial",
			filter: m.NewWorkflowFilter(newGetWorkflowsRequest(
				uuid.Nil,
				workflow.StateInitial.String(),
				"",
				"",
				allWorkflowUserID,
			)),
			expectedCount:       3,
			expectedState:       "",
			expectedActionType:  "",
			expectedArtfactType: "",
			expectedInitiatorID: uuid.Nil,
		},
		{
			name: "Get workflows by artifact type",
			filter: m.NewWorkflowFilter(newGetWorkflowsRequest(
				uuid.Nil,
				"",
				"",
				workflow.ArtifactTypeKey.String(),
				uuid.Nil,
			)),
			expectedCount:       4,
			expectedState:       "",
			expectedActionType:  "",
			expectedArtfactType: workflow.ArtifactTypeKey.String(),
			expectedInitiatorID: uuid.Nil,
		},
		{
			name: "Get workflows by artifact id",
			filter: m.NewWorkflowFilter(newGetWorkflowsRequest(
				artifactID,
				"",
				"",
				workflow.ArtifactTypeKey.String(),
				uuid.Nil,
			)),
			expectedCount:       1,
			expectedState:       "",
			expectedActionType:  "",
			expectedArtfactType: "",
			expectedInitiatorID: uuid.Nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			workflows, count, err := m.GetWorkflows(testutils.CreateCtxWithTenant(tenant), tc.filter)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedCount, count)

			if tc.expectedState != "" {
				for _, wf := range workflows {
					assert.Equal(t, tc.expectedState, wf.State)
				}
			}

			if tc.expectedActionType != "" {
				for _, wf := range workflows {
					assert.Equal(t, tc.expectedActionType, wf.ActionType)
				}
			}

			if tc.expectedInitiatorID != uuid.Nil {
				for _, wf := range workflows {
					assert.True(t, tc.expectedInitiatorID == wf.InitiatorID || tc.expectedInitiatorID == wf.Approvers[0].UserID)
				}
			}
		})
	}
}

func TestWorlfowManager_ListApprovers(t *testing.T) {
	m, r, tenant := SetupWorkflowManager(t)
	wf, err := createTestWorkflow(testutils.CreateCtxWithTenant(tenant), r,
		workflow.StateInitial, uuid.New(), "DELETE", uuid.New(), uuid.New())
	assert.NoError(t, err)

	tests := []struct {
		name       string
		workflowID uuid.UUID
		expectErr  bool
		errMessage error
	}{
		{
			name:       "TestWorlfowManager_ListApproversByWorkflowID_ValidUUID",
			workflowID: wf.ID,
			expectErr:  false,
		},
		{
			name:       "TestWorlfowManager_ListApproversByWorkflowID_NonExistent",
			workflowID: uuid.New(),
			expectErr:  true,
			errMessage: repo.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			approvers, _, err := m.ListWorkflowApprovers(testutils.CreateCtxWithTenant(tenant), tt.workflowID,
				constants.DefaultSkip, constants.DefaultTop)
			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, approvers)
				assert.ErrorIs(t, err, tt.errMessage)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, approvers)

				for i := range approvers {
					assert.Equal(t, wf.Approvers[i], *approvers[i])
				}
			}
		})
	}
}

func TestWorlfowManager_AddApprover(t *testing.T) {
	m, r, tenant := SetupWorkflowManager(t)
	wf, err := createTestWorkflow(testutils.CreateCtxWithTenant(tenant), r,
		workflow.StateInitial, uuid.New(), "DELETE", uuid.New(), uuid.New())
	assert.NoError(t, err)

	tests := []struct {
		name       string
		workflowID uuid.UUID
		approverID uuid.UUID
		expectErr  bool
		errMessage error
	}{
		{
			name:       "TestWorlfowManager_AddApprover_WorkflowNotExist",
			workflowID: uuid.New(),
			approverID: uuid.New(),
			expectErr:  true,
			errMessage: repo.ErrNotFound,
		},
		{
			name:       "TestWorlfowManager_AddApprover_Conflict",
			workflowID: wf.ID,
			approverID: wf.Approvers[0].UserID,
			expectErr:  true,
			errMessage: manager.ErrValidateActor,
		},
		{
			name:       "TestWorlfowManager_AddApprover_NewApprover",
			workflowID: wf.ID,
			approverID: wf.InitiatorID,
			expectErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err = m.AddWorkflowApprovers(
				testutils.CreateCtxWithTenant(tenant),
				tt.workflowID,
				tt.approverID,
				[]cmkapi.WorkflowApprover{
					{
						Id: uuid.New(),
					}, {
						Id: uuid.New(),
					},
				},
			)
			if tt.expectErr {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.errMessage)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
