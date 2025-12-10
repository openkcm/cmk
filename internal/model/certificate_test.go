package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.tools.sap/kms/cmk/internal/model"
)

func TestCertificateTable(t *testing.T) {
	t.Run("Should have table name certificates", func(t *testing.T) {
		expectedTableName := "certificates"

		tableName := model.Certificate{}.TableName()

		assert.Equal(t, expectedTableName, tableName)
	})

	t.Run("Should be a tenant table", func(t *testing.T) {
		assert.False(t, model.Certificate{}.IsSharedModel())
	})
}
