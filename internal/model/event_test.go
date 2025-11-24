package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/model"
)

func TestEventTable(t *testing.T) {
	t.Run("Should have table name event", func(t *testing.T) {
		expectedTableName := "events"

		tableName := model.Event{}.TableName()

		assert.Equal(t, expectedTableName, tableName)
	})

	t.Run("Should be a tenant table", func(t *testing.T) {
		assert.False(t, model.Event{}.IsSharedModel())
	})
}
