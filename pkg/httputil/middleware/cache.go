package middleware

import (
	"sync"
	"time"
)

// Cache is a simple in-memory cache with expiration
type Cache struct {
	items map[string]cacheItem
	sync.RWMutex
}

// cacheItem holds cached data along with its expiration
type cacheItem struct {
	value      interface{}
	expiration time.Time
}

// NewCache creates a new Cache
func NewCache() *Cache {
	return &Cache{
		items: make(map[string]cacheItem),
	}
}

// Set adds an item to the cache with a specified expiration duration
func (c *Cache) Set(key string, value interface{}, duration time.Duration) {
	c.Lock()
	defer c.Unlock()
	c.items[key] = cacheItem{
		value:      value,
		expiration: time.Now().Add(duration),
	}
}

// Get retrieves an item from the cache
func (c *Cache) Get(key string) (interface{}, bool) {
	c.RLock()
	defer c.RUnlock()
	item, found := c.items[key]
	if !found {
		return nil, false
	}
	if time.Now().After(item.expiration) {
		delete(c.items, key)
		return nil, false
	}
	return item.value, true
}

// CleanupExpired removes expired items from the cache
func (c *Cache) CleanupExpired() {
	c.Lock()
	defer c.Unlock()
	for key, item := range c.items {
		if time.Now().After(item.expiration) {
			delete(c.items, key)
		}
	}
}
