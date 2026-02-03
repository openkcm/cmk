package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/model"
)

func TestTagsTable(t *testing.T) {
	t.Run("Should have table name tags", func(t *testing.T) {
		expectedTableName := "tags"

		tableName := model.Tag{}.TableName()

		assert.Equal(t, expectedTableName, tableName)
	})

	t.Run("Should be a tenant table", func(t *testing.T) {
		assert.False(t, model.Tag{}.IsSharedModel())
	})
}
