package async_test

import (
	"slices"
	"testing"
	"time"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	integrationutils "github.com/openkcm/cmk/test/integration_utils"
)

func TestCertRotation(t *testing.T) {
	testConfig := getConfig(t, config.Scheduler{
		TaskQueue: integrationutils.MessageService,
		Tasks: []config.Task{{
			Cronspec: "@every 1s",
			TaskType: config.TypeCertificateTask,
			Retries:  3,
		}},
	})

	db, tenants := testutils.NewTestDB(t, testutils.TestDBConfig{
		Models: []driver.TenantTabler{&model.System{}, &model.Certificate{}},
	}, testutils.WithDatabase(integrationutils.DB))

	ctx := testutils.CreateCtxWithTenant(tenants[0])

	repository := sql.NewRepository(db)

	setupDatabase(ctx, t, repository, false)

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

	// Check that new certificates have been created
	certsAll := []*model.Certificate{}
	countAll, err := repository.List(
		ctx,
		model.Certificate{},
		&certsAll,
		*repo.NewQuery(),
	)
	assert.NoError(t, err)
	assert.Greater(t, countAll, 1)

	// Check that all rotated certs have a SupersedesID. Only original doesn't
	certsMod := slices.DeleteFunc(certsAll, func(c *model.Certificate) bool {
		return c.SupersedesID == nil
	})
	assert.Len(t, certsMod, countAll-1)

	// Check only the head has AutoRotate remaining
	certsAuto := []*model.Certificate{}
	compositeKey := repo.NewCompositeKey().Where(repo.AutoRotateField, true)
	countAuto, err := repository.List(
		ctx,
		model.Certificate{},
		&certsAuto,
		*repo.NewQuery().Where(repo.NewCompositeKeyGroup(compositeKey)),
	)
	assert.NoError(t, err)
	assert.Equal(t, 1, countAuto)

	err = cronWorker.Shutdown(ctx)
	assert.NoError(t, err)
}
