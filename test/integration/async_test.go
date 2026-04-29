package integration_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/pkg/stdcopy"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"

	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"

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

	var wg sync.WaitGroup
	var taskCli testcontainers.Container

	wg.Add(2)
	go func() {
		defer wg.Done()
		integrationutils.RunCMKService(t, integrationutils.ServiceConfig{
			Service: integrationutils.TaskWorker,
		}, cfg)
	}()
	go func() {
		defer wg.Done()
		taskCli = integrationutils.RunCMKService(t, integrationutils.ServiceConfig{
			Service: integrationutils.TaskCLI,
			Args:    []string{"--sleep"},
		}, cfg)
	}()
	wg.Wait()

	exitCode, _, err := taskCli.Exec(t.Context(), []string{string(integrationutils.TaskCLI), "invoke", "--task", config.TypeHYOKSync, "--tenants", tenant})
	require.NoError(t, err)
	require.Equal(t, 0, exitCode)

	require.NoError(t, err)
	exitCode, reader, err := taskCli.Exec(t.Context(), []string{string(integrationutils.TaskCLI), "stats", "--queue", "default", "--pending-tasks"})
	require.NoError(t, err)
	require.Equal(t, 0, exitCode)

	// The output seems to be written in stderr instead of stdout
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

// This test checks that children tasks are created for the amount of tenants
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

	var wg sync.WaitGroup
	var taskCli testcontainers.Container

	wg.Add(3)
	go func() {
		defer wg.Done()
		integrationutils.RunCMKService(t, integrationutils.ServiceConfig{
			Service: integrationutils.TaskWorker,
		}, cfg)
	}()
	go func() {
		defer wg.Done()
		integrationutils.RunCMKService(t, integrationutils.ServiceConfig{
			Service: integrationutils.TaskScheduler,
		}, cfg)
	}()
	go func() {
		defer wg.Done()
		taskCli = integrationutils.RunCMKService(t, integrationutils.ServiceConfig{
			Service: integrationutils.TaskCLI,
			Args:    []string{"--sleep"},
		}, cfg)
	}()
	wg.Wait()

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
			Processed int `json:"Processed"` //nolint:tagliatelle
		}
		if err := json.Unmarshal(stderr.Bytes(), &stats); err != nil {
			return false
		}

		return exitCode == 0 && stats.Processed > 0 && stats.Processed%nTenants != 0 && stats.Processed > nTenants+1
	}, 15*time.Second, 100*time.Millisecond)
}
