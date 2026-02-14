//go:build !unit

package systeminformation_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"

	"github.com/openkcm/cmk/internal/config"
	cmkplugincatalog "github.com/openkcm/cmk/internal/grpc/catalog"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	integrationutils "github.com/openkcm/cmk/test/integration/integration_utils"
)

const (
	SystemRole   = "roleName"
	SystemRoleID = "roleID"
	SystemName   = "externalName"
)

type SystemInformationSuite struct {
	suite.Suite
}

func (s *SystemInformationSuite) TestUpdateSystems() {
	t := s.T()
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	ctx := testutils.CreateCtxWithTenant(tenants[0])

	repository := sql.NewRepository(db)

	const startID = 20

	const endID = 31
	for i := startID; i < endID; i++ {
		sys := testutils.NewSystem(func(s *model.System) {
			s.Identifier = fmt.Sprintf("Identifier%d", i)
		})
		testutils.CreateTestEntities(ctx, t, repository, sys)
	}

	clg, err := cmkplugincatalog.New(
		t.Context(),
		&config.Config{Plugins: []plugincatalog.PluginConfig{integrationutils.SISPlugin(t)}},
	)
	assert.NoError(t, err)

	defer clg.Close()

	assert.NoError(t, err)

	si := manager.NewSystemInformationManager(repository, clg.SystemInformation(), &config.System{
		OptionalProperties: map[string]config.SystemProperty{
			SystemRole:   {},
			SystemRoleID: {},
			SystemName:   {},
		},
	})
	assert.NoError(t, err)
	assert.NotNil(t, si)

	err = si.UpdateSystems(ctx)
	assert.NoError(t, err)

	for i := startID; i < endID; i++ {
		externalID := fmt.Sprintf("Identifier%d", i)
		sys := &model.System{Identifier: externalID}
		ck := repo.NewCompositeKey().
			Where(repo.IdentifierField, externalID)
		ok, err := repository.First(
			ctx,
			sys,
			*repo.NewQuery().
				Where(repo.NewCompositeKeyGroup(ck)),
		)
		assert.NoError(t, err)
		sys, err = repo.GetSystemByIDWithProperties(ctx, repository, sys.ID, repo.NewQuery())
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, sys.Properties[SystemName],
			fmt.Sprintf("ExternalName%d", i))
		assert.Equal(t, sys.Properties[SystemRoleID],
			fmt.Sprintf("roleId%d", i))

		ok, err = repository.Delete(ctx, sys, *repo.NewQuery())
		assert.NoError(t, err)
		assert.True(t, ok)
	}
}

func (s *SystemInformationSuite) TestUpdateSystemByExternalID() {
	t := s.T()
	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	ctx := testutils.CreateCtxWithTenant(tenants[0])

	repository := sql.NewRepository(db)

	systemNumber := 7
	identifier := fmt.Sprintf("Identifier%d", systemNumber)
	system := testutils.NewSystem(func(s *model.System) {
		s.Identifier = identifier
	})
	testutils.CreateTestEntities(ctx, t, repository, system)

	defer func() {
		ck := repo.NewCompositeKey().
			Where(repo.IdentifierField, identifier)
		ok, err := repository.Delete(
			ctx,
			system,
			*repo.NewQuery().
				Where(repo.NewCompositeKeyGroup(ck)),
		)
		assert.NoError(t, err)
		assert.True(t, ok)
	}()

	clg, err := cmkplugincatalog.New(
		t.Context(),
		&config.Config{Plugins: []plugincatalog.PluginConfig{integrationutils.SISPlugin(t)}},
	)
	assert.NoError(t, err)

	defer clg.Close()

	assert.NoError(t, err)

	si := manager.NewSystemInformationManager(repository, clg.SystemInformation(), &config.System{
		OptionalProperties: map[string]config.SystemProperty{
			SystemRole:   {},
			SystemRoleID: {},
			SystemName:   {},
		},
	})
	assert.NoError(t, err)
	assert.NotNil(t, si)

	err = si.UpdateSystemByExternalID(ctx, identifier)
	assert.NoError(t, err)

	sys := &model.System{Identifier: identifier}
	ok, err := repository.First(ctx, sys, *repo.NewQuery())
	assert.NoError(t, err)
	assert.True(t, ok)

	sys, err = repo.GetSystemByIDWithProperties(ctx, repository, sys.ID, repo.NewQuery())
	assert.NoError(t, err)
	assert.Equal(t, sys.Properties[SystemName],
		fmt.Sprintf("ExternalName%d", systemNumber))
	assert.Equal(t, sys.Properties[SystemRoleID],
		fmt.Sprintf("roleId%d", systemNumber))
}

func TestSystemInformationTest(t *testing.T) {
	suite.Run(t, new(SystemInformationSuite))
}
