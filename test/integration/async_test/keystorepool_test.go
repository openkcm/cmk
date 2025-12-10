package async_test

import (
	"context"
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

func TestKeystorePoolFilling(t *testing.T) {
	if integrationutils.CheckAllPluginsMissingFiles(t) {
		return
	}

	testConfig := getConfig(t, config.Scheduler{
		TaskQueue: integrationutils.MessageService,
		Tasks: []config.Task{{
			Cronspec: "@every 1s",
			TaskType: config.TypeKeystorePool,
			Retries:  3,
		}},
	})
	SetupTestContainers(t, testConfig)

	db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		Models: []driver.TenantTabler{
			&model.Certificate{}, &model.KeystoreConfiguration{},
		},
	}, testutils.WithDatabase(testConfig.Database))

	ctx := context.Background()

	repository := sql.NewRepository(db)

	cronWorker, err := async.New(testConfig)
	assert.NoError(t, err)

	overrideDatabase(t, cronWorker, db, testConfig)

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
