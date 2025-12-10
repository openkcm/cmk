package manager_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/auth"
	"github.com/stretchr/testify/assert"

	"github.tools.sap/kms/cmk/internal/async"
	"github.tools.sap/kms/cmk/internal/auditor"
	"github.tools.sap/kms/cmk/internal/clients"
	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/constants"
	"github.tools.sap/kms/cmk/internal/errs"
	"github.tools.sap/kms/cmk/internal/grpc/catalog"
	"github.tools.sap/kms/cmk/internal/manager"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/repo"
	"github.tools.sap/kms/cmk/internal/repo/sql"
	"github.tools.sap/kms/cmk/internal/testutils"
	"github.tools.sap/kms/cmk/internal/workflow"
	cmkcontext "github.tools.sap/kms/cmk/utils/context"
)

var ErrEnqueuingTask = errors.New("error enqueuing task")

func SetupWorkflowManager(t *testing.T, cfg *config.Config) (
	*manager.WorkflowManager,
	repo.Repo, string,
) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(
		t, testutils.TestDBConfig{
			Models: []driver.TenantTabler{
				&model.Tenant{},
				&model.Group{},
				&model.KeyConfiguration{},
				&model.Key{},
				&model.KeyVersion{},
				&model.System{},
				&model.SystemProperty{},
				&model.Workflow{},
				&model.WorkflowApprover{},
				&model.TenantConfig{},
			},
		},
	)

	r := sql.NewRepository(db)

	ctlg, err := catalog.New(t.Context(), cfg)
	assert.NoError(t, err)

	tenantConfigManager := manager.NewTenantConfigManager(r, ctlg)
	certManager := manager.NewCertificateManager(t.Context(), r, ctlg, &cfg.Certificates)
	cmkAuditor := auditor.New(t.Context(), cfg)
	keyConfigManager := manager.NewKeyConfigManager(r, certManager, cmkAuditor, cfg)
	groupManager := manager.NewGroupManager(r, ctlg)

	clientsFactory, err := clients.NewFactory(cfg.Services)
	assert.NoError(t, err)
	systemManager := manager.NewSystemManager(t.Context(), r, clientsFactory, nil, ctlg, cfg, keyConfigManager)

	keym := manager.NewKeyManager(r, ctlg, tenantConfigManager, keyConfigManager, certManager, nil, cmkAuditor)
	m := manager.NewWorkflowManager(
		r, keym, keyConfigManager, systemManager,
		groupManager, nil, tenantConfigManager,
	)

	return m, r, tenants[0]
}

func createTestWorkflow(
	ctx context.Context,
	repo repo.Repo,
	wf *model.Workflow,
) (*model.Workflow, error) {
	err := repo.Create(ctx, wf)
	if err != nil {
		return nil, errs.Wrapf(err, "failed to create test workflow")
	}

	return wf, nil
}

func TestWorkflowManager_CheckWorkflow(t *testing.T) {
	t.Run(
		"Should return disabled on workflow disabled", func(t *testing.T) {
			m, _, tenant := SetupWorkflowManager(t, &config.Config{})
			testutils.CreateCtxWithTenant(tenant)
			status, err := m.CheckWorkflow(testutils.CreateCtxWithTenant(tenant), &model.Workflow{})
			assert.False(t, status.Enabled)
			assert.False(t, status.Exists)
			assert.NoError(t, err)
		},
	)

	m, repo, tenant := SetupWorkflowManager(t, &config.Config{})

	ctx := testutils.CreateCtxWithTenant(tenant)
	workflowConfig := testutils.NewWorkflowConfig(func(_ *model.TenantConfig) {})
	testutils.CreateTestEntities(ctx, t, repo, workflowConfig)

	t.Run(
		"Should return false on non existing artifacts", func(t *testing.T) {
			status, err := m.CheckWorkflow(ctx, &model.Workflow{})
			assert.True(t, status.Enabled)
			assert.False(t, status.Exists)
			assert.NoError(t, err)
		},
	)

	t.Run(
		"Should return true on existing active artifact without parameters", func(t *testing.T) {
			wf, err := createTestWorkflow(
				ctx, repo, testutils.NewWorkflow(
					func(w *model.Workflow) {
						w.State = workflow.StateInitial.String()
						w.ActionType = workflow.ActionTypeDelete.String()
						w.ArtifactType = workflow.ArtifactTypeKey.String()
					},
				),
			)
			assert.NoError(t, err)

			status, err := m.CheckWorkflow(ctx, wf)
			assert.True(t, status.Enabled)
			assert.True(t, status.Exists)
			assert.NoError(t, err)
		},
	)

	t.Run(
		"Should return true on active artifact requiring parameters", func(t *testing.T) {
			wf, err := createTestWorkflow(
				ctx, repo, testutils.NewWorkflow(
					func(w *model.Workflow) {
						w.State = workflow.StateInitial.String()
						w.ActionType = workflow.ActionTypeUpdatePrimary.String()
						w.ArtifactType = workflow.ArtifactTypeKeyConfiguration.String()
					},
				),
			)
			assert.NoError(t, err)

			status, err := m.CheckWorkflow(ctx, wf)
			assert.True(t, status.Enabled)
			assert.True(t, status.Exists)
			assert.NoError(t, err)
		},
	)

	t.Run(
		"Should return false on artifact requiring parameters", func(t *testing.T) {
			wf, err := createTestWorkflow(
				ctx, repo, testutils.NewWorkflow(
					func(w *model.Workflow) {
						w.State = workflow.StateInitial.String()
						w.ActionType = workflow.ActionTypeUpdatePrimary.String()
						w.ArtifactType = workflow.ArtifactTypeKeyConfiguration.String()
					},
				),
			)
			assert.NoError(t, err)

			wf.Parameters = uuid.NewString()

			status, err := m.CheckWorkflow(ctx, wf)
			assert.True(t, status.Enabled)
			assert.False(t, status.Exists)
			assert.NoError(t, err)
		},
	)

	t.Run(
		"Should return false on non active artifact", func(t *testing.T) {
			wf, err := createTestWorkflow(
				ctx, repo, testutils.NewWorkflow(
					func(w *model.Workflow) {
						w.State = workflow.StateRejected.String()
						w.ActionType = workflow.ActionTypeDelete.String()
						w.ArtifactType = workflow.ArtifactTypeKey.String()
					},
				),
			)
			assert.NoError(t, err)

			status, err := m.CheckWorkflow(ctx, wf)
			assert.True(t, status.Enabled)
			assert.False(t, status.Exists)
			assert.NoError(t, err)
		},
	)
}

func TestWorkflowManager_CreateWorkflow(t *testing.T) {
	m, repo, tenant := SetupWorkflowManager(t, &config.Config{})

	t.Run(
		"Should error on existing workflow", func(t *testing.T) {
			wf, err := createTestWorkflow(
				testutils.CreateCtxWithTenant(tenant),
				repo,
				testutils.NewWorkflow(
					func(w *model.Workflow) {
						w.State = workflow.StateInitial.String()
						w.ActionType = workflow.ActionTypeDelete.String()
						w.ArtifactType = workflow.ArtifactTypeKey.String()
					},
				),
			)
			assert.NoError(t, err)

			_, err = m.CreateWorkflow(testutils.CreateCtxWithTenant(tenant), wf)
			assert.ErrorIs(t, err, manager.ErrOngoingWorkflowExist)
		},
	)

	t.Run(
		"Should create workflow", func(t *testing.T) {
			expected := &model.Workflow{
				ID:           uuid.New(),
				State:        "INITIAL",
				InitiatorID:  uuid.NewString(),
				ArtifactType: "KEY",
				ArtifactID:   uuid.New(),
				ActionType:   "DELETE",
				Approvers:    []model.WorkflowApprover{{UserID: uuid.NewString()}},
			}
			res, err := m.CreateWorkflow(testutils.CreateCtxWithTenant(tenant), expected)
			assert.NoError(t, err)
			assert.Equal(t, expected, res)
		},
	)
}

func TestWorkflowManager_TransitWorkflow(t *testing.T) {
	m, repo, tenant := SetupWorkflowManager(t, &config.Config{})

	ctx := testutils.CreateCtxWithTenant(tenant)
	workflowConfig := testutils.NewWorkflowConfig(func(_ *model.TenantConfig) {})

	testutils.CreateTestEntities(ctx, t, repo, workflowConfig)

	t.Run(
		"Should error on invalid event actor", func(t *testing.T) {
			wf, err := createTestWorkflow(
				testutils.CreateCtxWithTenant(tenant),
				repo,
				testutils.NewWorkflow(
					func(w *model.Workflow) {
						w.State = workflow.StateInitial.String()
						w.ActionType = workflow.ActionTypeDelete.String()
						w.ArtifactType = workflow.ArtifactTypeKey.String()
					},
				),
			)
			assert.NoError(t, err)

			ctx = cmkcontext.InjectClientData(
				cmkcontext.CreateTenantContext(t.Context(), tenant),
				&auth.ClientData{
					Identifier: wf.InitiatorID,
				},
				nil,
			)
			_, err = m.TransitionWorkflow(
				ctx,
				wf.ID,
				workflow.TransitionApprove,
			)
			assert.ErrorIs(t, err, workflow.ErrInvalidEventActor)
		},
	)

	t.Run(
		"Should transit to wait confirmation on approve", func(t *testing.T) {
			wf, err := createTestWorkflow(
				testutils.CreateCtxWithTenant(tenant),
				repo,
				testutils.NewWorkflow(
					func(w *model.Workflow) {
						w.State = workflow.StateWaitApproval.String()
						w.ActionType = workflow.ActionTypeDelete.String()
						w.ArtifactType = workflow.ArtifactTypeKey.String()
					},
				),
			)
			assert.NoError(t, err)
			ctx = cmkcontext.InjectClientData(
				cmkcontext.CreateTenantContext(t.Context(), tenant),
				&auth.ClientData{
					Identifier: wf.Approvers[0].UserID,
				},
				nil,
			)
			res, err := m.TransitionWorkflow(
				ctx,
				wf.ID,
				workflow.TransitionApprove,
			)
			assert.NoError(t, err)
			assert.EqualValues(t, workflow.StateWaitConfirmation, res.State)
		},
	)

	t.Run(
		"Should transit to reject on reject", func(t *testing.T) {
			wf, err := createTestWorkflow(
				testutils.CreateCtxWithTenant(tenant),
				repo,
				testutils.NewWorkflow(
					func(w *model.Workflow) {
						w.State = workflow.StateWaitApproval.String()
						w.ActionType = workflow.ActionTypeDelete.String()
						w.ArtifactType = workflow.ArtifactTypeKey.String()
					},
				),
			)
			assert.NoError(t, err)
			ctx = cmkcontext.InjectClientData(
				cmkcontext.CreateTenantContext(t.Context(), tenant),
				&auth.ClientData{
					Identifier: wf.Approvers[0].UserID,
				},
				nil,
			)
			res, err := m.TransitionWorkflow(
				ctx,
				wf.ID,
				workflow.TransitionReject,
			)
			assert.NoError(t, err)
			assert.EqualValues(t, workflow.StateRejected, res.State)
		},
	)
}

func TestWorlfowManager_GetWorkflowByID(t *testing.T) {
	m, r, tenant := SetupWorkflowManager(t, &config.Config{})
	wf, err := createTestWorkflow(
		testutils.CreateCtxWithTenant(tenant),
		r,
		testutils.NewWorkflow(
			func(w *model.Workflow) {
				w.State = workflow.StateInitial.String()
				w.ActionType = workflow.ActionTypeDelete.String()
				w.ArtifactType = workflow.ArtifactTypeKey.String()
			},
		),
	)
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
		t.Run(
			tt.name, func(t *testing.T) {
				retrievedWf, err := m.GetWorkflowsByID(
					testutils.CreateCtxWithTenant(tenant),
					tt.workflowID,
				)
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
			},
		)
	}
}

func newGetWorkflowsFilter(
	artifactID uuid.UUID,
	state string,
	actionType string,
	artifactType string,
	userID string,
) manager.WorkflowFilter {
	return manager.WorkflowFilter{
		State:        state,
		ArtifactType: artifactType,
		ArtifactID:   artifactID,
		ActionType:   actionType,
		UserID:       userID,
		Skip:         constants.DefaultSkip,
		Top:          constants.DefaultTop,
	}
}

func TestWorkflowFilter_GetUUID(t *testing.T) {
	u := uuid.New()
	filter := manager.WorkflowFilter{
		ArtifactID: u,
	}

	// Should return ArtifactID for repo.ArtifactIDField
	id, err := filter.GetUUID(repo.ArtifactIDField)
	assert.NoError(t, err)
	assert.Equal(t, u, id)

	// Should return error for unsupported field
	id, err = filter.GetUUID(repo.StateField)
	assert.Error(t, err)
	assert.Equal(t, uuid.Nil, id)
}

func TestWorkflowFilter_GetString(t *testing.T) {
	filter := manager.WorkflowFilter{
		State:        "INITIAL",
		ArtifactType: "KEY",
		ActionType:   "DELETE",
		UserID:       "user1",
	}

	// Should return correct values for supported fields
	val, err := filter.GetString(repo.StateField)
	assert.NoError(t, err)
	assert.Equal(t, "INITIAL", val)

	val, err = filter.GetString(repo.ArtifactTypeField)
	assert.NoError(t, err)
	assert.Equal(t, "KEY", val)

	val, err = filter.GetString(repo.ActionTypeField)
	assert.NoError(t, err)
	assert.Equal(t, "DELETE", val)

	val, err = filter.GetString(repo.UserIDField)
	assert.NoError(t, err)
	assert.Equal(t, "user1", val)

	// Should return error for unsupported field
	val, err = filter.GetString(repo.ArtifactIDField)
	assert.Error(t, err)
	assert.Empty(t, val)
}

func TestWorkfowManager_GetWorkflows(t *testing.T) {
	m, repo, tenant := SetupWorkflowManager(t, &config.Config{})
	userID := uuid.NewString()
	allWorkflowUserID := uuid.NewString()
	artifactID := uuid.New()

	_, err := createTestWorkflow(
		testutils.CreateCtxWithTenant(tenant),
		repo,
		testutils.NewWorkflow(
			func(w *model.Workflow) {
				w.State = workflow.StateInitial.String()
				w.ActionType = workflow.ActionTypeDelete.String()
				w.ArtifactType = workflow.ArtifactTypeKey.String()
				w.Approvers = []model.WorkflowApprover{{UserID: allWorkflowUserID}}
				w.InitiatorID = userID
			},
		),
	)
	assert.NoError(t, err)

	_, err = createTestWorkflow(
		testutils.CreateCtxWithTenant(tenant),
		repo,
		testutils.NewWorkflow(
			func(w *model.Workflow) {
				w.State = workflow.StateInitial.String()
				w.ActionType = workflow.ActionTypeDelete.String()
				w.ArtifactType = workflow.ArtifactTypeKey.String()
				w.ArtifactID = artifactID
				w.Approvers = []model.WorkflowApprover{{UserID: uuid.NewString()}}
				w.InitiatorID = allWorkflowUserID
			},
		),
	)

	assert.NoError(t, err)
	_, err = createTestWorkflow(
		testutils.CreateCtxWithTenant(tenant),
		repo,
		testutils.NewWorkflow(
			func(w *model.Workflow) {
				w.State = workflow.StateRejected.String()
				w.ActionType = workflow.ActionTypeDelete.String()
				w.ArtifactType = workflow.ArtifactTypeKey.String()
				w.Approvers = []model.WorkflowApprover{{UserID: uuid.NewString()}}
				w.InitiatorID = allWorkflowUserID
			},
		),
	)
	assert.NoError(t, err)
	_, err = createTestWorkflow(
		testutils.CreateCtxWithTenant(tenant),
		repo,
		testutils.NewWorkflow(
			func(w *model.Workflow) {
				w.State = workflow.StateInitial.String()
				w.ActionType = workflow.ActionTypeUpdateState.String()
				w.ArtifactType = workflow.ArtifactTypeKey.String()
				w.Approvers = []model.WorkflowApprover{{UserID: allWorkflowUserID}}
				w.InitiatorID = userID
			},
		),
	)
	assert.NoError(t, err)

	tests := []struct {
		name                string
		filter              manager.WorkflowFilter
		expectedCount       int
		expectedState       string
		expectedActionType  string
		expectedArtfactType string
		expectedInitiatorID string
	}{
		{
			name:                "Should get all workflows",
			filter:              manager.WorkflowFilter{},
			expectedCount:       4,
			expectedState:       "",
			expectedActionType:  "",
			expectedArtfactType: "",
		},
		{
			name:                "Should get rejected workflows",
			filter:              manager.WorkflowFilter{State: workflow.StateRejected.String()},
			expectedCount:       1,
			expectedState:       workflow.StateRejected.String(),
			expectedActionType:  "",
			expectedArtfactType: "",
		},
		{
			name: "Should get initial workflows",
			filter: newGetWorkflowsFilter(
				uuid.Nil,
				workflow.StateInitial.String(),
				"",
				"",
				"",
			),
			expectedCount:      3,
			expectedState:      workflow.StateInitial.String(),
			expectedActionType: "",
		},
		{
			name: "Should get action type UPDATE_STATE workflows",
			filter: newGetWorkflowsFilter(
				uuid.Nil,
				"",
				workflow.ActionTypeUpdateState.String(),
				"",
				"",
			),
			expectedCount:       1,
			expectedState:       "",
			expectedActionType:  workflow.ActionTypeUpdateState.String(),
			expectedArtfactType: "",
		},
		{
			name: "Get workflows for user with 2",
			filter: newGetWorkflowsFilter(
				uuid.Nil,
				"",
				"",
				"",
				userID,
			),
			expectedCount:       2,
			expectedState:       "",
			expectedActionType:  "",
			expectedArtfactType: "",
			expectedInitiatorID: userID,
		},
		{
			name: "Get workflows for user with all",
			filter: newGetWorkflowsFilter(
				uuid.Nil,
				"",
				"",
				"",
				allWorkflowUserID,
			),
			expectedCount:       4,
			expectedState:       "",
			expectedActionType:  "",
			expectedArtfactType: "",
		},
		{
			name: "Get workflows for user with all and state initial",
			filter: newGetWorkflowsFilter(
				uuid.Nil,
				workflow.StateInitial.String(),
				"",
				"",
				allWorkflowUserID,
			),
			expectedCount:       3,
			expectedState:       "",
			expectedActionType:  "",
			expectedArtfactType: "",
		},
		{
			name: "Get workflows by artifact type",
			filter: newGetWorkflowsFilter(
				uuid.Nil,
				"",
				"",
				workflow.ArtifactTypeKey.String(),
				"",
			),
			expectedCount:       4,
			expectedState:       "",
			expectedActionType:  "",
			expectedArtfactType: workflow.ArtifactTypeKey.String(),
		},
		{
			name: "Get workflows by artifact id",
			filter: newGetWorkflowsFilter(
				artifactID,
				"",
				"",
				workflow.ArtifactTypeKey.String(),
				"",
			),
			expectedCount:       1,
			expectedState:       "",
			expectedActionType:  "",
			expectedArtfactType: "",
		},
	}

	for _, tc := range tests {
		t.Run(
			tc.name, func(t *testing.T) {
				ctx := cmkcontext.InjectClientData(
					cmkcontext.CreateTenantContext(t.Context(), tenant),
					&auth.ClientData{
						Identifier: tc.expectedInitiatorID,
					},
					nil,
				)
				workflows, count, err := m.GetWorkflows(ctx, tc.filter)
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

				if tc.expectedInitiatorID != "" {
					for _, wf := range workflows {
						assert.True(
							t,
							tc.expectedInitiatorID == wf.InitiatorID || tc.expectedInitiatorID == wf.Approvers[0].UserID,
						)
					}
				}
			},
		)
	}
}

func TestWorlfowManager_ListApprovers(t *testing.T) {
	m, r, tenant := SetupWorkflowManager(t, &config.Config{})
	wf, err := createTestWorkflow(
		testutils.CreateCtxWithTenant(tenant),
		r,
		testutils.NewWorkflow(
			func(w *model.Workflow) {
				w.State = workflow.StateInitial.String()
				w.ActionType = workflow.ActionTypeDelete.String()
				w.ArtifactType = workflow.ArtifactTypeKey.String()
			},
		),
	)
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
		t.Run(
			tt.name, func(t *testing.T) {
				approvers, _, err := m.ListWorkflowApprovers(
					testutils.CreateCtxWithTenant(tenant), tt.workflowID,
					constants.DefaultSkip, constants.DefaultTop,
				)
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
			},
		)
	}
}

func TestWorkflowManager_AutoAddApprover(t *testing.T) {
	m, r, tenant := SetupWorkflowManager(
		t, &config.Config{
			Plugins: testutils.SetupMockPlugins(testutils.IdentityPlugin),
		},
	)
	ctx := testutils.CreateCtxWithTenant(tenant)
	ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"KMS_001", "KMS_002"})

	adminGroups := []*model.Group{
		{ID: uuid.New(), Name: "group1", IAMIdentifier: "KMS_001", Role: constants.KeyAdminRole},
		{ID: uuid.New(), Name: "group2", IAMIdentifier: "KMS_002", Role: constants.KeyAdminRole},
	}
	keyConfigs := make([]*model.KeyConfiguration, len(adminGroups))

	for i, g := range adminGroups {
		err := r.Create(ctx, g)
		assert.NoError(t, err)

		keyConfig := testutils.NewKeyConfig(
			func(kc *model.KeyConfiguration) {
				kc.AdminGroup = *g
			},
		)
		err = r.Create(ctx, keyConfig)
		assert.NoError(t, err)

		keyConfigs[i] = keyConfig
	}

	key := testutils.NewKey(
		func(k *model.Key) {
			k.KeyConfigurationID = keyConfigs[0].ID
		},
	)

	err := r.Create(ctx, key)
	assert.NoError(t, err)

	systems := []*model.System{
		testutils.NewSystem(func(_ *model.System) {}),
		testutils.NewSystem(func(k *model.System) { k.KeyConfigurationID = &keyConfigs[0].ID }),
	}

	for _, s := range systems {
		err = r.Create(ctx, s)
		assert.NoError(t, err)
	}

	tests := []struct {
		name           string
		workflowMut    func(*model.Workflow)
		approversCount int
		expectErr      bool
		errMessage     error
	}{
		{
			name: "KeyDelete",
			workflowMut: func(w *model.Workflow) {
				w.ArtifactID = key.ID
				w.ArtifactType = workflow.ArtifactTypeKey.String()
				w.ActionType = workflow.ActionTypeDelete.String()
				w.Approvers = nil
			},
			approversCount: 2,
		},
		{
			name: "KeyDelete - Invalid key",
			workflowMut: func(w *model.Workflow) {
				w.ArtifactID = uuid.New()
				w.ArtifactType = workflow.ArtifactTypeKey.String()
				w.ActionType = workflow.ActionTypeDelete.String()
				w.Approvers = nil
			},
			expectErr:  true,
			errMessage: repo.ErrNotFound,
		},
		{
			name: "KeyStateUpdate",
			workflowMut: func(w *model.Workflow) {
				w.ArtifactID = key.ID
				w.ArtifactType = workflow.ArtifactTypeKey.String()
				w.ActionType = workflow.ActionTypeUpdateState.String()
				w.Parameters = "DISABLED"
				w.Approvers = nil
			},
			approversCount: 2,
		},
		{
			name: "KeyConfigDelete",
			workflowMut: func(w *model.Workflow) {
				w.ArtifactID = keyConfigs[0].ID
				w.ArtifactType = workflow.ArtifactTypeKeyConfiguration.String()
				w.ActionType = workflow.ActionTypeDelete.String()
				w.Approvers = nil
			},
			approversCount: 2,
		},
		{
			name: "KeyConfigDelete - Invalid key config",
			workflowMut: func(w *model.Workflow) {
				w.ArtifactID = uuid.New()
				w.ArtifactType = workflow.ArtifactTypeKeyConfiguration.String()
				w.ActionType = workflow.ActionTypeDelete.String()
				w.Approvers = nil
			},
			expectErr:  true,
			errMessage: repo.ErrNotFound,
		},
		{
			name: "KeyConfigUpdatePK",
			workflowMut: func(w *model.Workflow) {
				w.ArtifactID = keyConfigs[0].ID
				w.ArtifactType = workflow.ArtifactTypeKeyConfiguration.String()
				w.ActionType = workflow.ActionTypeUpdatePrimary.String()
				w.Parameters = uuid.NewString()
				w.Approvers = nil
			},
			approversCount: 2,
		},
		{
			name: "SystemLink",
			workflowMut: func(w *model.Workflow) {
				w.ArtifactID = systems[0].ID
				w.ArtifactType = workflow.ArtifactTypeSystem.String()
				w.ActionType = workflow.ActionTypeLink.String()
				w.Parameters = keyConfigs[0].ID.String()
				w.Approvers = nil
			},
			approversCount: 2,
		},
		{
			name: "SystemLink - Invalid key config",
			workflowMut: func(w *model.Workflow) {
				w.ArtifactID = systems[0].ID
				w.ArtifactType = workflow.ArtifactTypeSystem.String()
				w.ActionType = workflow.ActionTypeLink.String()
				w.Parameters = uuid.NewString()
				w.Approvers = nil
			},
			expectErr:  true,
			errMessage: repo.ErrNotFound,
		},
		{
			name: "SystemUnlink",
			workflowMut: func(w *model.Workflow) {
				w.ArtifactID = systems[1].ID
				w.ArtifactType = workflow.ArtifactTypeSystem.String()
				w.ActionType = workflow.ActionTypeUnlink.String()
				w.Approvers = nil
			},
			approversCount: 2,
		},
		{
			name: "SystemUnLink - Invalid system",
			workflowMut: func(w *model.Workflow) {
				w.ArtifactID = uuid.New()
				w.ArtifactType = workflow.ArtifactTypeSystem.String()
				w.ActionType = workflow.ActionTypeUnlink.String()
				w.Approvers = nil
			},
			expectErr:  true,
			errMessage: repo.ErrNotFound,
		},
		{
			name: "SystemSwitch",
			workflowMut: func(w *model.Workflow) {
				w.ArtifactID = systems[1].ID
				w.ArtifactType = workflow.ArtifactTypeSystem.String()
				w.ActionType = workflow.ActionTypeSwitch.String()
				w.Parameters = keyConfigs[1].ID.String()
				w.Approvers = nil
			},
			approversCount: 4,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				wf := testutils.NewWorkflow(tt.workflowMut)
				err = r.Create(ctx, wf)
				assert.NoError(t, err)

				_, err = m.AutoAssignApprovers(
					context.WithValue(
						ctx,
						constants.ClientData, &auth.ClientData{
							Identifier: constants.SystemUser.String(),
						},
					), wf.ID,
				)
				if tt.expectErr {
					assert.Error(t, err)
					assert.ErrorIs(t, err, tt.errMessage)
				} else {
					assert.NoError(t, err)

					approvers, _, err := m.ListWorkflowApprovers(ctx, wf.ID, 0, 100)
					assert.NoError(t, err)

					assert.Len(t, approvers, tt.approversCount)
				}
			},
		)
	}
}

func TestWorkflowManager_CreateWorkflowTransitionNotificationTask(t *testing.T) {
	cfg := &config.Config{}
	wm, _, tenantID := SetupWorkflowManager(t, cfg)

	t.Run(
		"should successfully create and enqueue notification task", func(t *testing.T) {
			// Arrange
			ctx := testutils.CreateCtxWithTenant(tenantID)

			mockClient := &async.MockClient{}
			wm.SetAsyncClient(mockClient)

			wf := model.Workflow{
				ID:           uuid.New(),
				ActionType:   "CREATE",
				ArtifactType: "KEY",
				ArtifactID:   uuid.New(),
			}

			recipients := []string{"approver1@example.com", "approver2@example.com"}

			// Act
			err := wm.CreateWorkflowTransitionNotificationTask(ctx, wf, workflow.TransitionApprove, recipients)

			// Assert
			assert.NoError(t, err)
			assert.Equal(t, 1, mockClient.CallCount)
			assert.NotNil(t, mockClient.LastTask)
		},
	)

	t.Run(
		"should skip notification when async client is nil", func(t *testing.T) {
			// Arrange
			ctx := testutils.CreateCtxWithTenant(tenantID)

			wf := model.Workflow{
				ID:           uuid.New(),
				ActionType:   "CREATE",
				ArtifactType: "KEY",
				ArtifactID:   uuid.New(),
			}

			recipients := []string{"approver@example.com"}

			// Act
			err := wm.CreateWorkflowTransitionNotificationTask(ctx, wf, workflow.TransitionCreate, recipients)

			// Assert
			assert.NoError(t, err)
		},
	)

	t.Run(
		"should skip notification when recipients list is empty", func(t *testing.T) {
			// Arrange
			ctx := testutils.CreateCtxWithTenant(tenantID)

			mockClient := &async.MockClient{}
			wm.SetAsyncClient(mockClient)

			wf := model.Workflow{
				ID:           uuid.New(),
				ActionType:   "CREATE",
				ArtifactType: "KEY",
				ArtifactID:   uuid.New(),
			}

			var recipients []string

			// Act
			err := wm.CreateWorkflowTransitionNotificationTask(ctx, wf, workflow.TransitionApprove, recipients)

			// Assert
			assert.NoError(t, err)
			assert.Equal(t, 0, mockClient.CallCount)
		},
	)

	t.Run(
		"should return error when GetTenant fails", func(t *testing.T) {
			// Arrange
			ctx := t.Context()

			mockClient := &async.MockClient{}
			wm.SetAsyncClient(mockClient)

			wf := model.Workflow{
				ID:           uuid.New(),
				ActionType:   "CREATE",
				ArtifactType: "KEY",
				ArtifactID:   uuid.New(),
			}

			recipients := []string{"approver@example.com"}

			// Act
			err := wm.CreateWorkflowTransitionNotificationTask(ctx, wf, workflow.TransitionCreate, recipients)

			// Assert
			assert.Error(t, err)
		},
	)

	t.Run(
		"should return error when async client enqueue fails", func(t *testing.T) {
			// Arrange
			ctx := testutils.CreateCtxWithTenant(tenantID)

			expectedError := ErrEnqueuingTask
			mockClient := &async.MockClient{Error: expectedError}
			wm.SetAsyncClient(mockClient)

			wf := model.Workflow{
				ID:           uuid.New(),
				ActionType:   "CREATE",
				ArtifactType: "KEY",
				ArtifactID:   uuid.New(),
			}

			recipients := []string{"approver@example.com"}

			// Act
			err := wm.CreateWorkflowTransitionNotificationTask(ctx, wf, workflow.TransitionApprove, recipients)

			// Assert
			assert.Error(t, err)
			assert.Equal(t, expectedError, err)
			assert.Equal(t, 1, mockClient.CallCount)
		},
	)

	t.Run(
		"should handle different workflow transitions", func(t *testing.T) {
			// Arrange
			ctx := testutils.CreateCtxWithTenant(tenantID)

			mockClient := &async.MockClient{}
			wm.SetAsyncClient(mockClient)

			wf := model.Workflow{
				ID:           uuid.New(),
				ActionType:   "CREATE",
				ArtifactType: "KEY",
				ArtifactID:   uuid.New(),
				State:        string(workflow.StateSuccessful),
			}

			recipients := []string{"user@example.com"}

			transitions := []workflow.Transition{
				workflow.TransitionCreate,
				workflow.TransitionApprove,
				workflow.TransitionReject,
				workflow.TransitionConfirm,
				workflow.TransitionRevoke,
			}

			// Act & Assert
			for _, transition := range transitions {
				err := wm.CreateWorkflowTransitionNotificationTask(ctx, wf, transition, recipients)
				assert.NoError(t, err)
			}

			assert.Equal(t, len(transitions), mockClient.CallCount)
		},
	)
}

func TestWorkflowManager_CleanupTerminalWorkflows(t *testing.T) {
	cfg := &config.Config{}
	wm, r, tenantID := SetupWorkflowManager(t, cfg)
	ctx := testutils.CreateCtxWithTenant(tenantID)

	// Create workflow config
	workflowConfig := testutils.NewWorkflowConfig(func(_ *model.TenantConfig) {})
	testutils.CreateTestEntities(ctx, t, r, workflowConfig)

	t.Run(
		"should delete expired terminal workflow", func(t *testing.T) {
			// Create old terminal workflow (should be deleted)
			oldTerminalWf := testutils.NewWorkflow(
				func(w *model.Workflow) {
					w.State = workflow.StateSuccessful.String()
					w.CreatedAt = time.Now().AddDate(0, 0, -31) // 31 days ago
				},
			)

			testutils.CreateTestEntities(ctx, t, r, oldTerminalWf)

			err := wm.CleanupTerminalWorkflows(ctx)
			assert.NoError(t, err)

			// Verify old terminal workflow was deleted
			_, err = wm.GetWorkflowsByID(ctx, oldTerminalWf.ID)
			assert.ErrorIs(t, err, repo.ErrNotFound)

			// Verify workflow approvers were also deleted
			var approversAfter []*model.WorkflowApprover
			approverQuery := repo.NewQuery().Where(
				repo.NewCompositeKeyGroup(
					repo.NewCompositeKey().Where(model.WorkflowID, oldTerminalWf.ID),
				),
			)
			countAfter, err := r.List(ctx, &model.WorkflowApprover{}, &approversAfter, *approverQuery)
			assert.NoError(t, err)
			assert.Equal(t, 0, countAfter, "Approvers should be deleted with workflow")
		},
	)

	t.Run(
		"should not delete recent terminal workflow", func(t *testing.T) {
			// Create recent terminal workflow (should NOT be deleted)
			recentTerminalWf := testutils.NewWorkflow(
				func(w *model.Workflow) {
					w.State = workflow.StateRejected.String()
					w.CreatedAt = time.Now().AddDate(0, 0, -15) // 15 days ago
				},
			)

			testutils.CreateTestEntities(ctx, t, r, recentTerminalWf)

			err := wm.CleanupTerminalWorkflows(testutils.CreateCtxWithTenant(tenantID))
			assert.NoError(t, err)

			// Verify recent terminal workflow still exists
			_, err = wm.GetWorkflowsByID(ctx, recentTerminalWf.ID)
			assert.NoError(t, err)

			// Verify workflow approvers still exist
			var approvers []*model.WorkflowApprover
			approverQuery := repo.NewQuery().Where(
				repo.NewCompositeKeyGroup(
					repo.NewCompositeKey().Where(model.WorkflowID, recentTerminalWf.ID),
				),
			)
			count, err := r.List(ctx, &model.WorkflowApprover{}, &approvers, *approverQuery)
			assert.NoError(t, err)
			assert.Positive(t, count, "Approvers should still exist for recent workflow")
		},
	)

	t.Run(
		"should not delete old non-terminal workflow", func(t *testing.T) {
			// Create old non-terminal workflow (should NOT be deleted)
			oldActiveWf := testutils.NewWorkflow(
				func(w *model.Workflow) {
					w.State = workflow.StateWaitApproval.String()
					w.CreatedAt = time.Now().AddDate(0, 0, -31) // 31 days ago
				},
			)

			testutils.CreateTestEntities(ctx, t, r, oldActiveWf)

			err := wm.CleanupTerminalWorkflows(testutils.CreateCtxWithTenant(tenantID))
			assert.NoError(t, err)

			// Verify old active workflow still exists
			_, err = wm.GetWorkflowsByID(ctx, oldActiveWf.ID)
			assert.NoError(t, err)
		},
	)

	t.Run(
		"should delete all terminal state types", func(t *testing.T) {
			// Create workflows in all terminal states (all old enough to be deleted)
			terminalStates := workflow.TerminalStates

			workflowIDs := make([]uuid.UUID, len(terminalStates))
			for i, state := range terminalStates {
				wf := testutils.NewWorkflow(
					func(w *model.Workflow) {
						w.State = state
						w.CreatedAt = time.Now().AddDate(0, 0, -31)
					},
				)
				testutils.CreateTestEntities(ctx, t, r, wf)
				workflowIDs[i] = wf.ID
			}

			err := wm.CleanupTerminalWorkflows(testutils.CreateCtxWithTenant(tenantID))
			assert.NoError(t, err)

			// Verify all terminal workflows were deleted
			for i, wfID := range workflowIDs {
				_, err = wm.GetWorkflowsByID(ctx, wfID)
				assert.ErrorIs(
					t, err, repo.ErrNotFound, "Terminal workflow in state %s should be deleted", terminalStates[i],
				)
			}
		},
	)

	t.Run(
		"should handle batch processing for large number of workflows", func(t *testing.T) {
			// Create more workflows than batch size to test batch processing
			total := 101 // More than repo.DefaultLimit (100)
			workflowIDs := make([]uuid.UUID, total)

			for i := range total {
				wf := testutils.NewWorkflow(
					func(w *model.Workflow) {
						w.State = workflow.StateSuccessful.String()
						w.CreatedAt = time.Now().AddDate(0, 0, -31)
					},
				)
				testutils.CreateTestEntities(ctx, t, r, wf)
				workflowIDs[i] = wf.ID
			}

			err := wm.CleanupTerminalWorkflows(ctx)
			assert.NoError(t, err)

			// Verify all workflows were deleted across multiple batches
			for _, wfID := range workflowIDs {
				_, err = wm.GetWorkflowsByID(ctx, wfID)
				assert.ErrorIs(t, err, repo.ErrNotFound, "All workflows should be deleted even with batch processing")
			}
		},
	)

	t.Run(
		"should handle empty result when no expired workflows exist", func(t *testing.T) {
			// Create only recent terminal workflows
			recentWf := testutils.NewWorkflow(
				func(w *model.Workflow) {
					w.State = workflow.StateSuccessful.String()
					w.CreatedAt = time.Now().AddDate(0, 0, -5)
				},
			)
			testutils.CreateTestEntities(ctx, t, r, recentWf)

			// Should not error when no workflows to delete
			err := wm.CleanupTerminalWorkflows(testutils.CreateCtxWithTenant(tenantID))
			assert.NoError(t, err)

			// Recent workflow should still exist
			_, err = wm.GetWorkflowsByID(ctx, recentWf.ID)
			assert.NoError(t, err)
		},
	)

	t.Run(
		"should handle workflows without approvers", func(t *testing.T) {
			// Create workflow without approvers
			oldWf := testutils.NewWorkflow(
				func(w *model.Workflow) {
					w.State = workflow.StateSuccessful.String()
					w.CreatedAt = time.Now().AddDate(0, 0, -31)
					w.Approvers = nil // No approvers
				},
			)
			testutils.CreateTestEntities(ctx, t, r, oldWf)

			err := wm.CleanupTerminalWorkflows(testutils.CreateCtxWithTenant(tenantID))
			assert.NoError(t, err)

			// Workflow should still be deleted even without approvers
			_, err = wm.GetWorkflowsByID(ctx, oldWf.ID)
			assert.ErrorIs(t, err, repo.ErrNotFound)
		},
	)

	t.Run(
		"should preserve non-terminal workflow states", func(t *testing.T) {
			// Create workflows in all non-terminal states (all old)
			nonTerminalStates := workflow.NonTerminalStates

			workflowIDs := make([]uuid.UUID, len(nonTerminalStates))
			for i, state := range nonTerminalStates {
				wf := testutils.NewWorkflow(
					func(w *model.Workflow) {
						w.State = state
						w.CreatedAt = time.Now().AddDate(0, 0, -60) // Very old
					},
				)
				testutils.CreateTestEntities(ctx, t, r, wf)
				workflowIDs[i] = wf.ID
			}

			err := wm.CleanupTerminalWorkflows(testutils.CreateCtxWithTenant(tenantID))
			assert.NoError(t, err)

			// Verify all non-terminal workflows still exist
			for i, wfID := range workflowIDs {
				_, err = wm.GetWorkflowsByID(ctx, wfID)
				assert.NoError(t, err, "Non-terminal workflow in state %s should not be deleted", nonTerminalStates[i])
			}
		},
	)
}
