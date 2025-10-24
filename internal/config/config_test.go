package config_test

import (
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk-core/internal/config"
	"github.com/openkcm/cmk-core/internal/testutils"
)

func TestValidateCertificate(t *testing.T) {
	t.Run("Should successfully validate", func(t *testing.T) {
		certs := config.Certificates{ValidityDays: 7, RotationThresholdDays: 3}
		assert.NoError(t, certs.Validate())
	})

	t.Run("Should fail validation for ValidityDays too short", func(t *testing.T) {
		certs := config.Certificates{ValidityDays: 6, RotationThresholdDays: 3}
		assert.Error(t, certs.Validate())
	})

	t.Run("Should fail validation for ValidityDays too long", func(t *testing.T) {
		certs := config.Certificates{ValidityDays: 31, RotationThresholdDays: 3}
		assert.Error(t, certs.Validate())
	})
}

func TestValidateScheduler(t *testing.T) {
	t.Run("Should successfully validate", func(t *testing.T) {
		scheduler := config.Scheduler{
			Tasks: []config.Task{
				{
					TaskType: config.TypeCertificateTask,
					Cronspec: "@daily",
				},
			},
		}
		assert.NoError(t, scheduler.Validate())
	})

	t.Run("Should fail validation", func(t *testing.T) {
		scheduler := config.Scheduler{
			Tasks: []config.Task{
				{
					TaskType: "UnknownTask",
					Cronspec: "@daily",
				},
			},
		}
		assert.Error(t, scheduler.Validate())
	})
}

func TestValidateTenantManager(t *testing.T) {
	mutator := testutils.NewMutator(func() config.TenantManager {
		return config.TenantManager{
			SecretRef: commoncfg.SecretRef{
				Type: commoncfg.MTLSSecretType,
			},
			AMQP: config.AMQP{
				URL:    "amqp://guest:guest@localhost:5672",
				Target: "target",
				Source: "source",
			},
		}
	})

	tests := []struct {
		name   string
		config config.TenantManager
		expErr error
	}{
		{
			name:   "Valid configuration",
			config: mutator(),
		},
		{
			name: "Invalid Secret Type",
			config: mutator(func(tm *config.TenantManager) {
				tm.SecretRef.Type = "unknown"
			}),
			expErr: config.ErrConfigurationValuesError,
		},
		{
			name: "Invalid AMQP URL",
			config: mutator(func(tm *config.TenantManager) {
				tm.AMQP.URL = ""
			}),
			expErr: config.ErrAMQPEmptyURL,
		},
		{
			name: "Invalid AMQP Target",
			config: mutator(func(tm *config.TenantManager) {
				tm.AMQP.Target = ""
			}),
			expErr: config.ErrAMQPEmptyTarget,
		},
		{
			name: "Invalid AMQP Source",
			config: mutator(func(tm *config.TenantManager) {
				tm.AMQP.Source = ""
			}),
			expErr: config.ErrAMQPEmptySource,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expErr != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.expErr)

				return
			}

			assert.NoError(t, err)
		})
	}
}
