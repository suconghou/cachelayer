package ttlcache

import (
	"sync"
	"time"
)

type CacheItem[V any] struct {
	Value      V
	Expiration time.Time
}

type Cache[K comparable, V any] struct {
	mu          sync.Mutex
	items       map[K]CacheItem[V]
	defaultTTL  time.Duration
	lastCleanup time.Time // 上次清理时间
}

func NewCache[K comparable, V any](defaultTTL time.Duration) *Cache[K, V] {
	return &Cache[K, V]{
		items:       make(map[K]CacheItem[V]),
		defaultTTL:  defaultTTL,
		lastCleanup: time.Now(), // 初始化上次清理时间
	}
}

// cleanup 在持有锁的情况下清理过期项目
func (c *Cache[K, V]) cleanup(now time.Time) {
	if now.Sub(c.lastCleanup) > c.defaultTTL {
		for key, item := range c.items {
			if now.After(item.Expiration) {
				delete(c.items, key)
			}
		}
		c.lastCleanup = now
	}
}

func (c *Cache[K, V]) Set(key K, value V, ttl time.Duration) {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cleanup(now)
	if ttl < time.Millisecond {
		ttl = c.defaultTTL
	}
	c.items[key] = CacheItem[V]{
		Value:      value,
		Expiration: now.Add(ttl),
	}
}

func (c *Cache[K, V]) Get(key K) (V, bool) {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cleanup(now)
	item, found := c.items[key]
	if !found {
		var zero V
		return zero, false
	}
	if now.After(item.Expiration) {
		delete(c.items, key)
		var zero V
		return zero, false
	}
	return item.Value, true
}
