package device

import (
	"sync"
	"sync/atomic"
	"time"
)

type cacheEntry struct {
	result    *CommandResult
	timestamp time.Time
}

type CommandCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	maxSize int
	ttl     time.Duration

	// Counters are accessed concurrently by Get under RLock (read path,
	// hot) and by Set/evictOldest/cleanup under Lock. Atomics let the
	// read path stay lock-free for counter mutation while still being
	// race-free.
	hits      atomic.Int64
	misses    atomic.Int64
	evictions atomic.Int64
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
		c.misses.Add(1)
		return nil
	}

	if time.Since(entry.timestamp) > c.ttl {
		c.misses.Add(1)
		return nil
	}

	c.hits.Add(1)
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
	c.hits.Store(0)
	c.misses.Store(0)
	c.evictions.Store(0)
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
		c.evictions.Add(1)
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
			c.evictions.Add(1)
		}
	}
}

func (c *CommandCache) Stats() map[string]int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]int64{
		"hits":      c.hits.Load(),
		"misses":    c.misses.Load(),
		"evictions": c.evictions.Load(),
		"size":      int64(len(c.entries)),
	}
}