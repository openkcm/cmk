//nolint:contextcheck
package eventprocessor_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/bartventer/gorm-multitenancy/middleware/nethttp/v8"
	"github.com/bartventer/gorm-multitenancy/v8/pkg/driver"
	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/orbital"
	"github.com/openkcm/orbital/client/amqp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	_ "github.com/lib/pq"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk-core/internal/api/cmkapi"
	"github.com/openkcm/cmk-core/internal/config"
	eventprocessor "github.com/openkcm/cmk-core/internal/event-processor"
	eventProto "github.com/openkcm/cmk-core/internal/event-processor/proto"
	"github.com/openkcm/cmk-core/internal/grpc/catalog"
	"github.com/openkcm/cmk-core/internal/manager"
	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/internal/repo"
	sqlPkg "github.com/openkcm/cmk-core/internal/repo/sql"
	"github.com/openkcm/cmk-core/internal/testutils"
	integrationutils "github.com/openkcm/cmk-core/test/integration_utils"
	cmkcontext "github.com/openkcm/cmk-core/utils/context"
	"github.com/openkcm/cmk-core/utils/ptr"
)

type TestSuite struct {
	rabbitMQURL string
	repository  repo.Repo
	config      *config.Config
	orbitalDB   *multitenancy.DB
	tenant      string
	reconciler  *eventprocessor.CryptoReconciler
}

//nolint:funlen
func setupTest(t *testing.T) *TestSuite {
	t.Helper()

	ctx := t.Context()

	rabbitMQ := integrationutils.StartRabbitMQ(t)
	rabbitMQURL, err := rabbitMQ.AmqpURL(ctx)
	require.NoError(t, err)

	dbConf := integrationutils.DB
	_, dbConf.Port = integrationutils.StartPostgresSQL(t)

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

	db, tenants := testutils.NewTestDB(
		t,
		testutils.TestDBConfig{
			Models: []driver.TenantTabler{
				&testutils.TestModel{},
				&model.Key{},
				&model.System{},
				&model.Tenant{},
			},
			RequiresMultitenancyOrShared: true,
		},
		testutils.WithDatabase(cfg.Database),
	)

	// NewTestDB created a new database instance for this test
	err = db.Raw("SELECT current_database()").Scan(&cfg.Database.Name).Error
	require.NoError(t, err)

	ctlg, err := catalog.New(t.Context(), cfg)
	require.NoError(t, err)

	r := sqlPkg.NewRepository(db)

	reconciler, err := eventprocessor.NewCryptoReconciler(
		ctx,
		&cfg,
		r,
		ctlg,
		eventprocessor.WithExecInterval(5*time.Millisecond),
		eventprocessor.WithConfirmJobAfter(10*time.Millisecond),
	)
	require.NoError(t, err)
	require.NoError(t, reconciler.Start(ctx))

	return &TestSuite{
		rabbitMQURL: rabbitMQURL,
		repository:  r,
		orbitalDB:   db,
		config:      &cfg,
		tenant:      tenants[0],
		reconciler:  reconciler,
	}
}

func addDataToDB(ctx context.Context, t *testing.T, repo repo.Repo) (string, string) {
	t.Helper()

	tenantManager := manager.NewTenantManager(repo)

	tenant, err := tenantManager.GetTenant(ctx)
	require.NoError(t, err)
	assert.Equal(t, "tenant0", tenant.SchemaName)

	keyUUID := uuid.New()

	data := map[string]map[string]interface{}{
		"us-east-1": {
			"trustAnchorArn": "arn:aws:iam::123456789012:role/TrustAnchor",
			"profileArn":     "arn:aws:iam::123456789012:role/Profile",
			"roleArn":        "arn:aws:iam::123456789012:role/Role",
		},
	}
	bytes, err := json.Marshal(data)
	require.NoError(t, err)

	err = repo.Create(ctx, &model.Key{
		ID:               keyUUID,
		Name:             "test key",
		Provider:         "TEST",
		NativeID:         ptr.PointTo("arn:aws:kms:us-east-1:123456789012:key/12345678-90ab-cdef-1234-567890abcdef"),
		CryptoAccessData: bytes,
	})
	require.NoError(t, err)

	systemUUID := uuid.New()
	err = repo.Create(ctx, &model.System{
		ID:         systemUUID,
		Identifier: "externalID",
		Region:     "us-east-1",
		Status:     cmkapi.SystemStatusPROCESSING,
		Type:       "SYSTEM",
	})
	require.NoError(t, err)

	return systemUUID.String(), keyUUID.String()
}

func deleteDataFromDB(ctx context.Context, t *testing.T, repository repo.Repo, systemID string, keyID string) {
	t.Helper()

	ck := repo.NewCompositeKey().Where(repo.IDField, systemID)
	query := repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(ck),
	)
	_, err := repository.Delete(ctx, &model.System{}, *query)
	require.NoError(t, err)

	ck = repo.NewCompositeKey().Where(repo.IDField, keyID)
	query = repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(ck),
	)
	_, err = repository.Delete(ctx, &model.Key{}, *query)
	require.NoError(t, err)
}

func updateSystemRegion(ctx context.Context, t *testing.T, repository repo.Repo, systemID, region string) {
	t.Helper()

	var system model.System

	ck := repo.NewCompositeKey().Where(repo.IDField, systemID)
	query := repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(ck),
	)

	_, err := repository.First(ctx, &system, *query)
	require.NoError(t, err)

	system.Region = region

	ck = repo.NewCompositeKey().Where(repo.IDField, system.ID)
	query = repo.NewQuery().Where(
		repo.NewCompositeKeyGroup(ck),
	).Update(repo.RegionField)

	_, err = repository.Patch(ctx, &system, *query)
	require.NoError(t, err)
}

func getJobFromDB(t *testing.T, orbitalDB *multitenancy.DB, jobID string) *orbital.Job {
	t.Helper()

	var job orbital.Job

	err := orbitalDB.WithTenant(t.Context(), "orbital", func(tx *multitenancy.DB) error {
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

func getTasksFromDB(t *testing.T, orbitalDB *multitenancy.DB, jobID string) []orbital.Task {
	t.Helper()

	var tasks []orbital.Task

	err := orbitalDB.WithTenant(t.Context(), "orbital", func(tx *multitenancy.DB) error {
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

func TestReconciler_JobCreation(t *testing.T) {
	ts := setupTest(t)

	ctx := cmkcontext.CreateTenantContext(t.Context(), ts.tenant)

	t.Run("system link job creation", func(t *testing.T) {
		job, err := ts.reconciler.SystemLink(ctx, "systemID", "keyID")
		require.NoError(t, err)

		var jobData eventprocessor.SystemActionJobData
		assert.NoError(t, json.Unmarshal(job.Data, &jobData))

		assert.Equal(t, "SYSTEM_LINK", job.Type)
		assert.Equal(t, ts.tenant, jobData.TenantID)
		assert.Equal(t, "keyID", jobData.KeyID)
		assert.Equal(t, "systemID", jobData.SystemID)
	})

	t.Run("system link job creation failure because context does not have tenant ID", func(t *testing.T) {
		job, err := ts.reconciler.SystemLink(t.Context(), "systemID", "keyID")
		require.Error(t, err)
		assert.Equal(t, orbital.Job{}, job)
	})

	t.Run("system unlink job creation", func(t *testing.T) {
		job, err := ts.reconciler.SystemUnlink(
			context.WithValue(ctx, nethttp.TenantKey, ts.tenant), "systemID", "keyID")
		require.NoError(t, err)

		var jobData eventprocessor.SystemActionJobData
		assert.NoError(t, json.Unmarshal(job.Data, &jobData))

		assert.Equal(t, "SYSTEM_UNLINK", job.Type)
		assert.Equal(t, ts.tenant, jobData.TenantID)
		assert.Equal(t, "keyID", jobData.KeyID)
		assert.Equal(t, "systemID", jobData.SystemID)
	})

	t.Run("key enable job creation", func(t *testing.T) {
		job, err := ts.reconciler.KeyEnable(ctx, "keyID")
		require.NoError(t, err)

		var jobData eventprocessor.KeyActionJobData
		assert.NoError(t, json.Unmarshal(job.Data, &jobData))

		assert.Equal(t, "KEY_ENABLE", job.Type)
		assert.Equal(t, ts.tenant, jobData.TenantID)
		assert.Equal(t, "keyID", jobData.KeyID)
	})

	t.Run("key enable job creation failure because context does not have tenant ID", func(t *testing.T) {
		job, err := ts.reconciler.KeyEnable(t.Context(), "keyID")
		require.Error(t, err)
		assert.Equal(t, orbital.Job{}, job)
	})

	t.Run("key disable job creation", func(t *testing.T) {
		job, err := ts.reconciler.KeyDisable(ctx, "keyID")
		require.NoError(t, err)

		var jobData eventprocessor.KeyActionJobData
		assert.NoError(t, json.Unmarshal(job.Data, &jobData))

		assert.Equal(t, "KEY_DISABLE", job.Type)
		assert.Equal(t, ts.tenant, jobData.TenantID)
		assert.Equal(t, "keyID", jobData.KeyID)
	})

	t.Run("key disable job creation failure because context does not have tenant ID", func(t *testing.T) {
		job, err := ts.reconciler.KeyDisable(t.Context(), "keyID")
		require.Error(t, err)
		assert.Equal(t, orbital.Job{}, job)
	})
}

func TestReconciler_JobConfirmation(t *testing.T) {
	ts := setupTest(t)

	ctx := cmkcontext.CreateTenantContext(t.Context(), ts.tenant)

	systemID, keyID := addDataToDB(ctx, t, ts.repository)

	t.Run("confirm SYSTEM_LINK job", func(t *testing.T) {
		job, err := ts.reconciler.SystemLink(ctx, systemID, keyID)
		require.NoError(t, err)

		jobFromDB := getJobFromDB(t, ts.orbitalDB, job.ID.String())

		assert.Equal(t, job.Type, jobFromDB.Type)
		assert.Equal(t, job.ID.String(), jobFromDB.ID.String())

		assert.NotEqual(t, orbital.JobStatusConfirming, jobFromDB.Status)
	})

	t.Run("confirm SYSTEM_UNLINK job", func(t *testing.T) {
		job, err := ts.reconciler.SystemUnlink(context.WithValue(ctx, nethttp.TenantKey, ts.tenant), systemID, keyID)
		require.NoError(t, err)

		jobFromDB := getJobFromDB(t, ts.orbitalDB, job.ID.String())
		require.NoError(t, err)

		assert.Equal(t, job.Type, jobFromDB.Type)
		assert.Equal(t, job.ID.String(), jobFromDB.ID.String())

		assert.NotEqual(t, orbital.JobStatusConfirming, jobFromDB.Status)
	})

	t.Run("key job is confirmed", func(t *testing.T) {
		job, err := ts.reconciler.KeyEnable(ctx, keyID)
		require.NoError(t, err)

		jobFromDB := getJobFromDB(t, ts.orbitalDB, job.ID.String())

		assert.Equal(t, job.Type, jobFromDB.Type)
		assert.Equal(t, job.ID.String(), jobFromDB.ID.String())

		assert.NotEqual(t, orbital.JobStatusConfirming, jobFromDB.Status)
	})
}

func TestReconciler_TaskResolution(t *testing.T) {
	ts := setupTest(t)

	ctx := cmkcontext.CreateTenantContext(t.Context(), ts.tenant)

	systemID, keyID := addDataToDB(ctx, t, ts.repository)

	t.Run("key action", func(t *testing.T) {
		testCases := []struct {
			name     string
			jobType  string
			taskType eventProto.TaskType
		}{
			{
				name:     "KEY_ENABLE creates tasks for all targets",
				jobType:  "KEY_ENABLE",
				taskType: eventProto.TaskType_KEY_ENABLE,
			},
			{
				name:     "KEY_DISABLE creates tasks for all targets",
				jobType:  "KEY_DISABLE",
				taskType: eventProto.TaskType_KEY_DISABLE,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				var (
					job orbital.Job
					err error
				)

				if tc.jobType == "KEY_ENABLE" {
					job, err = ts.reconciler.KeyEnable(ctx, keyID)
				} else {
					job, err = ts.reconciler.KeyDisable(ctx, keyID)
				}

				require.NoError(t, err)

				var tasks []orbital.Task
				for {
					tasks = getTasksFromDB(t, ts.orbitalDB, job.ID.String())

					if len(tasks) == len(ts.config.EventProcessor.Targets) {
						break
					}
				}

				for _, task := range tasks {
					assert.Equal(t, tc.jobType, task.Type)

					var taskData eventProto.Data

					err := proto.Unmarshal(task.Data, &taskData)
					require.NoError(t, err)

					assert.Equal(t, tc.taskType, taskData.GetTaskType())
					assert.NotNil(t, taskData.GetKeyAction())
					assert.Equal(t, keyID, taskData.GetKeyAction().GetKeyId())
					assert.Equal(t, ts.tenant, taskData.GetKeyAction().GetTenantId())
				}
			})
		}
	})

	t.Run("SYSTEM_LINK creates task for system's region only", func(t *testing.T) {
		job, err := ts.reconciler.SystemLink(ctx, systemID, keyID)
		require.NoError(t, err)

		var tasks []orbital.Task

		for {
			tasks = getTasksFromDB(t, ts.orbitalDB, job.ID.String())

			if len(tasks) == 1 {
				break
			}
		}

		task := tasks[0]
		assert.Equal(t, "SYSTEM_LINK", task.Type)
		assert.Equal(t, "us-east-1", task.Target)

		var taskData eventProto.Data

		err = proto.Unmarshal(task.Data, &taskData)
		require.NoError(t, err)

		assert.Equal(t, eventProto.TaskType_SYSTEM_LINK, taskData.GetTaskType())
		assert.NotNil(t, taskData.GetSystemAction())
		assert.Equal(t, "externalID", taskData.GetSystemAction().GetSystemId())
		assert.Equal(t, "us-east-1", taskData.GetSystemAction().GetSystemRegion())
		assert.Equal(t, "system", taskData.GetSystemAction().GetSystemType())
		assert.Empty(t, taskData.GetSystemAction().GetKeyIdFrom())
		assert.Equal(t, keyID, taskData.GetSystemAction().GetKeyIdTo())
		assert.Equal(t, "test", taskData.GetSystemAction().GetKeyProvider())
		assert.Equal(t, ts.tenant, taskData.GetSystemAction().GetTenantId())
	})

	t.Run("SYSTEM_UNLINK creates task for system's region only", func(t *testing.T) {
		job, err := ts.reconciler.SystemUnlink(context.WithValue(ctx, nethttp.TenantKey, ts.tenant), systemID, keyID)
		require.NoError(t, err)

		var tasks []orbital.Task

		for {
			tasks = getTasksFromDB(t, ts.orbitalDB, job.ID.String())
			require.NoError(t, err)

			if len(tasks) == 1 {
				break
			}
		}

		task := tasks[0]
		assert.Equal(t, "SYSTEM_UNLINK", task.Type)
		assert.Equal(t, "us-east-1", task.Target)

		var taskData eventProto.Data

		err = proto.Unmarshal(task.Data, &taskData)
		require.NoError(t, err)

		assert.Equal(t, eventProto.TaskType_SYSTEM_UNLINK, taskData.GetTaskType())
		assert.NotNil(t, taskData.GetSystemAction())
		assert.Equal(t, "externalID", taskData.GetSystemAction().GetSystemId())
		assert.Equal(t, "us-east-1", taskData.GetSystemAction().GetSystemRegion())
		assert.Equal(t, "system", taskData.GetSystemAction().GetSystemType())
		assert.Equal(t, keyID, taskData.GetSystemAction().GetKeyIdFrom())
		assert.Empty(t, taskData.GetSystemAction().GetKeyIdTo())
		assert.Equal(t, "test", taskData.GetSystemAction().GetKeyProvider())
		assert.Equal(t, ts.tenant, taskData.GetSystemAction().GetTenantId())
	})
}

func TestReconciler_TaskResolution_Errors(t *testing.T) {
	ts := setupTest(t)

	ctx := cmkcontext.CreateTenantContext(t.Context(), ts.tenant)

	systemID, keyID := addDataToDB(ctx, t, ts.repository)

	t.Run("SYSTEM_LINK fails when target not configured for system region", func(t *testing.T) {
		updateSystemRegion(ctx, t, ts.repository, systemID, "eu-west-10")
		defer updateSystemRegion(ctx, t, ts.repository, systemID, "eu-east-10")

		job, err := ts.reconciler.SystemLink(ctx, systemID, keyID)
		require.NoError(t, err)

		for {
			j := getJobFromDB(t, ts.orbitalDB, job.ID.String())

			if j.Status == orbital.JobStatusResolveCanceled {
				break
			}
		}

		tasks := getTasksFromDB(t, ts.orbitalDB, job.ID.String())
		assert.Empty(t, tasks, "No tasks should be created when target not configured")
	})

	t.Run("task resolution for system event fails when key does not exist", func(t *testing.T) {
		job, err := ts.reconciler.SystemLink(ctx, systemID, "non-existent-key")
		require.NoError(t, err)

		for {
			j := getJobFromDB(t, ts.orbitalDB, job.ID.String())

			if j.Status == orbital.JobStatusResolveCanceled {
				break
			}
		}

		tasks := getTasksFromDB(t, ts.orbitalDB, job.ID.String())
		assert.Empty(t, tasks)
	})
}

func TestReconciler_JobTermination(t *testing.T) {
	ts := setupTest(t)

	t.Run("System job terminated successfully", func(t *testing.T) {
		ctx := cmkcontext.CreateTenantContext(t.Context(), ts.tenant)

		systemID, keyID := addDataToDB(ctx, t, ts.repository)
		defer deleteDataFromDB(ctx, t, ts.repository, systemID, keyID)

		operator, err := testutils.NewTestAMQPOperator(ctx, 1, true, amqp.ConnectionInfo{
			URL:    ts.rabbitMQURL,
			Target: "us-east-1-responses",
			Source: "us-east-1-tasks",
		})
		require.NoError(t, err)

		systemJob, err := ts.reconciler.SystemLink(ctx, systemID, keyID)
		require.NoError(t, err)

		go func() {
			_ = operator.Start(ctx)
		}()

		var job *orbital.Job
		for {
			job = getJobFromDB(t, ts.orbitalDB, systemJob.ID.String())

			if job.Status == orbital.JobStatusDone {
				break
			}
		}

		operator.Stop()

		var system model.System

		ck := repo.NewCompositeKey().Where(repo.IDField, systemID)
		query := repo.NewQuery().Where(
			repo.NewCompositeKeyGroup(ck),
		)

		for {
			_, err = ts.repository.First(ctx, &system, *query)
			require.NoError(t, err)

			if system.Status == cmkapi.SystemStatusCONNECTED {
				break
			}
		}

		assert.Equal(t, orbital.JobStatusDone, job.Status, "Job should be marked as done")
		assert.Equal(t, cmkapi.SystemStatusCONNECTED, system.Status)
	})

	t.Run("System job failure", func(t *testing.T) {
		ctx := cmkcontext.CreateTenantContext(t.Context(), ts.tenant)
		systemID, keyID := addDataToDB(ctx, t, ts.repository)
		systemJob, err := ts.reconciler.SystemLink(ctx, systemID, keyID)
		require.NoError(t, err)

		operator, err := testutils.NewTestAMQPOperator(ctx, 1, false, amqp.ConnectionInfo{
			URL:    ts.rabbitMQURL,
			Target: "us-east-1-responses",
			Source: "us-east-1-tasks",
		})
		require.NoError(t, err)

		go func() {
			err := operator.Start(ctx)
			assert.NoError(t, err)
		}()

		var job *orbital.Job
		for {
			job = getJobFromDB(t, ts.orbitalDB, systemJob.ID.String())

			if job.Status == orbital.JobStatusFailed {
				break
			}
		}

		operator.Stop()

		var system model.System

		ck := repo.NewCompositeKey().Where(repo.IDField, systemID)

		query := repo.NewQuery().Where(
			repo.NewCompositeKeyGroup(ck),
		)
		for {
			_, err = ts.repository.First(ctx, &system, *query)
			require.NoError(t, err)

			if system.Status == cmkapi.SystemStatusFAILED {
				break
			}
		}

		assert.Equal(t, orbital.JobStatusFailed, job.Status, "Job should be marked as failed")
		assert.Equal(t, cmkapi.SystemStatusFAILED, system.Status)
	})
}
