package business_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/testutils"
	"github.com/openkcm/cmk/tenant-manager/business"
)

func TestBusinessMain(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "no amqp connection",
			config: &config.Config{
				TenantManager: config.TenantManager{}, // No AMQP connection info provided
				Database:      testutils.TestDB,
				Services: config.Services{
					Registry: testutils.TestRegistryConfig,
				},
			},
			expectError: true,
			errorMsg:    "Expected error due to missing AMQP connection info",
		},
		{
			name: "no db connection",
			config: &config.Config{
				TenantManager: config.TenantManager{AMQP: config.AMQP{
					URL:    "amqp://guest:guest@localhost:5672",
					Target: "target",
					Source: "source",
				}},
				Database: config.Database{}, // No database connection info provided
				Services: config.Services{
					Registry: testutils.TestRegistryConfig,
				},
			},
			expectError: true,
			errorMsg:    "Expected error due to missing database configuration",
		},
		{
			name: "no grpc configuration",
			config: &config.Config{
				TenantManager: config.TenantManager{AMQP: config.AMQP{
					URL:    "amqp://guest:guest@localhost:5672",
					Target: "target",
					Source: "source",
				}},
				Database: testutils.TestDB,
				Services: config.Services{}, // No gRPC configuration provided
			},
			expectError: true,
			errorMsg:    "Expected error due to missing gRPC configuration",
		},
		{
			name: "valid configuration",
			config: &config.Config{
				TenantManager: config.TenantManager{AMQP: config.AMQP{
					URL:    "amqp://guest:guest@localhost:5672",
					Target: "target",
					Source: "source",
				}},
				Database: testutils.TestDB,
				Services: config.Services{
					Registry: testutils.TestRegistryConfig,
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			err := business.Main(ctx, tt.config)

			if tt.expectError {
				assert.Error(t, err, tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
