package tasks_test

import (
	"context"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/async/tasks"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
)

type KeystorePoolFillerMock struct{}

func (s *KeystorePoolFillerMock) FillKeystorePool(_ context.Context, _ int) error {
	return nil
}

func TestKeystorePoolFillingAction(t *testing.T) {
	db, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
		Models: []driver.TenantTabler{&testutils.TestModel{}},
	})
	repo := sql.NewRepository(db)

	filler := tasks.NewKeystorePoolFiller(
		&KeystorePoolFillerMock{},
		repo,
		config.KeystorePool{
			Size: 5,
		},
	)

	t.Run("Should Create", func(t *testing.T) {
		err := filler.ProcessTask(t.Context(), nil)
		assert.NoError(t, err)
	})
}
