package manager_test

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo/sql"
	"github.com/openkcm/cmk/internal/testutils"
)

var itemID = uuid.New()

func TestPool_Add(t *testing.T) {
	t.Run("should save Configuration in repo", func(t *testing.T) {
		db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{
			CreateDatabase: true,
		})

		testRepo := sql.NewRepository(db)
		testPool := manager.NewPool(testRepo)
		item := testutils.NewKeystore(func(kc *model.Keystore) {
			kc.ID = itemID
		})

		addedItem, err := testPool.Add(t.Context(), item)

		require.NoError(t, err)
		require.Equal(t, item, addedItem)
	})
}

func TestPool_Pop(t *testing.T) {
	t.Run("should get first available Configuration from repo", func(t *testing.T) {
		db, _, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})

		testRepo := sql.NewRepository(db)
		testPool := manager.NewPool(testRepo)

		item := testutils.NewKeystore(func(kc *model.Keystore) {
			kc.ID = itemID
		})
		addedItem, err := testPool.Add(t.Context(), item)
		require.NoError(t, err)
		require.Equal(t, item, addedItem)

		wg := sync.WaitGroup{}
		wg.Add(2)

		var foundConfig *model.Keystore

		var encounteredError bool

		go func() {
			receivedItem, popErr := testPool.Pop(t.Context())
			if popErr != nil {
				encounteredError = true
			} else {
				foundConfig = receivedItem
			}

			wg.Done()
		}()

		go func() {
			receivedItem, popErr := testPool.Pop(t.Context())
			if popErr != nil {
				encounteredError = true
			} else if receivedItem != nil {
				foundConfig = receivedItem
			}

			wg.Done()
		}()

		wg.Wait()

		require.Equal(t, addedItem.ID, foundConfig.ID)
		require.Equal(t, addedItem.Provider, foundConfig.Provider)

		var expectedValue, actualValue map[string]any

		err = json.Unmarshal(addedItem.Config, &expectedValue)
		require.NoError(t, err)
		err = json.Unmarshal(foundConfig.Config, &actualValue)
		require.NoError(t, err)
		require.Equal(t, expectedValue, actualValue)
		require.True(t, encounteredError)
	})
}
