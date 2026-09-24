package security

import (
	"testing"
	"time"
)

func TestRateLimiter_GivenManyDistinctClients_WhenCapacityIsReached_ThenItEvictsAndStaysBounded(t *testing.T) {
	limiter := NewRateLimiter(2)
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }
	for _, key := range []string{"one", "two", "three"} {
		if allowed, _ := limiter.Allow(key, 2, time.Minute); !allowed {
			t.Fatalf("first request for %s was denied", key)
		}
		now = now.Add(time.Second)
	}
	if len(limiter.entries) != 2 {
		t.Fatalf("tracked clients = %d, want 2", len(limiter.entries))
	}
	if _, exists := limiter.entries["one"]; exists {
		t.Fatal("least recently seen client was not evicted")
	}
}

func TestRateLimiter_GivenRequestsInWindow_WhenLimitIsReached_ThenItReturnsRetryAfter(t *testing.T) {
	limiter := NewRateLimiter(2)
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }
	if allowed, _ := limiter.Allow("client", 1, time.Minute); !allowed {
		t.Fatal("first request was denied")
	}
	if allowed, retryAfter := limiter.Allow("client", 1, time.Minute); allowed || retryAfter != time.Minute {
		t.Fatalf("second request = allowed:%t retry:%s, want denied for one minute", allowed, retryAfter)
	}
}
