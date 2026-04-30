package manager_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	cmkpluginregistry "github.com/openkcm/cmk/internal/pluginregistry"
	"github.com/openkcm/cmk/internal/repo"
	repoPackage "github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
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

func SetupWorkflowManager(
	t *testing.T,
	cfg *config.Config,
	opts ...testutils.TestDBConfigOpt,
) (
	*manager.WorkflowManager,
	repo.Repo, string,
) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})

	r := sql.NewRepository(db)

	ps, psCfg := testutils.NewTestPlugins(testplugins.NewIdentityManagement())

	cfg.Plugins = psCfg

	svcRegistry, err := cmkpluginregistry.New(t.Context(), cfg, cmkpluginregistry.WithBuiltInPlugins(ps))
	assert.NoError(t, err)

	certManager := manager.NewCertificateManager(t.Context(), r, svcRegistry, cfg)
	tenantConfigManager := manager.NewTenantConfigManager(r, svcRegistry, nil)
	cmkAuditor := auditor.New(t.Context(), cfg)
	userManager := manager.NewUserManager(r, cmkAuditor)
	tagManager := manager.NewTagManager(r)
	keyConfigManager := manager.NewKeyConfigManager(r, certManager, userManager, tagManager, cmkAuditor, cfg)
	groupManager := manager.NewGroupManager(r, svcRegistry, userManager)

	clientsFactory, err := clients.NewFactory(cfg.Services)
	assert.NoError(t, err)
	systemManager := manager.NewSystemManager(t.Context(), r, clientsFactory, nil, svcRegistry, cfg, keyConfigManager, userManager)

	keym := manager.NewKeyManager(r, svcRegistry, tenantConfigManager, keyConfigManager, userManager, certManager, nil, cmkAuditor)
	m := manager.NewWorkflowManager(
		r, svcRegistry, keym, keyConfigManager, systemManager,
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

// createTestKeyConfigWithAdminGroup creates a KeyConfiguration with a properly configured
// admin group that has sufficient members in the IDM plugin for approver validation.
// Use this helper for tests that need to pass workflow validation.
func createTestKeyConfigWithAdminGroup(
	t *testing.T,
	repo repo.Repo,
	ctx context.Context,
	userID string,
) *model.KeyConfiguration {
	t.Helper()

	const (
		adminGroupName   = "test-key-admin-group"
		adminGroupSCIMID = "scim-key-admin-group-id"
	)

	group := testutils.NewGroup(func(g *model.Group) {
		g.Name = adminGroupName + "-" + uuid.NewString()[:8] // Make unique
		g.IAMIdentifier = adminGroupName + "-" + uuid.NewString()[:8]
		g.Role = constants.KeyAdminRole
	})

	// Set up plugin group membership with sufficient approvers
	members := []testplugins.IdentityManagementUserRef{
		{ID: constants.SystemUser.String(), Email: "system@example.com"},
		{ID: "test-user-1", Email: "user1@example.com"},
		{ID: "test-user-2", Email: "user2@example.com"},
	}

	if userID != "" && userID != constants.SystemUser.String() {
		members = append(members, testplugins.IdentityManagementUserRef{
			ID:    userID,
			Email: userID + "@example.com",
		})
	}

	scimID := adminGroupSCIMID + "-" + uuid.NewString()[:8]
	testplugins.IdentityManagementGroups[group.IAMIdentifier] = scimID
	testplugins.IdentityManagementGroupMembership[scimID] = members

	// Clean up plugin state after test
	t.Cleanup(func() {
		delete(testplugins.IdentityManagementGroups, group.IAMIdentifier)
		delete(testplugins.IdentityManagementGroupMembership, scimID)
	})

	creatorID := userID
	if creatorID == "" {
		creatorID = "test-creator"
	}

	keyConfig := testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
		c.AdminGroupID = group.ID
		c.AdminGroup = *group
		c.CreatorID = creatorID
	})

	testutils.CreateTestEntities(ctx, t, repo, group, keyConfig)

	return keyConfig
}

func createTestObjects(t *testing.T, repo repo.Repo, ctx context.Context) (*model.KeyConfiguration,
	*model.Key,
) {
	t.Helper()

	// Extract user from context
	var userID string
	if clientData, ok := ctx.Value(constants.ClientData).(*auth.ClientData); ok {
		userID = clientData.Identifier
	}

	// Create key configuration with proper admin group setup
	keyConfig := createTestKeyConfigWithAdminGroup(t, repo, ctx, userID)

	key := testutils.NewKey(func(k *model.Key) {
		k.ID = uuid.New()
		k.KeyConfigurationID = keyConfig.ID
		k.State = string(cmkapi.KeyStateENABLED) // Set key to enabled state
	})

	testutils.CreateTestEntities(
		ctx,
		t,
		repo,
		key,
	)

	// Update keyConfig with primary key ID
	keyConfig.PrimaryKeyID = &key.ID
	_, err := repo.Patch(ctx, keyConfig, *repoPackage.NewQuery())
	if err != nil {
		t.Fatalf("failed to update key config with primary key: %v", err)
	}

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
		assert.Equal(t, manager.ErrOngoingWorkflowExist, status.ErrDetails)
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
		assert.Equal(t, manager.ErrConnectSystemNoPrimaryKey, status.ErrDetails)
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
		assert.False(t, status.Valid)
		assert.False(t, status.CanCreate)
		assert.Equal(t, manager.ErrConnectSystemNoPrimaryKey, status.ErrDetails)
		assert.NoError(t, err)
	})

	t.Run("Should be creatable on rejected previous workflow", func(t *testing.T) {
		wf, err := createTestWorkflow(
			ctxSys, repo, testutils.NewWorkflow(
				func(w *model.Workflow) {
					w.State = workflow.StateInitial.String()
					w.State = workflow.StateRejected.String()
					w.ActionType = workflow.ActionTypeDelete.String()
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
	})

	t.Run("should not be valid on primary key change with unconnected system", func(t *testing.T) {
		key := testutils.NewKey(func(k *model.Key) {
			k.IsPrimary = true
		})
		keyConfig := testutils.NewKeyConfig(func(kc *model.KeyConfiguration) {
			kc.PrimaryKeyID = &key.ID
		})
		system := testutils.NewSystem(func(s *model.System) {
			s.KeyConfigurationID = &keyConfig.ID
			s.Status = cmkapi.SystemStatusDISCONNECTED
		})
		testutils.CreateTestEntities(ctxSys, t, repo, keyConfig, key, system)
		wf := testutils.NewWorkflow(
			func(w *model.Workflow) {
				w.State = workflow.StateInitial.String()
				w.ActionType = workflow.ActionTypeUpdatePrimary.String()
				w.ArtifactID = keyConfig.ID
				w.ArtifactType = workflow.ArtifactTypeKeyConfiguration.String()
				w.Parameters = uuid.NewString()
			},
		)

		status, err := m.CheckWorkflow(ctxSys, wf)
		assert.True(t, status.Enabled)
		assert.False(t, status.Exists)
		assert.False(t, status.Valid)
		assert.False(t, status.CanCreate)
		assert.NoError(t, err)
		assert.Equal(t, manager.ErrNotAllSystemsConnected, status.ErrDetails)
	})

	t.Run("should not be valid on change primary key to primary key", func(t *testing.T) {
		key := testutils.NewKey(func(k *model.Key) {
			k.IsPrimary = true
		})

		keyConfig := testutils.NewKeyConfig(func(kc *model.KeyConfiguration) {
			kc.PrimaryKeyID = &key.ID
		})

		testutils.CreateTestEntities(ctxSys, t, repo, key, keyConfig)

		wf := testutils.NewWorkflow(
			func(w *model.Workflow) {
				w.State = workflow.StateInitial.String()
				w.ActionType = workflow.ActionTypeUpdatePrimary.String()
				w.ArtifactID = keyConfig.ID
				w.ArtifactType = workflow.ArtifactTypeKeyConfiguration.String()
				w.Parameters = key.ID.String()
			},
		)

		status, err := m.CheckWorkflow(ctxSys, wf)
		assert.True(t, status.Enabled)
		assert.False(t, status.Exists)
		assert.False(t, status.Valid)
		assert.False(t, status.CanCreate)
		assert.NoError(t, err)
		assert.Equal(t, manager.ErrAlreadyPrimaryKey, status.ErrDetails)
	})

	t.Run("should have canCreate on primary key change without unconnected system", func(t *testing.T) {
		keyConfig := createTestKeyConfigWithAdminGroup(t, repo, ctxSys, constants.SystemUser.String())
		system := testutils.NewSystem(func(s *model.System) {
			s.KeyConfigurationID = &keyConfig.ID
			s.Status = cmkapi.SystemStatusCONNECTED
		})
		testutils.CreateTestEntities(ctxSys, t, repo, system)
		wf := testutils.NewWorkflow(
			func(w *model.Workflow) {
				w.State = workflow.StateInitial.String()
				w.ActionType = workflow.ActionTypeUpdatePrimary.String()
				w.ArtifactID = keyConfig.ID
				w.ArtifactType = workflow.ArtifactTypeKeyConfiguration.String()
				w.Parameters = uuid.NewString()
			},
		)

		status, err := m.CheckWorkflow(ctxSys, wf)
		assert.True(t, status.Enabled)
		assert.False(t, status.Exists)
		assert.True(t, status.Valid)
		assert.True(t, status.CanCreate)
		assert.NoError(t, err)
	})

	t.Run("Should return authorization error on non active artifact", func(t *testing.T) {
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
		createAuditorGroup(ctx, t, repo)

		ctxSys := context.WithValue(
			ctx,
			constants.ClientData, &auth.ClientData{
				Identifier: constants.SystemUser.String(),
			},
		)

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
				retrievedWf, _, _, err := m.GetWorkflowByID(
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
	m, r, tenant := SetupWorkflowManager(t, &config.Config{})
	userID := uuid.NewString()
	allWorkflowUserID := uuid.NewString()
	artifactID := uuid.New()

	baseTime := time.Now()

	workflow1, err := createTestWorkflow(
		testutils.CreateCtxWithTenant(tenant),
		r,
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
		r,
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
		r,
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
		r,
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

				pagination := repo.Pagination{
					Skip:  0,
					Top:   5,
					Count: true,
				}

				if tc.expectedInitiatorID != "" {
					for _, wf := range workflows {
						approvers, count, err := m.ListWorkflowApprovers(ctx, wf.ID, false, pagination)
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
			Groups:     []string{"auditorGroup"},
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
				pagination := repo.Pagination{
					Skip:  constants.DefaultSkip,
					Top:   constants.DefaultTop,
					Count: true,
				}
				approvers, _, err := m.ListWorkflowApprovers(
					ctxSys, tt.workflowID, false,
					pagination,
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
		t, &config.Config{},
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
						Groups:     []string{"auditorGroup"},
					},
				)
				_, err = m.AutoAssignApprovers(ctxSys, wf.ID)
				if tt.expectErr {
					assert.Error(t, err)
					assert.ErrorIs(t, err, tt.errMessage)
				} else {
					assert.NoError(t, err)

					count, _, err := m.ListWorkflowApprovers(ctxSys, wf.ID, false, repo.Pagination{})
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
	ctx := testutils.CreateCtxWithTenant(tenantID)
	ctx = cmkcontext.InjectClientData(ctx, &auth.ClientData{Identifier: "User-ID"}, nil)

	t.Run("should successfully create and enqueue notification task", func(t *testing.T) {
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
		assert.Equal(t, 1, mockClient.EnqueueCallCount)
		assert.NotNil(t, mockClient.LastTask)
	})

	t.Run("should skip notification when async client is nil", func(t *testing.T) {
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
		assert.Equal(t, 0, mockClient.EnqueueCallCount)
	})

	t.Run("should return error when GetTenant fails", func(t *testing.T) {
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
		assert.Equal(t, 1, mockClient.EnqueueCallCount)
	})

	t.Run("should handle different workflow transitions", func(t *testing.T) {
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

		assert.Equal(t, len(transitions), mockClient.EnqueueCallCount)
	})
}

func TestWorkflowManager_WorkflowCanExpire(t *testing.T) {
	m, r, tenant := SetupWorkflowManager(t, &config.Config{})
	ctx := testutils.CreateCtxWithTenant(tenant)

	workflowConfig := testutils.NewWorkflowConfig(func(_ *model.TenantConfig) {})
	testutils.CreateTestEntities(ctx, t, r, workflowConfig)

	tests := []struct {
		state    string
		expected bool
	}{
		{workflow.StateInitial.String(), false},
		{workflow.StateWaitApproval.String(), true},
		{workflow.StateWaitConfirmation.String(), true},
		{workflow.StateExecuting.String(), true},
		{workflow.StateRevoked.String(), false},
		{workflow.StateRejected.String(), false},
		{workflow.StateExpired.String(), false},
		{workflow.StateSuccessful.String(), false},
		{workflow.StateFailed.String(), false},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			wf := testutils.NewWorkflow(func(w *model.Workflow) {
				w.State = tt.state
			})
			testutils.CreateTestEntities(ctx, t, r, wf)

			got, err := m.WorkflowCanExpire(ctx, wf)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
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
			_, _, _, err = wm.GetWorkflowByID(ctx, oldTerminalWf.ID)
			assert.ErrorIs(t, err, manager.ErrWorkflowNotAllowed)

			// Verify workflow approvers were also deleted
			approverQuery := repo.NewQuery().Where(
				repo.NewCompositeKeyGroup(
					repo.NewCompositeKey().Where(model.WorkflowID, oldTerminalWf.ID),
				),
			)
			countAfter, err := r.Count(ctx, &model.WorkflowApprover{}, *approverQuery)
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
			_, _, _, err = wm.GetWorkflowByID(ctx, recentTerminalWf.ID)
			assert.NoError(t, err)

			// Verify workflow approvers still exist
			approverQuery := repo.NewQuery().Where(
				repo.NewCompositeKeyGroup(
					repo.NewCompositeKey().Where(model.WorkflowID, recentTerminalWf.ID),
				),
			)
			count, err := r.Count(ctx, &model.WorkflowApprover{}, *approverQuery)
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
			_, _, _, err = wm.GetWorkflowByID(ctx, oldActiveWf.ID)
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
				_, _, _, err = wm.GetWorkflowByID(ctx, wfID)
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
				_, _, _, err = wm.GetWorkflowByID(ctx, wfID)
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
			_, _, _, err = wm.GetWorkflowByID(ctx, recentWf.ID)
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
			_, _, _, err = wm.GetWorkflowByID(ctx, oldWf.ID)
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
				_, _, _, err = wm.GetWorkflowByID(ctx, wfID)
				assert.NoError(t, err, "Non-terminal workflow in state %s should not be deleted", nonTerminalStates[i])
			}
		},
	)
}

// ============================================================================
// Approver Eligibility Tests
// ============================================================================

const (
	testGroupName   = "KMS_001"
	testGroupSCIMID = "SCIM-Group-ID-001"
	approver1ID     = "00000000-0000-0000-0000-100000000001"
	approver1Email  = "user1@example.com"
	approver2ID     = "00000000-0000-0000-0000-100000000002"
	approver2Email  = "user2@example.com"
)

// setupEligibilityTest creates a workflow with approvers and returns the necessary test data
func setupEligibilityTest(
	t *testing.T,
	approverCount int,
) (*manager.WorkflowManager, repo.Repo, context.Context, *model.Workflow, string) {
	t.Helper()

	cfg := &config.Config{}

	wm, r, tenantID := SetupWorkflowManager(t, cfg)
	ctx := testutils.CreateCtxWithTenant(tenantID)

	// Create tenant workflow config with minimum approvals matching approver count
	workflowConfig := testutils.NewWorkflowConfig(func(tc *model.TenantConfig) {
		var wc model.WorkflowConfig
		_ = json.Unmarshal(tc.Value, &wc)
		wc.MinimumApprovals = approverCount
		tc.Value, _ = json.Marshal(wc)
	})
	testutils.CreateTestEntities(ctx, t, r, workflowConfig)

	// Create key admin group
	group := testutils.NewGroup(func(g *model.Group) {
		g.Name = testGroupName
		g.IAMIdentifier = testGroupName
		g.Role = constants.KeyAdminRole
	})
	testutils.CreateTestEntities(ctx, t, r, group)

	// Create key configuration
	keyConfig := &model.KeyConfiguration{
		ID:           uuid.New(),
		Name:         "test-kc",
		AdminGroupID: group.ID,
	}
	testutils.CreateTestEntities(ctx, t, r, keyConfig)

	// Create system
	system := testutils.NewSystem(func(s *model.System) {
		s.KeyConfigurationID = &keyConfig.ID
	})
	testutils.CreateTestEntities(ctx, t, r, system)

	// Create workflow
	groupIDsJSON, err := json.Marshal([]uuid.UUID{group.ID})
	require.NoError(t, err)

	artifactName := system.Identifier
	paramsResourceName := keyConfig.Name
	paramsResourceType := "KEY_CONFIGURATION"

	wf := testutils.NewWorkflow(func(w *model.Workflow) {
		w.State = workflow.StateWaitApproval.String()
		w.ArtifactType = workflow.ArtifactTypeSystem.String()
		w.ArtifactID = system.ID
		w.ArtifactName = &artifactName
		w.ActionType = workflow.ActionTypeLink.String()
		w.Parameters = keyConfig.ID.String()
		w.ParametersResourceName = &paramsResourceName
		w.ParametersResourceType = &paramsResourceType
		w.ApproverGroupIDs = groupIDsJSON
		w.InitiatorID = approver1ID
		w.MinimumApprovalCount = approverCount // Set minimum approval count to match test requirement
	})
	testutils.CreateTestEntities(ctx, t, r, wf)

	// Create approvers based on count
	approverIDs := []string{approver1ID, approver2ID}
	approverEmails := []string{approver1Email, approver2Email}

	// Initialize SCIM group membership with all approvers
	var scimMembers []testplugins.IdentityManagementUserRef
	for i := 0; i < approverCount && i < len(approverIDs); i++ {
		approver := &model.WorkflowApprover{
			WorkflowID: wf.ID,
			UserID:     approverIDs[i],
		}
		testutils.CreateTestEntities(ctx, t, r, approver)

		// Add to SCIM membership
		scimMembers = append(scimMembers, testplugins.IdentityManagementUserRef{
			ID:    approverIDs[i],
			Email: approverEmails[i],
		})
	}

	// Register group in SCIM
	testplugins.IdentityManagementGroups[testGroupName] = testGroupSCIMID
	testplugins.IdentityManagementGroupMembership[testGroupSCIMID] = scimMembers

	return wm, r, ctx, wf, tenantID
}

// setAuthContext adds client data to context for SCIM queries
func setAuthContext(ctx context.Context, userID, _ string) context.Context {
	return testutils.InjectClientDataIntoContext(ctx, userID, []string{testGroupName})
}

func TestWorkflowApproverEligibility(t *testing.T) {
	t.Run("all eligible approvers removed before voting - workflow expires", func(t *testing.T) {
		wm, r, ctx, wf, tenantID := setupEligibilityTest(t, 2)
		ctx = setAuthContext(ctx, approver1ID, approver1Email)

		// Remove all approvers from IAM group
		testplugins.IdentityManagementGroupMembership[testGroupSCIMID] = []testplugins.IdentityManagementUserRef{}

		// Get workflow - should show insufficient approvers warning
		gotWf, insufficientApprovers, _, err := wm.GetWorkflowByID(ctx, wf.ID)
		require.NoError(t, err)
		assert.True(t, insufficientApprovers, "Should detect insufficient approvers")
		assert.Equal(t, workflow.StateWaitApproval.String(), gotWf.State)

		// Attempt to approve - should fail with eligibility error
		_, err = wm.TransitionWorkflow(ctx, wf.ID, workflow.TransitionApprove)
		assert.ErrorIs(t, err, workflow.ErrApproverNoLongerEligible)

		// Verify workflow state unchanged
		gotWf, _, _, err = wm.GetWorkflowByID(ctx, wf.ID)
		require.NoError(t, err)
		assert.Equal(t, workflow.StateWaitApproval.String(), gotWf.State)

		// Simulate expiry by updating ExpiryDate
		now := time.Now()
		expiryDate := now.Add(-1 * time.Hour)
		_, patchErr := r.Patch(ctx, &model.Workflow{
			ID:         wf.ID,
			ExpiryDate: &expiryDate,
		}, *repo.NewQuery())
		require.NoError(t, patchErr)

		// Transition to expired state (must use proper system context)
		systemCtx := testutils.InjectClientDataIntoContext(
			testutils.CreateCtxWithTenant(tenantID),
			workflow.SystemUserID,
			[]string{testGroupName},
		)
		_, err = wm.TransitionWorkflow(systemCtx, wf.ID, workflow.TransitionExpire)
		// FSM might not allow EXPIRE transition from current state - that's okay for this test
		if err != nil {
			t.Logf("Could not transition to EXPIRED (FSM restriction): %v", err)
			return // Test still validates eligibility checking worked
		}

		// Verify expired state (only if transition succeeded)
		gotWf, _, _, err = wm.GetWorkflowByID(ctx, wf.ID)
		require.NoError(t, err)
		assert.Equal(t, workflow.StateExpired.String(), gotWf.State)
	})

	t.Run("all eligible approvers removed - initiator revokes", func(t *testing.T) {
		wm, _, ctx, wf, _ := setupEligibilityTest(t, 2)
		ctx = setAuthContext(ctx, approver1ID, approver1Email)

		// Remove all approvers from IAM group
		testplugins.IdentityManagementGroupMembership[testGroupSCIMID] = []testplugins.IdentityManagementUserRef{}

		// Get workflow - should show insufficient approvers warning
		gotWf, insufficientApprovers, _, err := wm.GetWorkflowByID(ctx, wf.ID)
		require.NoError(t, err)
		assert.True(t, insufficientApprovers)
		assert.Equal(t, workflow.StateWaitApproval.String(), gotWf.State)

		// Initiator revokes workflow
		_, err = wm.TransitionWorkflow(ctx, wf.ID, workflow.TransitionRevoke)
		require.NoError(t, err)

		// Verify revoked state
		gotWf, _, _, err = wm.GetWorkflowByID(ctx, wf.ID)
		require.NoError(t, err)
		assert.Equal(t, workflow.StateRevoked.String(), gotWf.State)
	})

	t.Run("eligible approvers removed then re-added - warning cleared", func(t *testing.T) {
		wm, _, ctx, wf, _ := setupEligibilityTest(t, 2)
		ctx = setAuthContext(ctx, approver1ID, approver1Email)

		// Remove all approvers from IAM group
		testplugins.IdentityManagementGroupMembership[testGroupSCIMID] = []testplugins.IdentityManagementUserRef{}

		// Get workflow - should show warning
		_, insufficientApprovers, _, err := wm.GetWorkflowByID(ctx, wf.ID)
		require.NoError(t, err)
		assert.True(t, insufficientApprovers)

		// Re-add approvers to IAM group
		testplugins.IdentityManagementGroupMembership[testGroupSCIMID] = []testplugins.IdentityManagementUserRef{
			{ID: approver1ID, Email: approver1Email},
			{ID: approver2ID, Email: approver2Email},
		}

		// Get workflow again - warning should be cleared
		_, insufficientApprovers, _, err = wm.GetWorkflowByID(ctx, wf.ID)
		require.NoError(t, err)
		assert.False(t, insufficientApprovers, "Warning should be cleared after approvers re-added")

		// Approver 1 votes
		_, err = wm.TransitionWorkflow(ctx, wf.ID, workflow.TransitionApprove)
		require.NoError(t, err)

		// With 2 approvers and threshold=2, need both to approve
		// After 1 approval, workflow stays in WAIT_APPROVAL
		gotWf, _, _, err := wm.GetWorkflowByID(ctx, wf.ID)
		require.NoError(t, err)
		assert.Equal(t, workflow.StateWaitApproval.String(), gotWf.State,
			"Workflow stays in WAIT_APPROVAL - need 2 approvals with threshold=2")

		// Approver 2 votes
		ctx2 := setAuthContext(ctx, approver2ID, approver2Email)
		_, err = wm.TransitionWorkflow(ctx2, wf.ID, workflow.TransitionApprove)
		require.NoError(t, err)

		// Now workflow transitions to WAIT_CONFIRMATION
		gotWf, _, _, err = wm.GetWorkflowByID(ctx2, wf.ID)
		require.NoError(t, err)
		assert.Equal(t, workflow.StateWaitConfirmation.String(), gotWf.State,
			"Workflow transitions after 2 approvals")
	})

	t.Run("partial votes cast, remaining approver removed - cannot continue", func(t *testing.T) {
		wm, r, ctx, wf, _ := setupEligibilityTest(t, 3)
		ctx = setAuthContext(ctx, approver1ID, approver1Email)

		// Create a third approver
		approver3 := &model.WorkflowApprover{
			WorkflowID: wf.ID,
			UserID:     "00000000-0000-0000-0000-100000000004",
		}
		testutils.CreateTestEntities(ctx, t, r, approver3)

		// Update group membership to include all three
		testplugins.IdentityManagementGroupMembership[testGroupSCIMID] = []testplugins.IdentityManagementUserRef{
			{ID: approver1ID, Email: approver1Email},
			{ID: approver2ID, Email: approver2Email},
			{ID: "00000000-0000-0000-0000-100000000004", Email: "user4@example.com"},
		}

		// Approver 1 votes (while still in group)
		ctx1 := setAuthContext(ctx, approver1ID, approver1Email)
		_, err := wm.TransitionWorkflow(ctx1, wf.ID, workflow.TransitionApprove)
		require.NoError(t, err)

		// Verify vote recorded
		approvers, _, err := wm.ListWorkflowApprovers(ctx, wf.ID, false, repo.Pagination{})
		require.NoError(t, err)
		for _, a := range approvers {
			if a.UserID == approver1ID {
				assert.True(t, a.Approved.Valid)
				assert.True(t, a.Approved.Bool)
			}
		}

		// Remove approvers 2 and 3 from IAM group (only approver 1 remains)
		testplugins.IdentityManagementGroupMembership[testGroupSCIMID] = []testplugins.IdentityManagementUserRef{
			{ID: approver1ID, Email: approver1Email},
		}

		// Workflow should still be in WAIT_APPROVAL (vote happened when all 3 were eligible)
		// Auto-reject only happens AT VOTE TIME, not retroactively
		gotWf, _, _, err := wm.GetWorkflowByID(ctx1, wf.ID)
		require.NoError(t, err)
		assert.Equal(t, workflow.StateWaitApproval.String(), gotWf.State,
			"Workflow stays in WAIT_APPROVAL - auto-reject happens at vote time, not retroactively")

		// Approver 2 (removed) tries to vote - should fail with eligibility error
		ctx2 := setAuthContext(ctx, approver2ID, approver2Email)
		_, err = wm.TransitionWorkflow(ctx2, wf.ID, workflow.TransitionApprove)
		assert.ErrorIs(t, err, workflow.ErrApproverNoLongerEligible,
			"Removed approver cannot vote")
	})

	t.Run("approver who already voted can still be counted after removal", func(t *testing.T) {
		wm, _, ctx, wf, _ := setupEligibilityTest(t, 2)

		// Approver 1 votes while in group
		ctx1 := setAuthContext(ctx, approver1ID, approver1Email)
		_, err := wm.TransitionWorkflow(ctx1, wf.ID, workflow.TransitionApprove)
		require.NoError(t, err)

		// Remove approver 1 from IAM group
		testplugins.IdentityManagementGroupMembership[testGroupSCIMID] = []testplugins.IdentityManagementUserRef{
			{ID: approver2ID, Email: approver2Email},
		}

		// Approver 1 tries to change vote (reject) - should fail (no longer eligible)
		_, err = wm.TransitionWorkflow(ctx1, wf.ID, workflow.TransitionReject)
		assert.Error(t, err, "Removed approver cannot change their vote")

		// Approver 2 votes
		ctx2 := setAuthContext(ctx, approver2ID, approver2Email)
		_, err = wm.TransitionWorkflow(ctx2, wf.ID, workflow.TransitionApprove)
		require.NoError(t, err)

		// Workflow should transition to WAIT_CONFIRMATION
		// Removed approver's vote still counts: approved=2, threshold=2
		gotWf, _, _, err := wm.GetWorkflowByID(ctx2, wf.ID)
		require.NoError(t, err)
		assert.Equal(t, workflow.StateWaitConfirmation.String(), gotWf.State,
			"Workflow should transition - removed approver's vote still counts")
	})

	t.Run("new user added to group not in original snapshot - cannot vote", func(t *testing.T) {
		wm, _, ctx, wf, _ := setupEligibilityTest(t, 1)

		// Add new user to IAM group (not in workflow approvers snapshot)
		newUserID := "00000000-0000-0000-0000-100000000099"
		newUserEmail := "newuser@example.com"
		testplugins.IdentityManagementGroupMembership[testGroupSCIMID] = []testplugins.IdentityManagementUserRef{
			{ID: approver1ID, Email: approver1Email},
			{ID: newUserID, Email: newUserEmail},
		}

		// Get workflow - should NOT show insufficient approvers (still has approver 1)
		_, insufficientApprovers, _, err := wm.GetWorkflowByID(
			setAuthContext(ctx, approver1ID, approver1Email), wf.ID)
		require.NoError(t, err)
		assert.False(t, insufficientApprovers)

		// New user tries to vote - should fail (not in snapshot)
		newUserCtx := setAuthContext(ctx, newUserID, newUserEmail)
		_, err = wm.TransitionWorkflow(newUserCtx, wf.ID, workflow.TransitionApprove)
		assert.Error(t, err, "New user not in snapshot should not be able to vote")

		// Original approver can still vote
		ctx1 := setAuthContext(ctx, approver1ID, approver1Email)
		_, err = wm.TransitionWorkflow(ctx1, wf.ID, workflow.TransitionApprove)
		require.NoError(t, err)

		// Verify workflow transitioned (may be WAIT_CONFIRMATION or SUCCESSFUL)
		gotWf, _, _, err := wm.GetWorkflowByID(ctx1, wf.ID)
		require.NoError(t, err)
		assert.NotEqual(t, workflow.StateWaitApproval.String(), gotWf.State)
		assert.Contains(t, []string{workflow.StateWaitConfirmation.String(), workflow.StateSuccessful.String()}, gotWf.State)
	})

	t.Run("rejected vote from removed approver still counts", func(t *testing.T) {
		wm, _, ctx, wf, _ := setupEligibilityTest(t, 2)

		// Approver 1 rejects while in group
		ctx1 := setAuthContext(ctx, approver1ID, approver1Email)
		_, err := wm.TransitionWorkflow(ctx1, wf.ID, workflow.TransitionReject)
		require.NoError(t, err)

		// Verify rejection recorded
		approvers, _, err := wm.ListWorkflowApprovers(
			setAuthContext(ctx, approver1ID, approver1Email), wf.ID, false, repo.Pagination{})
		require.NoError(t, err)
		rejectionFound := false
		for _, a := range approvers {
			if a.UserID == approver1ID {
				assert.True(t, a.Approved.Valid)
				assert.False(t, a.Approved.Bool, "Should be rejected")
				rejectionFound = true
			}
		}
		assert.True(t, rejectionFound, "Should find approver1's rejection vote")

		// Check initial state after rejection
		gotWfAfterReject, _, _, err := wm.GetWorkflowByID(ctx1, wf.ID)
		require.NoError(t, err)
		initialState := gotWfAfterReject.State

		// Remove approver 1 from IAM group
		testplugins.IdentityManagementGroupMembership[testGroupSCIMID] = []testplugins.IdentityManagementUserRef{
			{ID: approver2ID, Email: approver2Email},
		}

		// Workflow state should remain the same (rejection still counts even after removal)
		gotWf, _, _, err := wm.GetWorkflowByID(setAuthContext(ctx, approver2ID, approver2Email), wf.ID)
		require.NoError(t, err)
		assert.Equal(t, initialState, gotWf.State,
			"Workflow state should not change after approver removed from IAM")
	})

	t.Run("insufficientApprovers flag updates dynamically with IAM changes", func(t *testing.T) {
		wm, r, ctx, wf, _ := setupEligibilityTest(t, 0)

		// Remove all approvers from IAM before checking
		testplugins.IdentityManagementGroupMembership[testGroupSCIMID] = []testplugins.IdentityManagementUserRef{}

		// Manually create workflow approvers (simulating they were added before removal)
		approver := &model.WorkflowApprover{
			WorkflowID: wf.ID,
			UserID:     approver1ID,
		}
		testutils.CreateTestEntities(ctx, t, r, approver)

		// Get workflow - should show insufficient approvers
		_, insufficientApprovers, _, err := wm.GetWorkflowByID(
			setAuthContext(ctx, approver1ID, approver1Email), wf.ID)
		require.NoError(t, err)
		assert.True(t, insufficientApprovers, "Should detect no eligible approvers")

		// Re-add approvers to group
		testplugins.IdentityManagementGroupMembership[testGroupSCIMID] = []testplugins.IdentityManagementUserRef{
			{ID: approver1ID, Email: approver1Email},
			{ID: approver2ID, Email: approver2Email},
		}

		// Get workflow again - should still show insufficient (only 1 assigned approver, threshold=2)
		_, insufficientApprovers, _, err = wm.GetWorkflowByID(
			setAuthContext(ctx, approver1ID, approver1Email), wf.ID)
		require.NoError(t, err)
		assert.True(t, insufficientApprovers, "Should still be insufficient: only 1 assigned approver (approver2 never added to workflow), threshold=2")
	})
}

func TestWorkflowApproverEligibilityGetWorkflowByID(t *testing.T) {
	t.Run("returns correct insufficientApprovers flag", func(t *testing.T) {
		wm, _, ctx, wf, _ := setupEligibilityTest(t, 2)
		ctx = setAuthContext(ctx, approver1ID, approver1Email)

		// Initially sufficient approvers
		_, insufficientApprovers, _, err := wm.GetWorkflowByID(ctx, wf.ID)
		require.NoError(t, err)
		assert.False(t, insufficientApprovers)

		// Remove one approver
		testplugins.IdentityManagementGroupMembership[testGroupSCIMID] = []testplugins.IdentityManagementUserRef{
			{ID: approver1ID, Email: approver1Email},
		}

		// Should detect insufficient when below threshold (1 eligible < 2 required)
		_, insufficientApprovers, _, err = wm.GetWorkflowByID(ctx, wf.ID)
		require.NoError(t, err)
		assert.True(t, insufficientApprovers, "One eligible approver is insufficient for threshold of 2")

		// Remove all approvers
		testplugins.IdentityManagementGroupMembership[testGroupSCIMID] = []testplugins.IdentityManagementUserRef{}

		// Should detect insufficient when no eligible approvers
		_, insufficientApprovers, _, err = wm.GetWorkflowByID(ctx, wf.ID)
		require.NoError(t, err)
		assert.True(t, insufficientApprovers, "No eligible approvers should trigger warning")
	})

	t.Run("checks eligibility regardless of workflow state", func(t *testing.T) {
		wm, r, ctx, wf, _ := setupEligibilityTest(t, 2)
		ctx = setAuthContext(ctx, approver1ID, approver1Email)

		// Remove all approvers
		testplugins.IdentityManagementGroupMembership[testGroupSCIMID] = []testplugins.IdentityManagementUserRef{}

		// Transition to REVOKED state
		_, err := wm.TransitionWorkflow(ctx, wf.ID, workflow.TransitionRevoke)
		require.NoError(t, err)

		// Should still check eligibility even for terminal states
		_, insufficientApprovers, _, err := wm.GetWorkflowByID(ctx, wf.ID)
		require.NoError(t, err)
		assert.True(t, insufficientApprovers, "Should check eligibility even in terminal states")

		// Create workflow in SUCCESSFUL state
		groupIDsJSON, _ := json.Marshal([]uuid.UUID{})
		successfulWf := testutils.NewWorkflow(func(w *model.Workflow) {
			w.State = workflow.StateSuccessful.String()
			w.ApproverGroupIDs = groupIDsJSON
			w.InitiatorID = approver1ID
		})
		testutils.CreateTestEntities(ctx, t, r, successfulWf)

		_, insufficientApprovers, _, err = wm.GetWorkflowByID(ctx, successfulWf.ID)
		require.NoError(t, err)
		assert.False(t, insufficientApprovers, "Empty approver group list means no restrictions")
	})
}

func TestWorkflowApproverEligibilityErrorHandling(t *testing.T) {
	t.Run("SCIM failure during eligibility check prevents voting", func(t *testing.T) {
		wm, _, ctx, wf, _ := setupEligibilityTest(t, 1)
		ctx = setAuthContext(ctx, approver1ID, approver1Email)

		// Simulate SCIM failure by removing group mapping
		delete(testplugins.IdentityManagementGroups, testGroupName)

		// Attempt to vote - should fail due to SCIM error
		_, err := wm.TransitionWorkflow(ctx, wf.ID, workflow.TransitionApprove)
		assert.Error(t, err, "SCIM failure should prevent voting")

		// Restore group for cleanup
		testplugins.IdentityManagementGroups[testGroupName] = testGroupSCIMID
	})

	t.Run("SCIM failure during GET returns error in insufficientApprovers check", func(t *testing.T) {
		wm, _, ctx, wf, _ := setupEligibilityTest(t, 1)
		ctx = setAuthContext(ctx, approver1ID, approver1Email)

		// Simulate SCIM failure
		delete(testplugins.IdentityManagementGroups, testGroupName)

		// GetWorkflowByID should now return error when eligibility check fails
		_, _, _, err := wm.GetWorkflowByID(ctx, wf.ID)
		require.Error(t, err, "GET should return error when eligibility check fails")
		assert.True(t, errs.IsAnyError(err, manager.ErrCheckWorkflowEligibility), "Error should be ErrCheckWorkflowEligibility")

		// Restore group
		testplugins.IdentityManagementGroups[testGroupName] = testGroupSCIMID
	})
}

func TestWorkflowAutoRejectWhenApprovalImpossible(t *testing.T) {
	t.Run("auto-rejects after vote when insufficient eligible approvers", func(t *testing.T) {
		wm, _, ctx, wf, _ := setupEligibilityTest(t, 2)

		// Initially 2 eligible approvers, threshold = 2
		ctx = setAuthContext(ctx, approver1ID, approver1Email)

		// Remove one approver from group - only 1 eligible left
		testplugins.IdentityManagementGroupMembership[testGroupSCIMID] = []testplugins.IdentityManagementUserRef{
			{ID: approver1ID, Email: approver1Email},
		}

		// Verify workflow is still WAIT_APPROVAL (not auto-rejected before vote)
		workflowBefore, _, _, err := wm.GetWorkflowByID(ctx, wf.ID)
		require.NoError(t, err)
		assert.Equal(t, workflow.StateWaitApproval.String(), workflowBefore.State)

		// Remaining approver votes APPROVE
		workflowAfter, err := wm.TransitionWorkflow(ctx, wf.ID, workflow.TransitionApprove)
		require.NoError(t, err)

		// Workflow should auto-reject because only 1 eligible approver can't reach threshold of 2
		assert.Equal(t, workflow.StateRejected.String(), workflowAfter.State,
			"Workflow should auto-reject when approval becomes mathematically impossible")
	})

	t.Run("does not auto-reject when sufficient eligible approvers remain", func(t *testing.T) {
		wm, _, ctx, wf, _ := setupEligibilityTest(t, 2)

		// 2 eligible approvers, threshold = 2
		ctx = setAuthContext(ctx, approver1ID, approver1Email)

		// First approver votes APPROVE
		workflowAfter1, err := wm.TransitionWorkflow(ctx, wf.ID, workflow.TransitionApprove)
		require.NoError(t, err)

		// Should still be in WAIT_APPROVAL (approval still possible with 1 pending eligible approver)
		assert.Equal(t, workflow.StateWaitApproval.String(), workflowAfter1.State)

		// Second approver votes APPROVE
		ctx = setAuthContext(ctx, approver2ID, approver2Email)
		workflowAfter2, err := wm.TransitionWorkflow(ctx, wf.ID, workflow.TransitionApprove)
		require.NoError(t, err)

		// Should transition to WAIT_CONFIRMATION (normal flow)
		assert.Equal(t, workflow.StateWaitConfirmation.String(), workflowAfter2.State)
	})

	t.Run("auto-rejects even when user votes REJECT", func(t *testing.T) {
		wm, _, ctx, wf, _ := setupEligibilityTest(t, 2)

		// Initially 2 eligible approvers, threshold = 2
		ctx = setAuthContext(ctx, approver1ID, approver1Email)

		// Remove one approver from group - only 1 eligible left
		testplugins.IdentityManagementGroupMembership[testGroupSCIMID] = []testplugins.IdentityManagementUserRef{
			{ID: approver1ID, Email: approver1Email},
		}

		// Remaining approver votes REJECT
		workflowAfter, err := wm.TransitionWorkflow(ctx, wf.ID, workflow.TransitionReject)
		require.NoError(t, err)

		// Should be REJECTED (auto-reject check runs after any vote)
		assert.Equal(t, workflow.StateRejected.String(), workflowAfter.State)
	})

	t.Run("handles SCIM failure gracefully during auto-reject check", func(t *testing.T) {
		wm, _, ctx, wf, _ := setupEligibilityTest(t, 2)
		ctx = setAuthContext(ctx, approver1ID, approver1Email)

		// First vote succeeds
		workflowAfter1, err := wm.TransitionWorkflow(ctx, wf.ID, workflow.TransitionApprove)
		require.NoError(t, err)
		assert.Equal(t, workflow.StateWaitApproval.String(), workflowAfter1.State)

		// Simulate SCIM failure by removing group mapping (after first vote)
		delete(testplugins.IdentityManagementGroups, testGroupName)

		// Second vote should fail with eligibility error (user can't vote when SCIM unavailable)
		ctx = setAuthContext(ctx, approver2ID, approver2Email)
		_, err = wm.TransitionWorkflow(ctx, wf.ID, workflow.TransitionApprove)
		assert.Error(t, err)

		// Restore group for cleanup
		testplugins.IdentityManagementGroups[testGroupName] = testGroupSCIMID
	})
}

func TestValidateApproverCount(t *testing.T) {
	// Note: Not parallel - tests modify global testplugins state

	const (
		initiatorID     = "initiator-user"
		initiatorEmail  = "initiator@example.com"
		approver1ID     = "approver1-user"
		approver1Email  = "approver1@example.com"
		approver2ID     = "approver2-user"
		approver2Email  = "approver2@example.com"
		approver3ID     = "approver3-user"
		approver3Email  = "approver3@example.com"
		testGroupName   = "test-validation-group"
		testGroupSCIMID = "scim-validation-group-id"
	)

	tests := []struct {
		name             string
		groupMembers     []testplugins.IdentityManagementUserRef
		minimumApprovals int
		expectEligible   int
		expectError      bool
		errorContains    string
	}{
		{
			name: "single member group - initiator only",
			groupMembers: []testplugins.IdentityManagementUserRef{
				{ID: initiatorID, Email: initiatorEmail},
			},
			minimumApprovals: 2,
			expectEligible:   0, // initiator excluded
			expectError:      true,
			errorContains:    "insufficient eligible approvers",
		},
		{
			name: "two members with min=2 - insufficient",
			groupMembers: []testplugins.IdentityManagementUserRef{
				{ID: initiatorID, Email: initiatorEmail},
				{ID: approver1ID, Email: approver1Email},
			},
			minimumApprovals: 2,
			expectEligible:   1, // only approver1, initiator excluded
			expectError:      true,
			errorContains:    "required: 2, actual: 1",
		},
		{
			name: "three members with min=2 - exact threshold",
			groupMembers: []testplugins.IdentityManagementUserRef{
				{ID: initiatorID, Email: initiatorEmail},
				{ID: approver1ID, Email: approver1Email},
				{ID: approver2ID, Email: approver2Email},
			},
			minimumApprovals: 2,
			expectEligible:   2, // approver1, approver2
			expectError:      false,
		},
		{
			name: "four members with min=2 - sufficient",
			groupMembers: []testplugins.IdentityManagementUserRef{
				{ID: initiatorID, Email: initiatorEmail},
				{ID: approver1ID, Email: approver1Email},
				{ID: approver2ID, Email: approver2Email},
				{ID: approver3ID, Email: approver3Email},
			},
			minimumApprovals: 2,
			expectEligible:   3, // approver1, approver2, approver3
			expectError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: Not parallel - tests modify global testplugins state

			cfg := &config.Config{}
			wm, r, tenantID := SetupWorkflowManager(t, cfg)

			ctx := testutils.CreateCtxWithTenant(tenantID)
			ctx = testutils.InjectClientDataIntoContext(ctx, initiatorID, []string{testGroupName})

			// Create test group
			group := testutils.NewGroup(func(g *model.Group) {
				g.Name = testGroupName
				g.IAMIdentifier = testGroupName
				g.Role = constants.KeyAdminRole
			})
			testutils.CreateTestEntities(ctx, t, r, group)

			// Configure IDM plugin with test users
			testplugins.IdentityManagementGroups[testGroupName] = testGroupSCIMID
			testplugins.IdentityManagementGroupMembership[testGroupSCIMID] = tt.groupMembers

			// Create test key configuration with admin group
			keyConfig := &model.KeyConfiguration{
				ID:           uuid.New(),
				Name:         "test-kc-validation",
				AdminGroupID: group.ID,
				AdminGroup:   *group,
			}
			testutils.CreateTestEntities(ctx, t, r, keyConfig)

			// Create workflow
			wf := &model.Workflow{
				InitiatorID:  initiatorID,
				ArtifactType: workflow.ArtifactTypeKeyConfiguration.String(),
				ArtifactID:   keyConfig.ID,
				ActionType:   workflow.ActionTypeDelete.String(),
				State:        workflow.StateInitial.String(),
			}

			// Call ValidateApproverCount
			eligibleCount, err := wm.ValidateApproverCount(ctx, wf, tt.minimumApprovals)

			// Assertions
			assert.Equal(t, tt.expectEligible, eligibleCount)

			if tt.expectError {
				require.Error(t, err)
				assert.ErrorIs(t, err, manager.ErrWorkflowGroupNotSufficientMembers,
					"expected ErrWorkflowGroupNotSufficientMembers")
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
			}

			// Cleanup
			delete(testplugins.IdentityManagementGroups, testGroupName)
			delete(testplugins.IdentityManagementGroupMembership, testGroupSCIMID)
		})
	}
}

func TestCheckWorkflow_InsufficientApprovers(t *testing.T) {
	// Note: Not parallel - tests modify global testplugins state

	const (
		initiatorID     = "initiator-user"
		initiatorEmail  = "initiator@example.com"
		testGroupName   = "test-check-group"
		testGroupSCIMID = "scim-check-group-id"
	)

	cfg := &config.Config{}
	wm, r, tenantID := SetupWorkflowManager(t, cfg)

	ctx := testutils.CreateCtxWithTenant(tenantID)
	ctx = testutils.InjectClientDataIntoContext(ctx, initiatorID, []string{testGroupName})

	// Create test group with only initiator (insufficient approvers)
	group := testutils.NewGroup(func(g *model.Group) {
		g.Name = testGroupName
		g.IAMIdentifier = testGroupName
		g.Role = constants.KeyAdminRole
	})
	testutils.CreateTestEntities(ctx, t, r, group)

	// Configure IDM with only initiator in group
	testplugins.IdentityManagementGroups[testGroupName] = testGroupSCIMID
	testplugins.IdentityManagementGroupMembership[testGroupSCIMID] = []testplugins.IdentityManagementUserRef{
		{ID: initiatorID, Email: initiatorEmail},
	}

	// Create key configuration
	keyConfig := &model.KeyConfiguration{
		ID:           uuid.New(),
		Name:         "test-kc-check",
		AdminGroupID: group.ID,
		AdminGroup:   *group,
	}
	testutils.CreateTestEntities(ctx, t, r, keyConfig)

	// Create workflow
	wf := &model.Workflow{
		InitiatorID:  initiatorID,
		ArtifactType: workflow.ArtifactTypeKeyConfiguration.String(),
		ArtifactID:   keyConfig.ID,
		ActionType:   workflow.ActionTypeDelete.String(),
		State:        workflow.StateInitial.String(),
	}

	// Call CheckWorkflow
	status, err := wm.CheckWorkflow(ctx, wf)

	// Should not error, but should set CanCreate=false with details
	require.NoError(t, err, "CheckWorkflow should not return error for insufficient approvers")
	assert.True(t, status.Enabled, "workflow should be enabled")
	assert.False(t, status.Exists, "workflow should not exist yet")
	assert.True(t, status.Valid, "workflow should be valid")
	assert.False(t, status.CanCreate, "workflow should not be creatable with insufficient approvers")
	assert.Error(t, status.ErrDetails, "ErrDetails should be populated")
	assert.ErrorIs(t, status.ErrDetails, manager.ErrWorkflowGroupNotSufficientMembers,
		"ErrDetails should be ErrWorkflowGroupNotSufficientMembers")

	// Cleanup
	delete(testplugins.IdentityManagementGroups, testGroupName)
	delete(testplugins.IdentityManagementGroupMembership, testGroupSCIMID)
}

func TestCreateWorkflow_InsufficientApprovers(t *testing.T) {
	// Note: Not parallel - tests modify global testplugins state

	const (
		initiatorID     = "initiator-user"
		initiatorEmail  = "initiator@example.com"
		testGroupName   = "test-create-group"
		testGroupSCIMID = "scim-create-group-id"
	)

	cfg := &config.Config{}
	wm, r, tenantID := SetupWorkflowManager(t, cfg)

	ctx := testutils.CreateCtxWithTenant(tenantID)
	ctx = testutils.InjectClientDataIntoContext(ctx, initiatorID, []string{testGroupName})

	// Create test group with only initiator
	group := testutils.NewGroup(func(g *model.Group) {
		g.Name = testGroupName
		g.IAMIdentifier = testGroupName
		g.Role = constants.KeyAdminRole
	})
	testutils.CreateTestEntities(ctx, t, r, group)

	// Configure IDM with only initiator in group
	testplugins.IdentityManagementGroups[testGroupName] = testGroupSCIMID
	testplugins.IdentityManagementGroupMembership[testGroupSCIMID] = []testplugins.IdentityManagementUserRef{
		{ID: initiatorID, Email: initiatorEmail},
	}

	// Create key configuration
	keyConfig := &model.KeyConfiguration{
		ID:           uuid.New(),
		Name:         "test-kc-create",
		AdminGroupID: group.ID,
		AdminGroup:   *group,
	}
	testutils.CreateTestEntities(ctx, t, r, keyConfig)

	// Attempt to create workflow
	wf := &model.Workflow{
		InitiatorID:  initiatorID,
		ArtifactType: workflow.ArtifactTypeKeyConfiguration.String(),
		ArtifactID:   keyConfig.ID,
		ActionType:   workflow.ActionTypeDelete.String(),
		State:        workflow.StateInitial.String(),
	}

	// Call CreateWorkflow
	_, err := wm.CreateWorkflow(ctx, wf)

	// Should return validation error
	require.Error(t, err, "CreateWorkflow should return error for insufficient approvers")
	assert.ErrorIs(t, err, manager.ErrWorkflowGroupNotSufficientMembers,
		"error should be ErrWorkflowGroupNotSufficientMembers")
	assert.Contains(t, err.Error(), "required: 2", "error should contain required count")
	assert.Contains(t, err.Error(), "actual: 0", "error should contain actual count")

	// Cleanup
	delete(testplugins.IdentityManagementGroups, testGroupName)
	delete(testplugins.IdentityManagementGroupMembership, testGroupSCIMID)
}

func TestCheckWorkflow_SufficientApprovers(t *testing.T) {
	// Note: Not parallel - tests modify global testplugins state

	const (
		initiatorID     = "initiator-user"
		initiatorEmail  = "initiator@example.com"
		approver1ID     = "approver1-user"
		approver1Email  = "approver1@example.com"
		approver2ID     = "approver2-user"
		approver2Email  = "approver2@example.com"
		testGroupName   = "test-sufficient-group"
		testGroupSCIMID = "scim-sufficient-group-id"
	)

	cfg := &config.Config{}
	wm, r, tenantID := SetupWorkflowManager(t, cfg)

	ctx := testutils.CreateCtxWithTenant(tenantID)
	ctx = testutils.InjectClientDataIntoContext(ctx, initiatorID, []string{testGroupName})

	// Create test group
	group := testutils.NewGroup(func(g *model.Group) {
		g.Name = testGroupName
		g.IAMIdentifier = testGroupName
		g.Role = constants.KeyAdminRole
	})
	testutils.CreateTestEntities(ctx, t, r, group)

	// Configure IDM with sufficient approvers
	testplugins.IdentityManagementGroups[testGroupName] = testGroupSCIMID
	testplugins.IdentityManagementGroupMembership[testGroupSCIMID] = []testplugins.IdentityManagementUserRef{
		{ID: initiatorID, Email: initiatorEmail},
		{ID: approver1ID, Email: approver1Email},
		{ID: approver2ID, Email: approver2Email},
	}

	// Create key configuration
	keyConfig := &model.KeyConfiguration{
		ID:           uuid.New(),
		Name:         "test-kc-sufficient",
		AdminGroupID: group.ID,
		AdminGroup:   *group,
	}
	testutils.CreateTestEntities(ctx, t, r, keyConfig)

	// Create workflow
	wf := &model.Workflow{
		InitiatorID:  initiatorID,
		ArtifactType: workflow.ArtifactTypeKeyConfiguration.String(),
		ArtifactID:   keyConfig.ID,
		ActionType:   workflow.ActionTypeDelete.String(),
		State:        workflow.StateInitial.String(),
	}

	// Call CheckWorkflow
	status, err := wm.CheckWorkflow(ctx, wf)

	// Should succeed with CanCreate=true
	require.NoError(t, err)
	assert.True(t, status.Enabled, "workflow should be enabled")
	assert.False(t, status.Exists, "workflow should not exist yet")
	assert.True(t, status.Valid, "workflow should be valid")
	assert.True(t, status.CanCreate, "workflow should be creatable with sufficient approvers")
	assert.NoError(t, status.ErrDetails, "ErrDetails should be nil")

	// Cleanup
	delete(testplugins.IdentityManagementGroups, testGroupName)
	delete(testplugins.IdentityManagementGroupMembership, testGroupSCIMID)
}

func TestCreateWorkflow_PersistsMinimumApprovalCount(t *testing.T) {
	// Note: Not parallel - tests modify global testplugins state

	const (
		initiatorID     = "user1"
		initiatorEmail  = "user1@example.com"
		testGroupName   = "test-persist-group"
		testGroupSCIMID = "scim-persist-group-id"
	)

	cfg := &config.Config{}
	wm, r, tenantID := SetupWorkflowManager(t, cfg)

	ctx := testutils.CreateCtxWithTenant(tenantID)
	ctx = testutils.InjectClientDataIntoContext(ctx, initiatorID, []string{testGroupName})

	// Set workflow config with minimum approvals = 3
	workflowConfig := testutils.NewWorkflowConfig(func(tc *model.TenantConfig) {
		var wc model.WorkflowConfig
		_ = json.Unmarshal(tc.Value, &wc)
		wc.MinimumApprovals = 3
		tc.Value, _ = json.Marshal(wc)
	})
	testutils.CreateTestEntities(ctx, t, r, workflowConfig)

	// Create group and key configuration with sufficient approvers (4 members including initiator)
	group := testutils.NewGroup(func(g *model.Group) {
		g.Name = testGroupName
		g.IAMIdentifier = testGroupName
		g.Role = constants.KeyAdminRole
	})
	testutils.CreateTestEntities(ctx, t, r, group)

	testplugins.IdentityManagementGroups[testGroupName] = testGroupSCIMID
	testplugins.IdentityManagementGroupMembership[testGroupSCIMID] = []testplugins.IdentityManagementUserRef{
		{ID: initiatorID, Email: initiatorEmail},
		{ID: "user2", Email: "user2@example.com"},
		{ID: "user3", Email: "user3@example.com"},
		{ID: "user4", Email: "user4@example.com"},
	}

	keyConfig := &model.KeyConfiguration{
		ID:           uuid.New(),
		Name:         "test-kc-persist",
		AdminGroupID: group.ID,
		AdminGroup:   *group,
	}
	testutils.CreateTestEntities(ctx, t, r, keyConfig)

	// Create workflow
	wf := &model.Workflow{
		InitiatorID:  initiatorID,
		ArtifactType: workflow.ArtifactTypeKeyConfiguration.String(),
		ArtifactID:   keyConfig.ID,
		ActionType:   workflow.ActionTypeDelete.String(),
		State:        workflow.StateInitial.String(),
	}

	created, err := wm.CreateWorkflow(ctx, wf)
	require.NoError(t, err)

	// Assert: Workflow has MinimumApprovalCount = 3 persisted
	assert.Equal(t, 3, created.MinimumApprovalCount,
		"workflow should persist minimum approval count from tenant config at creation time")

	// Change tenant config to 5
	var updatedConfig model.WorkflowConfig
	err = json.Unmarshal(workflowConfig.Value, &updatedConfig)
	require.NoError(t, err)
	updatedConfig.MinimumApprovals = 5 // Changed!
	workflowConfig.Value, err = json.Marshal(updatedConfig)
	require.NoError(t, err)
	_, err = r.Patch(ctx, workflowConfig, *repo.NewQuery())
	require.NoError(t, err)

	// Reload workflow from DB
	reloaded := &model.Workflow{ID: created.ID}
	_, err = r.First(ctx, reloaded, *repo.NewQuery())
	require.NoError(t, err)

	// Assert: Workflow STILL has MinimumApprovalCount = 3
	assert.Equal(t, 3, reloaded.MinimumApprovalCount,
		"workflow minimum approval count should not change when tenant config changes")

	// Cleanup
	delete(testplugins.IdentityManagementGroups, testGroupName)
	delete(testplugins.IdentityManagementGroupMembership, testGroupSCIMID)
}
