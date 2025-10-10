package mock_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/mock"
	"github.com/openkcm/cmk/internal/testutils"
)

func TestInMemoryRepository_Create(t *testing.T) {
	// Arrange
	ctx := testutils.CreateCtxWithTenant(mock.TenantID)
	mockRepo := mock.NewInMemoryRepository()

	keyID := uuid.New()
	key := model.Key{ID: keyID, Name: "test1"}
	getKey := model.Key{ID: keyID}

	// Act
	err := mockRepo.Create(ctx, &key)
	assert.NoError(t, err)

	// Assert
	ok, err := mockRepo.First(ctx, &getKey, repo.Query{})
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, key.ID, getKey.ID)
	assert.Equal(t, key.Name, getKey.Name)
}

func TestInMemoryRepository_List(t *testing.T) {
	// Arrange
	ctx := testutils.CreateCtxWithTenant(mock.TenantID)
	mockRepo := mock.NewInMemoryRepository()

	keyID := uuid.New()
	key := model.Key{ID: keyID, Name: "test1"}
	keyID2 := uuid.New()
	key2 := model.Key{ID: keyID2, Name: "test2"}
	keyID3 := uuid.New()
	key3 := model.Key{ID: keyID3, Name: "test3"}

	err := mockRepo.Create(ctx, key)
	assert.NoError(t, err)
	err = mockRepo.Create(ctx, key2)
	assert.NoError(t, err)
	err = mockRepo.Create(ctx, key3)
	assert.NoError(t, err)

	// Act
	var result []model.Key

	count, err := mockRepo.List(ctx, model.Key{}, &result, repo.Query{})

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestInMemoryRepository_First(t *testing.T) {
	// Arrange
	ctx := testutils.CreateCtxWithTenant(mock.TenantID)
	mockRepo := mock.NewInMemoryRepository()

	keyID := uuid.New()
	key := model.Key{ID: keyID, Name: "test1"}

	// Act
	err := mockRepo.Create(ctx, key)
	assert.NoError(t, err)

	result := model.Key{ID: keyID}
	ok, err := mockRepo.First(ctx, &result, repo.Query{})

	// Assert
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, key, result)
}

func TestInMemoryRepository_Delete(t *testing.T) {
	// Arrange
	ctx := testutils.CreateCtxWithTenant(mock.TenantID)
	mockRepo := mock.NewInMemoryRepository()

	keyID := uuid.New()
	key := model.Key{ID: keyID, Name: "test1"}

	// Act
	err := mockRepo.Create(ctx, key)
	assert.NoError(t, err)

	result := model.Key{ID: keyID}
	ok, err := mockRepo.Delete(ctx, &result, repo.Query{})
	assert.NoError(t, err)
	assert.True(t, ok)

	ok, err = mockRepo.First(ctx, &result, repo.Query{})
	// Assert
	assert.Error(t, err)
	assert.False(t, ok)
}

func TestInMemoryRepository_Patch(t *testing.T) {
	// Arrange
	ctx := testutils.CreateCtxWithTenant(mock.TenantID)
	mockRepo := mock.NewInMemoryRepository()

	keyID := uuid.New()
	key := model.Key{ID: keyID, Name: "test1"}

	// Act
	err := mockRepo.Create(ctx, key)
	assert.NoError(t, err)

	newKey := model.Key{ID: keyID, Name: "test2"}
	ok, err := mockRepo.Patch(ctx, &newKey, repo.Query{})
	assert.NoError(t, err)
	assert.True(t, ok)

	result := model.Key{ID: keyID}
	ok, err = mockRepo.First(ctx, &result, repo.Query{})
	// Assert
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, newKey, result)
}

func TestInMemoryRepository_Set(t *testing.T) {
	ctx := testutils.CreateCtxWithTenant(mock.TenantID)
	mockRepo := mock.NewInMemoryRepository()

	t.Run("Should throw error, empty tenantID context", func(t *testing.T) {
		ctx := testutils.CreateCtxWithTenant("")
		newKey := model.Key{ID: uuid.New(), Name: "test2"}
		err := mockRepo.Set(ctx, &newKey)
		assert.Error(t, err)
	})

	t.Run("Should create if empty", func(t *testing.T) {
		keyID := uuid.New()

		newKey := model.Key{ID: keyID, Name: "test2"}
		err := mockRepo.Set(ctx, &newKey)
		assert.NoError(t, err)

		result := model.Key{ID: keyID}
		ok, err := mockRepo.First(ctx, &result, repo.Query{})
		// Assert
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, newKey, result)
	})

	t.Run("Should override if not empty", func(t *testing.T) {
		keyID := uuid.New()
		key := model.Key{ID: keyID, Name: "test1"}

		// Act
		err := mockRepo.Create(ctx, key)
		assert.NoError(t, err)

		newKey := model.Key{ID: keyID, Name: "test2"}
		err = mockRepo.Set(ctx, &newKey)
		assert.NoError(t, err)

		result := model.Key{ID: keyID}
		ok, err := mockRepo.First(ctx, &result, repo.Query{})
		// Assert
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, newKey, result)
	})
}

func TestInMemoryRepository_Transaction(t *testing.T) {
	mockRepo := mock.NewInMemoryRepository()
	m := model.Key{ID: uuid.New(), Name: "test1"}

	t.Run("Should rollback on error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(testutils.CreateCtxWithTenant(mock.TenantID))

		_ = mockRepo.Transaction(ctx, func(ctx context.Context, r repo.Repo) error {
			err := r.Create(ctx, &m)
			assert.NoError(t, err)

			cancel()

			return nil
		})

		res := model.Key{ID: m.ID}
		ok, err := mockRepo.First(ctx, &res, repo.Query{})
		assert.NoError(t, err)
		assert.True(t, ok)
	})
}
