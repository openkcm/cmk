package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk-core/internal/model"
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
