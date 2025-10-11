package tenantmanager_test

import (
	"os"
	"testing"
	"time"

	"github.com/openkcm/cmk/cmd/tenantmanager"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/config"
)

func TestLoadConfig(t *testing.T) {
	t.Run("Should return error if config file not found", func(t *testing.T) {
		_, err := tenantmanager.LoadConfig()
		t.Log("Error:", err)
		require.Error(t, err)
	})

	t.Run("Should return error if config file has wrong struct", func(t *testing.T) {
		content := []byte("application:\n  nameE: test-app\n")
		err := os.WriteFile("config.yaml", content, 0600)
		require.NoError(t, err)

		defer os.Remove("config.yaml")

		_, err = tenantmanager.LoadConfig()
		t.Log("Error:", err)
		require.Error(t, err)
	})

	t.Run("Should load config if config file exists", func(t *testing.T) {
		content := []byte("application:\n  name: test-app\nlogger:\n  level: info\n")
		err := os.WriteFile("config.yaml", content, 0600)
		require.NoError(t, err)

		defer os.Remove("config.yaml")

		cfg, err := tenantmanager.LoadConfig()
		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.Equal(t, "test-app", cfg.Application.Name)
	})
}

func TestStartStatusServer(t *testing.T) {
	cfg := &config.Config{}

	// Call and check for no panic.
	tenantmanager.StartStatusServer(t.Context(), cfg)

	// Optionally, wait a short time to let the goroutine start
	time.Sleep(100 * time.Millisecond)
}
