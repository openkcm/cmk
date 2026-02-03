package manager_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
)

func SetupUserManager(t *testing.T) (manager.User, *multitenancy.DB, string) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{CreateDatabase: true})

	return manager.NewUserManager(
		sql.NewRepository(db),
		auditor.New(t.Context(), &config.Config{}),
	), db, tenants[0]
}

func TestGetRoleFromGroupIAMIdentifiers(t *testing.T) {
	m, db, tenant := SetupUserManager(t)
	r := sql.NewRepository(db)
	ctx := testutils.CreateCtxWithTenant(tenant)

	t.Run("Should error if user groups have more than one role", func(t *testing.T) {
		g1 := testutils.NewGroup(func(g *model.Group) {
			g.Role = "test1"
		})
		g2 := testutils.NewGroup(func(g *model.Group) {
			g.Role = "test2"
		})

		testutils.CreateTestEntities(ctx, t, r, g1, g2)

		_, err := m.GetRoleFromIAM(
			ctx,
			[]string{g1.IAMIdentifier, g2.IAMIdentifier},
		)
		assert.ErrorIs(t, err, manager.ErrMultipleRolesInGroups)
	})

	t.Run("Should return role from groups", func(t *testing.T) {
		g1 := testutils.NewGroup(func(g *model.Group) {})
		g2 := testutils.NewGroup(func(g *model.Group) {})

		testutils.CreateTestEntities(ctx, t, r, g1, g2)

		role, err := m.GetRoleFromIAM(
			ctx,
			[]string{g1.IAMIdentifier, g2.IAMIdentifier},
		)
		assert.NoError(t, err)
		assert.Equal(t, g1.Role, role)
	})

	t.Run("Should empty with no groups", func(t *testing.T) {
		m, _, tenant := SetupUserManager(t)
		ctx := testutils.CreateCtxWithTenant(tenant)

		role, err := m.GetRoleFromIAM(ctx, []string{})
		assert.NoError(t, err)
		assert.Empty(t, role)
	})
}
