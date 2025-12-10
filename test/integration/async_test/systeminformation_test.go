package async_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/stretchr/testify/assert"

	"github.tools.sap/kms/cmk/internal/async"
	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/repo"
	"github.tools.sap/kms/cmk/internal/repo/sql"
	"github.tools.sap/kms/cmk/internal/testutils"
	integrationutils "github.tools.sap/kms/cmk/test/integration/integration_utils"
)

var (
	SystemRole   = "roleName"
	SystemRoleID = "roleID"
	SystemName   = "externalName"
)

func TestSchedulerSystemRefresh(t *testing.T) {
	if integrationutils.CheckAllPluginsMissingFiles(t) {
		return
	}

	testConfig := getConfig(t, config.Scheduler{
		TaskQueue: integrationutils.MessageService,
		Tasks: []config.Task{{
			Cronspec: "@every 1s",
			TaskType: config.TypeSystemsTask,
			Retries:  3,
		}},
	})

	SetupTestContainers(t, testConfig)

	testDB, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		Models: []driver.TenantTabler{&model.System{}, &model.SystemProperty{},
			&model.KeyConfiguration{}},
	}, testutils.WithDatabase(testConfig.Database))
	ctx := testutils.CreateCtxWithTenant(tenants[0])

	repository := sql.NewRepository(testDB)

	id := 20
	externalID := fmt.Sprintf("External%d", id)

	testutils.CreateTestEntities(ctx, t, repository, testutils.NewSystem(func(s *model.System) {
		s.Identifier = externalID
	}))

	cronWorker, err := async.New(testConfig)
	assert.NoError(t, err)

	overrideDatabase(t, cronWorker, testDB, testConfig)

	// Start worker
	go func() {
		err := cronWorker.RunWorker(t.Context())
		assert.NoError(t, err)
	}()

	// Start scheduler
	go func() {
		err := cronWorker.RunScheduler()
		assert.NoError(t, err)
	}()

	time.Sleep(2 * time.Second)

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
	assert.True(t, ok)

	sys, err = repo.GetSystemByIDWithProperties(ctx, repository, sys.ID, repo.NewQuery())
	assert.NoError(t, err)
	assert.Equal(
		t,
		fmt.Sprintf("ExternalName%d", id),
		sys.Properties[SystemName],
	)
	assert.Equal(
		t,
		fmt.Sprintf("roleId%d", id),
		sys.Properties[SystemRoleID],
	)

	ok, err = repository.Delete(ctx, sys, *repo.NewQuery())
	assert.NoError(t, err)
	assert.True(t, ok)

	err = cronWorker.Shutdown(ctx)
	assert.NoError(t, err)
}
