package async_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	integrationutils "github.com/openkcm/cmk/test/integration/integration_utils"
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

	db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})

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

	ks := []*model.Keystore{}
	count, err := repository.List(
		ctx,
		model.Keystore{},
		&ks,
		*repo.NewQuery(),
	)
	assert.NoError(t, err)
	assert.Equal(t, testConfig.KeystorePool.Size, count)
}
