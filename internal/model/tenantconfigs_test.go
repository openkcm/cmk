package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/constants"
)

func TestConfigFlattenRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		in   Flattable
		want map[string]any
	}{
		{
			name: "KeystoreConfig",
			in: &KeystoreConfig{
				RoleManagementConfig: ManagementConfig{
					LocalityID: "loc-1",
					CommonName: "cn-1",
					AccessData: KeystoreAccessData{"projectId": "p-123"},
				},
				KeyManagementConfig: ManagementConfig{LocalityID: "loc-2"},
				CryptoAccessData: map[string]CryptoConfig{
					"prod": {Subject: "subj", AccessData: KeystoreAccessData{"token": "t-9"}},
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
			in: &WorkflowConfig{
				Enabled:                 true,
				MinimumApprovals:        constants.DefaultMinimumApprovalCount,
				MaxApprovals:            constants.MaxMinimumApprovals,
				RetentionPeriodDays:     constants.DefaultRetentionPeriodDays,
				MinRetentionPeriodDays:  constants.MinRetentionPeriodDays,
				MaxRetentionPeriodDays:  constants.MaxRetentionPeriodDays,
				DefaultExpiryPeriodDays: constants.DefaultExpiryPeriodDays,
				MaxExpiryPeriodDays:     constants.DefaultMaxExpiryPeriodDays,
			},
			want: map[string]any{
				"enabled":                    true,
				"minimum_approvals":          constants.DefaultMinimumApprovalCount,
				"max_approvals":              constants.MaxMinimumApprovals,
				"retention_period_days":      constants.DefaultRetentionPeriodDays,
				"min_retention_period_days":  constants.MinRetentionPeriodDays,
				"max_retention_period_days":  constants.MaxRetentionPeriodDays,
				"default_expiry_period_days": constants.DefaultExpiryPeriodDays,
				"max_expiry_period_days":     constants.DefaultMaxExpiryPeriodDays,
			},
		},
		{
			name: "SystemGroupConfig",
			in:   &SystemGroupConfig{Enabled: true, WarnCoverage: true},
			want: map[string]any{
				"enabled":       true,
				"warn_coverage": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flat, err := Flatten(tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.want, flat)
		})
	}
}

func TestConfigUnflattenRoundTrip(t *testing.T) {
	t.Run("KeystoreConfig", func(t *testing.T) {
		in := &KeystoreConfig{
			RoleManagementConfig: ManagementConfig{
				LocalityID: "loc-1",
				CommonName: "cn-1",
				AccessData: KeystoreAccessData{"projectId": "p-123"},
			},
			KeyManagementConfig: ManagementConfig{LocalityID: "loc-2"},
			CryptoAccessData: map[string]CryptoConfig{
				"prod": {Subject: "subj", AccessData: KeystoreAccessData{"token": "t-9"}},
			},
			SupportedRegions: map[string]config.Region{
				"eu-10": {Name: "Europe", TechnicalName: "eu-10"},
			},
		}
		flat, err := Flatten(in)
		require.NoError(t, err)
		got, err := Unflatten[KeystoreConfig](flat)
		require.NoError(t, err)
		assert.Equal(t, *in, got)
	})

	t.Run("WorkflowConfig", func(t *testing.T) {
		in := &WorkflowConfig{
			Enabled:                 true,
			MinimumApprovals:        constants.DefaultMinimumApprovalCount,
			MaxApprovals:            constants.MaxMinimumApprovals,
			RetentionPeriodDays:     constants.DefaultRetentionPeriodDays,
			MinRetentionPeriodDays:  constants.MinRetentionPeriodDays,
			MaxRetentionPeriodDays:  constants.MaxRetentionPeriodDays,
			DefaultExpiryPeriodDays: constants.DefaultExpiryPeriodDays,
			MaxExpiryPeriodDays:     constants.DefaultMaxExpiryPeriodDays,
		}
		flat, err := Flatten(in)
		require.NoError(t, err)
		got, err := Unflatten[WorkflowConfig](flat)
		require.NoError(t, err)
		assert.Equal(t, *in, got)
	})

	t.Run("SystemGroupConfig", func(t *testing.T) {
		in := &SystemGroupConfig{Enabled: true, WarnCoverage: true}
		flat, err := Flatten(in)
		require.NoError(t, err)
		got, err := Unflatten[SystemGroupConfig](flat)
		require.NoError(t, err)
		assert.Equal(t, *in, got)
	})
}
