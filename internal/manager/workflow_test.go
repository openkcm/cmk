package manager_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/auth"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/api/cmkapi"
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
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

var ErrEnqueuingTask = errors.New("error enqueuing task")

var auditorGroupName = "auditors"

func createAuditorGroup(ctx context.Context, tb testing.TB, r repo.Repo) {
	tb.Helper()

	group := testutils.NewGroup(func(g *model.Group) {
		g.Name = auditorGroupName
		g.IAMIdentifier = auditorGroupName
		g.Role = constants.TenantAuditorRole
	})
	testutils.CreateTestEntities(ctx, tb, r, group)
}

func SetupWorkflowManager(t *testing.T, cfg *config.Config,
	opts ...testutils.TestDBConfigOpt) (
	*manager.WorkflowManager,
	repo.Repo, string,
) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})

	r := sql.NewRepository(db)

	ctlg, err := catalog.New(t.Context(), cfg)
	assert.NoError(t, err)

	tenantConfigManager := manager.NewTenantConfigManager(r, ctlg, nil)
	certManager := manager.NewCertificateManager(t.Context(), r, ctlg, &cfg.Certificates)
	cmkAuditor := auditor.New(t.Context(), cfg)
	userManager := manager.NewUserManager(r, cmkAuditor)
	tagManager := manager.NewTagManager(r)
	keyConfigManager := manager.NewKeyConfigManager(r, certManager, userManager, tagManager, cmkAuditor, cfg)
	groupManager := manager.NewGroupManager(r, ctlg, userManager)

	clientsFactory, err := clients.NewFactory(cfg.Services)
	assert.NoError(t, err)
	systemManager := manager.NewSystemManager(t.Context(), r, clientsFactory, nil, ctlg, cfg, keyConfigManager, userManager)

	keym := manager.NewKeyManager(r, ctlg, tenantConfigManager, keyConfigManager, userManager, certManager, nil, cmkAuditor)
	m := manager.NewWorkflowManager(
		r, keym, keyConfigManager, systemManager,
		groupManager, userManager, nil, tenantConfigManager, cfg,
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

func createTestObjects(t *testing.T, repo repo.Repo, ctx context.Context) (*model.KeyConfiguration,
	*model.Key,
) {
	t.Helper()

	key := testutils.NewKey(func(k *model.Key) {
		k.ID = uuid.New()
	})

	// Create test key configuration once for all tests
	keyConfig := testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
		c.PrimaryKeyID = &key.ID
	})

	testutils.CreateTestEntities(
		ctx,
		t,
		repo,
		key,
		keyConfig,
	)

	return keyConfig, key
}

func TestWorkflowManager_CheckWorkflow(t *testing.T) {
	m, repo, tenant := SetupWorkflowManager(t, &config.Config{})

	ctx := testutils.CreateCtxWithTenant(tenant)
	workflowConfig := testutils.NewWorkflowConfig(func(_ *model.TenantConfig) {})
	testutils.CreateTestEntities(ctx, t, repo, workflowConfig)

	keyConfig, key := createTestObjects(t, repo, ctx)
	createAuditorGroup(ctx, t, repo)

	ctxSys := context.WithValue(
		ctx,
		constants.ClientData, &auth.ClientData{
			Identifier: constants.SystemUser.String(),
		},
	)

	t.Run("Should return false on canCreate and error on non existing artifacts", func(t *testing.T) {
		status, err := m.CheckWorkflow(ctx, &model.Workflow{})
		assert.False(t, status.Enabled)
		assert.False(t, status.Exists)
		assert.False(t, status.Valid)
		assert.False(t, status.CanCreate)
		assert.Error(t, err)
	},
	)

	t.Run("Should return be valid and cant create on existing active workflow", func(t *testing.T) {
		wf, err := createTestWorkflow(
			ctxSys, repo, testutils.NewWorkflow(
				func(w *model.Workflow) {
					w.State = workflow.StateInitial.String()
					w.ActionType = workflow.ActionTypeDelete.String()
					w.ArtifactID = key.ID
					w.ArtifactType = workflow.ArtifactTypeKey.String()
				},
			),
		)
		assert.NoError(t, err)

		status, err := m.CheckWorkflow(ctxSys, wf)
		assert.True(t, status.Enabled)
		assert.True(t, status.Exists)
		assert.True(t, status.Valid)
		assert.False(t, status.CanCreate)
		assert.NoError(t, err)
	})

	t.Run("Should be invalid and cant create on system connect with invalid key state", func(t *testing.T) {
		groupIAM := uuid.NewString()
		ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{groupIAM})
		key := testutils.NewKey(func(k *model.Key) {
			k.State = string(cmkapi.KeyStateFORBIDDEN)
		})

		testGroup := testutils.NewGroup(
			func(g *model.Group) {
				g.IAMIdentifier = groupIAM
			},
		)

		keyConfig := testutils.NewKeyConfig(func(kc *model.KeyConfiguration) {
			kc.PrimaryKeyID = ptr.PointTo(key.ID)
			kc.AdminGroup = *testGroup
			kc.AdminGroupID = testGroup.ID
		})
		system := testutils.NewSystem(func(s *model.System) {
			s.KeyConfigurationID = ptr.PointTo(keyConfig.ID)
		})
		testutils.CreateTestEntities(ctx, t, repo, key, testGroup, keyConfig, system)

		wf, err := createTestWorkflow(
			ctx, repo, testutils.NewWorkflow(
				func(w *model.Workflow) {
					w.State = workflow.StateInitial.String()
					w.ActionType = workflow.ActionTypeLink.String()
					w.ArtifactID = system.ID
					w.ArtifactType = workflow.ArtifactTypeSystem.String()
					w.Parameters = keyConfig.ID.String()
				},
			),
		)
		assert.NoError(t, err)

		status, err := m.CheckWorkflow(ctx, wf)
		assert.True(t, status.Enabled)
		assert.False(t, status.Exists)
		assert.False(t, status.Valid)
		assert.False(t, status.CanCreate)
		assert.NoError(t, err)
	})

	t.Run("Should be invalid and cant create on system connect without pkey", func(t *testing.T) {
		groupIAM := uuid.NewString()
		ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{groupIAM})
		testGroup := testutils.NewGroup(
			func(g *model.Group) {
				g.IAMIdentifier = groupIAM
			},
		)
		keyConfig := testutils.NewKeyConfig(func(kc *model.KeyConfiguration) {
			kc.AdminGroup = *testGroup
			kc.AdminGroupID = testGroup.ID
		})
		system := testutils.NewSystem(func(s *model.System) {
			s.KeyConfigurationID = ptr.PointTo(keyConfig.ID)
		})
		testutils.CreateTestEntities(ctx, t, repo, testGroup, keyConfig, system)

		wf, err := createTestWorkflow(
			ctx, repo, testutils.NewWorkflow(
				func(w *model.Workflow) {
					w.State = workflow.StateInitial.String()
					w.ActionType = workflow.ActionTypeLink.String()
					w.ArtifactID = system.ID
					w.ArtifactType = workflow.ArtifactTypeSystem.String()
					w.Parameters = keyConfig.ID.String()
				},
			),
		)
		assert.NoError(t, err)

		status, err := m.CheckWorkflow(ctx, wf)
		assert.True(t, status.Enabled)
		assert.False(t, status.Exists)
		assert.True(t, status.Enabled)
		assert.False(t, status.Exists)
		assert.False(t, status.Valid)
		assert.False(t, status.CanCreate)
		assert.NoError(t, err)
	})

	t.Run("Should be creatable on rejected previous workflow", func(t *testing.T) {
		wf, err := createTestWorkflow(
			ctxSys, repo, testutils.NewWorkflow(
				func(w *model.Workflow) {
					w.State = workflow.StateInitial.String()
					w.State = workflow.StateRejected.String()
					w.ActionType = workflow.ActionTypeUpdatePrimary.String()
					w.ArtifactID = keyConfig.ID
					w.ArtifactType = workflow.ArtifactTypeKeyConfiguration.String()
				},
			),
		)
		assert.NoError(t, err)

		status, err := m.CheckWorkflow(ctxSys, wf)
		assert.True(t, status.Enabled)
		assert.False(t, status.Exists)
		assert.True(t, status.Valid)
		assert.True(t, status.CanCreate)
		assert.NoError(t, err)
	},
	)

	t.Run(
		"Should return authorization error on non active artifact", func(t *testing.T) {
			wf, err := createTestWorkflow(
				ctxSys, repo, testutils.NewWorkflow(
					func(w *model.Workflow) {
						w.State = workflow.StateRejected.String()
						w.ActionType = workflow.ActionTypeDelete.String()
						w.ArtifactType = workflow.ArtifactTypeKey.String()
					},
				),
			)
			assert.NoError(t, err)

			status, err := m.CheckWorkflow(ctxSys, wf)
			assert.False(t, status.Enabled)
			assert.False(t, status.Exists)
			assert.ErrorIs(t, err, manager.ErrWorkflowCreationNotAllowed)
		},
	)
}

func TestWorkflowManager_CreateWorkflow(t *testing.T) {
	m, repo, tenant := SetupWorkflowManager(t, &config.Config{
		ContextModels: config.ContextModels{
			System: config.System{
				OptionalProperties: map[string]config.SystemProperty{
					"NameOfTheSystem": {
						DisplayName: "Name",
						Optional:    true,
						Default:     "n/a",
					},
				},
			},
		},
	})

	ctx := testutils.CreateCtxWithTenant(tenant)
	createAuditorGroup(ctx, t, repo)

	ctxSys := context.WithValue(
		ctx,
		constants.ClientData, &auth.ClientData{
			Identifier: constants.SystemUser.String(),
		},
	)
	keyConfig, key := createTestObjects(t, repo, ctxSys)

	t.Run("Should error on existing workflow", func(t *testing.T) {
		wf := testutils.NewWorkflow(func(w *model.Workflow) {
			w.State = workflow.StateInitial.String()
			w.ActionType = workflow.ActionTypeDelete.String()
			w.ArtifactType = workflow.ArtifactTypeKey.String()
			w.ArtifactID = key.ID
		})
		err := repo.Create(ctx, wf)
		assert.NoError(t, err)

		_, err = m.CreateWorkflow(ctxSys, wf)
		assert.ErrorIs(t, err, manager.ErrOngoingWorkflowExist)
	},
	)

	t.Run("Should create workflow", func(t *testing.T) {
		_, key := createTestObjects(t, repo, ctxSys)
		wf := testutils.NewWorkflow(func(w *model.Workflow) {
			w.State = workflow.StateInitial.String()
			w.ActionType = workflow.ActionTypeDelete.String()
			w.ArtifactType = workflow.ArtifactTypeKey.String()
			w.ArtifactID = key.ID
		})
		res, err := m.CreateWorkflow(ctxSys, wf)
		assert.NoError(t, err)
		assert.Equal(t, wf, res)
	},
	)

	t.Run("Should create system workflow with artifact name from property", func(t *testing.T) {
		system := testutils.NewSystem(func(s *model.System) {
			s.Properties = map[string]string{
				"NameOfTheSystem": "MySystem",
			}
		})
		testutils.CreateTestEntities(ctxSys, t, repo, system)

		expected := &model.Workflow{
			ID:           uuid.New(),
			State:        "INITIAL",
			InitiatorID:  uuid.NewString(),
			ArtifactType: "SYSTEM",
			ArtifactID:   system.ID,
			ActionType:   "LINK",
			Approvers:    []model.WorkflowApprover{{UserID: uuid.NewString()}},
			Parameters:   keyConfig.ID.String(),
		}
		res, err := m.CreateWorkflow(ctxSys, expected)
		assert.NoError(t, err)
		assert.Equal(t, "MySystem", *res.ArtifactName)
		assert.Equal(t, keyConfig.Name, *res.ParametersResourceName)
	},
	)

	t.Run("Should create system workflow with artifact name from identifier", func(t *testing.T) {
		system := testutils.NewSystem(func(s *model.System) {})
		testutils.CreateTestEntities(ctxSys, t, repo, system)

		expected := &model.Workflow{
			ID:           uuid.New(),
			State:        "INITIAL",
			InitiatorID:  uuid.NewString(),
			ArtifactType: "SYSTEM",
			ArtifactID:   system.ID,
			ActionType:   "LINK",
			Approvers:    []model.WorkflowApprover{{UserID: uuid.NewString()}},
			Parameters:   keyConfig.ID.String(),
		}
		res, err := m.CreateWorkflow(ctxSys, expected)
		assert.NoError(t, err)
		assert.Equal(t, system.Identifier, *res.ArtifactName)
		assert.Equal(t, keyConfig.Name, *res.ParametersResourceName)
	},
	)
}

func TestWorkflowManager_TransitionWorkflow(t *testing.T) {
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

func TestWorkflowManager_GetWorkflowByID(t *testing.T) {
	m, r, tenant := SetupWorkflowManager(t, &config.Config{})
	userID := uuid.NewString()
	wf, err := createTestWorkflow(
		testutils.CreateCtxWithTenant(tenant),
		r,
		testutils.NewWorkflow(
			func(w *model.Workflow) {
				w.State = workflow.StateInitial.String()
				w.ActionType = workflow.ActionTypeDelete.String()
				w.ArtifactType = workflow.ArtifactTypeKey.String()
				w.InitiatorID = userID
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
			name:       "TestWorkflowManager_GetByID_ValidUUID",
			workflowID: wf.ID,
			expectErr:  false,
		},
		{
			name:       "TestWorkflowManager_GetByID_NonExistent",
			workflowID: uuid.New(),
			expectErr:  true,
			errMessage: manager.ErrWorkflowNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				ctx := cmkcontext.InjectClientData(
					cmkcontext.CreateTenantContext(t.Context(), tenant),
					&auth.ClientData{
						Identifier: userID,
					},
					nil,
				)
				retrievedWf, err := m.GetWorkflowByID(
					ctx, tt.workflowID,
				)
				if tt.expectErr {
					assert.Error(t, err)
					assert.Nil(t, retrievedWf)
					assert.ErrorIs(t, err, tt.errMessage)
				} else {
					assert.NoError(t, err)
					assert.NotNil(t, retrievedWf)
					assert.Equal(t, wf.ID, retrievedWf.ID)
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
) manager.WorkflowFilter {
	return manager.WorkflowFilter{
		State:        state,
		ArtifactType: artifactType,
		ArtifactID:   artifactID,
		ActionType:   actionType,
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

	baseTime := time.Now()

	workflow1, err := createTestWorkflow(
		testutils.CreateCtxWithTenant(tenant),
		repo,
		testutils.NewWorkflow(
			func(w *model.Workflow) {
				w.State = workflow.StateInitial.String()
				w.ActionType = workflow.ActionTypeDelete.String()
				w.ArtifactType = workflow.ArtifactTypeKey.String()
				w.Approvers = []model.WorkflowApprover{{UserID: allWorkflowUserID}}
				w.InitiatorID = userID
				w.CreatedAt = baseTime.Add(-3 * time.Hour)
				w.UpdatedAt = baseTime.Add(-3 * time.Hour)
			},
		),
	)
	assert.NoError(t, err)

	workflow2, err := createTestWorkflow(
		testutils.CreateCtxWithTenant(tenant),
		repo,
		testutils.NewWorkflow(
			func(w *model.Workflow) {
				w.State = workflow.StateInitial.String()
				w.ActionType = workflow.ActionTypeDelete.String()
				w.ArtifactType = workflow.ArtifactTypeKey.String()
				w.ArtifactID = artifactID
				w.Approvers = []model.WorkflowApprover{{UserID: userID}}
				w.InitiatorID = allWorkflowUserID
				w.CreatedAt = baseTime.Add(-2 * time.Hour)
				w.UpdatedAt = baseTime.Add(-2 * time.Hour)
			},
		),
	)
	assert.NoError(t, err)

	workflow3, err := createTestWorkflow(
		testutils.CreateCtxWithTenant(tenant),
		repo,
		testutils.NewWorkflow(
			func(w *model.Workflow) {
				w.State = workflow.StateRejected.String()
				w.ActionType = workflow.ActionTypeDelete.String()
				w.ArtifactType = workflow.ArtifactTypeKey.String()
				w.Approvers = []model.WorkflowApprover{{UserID: userID}}
				w.InitiatorID = allWorkflowUserID
				w.CreatedAt = baseTime.Add(-1 * time.Hour)
				w.UpdatedAt = baseTime.Add(-1 * time.Hour)
			},
		),
	)
	assert.NoError(t, err)

	workflow4, err := createTestWorkflow(
		testutils.CreateCtxWithTenant(tenant),
		repo,
		testutils.NewWorkflow(
			func(w *model.Workflow) {
				w.State = workflow.StateInitial.String()
				w.ActionType = workflow.ActionTypeUpdateState.String()
				w.ArtifactType = workflow.ArtifactTypeKey.String()
				w.Approvers = []model.WorkflowApprover{{UserID: allWorkflowUserID}}
				w.InitiatorID = userID
				w.CreatedAt = baseTime
				w.UpdatedAt = baseTime
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
			),
			expectedCount:       1,
			expectedState:       "",
			expectedActionType:  workflow.ActionTypeUpdateState.String(),
			expectedArtfactType: "",
		},
		{
			name: "Get workflows by artifact type",
			filter: newGetWorkflowsFilter(
				uuid.Nil,
				"",
				"",
				workflow.ArtifactTypeKey.String(),
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
						Identifier: userID,
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
						approvers, count, err := m.ListWorkflowApprovers(ctx, wf.ID, false, 0, 5)
						assert.NoError(t, err)
						assert.Equal(t, 1, count)
						assert.True(
							t,
							tc.expectedInitiatorID == wf.InitiatorID || tc.expectedInitiatorID == approvers[0].UserID,
						)
					}
				}
			},
		)
	}

	t.Run("Should return workflows ordered by created time descending", func(t *testing.T) {
		ctx := cmkcontext.InjectClientData(
			cmkcontext.CreateTenantContext(t.Context(), tenant),
			&auth.ClientData{
				Identifier: userID,
			},
			nil,
		)

		workflows, count, err := m.GetWorkflows(ctx, manager.WorkflowFilter{})
		assert.NoError(t, err)
		assert.Equal(t, 4, count)
		assert.Len(t, workflows, 4)

		// Verify workflows are ordered by created time descending (newest first)
		// workflow4 should be first (created last)
		assert.Equal(t, workflow4.ID, workflows[0].ID, "First workflow should be workflow4 (newest)")
		assert.Equal(t, workflow3.ID, workflows[1].ID, "Second workflow should be workflow3")
		assert.Equal(t, workflow2.ID, workflows[2].ID, "Third workflow should be workflow2")
		assert.Equal(t, workflow1.ID, workflows[3].ID, "Fourth workflow should be workflow1 (oldest)")
	})
}

func TestWorkflowManager_ListApprovers(t *testing.T) {
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

	ctx := testutils.CreateCtxWithTenant(tenant)

	createAuditorGroup(ctx, t, r)

	ctxSys := context.WithValue(
		ctx,
		constants.ClientData, &auth.ClientData{
			Identifier: constants.SystemUser.String(),
			Groups:     []string{auditorGroupName},
		},
	)

	tests := []struct {
		name       string
		workflowID uuid.UUID
		expectErr  bool
		errMessage error
	}{
		{
			name:       "TestWorkflowManager_ListApproversByWorkflowID_ValidUUID",
			workflowID: wf.ID,
			expectErr:  false,
		},
		{
			name:       "TestWorkflowManager_ListApproversByWorkflowID_NonExistent",
			workflowID: uuid.New(),
			expectErr:  true,
			errMessage: manager.ErrWorkflowNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				approvers, _, err := m.ListWorkflowApprovers(
					ctxSys, tt.workflowID, false,
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

	createAuditorGroup(ctx, t, r)

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
		approverGroups int
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
			approverGroups: 1,
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
			approverGroups: 1,
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
			approverGroups: 1,
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
			approverGroups: 1,
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
			approverGroups: 1,
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
			approverGroups: 1,
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
			approverGroups: 2,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				wf := testutils.NewWorkflow(tt.workflowMut)
				err = r.Create(ctx, wf)
				assert.NoError(t, err)

				// We need the auditor group here to allow listing approvers
				ctxSys := context.WithValue(
					ctx,
					constants.ClientData, &auth.ClientData{
						Identifier: constants.SystemUser.String(),
						Groups:     []string{auditorGroupName},
					},
				)
				_, err = m.AutoAssignApprovers(ctxSys, wf.ID)
				if tt.expectErr {
					assert.Error(t, err)
					assert.ErrorIs(t, err, tt.errMessage)
				} else {
					assert.NoError(t, err)

					count, _, err := m.ListWorkflowApprovers(ctxSys, wf.ID, false, 0, 0)
					assert.NoError(t, err)
					assert.Len(t, count, tt.approversCount)
				}
			},
		)
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
			State:        string(workflow.StateWaitConfirmation),
		}

		recipients := []string{"approver1@example.com", "approver2@example.com"}

		// Act
		err := wm.CreateWorkflowTransitionNotificationTask(ctx, wf, workflow.TransitionApprove, recipients)

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, 1, mockClient.CallCount)
		assert.NotNil(t, mockClient.LastTask)
	})

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
			err := wm.CreateWorkflowTransitionNotificationTask(ctx, wf, workflow.TransitionConfirm, recipients)

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
				State:        string(workflow.StateWaitConfirmation),
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

	userID := uuid.NewString()

	ctx := cmkcontext.InjectClientData(
		cmkcontext.CreateTenantContext(t.Context(), tenantID),
		&auth.ClientData{
			Identifier: userID,
		},
		nil,
	)

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
					w.InitiatorID = userID
				},
			)

			testutils.CreateTestEntities(ctx, t, r, oldTerminalWf)

			err := wm.CleanupTerminalWorkflows(ctx)
			assert.NoError(t, err)

			// Verify old terminal workflow was deleted
			_, err = wm.GetWorkflowByID(ctx, oldTerminalWf.ID)
			assert.ErrorIs(t, err, manager.ErrWorkflowNotAllowed)

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
					w.InitiatorID = userID
				},
			)

			testutils.CreateTestEntities(ctx, t, r, recentTerminalWf)

			err := wm.CleanupTerminalWorkflows(testutils.CreateCtxWithTenant(tenantID))
			assert.NoError(t, err)

			// Verify recent terminal workflow still exists
			_, err = wm.GetWorkflowByID(ctx, recentTerminalWf.ID)
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
					w.InitiatorID = userID
				},
			)

			testutils.CreateTestEntities(ctx, t, r, oldActiveWf)

			err := wm.CleanupTerminalWorkflows(testutils.CreateCtxWithTenant(tenantID))
			assert.NoError(t, err)

			// Verify old active workflow still exists
			_, err = wm.GetWorkflowByID(ctx, oldActiveWf.ID)
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
						w.InitiatorID = userID
					},
				)
				testutils.CreateTestEntities(ctx, t, r, wf)
				workflowIDs[i] = wf.ID
			}

			err := wm.CleanupTerminalWorkflows(testutils.CreateCtxWithTenant(tenantID))
			assert.NoError(t, err)

			// Verify all terminal workflows were deleted
			for i, wfID := range workflowIDs {
				_, err = wm.GetWorkflowByID(ctx, wfID)
				assert.ErrorIs(
					t, err, manager.ErrWorkflowNotAllowed,
					"Terminal workflow in state %s should be deleted", terminalStates[i],
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
						w.InitiatorID = userID
					},
				)
				testutils.CreateTestEntities(ctx, t, r, wf)
				workflowIDs[i] = wf.ID
			}

			err := wm.CleanupTerminalWorkflows(ctx)
			assert.NoError(t, err)

			// Verify all workflows were deleted across multiple batches
			for _, wfID := range workflowIDs {
				_, err = wm.GetWorkflowByID(ctx, wfID)
				assert.ErrorIs(t, err, manager.ErrWorkflowNotAllowed,
					"All workflows should be deleted even with batch processing")
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
					w.InitiatorID = userID
				},
			)
			testutils.CreateTestEntities(ctx, t, r, recentWf)

			// Should not error when no workflows to delete
			err := wm.CleanupTerminalWorkflows(testutils.CreateCtxWithTenant(tenantID))
			assert.NoError(t, err)

			// Recent workflow should still exist
			_, err = wm.GetWorkflowByID(ctx, recentWf.ID)
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
					w.InitiatorID = userID
				},
			)
			testutils.CreateTestEntities(ctx, t, r, oldWf)

			err := wm.CleanupTerminalWorkflows(testutils.CreateCtxWithTenant(tenantID))
			assert.NoError(t, err)

			// Workflow should still be deleted even without approvers
			_, err = wm.GetWorkflowByID(ctx, oldWf.ID)
			assert.ErrorIs(t, err, manager.ErrWorkflowNotAllowed)
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
						w.InitiatorID = userID
					},
				)
				testutils.CreateTestEntities(ctx, t, r, wf)
				workflowIDs[i] = wf.ID
			}

			err := wm.CleanupTerminalWorkflows(testutils.CreateCtxWithTenant(tenantID))
			assert.NoError(t, err)

			// Verify all non-terminal workflows still exist
			for i, wfID := range workflowIDs {
				_, err = wm.GetWorkflowByID(ctx, wfID)
				assert.NoError(t, err, "Non-terminal workflow in state %s should not be deleted", nonTerminalStates[i])
			}
		},
	)
}
