package device

import (
	"sync"
	"time"
)

type cacheEntry struct {
	result    *CommandResult
	timestamp time.Time
}

type CommandCache struct {
	mu       sync.RWMutex
	entries  map[string]*cacheEntry
	maxSize  int
	ttl      time.Duration
	hits     int64
	misses   int64
	evictions int64
}

func NewCommandCache(maxSize int, ttl time.Duration) *CommandCache {
	cache := &CommandCache{
		entries: make(map[string]*cacheEntry),
		maxSize: maxSize,
		ttl:     ttl,
	}

	go cache.cleanupRoutine()
	return cache
}

func (c *CommandCache) Get(key string) *CommandResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		c.misses++
		return nil
	}

	if time.Since(entry.timestamp) > c.ttl {
		c.misses++
		return nil
	}

	c.hits++
	return entry.result
}

func (c *CommandCache) Set(key string, result *CommandResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	c.entries[key] = &cacheEntry{
		result:    result,
		timestamp: time.Now(),
	}
}

func (c *CommandCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*cacheEntry)
	c.hits = 0
	c.misses = 0
	c.evictions = 0
}

func (c *CommandCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.entries {
		if oldestKey == "" || entry.timestamp.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.timestamp
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
		c.evictions++
	}
}

func (c *CommandCache) cleanupRoutine() {
	ticker := time.NewTicker(c.ttl / 2)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

func (c *CommandCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.entries {
		if now.Sub(entry.timestamp) > c.ttl {
			delete(c.entries, key)
			c.evictions++
		}
	}
}

func (c *CommandCache) Stats() map[string]int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]int64{
		"hits":      c.hits,
		"misses":    c.misses,
		"evictions": c.evictions,
		"size":      int64(len(c.entries)),
	}
}