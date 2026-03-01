package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// visitor tracks the request count for a single client within the current
// window.
type visitor struct {
	count     int
	windowEnd time.Time
}

// RateLimiter holds the per-IP state for an in-memory token-bucket style
// rate limiter.
type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	limit    int
	window   time.Duration
}

// NewRateLimiter creates a RateLimiter that allows at most `limit` requests per
// `window` duration per unique client IP.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		limit:    limit,
		window:   window,
	}
	// Background goroutine that evicts stale entries every minute so the map
	// does not grow unboundedly over time.
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if now.After(v.windowEnd) {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// allow returns true when the caller identified by `key` is within the allowed
// rate, and false when the limit has been exceeded.
func (rl *RateLimiter) allow(key string) bool {
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, ok := rl.visitors[key]
	if !ok || now.After(v.windowEnd) {
		rl.visitors[key] = &visitor{count: 1, windowEnd: now.Add(rl.window)}
		return true
	}

	if v.count >= rl.limit {
		return false
	}
	v.count++
	return true
}

// Middleware returns a Gin handler that enforces the rate limit.
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !rl.allow(ip) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "too many requests, please slow down",
				"data":    nil,
			})
			return
		}
		c.Next()
	}
}
