package async_test

import (
	"context"
	"testing"
	"time"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk-core/internal/async"
	"github.com/openkcm/cmk-core/internal/config"
	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/internal/repo"
	"github.com/openkcm/cmk-core/internal/repo/sql"
	"github.com/openkcm/cmk-core/internal/testutils"
	integrationutils "github.com/openkcm/cmk-core/test/integration_utils"
)

func TestKeystorePoolFilling(t *testing.T) {
	testConfig := getConfig(t, config.Scheduler{
		TaskQueue: integrationutils.MessageService,
		Tasks: []config.Task{{
			Cronspec: "@every 1s",
			TaskType: config.TypeKeystorePool,
			Retries:  3,
		}},
	})

	db, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		Models: []driver.TenantTabler{
			&model.Certificate{}, &model.KeystoreConfiguration{},
		},
	}, testutils.WithDatabase(integrationutils.DB))

	ctx := context.Background()

	repository := sql.NewRepository(db)

	cronWorker, err := async.New(testConfig)
	assert.NoError(t, err)

	overrideDatabase(t, cronWorker, db, testConfig)

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

	ks := []*model.KeystoreConfiguration{}
	count, err := repository.List(
		ctx,
		model.KeystoreConfiguration{},
		&ks,
		*repo.NewQuery(),
	)
	assert.NoError(t, err)
	assert.Equal(t, testConfig.KeystorePool.Size, count)
}
