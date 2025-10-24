package mock_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/mock"
)

func TestInMemoryMultitenancyDB_CreateDB(t *testing.T) {
	// Arrange
	mtDB := mock.NewInMemoryMultitenancyDB()
	tenantID := "tenant1"

	// Act
	db, err := mtDB.CreateDB(tenantID)
	// Assert

	assert.NoError(t, err)
	assert.NotNil(t, db)
}

func TestInMemoryMultitenancyDB_GetDB(t *testing.T) {
	// Arrange
	mtDB := mock.NewInMemoryMultitenancyDB()
	tenantID := "tenant1"
	keyID := uuid.New()
	key := model.Key{ID: keyID, Name: "test1"}

	// Act
	db, err := mtDB.CreateDB(tenantID)
	assert.NoError(t, err)
	assert.NotNil(t, db)

	err = db.Create(key)
	assert.NoError(t, err)
	// Assert
	retrievedKey, err := mtDB.GetDB(tenantID).Get(key)
	assert.NoError(t, err)

	result, ok := retrievedKey.(model.Key)
	assert.True(t, ok)
	assert.Equal(t, key, result)
}
