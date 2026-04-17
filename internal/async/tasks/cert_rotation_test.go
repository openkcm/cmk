package tasks_test

import (
	"context"
	"crypto/rsa"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/async/tasks"
	"github.com/openkcm/cmk/internal/authz"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	asyncUtils "github.com/openkcm/cmk/utils/async"
)

var allowedCertTestActions = []authz.RepoAction{
	authz.RepoActionList,
	authz.RepoActionFirst,
	authz.RepoActionCreate,
	authz.RepoActionCount,
	authz.RepoActionUpdate,
}

type CertUpdaterMock struct {
	authzLoader *authz_loader.AuthzLoader[authz.RepoResourceTypeName,
		authz.RepoAction]
}

func (s *CertUpdaterMock) RotateExpiredCertificates(ctx context.Context) error {
	s.authzLoader.LoadAllowList(ctx)
	for _, testAction := range allowedCertTestActions {
		isAllowed, err := authz.CheckAuthz(ctx, s.authzLoader.AuthzHandler,
			authz.RepoResourceTypeCertificate, testAction)
		if err != nil {
			return err
		}
		if !isAllowed {
			return authz.ErrAuthzDecision
		}
	}
	return nil
}

func (s *CertUpdaterMock) RotateCertificate(_ context.Context,
	_ model.RequestCertArgs,
) (*model.Certificate, *rsa.PrivateKey, error) {
	return nil, nil, nil
}

type CertUpdaterMockUnauthz struct {
	authzLoader *authz_loader.AuthzLoader[authz.RepoResourceTypeName,
		authz.RepoAction]
}

func (s *CertUpdaterMockUnauthz) RotateExpiredCertificates(ctx context.Context) error {
	s.authzLoader.LoadAllowList(ctx)
	_, err := authz.CheckAuthz(ctx, s.authzLoader.AuthzHandler,
		authz.RepoResourceTypeCertificate, authz.RepoActionDelete)
	if err != nil {
		return err
	}
	return nil
}

func (s *CertUpdaterMockUnauthz) RotateCertificate(_ context.Context,
	_ model.RequestCertArgs,
) (*model.Certificate, *rsa.PrivateKey, error) {
	return nil, nil, nil
}

func TestCertificateRotatorProcessAction(t *testing.T) {
	cfg := testutils.TestDBConfig{}

	db, _, _ := testutils.NewTestDB(t, cfg)
	repo := sql.NewRepository(db)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(),
		repo, &config.Config{})

	authzRepo := authz_repo.NewAuthzRepo(repo, authzRepoLoader)

	rotator := tasks.NewCertRotator(&CertUpdaterMock{authzLoader: authzRepoLoader}, authzRepo)

	unauthzRotator := tasks.NewCertRotator(&CertUpdaterMockUnauthz{authzLoader: authzRepoLoader}, authzRepo)

	ctx := t.Context()

	t.Run("Should Create", func(t *testing.T) {
		err := rotator.ProcessTask(ctx, nil)
		assert.NoError(t, err)
	})

	t.Run("Unauthorized processing", func(t *testing.T) {
		payload := asyncUtils.NewTaskPayload(ctx, []byte(GoodWorkflowID))
		data, err := payload.ToBytes()
		assert.NoError(t, err)

		task := asynq.NewTask(config.TypeWorkflowAutoAssign, data)
		err = unauthzRotator.ProcessTask(ctx, task)
		assert.ErrorIs(t, err, authz.ErrAuthorizationDenied)
	})

	t.Run("Should Create with tenant lists", func(t *testing.T) {
		payload := asyncUtils.NewTenantListPayload([]string{"tenant1", "tenant2"})
		payloadBytes, err := payload.ToBytes()
		assert.NoError(t, err)

		task := asynq.NewTask("", payloadBytes)
		err = rotator.ProcessTask(ctx, task)
		assert.NoError(t, err)
	})
}
