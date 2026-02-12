package eventprocessor_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/openkcm/orbital"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/config"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	eventProto "github.com/openkcm/cmk/internal/event-processor/proto"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

func setupFactory(t *testing.T) (*eventprocessor.EventFactory, repo.Repo, string) {
	t.Helper()

	db, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{
		CreateDatabase: true,
		WithOrbital:    true,
	})
	r := sql.NewRepository(db)

	cfg := &config.Config{
		Database: dbCfg,
	}

	eventFactory, err := eventprocessor.NewEventFactory(t.Context(), cfg, r)
	assert.NoError(t, err)

	return eventFactory, r, tenants[0]
}

func TestKeyEventCreation(t *testing.T) {
	eventProcessor, _, tenant := setupFactory(t)

	tests := []struct {
		name       string
		keyEventFn func(ctx context.Context, keyID string) (orbital.Job, error)
		keyID      string
		tenantID   string
		expErr     error
		expType    string
	}{
		{
			name:       "should return error on missing keyID for key detach",
			keyEventFn: eventProcessor.KeyDetach,
			expErr:     eventprocessor.ErrMissingKeyID,
		},
		{
			name:       "should return error on missing keyID for key enable",
			keyEventFn: eventProcessor.KeyEnable,
			expErr:     eventprocessor.ErrMissingKeyID,
		},
		{
			name:       "should return error on missing keyID for key disable",
			keyEventFn: eventProcessor.KeyDisable,
			expErr:     eventprocessor.ErrMissingKeyID,
		},
		{
			name:       "should return error on missing tenant from ctx for key detach",
			keyEventFn: eventProcessor.KeyDetach,
			keyID:      "keyID",
			expErr:     cmkcontext.ErrExtractTenantID,
		},
		{
			name:       "should return error on missing tenant from ctx for key enable",
			keyEventFn: eventProcessor.KeyEnable,
			keyID:      "keyID",
			expErr:     cmkcontext.ErrExtractTenantID,
		},
		{
			name:       "should return error on missing tenant from ctx for key disable",
			keyEventFn: eventProcessor.KeyDisable,
			keyID:      "keyID",
			expErr:     cmkcontext.ErrExtractTenantID,
		},
		{
			name:       "should create key detach event",
			keyEventFn: eventProcessor.KeyDetach,
			keyID:      "keyID",
			tenantID:   tenant,
			expType:    eventProto.TaskType_KEY_DETACH.String(),
		},
		{
			name:       "should create key enable event",
			keyEventFn: eventProcessor.KeyEnable,
			keyID:      "keyID",
			tenantID:   tenant,
			expType:    eventProto.TaskType_KEY_ENABLE.String(),
		},
		{
			name:       "should create key disable event",
			keyEventFn: eventProcessor.KeyDisable,
			keyID:      "keyID",
			tenantID:   tenant,
			expType:    eventProto.TaskType_KEY_DISABLE.String(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := testutils.CreateCtxWithTenant(tt.tenantID)
			job, err := tt.keyEventFn(ctx, tt.keyID)

			if tt.expErr != nil {
				assert.ErrorIs(t, err, tt.expErr)
				assert.Equal(t, orbital.Job{}, job)

				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expType, job.Type)
			assert.Equal(t, tt.keyID, job.ExternalID)

			var jobData eventprocessor.KeyActionJobData
			assert.NoError(t, json.Unmarshal(job.Data, &jobData))

			assert.Equal(t, tt.tenantID, jobData.TenantID)
			assert.Equal(t, tt.keyID, jobData.KeyID)
		})
	}
}

func TestSystemEventCreation(t *testing.T) {
	eventProcessor, r, tenant := setupFactory(t)

	tests := []struct {
		name          string
		systemEventFn func(ctx context.Context, system *model.System, keyIDTo, keyIDFrom string) (orbital.Job, error)
		systemStatus  cmkapi.SystemStatus
		tenantID      string
		keyIDTo       string
		keyIDFrom     string
		expErr        error
		expType       string
		expStatus     cmkapi.SystemStatus
		assertKeyID   func(t *testing.T, keyIDTo, keyIDFrom string, data eventprocessor.SystemActionJobData)
	}{
		{
			name: "should return error on missing tenant from ctx for system link",
			systemEventFn: func(ctx context.Context, system *model.System, to, _ string) (orbital.Job, error) {
				return eventProcessor.SystemLink(ctx, system, to)
			},
			systemStatus: cmkapi.SystemStatusCONNECTED,
			expErr:       cmkcontext.ErrExtractTenantID,
		},
		{
			name: "should return error on system in processing state for system link - KMS20-3467",
			systemEventFn: func(ctx context.Context, system *model.System, to, _ string) (orbital.Job, error) {
				return eventProcessor.SystemLink(ctx, system, to)
			},
			systemStatus: cmkapi.SystemStatusPROCESSING,
			tenantID:     tenant,
			expErr:       eventprocessor.ErrSystemProcessing,
		},
		{
			name: "should create system link event and set system to processing - KMS20-3467",
			systemEventFn: func(ctx context.Context, system *model.System, to, _ string) (orbital.Job, error) {
				return eventProcessor.SystemLink(ctx, system, to)
			},
			systemStatus: cmkapi.SystemStatusCONNECTED,
			tenantID:     tenant,
			keyIDTo:      "keyIDTo",
			expType:      eventProto.TaskType_SYSTEM_LINK.String(),
			expStatus:    cmkapi.SystemStatusPROCESSING,
			assertKeyID: func(t *testing.T, keyIDTo, _ string, data eventprocessor.SystemActionJobData) {
				t.Helper()
				assert.Equal(t, keyIDTo, data.KeyIDTo)
			},
		},
		{
			name: "should return error on missing tenant from ctx for system unlink",
			systemEventFn: func(ctx context.Context, system *model.System, _, from string) (orbital.Job, error) {
				return eventProcessor.SystemUnlink(ctx, system, from, "")
			},
			systemStatus: cmkapi.SystemStatusCONNECTED,
			expErr:       cmkcontext.ErrExtractTenantID,
		},
		{
			name: "should return error on system in processing state for system unlink - KMS20-3467",
			systemEventFn: func(ctx context.Context, system *model.System, _, from string) (orbital.Job, error) {
				return eventProcessor.SystemUnlink(ctx, system, from, "")
			},
			systemStatus: cmkapi.SystemStatusPROCESSING,
			tenantID:     tenant,
			expErr:       eventprocessor.ErrSystemProcessing,
		},
		{
			name: "should create system unlink event and set system to processing - KMS20-3467",
			systemEventFn: func(ctx context.Context, system *model.System, _, from string) (orbital.Job, error) {
				return eventProcessor.SystemUnlink(ctx, system, from, "")
			},
			systemStatus: cmkapi.SystemStatusCONNECTED,
			tenantID:     tenant,
			keyIDFrom:    "keyIDFrom",
			expType:      eventProto.TaskType_SYSTEM_UNLINK.String(),
			expStatus:    cmkapi.SystemStatusPROCESSING,
			assertKeyID: func(t *testing.T, _, keyIDFrom string, data eventprocessor.SystemActionJobData) {
				t.Helper()
				assert.Equal(t, keyIDFrom, data.KeyIDFrom)
			},
		},
		{
			name: "should return error on missing tenant from ctx for system switch",
			systemEventFn: func(ctx context.Context, system *model.System, to, from string) (orbital.Job, error) {
				return eventProcessor.SystemSwitch(ctx, system, to, from, "")
			},
			systemStatus: cmkapi.SystemStatusCONNECTED,
			expErr:       cmkcontext.ErrExtractTenantID,
		},
		{
			name: "should return error on system in processing state for system switch - KMS20-3467",
			systemEventFn: func(ctx context.Context, system *model.System, to, from string) (orbital.Job, error) {
				return eventProcessor.SystemSwitch(ctx, system, to, from, "")
			},
			systemStatus: cmkapi.SystemStatusPROCESSING,
			tenantID:     tenant,
			expErr:       eventprocessor.ErrSystemProcessing,
		},
		{
			name: "should create system switch event and set system to processing - KMS20-3467",
			systemEventFn: func(ctx context.Context, system *model.System, to, from string) (orbital.Job, error) {
				return eventProcessor.SystemSwitch(ctx, system, to, from, "")
			},
			systemStatus: cmkapi.SystemStatusCONNECTED,
			tenantID:     tenant,
			keyIDTo:      "keyIDTo",
			keyIDFrom:    "keyIDFrom",
			expType:      eventProto.TaskType_SYSTEM_SWITCH.String(),
			expStatus:    cmkapi.SystemStatusPROCESSING,
			assertKeyID: func(t *testing.T, keyIDTo, keyIDFrom string, data eventprocessor.SystemActionJobData) {
				t.Helper()
				assert.Equal(t, keyIDTo, data.KeyIDTo)
				assert.Equal(t, keyIDFrom, data.KeyIDFrom)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			system := testutils.NewSystem(func(s *model.System) {
				s.Status = tt.systemStatus
			})

			ctx := testutils.CreateCtxWithTenant(tt.tenantID)
			if tt.tenantID != "" {
				testutils.CreateTestEntities(ctx, t, r, system)
			}

			job, err := tt.systemEventFn(ctx, system, tt.keyIDTo, tt.keyIDFrom)

			if tt.expErr != nil {
				assert.ErrorIs(t, err, tt.expErr)
				assert.Equal(t, orbital.Job{}, job)

				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expType, job.Type)
			assert.Equal(t, system.ID.String(), job.ExternalID)

			var jobData eventprocessor.SystemActionJobData
			assert.NoError(t, json.Unmarshal(job.Data, &jobData))

			assert.Equal(t, tt.tenantID, jobData.TenantID)
			assert.Equal(t, system.ID.String(), jobData.SystemID)
			tt.assertKeyID(t, tt.keyIDTo, tt.keyIDFrom, jobData)

			_, err = r.First(ctx, system, *repo.NewQuery())
			assert.NoError(t, err)
			assert.Equal(t, tt.expStatus, system.Status)
		})
	}
}

func TestJobConfirmation(t *testing.T) {
	eventProcessor, r, tenant := setupFactory(t)
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
				return eventProcessor.SystemUnlink(ctx, s, keyID, "")
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
			jobFromDB := &testutils.OrbitalJob{ID: job.ID}
			_, err = r.First(orbitalCtx, jobFromDB, *repo.NewQuery())
			assert.NoError(t, err)

			assert.Equal(t, job.Type, jobFromDB.Type)
			assert.Equal(t, job.ID.String(), jobFromDB.ID.String())

			assert.NotEqual(t, orbital.JobStatusConfirming, jobFromDB.Status)
		})
	}
}
