package eventprocessor_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/openkcm/orbital"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/config"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	eventProto "github.com/openkcm/cmk/internal/event-processor/proto"
	"github.com/openkcm/cmk/internal/grpc/catalog"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

func setup(t *testing.T) (*eventprocessor.CryptoReconciler, repo.Repo, string) {
	t.Helper()

	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{
		Models: []driver.TenantTabler{
			&model.Event{},
			&model.System{},
			&model.Key{},
		},
		CreateDatabase: true,
		WithOrbital:    true,
	})
	r := sql.NewRepository(db)

	cfg := config.Config{
		Database: dbCfg,
	}

	ctlg, err := catalog.New(t.Context(), cfg)
	assert.NoError(t, err)

	eventProcessor, err := eventprocessor.NewCryptoReconciler(
		t.Context(), &cfg, r,
		ctlg,
	)
	assert.NoError(t, err)

	t.Cleanup(func() {
		eventProcessor.CloseAmqpClients(context.Background())
	})

	return eventProcessor, r, tenants[0]
}

func TestJobCreation(t *testing.T) {
	eventProcessor, r, tenant := setup(t)
	ctx := testutils.CreateCtxWithTenant(tenant)

	t.Run("should create system link job", func(t *testing.T) {
		system := testutils.NewSystem(func(_ *model.System) {})
		job, err := eventProcessor.SystemLink(ctx, system, "keyID")
		assert.NoError(t, err)

		var jobData eventprocessor.SystemActionJobData
		assert.NoError(t, json.Unmarshal(job.Data, &jobData))

		assert.Equal(t, "SYSTEM_LINK", job.Type)
		assert.Equal(t, tenant, jobData.TenantID)
		assert.Equal(t, "keyID", jobData.KeyIDTo)
		assert.Equal(t, system.ID.String(), jobData.SystemID)
	})

	t.Run("should fail to create system link job on missing tenant from ctx", func(t *testing.T) {
		system := testutils.NewSystem(func(_ *model.System) {})
		job, err := eventProcessor.SystemLink(t.Context(), system, "keyID")
		assert.Error(t, err)
		assert.Equal(t, orbital.Job{}, job)
	})

	t.Run("should create system unlink job", func(t *testing.T) {
		system := testutils.NewSystem(func(_ *model.System) {})
		job, err := eventProcessor.SystemUnlink(ctx, system, "keyID")
		assert.NoError(t, err)

		var jobData eventprocessor.SystemActionJobData
		assert.NoError(t, json.Unmarshal(job.Data, &jobData))

		assert.Equal(t, "SYSTEM_UNLINK", job.Type)
		assert.Equal(t, tenant, jobData.TenantID)
		assert.Equal(t, "keyID", jobData.KeyIDFrom)
		assert.Equal(t, system.ID.String(), jobData.SystemID)
	})

	t.Run("should create system switch job", func(t *testing.T) {
		system := testutils.NewSystem(func(_ *model.System) {})
		job, err := eventProcessor.SystemSwitch(ctx, system, "keyIDTo", "keyIDFrom")
		assert.NoError(t, err)

		var jobData eventprocessor.SystemActionJobData
		assert.NoError(t, json.Unmarshal(job.Data, &jobData))

		assert.Equal(t, "SYSTEM_SWITCH", job.Type)
		assert.Equal(t, tenant, jobData.TenantID)
		assert.Equal(t, "keyIDTo", jobData.KeyIDTo)
		assert.Equal(t, "keyIDFrom", jobData.KeyIDFrom)
		assert.Equal(t, system.ID.String(), jobData.SystemID)
	})

	t.Run("should create key enable", func(t *testing.T) {
		job, err := eventProcessor.KeyEnable(ctx, "keyID")
		assert.NoError(t, err)

		var jobData eventprocessor.KeyActionJobData
		assert.NoError(t, json.Unmarshal(job.Data, &jobData))

		assert.Equal(t, "KEY_ENABLE", job.Type)
		assert.Equal(t, tenant, jobData.TenantID)
		assert.Equal(t, "keyID", jobData.KeyID)
	})

	t.Run("should fail to create key enable job on missing tenant from ctx", func(t *testing.T) {
		job, err := eventProcessor.KeyEnable(t.Context(), "keyID")
		assert.Error(t, err)
		assert.Equal(t, orbital.Job{}, job)
	})

	t.Run("should create key disable job", func(t *testing.T) {
		job, err := eventProcessor.KeyDisable(ctx, "keyID")
		assert.NoError(t, err)

		var jobData eventprocessor.KeyActionJobData
		assert.NoError(t, json.Unmarshal(job.Data, &jobData))

		assert.Equal(t, "KEY_DISABLE", job.Type)
		assert.Equal(t, tenant, jobData.TenantID)
		assert.Equal(t, "keyID", jobData.KeyID)
	})

	t.Run("should fail to create key disable job on missing tenant from ctx", func(t *testing.T) {
		job, err := eventProcessor.KeyDisable(t.Context(), "keyID")
		assert.Error(t, err)
		assert.Equal(t, orbital.Job{}, job)
	})

	t.Run("Should fail to create job if system is in processing - KMS20-3467", func(t *testing.T) {
		system := testutils.NewSystem(func(s *model.System) {
			s.Status = cmkapi.SystemStatusPROCESSING
		})
		_, err := eventProcessor.SystemSwitch(ctx, system, "keyIDTo", "keyIDFrom")
		assert.ErrorIs(t, err, eventprocessor.ErrSystemProcessing)
	})

	t.Run("should set system to processing on successful job creation - KMS20-3467", func(t *testing.T) {
		system := testutils.NewSystem(func(_ *model.System) {})
		testutils.CreateTestEntities(ctx, t, r, system)
		job, err := eventProcessor.SystemLink(ctx, system, "keyID")
		assert.NoError(t, err)

		var jobData eventprocessor.SystemActionJobData
		assert.NoError(t, json.Unmarshal(job.Data, &jobData))

		assert.Equal(t, "SYSTEM_LINK", job.Type)
		assert.Equal(t, tenant, jobData.TenantID)
		assert.Equal(t, "keyID", jobData.KeyIDTo)
		assert.Equal(t, system.ID.String(), jobData.SystemID)

		_, err = r.First(ctx, system, *repo.NewQuery())
		assert.NoError(t, err)
		assert.Equal(t, cmkapi.SystemStatusPROCESSING, system.Status)
	})
}

type orbitalJob struct {
	ID           uuid.UUID
	ExternalID   string
	Data         []byte
	Type         string
	Status       string
	ErrorMessage string
	UpdatedAt    int64
	CreatedAt    int64
}

func (orbitalJob) TableName() string {
	return "jobs"
}

func (orbitalJob) IsSharedModel() bool {
	return false
}

func TestJobConfirmation(t *testing.T) {
	eventProcessor, r, tenant := setup(t)
	ctx := testutils.CreateCtxWithTenant(tenant)

	tests := []struct {
		name string
		job  func(s *model.System, keyID string) (orbital.Job, error)
	}{
		{
			name: "should confirm system link job",
			job: func(s *model.System, keyID string) (orbital.Job, error) {
				return eventProcessor.SystemLink(ctx, s, keyID)
			},
		},
		{
			name: "should confirm system unlink job",
			job: func(s *model.System, keyID string) (orbital.Job, error) {
				return eventProcessor.SystemUnlink(ctx, s, keyID)
			},
		},
		{
			name: "should confirm key enable job",
			job: func(_ *model.System, keyID string) (orbital.Job, error) {
				return eventProcessor.KeyEnable(ctx, keyID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			system := testutils.NewSystem(func(_ *model.System) {})
			key := testutils.NewKey(func(_ *model.Key) {})
			testutils.CreateTestEntities(ctx, t, r, system, key)

			job, err := tt.job(system, key.ID.String())

			assert.NoError(t, err)

			orbitalCtx := cmkcontext.CreateTenantContext(ctx, "orbital")
			jobFromDB := &orbitalJob{ID: job.ID}
			_, err = r.First(orbitalCtx, jobFromDB, *repo.NewQuery())
			assert.NoError(t, err)

			assert.Equal(t, job.Type, jobFromDB.Type)
			assert.Equal(t, job.ID.String(), jobFromDB.ID.String())

			assert.NotEqual(t, orbital.JobStatusConfirming, jobFromDB.Status)
		})
	}
}

func TestEventWritting(t *testing.T) {
	eventProcessor, r, tenant := setup(t)
	ctx := testutils.CreateCtxWithTenant(tenant)

	t.Run("Should create item in cmk events db on job termination", func(t *testing.T) {
		job := terminateNewJob(t, eventProcessor, &model.Event{
			Identifier: uuid.NewString(),
			Type:       eventProto.TaskType_SYSTEM_LINK.String(),
			Data:       fmt.Appendf(nil, "{\"tenantID\": \"%s\"}", tenant),
			Status:     orbital.JobStatusProcessing,
		})

		event := &model.Event{
			Identifier: job.ExternalID,
		}
		_, err := r.First(ctx, event, *repo.NewQuery())
		assert.NoError(t, err)

		assert.Equal(t, job.Status, event.Status)
		assert.Equal(t, job.Type, event.Type)
	})

	t.Run("Should update existing item in cmk events db on job termination", func(t *testing.T) {
		item := uuid.NewString()
		job := terminateNewJob(t, eventProcessor, &model.Event{
			Identifier: item,
			Type:       eventProto.TaskType_SYSTEM_LINK.String(),
			Data:       fmt.Appendf(nil, "{\"tenantID\": \"%s\"}", tenant),
			Status:     orbital.JobStatusProcessing,
		})

		job2 := terminateNewJob(t, eventProcessor, &model.Event{
			Identifier: item,
			Type:       eventProto.TaskType_SYSTEM_UNLINK.String(),
			Data:       fmt.Appendf(nil, "{\"tenantID\": \"%s\"}", tenant),
			Status:     orbital.JobStatusProcessing,
		})

		event := &model.Event{
			Identifier: job.ExternalID,
		}
		_, err := r.First(ctx, event, *repo.NewQuery())
		assert.NoError(t, err)

		assert.Equal(t, job.Status, event.Status)
		assert.Equal(t, job2.Type, event.Type)
	})
}

func terminateNewJob(t *testing.T, eventProcessor *eventprocessor.CryptoReconciler, e *model.Event) orbital.Job {
	t.Helper()

	job, err := eventProcessor.CreateJob(t.Context(), e)
	assert.NotNil(t, job)
	assert.NoError(t, err)

	// Ignored as this test is not testing the system update capabilities
	_ = eventProcessor.JobTerminationFunc(t.Context(), job)

	return job
}
