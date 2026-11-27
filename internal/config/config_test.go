package config_test

import (
	"strings"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/testutils"
)

func TestSystemSearchableOptions(t *testing.T) {
	s := config.System{
		Identifier: config.SystemProperty{DisplayName: "Identifier", Searchable: true},
		Region:     config.SystemProperty{DisplayName: "Region", Searchable: true},
		OptionalProperties: map[string]config.SystemProperty{
			"searchable": {DisplayName: "Searchable", Searchable: true},
			"hidden":     {DisplayName: "Hidden"},
		},
	}

	assert.Equal(t, map[string]config.SystemProperty{
		"identifier": s.Identifier,
		"region":     s.Region,
		"searchable": s.OptionalProperties["searchable"],
	}, s.SearchableOptions())
}

func TestSystemAllowsType(t *testing.T) {
	tests := []struct {
		name    string
		enum    []string
		value   string
		allowed bool
	}{
		{name: "should fail on value outside of enum", value: "ERROR", allowed: false, enum: []string{"SYSTEM"}},
		{name: "should pass", value: "SYSTEM", allowed: true, enum: []string{"SYSTEM"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := config.System{Type: config.SystemProperty{Enum: tt.enum}}
			assert.Equal(t, tt.allowed, s.AllowsType(tt.value))
		})
	}
}

func TestValidateCertificate(t *testing.T) {
	tests := map[string]struct {
		certs   config.Certificates
		wantErr error
	}{
		"valid: threshold below validity": {
			certs:   config.Certificates{ValidityDays: 7, RotationThresholdDays: 3, DefaultTenantCertPrefix: "prefix"},
			wantErr: nil,
		},
		"validity too short": {
			certs:   config.Certificates{ValidityDays: 6, RotationThresholdDays: 3},
			wantErr: config.ErrCertificateValidityOutsideRange,
		},
		"validity too long": {
			certs:   config.Certificates{ValidityDays: 31, RotationThresholdDays: 3},
			wantErr: config.ErrCertificateValidityOutsideRange,
		},
		"threshold equals validity": {
			certs:   config.Certificates{ValidityDays: 7, RotationThresholdDays: 7},
			wantErr: config.ErrRotationThresholdNotBelowValidity,
		},
		"threshold exceeds validity": {
			certs:   config.Certificates{ValidityDays: 7, RotationThresholdDays: 8},
			wantErr: config.ErrRotationThresholdNotBelowValidity,
		},
		"threshold just below validity": {
			certs:   config.Certificates{ValidityDays: 7, RotationThresholdDays: 6},
			wantErr: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := tc.certs.Validate()
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateCryptoLayer(t *testing.T) {
	t.Run("Should success", func(t *testing.T) {
		cryptoCerts := []config.CryptoCert{
			{
				Name: "crypto-1",
				Subject: config.CryptoCertSubject{
					CommonNamePrefix: "test_",
				},
				RootCA: "test",
			},
		}

		bytes, err := yaml.Marshal(cryptoCerts)
		assert.NoError(t, err)

		certs := config.CryptoLayer{CertX509Trusts: commoncfg.SourceRef{
			Source: commoncfg.EmbeddedSourceValue,
			Value:  string(bytes),
		}}
		assert.NoError(t, certs.Validate())
	})

	t.Run("Should fail on prefix bigger than limit", func(t *testing.T) {
		cryptoCerts := []config.CryptoCert{
			{
				Name: "crypto-1",
				Subject: config.CryptoCertSubject{
					CommonNamePrefix: strings.Repeat("t", config.MaxCryptoCNPrefix+1),
				},
				RootCA: "test",
			},
		}

		bytes, err := yaml.Marshal(cryptoCerts)
		assert.NoError(t, err)

		certs := config.CryptoLayer{CertX509Trusts: commoncfg.SourceRef{
			Source: commoncfg.EmbeddedSourceValue,
			Value:  string(bytes),
		}}
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

	t.Run("Should validate when sentinel is disabled", func(t *testing.T) {
		scheduler := config.Scheduler{
			TaskQueue: config.Redis{
				Sentinel: config.Sentinel{Enabled: false},
			},
		}
		assert.NoError(t, scheduler.Validate())
	})

	t.Run("Should validate when sentinel is enabled and complete", func(t *testing.T) {
		scheduler := config.Scheduler{
			TaskQueue: config.Redis{
				Sentinel: config.Sentinel{
					Enabled:    true,
					MasterName: "mymaster",
					Addresses:  []string{"sentinel-0:26379"},
				},
			},
		}
		assert.NoError(t, scheduler.Validate())
	})

	t.Run("Should fail when sentinel is enabled without master name", func(t *testing.T) {
		scheduler := config.Scheduler{
			TaskQueue: config.Redis{
				Sentinel: config.Sentinel{
					Enabled:   true,
					Addresses: []string{"sentinel-0:26379"},
				},
			},
		}
		assert.ErrorIs(t, scheduler.Validate(), config.ErrSentinelMasterNameEmpty)
	})

	t.Run("Should fail when sentinel is enabled without addresses", func(t *testing.T) {
		scheduler := config.Scheduler{
			TaskQueue: config.Redis{
				Sentinel: config.Sentinel{
					Enabled:    true,
					MasterName: "mymaster",
				},
			},
		}
		assert.ErrorIs(t, scheduler.Validate(), config.ErrSentinelNoAddresses)
	})
}

func TestValidateTenantLimits(t *testing.T) {
	t.Run("Should successfully validate minimum", func(t *testing.T) {
		tl := config.Tenant{SystemLimit: config.MinTenantLimit, KeyLimit: config.MinTenantLimit, KeyConfigLimit: config.MinTenantLimit}
		assert.NoError(t, tl.Validate())
	})

	t.Run("Should fail validation for Systems below minimum", func(t *testing.T) {
		tl := config.Tenant{SystemLimit: 0, KeyLimit: config.MinTenantLimit}
		err := tl.Validate()
		assert.Error(t, err)
		assert.ErrorIs(t, err, config.ErrTenantLimitsSystemsBelowMinimum)
	})

	t.Run("Should fail validation for Keys below minimum", func(t *testing.T) {
		tl := config.Tenant{SystemLimit: config.MinTenantLimit, KeyLimit: 0}
		err := tl.Validate()
		assert.Error(t, err)
		assert.ErrorIs(t, err, config.ErrTenantLimitsKeyBelowMinimum)
	})

	t.Run("Should fail validation for Key Configs below minimum", func(t *testing.T) {
		tl := config.Tenant{SystemLimit: config.MinTenantLimit, KeyLimit: config.MinTenantLimit, KeyConfigLimit: 0}
		err := tl.Validate()
		assert.Error(t, err)
		assert.ErrorIs(t, err, config.ErrTenantLimitsKeyConfigBelowMinimum)
	})
}

func TestValidateConfig(t *testing.T) {
	cryptoCerts := []config.CryptoCert{
		{
			Name: "crypto-1",
			Subject: config.CryptoCertSubject{
				CommonNamePrefix: "test_",
			},
			RootCA: "test",
		},
	}

	bytes, err := yaml.Marshal(cryptoCerts)
	assert.NoError(t, err)

	t.Run("Should validate", func(t *testing.T) {
		cfg := config.Config{
			Certificates: config.Certificates{
				ValidityDays: config.MinCertificateValidityDays,
			},
			CryptoLayer: config.CryptoLayer{CertX509Trusts: commoncfg.SourceRef{
				Source: commoncfg.EmbeddedSourceValue,
				Value:  string(bytes),
			}},
			Tenant: config.Tenant{
				SystemLimit:    config.MinTenantLimit,
				KeyLimit:       config.MinTenantLimit,
				KeyConfigLimit: config.MinTenantLimit,
			},
		}

		err = cfg.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should error on invalid tenant config", func(t *testing.T) {
		cfg := config.Config{
			Certificates: config.Certificates{
				ValidityDays: config.MinCertificateValidityDays,
			},
			CryptoLayer: config.CryptoLayer{CertX509Trusts: commoncfg.SourceRef{
				Source: commoncfg.EmbeddedSourceValue,
				Value:  string(bytes),
			}},
			Tenant: config.Tenant{
				SystemLimit: 0,
			},
		}

		err = cfg.Validate()
		assert.ErrorIs(t, err, config.ErrTenantLimitsSystemsBelowMinimum)
	})
}

func TestValidateTenantManager(t *testing.T) {
	mutator := testutils.NewMutator(func() config.TenantManager {
		return config.TenantManager{
			SecretRef: commoncfg.SecretRef{
				Type: commoncfg.MTLSSecretType,
			},
			AMQP: config.AMQP{ //nolint:gosec // well-known RabbitMQ default credentials used only in tests
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

func TestAsyncSigningBuildSigner(t *testing.T) {
	validKey := strings.Repeat("k", 32)
	embedded := func(v string) commoncfg.SourceRef {
		return commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: v}
	}

	t.Run("no key configured returns nil signer", func(t *testing.T) {
		signer, err := config.AsyncSigning{}.BuildSigner()
		assert.NoError(t, err)
		assert.Nil(t, signer)
	})

	t.Run("valid key builds signer", func(t *testing.T) {
		signer, err := config.AsyncSigning{KeyID: "k1", Key: embedded(validKey)}.BuildSigner()
		assert.NoError(t, err)
		assert.NotNil(t, signer)
	})

	t.Run("key without keyId is rejected", func(t *testing.T) {
		_, err := config.AsyncSigning{Key: embedded(validKey)}.BuildSigner()
		assert.ErrorIs(t, err, config.ErrAsyncSigningKeyIDMissing)
	})

	t.Run("short key is rejected", func(t *testing.T) {
		_, err := config.AsyncSigning{KeyID: "k1", Key: embedded("tooshort")}.BuildSigner()
		assert.ErrorIs(t, err, config.ErrAsyncSigningKeyTooShort)
	})

	t.Run("short previous key is rejected", func(t *testing.T) {
		_, err := config.AsyncSigning{
			KeyID: "k2", Key: embedded(validKey),
			PreviousKeyID: "k1", PreviousKey: embedded("short"),
		}.BuildSigner()
		assert.ErrorIs(t, err, config.ErrAsyncSigningKeyTooShort)
	})
}
