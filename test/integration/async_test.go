package integration_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/pkg/stdcopy"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/testutils"
	integrationutils "github.com/openkcm/cmk/test/integration/integration_utils"
	"github.com/openkcm/cmk/utils/ptr"
)

func TestAsync(t *testing.T) {
	_, tenants, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{})
	tenant := tenants[0]

	ctlg := plugincatalog.CreateBuiltInPluginRegistry()
	integrationutils.RegisterNoopPlugins(ctlg)
	_, pluginCfgs := testutils.NewTestPlugins(ctlg.Retrieve()...)

	_, grpcClient := testutils.NewGRPCSuite(t)
	defer grpcClient.Close()

	cfg := &config.Config{
		Certificates: config.Certificates{
			ValidityDays: config.MinCertificateValidityDays,
		},
		Database: dbCfg,
		Plugins:  pluginCfgs,
		Services: config.Services{
			Registry: &commoncfg.GRPCClient{
				Enabled: true,
				Address: grpcClient.Target(),
				SecretRef: &commoncfg.SecretRef{
					Type: commoncfg.InsecureSecretType,
				},
			},
		},
	}

	testutils.StartRedis(t, &cfg.Scheduler)

	_ = integrationutils.RunCMKService(t, integrationutils.ServiceConfig{
		Service: integrationutils.TaskWorker,
	}, cfg)
	taskCli := integrationutils.RunCMKService(t, integrationutils.ServiceConfig{
		Service: integrationutils.TaskCLI,
		Args:    []string{"--sleep"},
	}, cfg)

	exitCode, _, err := taskCli.Exec(t.Context(), []string{string(integrationutils.TaskCLI), "invoke", "--task", config.TypeHYOKSync, "--tenants", tenant})
	require.NoError(t, err)
	require.Equal(t, 0, exitCode)

	require.NoError(t, err)
	exitCode, reader, err := taskCli.Exec(t.Context(), []string{string(integrationutils.TaskCLI), "stats", "--queue", "default", "--pending-tasks"})
	require.NoError(t, err)
	require.Equal(t, 0, exitCode)

	// TODO: Why is the info in stderr?
	var stdout, stderr bytes.Buffer
	_, err = stdcopy.StdCopy(&stdout, &stderr, reader)
	require.NoError(t, err)
	require.Contains(t, stderr.String(), config.TypeHYOKSync)

	assert.Eventually(t, func() bool {
		stdout.Reset()
		stderr.Reset()
		exitCode, reader, err = taskCli.Exec(t.Context(), []string{string(integrationutils.TaskCLI), "stats", "--queue", "default", "--queue-info"})
		if err != nil {
			return false
		}
		_, err = stdcopy.StdCopy(&stdout, &stderr, reader)
		if err != nil {
			return false
		}
		return err == nil && exitCode == 0 && strings.Contains(stderr.String(), `"Processed": 1`)
	}, 5*time.Second, 100*time.Millisecond)
}

// This test checks that children tasks are created for the ammount of tenants
// If tenant count is 10, for 1 task there should be at least 11 tasks (original + one per tenant)
func TestAsyncFanout(t *testing.T) {
	nTenants := 10
	_, _, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{}, testutils.WithGenerateTenants(nTenants))

	ctlg := plugincatalog.CreateBuiltInPluginRegistry()
	integrationutils.RegisterNoopPlugins(ctlg)
	_, pluginCfgs := testutils.NewTestPlugins(ctlg.Retrieve()...)

	_, grpcClient := testutils.NewGRPCSuite(t)
	defer grpcClient.Close()

	cfg := &config.Config{
		Certificates: config.Certificates{
			ValidityDays: config.MinCertificateValidityDays,
		},
		Database: dbCfg,
		Plugins:  pluginCfgs,
		Services: config.Services{
			Registry: &commoncfg.GRPCClient{
				Enabled: true,
				Address: grpcClient.Target(),
				SecretRef: &commoncfg.SecretRef{
					Type: commoncfg.InsecureSecretType,
				},
			},
		},
		Scheduler: config.Scheduler{
			Tasks: []config.Task{
				{
					TaskType: config.TypeHYOKSync,
					Enabled:  ptr.PointTo(true),
					Cronspec: "@every 1s",
					Retries:  ptr.PointTo(3),
					TimeOut:  5 * time.Minute,
					FanOutTask: &config.FanOutTask{
						Enabled: true,
						Retries: ptr.PointTo(0),
						TimeOut: 5 * time.Minute,
					},
				},
			},
		},
	}

	testutils.StartRedis(t, &cfg.Scheduler)

	_ = integrationutils.RunCMKService(t, integrationutils.ServiceConfig{
		Service: integrationutils.TaskWorker,
	}, cfg)
	_ = integrationutils.RunCMKService(t, integrationutils.ServiceConfig{
		Service: integrationutils.TaskScheduler,
	}, cfg)
	taskCli := integrationutils.RunCMKService(t, integrationutils.ServiceConfig{
		Service: integrationutils.TaskCLI,
		Args:    []string{"--sleep"},
	}, cfg)

	var stdout, stderr bytes.Buffer
	assert.Eventually(t, func() bool {
		stdout.Reset()
		stderr.Reset()
		exitCode, reader, err := taskCli.Exec(t.Context(), []string{string(integrationutils.TaskCLI), "stats", "--queue", "default", "--queue-info"})
		if err != nil {
			return false
		}
		_, err = stdcopy.StdCopy(&stdout, &stderr, reader)
		if err != nil {
			return false
		}

		// Extract the Processed value
		var stats struct {
			Processed int `json:"Processed"`
		}
		if err := json.Unmarshal(stderr.Bytes(), &stats); err != nil {
			return false
		}

		return exitCode == 0 && stats.Processed > 0 && stats.Processed%nTenants != 0 && stats.Processed > nTenants+1
	}, 15*time.Second, 100*time.Millisecond)
}
