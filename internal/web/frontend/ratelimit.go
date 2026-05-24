package frontend

import (
	"sync"
	"time"
)

const (
	maxCertsPerHour = 10
	rateLimitWindow = time.Hour
)

// rateLimiter tracks per-user cert signing rate.
type rateLimiter struct {
	mu      sync.Mutex
	entries map[string][]time.Time
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{entries: make(map[string][]time.Time)}
}

// allow checks if the user can sign. Returns (allowed, remaining, resetIn).
func (rl *rateLimiter) allow(user string) (bool, int, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rateLimitWindow)

	// Prune old entries
	var recent []time.Time
	for _, t := range rl.entries[user] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	rl.entries[user] = recent

	remaining := maxCertsPerHour - len(recent)
	if remaining <= 0 {
		// Find when the oldest entry expires
		resetIn := rl.entries[user][0].Add(rateLimitWindow).Sub(now)
		return false, 0, resetIn
	}
	return true, remaining, 0
}

// record logs a successful signing.
func (rl *rateLimiter) record(user string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.entries[user] = append(rl.entries[user], time.Now())
}

// remaining returns how many signs the user has left this hour.
func (rl *rateLimiter) remaining(user string) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rateLimitWindow)

	count := 0
	for _, t := range rl.entries[user] {
		if t.After(cutoff) {
			count++
		}
	}
	remaining := maxCertsPerHour - count
	if remaining < 0 {
		return 0
	}
	return remaining
}
