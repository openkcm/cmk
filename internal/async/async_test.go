package async_test

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/async"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/utils/ptr"
)

type MockTenantTask struct{}

func (t *MockTenantTask) ProcessTask(_ context.Context, _ *asynq.Task) error {
	return nil
}

func (t *MockTenantTask) TaskType() string {
	return config.TypeHYOKSync
}

func (t *MockTenantTask) FanOutFunc() async.FanOutFunc {
	return async.TenantFanOut
}

func (t *MockTenantTask) TenantQuery() *repo.Query {
	return repo.NewQuery()
}

func defaultAsyncApp(t *testing.T, cfg *config.Config) *async.App {
	t.Helper()

	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	bytes, err := json.Marshal(cfg)
	assert.NoError(t, err)

	err = os.WriteFile(file, bytes, 0o600)
	assert.NoError(t, err)

	cfg, err = config.LoadConfig(
		commoncfg.WithPaths(dir),
	)
	assert.NoError(t, err)

	app, err := async.New(cfg)
	assert.NoError(t, err)

	return app
}

func defaultRedisConfig(tlsFiles testutils.TLSFiles) config.Redis {
	return config.Redis{
		Port: "6379",
		SecretRef: commoncfg.SecretRef{
			Type: commoncfg.MTLSSecretType,
			MTLS: commoncfg.MTLS{
				Cert: commoncfg.SourceRef{
					Source: commoncfg.FileSourceValue,
					File: commoncfg.CredentialFile{
						Path:   tlsFiles.ClientCertPath,
						Format: commoncfg.BinaryFileFormat,
					},
				},
				CertKey: commoncfg.SourceRef{
					Source: commoncfg.FileSourceValue,
					File: commoncfg.CredentialFile{
						Path:   tlsFiles.ClientKeyPath,
						Format: commoncfg.BinaryFileFormat,
					},
				},
				ServerCA: &commoncfg.SourceRef{
					Source: commoncfg.FileSourceValue,
					File: commoncfg.CredentialFile{
						Path:   tlsFiles.ServerCertPath,
						Format: commoncfg.BinaryFileFormat,
					},
				},
			},
		},
		ACL: config.RedisACL{
			Enabled: true,
			Username: commoncfg.SourceRef{
				Source: commoncfg.EmbeddedSourceValue,
				Value:  "default",
			},
			Password: commoncfg.SourceRef{
				Source: commoncfg.EmbeddedSourceValue,
				Value:  "password123",
			},
		},
		Host: commoncfg.SourceRef{
			Source: commoncfg.EmbeddedSourceValue,
			Value:  "localhost",
		},
	}
}

func TestAsyncNew(t *testing.T) {
	// Generate test TLS files using testutils
	tlsFiles := testutils.CreateTLSFiles(t)
	defaultCfg := defaultRedisConfig(tlsFiles)

	tests := []struct {
		name             string
		taskQueueCfg     config.Redis
		expectError      bool
		expectedAddr     string
		expectedPassword string
		expectedUsername string
		validateTLS      bool
		validateCA       bool
		aclEnabled       bool
	}{
		{
			name:             "Valid MTLS configuration",
			taskQueueCfg:     defaultCfg,
			expectError:      false,
			expectedAddr:     "localhost:6379",
			expectedPassword: "password123",
			expectedUsername: "default",
			validateTLS:      true,
			validateCA:       true,
			aclEnabled:       true,
		},
		{
			name: "Valid MTLS no server CA",
			taskQueueCfg: func() config.Redis {
				cfg := defaultCfg
				cfg.SecretRef.MTLS.ServerCA = nil // Remove ServerCA

				return cfg
			}(),
			expectError:      false,
			expectedAddr:     "localhost:6379",
			expectedPassword: "password123",
			expectedUsername: "default",
			validateTLS:      true,
			validateCA:       false,
			aclEnabled:       true,
		},
		{
			name: "Valid Insecure configuration",
			taskQueueCfg: config.Redis{
				Port: "6379",
				SecretRef: commoncfg.SecretRef{
					Type: commoncfg.InsecureSecretType,
				},
				ACL: config.RedisACL{
					Enabled: true,
					Username: commoncfg.SourceRef{
						Source: commoncfg.EmbeddedSourceValue,
						Value:  "default",
					},
					Password: commoncfg.SourceRef{
						Source: commoncfg.EmbeddedSourceValue,
						Value:  "password123",
					},
				},
				Host: commoncfg.SourceRef{
					Source: commoncfg.EmbeddedSourceValue,
					Value:  "localhost",
				},
			},
			expectError:      false,
			expectedAddr:     "localhost:6379",
			expectedPassword: "password123",
			expectedUsername: "default",
			validateTLS:      false,
			validateCA:       false,
		},
		{
			name: "Valid MTLS configuration with no ACL",
			taskQueueCfg: func() config.Redis {
				cfg := defaultCfg
				cfg.ACL.Enabled = false

				return cfg
			}(),
			expectError:      false,
			expectedAddr:     "localhost:6379",
			expectedPassword: "",
			expectedUsername: "",
			validateTLS:      true,
			validateCA:       true,
			aclEnabled:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, err := async.New(
				&config.Config{
					Scheduler: config.Scheduler{
						TaskQueue: tt.taskQueueCfg,
					},
					Database: config.Database{
						Host: commoncfg.SourceRef{
							Source: commoncfg.EmbeddedSourceValue,
							Value:  "localhost",
						},
						User: commoncfg.SourceRef{
							Source: commoncfg.EmbeddedSourceValue,
							Value:  "postgres",
						},
						Secret: commoncfg.SourceRef{
							Source: commoncfg.EmbeddedSourceValue,
							Value:  "secret",
						},
						Name: "cmk",
						Port: "5433",
					},
				},
			)
			assert.NotNil(t, app)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			result := app.GetTaskQueueCfg()

			require.NoError(t, err)
			assert.Equal(t, tt.expectedAddr, result.Addr)
			assert.Equal(t, tt.expectedPassword, result.Password)
			assert.Equal(t, tt.expectedUsername, result.Username)

			if tt.taskQueueCfg.SecretRef.Type == commoncfg.MTLSSecretType {
				require.NotNil(t, result.TLSConfig)
				assert.Len(t, result.TLSConfig.Certificates, 1)
				assert.Equal(t, uint16(tls.VersionTLS12), result.TLSConfig.MinVersion)

				ca := tt.taskQueueCfg.SecretRef.MTLS.ServerCA

				if ca != nil && ca.Source != "" {
					assert.NotNil(t, result.TLSConfig.RootCAs)
				}
			}
		})
	}
}

func TestGetFanOutOpts(t *testing.T) {
	tlsFiles := testutils.CreateTLSFiles(t)
	defaultCfg := defaultRedisConfig(tlsFiles)
	cfg := &config.Config{
		Scheduler: config.Scheduler{
			TaskQueue: defaultCfg,
			Tasks: []config.Task{
				{
					Enabled:  ptr.PointTo(true),
					Cronspec: "* * * * *",
					TaskType: config.TypeWorkflowCleanup,
				},
				{
					Enabled:  ptr.PointTo(true),
					Cronspec: "* * * * *",
					TaskType: config.TypeWorkflowExpire,
					FanOutTask: &config.FanOutTask{
						Enabled: false,
					},
				},
				{
					Enabled:  ptr.PointTo(true),
					Cronspec: "* * * * *",
					TaskType: config.TypeKeystorePool,
					FanOutTask: &config.FanOutTask{
						Enabled: true,
					},
				},
				{
					Enabled:  ptr.PointTo(true),
					Cronspec: "* * * * *",
					TaskType: config.TypeHYOKSync,
				},
			},
		},
		Database: testutils.TestDB,
		Certificates: config.Certificates{
			ValidityDays: 7,
		},
	}
	app := defaultAsyncApp(t, cfg)

	t.Run("Should return false on non existing task", func(t *testing.T) {
		ok, _ := app.GetFanOutOpts("no-fanout")
		assert.False(t, ok)
	})

	t.Run("Should return false on nil fanout", func(t *testing.T) {
		ok, _ := app.GetFanOutOpts(config.TypeWorkflowCleanup)
		assert.False(t, ok)
	})

	t.Run("Should return false on fanout disabled", func(t *testing.T) {
		ok, _ := app.GetFanOutOpts(config.TypeWorkflowCleanup)
		assert.False(t, ok)
	})

	t.Run("Should return true on fanout enabled", func(t *testing.T) {
		ok, _ := app.GetFanOutOpts(config.TypeKeystorePool)
		assert.True(t, ok)
	})
}

func TestRegisterTasks(t *testing.T) {
	tlsFiles := testutils.CreateTLSFiles(t)
	defaultCfg := defaultRedisConfig(tlsFiles)
	_, _, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{})
	cfg := &config.Config{
		Scheduler: config.Scheduler{
			TaskQueue: defaultCfg,
			Tasks: []config.Task{
				{
					Enabled:  ptr.PointTo(true),
					Cronspec: "* * * * *",
					TaskType: config.TypeHYOKSync,
					FanOutTask: &config.FanOutTask{
						Enabled: true,
					},
				},
			},
		},
		Database: dbCfg,
		Certificates: config.Certificates{
			ValidityDays: 7,
		},
	}
	testutils.StartRedis(t, &cfg.Scheduler)
	app := defaultAsyncApp(t, cfg)
	task := MockTenantTask{}
	app.RegisterTasks(
		t.Context(),
		[]async.TaskHandler{&task},
	)
}
