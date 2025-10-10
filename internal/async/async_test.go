package async_test

import (
	"crypto/tls"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/async"
	conf "github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/testutils"
)

func defaultRedisConfig(tlsFiles testutils.TLSFiles) conf.Redis {
	return conf.Redis{
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
				ServerCA: commoncfg.SourceRef{
					Source: commoncfg.FileSourceValue,
					File: commoncfg.CredentialFile{
						Path:   tlsFiles.ServerCertPath,
						Format: commoncfg.BinaryFileFormat,
					},
				},
			},
		},
		ACL: conf.RedisACL{
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
		taskQueueCfg     conf.Redis
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
			taskQueueCfg: func() conf.Redis {
				cfg := defaultCfg
				cfg.SecretRef.MTLS.ServerCA = commoncfg.SourceRef{} // Remove ServerCA

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
			taskQueueCfg: conf.Redis{
				Port: "6379",
				SecretRef: commoncfg.SecretRef{
					Type: commoncfg.InsecureSecretType,
				},
				ACL: conf.RedisACL{
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
			taskQueueCfg: func() conf.Redis {
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
				&conf.Config{
					Scheduler: conf.Scheduler{
						TaskQueue: tt.taskQueueCfg,
					},
					Database: conf.Database{
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

				if tt.taskQueueCfg.SecretRef.MTLS.ServerCA.Source != "" {
					assert.NotNil(t, result.TLSConfig.RootCAs)
				}
			}
		})
	}
}
