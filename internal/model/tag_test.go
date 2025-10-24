package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk-core/internal/model"
)

func TestTagsTable(t *testing.T) {
	t.Run("Should have table name tags", func(t *testing.T) {
		expectedTableName := "key_configuration_tags"

		tableName := model.KeyConfigurationTag{}.TableName()

		assert.Equal(t, expectedTableName, tableName)
	})

	t.Run("Should be a tenant table", func(t *testing.T) {
		assert.False(t, model.KeyConfigurationTag{}.IsSharedModel())
	})
}
