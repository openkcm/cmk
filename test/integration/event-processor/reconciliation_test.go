//nolint:contextcheck
package eventprocessor_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/orbital"
	"github.com/openkcm/orbital/client/amqp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	_ "github.com/lib/pq"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/config"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	eventProto "github.com/openkcm/cmk/internal/event-processor/proto"
	"github.com/openkcm/cmk/internal/grpc/catalog"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	sqlPkg "github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

type tester struct {
	repository repo.Repo
	db         *multitenancy.DB
	tenant     string
	reconciler *eventprocessor.CryptoReconciler
	config     *config.Config
}

func setupTest(t *testing.T) tester {
	t.Helper()

	rabbitMQURL := testutils.StartRabbitMQ(t)

	db, tenants, dbConf := testutils.NewTestDB(
		t,
		testutils.TestDBConfig{
			CreateDatabase:      true,
			WithIsolatedService: true,
		},
	)

	cfg := config.Config{
		EventProcessor: config.EventProcessor{
			SecretRef: commoncfg.SecretRef{
				Type: commoncfg.BasicSecretType,
			},
			Targets: []config.Target{
				{
					Region: "us-east-1",
					AMQP:   config.AMQP{URL: rabbitMQURL, Target: "us-east-1-tasks", Source: "us-east-1-responses"},
				},
				{
					Region: "eu-west-1",
					AMQP:   config.AMQP{URL: rabbitMQURL, Target: "eu-west-1-tasks", Source: "eu-west-1-responses"},
				},
				{
					Region: "ap-south-1",
					AMQP:   config.AMQP{URL: rabbitMQURL, Target: "ap-south-1-tasks", Source: "ap-south-1-responses"},
				},
				{
					Region: "us-west-2",
					AMQP:   config.AMQP{URL: rabbitMQURL, Target: "us-west-2-tasks", Source: "us-west-2-responses"},
				},
			},
		},
		Plugins:  testutils.SetupMockPlugins(testutils.KeyStorePlugin),
		Database: dbConf,
	}

	ctlg, err := catalog.New(t.Context(), &cfg)
	require.NoError(t, err)

	r := sqlPkg.NewRepository(db)

	reconcilerCtx, cancelFunc := context.WithCancel(t.Context())

	reconciler, err := eventprocessor.NewCryptoReconciler(
		reconcilerCtx,
		&cfg,
		r,
		ctlg,
		nil,
		eventprocessor.WithExecInterval(5*time.Millisecond),
		eventprocessor.WithConfirmJobAfter(10*time.Millisecond),
	)
	require.NoError(t, err)
	require.NoError(t, reconciler.Start(reconcilerCtx))

	t.Cleanup(func() {
		// Cancel reconciler context to stop background processes
		cancelFunc()
		// Close reconciler AMQP clients so that they don't interfere with other tests
		reconciler.CloseAmqpClients(context.Background())
	})

	return tester{
		repository: r,
		db:         db,
		tenant:     tenants[0],
		reconciler: reconciler,
		config:     &cfg,
	}
}

func TestReconciler_TaskResolution_KeyAction(t *testing.T) {
	tester := setupTest(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tester.tenant)

	_, keyID := addDataToDB(ctx, t, tester.repository)

	testCases := []struct {
		name        string
		jobType     string
		expTaskType eventProto.TaskType
	}{
		{
			name:        "KEY_ENABLE creates tasks for all targets",
			jobType:     eventProto.TaskType_KEY_ENABLE.String(),
			expTaskType: eventProto.TaskType_KEY_ENABLE,
		},
		{
			name:        "KEY_DISABLE creates tasks for all targets",
			jobType:     eventProto.TaskType_KEY_DISABLE.String(),
			expTaskType: eventProto.TaskType_KEY_DISABLE,
		},
		{
			name:        "KEY_DETACH creates tasks for all targets",
			jobType:     eventProto.TaskType_KEY_DETACH.String(),
			expTaskType: eventProto.TaskType_KEY_DETACH,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var (
				job orbital.Job
				err error
			)

			switch tc.jobType {
			case eventProto.TaskType_KEY_ENABLE.String():
				job, err = tester.reconciler.KeyEnable(ctx, keyID)
			case eventProto.TaskType_KEY_DISABLE.String():
				job, err = tester.reconciler.KeyDisable(ctx, keyID)
			case eventProto.TaskType_KEY_DETACH.String():
				job, err = tester.reconciler.KeyDetach(ctx, keyID)
			default:
				assert.Failf(t, "unsupported job type: %s", tc.jobType)
			}

			require.NoError(t, err)

			var tasks []orbital.Task
			for {
				tasks = getTasksFromDB(t, tester.db, job.ID.String())

				if len(tasks) == len(tester.config.EventProcessor.Targets) {
					break
				}
			}

			for _, task := range tasks {
				assert.Equal(t, tc.jobType, task.Type)

				var taskData eventProto.Data

				err := proto.Unmarshal(task.Data, &taskData)
				require.NoError(t, err)

				assert.Equal(t, tc.expTaskType, taskData.GetTaskType())
				assert.NotNil(t, taskData.GetKeyAction())
				assert.Equal(t, keyID, taskData.GetKeyAction().GetKeyId())
				assert.Equal(t, tester.tenant, taskData.GetKeyAction().GetTenantId())
			}
		})
	}
}

func TestReconciler_TaskResolution_SystemAction(t *testing.T) {
	tester := setupTest(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tester.tenant)

	systemLink, keyIDLink := addDataToDB(ctx, t, tester.repository)
	systemUnlink, keyIDUnlink := addDataToDB(ctx, t, tester.repository)

	testCases := []struct {
		name          string
		jobType       string
		protoTaskType eventProto.TaskType
		keyIDFrom     string
		keyIDTo       string
		system        *model.System
	}{
		{
			name:          "SYSTEM_LINK creates task for system's region only",
			jobType:       eventProto.TaskType_SYSTEM_LINK.String(),
			protoTaskType: eventProto.TaskType_SYSTEM_LINK,
			keyIDFrom:     "",
			keyIDTo:       keyIDLink,
			system:        systemLink,
		},
		{
			name:          "SYSTEM_UNLINK creates task for system's region only",
			jobType:       eventProto.TaskType_SYSTEM_UNLINK.String(),
			protoTaskType: eventProto.TaskType_SYSTEM_UNLINK,
			keyIDFrom:     keyIDUnlink,
			keyIDTo:       "",
			system:        systemUnlink,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// When
			var err error

			var job orbital.Job

			if tc.jobType == "SYSTEM_LINK" {
				job, err = tester.reconciler.SystemLink(ctx, tc.system, tc.keyIDTo)
			} else {
				job, err = tester.reconciler.SystemUnlink(ctx, tc.system, tc.keyIDFrom)
			}

			require.NoError(t, err)

			// Then
			var tasks []orbital.Task

			for {
				tasks = getTasksFromDB(t, tester.db, job.ID.String())

				if len(tasks) == 1 {
					break
				}
			}

			task := tasks[0]
			assert.Equal(t, tc.jobType, task.Type)
			assert.Equal(t, "us-east-1", task.Target)

			var taskData eventProto.Data

			err = proto.Unmarshal(task.Data, &taskData)
			require.NoError(t, err)

			assert.Equal(t, tc.protoTaskType, taskData.GetTaskType(), "task type should match")
			assert.NotNil(t, taskData.GetSystemAction(), "system action should not be nil")
			assert.Equal(t, tc.system.Identifier, taskData.GetSystemAction().GetSystemId(), "system ID should match")
			assert.Equal(t, "us-east-1", taskData.GetSystemAction().GetSystemRegion(), "system region should match")
			assert.Equal(t, "system", taskData.GetSystemAction().GetSystemType(), "system type should match")
			assert.Equal(t, tc.keyIDFrom, taskData.GetSystemAction().GetKeyIdFrom(), "key ID from should match")
			assert.Equal(t, tc.keyIDTo, taskData.GetSystemAction().GetKeyIdTo(), "key ID to should match")
			assert.Equal(t, "test", taskData.GetSystemAction().GetKeyProvider(), "key provider should match")
			assert.Equal(t, tester.tenant, taskData.GetSystemAction().GetTenantId(), "tenant ID should match")
			assert.Equal(t, "tenant0-owner-id", taskData.GetSystemAction().GetTenantOwnerId(), "tenant owner id should match")
			assert.Equal(t, "owner-type", taskData.GetSystemAction().GetTenantOwnerType(), "tenant owner type should match")
		})
	}
}

func TestReconciler_TaskResolution_Errors(t *testing.T) {
	tester := setupTest(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tester.tenant)

	t.Run("SYSTEM_LINK fails when target not configured for system region", func(t *testing.T) {
		system, keyID := addDataToDB(ctx, t, tester.repository)

		updateSystemRegion(ctx, t, tester.repository, system, "eu-west-10")

		job, err := tester.reconciler.SystemLink(ctx, system, keyID)
		require.NoError(t, err)

		err = waitForJobStatus(ctx, t, tester.db, job.ID.String(), orbital.JobStatusResolveCanceled)
		require.NoError(t, err, "Job should be marked as resolve canceled")

		tasks := getTasksFromDB(t, tester.db, job.ID.String())
		assert.Empty(t, tasks, "No tasks should be created when target not configured")
	})

	t.Run("task resolution for system event fails when key does not exist", func(t *testing.T) {
		system, _ := addDataToDB(ctx, t, tester.repository)

		job, err := tester.reconciler.SystemLink(ctx, system, "non-existent-key")
		require.NoError(t, err)

		err = waitForJobStatus(ctx, t, tester.db, job.ID.String(), orbital.JobStatusResolveCanceled)
		require.NoError(t, err, "Job should be marked as resolve canceled")

		tasks := getTasksFromDB(t, tester.db, job.ID.String())
		assert.Empty(t, tasks)
	})
}

func TestReconciler_JobTermination(t *testing.T) {
	tester := setupTest(t)

	tests := map[string]struct {
		operatorResult       bool
		expectedJobStatus    orbital.JobStatus
		expectedSystemStatus cmkapi.SystemStatus
	}{
		"System job terminated successfully": {
			operatorResult:       true,
			expectedJobStatus:    orbital.JobStatusDone,
			expectedSystemStatus: cmkapi.SystemStatusCONNECTED,
		},
		"System job fails because operator returns error": {
			operatorResult:       false,
			expectedJobStatus:    orbital.JobStatusFailed,
			expectedSystemStatus: cmkapi.SystemStatusFAILED,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Given
			ctx := cmkcontext.CreateTenantContext(t.Context(), tester.tenant)

			systemID, keyID := addDataToDB(ctx, t, tester.repository)

			operator := testutils.NewMockAMQPOperator(t, 1, tc.operatorResult, amqp.ConnectionInfo{
				URL:    tester.config.EventProcessor.Targets[0].AMQP.URL,
				Target: "us-east-1-responses",
				Source: "us-east-1-tasks",
			})

			go func(ctx context.Context) {
				operator.Start(ctx)
			}(t.Context())

			// When
			systemJob, err := tester.reconciler.SystemLink(ctx, systemID, keyID)
			require.NoError(t, err)

			// Then
			err = waitForJobStatus(ctx, t, tester.db, systemJob.ID.String(), tc.expectedJobStatus)
			require.NoError(t, err, "Job should be marked as done")

			err = waitForSystemStatus(ctx, t, tester.repository, systemID, tc.expectedSystemStatus)
			require.NoError(t, err, "System should be marked as connected")
		})
	}

	t.Run("System job canceled because of missing target configuration", func(t *testing.T) {
		// Given
		ctx := cmkcontext.CreateTenantContext(t.Context(), tester.tenant)

		systemID, keyID := addDataToDB(ctx, t, tester.repository)

		updateSystemRegion(ctx, t, tester.repository, systemID, "eu-west-10")
		defer updateSystemRegion(ctx, t, tester.repository, systemID, "eu-east-10")

		// When
		systemJob, err := tester.reconciler.SystemLink(ctx, systemID, keyID)
		require.NoError(t, err)

		// Then
		err = waitForJobStatus(ctx, t, tester.db, systemJob.ID.String(), orbital.JobStatusResolveCanceled)
		require.NoError(t, err, "Job should be marked as resolve canceled")

		err = waitForSystemStatus(ctx, t, tester.repository, systemID, cmkapi.SystemStatusFAILED)
		require.NoError(t, err, "System should be marked as failed")
	})
}

func addDataToDB(ctx context.Context, t *testing.T, r repo.Repo) (*model.System, string) {
	t.Helper()

	tenant, err := repo.GetTenant(ctx, r)

	require.NoError(t, err)
	assert.Equal(t, "tenant0", tenant.SchemaName)

	keyUUID := uuid.New()

	data := map[string]map[string]any{
		"us-east-1": {
			"trustAnchorArn": "arn:aws:iam::123456789012:role/TrustAnchor",
			"profileArn":     "arn:aws:iam::123456789012:role/Profile",
			"roleArn":        "arn:aws:iam::123456789012:role/Role",
		},
	}
	bytes, err := json.Marshal(data)
	require.NoError(t, err)

	err = r.Create(ctx, &model.Key{
		ID:               keyUUID,
		Name:             uuid.NewString(),
		Provider:         "TEST",
		NativeID:         ptr.PointTo("arn:aws:kms:us-east-1:123456789012:key/12345678-90ab-cdef-1234-567890abcdef"),
		CryptoAccessData: bytes,
	})
	require.NoError(t, err)

	system := &model.System{
		ID:         uuid.New(),
		Identifier: uuid.NewString(),
		Region:     "us-east-1",
		Status:     cmkapi.SystemStatusDISCONNECTED,
		Type:       "SYSTEM",
	}
	err = r.Create(ctx, system)
	require.NoError(t, err)

	return system, keyUUID.String()
}

func updateSystemRegion(ctx context.Context, t *testing.T, repository repo.Repo, system *model.System, region string) {
	t.Helper()

	system.Region = region

	ck := repo.NewCompositeKey().Where(repo.IDField, system.ID)
	query := repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(ck),
	).Update(repo.RegionField)

	_, err := repository.Patch(ctx, system, *query)
	require.NoError(t, err)
}

func getJobFromDB(t *testing.T, db *multitenancy.DB, jobID string) *orbital.Job {
	t.Helper()

	var job orbital.Job

	err := db.WithTenant(t.Context(), "orbital", func(tx *multitenancy.DB) error {
		query := `
		SELECT id, type, status, data, error_message 
		FROM orbital.jobs WHERE id = $1 
		ORDER BY created_at DESC 
		LIMIT 1`

		return tx.Raw(query, jobID).Scan(&job).Error
	})
	require.NoError(t, err)

	return &job
}

func getTasksFromDB(t *testing.T, db *multitenancy.DB, jobID string) []orbital.Task {
	t.Helper()

	var tasks []orbital.Task

	err := db.WithTenant(t.Context(), "orbital", func(tx *multitenancy.DB) error {
		query := `
		SELECT id, job_id, type, data, working_state, status, target, etag 
		FROM orbital.tasks 
		WHERE job_id = $1 
		ORDER BY created_at DESC`

		return tx.Raw(query, jobID).Scan(&tasks).Error
	})
	require.NoError(t, err)

	return tasks
}

func waitForJobStatus(ctx context.Context, t *testing.T, orbitalDB *multitenancy.DB, jobID string,
	status orbital.JobStatus,
) error {
	t.Helper()

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w; job: %s", ctx.Err(), jobID)
		default:
			job := getJobFromDB(t, orbitalDB, jobID)
			slogctx.Debug(ctx, "Job status", "id", jobID, "current", job.Status, "required", status)

			if job.Status == status {
				return nil
			}
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func waitForSystemStatus(ctx context.Context, t *testing.T, repository repo.Repo, system *model.System,
	status cmkapi.SystemStatus,
) error {
	t.Helper()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w; system: %s", ctx.Err(), system.ID)
		default:
			_, err := repository.First(ctx, system, *repo.NewQuery())
			if err != nil {
				return err
			}

			if system.Status == status {
				return nil
			}
		}

		time.Sleep(100 * time.Millisecond)
	}
}
