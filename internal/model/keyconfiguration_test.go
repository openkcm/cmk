package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk-core/internal/model"
)

func TestKeyConfiguration(t *testing.T) {
	t.Run("Should have table name key_configurations", func(t *testing.T) {
		expectedTableName := "key_configurations"

		tableName := model.KeyConfiguration{}.TableName()

		assert.Equal(t, expectedTableName, tableName)
	})

	t.Run("Should be a tenant table", func(t *testing.T) {
		assert.False(t, model.KeyConfiguration{}.IsSharedModel())
	})
}
