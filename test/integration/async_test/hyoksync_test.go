package async_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/stretchr/testify/assert"

	"github.tools.sap/kms/cmk/internal/async"
	"github.tools.sap/kms/cmk/internal/config"
	"github.tools.sap/kms/cmk/internal/log"
	"github.tools.sap/kms/cmk/internal/model"
	"github.tools.sap/kms/cmk/internal/repo"
	"github.tools.sap/kms/cmk/internal/repo/sql"
	"github.tools.sap/kms/cmk/internal/testutils"
	integrationutils "github.tools.sap/kms/cmk/test/integration/integration_utils"
)

func TestSchedulerHYOKSync(t *testing.T) {
	if integrationutils.CheckAllPluginsMissingFiles(t) {
		return
	}

	testConfig := getConfig(t, config.Scheduler{
		TaskQueue: integrationutils.MessageService,
		Tasks: []config.Task{{
			Cronspec: "@every 4s",
			TaskType: config.TypeHYOKSync,
			Retries:  0,
		}},
	})
	SetupTestContainers(t, testConfig)

	testDB, tenants, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		Models: []driver.TenantTabler{
			&model.System{}, &model.KeyConfiguration{}, &model.Key{},
			&model.Group{}, &model.Certificate{},
		},
	}, testutils.WithDatabase(testConfig.Database))

	ctx := testutils.CreateCtxWithTenant(tenants[0])

	repository := sql.NewRepository(testDB)

	setupDatabase(ctx, t, repository, true)

	cronWorker, err := async.New(testConfig)
	assert.NoError(t, err)

	overrideDatabase(t, cronWorker, testDB, testConfig)

	// Start worker
	go func() {
		err := cronWorker.RunWorker(ctx)
		assert.NoError(t, err)
	}()

	// Start scheduler
	go func() {
		err := cronWorker.RunScheduler()
		assert.NoError(t, err)
	}()

	time.Sleep(5 * time.Second)
	// Check that new keys have been created
	keys := []*model.Key{}
	countAll, err := repository.List(
		ctx,
		model.Key{},
		&keys,
		*repo.NewQuery(),
	)
	assert.NoError(t, err)
	assert.Positive(t, countAll, "No keys found after sync")

	for _, k := range keys {
		log.Info(ctx, "Key found", slog.Any("Key", k))
		assert.Equal(t, "UNKNOWN", k.State, "Key state should be UNKNOWN after sync")
	}

	err = cronWorker.Shutdown(ctx)
	assert.NoError(t, err)
}
