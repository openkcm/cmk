package manager_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/grpc/catalog"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/utils/ptr"
)

func SetupGroupManager(t *testing.T) (*manager.GroupManager, *multitenancy.DB, string) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(
		t, testutils.TestDBConfig{
			CreateDatabase: true,
		},
	)

	ctlg, err := catalog.New(
		t.Context(), &config.Config{
			Plugins: testutils.SetupMockPlugins(testutils.IdentityPlugin),
		},
	)
	assert.NoError(t, err)

	dbRepository := sql.NewRepository(db)

	m := manager.NewGroupManager(dbRepository, ctlg, manager.NewUserManager(dbRepository, auditor.New(t.Context(), &config.Config{})))

	return m, db, tenants[0]
}

func TestGetGroups(t *testing.T) {
	manager, _, tenant := SetupGroupManager(t)

	t.Run("Should get groups", func(t *testing.T) {
		group := testutils.NewGroup(
			func(g *model.Group) {
				g.IAMIdentifier = "test-group1"
			},
		)
		ctx := testutils.CreateCtxWithTenant(tenant)
		ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"test-group1"})
		_, err := manager.CreateGroup(ctx, group)
		assert.NoError(t, err)

		groups, total, err := manager.GetGroups(
			ctx,
			constants.DefaultSkip,
			constants.DefaultTop,
		)
		assert.NoError(t, err)
		assert.Equal(t, groups[0].ID, group.ID)
		assert.Equal(t, groups[0].IAMIdentifier, group.IAMIdentifier)
		assert.Equal(t, 1, total)
	})
}

func TestCreateGroup(t *testing.T) {
	groupManager, db, tenant := SetupGroupManager(t)
	t.Run(
		"Should create group", func(t *testing.T) {
			expected := testutils.NewGroup(
				func(g *model.Group) {
					g.IAMIdentifier = "test-group-create"
				},
			)
			ctx := testutils.CreateCtxWithTenant(tenant)
			ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"test-group-create"})
			res, err := groupManager.CreateGroup(
				ctx,
				expected,
			)
			assert.NoError(t, err)
			assert.Equal(t, expected.Name, res.Name)
		},
	)

	t.Run(
		"Should error on create group with duplicated name", func(t *testing.T) {
			ctx := testutils.CreateCtxWithTenant(tenant)
			ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"duplicated-iam"})
			_, err := groupManager.CreateGroup(
				ctx,
				testutils.NewGroup(
					func(g *model.Group) {
						g.Name = "duplicated-name"
						g.IAMIdentifier = "duplicated-iam"
					},
				),
			)
			assert.NoError(t, err)
			_, err = groupManager.CreateGroup(
				ctx,
				testutils.NewGroup(
					func(g *model.Group) {
						g.Name = "duplicated-name"
						g.IAMIdentifier = "duplicated-iam"
					},
				),
			)
			assert.ErrorIs(t, err, repo.ErrUniqueConstraint)
		},
	)

	t.Run(
		"Should error on create group with invalid role", func(t *testing.T) {
			ctx := testutils.CreateCtxWithTenant(tenant)
			ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"test-group-role"})
			_, err := groupManager.CreateGroup(
				ctx,
				testutils.NewGroup(
					func(g *model.Group) {
						g.Role = "invalid-role"
						g.IAMIdentifier = "test-group-role"
					},
				),
			)
			assert.ErrorIs(t, err, manager.ErrGroupRole)
		},
	)

	t.Run(
		"Should error on create group", func(t *testing.T) {
			forced := testutils.NewDBErrorForced(db, ErrForced)

			forced.Register()
			defer forced.Unregister()

			ctx := testutils.CreateCtxWithTenant(tenant)
			ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"test-group-error"})
			res, err := groupManager.CreateGroup(
				ctx,
				testutils.NewGroup(
					func(g *model.Group) {
						g.IAMIdentifier = "test-group-error"
					},
				),
			)
			assert.Error(t, err)
			assert.Nil(t, res)
		},
	)
}

func TestDeleteGroupByID(t *testing.T) {
	groupManager, db, tenant := SetupGroupManager(t)
	t.Run(
		"Should delete group", func(t *testing.T) {
			group := testutils.NewGroup(
				func(g *model.Group) {
					g.IAMIdentifier = "test-group-delete"
				},
			)
			ctx := testutils.CreateCtxWithTenant(tenant)
			ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"test-group-delete"})
			_, err := groupManager.CreateGroup(
				ctx,
				group,
			)
			assert.NoError(t, err)

			err = groupManager.DeleteGroupByID(ctx, group.ID)
			assert.NoError(t, err)
		},
	)

	t.Run(
		"Should error on invalid non existing group id", func(t *testing.T) {
			ctx := testutils.CreateCtxWithTenant(tenant)
			ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"test-group"})
			err := groupManager.DeleteGroupByID(ctx, uuid.New())
			assert.Error(t, err)
		},
	)

	t.Run(
		"Should error delete group if mandatory group", func(t *testing.T) {
			group := testutils.NewGroup(
				func(g *model.Group) {
					g.Name = constants.TenantAuditorGroup
					g.IAMIdentifier = "test-group-auditor"
				},
			)
			ctx := testutils.CreateCtxWithTenant(tenant)
			ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"test-group-auditor"})
			_, err := groupManager.CreateGroup(
				ctx,
				group,
			)
			assert.NoError(t, err)

			err = groupManager.DeleteGroupByID(ctx, group.ID)
			assert.ErrorIs(t, err, manager.ErrInvalidGroupDelete)
		},
	)

	t.Run(
		"Should error delete group if connected to keyConfig", func(t *testing.T) {
			group := testutils.NewGroup(
				func(g *model.Group) {
					g.IAMIdentifier = "test-group-keyconfig"
				},
			)
			ctx := testutils.CreateCtxWithTenant(tenant)
			ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"test-group-keyconfig"})
			_, err := groupManager.CreateGroup(
				ctx,
				group,
			)
			assert.NoError(t, err)

			r := sql.NewRepository(db)
			keyConfig := testutils.NewKeyConfig(
				func(kc *model.KeyConfiguration) {
					kc.AdminGroup.ID = group.ID
					kc.AdminGroup.IAMIdentifier = group.IAMIdentifier
				},
			)
			err = r.Create(ctx, keyConfig)
			assert.NoError(t, err)

			err = groupManager.DeleteGroupByID(ctx, group.ID)
			assert.ErrorIs(t, err, manager.ErrInvalidGroupDelete)
		},
	)

	t.Run(
		"Should error on delete", func(t *testing.T) {
			forced := testutils.NewDBErrorForced(db, ErrForced)

			forced.WithDelete().Register()
			defer forced.Unregister()

			group := testutils.NewGroup(
				func(g *model.Group) {
					g.IAMIdentifier = "test-group-error-delete"
				},
			)

			ctx := testutils.CreateCtxWithTenant(tenant)
			ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"test-group-error-delete"})
			_, err := groupManager.CreateGroup(
				ctx,
				group,
			)
			assert.NoError(t, err)

			err = groupManager.DeleteGroupByID(ctx, group.ID)
			assert.Error(t, err)
		},
	)
}

func TestGetGroupByID(t *testing.T) {
	groupManager, _, tenant := SetupGroupManager(t)
	t.Run(
		"Should get group", func(t *testing.T) {
			ctx := testutils.CreateCtxWithTenant(tenant)
			ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"test-group-getbyid"})
			expected, err := groupManager.CreateGroup(
				ctx,
				testutils.NewGroup(
					func(g *model.Group) {
						g.IAMIdentifier = "test-group-getbyid"
					},
				),
			)
			assert.NoError(t, err)

			group, err := groupManager.GetGroupByID(ctx, expected.ID)
			assert.Equal(t, expected, group)
			assert.NoError(t, err)
		},
	)

	t.Run(
		"Should fail on get group", func(t *testing.T) {
			ctx := testutils.CreateCtxWithTenant(tenant)
			ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"test-group"})
			group, err := groupManager.GetGroupByID(ctx, uuid.New())
			assert.Nil(t, group)
			assert.Error(t, err)
		},
	)
}

func TestUpdateGroup(t *testing.T) {
	groupManager, db, tenant := SetupGroupManager(t)
	ctx := testutils.CreateCtxWithTenant(tenant)
	ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"test-group-admin"})
	reservedGroup, err := groupManager.CreateGroup(
		ctx,
		testutils.NewGroup(
			func(g *model.Group) {
				g.Name = constants.TenantAdminGroup
				g.IAMIdentifier = "test-group-admin"
			},
		),
	)
	assert.NoError(t, err)

	t.Run(
		"Should rename group", func(t *testing.T) {
			ctx := testutils.CreateCtxWithTenant(tenant)
			ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"test-group-update"})
			expected, err := groupManager.CreateGroup(
				ctx,
				testutils.NewGroup(func(g *model.Group) {
					g.IAMIdentifier = "test-group-update"
				}),
			)
			assert.NoError(t, err)

			patchGroup := cmkapi.GroupPatch{Name: ptr.PointTo("test-updated")}
			group, err := groupManager.UpdateGroup(ctx, expected.ID, patchGroup)
			expected.Name = *patchGroup.Name
			assert.Equal(t, expected, group)
			assert.NoError(t, err)
		},
	)

	t.Run("Should change IAMIdentifier", func(t *testing.T) {
		ctx := testutils.CreateCtxWithTenant(tenant)
		ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"iam-identifier"})
		expected, err := groupManager.CreateGroup(
			ctx,
			testutils.NewGroup(func(g *model.Group) {
				g.IAMIdentifier = "iam-identifier"
			}),
		)
		assert.NoError(t, err)

		patchGroup := cmkapi.GroupPatch{IAMIdentifier: ptr.PointTo("new-identifier")}
		group, err := groupManager.UpdateGroup(ctx, expected.ID, patchGroup)
		expected.IAMIdentifier = *patchGroup.IAMIdentifier
		assert.Equal(t, expected, group)
		assert.NoError(t, err)
	})

	t.Run("Should error on change IAMIdentifier on reserved group", func(t *testing.T) {
		group, err := groupManager.UpdateGroup(
			ctx,
			reservedGroup.ID,
			cmkapi.GroupPatch{IAMIdentifier: ptr.PointTo("test")},
		)
		assert.Nil(t, group)
		assert.Error(t, err)
		assert.ErrorIs(t, err, manager.ErrInvalidGroupUpdate)
	})

	t.Run("Should error on change IAMIdentifier with invalid values", func(t *testing.T) {
		ctx := testutils.CreateCtxWithTenant(tenant)
		ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"iam-identifier"})
		expected, err := groupManager.CreateGroup(
			ctx,
			testutils.NewGroup(func(g *model.Group) {
				g.IAMIdentifier = "iam-identifier"
			}),
		)
		assert.NoError(t, err)

		group, err := groupManager.UpdateGroup(
			ctx,
			expected.ID,
			cmkapi.GroupPatch{IAMIdentifier: ptr.PointTo("!test!")},
		)
		assert.Nil(t, group)
		assert.Error(t, err)
		assert.ErrorIs(t, err, model.ErrInvalidIAMIdentifier)
	})

	t.Run(
		"Should error on rename group if name is empty", func(t *testing.T) {
			ctx := testutils.CreateCtxWithTenant(tenant)
			ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"test-group-empty"})
			expected, err := groupManager.CreateGroup(
				ctx,
				testutils.NewGroup(
					func(g *model.Group) {
						g.IAMIdentifier = "test-group-empty"
					},
				),
			)
			assert.NoError(t, err)

			patchGroup := cmkapi.GroupPatch{Name: ptr.PointTo("")}
			group, err := groupManager.UpdateGroup(ctx, expected.ID, patchGroup)
			assert.Nil(t, group)
			assert.Error(t, err)
			assert.ErrorIs(t, err, manager.ErrNameCannotBeEmpty)
		},
	)

	t.Run(
		"Should error on rename if group imanagerandatory", func(t *testing.T) {
			ctx := testutils.CreateCtxWithTenant(tenant)
			ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"test-group-admin"})
			group, err := groupManager.UpdateGroup(
				ctx,
				reservedGroup.ID,
				cmkapi.GroupPatch{Name: ptr.PointTo("test")},
			)
			assert.Nil(t, group)
			assert.Error(t, err)
			assert.ErrorIs(t, err, manager.ErrInvalidGroupUpdate)
		},
	)

	t.Run(
		"Should error on rename if new group name is reserved name", func(t *testing.T) {
			ctx := testutils.CreateCtxWithTenant(tenant)
			ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"test-group-admin"})
			group, err := groupManager.UpdateGroup(
				ctx,
				reservedGroup.ID,
				cmkapi.GroupPatch{Name: ptr.PointTo(constants.TenantAdminGroup)},
			)
			assert.Nil(t, group)
			assert.Error(t, err)
			assert.ErrorIs(t, err, manager.ErrInvalidGroupUpdate)
		},
	)

	t.Run(
		"Should error on rename if does not exist", func(t *testing.T) {
			ctx := testutils.CreateCtxWithTenant(tenant)
			ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"test-group"})
			group, err := groupManager.UpdateGroup(
				ctx,
				uuid.New(),
				cmkapi.GroupPatch{Name: ptr.PointTo("test")},
			)
			assert.Nil(t, group)
			assert.Error(t, err)
		},
	)

	t.Run(
		"Should error on rename with DB error", func(t *testing.T) {
			forced := testutils.NewDBErrorForced(db, ErrForced)

			forced.WithUpdate().Register()
			defer forced.Unregister()

			ctx := testutils.CreateCtxWithTenant(tenant)
			ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"test-group-dberror"})
			expected, err := groupManager.CreateGroup(
				ctx,
				testutils.NewGroup(
					func(g *model.Group) {
						g.IAMIdentifier = "test-group-dberror"
					},
				),
			)
			assert.NoError(t, err)

			group, err := groupManager.UpdateGroup(
				ctx,
				expected.ID,
				cmkapi.GroupPatch{Name: ptr.PointTo("test")},
			)
			assert.Nil(t, group)
			assert.Error(t, err)
		},
	)
}

func TestCheckGroupIAMExistence(t *testing.T) {
	m, _, tenant := SetupGroupManager(t)
	ctx := testutils.CreateCtxWithTenant(tenant)
	ctx = testutils.InjectClientDataIntoContext(ctx, "test-user", []string{"KMS_001", "KMS_002", "KMS_003"})

	t.Run(
		"Should confirm group IAM existence", func(t *testing.T) {
			result, err := m.CheckIAMExistenceOfGroups(
				ctx,
				[]string{"KMS_001", "KMS_002"},
			)
			assert.NoError(t, err)
			assert.Equal(
				t, []manager.GroupIAMExistence{
					{IAMIdentifier: "KMS_001", Exists: true},
					{IAMIdentifier: "KMS_002", Exists: true},
				}, result,
			)
		},
	)

	t.Run(
		"Should confirm group IAM non-existence", func(t *testing.T) {
			result, err := m.CheckIAMExistenceOfGroups(
				ctx,
				[]string{"NON_EXISTENT_GROUP"},
			)
			assert.NoError(t, err)
			assert.Equal(
				t, []manager.GroupIAMExistence{
					{IAMIdentifier: "NON_EXISTENT_GROUP", Exists: false},
				}, result,
			)
		},
	)
}
