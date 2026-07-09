package dpp_test

import (
	"context"
	"testing"

	"github.com/openkcm/common-sdk/pkg/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/auditor"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/identitymanagement"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/testplugins"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// TestWorkflowApproverIsolation verifies that when a workflow is created,
// only users who are members of the key configuration's admin group are
// offered as approvers. Users in other groups must not appear, regardless
// of how many groups exist in the tenant.
//
// This is a DPP isolation control: the approver list must be scoped to the
// relevant group, not the full user population of the tenant.
func TestWorkflowApproverIsolation(t *testing.T) {
	const (
		adminGroupName   = "key-admins"
		adminGroupSCIM   = "scim-key-admins-id"
		otherGroupName   = "other-group"
		otherGroupSCIM   = "scim-other-group-id"
		auditorGroup     = "auditors"
		auditorGroupSCIM = "scim-auditors-id"

		adminUser1 = "admin-user-1"
		adminUser2 = "admin-user-2"
		otherUser  = "other-user-1"
	)

	idmPlugin := testplugins.NewTestIdentityManagement(
		testplugins.WithGroups(map[string]string{
			adminGroupName: adminGroupSCIM,
			otherGroupName: otherGroupSCIM,
			auditorGroup:   auditorGroupSCIM,
		}),
		testplugins.WithGroupMembership(map[string][]string{
			adminGroupSCIM:   {adminUser1, adminUser2},
			otherGroupSCIM:   {otherUser},
			auditorGroupSCIM: {},
		}),
		testplugins.WithUsers([]identitymanagement.User{
			{ID: adminUser1, Name: "admin1@example.com", Email: "admin1@example.com"},
			{ID: adminUser2, Name: "admin2@example.com", Email: "admin2@example.com"},
			{ID: otherUser, Name: "other@example.com", Email: "other@example.com"},
		}),
	)

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	tenant := tenants[0]
	r := sql.NewRepository(db)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(), r, &config.Config{})
	authzRepoInst := authz_repo.NewAuthzRepo(r, authzRepoLoader)
	svcRegistry := testutils.NewTestPlugins(testplugins.WithIdentityManagement(idmPlugin))
	cfg := &config.Config{}

	cmkAuditor := auditor.New(t.Context(), cfg)
	certManager := manager.NewCertificateManager(t.Context(), r, svcRegistry, cfg)
	tenantConfigManager := manager.NewTenantConfigManager(r, svcRegistry, cfg, certManager)
	userManager := manager.NewUserManager(authzRepoInst, cmkAuditor)
	tagManager := manager.NewTagManager(r)
	keyConfigManager := manager.NewKeyConfigManager(r, certManager, userManager, tagManager, cmkAuditor, nil, cfg)
	groupManager := manager.NewGroupManager(r, svcRegistry, userManager)
	clientsFactory, err := clients.NewFactory(cfg.Services)
	require.NoError(t, err)
	systemManager := manager.NewSystemManager(t.Context(), r, nil, clientsFactory, nil, svcRegistry, cfg, keyConfigManager, userManager)
	keym := manager.NewKeyManager(r, svcRegistry, tenantConfigManager, keyConfigManager, userManager, certManager, nil, cmkAuditor, nil)
	m := manager.NewWorkflowManager(r, svcRegistry, keym, keyConfigManager, systemManager, groupManager, userManager, nil, tenantConfigManager, cfg)

	ctx := testutils.CreateCtxWithTenant(tenant)
	ctxSys, err := cmkcontext.BusinessToInternalContext(ctx, constants.InternalTaskWorkflowApproversRole)
	require.NoError(t, err)

	// Enable workflow feature
	testutils.NewWorkflowConfig(ctx, t, r, func(_ *model.WorkflowConfig) {})

	// Seed all groups in DB
	adminGroupModel := testutils.NewGroup(func(g *model.Group) {
		g.Name = adminGroupName
		g.IAMIdentifier = adminGroupName
		g.Role = constants.KeyAdminRole
	})
	otherGroupModel := testutils.NewGroup(func(g *model.Group) {
		g.Name = otherGroupName
		g.IAMIdentifier = otherGroupName
		g.Role = constants.KeyAdminRole
	})
	auditorGroupModel := testutils.NewGroup(func(g *model.Group) {
		g.Name = auditorGroup
		g.IAMIdentifier = auditorGroup
		g.Role = constants.TenantAuditorRole
	})
	testutils.CreateTestEntities(ctxSys, t, r, adminGroupModel, otherGroupModel, auditorGroupModel)

	// Key config is administered by adminGroup only
	key := testutils.NewKey(func(_ *model.Key) {})
	keyConfig := testutils.NewKeyConfig(func(c *model.KeyConfiguration) {
		c.PrimaryKeyID = &key.ID
		c.AdminGroup = *adminGroupModel
		c.AdminGroupID = adminGroupModel.ID
	})
	testutils.CreateTestEntities(ctxSys, t, r, key, keyConfig)

	// Create a workflow for that key
	wf := testutils.NewWorkflow(func(w *model.Workflow) {
		w.State = model.WorkflowStateInitial
		w.ActionType = model.WorkflowActionTypeDelete
		w.ArtifactType = model.WorkflowArtifactTypeKey
		w.ArtifactID = key.ID
		w.Approvers = nil
	})
	err = r.Create(ctxSys, wf)
	require.NoError(t, err)

	// AutoAssignApprovers is normally async; call it directly here.
	// The context must carry business user data for the auditor group check.
	ctxWithUser := context.WithValue(ctx, constants.BusinessUserData, &auth.ClientData{
		Identifier: "testuser",
		Groups:     []string{auditorGroup},
	})
	_, err = m.AutoAssignApprovers(ctxWithUser, wf.ID)
	require.NoError(t, err)

	approvers, _, err := m.ListWorkflowApprovers(ctxSys, wf.ID, false, repo.Pagination{})
	require.NoError(t, err)

	approverIDs := make([]string, 0, len(approvers))
	for _, a := range approvers {
		approverIDs = append(approverIDs, a.UserID)
	}

	// Only admin group members should be approvers
	assert.ElementsMatch(t, []string{adminUser1, adminUser2}, approverIDs,
		"approvers must be scoped to the key config's admin group")

	// Users from other groups must not appear
	assert.NotContains(t, approverIDs, otherUser,
		"user from unrelated group must not appear as an approver")
}
