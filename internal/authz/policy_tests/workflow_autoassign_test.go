package authz_policy_test

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/async/tasks"
	"github.com/openkcm/cmk/internal/auditor"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
	asyncUtils "github.com/openkcm/cmk/utils/async"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// TestWorkflowAutoAssign_AuthzPolicy verifies that the InternalTaskWorkflowApproversRole
// policy grants the repo access that WorkflowManager.AutoAssignApprovers requires,
// without the manager being mocked out.
//
// A workflow is seeded so ProcessTask finds it via First on Workflow (policy: First).
// AutoAssignApprovers then calls getKeyConfigurationsFromArtifact which needs First on
// Key and First on KeyConfiguration (both in policy). Execution stops at
// getApproversAndGroupsFromKeyConfigs which calls ExtractBusinessUserDataAuthContext —
// an expected non-authz failure since WorkflowProcessor injects internal context but
// the method requires business user auth context carried in the task payload.
// The test asserts ProcessTask returns an error (propagated, not swallowed) and that
// the error is NOT an authz error — confirming the policy covered all repo access up
// to that point.
//
// Note: addApproversAndGroupAssociations calls Set(WorkflowApproverGroup) which checks
// Delete+Create on "workflows" (WorkflowApproverGroup.TableResourceType returns Workflow).
// HandleTerminalWorkflow calls Patch on System to clear UnderWorkflow, requiring Update
// on "systems". All three permissions are granted to InternalTaskWorkflowApproversRole.
func TestWorkflowAutoAssign_AuthzPolicy(t *testing.T) {
	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
	})
	tenant := tenants[0]
	ctx := cmkcontext.CreateTenantContext(t.Context(), tenant)
	ctx, err := cmkcontext.InjectInternalUserData(ctx, constants.InternalTaskWorkflowApproversRole)
	assert.NoError(t, err)

	r := sql.NewRepository(db)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(), r, &config.Config{})
	authzRepo := authz_repo.NewAuthzRepo(r, authzRepoLoader)

	ps := testutils.NewTestPlugins(
		testplugins.WithCertificateIssuer(testplugins.NewTestCertificateIssuer()),
		testplugins.WithIdentityManagement(testplugins.NewTestIdentityManagement()),
	)
	cfg := &config.Config{
		Database: dbCfg,
	}

	eventFactory, err := eventprocessor.NewEventFactory(t.Context(), cfg, r)
	assert.NoError(t, err)

	cmkAuditor := auditor.New(t.Context(), cfg)
	userManager := manager.NewUserManager(authzRepo, cmkAuditor)
	certManager := manager.NewCertificateManager(t.Context(), authzRepo, ps, cfg)
	resourceLabelManager := manager.NewResourceLabelManager(authzRepo)
	tagManager := manager.NewTagManager(resourceLabelManager)
	tenantConfigManager := manager.NewTenantConfigManager(authzRepo, ps, cfg)
	keyConfigManager := manager.NewKeyConfigManager(authzRepo, certManager, userManager, tagManager, cmkAuditor, eventFactory, cfg)
	keyManager := manager.NewKeyManager(
		authzRepo,
		ps,
		tenantConfigManager,
		keyConfigManager,
		userManager,
		certManager,
		eventFactory,
		cmkAuditor,
	)
	groupManager := manager.NewGroupManager(authzRepo, ps, userManager)
	wfManager := manager.NewWorkflowManager(
		authzRepo,
		ps,
		keyManager,
		keyConfigManager,
		nil, // systemManager
		groupManager,
		userManager,
		nil, // asyncClient
		tenantConfigManager,
		cfg,
	)

	// Seed a key configuration and a key referencing it, plus a workflow with that key
	// as the artifact so AutoAssignApprovers can load the full dependency chain.
	group := testutils.NewGroup(func(_ *model.Group) {})
	keyConfig := testutils.NewKeyConfig(func(kc *model.KeyConfiguration) {
		kc.AdminGroup = *group
		kc.AdminGroupID = group.ID
	})
	key := testutils.NewKey(func(k *model.Key) {
		k.KeyConfigurationID = keyConfig.ID
	})
	workflow := testutils.NewWorkflow(func(wf *model.Workflow) {
		wf.ArtifactID = key.ID
	})
	testutils.CreateTestEntities(ctx, t, r, group, keyConfig, key, workflow)

	// Build a task payload carrying the workflow ID, as WorkflowProcessor expects.
	payload := asyncUtils.NewTaskPayload(ctx, []byte(workflow.ID.String()))
	payloadBytes, err := payload.ToBytes()
	assert.NoError(t, err)

	processor := tasks.NewWorkflowProcessor(wfManager, authzRepo)
	task := asynq.NewTask(config.TypeWorkflowAutoAssign, payloadBytes)

	t.Run("InternalTaskWorkflowApproversRole allows First on Workflow, Key, KeyConfiguration", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		err := processor.ProcessTask(ctx, task)

		// ProcessTask propagates errors for retry. We expect an error here because
		// AutoAssignApprovers calls into group resolution which requires business user
		// context that isn't available in a pure internal context. This is expected and
		// NOT an authz error — it confirms all repo access up to that point was permitted
		// by the policy.
		assert.Error(t, err)
		assert.NotContains(t, buf.String(), `"allowed":false`,
			"authz denial in log — policy is missing a required permission: %s", buf.String())
		assert.NotContains(t, strings.ToLower(err.Error()), "unauthorized",
			"authz error returned — policy is missing a required permission")
	})
}
