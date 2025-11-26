package manager_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/auth"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/async"
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

var ErrEnqueuingTask = errors.New("error enqueuing task")

func SetupWorkflowManager(t *testing.T, cfg *config.Config) (*manager.WorkflowManager,
	repo.Repo, string,
) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
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
	})

	r := sql.NewRepository(db)

	ctlg, err := catalog.New(t.Context(), *cfg)
	assert.NoError(t, err)

	tenantConfigManager := manager.NewTenantConfigManager(r, ctlg)
	certManager := manager.NewCertificateManager(t.Context(), r, ctlg, &cfg.Certificates)
	keyConfigManager := manager.NewKeyConfigManager(r, certManager, cfg)
	cmkAuditor := auditor.New(t.Context(), cfg)
	groupManager := manager.NewGroupManager(r, ctlg)

	clientsFactory, err := clients.NewFactory(cfg.Services)
	assert.NoError(t, err)
	systemManager := manager.NewSystemManager(t.Context(), r, clientsFactory, nil, ctlg, cmkAuditor, cfg)

	keym := manager.NewKeyManager(r, ctlg, tenantConfigManager, keyConfigManager, certManager, nil, cmkAuditor)
	m := manager.NewWorkflowManager(
		r, keym, keyConfigManager, systemManager,
		groupManager, nil, tenantConfigManager)

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
	t.Run("Should return disabled on workflow disabled", func(t *testing.T) {
		m, _, tenant := SetupWorkflowManager(t, &config.Config{})
		testutils.CreateCtxWithTenant(tenant)
		status, err := m.CheckWorkflow(testutils.CreateCtxWithTenant(tenant), &model.Workflow{})
		assert.False(t, status.Enabled)
		assert.False(t, status.Exists)
		assert.NoError(t, err)
	})

	m, repo, tenant := SetupWorkflowManager(t, &config.Config{})

	ctx := testutils.CreateCtxWithTenant(tenant)
	workflowConfig := testutils.NewWorkflowConfig(func(_ *model.TenantConfig) {})
	testutils.CreateTestEntities(ctx, t, repo, workflowConfig)

	t.Run("Should return false on non existing artifacts", func(t *testing.T) {
		status, err := m.CheckWorkflow(ctx, &model.Workflow{})
		assert.True(t, status.Enabled)
		assert.False(t, status.Exists)
		assert.NoError(t, err)
	})

	t.Run("Should return true on existing active artifact without parameters", func(t *testing.T) {
		wf, err := createTestWorkflow(ctx, repo, testutils.NewWorkflow(func(w *model.Workflow) {
			w.State = workflow.StateInitial.String()
			w.ActionType = workflow.ActionTypeDelete.String()
			w.ArtifactType = workflow.ArtifactTypeKey.String()
		}))
		assert.NoError(t, err)

		status, err := m.CheckWorkflow(ctx, wf)
		assert.True(t, status.Enabled)
		assert.True(t, status.Exists)
		assert.NoError(t, err)
	})

	t.Run("Should return true on active artifact requiring parameters", func(t *testing.T) {
		wf, err := createTestWorkflow(ctx, repo, testutils.NewWorkflow(func(w *model.Workflow) {
			w.State = workflow.StateInitial.String()
			w.ActionType = workflow.ActionTypeUpdatePrimary.String()
			w.ArtifactType = workflow.ArtifactTypeKeyConfiguration.String()
		}))
		assert.NoError(t, err)

		status, err := m.CheckWorkflow(ctx, wf)
		assert.True(t, status.Enabled)
		assert.True(t, status.Exists)
		assert.NoError(t, err)
	})

	t.Run("Should return false on artifact requiring parameters", func(t *testing.T) {
		wf, err := createTestWorkflow(ctx, repo, testutils.NewWorkflow(func(w *model.Workflow) {
			w.State = workflow.StateInitial.String()
			w.ActionType = workflow.ActionTypeUpdatePrimary.String()
			w.ArtifactType = workflow.ArtifactTypeKeyConfiguration.String()
		}))
		assert.NoError(t, err)

		wf.Parameters = uuid.NewString()

		status, err := m.CheckWorkflow(ctx, wf)
		assert.True(t, status.Enabled)
		assert.False(t, status.Exists)
		assert.NoError(t, err)
	})

	t.Run("Should return false on non active artifact", func(t *testing.T) {
		wf, err := createTestWorkflow(ctx, repo, testutils.NewWorkflow(func(w *model.Workflow) {
			w.State = workflow.StateRejected.String()
			w.ActionType = workflow.ActionTypeDelete.String()
			w.ArtifactType = workflow.ArtifactTypeKey.String()
		}))
		assert.NoError(t, err)

		status, err := m.CheckWorkflow(ctx, wf)
		assert.True(t, status.Enabled)
		assert.False(t, status.Exists)
		assert.NoError(t, err)
	})
}

func TestWorkflowManager_CreateWorkflow(t *testing.T) {
	m, repo, tenant := SetupWorkflowManager(t, &config.Config{})

	t.Run("Should error on existing workflow", func(t *testing.T) {
		wf, err := createTestWorkflow(
			testutils.CreateCtxWithTenant(tenant),
			repo,
			testutils.NewWorkflow(func(w *model.Workflow) {
				w.State = workflow.StateInitial.String()
				w.ActionType = workflow.ActionTypeDelete.String()
				w.ArtifactType = workflow.ArtifactTypeKey.String()
			}),
		)
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
	m, repo, tenant := SetupWorkflowManager(t, &config.Config{})

	ctx := testutils.CreateCtxWithTenant(tenant)
	workflowConfig := testutils.NewWorkflowConfig(func(_ *model.TenantConfig) {})

	testutils.CreateTestEntities(ctx, t, repo, workflowConfig)

	t.Run("Should error on invalid event actor", func(t *testing.T) {
		wf, err := createTestWorkflow(
			testutils.CreateCtxWithTenant(tenant),
			repo,
			testutils.NewWorkflow(func(w *model.Workflow) {
				w.State = workflow.StateInitial.String()
				w.ActionType = workflow.ActionTypeDelete.String()
				w.ArtifactType = workflow.ArtifactTypeKey.String()
			}),
		)
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
		wf, err := createTestWorkflow(
			testutils.CreateCtxWithTenant(tenant),
			repo,
			testutils.NewWorkflow(func(w *model.Workflow) {
				w.State = workflow.StateWaitApproval.String()
				w.ActionType = workflow.ActionTypeDelete.String()
				w.ArtifactType = workflow.ArtifactTypeKey.String()
			}),
		)
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
		wf, err := createTestWorkflow(
			testutils.CreateCtxWithTenant(tenant),
			repo,
			testutils.NewWorkflow(func(w *model.Workflow) {
				w.State = workflow.StateWaitApproval.String()
				w.ActionType = workflow.ActionTypeDelete.String()
				w.ArtifactType = workflow.ArtifactTypeKey.String()
			}),
		)
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
	m, r, tenant := SetupWorkflowManager(t, &config.Config{})
	wf, err := createTestWorkflow(
		testutils.CreateCtxWithTenant(tenant),
		r,
		testutils.NewWorkflow(func(w *model.Workflow) {
			w.State = workflow.StateInitial.String()
			w.ActionType = workflow.ActionTypeDelete.String()
			w.ArtifactType = workflow.ArtifactTypeKey.String()
		}),
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

func newGetWorkflowsFilter(
	artifactID uuid.UUID,
	state string,
	actionType string,
	artifactType string,
	userID uuid.UUID,
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

func TestWorkfowManager_GetWorkflows(t *testing.T) {
	m, repo, tenant := SetupWorkflowManager(t, &config.Config{})
	userID := uuid.New()
	allWorkflowUserID := uuid.New()
	artifactID := uuid.New()

	_, err := createTestWorkflow(
		testutils.CreateCtxWithTenant(tenant),
		repo,
		testutils.NewWorkflow(func(w *model.Workflow) {
			w.State = workflow.StateInitial.String()
			w.ActionType = workflow.ActionTypeDelete.String()
			w.ArtifactType = workflow.ArtifactTypeKey.String()
			w.Approvers = []model.WorkflowApprover{{UserID: allWorkflowUserID}}
			w.InitiatorID = userID
		}),
	)
	assert.NoError(t, err)

	_, err = createTestWorkflow(
		testutils.CreateCtxWithTenant(tenant),
		repo,
		testutils.NewWorkflow(func(w *model.Workflow) {
			w.State = workflow.StateInitial.String()
			w.ActionType = workflow.ActionTypeDelete.String()
			w.ArtifactType = workflow.ArtifactTypeKey.String()
			w.ArtifactID = artifactID
			w.Approvers = []model.WorkflowApprover{{UserID: uuid.New()}}
			w.InitiatorID = allWorkflowUserID
		}),
	)

	assert.NoError(t, err)
	_, err = createTestWorkflow(
		testutils.CreateCtxWithTenant(tenant),
		repo,
		testutils.NewWorkflow(func(w *model.Workflow) {
			w.State = workflow.StateRejected.String()
			w.ActionType = workflow.ActionTypeDelete.String()
			w.ArtifactType = workflow.ArtifactTypeKey.String()
			w.Approvers = []model.WorkflowApprover{{UserID: uuid.New()}}
			w.InitiatorID = allWorkflowUserID
		}),
	)
	assert.NoError(t, err)
	_, err = createTestWorkflow(
		testutils.CreateCtxWithTenant(tenant),
		repo,
		testutils.NewWorkflow(func(w *model.Workflow) {
			w.State = workflow.StateInitial.String()
			w.ActionType = workflow.ActionTypeUpdateState.String()
			w.ArtifactType = workflow.ArtifactTypeKey.String()
			w.Approvers = []model.WorkflowApprover{{UserID: allWorkflowUserID}}
			w.InitiatorID = userID
		}),
	)
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
			filter: newGetWorkflowsFilter(
				uuid.Nil,
				workflow.StateInitial.String(),
				"",
				"",
				uuid.Nil,
			),
			expectedCount:       3,
			expectedState:       workflow.StateInitial.String(),
			expectedActionType:  "",
			expectedInitiatorID: uuid.Nil,
		},
		{
			name: "Should get action type UPDATE_STATE workflows",
			filter: newGetWorkflowsFilter(
				uuid.Nil,
				"",
				workflow.ActionTypeUpdateState.String(),
				"",
				uuid.Nil,
			),
			expectedCount:       1,
			expectedState:       "",
			expectedActionType:  workflow.ActionTypeUpdateState.String(),
			expectedArtfactType: "",
			expectedInitiatorID: uuid.Nil,
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
			expectedInitiatorID: uuid.Nil,
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
			expectedInitiatorID: uuid.Nil,
		},
		{
			name: "Get workflows by artifact type",
			filter: newGetWorkflowsFilter(
				uuid.Nil,
				"",
				"",
				workflow.ArtifactTypeKey.String(),
				uuid.Nil,
			),
			expectedCount:       4,
			expectedState:       "",
			expectedActionType:  "",
			expectedArtfactType: workflow.ArtifactTypeKey.String(),
			expectedInitiatorID: uuid.Nil,
		},
		{
			name: "Get workflows by artifact id",
			filter: newGetWorkflowsFilter(
				artifactID,
				"",
				"",
				workflow.ArtifactTypeKey.String(),
				uuid.Nil,
			),
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
	m, r, tenant := SetupWorkflowManager(t, &config.Config{})
	wf, err := createTestWorkflow(
		testutils.CreateCtxWithTenant(tenant),
		r,
		testutils.NewWorkflow(func(w *model.Workflow) {
			w.State = workflow.StateInitial.String()
			w.ActionType = workflow.ActionTypeDelete.String()
			w.ArtifactType = workflow.ArtifactTypeKey.String()
		}),
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

func TestWorkflowManager_AutoAddApprover(t *testing.T) {
	m, r, tenant := SetupWorkflowManager(t, &config.Config{
		Plugins: testutils.SetupMockPlugins(testutils.IdentityPlugin),
	})
	ctx := testutils.CreateCtxWithTenant(tenant)

	adminGroups := []*model.Group{
		{ID: uuid.New(), Name: "group1", IAMIdentifier: "KMS_001", Role: constants.KeyAdminRole},
		{ID: uuid.New(), Name: "group2", IAMIdentifier: "KMS_002", Role: constants.KeyAdminRole},
	}
	keyConfigs := make([]*model.KeyConfiguration, len(adminGroups))

	for i, g := range adminGroups {
		err := r.Create(ctx, g)
		assert.NoError(t, err)

		keyConfig := testutils.NewKeyConfig(func(kc *model.KeyConfiguration) {
			kc.AdminGroup = *g
		})
		err = r.Create(ctx, keyConfig)
		assert.NoError(t, err)

		keyConfigs[i] = keyConfig
	}

	key := testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = keyConfigs[0].ID
	})

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
		t.Run(tt.name, func(t *testing.T) {
			wf := testutils.NewWorkflow(tt.workflowMut)
			err = r.Create(ctx, wf)
			assert.NoError(t, err)

			_, err = m.AutoAssignApprovers(context.WithValue(ctx, constants.ClientData, &auth.ClientData{}), wf.ID)
			if tt.expectErr {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.errMessage)
			} else {
				assert.NoError(t, err)

				approvers, _, err := m.ListWorkflowApprovers(ctx, wf.ID, 0, 100)
				assert.NoError(t, err)

				assert.Len(t, approvers, tt.approversCount)
			}
		})
	}
}

func TestWorkflowManager_CreateWorkflowTransitionNotificationTask(t *testing.T) {
	cfg := &config.Config{}
	wm, _, tenantID := SetupWorkflowManager(t, cfg)

	t.Run("should successfully create and enqueue notification task", func(t *testing.T) {
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
	})

	t.Run("should skip notification when async client is nil", func(t *testing.T) {
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
	})

	t.Run("should skip notification when recipients list is empty", func(t *testing.T) {
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
	})

	t.Run("should return error when GetTenant fails", func(t *testing.T) {
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
	})

	t.Run("should return error when async client enqueue fails", func(t *testing.T) {
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
	})

	t.Run("should handle different workflow transitions", func(t *testing.T) {
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
	})
}
