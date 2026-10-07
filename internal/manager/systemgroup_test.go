package manager_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cmkapi "github.com/openkcm/cmk/internal/api/cmk/generated"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/multitenancy"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
)

func SetupSystemGroupManager(t *testing.T) (manager.SystemGroup, repo.Repo, *multitenancy.DB, string) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})

	r := sql.NewRepository(db)
	m := manager.NewSystemGroup(r)

	return m, r, db, tenants[0]
}

func TestGetAllSystemGroups(t *testing.T) {
	m, r, _, tenant := SetupSystemGroupManager(t)
	ctx := testutils.CreateCtxWithTenant(tenant)
	ctx = testutils.InjectBusinessUserDataIntoContext(ctx, "test-user", []string{"test-group"})

	groupWithSystems := testutils.NewSystemGroup(func(g *model.SystemGroup) {})
	emptyGroup := testutils.NewSystemGroup(func(g *model.SystemGroup) {})
	sysA := testutils.NewSystem(func(s *model.System) {
		s.SystemGroupID = &groupWithSystems.ID
		s.Status = cmkapi.SystemStatusCONNECTED
	})
	sysB := testutils.NewSystem(func(s *model.System) {
		s.SystemGroupID = &groupWithSystems.ID
		s.Status = cmkapi.SystemStatusDISCONNECTED
	})
	testutils.CreateTestEntities(ctx, t, r, groupWithSystems, emptyGroup, sysA, sysB)

	t.Run("Should list system groups without expanding systems", func(t *testing.T) {
		groups, count, err := m.GetAllSystemGroups(ctx, manager.SystemGroupFilter{
			ExpandSystems: false,
			Pagination:    repo.Pagination{Count: true},
		})
		require.NoError(t, err)

		ids := getGroupIDsFromGroups(groups)
		require.Contains(t, ids, groupWithSystems.ID)
		require.Contains(t, ids, emptyGroup.ID)
		assert.GreaterOrEqual(t, count, 2)
	})

	t.Run("Should list system groups with expand", func(t *testing.T) {
		groups, _, err := m.GetAllSystemGroups(ctx, manager.SystemGroupFilter{
			ExpandSystems: true,
			Pagination:    repo.Pagination{Count: true},
		})
		require.NoError(t, err)

		for _, g := range groups {
			if g.ID == groupWithSystems.ID {
				assert.ElementsMatch(t,
					[]uuid.UUID{sysA.ID, sysB.ID},
					getSystemIDsFromGroup(g),
				)
			}
		}
	})

	t.Run("Should have system without group under root", func(t *testing.T) {
		orphan := testutils.NewSystem(func(s *model.System) {
			s.SystemGroupID = nil
			s.Status = cmkapi.SystemStatusCONNECTED
		})
		testutils.CreateTestEntities(ctx, t, r, orphan)

		groups, _, err := m.GetAllSystemGroups(ctx, manager.SystemGroupFilter{
			ExpandSystems: true,
		})
		require.NoError(t, err)

		ids := getGroupIDsFromGroups(groups)
		for i, id := range ids {
			if id == uuid.Nil {
				groups[i].Name = "root"
				assert.Contains(t, groups[i].Systems, orphan)
			}
		}
	})
}

func TestGetAllSystemGroupsWarnCoverage(t *testing.T) {
	t.Run("Should warn when a group mixes connected and disconnected systems", func(t *testing.T) {
		m, r, _, tenant := SetupSystemGroupManager(t)
		ctx := testutils.CreateCtxWithTenant(tenant)

		group := testutils.NewSystemGroup(func(g *model.SystemGroup) {})
		connected := testutils.NewSystem(func(s *model.System) {
			s.SystemGroupID = &group.ID
			s.Status = cmkapi.SystemStatusCONNECTED
		})
		disconnected := testutils.NewSystem(func(s *model.System) {
			s.SystemGroupID = &group.ID
			s.Status = cmkapi.SystemStatusDISCONNECTED
		})
		testutils.CreateTestEntities(ctx, t, r, group, connected, disconnected)

		groups, _, err := m.GetAllSystemGroups(ctx, manager.SystemGroupFilter{ExpandSystems: true})
		require.NoError(t, err)

		ids := getGroupIDsFromGroups(groups)
		for i, id := range ids {
			if group.ID == id {
				assert.True(t, groups[i].WarnCoverage)
			}
		}
	})

	t.Run("Should warn even when systems span pages", func(t *testing.T) {
		m, r, _, tenant := SetupSystemGroupManager(t)
		ctx := testutils.CreateCtxWithTenant(tenant)

		group := testutils.NewSystemGroup(func(g *model.SystemGroup) {})
		connected := testutils.NewSystem(func(s *model.System) {
			s.SystemGroupID = &group.ID
			s.Status = cmkapi.SystemStatusCONNECTED
		})
		disconnected := testutils.NewSystem(func(s *model.System) {
			s.SystemGroupID = &group.ID
			s.Status = cmkapi.SystemStatusDISCONNECTED
		})
		testutils.CreateTestEntities(ctx, t, r, group, connected, disconnected)

		groups, _, err := m.GetAllSystemGroups(ctx, manager.SystemGroupFilter{
			ExpandSystems: true,
			Pagination:    repo.Pagination{Top: 1},
		})
		require.NoError(t, err)

		ids := getGroupIDsFromGroups(groups)
		for i, id := range ids {
			if group.ID == id {
				assert.True(t, groups[i].WarnCoverage)
				assert.Len(t, groups[i].Systems, 1, "page should contain only one system")
			}
		}
	})

	t.Run("Should not warn when all systems are connected", func(t *testing.T) {
		m, r, _, tenant := SetupSystemGroupManager(t)
		ctx := testutils.CreateCtxWithTenant(tenant)

		group := testutils.NewSystemGroup(func(g *model.SystemGroup) {})
		sysA := testutils.NewSystem(func(s *model.System) {
			s.SystemGroupID = &group.ID
			s.Status = cmkapi.SystemStatusCONNECTED
		})
		sysB := testutils.NewSystem(func(s *model.System) {
			s.SystemGroupID = &group.ID
			s.Status = cmkapi.SystemStatusCONNECTED
		})
		testutils.CreateTestEntities(ctx, t, r, group, sysA, sysB)

		groups, _, err := m.GetAllSystemGroups(ctx, manager.SystemGroupFilter{ExpandSystems: true})
		require.NoError(t, err)

		ids := getGroupIDsFromGroups(groups)
		for i, id := range ids {
			if group.ID == id {
				assert.False(t, groups[i].WarnCoverage)
			}
		}
	})

	t.Run("Should not warn when warnings are suppressed", func(t *testing.T) {
		m, r, _, tenant := SetupSystemGroupManager(t)
		ctx := testutils.CreateCtxWithTenant(tenant)

		group := testutils.NewSystemGroup(func(g *model.SystemGroup) {
			g.SuppressWarning = true
		})
		connected := testutils.NewSystem(func(s *model.System) {
			s.SystemGroupID = &group.ID
			s.Status = cmkapi.SystemStatusCONNECTED
		})
		disconnected := testutils.NewSystem(func(s *model.System) {
			s.SystemGroupID = &group.ID
			s.Status = cmkapi.SystemStatusDISCONNECTED
		})
		testutils.CreateTestEntities(ctx, t, r, group, connected, disconnected)

		groups, _, err := m.GetAllSystemGroups(ctx, manager.SystemGroupFilter{ExpandSystems: true})
		require.NoError(t, err)

		ids := getGroupIDsFromGroups(groups)
		for i, id := range ids {
			if group.ID == id {
				assert.False(t, groups[i].WarnCoverage)
			}
		}
	})
}

func TestUpdateSystemGroup(t *testing.T) {
	m, r, db, tenant := SetupSystemGroupManager(t)
	ctx := testutils.CreateCtxWithTenant(tenant)

	t.Run("Should update name, description and suppress warning", func(t *testing.T) {
		group := testutils.NewSystemGroup(func(g *model.SystemGroup) {})
		testutils.CreateTestEntities(ctx, t, r, group)

		patch := cmkapi.SystemGroupPatch{
			Name:            new("group-updated"),
			Description:     new("updated description"),
			SuppressWarning: new(true),
		}
		updated, err := m.UpdateSystemGroup(ctx, group.ID, patch)
		require.NoError(t, err)
		assert.Equal(t, "group-updated", updated.Name)
		assert.Equal(t, "updated description", updated.Description)
		assert.True(t, updated.SuppressWarning)
	})

	t.Run("Should error when name is empty", func(t *testing.T) {
		group := testutils.NewSystemGroup(func(g *model.SystemGroup) {})
		testutils.CreateTestEntities(ctx, t, r, group)

		_, err := m.UpdateSystemGroup(ctx, group.ID, cmkapi.SystemGroupPatch{
			Name: new(""),
		})
		assert.ErrorIs(t, err, manager.ErrNameCannotBeEmpty)
	})

	t.Run("Should error when group does not exist", func(t *testing.T) {
		_, err := m.UpdateSystemGroup(ctx, uuid.New(), cmkapi.SystemGroupPatch{
			Name: new(uuid.New().String()),
		})
		assert.Error(t, err)
	})

	t.Run("Should error on DB failure", func(t *testing.T) {
		group := testutils.NewSystemGroup(func(g *model.SystemGroup) {})
		testutils.CreateTestEntities(ctx, t, r, group)

		forced := testutils.NewDBErrorForced(db, ErrForced)
		forced.WithUpdate().Register()
		defer forced.Unregister()

		_, err := m.UpdateSystemGroup(ctx, group.ID, cmkapi.SystemGroupPatch{
			Name: new("new-name"),
		})
		assert.Error(t, err)
	})
}

func getGroupIDsFromGroups(groups []*model.SystemGroup) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(groups))
	for _, g := range groups {
		ids = append(ids, g.ID)
	}
	return ids
}

func getSystemIDsFromGroup(group *model.SystemGroup) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(group.Systems))
	for _, g := range group.Systems {
		ids = append(ids, g.ID)
	}
	return ids
}
