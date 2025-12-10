package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.tools.sap/kms/cmk/internal/model"
)

func TestKeystoreConfigTable(t *testing.T) {
	t.Run("Should have table name public.keystore_configurations", func(t *testing.T) {
		expectedTableName := "public.keystore_configurations"

		tableName := model.KeystoreConfiguration{}.TableName()

		assert.Equal(t, expectedTableName, tableName)
	})

	t.Run("Should be a public table", func(t *testing.T) {
		assert.True(t, model.KeystoreConfiguration{}.IsSharedModel())
	})
}
