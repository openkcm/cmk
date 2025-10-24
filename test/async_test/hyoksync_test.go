package async_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk-core/internal/async"
	"github.com/openkcm/cmk-core/internal/config"
	"github.com/openkcm/cmk-core/internal/log"
	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/internal/repo"
	"github.com/openkcm/cmk-core/internal/repo/sql"
	"github.com/openkcm/cmk-core/internal/testutils"
	integrationutils "github.com/openkcm/cmk-core/test/integration_utils"
)

func TestSchedulerHYOKSync(t *testing.T) {
	testConfig := getConfig(t, config.Scheduler{
		TaskQueue: integrationutils.MessageService,
		Tasks: []config.Task{{
			Cronspec: "@every 4s",
			TaskType: config.TypeHYOKSync,
			Retries:  0,
		}},
	})

	testDB, tenants := testutils.NewTestDB(t, testutils.TestDBConfig{
		Models: []driver.TenantTabler{
			&model.System{}, &model.KeyConfiguration{}, &model.Key{},
			&model.Group{}, &model.Certificate{},
		},
	})

	ctx := testutils.CreateCtxWithTenant(tenants[0])

	repository := sql.NewRepository(testDB)

	setupDatabase(ctx, t, repository, true)

	cronWorker, err := async.New(testConfig)
	assert.NoError(t, err)

	overrideDatabase(t, cronWorker, testDB, testConfig)

	// Start worker
	go func() {
		err := cronWorker.RunWorker(ctx, testConfig)
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
