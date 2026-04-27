package integration_test

import (
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	plugincatalog "github.com/openkcm/plugin-sdk/pkg/catalog"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/testutils"
	integrationutils "github.com/openkcm/cmk/test/integration/integration_utils"
)

func TestAsync(t *testing.T) {
	_, _, dbCfg := testutils.NewTestDB(t, testutils.TestDBConfig{})

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
		BaseConfig: commoncfg.BaseConfig{
			Status: commoncfg.Status{
				Enabled: true,
				Address: "",
			},
		},
	}

	testutils.StartRedis(t, &cfg.Scheduler)

	_ = integrationutils.RunCMKService(t, integrationutils.TaskWorker, cfg)
	_ = integrationutils.RunCMKService(t, integrationutils.TaskCLI, cfg)
}
