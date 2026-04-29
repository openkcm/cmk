package integration_test

import (
	"bytes"
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
		_, err = stdcopy.StdCopy(&stdout, &stderr, reader)
		exitCode, reader, err = taskCli.Exec(t.Context(), []string{string(integrationutils.TaskCLI), "stats", "--queue", "default", "--queue-info"})
		return err == nil && exitCode == 0 && strings.Contains(stderr.String(), `"Processed": 1`)
	}, 5*time.Second, 100*time.Millisecond)
}
