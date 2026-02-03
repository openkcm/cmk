package tenant_manager_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/orbital"
	"github.com/openkcm/orbital/client/amqp"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"
	tenantgrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/tenant/v1"
	oidcmappinggrpc "github.com/openkcm/api-sdk/proto/kms/api/cmk/sessionmanager/oidcmapping/v1"

	"github.com/openkcm/cmk/internal/clients/registry/tenants"
	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
	sessionmanager "github.com/openkcm/cmk/internal/testutils/clients/session-manager"
)

// testEnv holds all test infrastructure
type testEnv struct {
	multitenancyDB *multitenancy.DB
	tenantManager  *tenantManagerProcess
	amqpClient     *amqp.Client
	dbConfig       config.Database
	fakeRegistry   *tenants.FakeTenantService
	registryAddr   string
}

// tenantManagerProcess represents a running TenantManager process
type tenantManagerProcess struct {
	cancel context.CancelFunc
	done   chan error
	cmd    *exec.Cmd
}

// setupLogger configures the global slog logger
func setupLogger(tb testing.TB) {
	tb.Helper()

	opts := &slog.HandlerOptions{Level: slog.LevelDebug}
	handler := slog.NewTextHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)
	tb.Log("Configured global slog logger for testing (use -v to see TenantManager logs)")
}

// setupInfrastructure sets up all required infrastructure components
func setupInfrastructure(tb testing.TB) (
	*tenants.FakeTenantService, string, string, *multitenancy.DB, config.Database,
) {
	tb.Helper()

	// Setup fake Registry server
	tb.Log("Setting up fake Registry gRPC server...")

	fakeTenantService := tenants.NewFakeTenantService()

	_, registryGRPCClient := testutils.NewGRPCSuite(tb, func(s *grpc.Server) {
		tenantgrpc.RegisterServiceServer(s, fakeTenantService)
	})

	// Get the actual address from the gRPC client
	registryAddr := registryGRPCClient.Target()

	// Setup fake SessionManager server
	tb.Log("Setting up fake SessionManager gRPC server...")

	fakeSessionManagerService := sessionmanager.NewFakeSessionManagerService()

	_, sessionManagerGRPCClient := testutils.NewGRPCSuite(tb, func(s *grpc.Server) {
		oidcmappinggrpc.RegisterServiceServer(s, fakeSessionManagerService)
	})

	// Get the actual address from the gRPC client
	sessionManagerAddr := sessionManagerGRPCClient.Target()

	// Wait for gRPC servers to be ready
	time.Sleep(1000 * time.Millisecond) // Give the servers time to start

	multitenancyDB, _, dbConfig := testutils.NewTestDB(tb, testutils.TestDBConfig{
		CreateDatabase:      true,
		WithIsolatedService: true,
	}, testutils.WithGenerateTenants(0), // Do not create tenants
	)

	return fakeTenantService, registryAddr, sessionManagerAddr, multitenancyDB, dbConfig
}

// createConfigurations creates test configurations
func createConfigurations(
	dbConfig config.Database,
	registryAddr,
	sessionManagerAddr string,
	amqpCfg config.AMQP,
) *config.Config {
	return &config.Config{
		BaseConfig: commoncfg.BaseConfig{
			Application: commoncfg.Application{
				Name:        "tenant-manager-test",
				Environment: "test",
			},
			Logger: commoncfg.Logger{
				Level:  "debug",
				Format: "text",
			},
			Telemetry: commoncfg.Telemetry{
				Logs:    commoncfg.Log{Enabled: false},
				Traces:  commoncfg.Trace{Enabled: false},
				Metrics: commoncfg.Metric{Enabled: false},
			},
		},
		Database: dbConfig,
		Services: config.Services{
			Registry: &commoncfg.GRPCClient{
				Enabled:   true,
				Address:   registryAddr,
				SecretRef: &commoncfg.SecretRef{Type: commoncfg.InsecureSecretType},
			},
			SessionManager: &commoncfg.GRPCClient{
				Enabled:   true,
				Address:   sessionManagerAddr,
				SecretRef: &commoncfg.SecretRef{Type: commoncfg.InsecureSecretType},
			},
		},
		TenantManager: config.TenantManager{
			SecretRef: commoncfg.SecretRef{
				Type: commoncfg.InsecureSecretType,
			},
			AMQP: amqpCfg,
		},
	}
}

// startTenantManagerProcess starts the TenantManager process as an external binary.
func startTenantManagerProcess(tb testing.TB, testConfig *config.Config) (*tenantManagerProcess, error) {
	tb.Helper()

	tb.Log("Starting TenantManager process...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	done := make(chan error, 1)

	configPath, err := prepareConfigFile(tb, testConfig)
	if err != nil {
		cancel()
		return nil, err
	}

	cmd, stdoutBuf, stderrBuf := setupCommand(ctx, configPath)

	go startProcessGoroutine(ctx, tb, cmd, done, stdoutBuf, stderrBuf)

	return &tenantManagerProcess{
		cancel: cancel,
		done:   done,
		cmd:    cmd,
	}, nil
}

func prepareConfigFile(tb testing.TB, testConfig *config.Config) (string, error) {
	tb.Helper()

	projectRoot, err := filepath.Abs("../..")
	if err != nil {
		return "", fmt.Errorf("failed to resolve project root: %w", err)
	}

	testDir := filepath.Join(projectRoot, "test", "tenant-manager")
	configPath := filepath.Join(testDir, "config.yaml")

	content, err := yaml.Marshal(testConfig)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}

	err = os.WriteFile(configPath, content, 0o600)
	if err != nil {
		return "", fmt.Errorf("failed to write config file: %w", err)
	}

	tb.Cleanup(func() { os.Remove(configPath) })

	return configPath, nil
}

func setupCommand(ctx context.Context, configPath string) (*exec.Cmd, *bytes.Buffer, *bytes.Buffer) {
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(configPath)))
	cmdPath := filepath.Join(projectRoot, "cmd", "tenant-manager")

	cmd := exec.CommandContext(ctx, "go", "run", cmdPath)

	var stdoutBuf, stderrBuf bytes.Buffer
	if testing.Verbose() {
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf
	} else {
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
	}

	cmd.Dir = filepath.Dir(configPath)

	return cmd, &stdoutBuf, &stderrBuf
}

func startProcessGoroutine(
	ctx context.Context,
	tb testing.TB,
	cmd *exec.Cmd,
	done chan error,
	stdoutBuf, stderrBuf *bytes.Buffer,
) {
	tb.Helper()

	defer close(done)

	tb.Logf("Starting external process: %v", cmd.String())

	err := cmd.Start()
	if err != nil {
		done <- fmt.Errorf("failed to start process: %w", err)
		return
	}

	err = cmd.Wait()

	writeBufferedOutput(stdoutBuf, stderrBuf)
	handleProcessCompletion(ctx, tb, cmd, err, done)
}

func writeBufferedOutput(stdoutBuf, stderrBuf *bytes.Buffer) {
	if testing.Verbose() && stdoutBuf.Len() > 0 {
		os.Stdout.Write(stdoutBuf.Bytes())
	}

	if testing.Verbose() && stderrBuf.Len() > 0 {
		os.Stderr.Write(stderrBuf.Bytes())
	}
}

func handleProcessCompletion(
	ctx context.Context,
	tb testing.TB,
	cmd *exec.Cmd,
	err error,
	done chan error,
) {
	tb.Helper()

	switch {
	case err != nil && ctx.Err() == nil:
		tb.Logf("TenantManager exited with error: %v", err)

		done <- err
	case ctx.Err() != nil:
		tb.Logf("TenantManager stopped due to context: %v", ctx.Err())

		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	default:
		tb.Log("TenantManager stopped successfully")
	}
}

// setupCleanup configures automatic cleanup for the test environment
func setupCleanup(tb testing.TB, env *testEnv) {
	tb.Helper()

	tb.Cleanup(func() {
		if env.amqpClient != nil {
			err := env.amqpClient.Close(tb.Context())
			if err != nil {
				tb.Logf("Warning: Failed to close AMQP client: %v", err)
			}
		}

		if env.tenantManager != nil {
			tb.Log("Stopping TenantManager process...")
			env.tenantManager.cancel()

			// Give the process a moment to terminate gracefully
			time.Sleep(500 * time.Millisecond)

			// Force kill if still running
			if env.tenantManager.cmd != nil && env.tenantManager.cmd.Process != nil {
				_ = env.tenantManager.cmd.Process.Kill()
			}

			select {
			case err := <-env.tenantManager.done:
				if err != nil {
					tb.Logf("TenantManager goroutine finished with error: %v", err)
				} else {
					tb.Log("TenantManager goroutine finished successfully")
				}
			case <-time.After(5 * time.Second):
				tb.Log("Timeout waiting for TenantManager goroutine to finish")
			}
		}
	})
}

// SetupTest sets up the complete test environment with automatic cleanup
func SetupTest(tb testing.TB) *testEnv {
	tb.Helper()

	// Setup logger
	setupLogger(tb)

	// Setup infrastructure
	fakeTenantService, registryAddr, sessionManagerAddr,
		multitenancyDB, dbConfig := setupInfrastructure(tb)
	amqpClient, amqpCfg := testutils.NewAMQPClient(tb, testutils.AMQPCfg{})

	// Create configurations
	testConfig := createConfigurations(dbConfig, registryAddr, sessionManagerAddr, amqpCfg)

	// Start TenantManager
	tenantManager, err := startTenantManagerProcess(tb, testConfig)
	require.NoError(tb, err, "Failed to start TenantManager process")

	// Create AMQP client

	env := &testEnv{
		multitenancyDB: multitenancyDB,
		tenantManager:  tenantManager,
		amqpClient:     amqpClient,
		dbConfig:       dbConfig,
		fakeRegistry:   fakeTenantService,
		registryAddr:   registryAddr,
	}

	// Setup automatic cleanup
	setupCleanup(tb, env)

	return env
}

// sendAMQPMessage sends a message to RabbitMQ using the orbital pattern
func (env *testEnv) sendAMQPMessage(t *testing.T, message any) {
	t.Helper()

	var taskType string

	var data []byte

	var err error

	switch msg := message.(type) {
	case *tenantgrpc.RegisterTenantRequest:
		taskType = tenantgrpc.ACTION_ACTION_PROVISION_TENANT.String()
		tenant := &tenantgrpc.Tenant{
			Id:   msg.GetId(),
			Name: msg.GetName(),
		}
		data, err = proto.Marshal(tenant)
		require.NoError(t, err, "Failed to marshal Tenant")
	case *tenantgrpc.BlockTenantRequest:
		taskType = tenantgrpc.ACTION_ACTION_BLOCK_TENANT.String()
		data, err = proto.Marshal(msg)
		require.NoError(t, err, "Failed to marshal BlockTenantRequest")
	case *tenantgrpc.UnblockTenantRequest:
		taskType = tenantgrpc.ACTION_ACTION_UNBLOCK_TENANT.String()
		data, err = proto.Marshal(msg)
		require.NoError(t, err, "Failed to marshal UnblockTenantRequest")
	case *tenantgrpc.TerminateTenantRequest:
		taskType = tenantgrpc.ACTION_ACTION_TERMINATE_TENANT.String()
		data, err = proto.Marshal(msg)
		require.NoError(t, err, "Failed to marshal TerminateTenantRequest")
	default:
		t.Fatalf("Unsupported message type: %T", message)
	}

	taskRequest := orbital.TaskRequest{
		TaskID:       uuid.New(),
		Type:         taskType,
		Data:         data,
		WorkingState: []byte(""),
		ETag:         "",
	}

	err = env.amqpClient.SendTaskRequest(t.Context(), taskRequest)
	require.NoError(t, err, "Failed to send task request")
}

// sendAMQPMessageAndWaitForResponse sends a message and waits for the response
func (env *testEnv) sendAMQPMessageAndWaitForResponse(t *testing.T, message any) *orbital.TaskResponse {
	t.Helper()

	// Send the message first
	env.sendAMQPMessage(t, message)

	// Wait for response with timeout
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	// Listen for the response
	response, err := env.amqpClient.ReceiveTaskResponse(ctx)
	require.NoError(t, err, "Failed to receive task response")
	require.NotNil(t, response, "Task response should not be nil")

	return &response
}

// verifyTenantInDatabase verifies that a tenant exists in the database using the repo interface
func (env *testEnv) verifyTenantInDatabase(t *testing.T, tenantID string) {
	t.Helper()

	// Create repository using the existing multitenancy DB connection
	repository := sql.NewRepository(env.multitenancyDB)

	// Create tenant context
	ctx := context.Background()

	// Create tenant model to search for
	tenant := &model.Tenant{
		ID: tenantID,
	}

	// Use repo.First to check if tenant exists
	exists, err := repository.First(ctx, tenant, *repo.NewQuery())
	require.NoError(t, err, "Failed to check tenant existence using repo")
	require.True(t, exists, "Tenant %s should exist in database", tenantID)
}

// TestTenantManagerIntegration tests the complete TenantManager integration
func TestTenantManagerIntegration(t *testing.T) {
	env := SetupTest(t)

	t.Run("CreateTenant", func(t *testing.T) {
		tenantID := uuid.NewString()
		tenantName := "IntegrationTestTenant"

		createRequest := &tenantgrpc.RegisterTenantRequest{
			Id:        tenantID,
			Name:      tenantName,
			Region:    "emea",
			OwnerId:   "owner123",
			OwnerType: "user",
			Role:      tenantgrpc.Role_ROLE_LIVE,
		}

		// Send message and wait for response
		response := env.sendAMQPMessageAndWaitForResponse(t, createRequest)

		// Verify the response
		require.Equal(t, string(orbital.TaskStatusProcessing), response.Status, "Task should be processing")
		require.Contains(t, string(response.WorkingState), "tenant is being created", "Response should be Working state")

		// If we get the response, we can call again the sendAMQPMessage function.
		// We do not wait, as the sendTenantUserGroupsToRegistry function will send the user groups in the background.
		env.sendAMQPMessage(t, createRequest)

		// Wait until user groups are sent to Registry (signals provisioning completion) or timeout
		select {
		case req := <-env.fakeRegistry.GroupsSetCh:
			require.Equal(t, tenantID, req.GetId(), "Expected user groups to be set for the new tenant")
		case <-time.After(10 * time.Second):
			require.Fail(t, "Timeout waiting for user groups to be sent to Registry")
		}

		env.verifyTenantInDatabase(t, tenantID)
	})

	t.Run("BlockTenant", func(t *testing.T) {
		t.Log("Block tenant test - TODO: We will implement this when functionality is implemented")
	})

	t.Run("UnblockTenant", func(t *testing.T) {
		t.Log("Unblock tenant test - TODO: We will implement this when functionality is implemented")
	})

	t.Run("TerminateTenant", func(t *testing.T) {
		t.Log("Terminate tenant test - TODO: We will implement this when functionality is implemented")
	})
}
