package cache_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/utils/cache"
)

func TestTTLCache(t *testing.T) {
	gcInterval := 200 * time.Millisecond
	cache := cache.NewTTLCache[string, string](cache.TTLConfig{
		ItemTTL: 100 * time.Millisecond,
		GCConfig: cache.TTLGC{
			Enabled:  true,
			Interval: gcInterval,
		},
	})

	t.Run("Should return empty when not cached", func(t *testing.T) {
		v, ok := cache.Get(uuid.NewString())
		assert.Empty(t, v)
		assert.False(t, ok)
	})

	t.Run("Should cache and be cleaned by garbage collector", func(t *testing.T) {
		k := uuid.NewString()
		expected := uuid.NewString()
		cache.Set(k, expected)

		v, ok := cache.Get(k)
		assert.Equal(t, expected, v)
		assert.True(t, ok)

		time.Sleep(gcInterval)
		v, ok = cache.Get(k)
		assert.Empty(t, v)
		assert.False(t, ok)
	})
}
