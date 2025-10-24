package testutils_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk-core/internal/testutils"
)

var ErrForced = errors.New("forced error")

// TestErrorForced - tests the ErrorForced helper.
func TestErrorForced(t *testing.T) {
	db, _ := testutils.NewTestDB(t, testutils.TestDBConfig{})

	t.Run("All callbacks", func(t *testing.T) {
		errForced := testutils.NewDBErrorForced(db, ErrForced)
		errForced.Register()
		callbacks := errForced.ExportCallbacks()
		assert.ElementsMatch(t, callbacks, testutils.ExportAll)
		errForced.Unregister()
		assert.Empty(t, errForced.ExportCallbacks())
	})

	t.Run("All callbacks Specified", func(t *testing.T) {
		errForced := testutils.NewDBErrorForced(db, ErrForced)
		errForced.WithCreate().WithUpdate().WithDelete().WithRow().WithRaw().WithQuery().Register()
		callbacks := errForced.ExportCallbacks()
		assert.ElementsMatch(t, callbacks, testutils.ExportAll)
		errForced.Unregister()
		assert.Empty(t, errForced.ExportCallbacks())
	})

	t.Run("Specific callback - Create", func(t *testing.T) {
		errForced := testutils.NewDBErrorForced(db, ErrForced)
		errForced.WithCreate().Register()
		callbacks := errForced.ExportCallbacks()
		assert.NotEmpty(t, callbacks)
		assert.Len(t, callbacks, 1)
		assert.Contains(t, callbacks, testutils.ExportCallbackCreate)
		assert.NotContains(t, callbacks, []string{
			testutils.ExportCallbackUpdate,
			testutils.ExportCallbackDelete,
			testutils.ExportCallbackQuery,
			testutils.ExportCallbackRow,
			testutils.ExportCallbackRaw,
		})
		errForced.Unregister()
		assert.Empty(t, errForced.ExportCallbacks())
	})

	t.Run("Specific callbacks - Create, Update, Delete", func(t *testing.T) {
		errForced := testutils.NewDBErrorForced(db, ErrForced)
		errForced.WithCreate().WithUpdate().WithDelete().Register()
		callbacks := errForced.ExportCallbacks()

		assert.NotEmpty(t, callbacks)

		expected := []string{
			testutils.ExportCallbackUpdate,
			testutils.ExportCallbackCreate,
			testutils.ExportCallbackDelete,
		}

		assert.Len(t, callbacks, len(expected))
		assert.ElementsMatch(t, callbacks, expected)
		assert.NotContains(t, callbacks, []string{
			testutils.ExportCallbackQuery,
			testutils.ExportCallbackRow,
			testutils.ExportCallbackRaw,
		})

		errForced.Unregister()
		assert.Empty(t, errForced.ExportCallbacks())
	})

	t.Run("Specific callbacks - Create, Query, Raw, Row", func(t *testing.T) {
		errForced := testutils.NewDBErrorForced(db, ErrForced)
		errForced.WithCreate().WithQuery().WithRaw().WithRow().Register()
		callbacks := errForced.ExportCallbacks()

		assert.NotEmpty(t, callbacks)

		expected := []string{
			testutils.ExportCallbackCreate,
			testutils.ExportCallbackQuery,
			testutils.ExportCallbackRaw,
			testutils.ExportCallbackRow,
		}

		assert.Len(t, callbacks, len(expected))
		assert.ElementsMatch(t, callbacks, expected)
		assert.NotContains(t, callbacks, []string{
			testutils.ExportCallbackDelete,
			testutils.ExportCallbackUpdate,
		})

		errForced.Unregister()
		assert.Empty(t, errForced.ExportCallbacks())
	})
}
