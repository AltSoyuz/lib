package httpserver

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/VictoriaMetrics/metrics"
)

// IPRateLimiter implements a fixed-window rate limiter keyed by client IP.
//
// Design (VictoriaMetrics style):
//   - No goroutines: cleanup is lazy, triggered inside Allow.
//   - Single mutex over a plain map: predictable, no lock-free magic.
//   - Metrics via VictoriaMetrics counters: one label per endpoint.
type IPRateLimiter struct {
	mu          sync.Mutex
	entries     map[string]*ipEntry
	maxReqs     int
	window      time.Duration
	lastCleanup time.Time
}

type ipEntry struct {
	count     int
	windowEnd time.Time
}

const cleanupInterval = 10 * time.Minute

// NewIPRateLimiter returns a limiter that allows maxReqs requests per IP within window.
func NewIPRateLimiter(maxReqs int, window time.Duration) *IPRateLimiter {
	return &IPRateLimiter{
		entries:     make(map[string]*ipEntry),
		maxReqs:     maxReqs,
		window:      window,
		lastCleanup: time.Now(),
	}
}

// Allow reports whether the given IP is within the rate limit.
// Expired entries are swept lazily every cleanupInterval to keep memory bounded.
func (l *IPRateLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()

	if now.Sub(l.lastCleanup) >= cleanupInterval {
		for k, e := range l.entries {
			if now.After(e.windowEnd) {
				delete(l.entries, k)
			}
		}
		l.lastCleanup = now
	}

	e, ok := l.entries[ip]
	if !ok || now.After(e.windowEnd) {
		l.entries[ip] = &ipEntry{count: 1, windowEnd: now.Add(l.window)}
		return true
	}
	e.count++
	return e.count <= l.maxReqs
}

// Check reports whether the caller's IP is within the rate limit.
// On rejection it writes a 429 response with a Retry-After header and returns
// false; the caller must return immediately (mirrors the CheckSession helper).
func (l *IPRateLimiter) Check(w http.ResponseWriter, r *http.Request) bool {
	if l.Allow(ClientIP(r)) {
		return true
	}
	metrics.GetOrCreateCounter(
		fmt.Sprintf(`app_rate_limited_total{endpoint=%q}`, r.URL.Path),
	).Inc()
	w.Header().Set("Retry-After", fmt.Sprintf("%.0f", l.window.Seconds()))
	WriteError(w, r, http.StatusTooManyRequests, fmt.Errorf("rate_limit_exceeded"))
	return false
}
