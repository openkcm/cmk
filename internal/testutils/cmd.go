package testutils

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/openkcm/cmk/internal/config"
)

func SetupTestBinary(tb testing.TB, statusServer string, f func() error) {
	tb.Helper()

	errCh := make(chan error, 1)
	go func() {
		errCh <- f()
	}()

	WaitForServer(tb, statusServer)

	select {
	case err := <-errCh:
		assert.NoError(tb, err)
	default:
	}
}

func WaitForServer(tb testing.TB, statusServer string) {
	tb.Helper()

	url := "http://" + statusServer + "/probe/liveness"
	assert.Eventually(tb, func() bool {
		req, err := http.NewRequestWithContext(tb.Context(), http.MethodGet, url, nil)
		if err != nil {
			return false
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 5*time.Second, 100*time.Millisecond)
}

// CreateTestConfigFile creates a minimal test config file with the given status server port
func CreateTestConfigFile(t *testing.T) *config.Config {
	t.Helper()

	_, _, dbCfg := NewTestDB(t, TestDBConfig{})
	_, amqpCfg := NewAMQPClient(t, AMQPCfg{})

	port, err := GetFreePort()
	require.NoError(t, err, "failed to get free port")

	cfg := &config.Config{
		BaseConfig: commoncfg.BaseConfig{
			Application: commoncfg.Application{
				Name: "test",
			},
			Status: commoncfg.Status{
				Enabled: true,
				Address: fmt.Sprintf("localhost:%d", port),
			},
			Logger: commoncfg.Logger{
				Level: "error",
			},
		},
		Database: dbCfg,
		Certificates: config.Certificates{
			ValidityDays: config.MinCertificateValidityDays,
		},
		Services: config.Services{
			Registry:       TestRegistryConfig,
			SessionManager: TestSessionManagerConfig,
		},
		TenantManager: config.TenantManager{
			SecretRef: commoncfg.SecretRef{
				Type: commoncfg.InsecureSecretType,
			},
			AMQP: amqpCfg,
		},
		Plugins: NoopPluginConfigs(),
	}

	StartRedis(t, &cfg.Scheduler)

	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	data, err := yaml.Marshal(cfg)
	require.NoError(t, err, "failed to marshal config")

	err = os.WriteFile("config.yaml", data, 0o600)
	require.NoError(t, err, "failed to write config file")

	return cfg
}
