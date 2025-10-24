package model_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk-core/internal/model"
)

func TestImportParamsTable(t *testing.T) {
	t.Run("Should have table name import_params", func(t *testing.T) {
		expectedTableName := "import_params"

		tableName := model.ImportParams{}.TableName()

		assert.Equal(t, expectedTableName, tableName)
	})

	t.Run("Should be a tenant table", func(t *testing.T) {
		assert.False(t, model.ImportParams{}.IsSharedModel())
	})
}

func TestImportParams_IsExpired(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	ipNoExpiry := model.ImportParams{}
	assert.False(t, ipNoExpiry.IsExpired())

	ipExpired := model.ImportParams{Expires: &past}
	assert.True(t, ipExpired.IsExpired())

	ipNotExpired := model.ImportParams{Expires: &future}
	assert.False(t, ipNotExpired.IsExpired())
}
