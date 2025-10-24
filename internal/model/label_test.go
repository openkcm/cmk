package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/model"
)

func TestKeyLabelsTable(t *testing.T) {
	t.Run("Should have table name key_labels", func(t *testing.T) {
		expectedTableName := "key_labels"

		tableName := model.KeyLabel{}.TableName()

		assert.Equal(t, expectedTableName, tableName)
	})

	t.Run("Should be a tenant table", func(t *testing.T) {
		assert.False(t, model.KeyLabel{}.IsSharedModel())
	})
}
