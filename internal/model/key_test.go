package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk-core/internal/model"
)

func TestKeyTable(t *testing.T) {
	t.Run("Should have table name keys", func(t *testing.T) {
		expectedTableName := "keys"

		tableName := model.Key{}.TableName()

		assert.Equal(t, expectedTableName, tableName)
	})

	t.Run("Should be a tenant table", func(t *testing.T) {
		assert.False(t, model.Key{}.IsSharedModel())
	})
}
