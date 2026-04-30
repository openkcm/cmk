package tasks_test

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/async/tasks"
	"github.com/openkcm/cmk/internal/authz"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
)

var allowedKeystorePoolActions = []authz.RepoAction{
	authz.RepoActionCreate,
	authz.RepoActionCount,
}

type KeystorePoolFillerMock struct {
	authzLoader *authz_loader.AuthzLoader[authz.RepoResourceTypeName,
		authz.RepoAction]
}

func (s *KeystorePoolFillerMock) FillKeystorePool(ctx context.Context, _ int) error {
	s.authzLoader.LoadAllowList(ctx)
	for _, testAction := range allowedKeystorePoolActions {
		isAllowed, err := authz.CheckAuthz(ctx, s.authzLoader.AuthzHandler,
			authz.RepoResourceTypeKeystore, testAction)
		if err != nil {
			return err
		}
		if !isAllowed {
			return authz.ErrAuthzDecision
		}
	}
	return nil
}

type KeystorePoolFillerMockUnauthz struct {
	authzLoader *authz_loader.AuthzLoader[authz.RepoResourceTypeName,
		authz.RepoAction]
}

func (s *KeystorePoolFillerMockUnauthz) FillKeystorePool(ctx context.Context, _ int) error {
	s.authzLoader.LoadAllowList(ctx)
	_, err := authz.CheckAuthz(ctx, s.authzLoader.AuthzHandler,
		authz.RepoResourceTypeKeystore, authz.RepoActionDelete)
	if err != nil {
		return err
	}
	return nil
}

func TestKeystorePoolFillingAction(t *testing.T) {
	db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	repo := sql.NewRepository(db)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(),
		repo, &config.Config{})

	authzRepo := authz_repo.NewAuthzRepo(repo, authzRepoLoader)

	filler := tasks.NewKeystorePoolFiller(
		&KeystorePoolFillerMock{authzLoader: authzRepoLoader},
		authzRepo,
		config.KeystorePool{
			Size: 5,
		},
	)
	task := asynq.NewTask(config.TypeKeystorePool, nil)

	t.Run("Should Create", func(t *testing.T) {
		err := filler.ProcessTask(t.Context(), task)
		assert.NoError(t, err)
	})

	t.Run("Unauthorized processing", func(t *testing.T) {
		filler := tasks.NewKeystorePoolFiller(
			&KeystorePoolFillerMockUnauthz{authzLoader: authzRepoLoader},
			authzRepo,
			config.KeystorePool{
				Size: 5,
			},
		)
		err := filler.ProcessTask(t.Context(), nil)
		assert.ErrorIs(t, err, authz.ErrAuthorizationDenied)
	})
}
