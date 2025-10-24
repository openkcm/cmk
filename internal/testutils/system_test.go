package testutils_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk-core/internal/model"
	"github.com/openkcm/cmk-core/internal/testutils"
)

func TestCreateTestSystem(t *testing.T) {
	tests := []struct {
		name     string
		mutator  func(*model.System)
		validate func(*testing.T, *model.System)
	}{
		{
			name:    "DefaultSystem",
			mutator: func(_ *model.System) {},
			validate: func(t *testing.T, system *model.System) {
				t.Helper()
				assert.Equal(t, "test-region", system.Region)
				assert.Nil(t, system.KeyConfigurationID)
			},
		},
		{
			name: "ModifiedSystem",
			mutator: func(k *model.System) {
				k.Region = "custom-system"
			},
			validate: func(t *testing.T, system *model.System) {
				t.Helper()
				assert.Equal(t, "custom-system", system.Region)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			system := testutils.CreateTestSystem(tt.mutator)
			tt.validate(t, system)
		})
	}
}
