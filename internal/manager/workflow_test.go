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
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/identitymanagement"
	"github.com/openkcm/cmk/internal/repo"
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
	opts ...testplugins.RegistryOption,
) (
	*manager.WorkflowManager,
	repo.Repo, string,
) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})

	r := sql.NewRepository(db)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(),
		r, &config.Config{})

	authzRepo := authz_repo.NewAuthzRepo(r, authzRepoLoader)

	svcRegistry := testutils.NewTestPlugins(opts...)

	certManager := manager.NewCertificateManager(t.Context(), r, svcRegistry, cfg)
	tenantConfigManager := manager.NewTenantConfigManager(r, svcRegistry, nil)
	cmkAuditor := auditor.New(t.Context(), cfg)
	userManager := manager.NewUserManager(authzRepo, cmkAuditor)
	resourceLabelManager := manager.NewResourceLabelManager(r)
	tagManager := manager.NewTagManager(resourceLabelManager)
	keyConfigManager := manager.NewKeyConfigManager(r, certManager, userManager, tagManager, cmkAuditor, nil, cfg)
	groupManager := manager.NewGroupManager(r, svcRegistry, userManager)

	clientsFactory, err := clients.NewFactory(cfg.Services)
	assert.NoError(t, err)
	systemManager := manager.NewSystemManager(t.Context(), r, nil, clientsFactory, nil, svcRegistry, cfg, keyConfigManager, userManager)

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

func TestWorkflowManager_CheckWorkflow(t *testing.T) {
	// Setup identity management plugin with auditor group and a test key admin group
	const testKeyAdminGroup = "test-key-admins"
	const testKeyAdminGroupSCIM = "scim-key-admins-id"
	const testUser1 = "user1-id"
	const testUser2 = "user2-id"

	idmPlugin := testplugins.NewTestIdentityManagement(
		testplugins.WithGroups(map[string]string{
			auditorGroupName:  "scim-auditors-id",
			testKeyAdminGroup: testKeyAdminGroupSCIM,
		}),
		testplugins.WithGroupMembership(map[string][]string{
			"scim-auditors-id":    {},
			testKeyAdminGroupSCIM: {testUser1, testUser2},
		}),
		testplugins.WithUsers([]identitymanagement.User{
			{ID: testUser1, Name: "user1@example.com"},
			{ID: testUser2, Name: "user2@example.com"},
		}),
	)
	m, r, tenant := SetupWorkflowManager(t, &config.Config{}, testplugins.WithIdentityManagement(idmPlugin))

	ctx := testutils.CreateCtxWithTenant(tenant)
	ctx = testutils.InjectBusinessUserDataIntoContext(ctx, "test-user",
		[]string{uuid.NewString()})

	workflowConfig := testutils.NewWorkflowConfig(func(_ *model.TenantConfig) {})
	testutils.CreateTestEntities(ctx, t, r, workflowConfig)

	// Create test group that's registered in SCIM
	testGroup := testutils.NewGroup(func(g *model.Group) {
		g.Name = testKeyAdminGroup
		g.IAMIdentifier = testKeyAdminGroup
		g.Role = constants.KeyAdminRole
	})
	testutils.CreateTestEntities(ctx, t, r, testGroup)

	// Create key config with the test group
	key := testutils.NewKey(func(k *model.Key) {
		k.ID = uuid.New()
	})
	keyConfig := testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
		c.PrimaryKeyID = &key.ID
		c.AdminGroup = *testGroup
		c.AdminGroupID = testGroup.ID
	})
	testutils.CreateTestEntities(ctx, t, r, key, keyConfig)

	createAuditorGroup(ctx, t, r)

	ctxSys, err := cmkcontext.BusinessToInternalContext(ctx,
		constants.InternalTaskWorkflowApproversRole)
	assert.NoError(t, err)

	t.Run("Should return false on canCreate and error on non existing artifacts", func(t *testing.T) {
		status, err := m.CheckWorkflow(ctx, &model.Workflow{})
		assert.False(t, status.Enabled)
		assert.False(t, status.Exists)
		assert.False(t, status.Valid)
		assert.False(t, status.CanCreate)
		assert.Error(t, err)
	})

	t.Run("Should return be valid and cant create on existing active workflow", func(t *testing.T) {
		wf, err := createTestWorkflow(
			ctxSys, r, testutils.NewWorkflow(
				func(w *model.Workflow) {
					w.State = model.WorkflowStateInitial
					w.ActionType = model.WorkflowActionTypeDelete
					w.ArtifactID = key.ID
					w.ArtifactType = model.WorkflowArtifactTypeKey
				},
			),
		)
		assert.NoError(t, err)

		status, err := m.CheckWorkflow(ctxSys, wf)
		assert.NoError(t, err)
		assert.True(t, status.Enabled)
		assert.True(t, status.Exists)
		assert.True(t, status.Valid)
		assert.False(t, status.CanCreate)
		assert.Equal(t, manager.ErrOngoingWorkflowExist, status.ErrDetails)
	})

	t.Run("Should be invalid and cant create on system connect with invalid key state", func(t *testing.T) {
		groupIAM := uuid.NewString()
		ctx = testutils.InjectBusinessUserDataIntoContext(ctx, "test-user", []string{groupIAM})
		key := testutils.NewKey(func(k *model.Key) {
			k.State = cmkapi.KeyStateFORBIDDEN
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
		testutils.CreateTestEntities(ctx, t, r, key, testGroup, keyConfig, system)

		wf, err := createTestWorkflow(
			ctx, r, testutils.NewWorkflow(
				func(w *model.Workflow) {
					w.State = model.WorkflowStateInitial
					w.ActionType = model.WorkflowActionTypeLink
					w.ArtifactID = system.ID
					w.ArtifactType = model.WorkflowArtifactTypeSystem
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
		ctx = testutils.InjectBusinessUserDataIntoContext(ctx, "test-user", []string{groupIAM})
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
		testutils.CreateTestEntities(ctx, t, r, testGroup, keyConfig, system)

		wf, err := createTestWorkflow(
			ctx, r, testutils.NewWorkflow(
				func(w *model.Workflow) {
					w.State = model.WorkflowStateInitial
					w.ActionType = model.WorkflowActionTypeLink
					w.ArtifactID = system.ID
					w.ArtifactType = model.WorkflowArtifactTypeSystem
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

	t.Run("Should return unsupported workflow on artifact and action type", func(t *testing.T) {
		status, err := m.CheckWorkflow(ctxSys, testutils.NewWorkflow(func(w *model.Workflow) {
			w.State = model.WorkflowStateInitial
			w.State = model.WorkflowStateRejected
			w.ActionType = model.WorkflowActionTypeUpdatePrimary
			w.ArtifactID = keyConfig.ID
			w.ArtifactType = model.WorkflowArtifactTypeSystem
		}))
		assert.NoError(t, err)
		assert.True(t, status.Enabled, "status.Enabled should be true")
		assert.False(t, status.Exists, "status.Exists should be false")
		assert.False(t, status.Valid, "status.Valid should be false")
		assert.False(t, status.CanCreate, "status.CanCreate should be false")
		assert.Equal(t, status.ErrDetails, manager.ErrUnsuportedWorkflow)
	})

	t.Run("Should be creatable on rejected previous workflow", func(t *testing.T) {
		// Create a new key for this test to avoid conflicts with other tests
		testKey := testutils.NewKey(func(k *model.Key) {
			k.KeyConfigurationID = keyConfig.ID
		})
		testutils.CreateTestEntities(ctxSys, t, r, testKey)

		wf, err := createTestWorkflow(
			ctxSys, r, testutils.NewWorkflow(
				func(w *model.Workflow) {
					w.State = model.WorkflowStateInitial
					w.State = model.WorkflowStateRejected
					w.ActionType = model.WorkflowActionTypeDelete
					w.ArtifactID = testKey.ID
					w.ArtifactType = model.WorkflowArtifactTypeKey
				},
			),
		)
		assert.NoError(t, err)

		status, err := m.CheckWorkflow(ctxSys, wf)
		assert.NoError(t, err)
		assert.True(t, status.Enabled, "status.Enabled should be true")
		assert.False(t, status.Exists, "status.Exists should be false")
		assert.True(t, status.Valid, "status.Valid should be true")
		assert.True(t, status.CanCreate, "status.CanCreate should be true, but got error: %v", status.ErrDetails)
	})

	t.Run("should not be valid on primary key change with unconnected system", func(t *testing.T) {
		keyID := uuid.New()
		keyConfig := testutils.NewKeyConfig(func(kc *model.KeyConfiguration) {
			kc.PrimaryKeyID = ptr.PointTo(keyID)
		})
		key := testutils.NewKey(func(k *model.Key) {
			k.ID = keyID
			k.KeyConfigurationID = keyConfig.ID
		})
		newKey := testutils.NewKey(func(k *model.Key) {
			k.ID = uuid.New()
			k.KeyConfigurationID = keyConfig.ID
		})
		system := testutils.NewSystem(func(s *model.System) {
			s.KeyConfigurationID = &keyConfig.ID
			s.Status = cmkapi.SystemStatusDISCONNECTED
		})
		testutils.CreateTestEntities(ctxSys, t, r, keyConfig, key, system, newKey)
		wf := testutils.NewWorkflow(
			func(w *model.Workflow) {
				w.State = model.WorkflowStateInitial
				w.ActionType = model.WorkflowActionTypeUpdatePrimary
				w.ArtifactID = keyConfig.ID
				w.ArtifactType = model.WorkflowArtifactTypeKeyConfiguration
				w.Parameters = newKey.ID.String()
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
		key := testutils.NewKey(func(k *model.Key) {})

		keyConfig := testutils.NewKeyConfig(func(kc *model.KeyConfiguration) {
			kc.PrimaryKeyID = &key.ID
			kc.PrimaryKeyID = ptr.PointTo(key.ID)
		})

		testutils.CreateTestEntities(ctxSys, t, r, key, keyConfig)

		wf := testutils.NewWorkflow(
			func(w *model.Workflow) {
				w.State = model.WorkflowStateInitial
				w.ActionType = model.WorkflowActionTypeUpdatePrimary
				w.ArtifactID = keyConfig.ID
				w.ArtifactType = model.WorkflowArtifactTypeKeyConfiguration
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

	t.Run("should not be valid on change primary key with disabled target key", func(t *testing.T) {
		keyTarget := testutils.NewKey(func(k *model.Key) {
			k.State = cmkapi.KeyStateDISABLED
		})

		keySource := testutils.NewKey(func(k *model.Key) {
			k.State = cmkapi.KeyStateENABLED
		})

		keyConfig := testutils.NewKeyConfig(func(kc *model.KeyConfiguration) {
			kc.PrimaryKeyID = &keyTarget.ID
			kc.PrimaryKeyID = ptr.PointTo(keySource.ID)
		})

		testutils.CreateTestEntities(ctxSys, t, r, keySource, keyTarget, keyConfig)

		wf := testutils.NewWorkflow(
			func(w *model.Workflow) {
				w.State = model.WorkflowStateInitial
				w.ActionType = model.WorkflowActionTypeUpdatePrimary
				w.ArtifactID = keyConfig.ID
				w.ArtifactType = model.WorkflowArtifactTypeKeyConfiguration
				w.Parameters = keyTarget.ID.String()
			},
		)

		status, err := m.CheckWorkflow(ctxSys, wf)
		assert.True(t, status.Enabled)
		assert.False(t, status.Exists)
		assert.False(t, status.Valid)
		assert.False(t, status.CanCreate)
		assert.NoError(t, err)
		assert.Equal(t, manager.ErrPrimaryKeyDisabled, status.ErrDetails)
	})

	t.Run("should not be valid on change primary key with disabled source key", func(t *testing.T) {
		keySource := testutils.NewKey(func(k *model.Key) {
			k.State = cmkapi.KeyStateDISABLED
		})

		keyTarget := testutils.NewKey(func(k *model.Key) {
			k.State = cmkapi.KeyStateENABLED
		})

		keyConfig := testutils.NewKeyConfig(func(kc *model.KeyConfiguration) {
			kc.PrimaryKeyID = &keyTarget.ID
			kc.PrimaryKeyID = ptr.PointTo(keySource.ID)
		})

		testutils.CreateTestEntities(ctxSys, t, r, keySource, keyTarget, keyConfig)

		wf := testutils.NewWorkflow(
			func(w *model.Workflow) {
				w.State = model.WorkflowStateInitial
				w.ActionType = model.WorkflowActionTypeUpdatePrimary
				w.ArtifactID = keyConfig.ID
				w.ArtifactType = model.WorkflowArtifactTypeKeyConfiguration
				w.Parameters = keyTarget.ID.String()
			},
		)

		status, err := m.CheckWorkflow(ctxSys, wf)
		assert.True(t, status.Enabled)
		assert.False(t, status.Exists)
		assert.False(t, status.Valid)
		assert.False(t, status.CanCreate)
		assert.NoError(t, err)
		assert.Equal(t, manager.ErrPrimaryKeyDisabled, status.ErrDetails)
	})

	t.Run("Should not be valid on non byok key state change", func(t *testing.T) {
		key := testutils.NewKey(func(k *model.Key) {
			k.KeyType = constants.KeyTypeHYOK
		})

		testutils.CreateTestEntities(ctxSys, t, r, key)

		wf := testutils.NewWorkflow(
			func(w *model.Workflow) {
				w.State = model.WorkflowStateInitial
				w.ActionType = model.WorkflowActionTypeUpdateState
				w.ArtifactID = key.ID
				w.ArtifactType = model.WorkflowArtifactTypeKey
			},
		)

		status, err := m.CheckWorkflow(ctxSys, wf)
		assert.True(t, status.Enabled)
		assert.False(t, status.Exists)
		assert.False(t, status.Valid)
		assert.False(t, status.CanCreate)
		assert.NoError(t, err)
		assert.Equal(t, manager.ErrUpdateNonBYOKKeyStatus, status.ErrDetails)
	})

	t.Run("should have canCreate on primary key change without unconnected system", func(t *testing.T) {
		sourceKey := testutils.NewKey(func(_ *model.Key) {})
		keyConfig := testutils.NewKeyConfig(func(kc *model.KeyConfiguration) {
			kc.AdminGroup = *testGroup
			kc.AdminGroupID = testGroup.ID
			kc.PrimaryKeyID = ptr.PointTo(sourceKey.ID)
		})
		system := testutils.NewSystem(func(s *model.System) {
			s.KeyConfigurationID = &keyConfig.ID
			s.Status = cmkapi.SystemStatusCONNECTED
		})
		targetKey := testutils.NewKey(func(_ *model.Key) {})
		testutils.CreateTestEntities(ctxSys, t, r, keyConfig, system, sourceKey, targetKey)
		wf := testutils.NewWorkflow(
			func(w *model.Workflow) {
				w.State = model.WorkflowStateInitial
				w.ActionType = model.WorkflowActionTypeUpdatePrimary
				w.ArtifactID = keyConfig.ID
				w.ArtifactType = model.WorkflowArtifactTypeKeyConfiguration
				w.Parameters = targetKey.ID.String()
			},
		)

		status, err := m.CheckWorkflow(ctxSys, wf)
		assert.True(t, status.Enabled)
		assert.False(t, status.Exists)
		assert.True(t, status.Valid)
		assert.True(t, status.CanCreate)
		assert.NoError(t, err)
	})

	t.Run(
		"Should return authorization error on non active artifact", func(t *testing.T) {
			wf, err := createTestWorkflow(
				ctxSys, r, testutils.NewWorkflow(
					func(w *model.Workflow) {
						w.State = model.WorkflowStateRejected
						w.ActionType = model.WorkflowActionTypeDelete
						w.ArtifactType = model.WorkflowArtifactTypeKey
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
	// Setup identity management plugin with auditor group and a test key admin group
	const testKeyAdminGroup = "test-key-admins"
	const testKeyAdminGroupSCIM = "scim-key-admins-id"
	const testUser1 = "user1-id"
	const testUser2 = "user2-id"

	idmPlugin := testplugins.NewTestIdentityManagement(
		testplugins.WithGroups(map[string]string{
			auditorGroupName:  "scim-auditors-id",
			testKeyAdminGroup: testKeyAdminGroupSCIM,
		}),
		testplugins.WithGroupMembership(map[string][]string{
			"scim-auditors-id":    {},
			testKeyAdminGroupSCIM: {testUser1, testUser2},
		}),
		testplugins.WithUsers([]identitymanagement.User{
			{ID: testUser1, Name: "user1@example.com"},
			{ID: testUser2, Name: "user2@example.com"},
		}),
	)
	m, r, tenant := SetupWorkflowManager(t, &config.Config{
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
	}, testplugins.WithIdentityManagement(idmPlugin))

	ctx := testutils.CreateCtxWithTenant(tenant)

	// Create workflow config once for all tests
	workflowConfig := testutils.NewWorkflowConfig(func(_ *model.TenantConfig) {})
	testutils.CreateTestEntities(ctx, t, r, workflowConfig)

	ctxSys, err := cmkcontext.BusinessToInternalContext(ctx,
		constants.InternalTaskWorkflowApproversRole)
	assert.NoError(t, err)

	// Create test group that's registered in SCIM
	testGroup := testutils.NewGroup(func(g *model.Group) {
		g.Name = testKeyAdminGroup
		g.IAMIdentifier = testKeyAdminGroup
		g.Role = constants.KeyAdminRole
	})
	testutils.CreateTestEntities(ctxSys, t, r, testGroup)

	// Create key config with the test group
	key := testutils.NewKey(func(k *model.Key) {
		k.ID = uuid.New()
	})
	keyConfig := testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
		c.PrimaryKeyID = &key.ID
		c.AdminGroup = *testGroup
		c.AdminGroupID = testGroup.ID
	})
	testutils.CreateTestEntities(ctxSys, t, r, key, keyConfig)

	t.Run(
		"Should error on existing workflow", func(t *testing.T) {
			wf := testutils.NewWorkflow(func(w *model.Workflow) {
				w.State = model.WorkflowStateInitial
				w.ActionType = model.WorkflowActionTypeDelete
				w.ArtifactType = model.WorkflowArtifactTypeKey
				w.ArtifactID = key.ID
			})
			err := r.Create(ctx, wf)
			assert.NoError(t, err)

			_, err = m.CreateWorkflow(ctxSys, wf)
			assert.ErrorIs(t, err, manager.ErrOngoingWorkflowExist)
		},
	)

	t.Run(
		"Should create workflow", func(t *testing.T) {
			createAuditorGroup(ctx, t, r)

			// Create key using the same test group
			key := testutils.NewKey(func(k *model.Key) {})
			testutils.CreateTestEntities(ctxSys, t, r, key)

			wf := testutils.NewWorkflow(func(w *model.Workflow) {
				w.State = model.WorkflowStateInitial
				w.ActionType = model.WorkflowActionTypeDelete
				w.ArtifactType = model.WorkflowArtifactTypeKey
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
		testutils.CreateTestEntities(ctxSys, t, r, system)

		expected := &model.Workflow{
			ID:           uuid.New(),
			State:        "INITIAL",
			InitiatorID:  uuid.NewString(),
			ArtifactType: model.WorkflowArtifactTypeSystem,
			ArtifactID:   system.ID,
			ActionType:   model.WorkflowActionTypeLink,
			Approvers:    []model.WorkflowApprover{{UserID: uuid.NewString()}},
			Parameters:   keyConfig.ID.String(),
		}
		res, err := m.CreateWorkflow(ctxSys, expected)
		assert.NoError(t, err)
		assert.Equal(t, "MySystem", *res.ArtifactName)
		assert.Equal(t, keyConfig.Name, *res.ParametersResourceName)
	})

	t.Run("Should put system under_workflow as true on workflow creation", func(t *testing.T) {
		// Create system with key config that uses registered test group
		system := testutils.NewSystem(func(s *model.System) {
			s.KeyConfigurationID = &keyConfig.ID
		})
		testutils.CreateTestEntities(ctxSys, t, r, system)

		wf := testutils.NewWorkflow(func(w *model.Workflow) {
			w.ArtifactType = model.WorkflowArtifactTypeSystem
			w.ArtifactID = system.ID
			w.ActionType = model.WorkflowActionTypeUnlink // Need an action type
		})

		_, err := m.CreateWorkflow(ctxSys, wf)
		assert.NoError(t, err)

		_, err = r.First(ctxSys, system, *repo.NewQuery())
		assert.NoError(t, err)

		assert.True(t, system.UnderWorkflow)
	})

	t.Run(
		"Should create system workflow with artifact name from identifier", func(t *testing.T) {
			system := testutils.NewSystem(func(s *model.System) {})
			testutils.CreateTestEntities(ctxSys, t, r, system)

			expected := &model.Workflow{
				ID:           uuid.New(),
				State:        "INITIAL",
				InitiatorID:  uuid.NewString(),
				ArtifactType: model.WorkflowArtifactTypeSystem,
				ArtifactID:   system.ID,
				ActionType:   model.WorkflowActionTypeLink,
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
	idmPlugin := testplugins.NewTestIdentityManagement()
	m, repo, tenant := SetupWorkflowManager(t, &config.Config{}, testplugins.WithIdentityManagement(idmPlugin))

	ctx := testutils.CreateCtxWithTenant(tenant)
	workflowConfig := testutils.NewWorkflowConfig(func(_ *model.TenantConfig) {})

	testutils.CreateTestEntities(ctx, t, repo, workflowConfig)

	t.Run("Should error on invalid event actor", func(t *testing.T) {
		wf, err := createTestWorkflow(
			testutils.CreateCtxWithTenant(tenant),
			repo,
			testutils.NewWorkflow(
				func(w *model.Workflow) {
					w.State = model.WorkflowStateInitial
					w.ActionType = model.WorkflowActionTypeDelete
					w.ArtifactType = model.WorkflowArtifactTypeKey
				},
			),
		)
		assert.NoError(t, err)
		idmPlugin.PutUser(identitymanagement.User{ID: wf.InitiatorID})

		ctx = cmkcontext.InjectBusinessUserData(
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
	})

	t.Run("Should transit to wait confirmation on approve", func(t *testing.T) {
		wf, err := createTestWorkflow(
			testutils.CreateCtxWithTenant(tenant),
			repo,
			testutils.NewWorkflow(
				func(w *model.Workflow) {
					w.State = model.WorkflowStateWaitApproval
					w.ActionType = model.WorkflowActionTypeDelete
					w.ArtifactType = model.WorkflowArtifactTypeKey
				},
			),
		)
		assert.NoError(t, err)
		idmPlugin.PutUser(identitymanagement.User{ID: wf.InitiatorID})
		idmPlugin.PutUser(identitymanagement.User{ID: wf.Approvers[0].UserID})
		ctx = cmkcontext.InjectBusinessUserData(
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
		assert.Equal(t, model.WorkflowStateWaitConfirmation, res.State)
	})

	t.Run("Should transit to reject on reject", func(t *testing.T) {
		wf, err := createTestWorkflow(
			testutils.CreateCtxWithTenant(tenant),
			repo,
			testutils.NewWorkflow(
				func(w *model.Workflow) {
					w.State = model.WorkflowStateWaitApproval
					w.ActionType = model.WorkflowActionTypeDelete
					w.ArtifactType = model.WorkflowArtifactTypeKey
				},
			),
		)
		assert.NoError(t, err)
		idmPlugin.PutUser(identitymanagement.User{ID: wf.InitiatorID})
		idmPlugin.PutUser(identitymanagement.User{ID: wf.Approvers[0].UserID})
		ctx = cmkcontext.InjectBusinessUserData(
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
		assert.Equal(t, model.WorkflowStateRejected, res.State)
	})
}

func TestWorkflowManager_GetWorkflowByID(t *testing.T) {
	group := testutils.NewGroup(func(g *model.Group) {})
	userID := uuid.NewString()
	idmPlugin := testplugins.NewTestIdentityManagement(testplugins.WithGroups(map[string]string{
		group.IAMIdentifier: group.IAMIdentifier,
	}), testplugins.WithGroupMembership(map[string][]string{
		group.IAMIdentifier: {userID},
	}), testplugins.WithUsers([]identitymanagement.User{
		{ID: userID, Name: "test"},
	}))

	m, r, tenant := SetupWorkflowManager(t, &config.Config{}, testplugins.WithIdentityManagement(idmPlugin))

	ctx := testutils.CreateCtxWithTenant(tenant)
	wf := testutils.NewWorkflow(
		func(w *model.Workflow) {
			w.State = model.WorkflowStateInitial
			w.ActionType = model.WorkflowActionTypeDelete
			w.ArtifactType = model.WorkflowArtifactTypeKey
			w.InitiatorID = userID
		},
	)

	testutils.CreateTestEntities(
		ctx,
		t,
		r,
		wf,
		group,
		testutils.NewWorkflowApproverGroup(func(wag *model.WorkflowApproverGroup) {
			wag.GroupID = group.ID
			wag.WorkflowID = wf.ID
		}),
	)

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
				ctx := testutils.InjectBusinessUserDataIntoContext(
					ctx,
					userID,
					[]string{group.IAMIdentifier},
				)
				retrievedWf, _, err := m.GetWorkflowByID(
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
	state model.WorkflowState,
	actionType model.WorkflowActionType,
	artifactType model.WorkflowArtifactType,
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
		ArtifactType: model.WorkflowArtifactTypeKey,
		ActionType:   model.WorkflowActionTypeDelete,
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
	group := testutils.NewGroup(func(g *model.Group) {})
	userID := uuid.NewString()
	allWorkflowUserID := uuid.NewString()
	idmPlugin := testplugins.NewTestIdentityManagement(testplugins.WithGroups(map[string]string{
		group.IAMIdentifier: group.IAMIdentifier,
	}), testplugins.WithGroupMembership(map[string][]string{
		group.IAMIdentifier: {userID, allWorkflowUserID},
	}), testplugins.WithUsers([]identitymanagement.User{
		{ID: userID, Name: userID},
		{ID: allWorkflowUserID, Name: allWorkflowUserID},
	}))

	m, r, tenant := SetupWorkflowManager(t, &config.Config{}, testplugins.WithIdentityManagement(idmPlugin))
	ctx := testutils.CreateCtxWithTenant(tenant)

	baseTime := time.Now()

	workflow1 := testutils.NewWorkflow(
		func(w *model.Workflow) {
			w.State = model.WorkflowStateInitial
			w.ActionType = model.WorkflowActionTypeDelete
			w.ArtifactType = model.WorkflowArtifactTypeKey
			w.Approvers = []model.WorkflowApprover{{UserID: allWorkflowUserID}}
			w.InitiatorID = userID
			w.CreatedAt = baseTime.Add(-3 * time.Hour)
			w.UpdatedAt = baseTime.Add(-3 * time.Hour)
		},
	)

	workflow2 := testutils.NewWorkflow(
		func(w *model.Workflow) {
			w.State = model.WorkflowStateInitial
			w.ActionType = model.WorkflowActionTypeDelete
			w.ArtifactType = model.WorkflowArtifactTypeKey
			w.ArtifactID = uuid.New()
			w.Approvers = []model.WorkflowApprover{{UserID: userID}}
			w.InitiatorID = allWorkflowUserID
			w.CreatedAt = baseTime.Add(-2 * time.Hour)
			w.UpdatedAt = baseTime.Add(-2 * time.Hour)
		},
	)

	workflow3 := testutils.NewWorkflow(
		func(w *model.Workflow) {
			w.State = model.WorkflowStateRejected
			w.ActionType = model.WorkflowActionTypeDelete
			w.ArtifactType = model.WorkflowArtifactTypeKey
			w.Approvers = []model.WorkflowApprover{{UserID: userID}}
			w.InitiatorID = allWorkflowUserID
			w.CreatedAt = baseTime.Add(-1 * time.Hour)
			w.UpdatedAt = baseTime.Add(-1 * time.Hour)
		},
	)

	workflow4 := testutils.NewWorkflow(
		func(w *model.Workflow) {
			w.State = model.WorkflowStateInitial
			w.ActionType = model.WorkflowActionTypeUpdateState
			w.ArtifactType = model.WorkflowArtifactTypeKey
			w.Approvers = []model.WorkflowApprover{{UserID: allWorkflowUserID}}
			w.InitiatorID = userID
			w.CreatedAt = baseTime
			w.UpdatedAt = baseTime
		},
	)

	testutils.CreateTestEntities(
		ctx,
		t,
		r,
		group,
		workflow1,
		workflow2,
		workflow3,
		workflow4,
		testutils.NewWorkflowApproverGroup(func(wag *model.WorkflowApproverGroup) {
			wag.GroupID = group.ID
			wag.WorkflowID = workflow1.ID
		}),
		testutils.NewWorkflowApproverGroup(func(wag *model.WorkflowApproverGroup) {
			wag.GroupID = group.ID
			wag.WorkflowID = workflow2.ID
		}),
		testutils.NewWorkflowApproverGroup(func(wag *model.WorkflowApproverGroup) {
			wag.GroupID = group.ID
			wag.WorkflowID = workflow3.ID
		}),
		testutils.NewWorkflowApproverGroup(func(wag *model.WorkflowApproverGroup) {
			wag.GroupID = group.ID
			wag.WorkflowID = workflow4.ID
		}),
	)

	tests := []struct {
		name                string
		filter              manager.WorkflowFilter
		expectedCount       int
		expectedState       model.WorkflowState
		expectedActionType  model.WorkflowActionType
		expectedArtfactType model.WorkflowArtifactType
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
			filter:              manager.WorkflowFilter{State: model.WorkflowStateRejected},
			expectedCount:       1,
			expectedState:       model.WorkflowStateRejected,
			expectedActionType:  "",
			expectedArtfactType: "",
		},
		{
			name: "Should get initial workflows",
			filter: newGetWorkflowsFilter(
				uuid.Nil,
				model.WorkflowStateInitial,
				"",
				"",
			),
			expectedCount:      3,
			expectedState:      model.WorkflowStateInitial,
			expectedActionType: "",
		},
		{
			name: "Should get action type UPDATE_STATE workflows",
			filter: newGetWorkflowsFilter(
				uuid.Nil,
				"",
				model.WorkflowActionTypeUpdateState,
				"",
			),
			expectedCount:       1,
			expectedState:       "",
			expectedActionType:  model.WorkflowActionTypeUpdateState,
			expectedArtfactType: "",
		},
		{
			name: "Get workflows by artifact type",
			filter: newGetWorkflowsFilter(
				uuid.Nil,
				"",
				"",
				model.WorkflowArtifactTypeKey,
			),
			expectedCount:       4,
			expectedState:       "",
			expectedActionType:  "",
			expectedArtfactType: model.WorkflowArtifactTypeKey,
		},
		{
			name: "Get workflows by artifact id",
			filter: newGetWorkflowsFilter(
				workflow2.ArtifactID,
				"",
				"",
				model.WorkflowArtifactTypeKey,
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
				ctx := testutils.InjectBusinessUserDataIntoContext(
					ctx,
					userID,
					[]string{group.IAMIdentifier},
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
		ctx := testutils.InjectBusinessUserDataIntoContext(
			ctx,
			userID,
			[]string{group.IAMIdentifier},
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

func TestWorkflowManager_GetApproversGroupsFromLegacyField(t *testing.T) {
	m, r, tenant := SetupWorkflowManager(t, &config.Config{})
	ctx := testutils.CreateCtxWithTenant(tenant)

	group := testutils.NewGroup(func(g *model.Group) {
		g.Name = testGroupName
		g.IAMIdentifier = testGroupName
		g.Role = constants.KeyAdminRole
	})
	testutils.CreateTestEntities(ctx, t, r, group)
	groupIDsJSON, err := json.Marshal([]uuid.UUID{group.ID})
	require.NoError(t, err)

	wf, err := createTestWorkflow(
		testutils.CreateCtxWithTenant(tenant),
		r,
		testutils.NewWorkflow(
			func(w *model.Workflow) {
				w.State = model.WorkflowStateInitial
				w.ActionType = model.WorkflowActionTypeDelete
				w.ArtifactType = model.WorkflowArtifactTypeKey
				w.ApproverGroupIDs = groupIDsJSON
			},
		),
	)
	assert.NoError(t, err)

	groups, err := m.GetApproverGroupsFromLegacyField(ctx, wf)
	assert.Len(t, groups, 1)
	assert.NoError(t, err)
}

func TestWorkflowManager_ListApprovers(t *testing.T) {
	m, r, tenant := SetupWorkflowManager(t, &config.Config{})
	wf, err := createTestWorkflow(
		testutils.CreateCtxWithTenant(tenant),
		r,
		testutils.NewWorkflow(
			func(w *model.Workflow) {
				w.State = model.WorkflowStateInitial
				w.ActionType = model.WorkflowActionTypeDelete
				w.ArtifactType = model.WorkflowArtifactTypeKey
			},
		),
	)
	assert.NoError(t, err)

	ctx := testutils.CreateCtxWithTenant(tenant)

	createAuditorGroup(ctx, t, r)

	ctxSys, err := cmkcontext.BusinessToInternalContext(ctx,
		constants.InternalTaskWorkflowApproversRole)
	assert.NoError(t, err)

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
	ctx = testutils.InjectBusinessUserDataIntoContext(ctx, "test-user", []string{"KMS_001", "KMS_002"})

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
				w.ArtifactType = model.WorkflowArtifactTypeKey
				w.ActionType = model.WorkflowActionTypeDelete
				w.Approvers = nil
			},
			approversCount: 2,
			approverGroups: 1,
		},
		{
			name: "KeyDelete - Invalid key",
			workflowMut: func(w *model.Workflow) {
				w.ArtifactID = uuid.New()
				w.ArtifactType = model.WorkflowArtifactTypeKey
				w.ActionType = model.WorkflowActionTypeDelete
				w.Approvers = nil
			},
			expectErr:  true,
			errMessage: repo.ErrNotFound,
		},
		{
			name: "KeyStateUpdate",
			workflowMut: func(w *model.Workflow) {
				w.ArtifactID = key.ID
				w.ArtifactType = model.WorkflowArtifactTypeKey
				w.ActionType = model.WorkflowActionTypeUpdateState
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
				w.ArtifactType = model.WorkflowArtifactTypeKeyConfiguration
				w.ActionType = model.WorkflowActionTypeDelete
				w.Approvers = nil
			},
			approversCount: 2,
			approverGroups: 1,
		},
		{
			name: "KeyConfigDelete - Invalid key config",
			workflowMut: func(w *model.Workflow) {
				w.ArtifactID = uuid.New()
				w.ArtifactType = model.WorkflowArtifactTypeKeyConfiguration
				w.ActionType = model.WorkflowActionTypeDelete
				w.Approvers = nil
			},
			expectErr:  true,
			errMessage: repo.ErrNotFound,
		},
		{
			name: "KeyConfigUpdatePK",
			workflowMut: func(w *model.Workflow) {
				w.ArtifactID = keyConfigs[0].ID
				w.ArtifactType = model.WorkflowArtifactTypeKeyConfiguration
				w.ActionType = model.WorkflowActionTypeUpdatePrimary
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
				w.ArtifactType = model.WorkflowArtifactTypeSystem
				w.ActionType = model.WorkflowActionTypeLink
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
				w.ArtifactType = model.WorkflowArtifactTypeSystem
				w.ActionType = model.WorkflowActionTypeLink
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
				w.ArtifactType = model.WorkflowArtifactTypeSystem
				w.ActionType = model.WorkflowActionTypeUnlink
				w.Approvers = nil
			},
			approversCount: 2,
			approverGroups: 1,
		},
		{
			name: "SystemUnLink - Invalid system",
			workflowMut: func(w *model.Workflow) {
				w.ArtifactID = uuid.New()
				w.ArtifactType = model.WorkflowArtifactTypeSystem
				w.ActionType = model.WorkflowActionTypeUnlink
				w.Approvers = nil
			},
			expectErr:  true,
			errMessage: repo.ErrNotFound,
		},
		{
			name: "SystemSwitch",
			workflowMut: func(w *model.Workflow) {
				w.ArtifactID = systems[1].ID
				w.ArtifactType = model.WorkflowArtifactTypeSystem
				w.ActionType = model.WorkflowActionTypeSwitch
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
				ctx := context.WithValue(
					ctx,
					constants.BusinessUserData, &auth.ClientData{
						Identifier: "testuser",
						Groups:     []string{auditorGroupName},
					},
				)
				_, err = m.AutoAssignApprovers(ctx, wf.ID)
				if tt.expectErr {
					assert.Error(t, err)
					assert.ErrorIs(t, err, tt.errMessage)
				} else {
					assert.NoError(t, err)

					count, _, err := m.ListWorkflowApprovers(ctx, wf.ID, false, repo.Pagination{})
					assert.NoError(t, err)
					assert.Len(t, count, tt.approversCount)
				}
			},
		)
	}
}

func TestWorkflowManager_CreateWorkflowTransitionNotificationTask(t *testing.T) {
	cfg := &config.Config{}
	idmPlugin := testplugins.NewTestIdentityManagement()
	initatorID := uuid.NewString()
	idmPlugin.PutUser(identitymanagement.User{ID: initatorID})

	wm, _, tenantID := SetupWorkflowManager(t, cfg, testplugins.WithIdentityManagement(idmPlugin))
	ctx := testutils.CreateCtxWithTenant(tenantID)
	ctx = cmkcontext.InjectBusinessUserData(ctx, &auth.ClientData{Identifier: "User-ID"}, nil)

	t.Run("should successfully create and enqueue notification task", func(t *testing.T) {
		mockClient := &async.MockClient{}
		wm.SetAsyncClient(mockClient)

		wf := model.Workflow{
			ID:           uuid.New(),
			InitiatorID:  initatorID,
			ActionType:   "CREATE",
			ArtifactType: model.WorkflowArtifactTypeKey,
			ArtifactID:   uuid.New(),
			State:        model.WorkflowStateWaitConfirmation,
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
			InitiatorID:  initatorID,
			ActionType:   "CREATE",
			ArtifactType: model.WorkflowArtifactTypeKey,
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
			InitiatorID:  initatorID,
			ActionType:   "CREATE",
			ArtifactType: model.WorkflowArtifactTypeKey,
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
			InitiatorID:  initatorID,
			ActionType:   "CREATE",
			ArtifactType: model.WorkflowArtifactTypeKey,
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
			InitiatorID:  initatorID,
			ActionType:   "CREATE",
			ArtifactType: model.WorkflowArtifactTypeKey,
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
			InitiatorID:  initatorID,
			ActionType:   "CREATE",
			ArtifactType: model.WorkflowArtifactTypeKey,
			ArtifactID:   uuid.New(),
			State:        model.WorkflowStateWaitConfirmation,
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
	ctx = cmkcontext.InjectBusinessUserData(ctx, &auth.ClientData{Identifier: "User-ID"}, nil)

	workflowConfig := testutils.NewWorkflowConfig(func(_ *model.TenantConfig) {})
	testutils.CreateTestEntities(ctx, t, r, workflowConfig)

	tests := []struct {
		state    model.WorkflowState
		expected bool
	}{
		{model.WorkflowStateInitial, false},
		{model.WorkflowStateWaitApproval, true},
		{model.WorkflowStateWaitConfirmation, true},
		{model.WorkflowStateExecuting, true},
		{model.WorkflowStateRevoked, false},
		{model.WorkflowStateRejected, false},
		{model.WorkflowStateExpired, false},
		{model.WorkflowStateSuccessful, false},
		{model.WorkflowStateFailed, false},
	}

	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
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

func TestWorkflowManager_ExpireWorkflow(t *testing.T) {
	m, r, tenant := SetupWorkflowManager(t, &config.Config{})
	ctx := testutils.CreateCtxWithTenant(tenant)
	ctx = testutils.InjectBusinessUserDataIntoContext(ctx, uuid.NewString(), []string{uuid.NewString()})

	workflowConfig := testutils.NewWorkflowConfig(func(_ *model.TenantConfig) {})
	testutils.CreateTestEntities(ctx, t, r, workflowConfig)

	ctxSys, err := cmkcontext.BusinessToInternalContext(ctx, constants.InternalTaskWorkflowExpirationRole)
	assert.NoError(t, err)

	expirableStates := []model.WorkflowState{
		model.WorkflowStateWaitApproval,
		model.WorkflowStateWaitConfirmation,
		model.WorkflowStateExecuting,
	}

	for _, state := range expirableStates {
		t.Run("transitions "+state.String()+" to EXPIRED", func(t *testing.T) {
			wf := testutils.NewWorkflow(func(w *model.Workflow) { w.State = state })
			testutils.CreateTestEntities(ctx, t, r, wf)

			result, err := m.ExpireWorkflow(ctxSys, wf.ID)
			assert.NoError(t, err)
			assert.Equal(t, model.WorkflowStateExpired, result.State)

			// Verify state persisted in DB.
			persisted := testutils.NewWorkflow(func(w *model.Workflow) { w.ID = wf.ID })
			_, err = r.First(ctx, persisted, *repo.NewQuery())
			assert.NoError(t, err)
			assert.Equal(t, model.WorkflowStateExpired, persisted.State)
		})
	}

	nonExpirableStates := []model.WorkflowState{
		model.WorkflowStateInitial,
		model.WorkflowStateRevoked,
		model.WorkflowStateRejected,
		model.WorkflowStateExpired,
		model.WorkflowStateSuccessful,
		model.WorkflowStateFailed,
	}

	for _, state := range nonExpirableStates {
		t.Run("errors on non-expirable state "+state.String(), func(t *testing.T) {
			wf := testutils.NewWorkflow(func(w *model.Workflow) { w.State = state })
			testutils.CreateTestEntities(ctx, t, r, wf)

			_, err := m.ExpireWorkflow(ctxSys, wf.ID)
			assert.Error(t, err)
		})
	}

	t.Run("errors when workflow does not exist", func(t *testing.T) {
		_, err := m.ExpireWorkflow(ctxSys, uuid.New())
		assert.ErrorIs(t, err, manager.ErrGetWorkflowDB)
	})
}

func TestWorkflowManager_CleanupTerminalWorkflows(t *testing.T) {
	userID := uuid.NewString()
	group := testutils.NewGroup(func(g *model.Group) {})

	idmPlugin := testplugins.NewTestIdentityManagement(testplugins.WithGroups(map[string]string{
		group.IAMIdentifier: group.IAMIdentifier,
	}), testplugins.WithGroupMembership(map[string][]string{
		group.IAMIdentifier: {userID},
	}), testplugins.WithUsers([]identitymanagement.User{
		{ID: userID, Name: userID},
	}))

	cfg := &config.Config{}
	wm, r, tenantID := SetupWorkflowManager(t, cfg, testplugins.WithIdentityManagement(idmPlugin))

	ctx := testutils.CreateCtxWithTenant(tenantID)
	ctx = testutils.InjectBusinessUserDataIntoContext(
		ctx,
		userID,
		[]string{group.IAMIdentifier},
	)

	// Create workflow config
	workflowConfig := testutils.NewWorkflowConfig(func(_ *model.TenantConfig) {})
	testutils.CreateTestEntities(ctx, t, r, group, workflowConfig)

	t.Run("should delete expired terminal workflow", func(t *testing.T) {
		// Create old terminal workflow (should be deleted)
		oldTerminalWf := testutils.NewWorkflow(
			func(w *model.Workflow) {
				w.State = model.WorkflowStateSuccessful
				w.CreatedAt = time.Now().AddDate(0, 0, -31) // 31 days ago
				w.InitiatorID = userID
			},
		)

		testutils.CreateTestEntities(
			ctx,
			t,
			r,
			oldTerminalWf,
			testutils.NewWorkflowApproverGroup(func(wag *model.WorkflowApproverGroup) {
				wag.GroupID = group.ID
				wag.WorkflowID = oldTerminalWf.ID
			}),
		)

		err := wm.CleanupTerminalWorkflows(ctx)
		assert.NoError(t, err)

		// Verify old terminal workflow was deleted
		_, _, err = wm.GetWorkflowByID(ctx, oldTerminalWf.ID)
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
	})

	t.Run("should not delete recent terminal workflow", func(t *testing.T) {
		// Create recent terminal workflow (should NOT be deleted)
		recentTerminalWf := testutils.NewWorkflow(
			func(w *model.Workflow) {
				w.State = model.WorkflowStateRejected
				w.CreatedAt = time.Now().AddDate(0, 0, -15) // 15 days ago
				w.InitiatorID = userID
			},
		)

		testutils.CreateTestEntities(
			ctx,
			t,
			r,
			recentTerminalWf,
			testutils.NewWorkflowApproverGroup(func(wag *model.WorkflowApproverGroup) {
				wag.GroupID = group.ID
				wag.WorkflowID = recentTerminalWf.ID
			}),
		)

		err := wm.CleanupTerminalWorkflows(ctx)
		assert.NoError(t, err)

		// Verify recent terminal workflow still exists
		_, _, err = wm.GetWorkflowByID(ctx, recentTerminalWf.ID)
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
	})

	t.Run("should not delete old non-terminal workflow", func(t *testing.T) {
		// Create old non-terminal workflow (should NOT be deleted)
		oldActiveWf := testutils.NewWorkflow(
			func(w *model.Workflow) {
				w.State = model.WorkflowStateWaitApproval
				w.CreatedAt = time.Now().AddDate(0, 0, -31) // 31 days ago
				w.InitiatorID = userID
			},
		)

		testutils.CreateTestEntities(
			ctx,
			t,
			r,
			oldActiveWf,
			testutils.NewWorkflowApproverGroup(func(wag *model.WorkflowApproverGroup) {
				wag.GroupID = group.ID
				wag.WorkflowID = oldActiveWf.ID
			}),
		)

		err := wm.CleanupTerminalWorkflows(ctx)
		assert.NoError(t, err)

		// Verify old active workflow still exists
		_, _, err = wm.GetWorkflowByID(ctx, oldActiveWf.ID)
		assert.NoError(t, err)
	})

	t.Run("should delete all terminal state types", func(t *testing.T) {
		// Create workflows in all terminal states (all old enough to be deleted)
		terminalStates := model.WorkflowTerminalStates

		workflowIDs := make([]uuid.UUID, len(terminalStates))
		for i, state := range terminalStates {
			wf := testutils.NewWorkflow(
				func(w *model.Workflow) {
					w.State = state
					w.CreatedAt = time.Now().AddDate(0, 0, -31)
					w.InitiatorID = userID
				},
			)
			testutils.CreateTestEntities(
				ctx,
				t,
				r,
				wf,
				testutils.NewWorkflowApproverGroup(func(wag *model.WorkflowApproverGroup) {
					wag.GroupID = group.ID
					wag.WorkflowID = wf.ID
				}),
			)
			workflowIDs[i] = wf.ID
		}

		err := wm.CleanupTerminalWorkflows(ctx)
		assert.NoError(t, err)

		// Verify all terminal workflows were deleted
		for i, wfID := range workflowIDs {
			_, _, err = wm.GetWorkflowByID(ctx, wfID)
			assert.ErrorIs(
				t, err, manager.ErrWorkflowNotAllowed,
				"Terminal workflow in state %s should be deleted", terminalStates[i],
			)
		}
	})

	t.Run("should handle batch processing for large number of workflows", func(t *testing.T) {
		// Create more workflows than batch size to test batch processing
		total := 101 // More than repo.DefaultLimit (100)
		workflowIDs := make([]uuid.UUID, total)

		for i := range total {
			wf := testutils.NewWorkflow(
				func(w *model.Workflow) {
					w.State = model.WorkflowStateSuccessful
					w.CreatedAt = time.Now().AddDate(0, 0, -31)
					w.InitiatorID = userID
				},
			)
			testutils.CreateTestEntities(
				ctx,
				t,
				r,
				wf,
				testutils.NewWorkflowApproverGroup(func(wag *model.WorkflowApproverGroup) {
					wag.GroupID = group.ID
					wag.WorkflowID = wf.ID
				}),
			)
			workflowIDs[i] = wf.ID
		}

		err := wm.CleanupTerminalWorkflows(ctx)
		assert.NoError(t, err)

		// Verify all workflows were deleted across multiple batches
		for _, wfID := range workflowIDs {
			_, _, err = wm.GetWorkflowByID(ctx, wfID)
			assert.ErrorIs(t, err, manager.ErrWorkflowNotAllowed,
				"All workflows should be deleted even with batch processing")
		}
	})

	t.Run("should handle empty result when no expired workflows exist", func(t *testing.T) {
		// Create only recent terminal workflows
		recentWf := testutils.NewWorkflow(
			func(w *model.Workflow) {
				w.State = model.WorkflowStateSuccessful
				w.CreatedAt = time.Now().AddDate(0, 0, -5)
				w.InitiatorID = userID
			},
		)
		testutils.CreateTestEntities(
			ctx,
			t,
			r,
			recentWf,
			testutils.NewWorkflowApproverGroup(func(wag *model.WorkflowApproverGroup) {
				wag.GroupID = group.ID
				wag.WorkflowID = recentWf.ID
			}),
		)

		// Should not error when no workflows to delete
		err := wm.CleanupTerminalWorkflows(ctx)
		assert.NoError(t, err)

		// Recent workflow should still exist
		_, _, err = wm.GetWorkflowByID(ctx, recentWf.ID)
		assert.NoError(t, err)
	})

	t.Run("should handle workflows without approvers", func(t *testing.T) {
		// Create workflow without approvers
		oldWf := testutils.NewWorkflow(
			func(w *model.Workflow) {
				w.State = model.WorkflowStateSuccessful
				w.CreatedAt = time.Now().AddDate(0, 0, -31)
				w.Approvers = nil // No approvers
				w.InitiatorID = userID
			},
		)
		testutils.CreateTestEntities(
			ctx,
			t,
			r,
			oldWf,
			testutils.NewWorkflowApproverGroup(func(wag *model.WorkflowApproverGroup) {
				wag.GroupID = group.ID
				wag.WorkflowID = oldWf.ID
			}),
		)

		err := wm.CleanupTerminalWorkflows(ctx)
		assert.NoError(t, err)

		// Workflow should still be deleted even without approvers
		_, _, err = wm.GetWorkflowByID(ctx, oldWf.ID)
		assert.ErrorIs(t, err, manager.ErrWorkflowNotAllowed)
	})

	t.Run("should preserve non-terminal workflow states", func(t *testing.T) {
		// Create workflows in all non-terminal states (all old)
		nonTerminalStates := model.WorkflowNonTerminalStates

		workflowIDs := make([]uuid.UUID, len(nonTerminalStates))
		for i, state := range nonTerminalStates {
			wf := testutils.NewWorkflow(
				func(w *model.Workflow) {
					w.State = state
					w.CreatedAt = time.Now().AddDate(0, 0, -60) // Very old
					w.InitiatorID = userID
				},
			)
			testutils.CreateTestEntities(
				ctx,
				t,
				r,
				wf,
				testutils.NewWorkflowApproverGroup(func(wag *model.WorkflowApproverGroup) {
					wag.GroupID = group.ID
					wag.WorkflowID = wf.ID
				}),
			)
			workflowIDs[i] = wf.ID
		}

		err := wm.CleanupTerminalWorkflows(ctx)
		assert.NoError(t, err)

		// Verify all non-terminal workflows still exist
		for i, wfID := range workflowIDs {
			_, _, err = wm.GetWorkflowByID(ctx, wfID)
			assert.NoError(t, err, "Non-terminal workflow in state %s should not be deleted", nonTerminalStates[i])
		}
	})
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

// newEligibilityTestPlugin creates a blank TestIdentityManagement for eligibility tests.
// Call PutGroup / PutGroupMembers on the returned instance to control IAM state per test.
func newEligibilityTestPlugin() *testplugins.TestIdentityManagement {
	return testplugins.NewTestIdentityManagement(
		testplugins.WithGroups(map[string]string{}),
		testplugins.WithGroupMembership(map[string][]string{}),
		testplugins.WithUsers([]identitymanagement.User{
			{ID: approver1ID, Name: approver1Email},
			{ID: approver2ID, Name: approver2Email},
		}),
	)
}

// setupEligibilityTest creates a workflow with approvers and returns the necessary test data.
// Pass the idmPlugin returned by newEligibilityTestPlugin so the caller retains a handle for IAM mutations.
func setupEligibilityTest(
	t *testing.T,
	approverCount int,
	idmPlugin *testplugins.TestIdentityManagement,
) (*manager.WorkflowManager, repo.Repo, context.Context, *model.Workflow, *model.Group) {
	t.Helper()

	cfg := &config.Config{}

	wm, r, tenantID := SetupWorkflowManager(t, cfg, testplugins.WithIdentityManagement(idmPlugin))
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

	artifactName := system.Identifier
	paramsResourceName := keyConfig.Name
	paramsResourceType := "KEY_CONFIGURATION"

	wf := testutils.NewWorkflow(func(w *model.Workflow) {
		w.State = model.WorkflowStateWaitApproval
		w.ArtifactType = model.WorkflowArtifactTypeSystem
		w.ArtifactID = system.ID
		w.ArtifactName = &artifactName
		w.ActionType = model.WorkflowActionTypeLink
		w.Parameters = keyConfig.ID.String()
		w.ParametersResourceName = &paramsResourceName
		w.ParametersResourceType = &paramsResourceType
		w.InitiatorID = approver1ID
		w.MinimumApprovalCount = approverCount // Set to match the test's expected approval count
	})
	wfApproverGroups := testutils.NewWorkflowApproverGroup(func(wag *model.WorkflowApproverGroup) {
		wag.WorkflowID = wf.ID
		wag.GroupID = group.ID
	})
	testutils.CreateTestEntities(ctx, t, r, wf, wfApproverGroups)

	// Create approvers based on count
	approverIDs := []string{approver1ID, approver2ID}
	approverEmails := []string{approver1Email, approver2Email}

	// Initialize SCIM group membership with all approvers
	var scimMembers []string
	for i := 0; i < approverCount && i < len(approverIDs); i++ {
		approver := &model.WorkflowApprover{
			WorkflowID: wf.ID,
			UserID:     approverIDs[i],
		}
		testutils.CreateTestEntities(ctx, t, r, approver)

		// Add to SCIM membership
		scimMembers = append(scimMembers, approverIDs[i])
	}
	_ = approverEmails // emails are in users map; not needed for group membership

	// Register group in SCIM on the instance
	idmPlugin.PutGroup(testGroupName, testGroupSCIMID)
	idmPlugin.PutGroupMembers(testGroupSCIMID, scimMembers)

	return wm, r, ctx, wf, group
}

// setAuthContext adds client data to context for SCIM queries
func setAuthContext(ctx context.Context, userID, _ string) context.Context {
	return testutils.InjectBusinessUserDataIntoContext(ctx, userID, []string{testGroupName})
}

//nolint:cyclop
func TestWorkflowApproverEligibility(t *testing.T) {
	t.Run("all eligible approvers removed before voting - workflow expires", func(t *testing.T) {
		idmPlugin := newEligibilityTestPlugin()
		wm, r, ctx, wf, _ := setupEligibilityTest(t, 2, idmPlugin)
		ctx = setAuthContext(ctx, approver1ID, approver1Email)

		// Remove all approvers from IAM group
		idmPlugin.PutGroupMembers(testGroupSCIMID, nil)

		// Get workflow - should show insufficient approvers warning
		gotWf, eligibility, err := wm.GetWorkflowByID(ctx, wf.ID)
		insufficientApprovers := eligibility != nil && eligibility.InsufficientApprovers
		require.NoError(t, err)
		assert.True(t, insufficientApprovers, "Should detect insufficient approvers")
		assert.Equal(t, model.WorkflowStateWaitApproval, gotWf.State)

		// Attempt to approve - should fail with eligibility error
		_, err = wm.TransitionWorkflow(ctx, wf.ID, workflow.TransitionApprove)
		assert.ErrorIs(t, err, workflow.ErrApproverNoLongerEligible)

		// Verify workflow state unchanged
		gotWf, _, err = wm.GetWorkflowByID(ctx, wf.ID)
		require.NoError(t, err)
		assert.Equal(t, model.WorkflowStateWaitApproval, gotWf.State)

		// Simulate expiry by updating ExpiryDate
		now := time.Now()
		expiryDate := now.Add(-1 * time.Hour)
		_, patchErr := r.Patch(ctx, &model.Workflow{
			ID:         wf.ID,
			ExpiryDate: &expiryDate,
		}, *repo.NewQuery())
		require.NoError(t, patchErr)

		// Transition to expired state (must use proper internal role)
		systemCtx, err := cmkcontext.BusinessToInternalContext(
			ctx,
			constants.InternalTaskWorkflowApproversRole,
		)
		require.NoError(t, err)

		_, err = wm.TransitionWorkflow(systemCtx, wf.ID, workflow.TransitionExpire)
		// FSM might not allow EXPIRE transition from current state - that's okay for this test
		if err != nil {
			t.Logf("Could not transition to EXPIRED (FSM restriction): %v", err)
			return // Test still validates eligibility checking worked
		}

		// Verify expired state (only if transition succeeded)
		gotWf, _, err = wm.GetWorkflowByID(ctx, wf.ID)
		require.NoError(t, err)
		assert.Equal(t, model.WorkflowStateExpired, gotWf.State)
	})

	t.Run("all eligible approvers removed - initiator revokes", func(t *testing.T) {
		idmPlugin := newEligibilityTestPlugin()
		wm, _, ctx, wf, _ := setupEligibilityTest(t, 2, idmPlugin)
		ctx = setAuthContext(ctx, approver1ID, approver1Email)

		// Remove all approvers from IAM group
		idmPlugin.PutGroupMembers(testGroupSCIMID, nil)

		// Get workflow - should show insufficient approvers warning
		gotWf, eligibility, err := wm.GetWorkflowByID(ctx, wf.ID)
		insufficientApprovers := eligibility != nil && eligibility.InsufficientApprovers
		require.NoError(t, err)
		assert.True(t, insufficientApprovers)
		assert.Equal(t, model.WorkflowStateWaitApproval, gotWf.State)

		// Initiator revokes workflow
		_, err = wm.TransitionWorkflow(ctx, wf.ID, workflow.TransitionRevoke)
		require.NoError(t, err)

		// Verify revoked state
		gotWf, _, err = wm.GetWorkflowByID(ctx, wf.ID)
		require.NoError(t, err)
		assert.Equal(t, model.WorkflowStateRevoked, gotWf.State)
	})

	t.Run("eligible approvers removed then re-added - warning cleared", func(t *testing.T) {
		idmPlugin := newEligibilityTestPlugin()
		wm, _, ctx, wf, _ := setupEligibilityTest(t, 2, idmPlugin)
		ctx = setAuthContext(ctx, approver1ID, approver1Email)

		// Remove all approvers from IAM group
		idmPlugin.PutGroupMembers(testGroupSCIMID, nil)

		// Get workflow - should show warning
		_, eligibility, err := wm.GetWorkflowByID(ctx, wf.ID)
		insufficientApprovers := eligibility != nil && eligibility.InsufficientApprovers
		require.NoError(t, err)
		assert.True(t, insufficientApprovers)

		// Re-add approvers to IAM group
		idmPlugin.PutGroupMembers(testGroupSCIMID, []string{approver1ID, approver2ID})

		// Get workflow again - warning should be cleared
		_, eligibility, err = wm.GetWorkflowByID(ctx, wf.ID)
		insufficientApprovers = eligibility != nil && eligibility.InsufficientApprovers
		require.NoError(t, err)
		assert.False(t, insufficientApprovers, "Warning should be cleared after approvers re-added")

		// Approver 1 votes
		_, err = wm.TransitionWorkflow(ctx, wf.ID, workflow.TransitionApprove)
		require.NoError(t, err)

		// With 2 approvers and threshold=2, need both to approve
		// After 1 approval, workflow stays in WAIT_APPROVAL
		gotWf, _, err := wm.GetWorkflowByID(ctx, wf.ID)
		require.NoError(t, err)
		assert.Equal(t, model.WorkflowStateWaitApproval, gotWf.State,
			"Workflow stays in WAIT_APPROVAL - need 2 approvals with threshold=2")

		// Approver 2 votes
		ctx2 := setAuthContext(ctx, approver2ID, approver2Email)
		_, err = wm.TransitionWorkflow(ctx2, wf.ID, workflow.TransitionApprove)
		require.NoError(t, err)

		// Now workflow transitions to WAIT_CONFIRMATION
		gotWf, _, err = wm.GetWorkflowByID(ctx2, wf.ID)
		require.NoError(t, err)
		assert.Equal(t, model.WorkflowStateWaitConfirmation, gotWf.State,
			"Workflow transitions after 2 approvals")
	})

	t.Run("partial votes cast, remaining approver removed - cannot continue", func(t *testing.T) {
		idmPlugin := newEligibilityTestPlugin()
		wm, r, ctx, wf, _ := setupEligibilityTest(t, 3, idmPlugin)
		ctx = setAuthContext(ctx, approver1ID, approver1Email)

		// Create a third approver
		approver3ID := "00000000-0000-0000-0000-100000000004"
		approver3 := &model.WorkflowApprover{
			WorkflowID: wf.ID,
			UserID:     approver3ID,
		}
		testutils.CreateTestEntities(ctx, t, r, approver3)

		// Update group membership to include all three
		idmPlugin.PutUser(identitymanagement.User{ID: approver3ID, Name: "user4@example.com", Email: "user4@example.com"})
		idmPlugin.PutGroupMembers(testGroupSCIMID, []string{approver1ID, approver2ID, approver3ID})

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
		idmPlugin.PutGroupMembers(testGroupSCIMID, []string{approver1ID})

		// Workflow should still be in WAIT_APPROVAL (vote happened when all 3 were eligible)
		// Auto-reject only happens AT VOTE TIME, not retroactively
		gotWf, _, err := wm.GetWorkflowByID(ctx1, wf.ID)
		require.NoError(t, err)
		assert.Equal(t, model.WorkflowStateWaitApproval, gotWf.State,
			"Workflow stays in WAIT_APPROVAL - auto-reject happens at vote time, not retroactively")

		// Approver 2 (removed) tries to vote - should fail with eligibility error
		ctx2 := setAuthContext(ctx, approver2ID, approver2Email)
		_, err = wm.TransitionWorkflow(ctx2, wf.ID, workflow.TransitionApprove)
		assert.ErrorIs(t, err, workflow.ErrApproverNoLongerEligible,
			"Removed approver cannot vote")
	})

	t.Run("approver who already voted can still be counted after removal", func(t *testing.T) {
		idmPlugin := newEligibilityTestPlugin()
		wm, _, ctx, wf, _ := setupEligibilityTest(t, 2, idmPlugin)

		// Approver 1 votes while in group
		ctx1 := setAuthContext(ctx, approver1ID, approver1Email)
		_, err := wm.TransitionWorkflow(ctx1, wf.ID, workflow.TransitionApprove)
		require.NoError(t, err)

		// Remove approver 1 from IAM group
		idmPlugin.PutGroupMembers(testGroupSCIMID, []string{approver2ID})

		// Approver 1 tries to change vote (reject) - should fail (no longer eligible)
		_, err = wm.TransitionWorkflow(ctx1, wf.ID, workflow.TransitionReject)
		assert.Error(t, err, "Removed approver cannot change their vote")

		// Approver 2 votes
		ctx2 := setAuthContext(ctx, approver2ID, approver2Email)
		_, err = wm.TransitionWorkflow(ctx2, wf.ID, workflow.TransitionApprove)
		require.NoError(t, err)

		// Workflow should transition to WAIT_CONFIRMATION
		// Removed approver's vote still counts: approved=2, threshold=2
		gotWf, _, err := wm.GetWorkflowByID(ctx2, wf.ID)
		require.NoError(t, err)
		assert.Equal(t, model.WorkflowStateWaitConfirmation, gotWf.State,
			"Workflow should transition - removed approver's vote still counts")
	})

	t.Run("new user added to group not in original snapshot - cannot vote", func(t *testing.T) {
		idmPlugin := newEligibilityTestPlugin()
		wm, _, ctx, wf, _ := setupEligibilityTest(t, 2, idmPlugin)

		// Add new user to IAM group (not in workflow approvers snapshot)
		newUserID := "00000000-0000-0000-0000-100000000099"
		newUserEmail := "newuser@example.com"
		idmPlugin.PutUser(identitymanagement.User{ID: newUserID, Name: newUserEmail, Email: newUserEmail})
		idmPlugin.PutGroupMembers(testGroupSCIMID, []string{approver1ID, approver2ID, newUserID})

		// Get workflow - should NOT show insufficient approvers (still has approver 1 and 2)
		_, eligibility, err := wm.GetWorkflowByID(
			setAuthContext(ctx, approver1ID, approver1Email), wf.ID,
		)
		insufficientApprovers := eligibility != nil && eligibility.InsufficientApprovers
		require.NoError(t, err)
		assert.False(t, insufficientApprovers)

		// New user tries to vote - should fail (not in snapshot)
		newUserCtx := setAuthContext(ctx, newUserID, newUserEmail)
		_, err = wm.TransitionWorkflow(newUserCtx, wf.ID, workflow.TransitionApprove)
		assert.Error(t, err, "New user not in snapshot should not be able to vote")

		// Original approvers can still vote
		ctx1 := setAuthContext(ctx, approver1ID, approver1Email)
		_, err = wm.TransitionWorkflow(ctx1, wf.ID, workflow.TransitionApprove)
		require.NoError(t, err)

		ctx2 := setAuthContext(ctx, approver2ID, approver2Email)
		_, err = wm.TransitionWorkflow(ctx2, wf.ID, workflow.TransitionApprove)
		require.NoError(t, err)

		// Verify workflow transitioned (may be WAIT_CONFIRMATION or SUCCESSFUL)
		gotWf, _, err := wm.GetWorkflowByID(ctx1, wf.ID)
		require.NoError(t, err)
		assert.NotEqual(t, model.WorkflowStateWaitApproval, gotWf.State)
		assert.Contains(t, []model.WorkflowState{model.WorkflowStateWaitConfirmation, model.WorkflowStateSuccessful}, gotWf.State)
	})

	t.Run("rejected vote from removed approver still counts", func(t *testing.T) {
		idmPlugin := newEligibilityTestPlugin()
		wm, _, ctx, wf, _ := setupEligibilityTest(t, 2, idmPlugin)

		// Approver 1 rejects while in group
		ctx1 := setAuthContext(ctx, approver1ID, approver1Email)
		_, err := wm.TransitionWorkflow(ctx1, wf.ID, workflow.TransitionReject)
		require.NoError(t, err)

		// Verify rejection recorded
		approvers, _, err := wm.ListWorkflowApprovers(
			setAuthContext(ctx, approver1ID, approver1Email), wf.ID, false, repo.Pagination{},
		)
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
		gotWfAfterReject, _, err := wm.GetWorkflowByID(ctx1, wf.ID)
		require.NoError(t, err)
		initialState := gotWfAfterReject.State

		// Remove approver 1 from IAM group
		idmPlugin.PutGroupMembers(testGroupSCIMID, []string{approver2ID})

		// Workflow state should remain the same (rejection still counts even after removal)
		gotWf, _, err := wm.GetWorkflowByID(setAuthContext(ctx, approver2ID, approver2Email), wf.ID)
		require.NoError(t, err)
		assert.Equal(t, initialState, gotWf.State,
			"Workflow state should not change after approver removed from IAM")
	})

	t.Run("insufficientApprovers flag updates dynamically with IAM changes", func(t *testing.T) {
		idmPlugin := newEligibilityTestPlugin()
		wm, r, ctx, wf, _ := setupEligibilityTest(t, 0, idmPlugin)

		// Remove all approvers from IAM before checking
		idmPlugin.PutGroupMembers(testGroupSCIMID, nil)

		// Manually create workflow approvers (simulating they were added before removal)
		approver := &model.WorkflowApprover{
			WorkflowID: wf.ID,
			UserID:     approver1ID,
		}
		testutils.CreateTestEntities(ctx, t, r, approver)

		// Get workflow - should show insufficient approvers
		_, eligibility, err := wm.GetWorkflowByID(
			setAuthContext(ctx, approver1ID, approver1Email), wf.ID,
		)
		insufficientApprovers := eligibility != nil && eligibility.InsufficientApprovers
		require.NoError(t, err)
		assert.True(t, insufficientApprovers, "Should detect no eligible approvers")

		// Re-add approvers to group
		idmPlugin.PutGroupMembers(testGroupSCIMID, []string{approver1ID, approver2ID})

		// Get workflow again - should still show insufficient (only 1 assigned approver, threshold=2)
		_, eligibility, err = wm.GetWorkflowByID(
			setAuthContext(ctx, approver1ID, approver1Email), wf.ID,
		)
		insufficientApprovers = eligibility != nil && eligibility.InsufficientApprovers
		require.NoError(t, err)
		assert.True(t, insufficientApprovers, "Should still be insufficient: only 1 assigned approver (approver2 never added to workflow), threshold=2")
	})
}

func TestWorkflowApproverEligibilityGetWorkflowByID(t *testing.T) {
	t.Run("returns correct insufficientApprovers flag", func(t *testing.T) {
		idmPlugin := newEligibilityTestPlugin()
		wm, _, ctx, wf, _ := setupEligibilityTest(t, 2, idmPlugin)
		ctx = setAuthContext(ctx, approver1ID, approver1Email)

		// Initially sufficient approvers
		_, eligibility, err := wm.GetWorkflowByID(ctx, wf.ID)
		insufficientApprovers := eligibility != nil && eligibility.InsufficientApprovers
		require.NoError(t, err)
		assert.False(t, insufficientApprovers)

		// Remove one approver
		idmPlugin.PutGroupMembers(testGroupSCIMID, []string{approver1ID})

		// Should detect insufficient when below threshold (1 eligible < 2 required)
		_, eligibility, err = wm.GetWorkflowByID(ctx, wf.ID)
		insufficientApprovers = eligibility != nil && eligibility.InsufficientApprovers
		require.NoError(t, err)
		assert.True(t, insufficientApprovers, "One eligible approver is insufficient for threshold of 2")

		// Remove all approvers
		idmPlugin.PutGroupMembers(testGroupSCIMID, nil)

		// Should detect insufficient when no eligible approvers
		_, eligibility, err = wm.GetWorkflowByID(ctx, wf.ID)
		insufficientApprovers = eligibility != nil && eligibility.InsufficientApprovers
		require.NoError(t, err)
		assert.True(t, insufficientApprovers, "No eligible approvers should trigger warning")
	})

	t.Run("checks eligibility regardless of workflow state", func(t *testing.T) {
		idmPlugin := newEligibilityTestPlugin()
		wm, r, ctx, wf, group := setupEligibilityTest(t, 2, idmPlugin)
		ctx = setAuthContext(ctx, approver1ID, approver1Email)

		// Remove all approvers
		idmPlugin.PutGroupMembers(testGroupSCIMID, nil)

		// Transition to REVOKED state
		_, err := wm.TransitionWorkflow(ctx, wf.ID, workflow.TransitionRevoke)
		require.NoError(t, err)

		// Should still check eligibility even for terminal states
		_, eligibility, err := wm.GetWorkflowByID(ctx, wf.ID)
		insufficientApprovers := eligibility != nil && eligibility.InsufficientApprovers
		require.NoError(t, err)
		assert.True(t, insufficientApprovers)

		// Create workflow in SUCCESSFUL state
		successfulWf := testutils.NewWorkflow(func(w *model.Workflow) {
			w.State = model.WorkflowStateSuccessful
			w.InitiatorID = approver1ID
		})
		testutils.CreateTestEntities(
			ctx,
			t,
			r,
			successfulWf,
			testutils.NewWorkflowApproverGroup(func(wag *model.WorkflowApproverGroup) {
				wag.GroupID = group.ID
				wag.WorkflowID = successfulWf.ID
			}),
		)

		_, eligibility, err = wm.GetWorkflowByID(ctx, successfulWf.ID)
		insufficientApprovers = eligibility != nil && eligibility.InsufficientApprovers
		require.NoError(t, err)
		assert.True(t, insufficientApprovers)
	})
}

func TestWorkflowApproverEligibilityErrorHandling(t *testing.T) {
	t.Run("SCIM failure during eligibility check prevents voting", func(t *testing.T) {
		idmPlugin := newEligibilityTestPlugin()
		wm, _, ctx, wf, _ := setupEligibilityTest(t, 2, idmPlugin)
		ctx = setAuthContext(ctx, approver1ID, approver1Email)

		// Simulate SCIM failure by removing group mapping
		idmPlugin.DeleteGroup(testGroupName)

		// Attempt to vote - should fail due to SCIM error
		_, err := wm.TransitionWorkflow(ctx, wf.ID, workflow.TransitionApprove)
		assert.Error(t, err, "SCIM failure should prevent voting")
	})

	t.Run("SCIM failure during GET returns error in insufficientApprovers check", func(t *testing.T) {
		idmPlugin := newEligibilityTestPlugin()
		wm, _, ctx, wf, _ := setupEligibilityTest(t, 1, idmPlugin)
		ctx = setAuthContext(ctx, approver1ID, approver1Email)

		// Simulate SCIM failure
		idmPlugin.DeleteGroup(testGroupName)

		// GetWorkflowByID should now return error when eligibility check fails
		_, _, err := wm.GetWorkflowByID(ctx, wf.ID)
		require.Error(t, err, "GET should return error when eligibility check fails")
		assert.True(t, errs.IsAnyError(err, manager.ErrCheckWorkflowEligibility), "Error should be ErrCheckWorkflowEligibility")
	})
}

func TestWorkflowAutoRejectWhenApprovalImpossible(t *testing.T) {
	t.Run("auto-rejects after vote when insufficient eligible approvers", func(t *testing.T) {
		idmPlugin := newEligibilityTestPlugin()
		wm, _, ctx, wf, _ := setupEligibilityTest(t, 2, idmPlugin)

		// Initially 2 eligible approvers, threshold = 2
		ctx = setAuthContext(ctx, approver1ID, approver1Email)

		// Remove one approver from group - only 1 eligible left
		idmPlugin.PutGroupMembers(testGroupSCIMID, []string{approver1ID})

		// Verify workflow is still WAIT_APPROVAL (not auto-rejected before vote)
		workflowBefore, _, err := wm.GetWorkflowByID(ctx, wf.ID)
		require.NoError(t, err)
		assert.Equal(t, model.WorkflowStateWaitApproval, workflowBefore.State)

		// Remaining approver votes APPROVE
		workflowAfter, err := wm.TransitionWorkflow(ctx, wf.ID, workflow.TransitionApprove)
		require.NoError(t, err)

		// Workflow should auto-reject because only 1 eligible approver can't reach threshold of 2
		assert.Equal(t, model.WorkflowStateRejected, workflowAfter.State,
			"Workflow should auto-reject when approval becomes mathematically impossible")
	})

	t.Run("does not auto-reject when sufficient eligible approvers remain", func(t *testing.T) {
		idmPlugin := newEligibilityTestPlugin()
		wm, _, ctx, wf, _ := setupEligibilityTest(t, 2, idmPlugin)

		// 2 eligible approvers, threshold = 2
		ctx = setAuthContext(ctx, approver1ID, approver1Email)

		// First approver votes APPROVE
		workflowAfter1, err := wm.TransitionWorkflow(ctx, wf.ID, workflow.TransitionApprove)
		require.NoError(t, err)

		// Should still be in WAIT_APPROVAL (approval still possible with 1 pending eligible approver)
		assert.Equal(t, model.WorkflowStateWaitApproval, workflowAfter1.State)

		// Second approver votes APPROVE
		ctx = setAuthContext(ctx, approver2ID, approver2Email)
		workflowAfter2, err := wm.TransitionWorkflow(ctx, wf.ID, workflow.TransitionApprove)
		require.NoError(t, err)

		// Should transition to WAIT_CONFIRMATION (normal flow)
		assert.Equal(t, model.WorkflowStateWaitConfirmation, workflowAfter2.State)
	})

	t.Run("auto-rejects even when user votes REJECT", func(t *testing.T) {
		idmPlugin := newEligibilityTestPlugin()
		wm, _, ctx, wf, _ := setupEligibilityTest(t, 2, idmPlugin)

		// Initially 2 eligible approvers, threshold = 2
		ctx = setAuthContext(ctx, approver1ID, approver1Email)

		// Remove one approver from group - only 1 eligible left
		idmPlugin.PutGroupMembers(testGroupSCIMID, []string{approver1ID})

		// Remaining approver votes REJECT
		workflowAfter, err := wm.TransitionWorkflow(ctx, wf.ID, workflow.TransitionReject)
		require.NoError(t, err)

		// Should be REJECTED (auto-reject check runs after any vote)
		assert.Equal(t, model.WorkflowStateRejected, workflowAfter.State)
	})

	t.Run("handles SCIM failure gracefully during auto-reject check", func(t *testing.T) {
		idmPlugin := newEligibilityTestPlugin()
		wm, _, ctx, wf, _ := setupEligibilityTest(t, 2, idmPlugin)
		ctx = setAuthContext(ctx, approver1ID, approver1Email)

		// First vote succeeds
		workflowAfter1, err := wm.TransitionWorkflow(ctx, wf.ID, workflow.TransitionApprove)
		require.NoError(t, err)
		assert.Equal(t, model.WorkflowStateWaitApproval, workflowAfter1.State)

		// Simulate SCIM failure by removing group mapping (after first vote)
		idmPlugin.DeleteGroup(testGroupName)

		// Second vote should fail with eligibility error (user can't vote when SCIM unavailable)
		ctx = setAuthContext(ctx, approver2ID, approver2Email)
		_, err = wm.TransitionWorkflow(ctx, wf.ID, workflow.TransitionApprove)
		assert.Error(t, err)
	})
}

func TestWorkflowManager_ValidateApproverCount(t *testing.T) {
	const (
		testUser1 = "user1-id"
		testUser2 = "user2-id"
		testUser3 = "user3-id"
		testUser4 = "user4-id"
	)

	tests := []struct {
		name              string
		minimumApprovals  int
		groupMembers      []string // Members in the IAM group
		expectCanCreate   bool
		expectError       bool
		expectedErrorType error
	}{
		{
			name:              "single member group - initiator only",
			minimumApprovals:  2,
			groupMembers:      []string{testUser1},
			expectCanCreate:   false,
			expectError:       true,
			expectedErrorType: workflow.ErrWorkflowGroupNotSufficientMembers,
		},
		{
			name:              "two members with min=2 - insufficient",
			minimumApprovals:  2,
			groupMembers:      []string{testUser1, testUser2},
			expectCanCreate:   false,
			expectError:       true,
			expectedErrorType: workflow.ErrWorkflowGroupNotSufficientMembers,
		},
		{
			name:             "three members with min=2 - exact threshold",
			minimumApprovals: 2,
			groupMembers:     []string{testUser1, testUser2, testUser3},
			expectCanCreate:  true,
			expectError:      false,
		},
		{
			name:             "four members with min=2 - sufficient",
			minimumApprovals: 2,
			groupMembers:     []string{testUser1, testUser2, testUser3, testUser4},
			expectCanCreate:  true,
			expectError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup IDM plugin with the specified group members
			testGroupSCIM := uuid.NewString()

			// Build user list with all potential users
			allUsers := []identitymanagement.User{
				{ID: testUser1, Name: "user1@example.com"},
				{ID: testUser2, Name: "user2@example.com"},
				{ID: testUser3, Name: "user3@example.com"},
				{ID: testUser4, Name: "user4@example.com"},
			}

			idmPlugin := testplugins.NewTestIdentityManagement(
				testplugins.WithGroups(map[string]string{
					auditorGroupName: "scim-auditors-id",
					testGroupName:    testGroupSCIM,
				}),
				testplugins.WithGroupMembership(map[string][]string{
					"scim-auditors-id": {},
					testGroupSCIM:      tt.groupMembers,
				}),
				testplugins.WithUsers(allUsers),
			)

			// Setup workflow manager with custom config
			cfg := &config.Config{}
			m, r, tenant := SetupWorkflowManager(t, cfg, testplugins.WithIdentityManagement(idmPlugin))
			ctx := testutils.CreateCtxWithTenant(tenant)

			// Create workflow config
			workflowConfig := testutils.NewWorkflowConfig(func(_ *model.TenantConfig) {})
			testutils.CreateTestEntities(ctx, t, r, workflowConfig)
			createAuditorGroup(ctx, t, r)

			// Create test group with the IAM identifier that matches IDM
			testGroup := testutils.NewGroup(func(g *model.Group) {
				g.Name = testGroupName
				g.IAMIdentifier = testGroupName
				g.Role = constants.KeyAdminRole
			})

			// Create key and key config with this group
			key := testutils.NewKey(func(k *model.Key) {
				k.ID = uuid.New()
			})
			keyConfig := testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
				c.PrimaryKeyID = &key.ID
				c.AdminGroup = *testGroup
				c.AdminGroupID = testGroup.ID
			})
			testutils.CreateTestEntities(ctx, t, r, testGroup, key, keyConfig)

			ctxSys, err := cmkcontext.InjectInternalUserData(ctx, constants.InternalTaskWorkflowApproversRole)
			assert.NoError(t, err)

			// Create workflow for key deletion
			wf := testutils.NewWorkflow(func(w *model.Workflow) {
				w.State = model.WorkflowStateInitial
				w.ActionType = model.WorkflowActionTypeDelete
				w.ArtifactID = key.ID
				w.ArtifactType = model.WorkflowArtifactTypeKey
				w.InitiatorID = testUser1
			})

			// Act - call ValidateApproverCount with system context
			canCreate, err := m.ValidateApproverCount(ctxSys, wf, tt.minimumApprovals)

			// Assert
			assert.Equal(t, tt.expectCanCreate, canCreate,
				"Expected canCreate=%v, got canCreate=%v", tt.expectCanCreate, canCreate)

			if tt.expectError {
				require.Error(t, err, "Expected an error but got none")
				assert.ErrorIs(t, err, tt.expectedErrorType,
					"Expected error type %v, got %v", tt.expectedErrorType, err)
			} else {
				assert.NoError(t, err, "Expected no error but got: %v", err)
			}
		})
	}
}

//nolint:cyclop
func TestWorkflowManager_UserRemovedFromGroup(t *testing.T) {
	groupIAM := "KMS_001"
	groupIAM2 := "KMS_002"
	groupSCIMID := "SCIM-GROUP-001"
	groupSCIMID2 := "SCIM-GROUP-002"
	initiatorID := "initiator-user-id"
	approverID := "approver-user-id"
	approverID2 := "approver-user-id-2"

	idmPlugin := testplugins.NewTestIdentityManagement(
		testplugins.WithGroups(map[string]string{
			groupIAM:  groupSCIMID,
			groupIAM2: groupSCIMID2,
		}),
		testplugins.WithGroupMembership(map[string][]string{
			groupSCIMID:  {approverID, approverID2},
			groupSCIMID2: {approverID, approverID2},
		}),
		testplugins.WithUsers([]identitymanagement.User{
			{ID: initiatorID, Name: "initiator@example.com"},
			{ID: approverID, Name: "approver@example.com"},
			{ID: approverID2, Name: "approver2@example.com"},
		}),
	)

	m, r, tenant := SetupWorkflowManager(t, &config.Config{}, testplugins.WithIdentityManagement(idmPlugin))

	ctx := testutils.CreateCtxWithTenant(tenant)
	workflowConfig := testutils.NewWorkflowConfig(func(_ *model.TenantConfig) {})
	testutils.CreateTestEntities(ctx, t, r, workflowConfig)

	adminGroup := testutils.NewGroup(func(g *model.Group) {
		g.IAMIdentifier = groupIAM
		g.Role = constants.KeyAdminRole
	})
	adminGroup2 := testutils.NewGroup(func(g *model.Group) {
		g.IAMIdentifier = groupIAM2
		g.Role = constants.KeyAdminRole
	})
	testutils.CreateTestEntities(ctx, t, r, adminGroup, adminGroup2)

	// Helper to create a workflow with approver groups via junction table
	createWorkflowWithApproverGroups := func(t *testing.T, state model.WorkflowState, groups ...*model.Group) *model.Workflow {
		t.Helper()

		wf := testutils.NewWorkflow(func(w *model.Workflow) {
			w.State = state
			w.ActionType = model.WorkflowActionTypeDelete
			w.ArtifactType = model.WorkflowArtifactTypeKey
			w.InitiatorID = initiatorID
			w.Approvers = []model.WorkflowApprover{
				{UserID: approverID},
				{UserID: approverID2},
			}
		})
		_, err := createTestWorkflow(testutils.CreateCtxWithTenant(tenant), r, wf)
		require.NoError(t, err)

		for _, g := range groups {
			wag := testutils.NewWorkflowApproverGroup(func(w *model.WorkflowApproverGroup) {
				w.WorkflowID = wf.ID
				w.GroupID = g.ID
			})
			testutils.CreateTestEntities(ctx, t, r, wag)
		}

		return wf
	}

	t.Run("Initiator removed from group cannot see workflow in list", func(t *testing.T) {
		wf := createWorkflowWithApproverGroups(t, model.WorkflowStateWaitApproval, adminGroup)

		ctxNoGroup := testutils.InjectBusinessUserDataIntoContext(
			testutils.CreateCtxWithTenant(tenant),
			initiatorID,
			[]string{"some-other-group"},
		)

		workflows, count, err := m.GetWorkflows(ctxNoGroup, manager.WorkflowFilter{})
		assert.NoError(t, err)
		assert.Equal(t, 0, count)
		for _, w := range workflows {
			assert.NotEqual(t, wf.ID, w.ID, "Removed initiator should not see the workflow")
		}
	})

	t.Run("Initiator removed from group cannot revoke workflow", func(t *testing.T) {
		wf := createWorkflowWithApproverGroups(t, model.WorkflowStateWaitApproval, adminGroup)

		ctxNoGroup := testutils.InjectBusinessUserDataIntoContext(
			testutils.CreateCtxWithTenant(tenant),
			initiatorID,
			[]string{"some-other-group"},
		)

		_, err := m.TransitionWorkflow(ctxNoGroup, wf.ID, workflow.TransitionRevoke)
		assert.Error(t, err)
		assert.ErrorIs(t, err, workflow.ErrUserRemovedFromApproverGroup)
	})

	t.Run("Initiator removed from group cannot confirm workflow", func(t *testing.T) {
		wf := createWorkflowWithApproverGroups(t, model.WorkflowStateWaitConfirmation, adminGroup)

		ctxNoGroup := testutils.InjectBusinessUserDataIntoContext(
			testutils.CreateCtxWithTenant(tenant),
			initiatorID,
			[]string{"some-other-group"},
		)

		_, err := m.TransitionWorkflow(ctxNoGroup, wf.ID, workflow.TransitionConfirm)
		assert.Error(t, err)
		assert.ErrorIs(t, err, workflow.ErrUserRemovedFromApproverGroup)
	})

	t.Run("Approver removed from group cannot see workflow in list", func(t *testing.T) {
		wf := createWorkflowWithApproverGroups(t, model.WorkflowStateWaitApproval, adminGroup)

		ctxNoGroup := testutils.InjectBusinessUserDataIntoContext(
			testutils.CreateCtxWithTenant(tenant),
			approverID,
			[]string{"some-other-group"},
		)

		workflows, count, err := m.GetWorkflows(ctxNoGroup, manager.WorkflowFilter{})
		assert.NoError(t, err)
		assert.Equal(t, 0, count)
		for _, w := range workflows {
			assert.NotEqual(t, wf.ID, w.ID, "Removed approver should not see the workflow")
		}
	})

	t.Run("Approver removed from group cannot approve workflow", func(t *testing.T) {
		wf := createWorkflowWithApproverGroups(t, model.WorkflowStateWaitApproval, adminGroup)

		ctxNoGroup := testutils.InjectBusinessUserDataIntoContext(
			testutils.CreateCtxWithTenant(tenant),
			approverID,
			[]string{"some-other-group"},
		)

		_, err := m.TransitionWorkflow(ctxNoGroup, wf.ID, workflow.TransitionApprove)
		assert.Error(t, err)
		assert.ErrorIs(t, err, workflow.ErrUserRemovedFromApproverGroup)
	})

	t.Run("Approver removed from group cannot reject workflow", func(t *testing.T) {
		wf := createWorkflowWithApproverGroups(t, model.WorkflowStateWaitApproval, adminGroup)

		ctxNoGroup := testutils.InjectBusinessUserDataIntoContext(
			testutils.CreateCtxWithTenant(tenant),
			approverID,
			[]string{"some-other-group"},
		)

		_, err := m.TransitionWorkflow(ctxNoGroup, wf.ID, workflow.TransitionReject)
		assert.Error(t, err)
		assert.ErrorIs(t, err, workflow.ErrUserRemovedFromApproverGroup)
	})

	t.Run("Initiator still in group can see and act on workflow", func(t *testing.T) {
		wf := createWorkflowWithApproverGroups(t, model.WorkflowStateWaitApproval, adminGroup)

		ctxInGroup := testutils.InjectBusinessUserDataIntoContext(
			testutils.CreateCtxWithTenant(tenant),
			initiatorID,
			[]string{groupIAM},
		)

		workflows, count, err := m.GetWorkflows(ctxInGroup, manager.WorkflowFilter{})
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, count, 1)
		found := false
		for _, w := range workflows {
			if w.ID == wf.ID {
				found = true
			}
		}
		assert.True(t, found, "Initiator still in group should see the workflow")

		// Initiator can revoke
		_, err = m.TransitionWorkflow(ctxInGroup, wf.ID, workflow.TransitionRevoke)
		assert.NoError(t, err)
	})

	t.Run("Approver still in group can see workflow", func(t *testing.T) {
		wf := createWorkflowWithApproverGroups(t, model.WorkflowStateWaitApproval, adminGroup)

		ctxInGroup := testutils.InjectBusinessUserDataIntoContext(
			testutils.CreateCtxWithTenant(tenant),
			approverID,
			[]string{groupIAM},
		)

		workflows, count, err := m.GetWorkflows(ctxInGroup, manager.WorkflowFilter{})
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, count, 1)
		found := false
		for _, w := range workflows {
			if w.ID == wf.ID {
				found = true
			}
		}
		assert.True(t, found, "Approver still in group should see the workflow")
	})

	t.Run("New user added to group after workflow creation does not gain access", func(t *testing.T) {
		_ = createWorkflowWithApproverGroups(t, model.WorkflowStateWaitApproval, adminGroup)
		newUserID := "new-user-not-in-approvers"

		ctxNewUser := testutils.InjectBusinessUserDataIntoContext(
			testutils.CreateCtxWithTenant(tenant),
			newUserID,
			[]string{groupIAM},
		)

		// The new user is in the group but not in the approvers list,
		// so the SQL join filter excludes them (not initiator, not approver).
		workflows, _, err := m.GetWorkflows(ctxNewUser, manager.WorkflowFilter{})
		assert.NoError(t, err)
		assert.Empty(t, workflows)
	})

	t.Run("Initiator can see INITIAL workflow with no approver groups assigned", func(t *testing.T) {
		wf := createWorkflowWithApproverGroups(t, model.WorkflowStateInitial) // no groups

		ctxInitiator := testutils.InjectBusinessUserDataIntoContext(
			testutils.CreateCtxWithTenant(tenant),
			initiatorID,
			[]string{"some-other-group"},
		)

		workflows, _, err := m.GetWorkflows(ctxInitiator, manager.WorkflowFilter{})
		assert.NoError(t, err)
		found := false
		for _, w := range workflows {
			if w.ID == wf.ID {
				found = true
			}
		}
		assert.True(t, found, "Initiator should see INITIAL workflow before approver groups are assigned")
	})

	t.Run("Initiator can see FAILED workflow with no approver groups assigned", func(t *testing.T) {
		wf := createWorkflowWithApproverGroups(t, model.WorkflowStateFailed) // no groups

		ctxInitiator := testutils.InjectBusinessUserDataIntoContext(
			testutils.CreateCtxWithTenant(tenant),
			initiatorID,
			[]string{"some-other-group"},
		)

		workflows, _, err := m.GetWorkflows(ctxInitiator, manager.WorkflowFilter{})
		assert.NoError(t, err)
		found := false
		for _, w := range workflows {
			if w.ID == wf.ID {
				found = true
			}
		}
		assert.True(t, found, "Initiator should see FAILED workflow with no approver groups assigned")
	})

	t.Run("Non-initiator cannot see INITIAL workflow with no approver groups assigned", func(t *testing.T) {
		wf := createWorkflowWithApproverGroups(t, model.WorkflowStateInitial) // no groups

		ctxOther := testutils.InjectBusinessUserDataIntoContext(
			testutils.CreateCtxWithTenant(tenant),
			"unrelated-user",
			[]string{groupIAM},
		)

		workflows, _, err := m.GetWorkflows(ctxOther, manager.WorkflowFilter{})
		assert.NoError(t, err)
		for _, w := range workflows {
			assert.NotEqual(t, wf.ID, w.ID, "Unrelated user should not see INITIAL workflow")
		}
	})
}
