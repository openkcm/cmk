package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/model"
)

func TestKeyVersionTable(t *testing.T) {
	t.Run("Should have table name key_versions", func(t *testing.T) {
		expectedTableName := "key_versions"

		tableName := model.KeyVersion{}.TableName()

		assert.Equal(t, expectedTableName, tableName)
	})

	t.Run("Should be a tenant table", func(t *testing.T) {
		assert.False(t, model.KeyVersion{}.IsSharedModel())
	})
}
