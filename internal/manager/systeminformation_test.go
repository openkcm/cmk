package manager_test

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"
	systeminformationv1 "github.com/openkcm/plugin-sdk/proto/plugin/systeminformation/v1"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	cmkplugincatalog "github.com/openkcm/cmk/internal/plugincatalog"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
)

const (
	SystemRole   = "roleName"
	SystemRoleID = "roleID"
	SystemName   = "externalName"
)

func SetupSystemInfoManager(t *testing.T) (
	*manager.SystemInformation,
	*multitenancy.DB,
	string,
) {
	t.Helper()

	db, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	dbRepository := sql.NewRepository(db)
	ctlg, err := cmkplugincatalog.New(t.Context(), &config.Config{Plugins: testutils.SetupMockPlugins(testutils.SystemInfo)})
	assert.NoError(t, err)
	systemManager, err := manager.NewSystemInformationManager(
		dbRepository,
		ctlg,
		&config.System{
			OptionalProperties: map[string]config.SystemProperty{
				SystemRole:   {},
				SystemRoleID: {},
				SystemName:   {},
			},
		},
	)
	assert.NoError(t, err)

	return systemManager, db, tenants[0]
}

type ErrorMockClient struct{}

func (ErrorMockClient) Get(_ context.Context,
	_ *systeminformationv1.GetRequest, _ ...grpc.CallOption,
) (*systeminformationv1.GetResponse, error) {
	return nil, status.Errorf(codes.Internal, "error")
}

func allFakeData(id string) map[string]string {
	return map[string]string{
		"externalName": fakeData(id, "external-name"),
		"roleID":       fakeData(id, "system-role-id"),
		"roleName":     fakeData(id, "system-role-name"),
	}
}

func roleFakeData(id string) map[string]string {
	return map[string]string{"roleID": fakeData(id, "system-role-id")}
}

func externalNameFakeData(id string) map[string]string {
	return map[string]string{"externalName": fakeData(id, "external-name")}
}

func fakeData(id, obj string) string {
	return id + obj
}

func fakeDataReturned(m map[string]func(ID string) map[string]string) func(ID string) map[string]string {
	return func(ID string) map[string]string {
		if fn, exists := m[ID]; exists {
			return fn(ID)
		}

		return allFakeData(ID)
	}
}

type PredictedResponseMock struct {
	ResponseFunc  func(ID string) map[string]string
	noResponseIDs []string
}

func (e PredictedResponseMock) Get(_ context.Context,
	req *systeminformationv1.GetRequest, _ ...grpc.CallOption,
) (*systeminformationv1.GetResponse, error) {
	ID := req.GetId()
	if slices.Contains(e.noResponseIDs, ID) {
		return nil, nil //nolint:nilnil
	}

	return &systeminformationv1.GetResponse{
		Metadata: e.ResponseFunc(ID),
	}, nil
}

func createSystemForTestsWithEmptyExternalData() *model.System {
	system := testutils.NewSystem(func(s *model.System) {
		s.Status = "DISCONNECTED"
	})

	return system
}

func createSystemForTests() *model.System {
	system := testutils.NewSystem(func(s *model.System) {
		s.Status = "DISCONNECTED"
		s.Properties = map[string]string{
			SystemRole:   "givenSystemRole",
			SystemRoleID: "givenSystemRoleID",
			SystemName:   "givenExternalName",
		}
	})

	return system
}

func TestNewSystemInformationManager(t *testing.T) {
	tests := []struct {
		name          string
		plugins       []testutils.MockPlugin
		expectedError error
	}{
		{
			name:          "NoPluginInCatalog",
			plugins:       []testutils.MockPlugin{},
			expectedError: manager.ErrNoPluginInCatalog,
		},
		{
			name:          "ValidPluginInCatalog",
			plugins:       []testutils.MockPlugin{testutils.SystemInfo},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Config{
				Plugins: testutils.SetupMockPlugins(tt.plugins...),
				ContextModels: config.ContextModels{
					System: config.System{
						OptionalProperties: map[string]config.SystemProperty{
							SystemRole:   {},
							SystemRoleID: {},
							SystemName:   {},
						},
					},
				},
			}
			ctlg, err := cmkplugincatalog.New(t.Context(), &cfg)
			assert.NoError(t, err)

			_, err = manager.NewSystemInformationManager(nil, ctlg, &cfg.ContextModels.System)
			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUpdateSystemsDbError(t *testing.T) {
	si, db, tenant := SetupSystemInfoManager(t)
	forced := testutils.NewDBErrorForced(db, ErrForced)
	forced.Register()
	t.Cleanup(func() {
		forced.Unregister()
	})

	err := si.UpdateSystems(testutils.CreateCtxWithTenant(tenant))
	assert.ErrorIs(t, err, manager.ErrGettingSystemList)
}

func TestUpdateNoSystems(t *testing.T) {
	si, _, tenant := SetupSystemInfoManager(t)

	err := si.UpdateSystems(testutils.CreateCtxWithTenant(tenant))
	assert.NoError(t, err)
}

func TestUpdateSystems(t *testing.T) {
	si, db, tenant := SetupSystemInfoManager(t)
	ctx := testutils.CreateCtxWithTenant(tenant)
	r := sql.NewRepository(db)

	firstSystem := createSystemForTests()
	secondSystem := createSystemForTestsWithEmptyExternalData()
	thirdSystem := createSystemForTestsWithEmptyExternalData()
	testutils.CreateTestEntities(ctx, t, r, firstSystem, secondSystem, thirdSystem)

	si.SetClient(PredictedResponseMock{
		ResponseFunc: fakeDataReturned(map[string]func(ID string) map[string]string{
			firstSystem.Identifier:  roleFakeData,
			secondSystem.Identifier: externalNameFakeData,
		}),
		noResponseIDs: []string{thirdSystem.Identifier},
	})

	err := si.UpdateSystems(ctx)
	assert.NoError(t, err)

	firstSystemUpdated, err := repo.GetSystemByIDWithProperties(ctx, r, firstSystem.ID, repo.NewQuery())

	assert.NoError(t, err)
	assert.Equal(
		t,
		fakeData(firstSystem.Identifier, "system-role-id"),
		firstSystemUpdated.Properties[SystemRoleID],
	)
	assert.Equal(t, "givenExternalName", firstSystemUpdated.Properties[SystemName])
	assert.Equal(t, "givenSystemRole", firstSystemUpdated.Properties[SystemRole])

	secondSystemUpdated, err := repo.GetSystemByIDWithProperties(ctx, r, secondSystem.ID, repo.NewQuery())
	assert.NoError(t, err)
	assert.Empty(t, secondSystemUpdated.Properties[SystemRoleID])
	assert.Equal(
		t,
		fakeData(secondSystem.Identifier, "external-name"),
		secondSystemUpdated.Properties[SystemName],
	)
	assert.Empty(t, secondSystemUpdated.Properties[SystemRole])

	thirdSystemNotUpdated, err := repo.GetSystemByIDWithProperties(ctx, r, thirdSystem.ID, repo.NewQuery())
	assert.NoError(t, err)
	assert.Empty(t, thirdSystemNotUpdated.Properties[SystemRoleID])
	assert.Empty(t, thirdSystemNotUpdated.Properties[SystemName])
	assert.Empty(t, thirdSystemNotUpdated.Properties[SystemRole])
}

func TestUpdateSystemByExternalIDDbError(t *testing.T) {
	si, db, tenant := SetupSystemInfoManager(t)
	ctx := testutils.CreateCtxWithTenant(tenant)
	forced := testutils.NewDBErrorForced(db, ErrForced)
	forced.Register()
	t.Cleanup(func() {
		forced.Unregister()
	})

	err := si.UpdateSystemByExternalID(ctx, "test-system-id")
	assert.ErrorIs(t, err, manager.ErrGettingSystem)
}

func TestUpdateSystemByExternalIDNoSystemInformation(t *testing.T) {
	si, db, tenant := SetupSystemInfoManager(t)
	repository := sql.NewRepository(db)
	ctx := testutils.CreateCtxWithTenant(tenant)
	r := sql.NewRepository(db)

	system := createSystemForTestsWithEmptyExternalData()
	testutils.CreateTestEntities(ctx, t, r, system)

	err := si.UpdateSystemByExternalID(ctx, system.Identifier)
	assert.NoError(t, err)

	_, err = repository.First(ctx, system, *repo.NewQuery())
	assert.NoError(t, err)
	assert.Empty(t, system.Properties[SystemRole])
	assert.Empty(t, system.Properties[SystemName])
	assert.Empty(t, system.Properties[SystemRoleID])
}

func TestUpdateSystemByExternalIDNoSystem(t *testing.T) {
	si, _, tenant := SetupSystemInfoManager(t)
	ctx := testutils.CreateCtxWithTenant(tenant)
	err := si.UpdateSystemByExternalID(ctx, "system-id")
	assert.ErrorIs(t, err, manager.ErrNoSystem)
}

func TestUpdateSystemByExternalID(t *testing.T) {
	si, db, tenant := SetupSystemInfoManager(t)
	ctx := testutils.CreateCtxWithTenant(tenant)
	r := sql.NewRepository(db)

	system := createSystemForTestsWithEmptyExternalData()
	testutils.CreateTestEntities(ctx, t, r, system)

	si.SetClient(PredictedResponseMock{ResponseFunc: allFakeData})

	err := si.UpdateSystemByExternalID(ctx, system.Identifier)
	assert.NoError(t, err)

	sys, err := repo.GetSystemByIDWithProperties(ctx, r, system.ID, repo.NewQuery())
	assert.NoError(t, err)
	assert.Equal(t, fakeData(sys.Identifier, "system-role-id"), sys.Properties[SystemRoleID])
	assert.Equal(t, fakeData(sys.Identifier, "external-name"), sys.Properties[SystemName])
	assert.Equal(t, fakeData(sys.Identifier, "system-role-name"), sys.Properties[SystemRole])
}

func TestUpdateSystemByExternalIDReplace(t *testing.T) {
	si, db, tenant := SetupSystemInfoManager(t)
	ctx := testutils.CreateCtxWithTenant(tenant)
	r := sql.NewRepository(db)

	system := createSystemForTests()
	testutils.CreateTestEntities(ctx, t, r, system)

	si.SetClient(PredictedResponseMock{ResponseFunc: roleFakeData})

	err := si.UpdateSystemByExternalID(ctx, system.Identifier)
	assert.NoError(t, err)

	sys, err := repo.GetSystemByIDWithProperties(ctx, r, system.ID, repo.NewQuery())
	assert.NoError(t, err)
	assert.Equal(t, fakeData(system.Identifier, "system-role-id"), sys.Properties[SystemRoleID])
	assert.Equal(t, "givenExternalName", sys.Properties[SystemName])
	assert.Equal(t, "givenSystemRole", sys.Properties[SystemRole])
}
