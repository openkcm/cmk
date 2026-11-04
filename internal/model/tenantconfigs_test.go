package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/model"
)

func TestConfigFlattenRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		in   model.Flattenable
		want map[string]any
	}{
		{
			name: "KeystoreConfig",
			in: &model.KeystoreConfig{
				RoleManagementConfig: model.ManagementConfig{
					LocalityID: "loc-1",
					CommonName: "cn-1",
					AccessData: model.KeystoreAccessData{"projectId": "p-123"},
				},
				KeyManagementConfig: model.ManagementConfig{LocalityID: "loc-2"},
				CryptoAccessData: map[string]model.CryptoConfig{
					"prod": {Subject: "subj", AccessData: model.KeystoreAccessData{"token": "t-9"}},
				},
				SupportedRegions: map[string]config.Region{
					"eu-10": {Name: "Europe", TechnicalName: "eu-10"},
				},
			},
			want: map[string]any{
				"keystore_config/role_mgmt/locality_id":                "loc-1",
				"keystore_config/role_mgmt/common_name":                "cn-1",
				"keystore_config/role_mgmt/access_data/projectId":      "p-123",
				"keystore_config/key_mgmt/locality_id":                 "loc-2",
				"keystore_config/key_mgmt/common_name":                 "",
				"keystore_config/crypto/prod/subject":                  "subj",
				"keystore_config/crypto/prod/access_data/token":        "t-9",
				"keystore_config/supported_region/eu-10/name":          "Europe",
				"keystore_config/supported_region/eu-10/technicalName": "eu-10",
			},
		},
		{
			name: "WorkflowConfig",
			in: &model.WorkflowConfig{
				Enabled:                       true,
				MinAllowedApprovals:           constants.MinAllowedApprovals,
				RequiredApprovals:             constants.MinAllowedApprovals,
				MaxAllowedApprovals:           constants.MaxAllowedApprovals,
				RetentionPeriodDays:           constants.DefaultRetentionPeriodDays,
				MinAllowedRetentionPeriodDays: constants.MinRetentionPeriodDays,
				MaxAllowedRetentionPeriodDays: constants.MaxRetentionPeriodDays,
				ExpiryPeriodDays:              constants.DefaultExpiryPeriodDays,
				MaxAllowedExpiryPeriodDays:    constants.MaxAllowedExpiryPeriodDays,
			},
			want: map[string]any{
				"enabled":                           true,
				"required_approvals":                constants.MinAllowedApprovals,
				"min_allowed_approvals":             constants.MinAllowedApprovals,
				"max_allowed_approvals":             constants.MaxAllowedApprovals,
				"retention_period_days":             constants.DefaultRetentionPeriodDays,
				"min_allowed_retention_period_days": constants.MinRetentionPeriodDays,
				"max_allowed_retention_period_days": constants.MaxRetentionPeriodDays,
				"expiry_period_days":                constants.DefaultExpiryPeriodDays,
				"max_allowed_expiry_period_days":    constants.MaxAllowedExpiryPeriodDays,
			},
		},
		{
			name: "SystemGroupConfig",
			in:   &model.SystemGroupConfig{Enabled: true, WarnCoverage: true},
			want: map[string]any{
				"enabled":       true,
				"warn_coverage": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flat, err := model.Flatten(tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.want, flat)
		})
	}
}

func TestConfigUnflattenRoundTrip(t *testing.T) {
	t.Run("KeystoreConfig", func(t *testing.T) {
		in := &model.KeystoreConfig{
			RoleManagementConfig: model.ManagementConfig{
				LocalityID: "loc-1",
				CommonName: "cn-1",
				AccessData: model.KeystoreAccessData{"projectId": "p-123"},
			},
			KeyManagementConfig: model.ManagementConfig{LocalityID: "loc-2"},
			CryptoAccessData: map[string]model.CryptoConfig{
				"prod": {Subject: "subj", AccessData: model.KeystoreAccessData{"token": "t-9"}},
			},
			SupportedRegions: map[string]config.Region{
				"eu-10": {Name: "Europe", TechnicalName: "eu-10"},
			},
		}
		flat, err := model.Flatten(in)
		require.NoError(t, err)
		got, err := model.Unflatten[model.KeystoreConfig](flat)
		require.NoError(t, err)
		assert.Equal(t, *in, got)
	})

	t.Run("WorkflowConfig", func(t *testing.T) {
		in := &model.WorkflowConfig{
			Enabled:                       true,
			MinAllowedApprovals:           constants.MinAllowedApprovals,
			MaxAllowedApprovals:           constants.MaxAllowedApprovals,
			RetentionPeriodDays:           constants.DefaultRetentionPeriodDays,
			MinAllowedRetentionPeriodDays: constants.MinRetentionPeriodDays,
			MaxAllowedRetentionPeriodDays: constants.MaxRetentionPeriodDays,
			ExpiryPeriodDays:              constants.DefaultExpiryPeriodDays,
			MaxAllowedExpiryPeriodDays:    constants.MaxAllowedExpiryPeriodDays,
		}
		flat, err := model.Flatten(in)
		require.NoError(t, err)
		got, err := model.Unflatten[model.WorkflowConfig](flat)
		require.NoError(t, err)
		assert.Equal(t, *in, got)
	})

	t.Run("SystemGroupConfig", func(t *testing.T) {
		in := &model.SystemGroupConfig{Enabled: true, WarnCoverage: true}
		flat, err := model.Flatten(in)
		require.NoError(t, err)
		got, err := model.Unflatten[model.SystemGroupConfig](flat)
		require.NoError(t, err)
		assert.Equal(t, *in, got)
	})
}
