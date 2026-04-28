package integrationutils

import (
	"bytes"
	"io"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"gopkg.in/yaml.v3"
)

type Service string

const (
	ApiServer        Service = "/bin/api-server"
	TaskWorker       Service = "/bin/task-worker"
	TaskScheduler    Service = "/bin/task-scheduler"
	TaskCLI          Service = "/bin/task-cli"
	TenantManager    Service = "/bin/tenant-manager"
	TenantManagerCLI Service = "/bin/tenant-manager-cli"
	EventReconciler  Service = "/bin/event-reconciler"
	DBMigrator       Service = "/bin/db-migrator"
)

// Builds image and starts a testcontainer with the provided service
// This might take some time if there isn't an image built, but it has caching mechanisms
// Returns the container so you can execute commands or interact with it
func RunCMKService(t *testing.T, service Service, cfg *config.Config) testcontainers.Container {
	t.Helper()

	statusPort, err := testutils.GetFreePortString()
	require.NoError(t, err)

	cfg.Status = commoncfg.Status{
		Enabled: true,
		Address: ":" + statusPort,
	}

	cfgBytes, err := yaml.Marshal(cfg)
	require.NoError(t, err)

	ctx := t.Context()

	req := testcontainers.ContainerRequest{
		Image:      BuildCMKImage(t),
		Entrypoint: []string{string(service)},
		Files: []testcontainers.ContainerFile{
			{
				ContainerFilePath: path.Join(constants.DefaultConfigPath1, "/config.yaml"),
				Reader:            bytes.NewReader(cfgBytes),
				FileMode:          0o644,
			},
		},
		NetworkMode: "host",
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	testutils.WaitForServer(t, cfg.Status.Address)

	// Check container state
	state, err := container.State(ctx)
	require.NoError(t, err)

	// If container is not running, print logs before failing
	if !state.Running {
		logs, err := container.Logs(ctx)
		if err == nil {
			logBytes, _ := io.ReadAll(logs)
			t.Logf("Container logs:\n%s", string(logBytes))
			logs.Close()
		}
		require.True(t, state.Running, "container should be running")
	}

	return container
}

// BuildCMKImage runs makefile step docker-dev-build to build the image
// This is only needed as there is a bug with testcontainers and docker buildkit
func BuildCMKImage(t *testing.T) string {
	t.Helper()

	_, filename, _, _ := runtime.Caller(0)

	baseDir := filepath.Dir(filename)
	projectRoot, err := filepath.Abs(baseDir + "../../../../")
	require.NoError(t, err)

	cmd := exec.Command("make", "docker-dev-build")
	cmd.Dir = projectRoot
	_, err = cmd.CombinedOutput()
	if err != nil {
		require.NoError(t, err, "failed to build docker image")
	}

	return "cmk-api-server-dev:latest"
}
