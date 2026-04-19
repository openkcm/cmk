package tasks_test

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
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
	db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	repo := sql.NewRepository(db)

	filler := tasks.NewKeystorePoolFiller(
		&KeystorePoolFillerMock{},
		repo,
		config.KeystorePool{
			Size: 5,
		},
	)
	task := asynq.NewTask(config.TypeKeystorePool, nil)

	t.Run("Should Create", func(t *testing.T) {
		err := filler.ProcessTask(t.Context(), task)
		assert.NoError(t, err)
	})
}
