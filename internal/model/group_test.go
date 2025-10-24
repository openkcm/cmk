package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk-core/internal/model"
)

func TestGroupTable(t *testing.T) {
	t.Run("Should have table name group", func(t *testing.T) {
		expectedTableName := "group"

		tableName := model.Group{}.TableName()

		assert.Equal(t, expectedTableName, tableName)
	})

	t.Run("Should be a tenant table", func(t *testing.T) {
		assert.False(t, model.Group{}.IsSharedModel())
	})
}
