package testutils_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/testutils"
)

// TestRelatedModel represents a model for testing preload functionality
func TestSetupTestDB(t *testing.T) {
	db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})
	assert.NotNil(t, db)

	sqlDB, err := db.DB.DB()
	assert.NoError(t, err)
	assert.NoError(t, sqlDB.Ping())

	assert.True(t, db.Migrator().HasTable(&model.Tenant{}))
}
