package security

import (
	"sync"
	"time"
)

// RateLimiter has a fixed maximum number of tracked keys. It evicts the least
// recently seen key only when it reaches capacity, so ordinary requests never
// scan the complete key set.
type RateLimiter struct {
	mu         sync.Mutex
	entries    map[string]rateEntry
	maxEntries int
	now        func() time.Time
}

type rateEntry struct {
	values   []time.Time
	lastSeen time.Time
}

func NewRateLimiter(maxEntries int) *RateLimiter {
	if maxEntries < 1 {
		maxEntries = 1
	}
	return &RateLimiter{entries: map[string]rateEntry{}, maxEntries: maxEntries, now: time.Now}
}

func (limiter *RateLimiter) Allow(key string, limit int, window time.Duration) (bool, time.Duration) {
	if limit < 1 || window <= 0 {
		return false, 0
	}
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	now := limiter.now().UTC()
	entry, found := limiter.entries[key]
	if !found && len(limiter.entries) >= limiter.maxEntries {
		limiter.evictOne()
	}
	active := entry.values[:0]
	for _, value := range entry.values {
		if now.Sub(value) < window {
			active = append(active, value)
		}
	}
	entry.values = active
	entry.lastSeen = now
	if len(entry.values) >= limit {
		limiter.entries[key] = entry
		return false, window - now.Sub(entry.values[0])
	}
	entry.values = append(entry.values, now)
	limiter.entries[key] = entry
	return true, 0
}

func (limiter *RateLimiter) evictOne() {
	var oldestKey string
	var oldest time.Time
	for key, entry := range limiter.entries {
		if oldestKey == "" || entry.lastSeen.Before(oldest) {
			oldestKey, oldest = key, entry.lastSeen
		}
	}
	delete(limiter.entries, oldestKey)
}
