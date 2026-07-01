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
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	_ "github.com/bartventer/gorm-multitenancy/postgres/v8"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"
	mappingv1 "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/mapping/v1"
	systemgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/system/v1"
	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/cmk/internal/api/cmkapi"
	"github.com/openkcm/cmk/internal/clients"
	"github.com/openkcm/cmk/internal/clients/registry/systems"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	eventprocessor "github.com/openkcm/cmk/internal/event-processor"
	eventProto "github.com/openkcm/cmk/internal/event-processor/proto"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	sqlPkg "github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/internal/testutils/clients/registry/mapping"
	cmkcontext "github.com/openkcm/cmk/utils/context"
	"github.com/openkcm/cmk/utils/ptr"
)

type tester struct {
	repository   repo.Repo
	db           *multitenancy.DB
	tenant       string
	eventFactory *eventprocessor.EventFactory
	config       *config.Config
}

func setupTest(t *testing.T) tester {
	t.Helper()

	rabbitMQURL := testutils.StartRabbitMQ(t)

	db, tenants, dbConf := testutils.NewTestDB(
		t,
		testutils.TestDBConfig{
			CreateDatabase: true,
		},
	)

	svcRegistry := testutils.NewTestPlugins()

	// Unique queue suffix per setupTest call: StartRabbitMQ reuses one container
	// across the binary, so subtests that share queue names would consume each
	// other's task responses.
	queueSuffix := "-" + uuid.NewString()

	cfg := config.Config{
		EventProcessor: config.EventProcessor{
			SecretRef: commoncfg.SecretRef{
				Type: commoncfg.BasicSecretType,
			},
			Targets: []config.Target{
				{
					Region: "us-east-1",
					AMQP: config.AMQP{
						URL:    rabbitMQURL,
						Target: "us-east-1-tasks" + queueSuffix,
						Source: "us-east-1-responses" + queueSuffix,
					},
				},
				{
					Region: "eu-west-1",
					AMQP: config.AMQP{
						URL:    rabbitMQURL,
						Target: "eu-west-1-tasks" + queueSuffix,
						Source: "eu-west-1-responses" + queueSuffix,
					},
				},
				{
					Region: "ap-south-1",
					AMQP: config.AMQP{
						URL:    rabbitMQURL,
						Target: "ap-south-1-tasks" + queueSuffix,
						Source: "ap-south-1-responses" + queueSuffix,
					},
				},
				{
					Region: "us-west-2",
					AMQP: config.AMQP{
						URL:    rabbitMQURL,
						Target: "us-west-2-tasks" + queueSuffix,
						Source: "us-west-2-responses" + queueSuffix,
					},
				},
			},
		},
		Database: dbConf,
	}

	// JobDone/JobFailed handlers call clientsFactory.Registry() (L1 key claim,
	// locked status). A nil registry panics the worker before the job status is
	// written, which is why the original tests "reconciled indefinitely". Mount
	// a FakeService over an in-process gRPC server so both paths have a real
	// callee.
	logger := testutils.SetupLoggerWithBuffer()
	systemService := systems.NewFakeService(logger)
	mappingService := mapping.NewFakeService()
	_, grpcClient := testutils.NewGRPCSuite(
		t,
		func(s *grpc.Server) {
			systemgrpc.RegisterServiceServer(s, systemService)
			mappingv1.RegisterServiceServer(s, mappingService)
		},
	)
	clientsFactory, err := clients.NewFactory(config.Services{
		Registry: &commoncfg.GRPCClient{
			Enabled: true,
			Address: grpcClient.Target(),
			SecretRef: &commoncfg.SecretRef{
				Type: commoncfg.InsecureSecretType,
			},
		},
	})
	require.NoError(t, err)

	r := sqlPkg.NewRepository(db)

	reconcilerCtx, cancelFunc := context.WithCancel(t.Context())

	tcm := manager.NewTenantConfigManager(r, svcRegistry, &cfg, nil)
	reconciler, err := eventprocessor.NewCryptoReconciler(
		reconcilerCtx,
		&cfg,
		r,
		svcRegistry,
		clientsFactory,
		tcm,
		eventprocessor.WithExecInterval(5*time.Millisecond),
		eventprocessor.WithConfirmJobAfter(10*time.Millisecond),
	)
	require.NoError(t, err)
	require.NoError(t, reconciler.Start(reconcilerCtx))

	eventFactory, err := eventprocessor.NewEventFactory(reconcilerCtx, &cfg, r)
	require.NoError(t, err)

	t.Cleanup(func() {
		// Cancel reconciler context to stop background processes
		cancelFunc()
		// Close reconciler AMQP clients so that they don't interfere with other tests
		reconciler.CloseAmqpClients(context.Background())
	})

	return tester{
		repository:   r,
		db:           db,
		tenant:       tenants[0],
		eventFactory: eventFactory,
		config:       &cfg,
	}
}

func TestReconciler_TaskResolution_KeyAction(t *testing.T) {
	tester := setupTest(t)
	ctx := cmkcontext.CreateTenantContext(t.Context(), tester.tenant)

	// KEY_DETACH needs the key in DETACHING state; KEY_ENABLE/DISABLE reject it.
	// Each case creates its own key with the right state and connected systems.
	regions := make([]string, 0, len(tester.config.EventProcessor.Targets))
	for _, target := range tester.config.EventProcessor.Targets {
		regions = append(regions, target.Region)
	}

	testCases := []struct {
		name        string
		jobType     string
		expTaskType eventProto.TaskType
		keyState    cmkapi.KeyState
		// connectAllRegions adds a CONNECTED system in every configured region
		// so KEY_ENABLE/DISABLE fan out to all targets. KEY_DETACH uses the
		// reconciler's target map directly and doesn't need this.
		connectAllRegions bool
	}{
		{
			name:              "KEY_ENABLE creates tasks for all targets",
			jobType:           eventProto.TaskType_KEY_ENABLE.String(),
			expTaskType:       eventProto.TaskType_KEY_ENABLE,
			keyState:          cmkapi.KeyStateENABLED,
			connectAllRegions: true,
		},
		{
			name:              "KEY_DISABLE creates tasks for all targets",
			jobType:           eventProto.TaskType_KEY_DISABLE.String(),
			expTaskType:       eventProto.TaskType_KEY_DISABLE,
			keyState:          cmkapi.KeyStateENABLED,
			connectAllRegions: true,
		},
		{
			name:        "KEY_DETACH creates tasks for all targets",
			jobType:     eventProto.TaskType_KEY_DETACH.String(),
			expTaskType: eventProto.TaskType_KEY_DETACH,
			keyState:    cmkapi.KeyStateDETACHING,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			keyID := createKeyForTest(ctx, t, tester.repository, tc.keyState, tc.connectAllRegions, regions)

			var (
				job orbital.Job
				err error
			)

			switch tc.jobType {
			case eventProto.TaskType_KEY_ENABLE.String():
				job, err = tester.eventFactory.KeyEnable(ctx, keyID)
			case eventProto.TaskType_KEY_DISABLE.String():
				job, err = tester.eventFactory.KeyDisable(ctx, keyID)
			case eventProto.TaskType_KEY_DETACH.String():
				job, err = tester.eventFactory.KeyDetach(ctx, keyID)
			default:
				assert.Failf(t, "unsupported job type: %s", tc.jobType)
			}

			require.NoError(t, err)

			tasks := waitForTasks(ctx, t, tester.db, job.ID.String(), len(tester.config.EventProcessor.Targets))

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

// createKeyForTest persists one Key and, if connectAllRegions is true, one
// CONNECTED System per region sharing the key's KeyConfigurationID. Returns
// the key ID.
func createKeyForTest(
	ctx context.Context,
	t *testing.T,
	r repo.Repo,
	state cmkapi.KeyState,
	connectAllRegions bool,
	regions []string,
) string {
	t.Helper()

	keyUUID := uuid.New()
	keyConfigID := uuid.New()

	accessData := map[string]map[string]any{}
	for _, region := range regions {
		accessData[region] = map[string]any{
			"trustAnchorArn": "arn:aws:iam::123456789012:role/TrustAnchor",
			"profileArn":     "arn:aws:iam::123456789012:role/Profile",
			"roleArn":        "arn:aws:iam::123456789012:role/Role",
		}
	}
	bytes, err := json.Marshal(accessData)
	require.NoError(t, err)

	require.NoError(t, r.Create(ctx, &model.Key{
		ID:                 keyUUID,
		KeyConfigurationID: keyConfigID,
		Name:               uuid.NewString(),
		Provider:           "TEST",
		KeyType:            string(cmkapi.KeyTypeHYOK),
		State:              state,
		NativeID:           ptr.PointTo("arn:aws:kms:us-east-1:123456789012:key/" + keyUUID.String()),
		CryptoAccessData:   bytes,
	}))

	if !connectAllRegions {
		return keyUUID.String()
	}

	for _, region := range regions {
		require.NoError(t, r.Create(ctx, &model.System{
			ID:                 uuid.New(),
			Identifier:         uuid.NewString(),
			Region:             region,
			Status:             cmkapi.SystemStatusCONNECTED,
			Type:               "SYSTEM",
			KeyConfigurationID: &keyConfigID,
		}))
	}

	return keyUUID.String()
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
				job, err = tester.eventFactory.SystemLink(ctx, tc.system, tc.keyIDTo)
			} else {
				job, err = tester.eventFactory.SystemUnlink(ctx, tc.system, tc.keyIDFrom)
			}

			require.NoError(t, err)

			// Then
			tasks := waitForTasks(ctx, t, tester.db, job.ID.String(), 1)

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

		job, err := tester.eventFactory.SystemLink(ctx, system, keyID)
		require.NoError(t, err)

		err = waitForJobStatus(ctx, t, tester.db, job.ID.String(), orbital.JobStatusResolveCanceled)
		require.NoError(t, err, "Job should be marked as resolve canceled")

		tasks := getTasksFromDB(t, tester.db, job.ID.String())
		assert.Empty(t, tasks, "No tasks should be created when target not configured")
	})

	t.Run("task resolution for system event fails when key does not exist", func(t *testing.T) {
		system, _ := addDataToDB(ctx, t, tester.repository)

		job, err := tester.eventFactory.SystemLink(ctx, system, "non-existent-key")
		require.NoError(t, err)

		err = waitForJobStatus(ctx, t, tester.db, job.ID.String(), orbital.JobStatusResolveCanceled)
		require.NoError(t, err, "Job should be marked as resolve canceled")

		tasks := getTasksFromDB(t, tester.db, job.ID.String())
		assert.Empty(t, tasks)
	})
}

func TestReconciler_JobTermination(t *testing.T) {
	tests := map[string]struct {
		operatorResult       bool
		operatorError        testutils.MockOperatorError
		expectedJobStatus    orbital.JobStatus
		expectedSystemStatus cmkapi.SystemStatus
		// Asserted against the persisted Event row; empty means skip.
		expectedErrorCode    string
		expectedErrorMessage string
	}{
		"System job terminated successfully": {
			operatorResult:       true,
			expectedJobStatus:    orbital.JobStatusDone,
			expectedSystemStatus: cmkapi.SystemStatusCONNECTED,
		},
		"System job fails with unstructured error message (legacy)": {
			operatorResult:       false,
			expectedJobStatus:    orbital.JobStatusFailed,
			expectedSystemStatus: cmkapi.SystemStatusFAILED,
			// No code → ParseOrbitalError falls back to DefaultErrorCode.
			expectedErrorCode:    constants.DefaultErrorCode,
			expectedErrorMessage: "simulated failure",
		},
		"System job fails with orbital ErrorCode:ErrorMessage format": {
			operatorResult: false,
			operatorError: testutils.MockOperatorError{
				Code:    "PROCESSING_FAILURE",
				Message: "operator could not apply key change",
			},
			expectedJobStatus:    orbital.JobStatusFailed,
			expectedSystemStatus: cmkapi.SystemStatusFAILED,
			expectedErrorCode:    "PROCESSING_FAILURE",
			expectedErrorMessage: "operator could not apply key change",
		},
		"System job fails with version mismatch processing error": {
			operatorResult: false,
			operatorError: testutils.MockOperatorError{
				Code:    "KEY_VERSION_MISMATCH",
				Message: "key version on operator is ahead of CMK",
			},
			expectedJobStatus:    orbital.JobStatusFailed,
			expectedSystemStatus: cmkapi.SystemStatusFAILED,
			expectedErrorCode:    "KEY_VERSION_MISMATCH",
			expectedErrorMessage: "key version on operator is ahead of CMK",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Fresh setup per subtest isolates AMQP queues from prior cases.
			tester := setupTest(t)

			// Given
			ctx := cmkcontext.CreateTenantContext(t.Context(), tester.tenant)

			systemID, keyID := addDataToDB(ctx, t, tester.repository)

			var operatorOpts []testutils.MockOperatorOption
			if tc.operatorError != (testutils.MockOperatorError{}) {
				operatorOpts = append(operatorOpts, testutils.WithFailureError(tc.operatorError))
			}

			usEast1 := tester.config.EventProcessor.Targets[0].AMQP
			operator := testutils.NewMockAMQPOperator(t, 1, tc.operatorResult, amqp.ConnectionInfo{
				URL:    usEast1.URL,
				Target: usEast1.Source, // operator publishes where the reconciler reads
				Source: usEast1.Target, // operator reads where the reconciler publishes
			}, operatorOpts...)

			go func(ctx context.Context) {
				operator.Start(ctx)
			}(t.Context())

			// When
			systemJob, err := tester.eventFactory.SystemLink(ctx, systemID, keyID)
			require.NoError(t, err)

			// Then
			err = waitForJobStatus(ctx, t, tester.db, systemJob.ID.String(), tc.expectedJobStatus)
			require.NoError(t, err, "Job should reach expected status")

			err = waitForSystemStatus(ctx, t, tester.repository, systemID, tc.expectedSystemStatus)
			require.NoError(t, err, "System should reach expected status")

			// "CODE:message" must round-trip into Event.ErrorCode/ErrorMessage.
			if tc.expectedErrorCode != "" {
				event := assertEventError(ctx, t, tester.repository, systemJob.ExternalID)
				assert.Equal(t, tc.expectedErrorCode, event.ErrorCode, "event error code should match operator response")
				assert.Equal(t, tc.expectedErrorMessage, event.ErrorMessage, "event error message should match operator response")
			}
		})
	}

	t.Run("System job canceled because of missing target configuration", func(t *testing.T) {
		tester := setupTest(t)

		// Given
		ctx := cmkcontext.CreateTenantContext(t.Context(), tester.tenant)

		systemID, keyID := addDataToDB(ctx, t, tester.repository)

		updateSystemRegion(ctx, t, tester.repository, systemID, "eu-west-10")
		defer updateSystemRegion(ctx, t, tester.repository, systemID, "eu-east-10")

		// When
		systemJob, err := tester.eventFactory.SystemLink(ctx, systemID, keyID)
		require.NoError(t, err)

		// Then
		err = waitForJobStatus(ctx, t, tester.db, systemJob.ID.String(), orbital.JobStatusResolveCanceled)
		require.NoError(t, err, "Job should be marked as resolve canceled")

		err = waitForSystemStatus(ctx, t, tester.repository, systemID, cmkapi.SystemStatusFAILED)
		require.NoError(t, err, "System should be marked as failed")
	})
}

// assertEventError loads the model.Event row for externalID and requires it to
// exist.
func assertEventError(ctx context.Context, t *testing.T, r repo.Repo, externalID string) *model.Event {
	t.Helper()

	event := &model.Event{Identifier: externalID}
	found, err := r.First(ctx, event, *repo.NewQuery())
	require.NoError(t, err, "failed to read event row")
	require.True(t, found, "event row for job %s should exist", externalID)

	return event
}

// testTimeout is the fail-safe deadline for every reconciliation polling loop.
// Real failures should surface through assertions long before this trips.
const testTimeout = 30 * time.Second

const pollInterval = 100 * time.Millisecond

// waitForTasks polls until expectedCount tasks exist for the job, or the
// deadline elapses. Fatals on timeout so a stuck pipeline fails fast.
func waitForTasks(
	ctx context.Context,
	t *testing.T,
	db *multitenancy.DB,
	jobID string,
	expectedCount int,
) []orbital.Task {
	t.Helper()

	ctx, cancel := context.WithTimeout(ctx, testTimeout)
	defer cancel()

	var tasks []orbital.Task
	for {
		tasks = getTasksFromDB(t, db, jobID)
		if len(tasks) == expectedCount {
			return tasks
		}

		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for %d tasks on job %s; observed %d: %v",
				expectedCount, jobID, len(tasks), ctx.Err())
			return nil
		case <-time.After(pollInterval):
		}
	}
}

// addDataToDB persists one Key + one DISCONNECTED System. For key-level
// fan-out tests use createKeyForTest, which can connect systems across regions.
func addDataToDB(
	ctx context.Context, t *testing.T, r repo.Repo,
) (*model.System, string) {
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
		ID:       keyUUID,
		Name:     uuid.NewString(),
		Provider: "TEST",
		// HYOK routes through fetchAndPopulateVersionInfo, which reads the key's
		// own CryptoAccessData. The non-HYOK path goes through
		// CryptoAccessDataSyncer, which has no crypto certs in the test
		// landscape and would fail with UNSUPPORTED_REGION.
		KeyType:          string(cmkapi.KeyTypeHYOK),
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

	ctx, cancel := context.WithTimeout(ctx, testTimeout)
	defer cancel()

	var (
		lastStatus       orbital.JobStatus
		lastErrorMessage string
	)
	for {
		select {
		case <-ctx.Done():
			// Include last status + error so CI logs explain why the test
			// timed out instead of just "deadline exceeded".
			return fmt.Errorf("%w; job: %s; last status: %q; last error: %q",
				ctx.Err(), jobID, lastStatus, lastErrorMessage)
		default:
			job := getJobFromDB(t, orbitalDB, jobID)
			lastStatus = job.Status
			lastErrorMessage = job.ErrorMessage
			slogctx.Debug(ctx, "Job status", "id", jobID, "current", job.Status, "required", status)

			if job.Status == status {
				return nil
			}
		}

		time.Sleep(pollInterval)
	}
}

func waitForSystemStatus(ctx context.Context, t *testing.T, repository repo.Repo, system *model.System,
	status cmkapi.SystemStatus,
) error {
	t.Helper()

	ctx, cancel := context.WithTimeout(ctx, testTimeout)
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

		time.Sleep(pollInterval)
	}
}
