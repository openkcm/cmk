package manager_test

import (
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/utils/ptr"
)

func SetupGroupManager(t *testing.T) (*manager.GroupManager, *multitenancy.DB, string) {
	t.Helper()

	db, tenants := testutils.NewTestDB(t, testutils.TestDBConfig{
		Models: []driver.TenantTabler{&model.KeyConfiguration{}, &model.Group{}},
	})

	dbRepository := sql.NewRepository(db)

	m := manager.NewGroupManager(dbRepository)

	return m, db, tenants[0]
}

func TestNewGroupManager(t *testing.T) {
	t.Run("Should create group manager", func(t *testing.T) {
		m, _, _ := SetupGroupManager(t)
		assert.NotNil(t, m)
	})
}

func TestGetGroups(t *testing.T) {
	m, db, tenant := SetupGroupManager(t)
	t.Run("Should get groups", func(t *testing.T) {
		group := testutils.NewGroup(func(_ *model.Group) {})
		_, err := m.CreateGroup(testutils.CreateCtxWithTenant(tenant), group)
		assert.NoError(t, err)

		groups, total, err := m.GetGroups(
			testutils.CreateCtxWithTenant(tenant),
			constants.DefaultSkip,
			constants.DefaultTop,
		)
		assert.NoError(t, err)
		assert.Equal(t, groups[0].ID, group.ID)
		assert.Equal(t, groups[0].IAMIdentifier, group.IAMIdentifier)
		assert.Equal(t, 1, total)
	})

	t.Run("Should fail on get groups", func(t *testing.T) {
		forced := testutils.NewDBErrorForced(db, ErrForced)

		forced.Register()
		defer forced.Unregister()

		groups, total, err := m.GetGroups(
			testutils.CreateCtxWithTenant(tenant),
			constants.DefaultSkip,
			constants.DefaultTop,
		)
		assert.Nil(t, groups)
		assert.Equal(t, 0, total)
		assert.Error(t, err)
	})
}

func TestCreateGroup(t *testing.T) {
	m, db, tenant := SetupGroupManager(t)
	t.Run("Should create group", func(t *testing.T) {
		expected := testutils.NewGroup(func(_ *model.Group) {})
		res, err := m.CreateGroup(
			testutils.CreateCtxWithTenant(tenant),
			expected,
		)
		assert.NoError(t, err)
		assert.Equal(t, expected.Name, res.Name)
	})

	t.Run("Should error on create group with duplicated name", func(t *testing.T) {
		_, err := m.CreateGroup(
			testutils.CreateCtxWithTenant(tenant),
			testutils.NewGroup(func(g *model.Group) {
				g.Name = "duplicated-name"
			}),
		)
		assert.NoError(t, err)
		_, err = m.CreateGroup(
			testutils.CreateCtxWithTenant(tenant),
			testutils.NewGroup(func(g *model.Group) {
				g.Name = "duplicated-name"
			}),
		)
		assert.ErrorIs(t, err, repo.ErrUniqueConstraint)
	})

	t.Run("Should error on create group with invalid role", func(t *testing.T) {
		_, err := m.CreateGroup(
			testutils.CreateCtxWithTenant(tenant),
			testutils.NewGroup(func(g *model.Group) { g.Role = "invalid-role" }),
		)
		assert.ErrorIs(t, err, manager.ErrGroupRole)
	})

	t.Run("Should error on create group", func(t *testing.T) {
		forced := testutils.NewDBErrorForced(db, ErrForced)

		forced.Register()
		defer forced.Unregister()

		res, err := m.CreateGroup(
			testutils.CreateCtxWithTenant(tenant),
			testutils.NewGroup(func(_ *model.Group) {}),
		)
		assert.Error(t, err)
		assert.Nil(t, res)
	})
}

func TestDeleteGroupByID(t *testing.T) {
	m, db, tenant := SetupGroupManager(t)

	t.Run("Should delete group", func(t *testing.T) {
		group := testutils.NewGroup(func(_ *model.Group) {
		})
		_, err := m.CreateGroup(
			testutils.CreateCtxWithTenant(tenant),
			group,
		)
		assert.NoError(t, err)

		err = m.DeleteGroupByID(testutils.CreateCtxWithTenant(tenant), group.ID)
		assert.NoError(t, err)
	})

	t.Run("Should error on invalid non existing group id", func(t *testing.T) {
		err := m.DeleteGroupByID(testutils.CreateCtxWithTenant(tenant), uuid.New())
		assert.Error(t, err)
	})

	t.Run("Should error delete group if mandatory group", func(t *testing.T) {
		group := testutils.NewGroup(func(g *model.Group) {
			g.Name = constants.TenantAuditorGroup
		})
		_, err := m.CreateGroup(
			testutils.CreateCtxWithTenant(tenant),
			group,
		)
		assert.NoError(t, err)

		err = m.DeleteGroupByID(testutils.CreateCtxWithTenant(tenant), group.ID)
		assert.ErrorIs(t, err, manager.ErrInvalidGroupDelete)
	})

	t.Run("Should error delete group if connected to keyConfig", func(t *testing.T) {
		group := testutils.NewGroup(func(_ *model.Group) {})
		_, err := m.CreateGroup(
			testutils.CreateCtxWithTenant(tenant),
			group,
		)
		assert.NoError(t, err)

		r := sql.NewRepository(db)
		keyConfig := testutils.NewKeyConfig(func(kc *model.KeyConfiguration) {
			kc.AdminGroup.ID = group.ID
		})
		err = r.Create(testutils.CreateCtxWithTenant(tenant), keyConfig)
		assert.NoError(t, err)

		err = m.DeleteGroupByID(testutils.CreateCtxWithTenant(tenant), group.ID)
		assert.ErrorIs(t, err, manager.ErrInvalidGroupDelete)
	})

	t.Run("Should error on delete", func(t *testing.T) {
		forced := testutils.NewDBErrorForced(db, ErrForced)

		forced.WithDelete().Register()
		defer forced.Unregister()

		group := testutils.NewGroup(func(_ *model.Group) {})

		_, err := m.CreateGroup(
			testutils.CreateCtxWithTenant(tenant),
			group,
		)
		assert.NoError(t, err)

		err = m.DeleteGroupByID(testutils.CreateCtxWithTenant(tenant), group.ID)
		assert.Error(t, err)
	})
}

func TestGetGroupByID(t *testing.T) {
	m, _, tenant := SetupGroupManager(t)
	t.Run("Should get group", func(t *testing.T) {
		expected, err := m.CreateGroup(
			testutils.CreateCtxWithTenant(tenant),
			testutils.NewGroup(func(_ *model.Group) {}),
		)
		assert.NoError(t, err)

		group, err := m.GetGroupByID(testutils.CreateCtxWithTenant(tenant), expected.ID)
		assert.Equal(t, expected, group)
		assert.NoError(t, err)
	})

	t.Run("Should fail on get group", func(t *testing.T) {
		group, err := m.GetGroupByID(testutils.CreateCtxWithTenant(tenant), uuid.New())
		assert.Nil(t, group)
		assert.Error(t, err)
	})
}

func TestUpdateGroup(t *testing.T) {
	m, db, tenant := SetupGroupManager(t)
	reservedGroup, err := m.CreateGroup(
		testutils.CreateCtxWithTenant(tenant),
		testutils.NewGroup(func(g *model.Group) {
			g.Name = constants.TenantAdminGroup
		}),
	)
	assert.NoError(t, err)

	t.Run("Should rename group", func(t *testing.T) {
		expected, err := m.CreateGroup(
			testutils.CreateCtxWithTenant(tenant),
			testutils.NewGroup(func(_ *model.Group) {}),
		)
		assert.NoError(t, err)

		patchGroup := cmkapi.GroupPatch{Name: ptr.PointTo("test-updated")}
		group, err := m.UpdateGroup(testutils.CreateCtxWithTenant(tenant), expected.ID, patchGroup)
		expected.Name = *patchGroup.Name
		expected.IAMIdentifier = model.NewIAMIdentifier(expected.Name, tenant)
		assert.Equal(t, expected, group)
		assert.NoError(t, err)
	})

	t.Run("Should error on rename group if name is empty", func(t *testing.T) {
		expected, err := m.CreateGroup(
			testutils.CreateCtxWithTenant(tenant),
			testutils.NewGroup(func(_ *model.Group) {}),
		)
		assert.NoError(t, err)

		patchGroup := cmkapi.GroupPatch{Name: ptr.PointTo("")}
		group, err := m.UpdateGroup(testutils.CreateCtxWithTenant(tenant), expected.ID, patchGroup)
		assert.Nil(t, group)
		assert.Error(t, err)
		assert.ErrorIs(t, err, manager.ErrNameCannotBeEmpty)
	})

	t.Run("Should error on rename if group is mandatory", func(t *testing.T) {
		group, err := m.UpdateGroup(
			testutils.CreateCtxWithTenant(tenant),
			reservedGroup.ID,
			cmkapi.GroupPatch{Name: ptr.PointTo("test")},
		)
		assert.Nil(t, group)
		assert.Error(t, err)
		assert.ErrorIs(t, err, manager.ErrInvalidGroupRename)
	})

	t.Run("Should error on rename if new group name is reserved name", func(t *testing.T) {
		group, err := m.UpdateGroup(
			testutils.CreateCtxWithTenant(tenant),
			reservedGroup.ID,
			cmkapi.GroupPatch{Name: ptr.PointTo(constants.TenantAdminGroup)},
		)
		assert.Nil(t, group)
		assert.Error(t, err)
		assert.ErrorIs(t, err, manager.ErrInvalidGroupRename)
	})

	t.Run("Should error on rename if does not exist", func(t *testing.T) {
		group, err := m.UpdateGroup(
			testutils.CreateCtxWithTenant(tenant),
			uuid.New(),
			cmkapi.GroupPatch{Name: ptr.PointTo("test")},
		)
		assert.Nil(t, group)
		assert.Error(t, err)
	})

	t.Run("Should error on rename with DB error", func(t *testing.T) {
		forced := testutils.NewDBErrorForced(db, ErrForced)

		forced.WithUpdate().Register()
		defer forced.Unregister()

		expected, err := m.CreateGroup(
			testutils.CreateCtxWithTenant(tenant),
			testutils.NewGroup(func(_ *model.Group) {}),
		)
		assert.NoError(t, err)

		group, err := m.UpdateGroup(
			testutils.CreateCtxWithTenant(tenant),
			expected.ID,
			cmkapi.GroupPatch{Name: ptr.PointTo("test")},
		)
		assert.Nil(t, group)
		assert.Error(t, err)
	})
}
