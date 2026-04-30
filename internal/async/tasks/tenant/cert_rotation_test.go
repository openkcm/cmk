package tasks_test

import (
	"context"
	"crypto/rsa"
	"log/slog"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"

	tasks "github.com/openkcm/cmk/internal/async/tasks/tenant"
	"github.com/openkcm/cmk/internal/authz"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	authz_repo "github.com/openkcm/cmk/internal/authz/repo"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	asyncUtils "github.com/openkcm/cmk/utils/async"
)

var GoodWorkflowID = "00000000-0000-0000-0000-000000000000"

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
	return err
}

func (s *CertUpdaterMockUnauthz) RotateCertificate(_ context.Context,
	_ model.RequestCertArgs,
) (*model.Certificate, *rsa.PrivateKey, error) {
	return nil, nil, nil
}

func TestCertificateRotatorProcessAction(t *testing.T) {
	cfg := testutils.TestDBConfig{}

	db, _, _ := testutils.NewTestDB(t, cfg)
	r := sql.NewRepository(db)

	authzRepoLoader := authz_loader.NewRepoAuthzLoader(t.Context(),
		r, &config.Config{})

	authzRepo := authz_repo.NewAuthzRepo(r, authzRepoLoader)

	rotator := tasks.NewCertRotator(&CertUpdaterMock{authzLoader: authzRepoLoader}, authzRepo)

	unauthzRotator := tasks.NewCertRotator(&CertUpdaterMockUnauthz{authzLoader: authzRepoLoader}, authzRepo)

	ctx := t.Context()

	t.Run("Should process without error", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		task := asynq.NewTask(config.TypeCertificateTask, nil)
		err := rotator.ProcessTask(ctx, task)
		assert.NoError(t, err)
		assert.NotContains(t, strings.ToLower(buf.String()), "error")
	})

	t.Run("Should have right taskType", func(t *testing.T) {
		assert.Equal(t, config.TypeCertificateTask, rotator.TaskType())
	})

	t.Run("Should have default tenant query", func(t *testing.T) {
		assert.Equal(t, repo.NewQuery(), rotator.TenantQuery())
	})

	t.Run("Should log on unauthorized processing", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		payload := asyncUtils.NewTaskPayload(ctx, []byte(GoodWorkflowID))
		data, err := payload.ToBytes()
		assert.NoError(t, err)

		task := asynq.NewTask(config.TypeWorkflowAutoAssign, data)
		err = unauthzRotator.ProcessTask(ctx, task)
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "Error during certificate rotation batch processing")
		assert.Contains(t, buf.String(), "authorization decision error")
	})

	t.Run("Should Create with tenant lists", func(t *testing.T) {
		logger, buf := testutils.NewLogBuffer()
		slog.SetDefault(logger)

		payload := asyncUtils.NewTenantListPayload([]string{"tenant1", "tenant2"})
		payloadBytes, err := payload.ToBytes()
		assert.NoError(t, err)

		task := asynq.NewTask(config.TypeCertificateTask, payloadBytes)
		err = rotator.ProcessTask(ctx, task)
		assert.NoError(t, err)
		assert.NotContains(t, strings.ToLower(buf.String()), "error")
	})
}
