package cache

import (
	"sync"
	"time"
)

// Item represents a cached item
type Item struct {
	value      interface{}
	expiration int64
}

// Cache represents a simple in-memory cache
type Cache struct {
	items             sync.Map
	defaultExpiration time.Duration
	cleanupInterval   time.Duration
}

// New creates a new cache instance
func New(defaultExpiration, cleanupInterval time.Duration) *Cache {
	cache := &Cache{
		defaultExpiration: defaultExpiration,
		cleanupInterval:   cleanupInterval,
	}
	go cache.cleanupExpired()
	return cache
}

// Set adds an item to the cache with a default expiration time
func (c *Cache) Set(key string, value interface{}) {
	c.SetWithExpiration(key, value, c.defaultExpiration)
}

// SetWithExpiration adds an item to the cache with a specified expiration time
func (c *Cache) SetWithExpiration(key string, value interface{}, expiration time.Duration) {
	var expiry int64
	if expiration > 0 {
		expiry = time.Now().Add(expiration).UnixNano()
	}
	c.items.Store(key, Item{value: value, expiration: expiry})
}

// Get retrieves an item from the cache
func (c *Cache) Get(key string) (interface{}, bool) {
	item, found := c.items.Load(key)
	if !found {
		return nil, false
	}
	cachedItem := item.(Item)
	if cachedItem.expiration > 0 && time.Now().UnixNano() > cachedItem.expiration {
		c.items.Delete(key)
		return nil, false
	}
	return cachedItem.value, true
}

// Delete removes an item from the cache
func (c *Cache) Delete(key string) {
	c.items.Delete(key)
}

// cleanupExpired periodically removes expired items from the cache
func (c *Cache) cleanupExpired() {
	ticker := time.NewTicker(c.cleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		c.items.Range(func(key, value interface{}) bool {
			item := value.(Item)
			if item.expiration > 0 && time.Now().UnixNano() > item.expiration {
				c.items.Delete(key)
			}
			return true
		})
	}
}

// DefaultCache is a convenient default cache instance
var DefaultCache = New(5*time.Minute, 1*time.Minute)