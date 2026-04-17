package tasks_test

import (
	"context"
	"crypto/rsa"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/async/tasks"
	"github.com/openkcm/cmk/internal/authz"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
)

var errMockSyncHYOKClient = errors.New("error syncing hyok client")

var allowedHYOKTestActions = []authz.RepoAction{
	authz.RepoActionList,
	authz.RepoActionCount,
	authz.RepoActionUpdate,
}

type HYOKClientMock struct {
	CallCount   int
	authzLoader *authz_loader.AuthzLoader[authz.RepoResourceTypeName,
		authz.RepoAction]
}

func (s *HYOKClientMock) SyncHYOKKeys(ctx context.Context) error {
	s.CallCount += 1
	s.authzLoader.LoadAllowList(ctx)
	for _, testAction := range allowedHYOKTestActions {
		isAllowed, err := authz.CheckAuthz(ctx, s.authzLoader.AuthzHandler,
			authz.RepoResourceTypeKey, testAction)
		if err != nil {
			return err
		}
		if !isAllowed {
			return authz.ErrAuthzDecision
		}
	}
	return nil
}

type HYOKClientMockFailed struct {
	CallCount int
}

func (s *HYOKClientMockFailed) SyncHYOKKeys(_ context.Context) error {
	s.CallCount += 1
	return errMockSyncHYOKClient
}

type HYOKClientMockUnauthz struct {
	CallCount   int
	authzLoader *authz_loader.AuthzLoader[authz.RepoResourceTypeName,
		authz.RepoAction]
}

func (s *HYOKClientMockUnauthz) SyncHYOKKeys(ctx context.Context) error {
	s.CallCount += 1
	s.authzLoader.LoadAllowList(ctx)
	_, err := authz.CheckAuthz(ctx, s.authzLoader.AuthzHandler,
		authz.RepoResourceTypeKey, authz.RepoActionDelete)
	if err != nil {
		return err
	}
	return nil
}

func (s *HYOKClientMockUnauthz) RotateCertificate(_ context.Context,
	_ model.RequestCertArgs,
) (*model.Certificate, *rsa.PrivateKey, error) {
	return nil, nil, nil
}

func TestHYOKSyncProcessAction(t *testing.T) {
	numTenants := 3
	db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{},
		testutils.WithGenerateTenants(numTenants))
	repo := sql.NewRepository(db)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(),
		repo, &config.Config{})

	authzRepo := authz_repo.NewAuthzRepo(repo, authzRepoLoader)

	mock := &HYOKClientMock{authzLoader: authzRepoLoader}
	sync := tasks.NewHYOKSync(mock, authzRepo)

	t.Run("Should complete", func(t *testing.T) {
		err := sync.ProcessTask(t.Context(), nil)
		assert.NoError(t, err)
		assert.Equal(t, numTenants, mock.CallCount)
	})

	t.Run("Task type is right", func(t *testing.T) {
		taskType := sync.TaskType()
		assert.Equal(t, config.TypeHYOKSync, taskType, "Task type should be HYOKSync")
	})

	t.Run("Task continues one failure of hyok client", func(t *testing.T) {
		mock := &HYOKClientMockFailed{}
		sync := tasks.NewHYOKSync(mock, authzRepo)
		err := sync.ProcessTask(t.Context(), nil)
		assert.Error(t, err)
		assert.Equal(t, numTenants, mock.CallCount)
	})

	t.Run("Unauthorized processing", func(t *testing.T) {
		mock := &HYOKClientMockUnauthz{authzLoader: authzRepoLoader}
		sync := tasks.NewHYOKSync(mock, authzRepo)
		err := sync.ProcessTask(t.Context(), nil)
		assert.Error(t, err)
		assert.Equal(t, numTenants, mock.CallCount)
	})
}
