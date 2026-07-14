package cache

import (
	"sync"
	"time"
)

type TTLItem[T any] struct {
	time time.Time
	item T
}

type TTLGC struct {
	Enabled  bool
	Interval time.Duration
}

type TTLConfig struct {
	ItemTTL  time.Duration
	GCConfig TTLGC
}

type TTLCache[T comparable, V any] struct {
	config TTLConfig

	mu    sync.Mutex
	cache map[T]TTLItem[V]
}

func NewTTLCache[T comparable, V any](conf TTLConfig) *TTLCache[T, V] {
	cache := &TTLCache[T, V]{
		config: conf,
		cache:  make(map[T]TTLItem[V]),
	}
	if conf.GCConfig.Enabled {
		cache.StartGC()
	}
	return cache
}

func (c *TTLCache[T, V]) Get(item T) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isValid(item) {
		delete(c.cache, item)
		var zero V
		return zero, false
	}
	return c.cache[item].item, true
}

func (c *TTLCache[T, V]) Set(item T, v V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[item] = TTLItem[V]{
		time: time.Now(),
		item: v,
	}
}

func (c *TTLCache[T, V]) StartGC() {
	ticker := time.NewTicker(c.config.GCConfig.Interval)

	go func() {
		defer ticker.Stop()

		for range ticker.C {
			c.mu.Lock()
			for item, v := range c.cache {
				if c.isExpired(v) {
					delete(c.cache, item)
				}
			}
			c.mu.Unlock()
		}
	}()
}

func (c *TTLCache[T, V]) isValid(item T) bool {
	v, ok := c.cache[item]
	if !ok {
		return false
	}

	return !c.isExpired(v)
}

func (c *TTLCache[T, V]) isExpired(item TTLItem[V]) bool {
	return time.Since(item.time) >= c.config.ItemTTL
}
