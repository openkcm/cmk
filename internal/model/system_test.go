package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/config"
	"github.com/openkcm/cmk/internal/model"
)

func TestSystem(t *testing.T) {
	t.Run("Should have table name systems", func(t *testing.T) {
		expectedTableName := "systems"

		tableName := model.System{}.TableName()

		assert.Equal(t, expectedTableName, tableName)
	})

	t.Run("Should be a tenant table", func(t *testing.T) {
		assert.False(t, model.System{}.IsSharedModel())
	})
}

func TestSystemHasEmptyRole(t *testing.T) {
	roleCfg := &config.System{
		OptionalProperties: map[string]config.SystemProperty{
			model.SystemPropertyRoleName: {},
			model.SystemPropertyRoleID:   {},
		},
	}

	tests := []struct {
		name     string
		cfg      *config.System
		props    map[string]string
		expected bool
	}{
		{
			name: "both role keys present",
			cfg:  roleCfg,
			props: map[string]string{
				model.SystemPropertyRoleName: "SAP BTP Subaccount",
				model.SystemPropertyRoleID:   "SCP_SUBACCOUNT",
			},
			expected: false,
		},
		{
			name:     "both role keys empty",
			cfg:      roleCfg,
			props:    map[string]string{},
			expected: true,
		},
		{
			name: "role name present but role id empty",
			cfg:  roleCfg,
			props: map[string]string{
				model.SystemPropertyRoleName: "SAP BTP Subaccount",
			},
			expected: true,
		},
		{
			name: "role key empty string",
			cfg:  roleCfg,
			props: map[string]string{
				model.SystemPropertyRoleName: "SAP BTP Subaccount",
				model.SystemPropertyRoleID:   "",
			},
			expected: true,
		},
		{
			name:     "role keys not configured",
			cfg:      &config.System{OptionalProperties: map[string]config.SystemProperty{}},
			props:    map[string]string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sys := &model.System{Properties: tt.props}

			assert.Equal(t, tt.expected, sys.HasEmptyRole(tt.cfg))
		})
	}
}
