package middleware

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jack/golang-short-url-service/internal/config"
	"github.com/redis/go-redis/v9"
)

// RateLimiter implements a sliding window rate limiter using Redis
type RateLimiter struct {
	client   *redis.Client
	requests int
	duration time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(client *redis.Client, cfg *config.RateLimitConfig) *RateLimiter {
	return &RateLimiter{
		client:   client,
		requests: cfg.Requests,
		duration: cfg.Duration,
	}
}

// Middleware returns a Gin middleware for rate limiting
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get client IP
		ip := c.ClientIP()
		key := "ratelimit:" + ip

		ctx := c.Request.Context()

		// Use Redis pipeline for atomic operations
		pipe := rl.client.Pipeline()

		// Get current count
		now := time.Now().UnixNano()
		windowStart := now - rl.duration.Nanoseconds()

		// Remove old entries outside the window
		pipe.ZRemRangeByScore(ctx, key, "0", formatInt64(windowStart))

		// Count entries in the current window
		countCmd := pipe.ZCard(ctx, key)

		_, err := pipe.Exec(ctx)
		if err != nil && err != redis.Nil {
			// fail-open：Redis 出錯時不擋請求，但必須留下 log 方便追查
			log.Printf("rate_limit redis error (precheck): ip=%s path=%s err=%v", ip, c.Request.URL.Path, err)
			c.Next()
			return
		}

		count := countCmd.Val()

		// Check if rate limit exceeded
		if count >= int64(rl.requests) {
			c.Header("X-RateLimit-Limit", formatInt(rl.requests))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("X-RateLimit-Reset", formatInt64(time.Now().Add(rl.duration).Unix()))
			c.Header("Retry-After", formatInt(int(rl.duration.Seconds())))

			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":   "rate limit exceeded",
				"message": "Too many requests. Please try again later.",
			})
			return
		}

		// Add current request to the window
		pipe = rl.client.Pipeline()
		pipe.ZAdd(ctx, key, redis.Z{
			Score:  float64(now),
			Member: now,
		})
		pipe.Expire(ctx, key, rl.duration)
		if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
			// fail-open：寫入窗口失敗時不影響本次請求，但需要記錄
			log.Printf("rate_limit redis error (record): ip=%s path=%s err=%v", ip, c.Request.URL.Path, err)
		}

		// Set rate limit headers
		remaining := rl.requests - int(count) - 1
		if remaining < 0 {
			remaining = 0
		}

		c.Header("X-RateLimit-Limit", formatInt(rl.requests))
		c.Header("X-RateLimit-Remaining", formatInt(remaining))
		c.Header("X-RateLimit-Reset", formatInt64(time.Now().Add(rl.duration).Unix()))

		c.Next()
	}
}

func formatInt(n int) string {
	return formatInt64(int64(n))
}

func formatInt64(n int64) string {
	// Simple int to string conversion
	if n == 0 {
		return "0"
	}

	negative := n < 0
	if negative {
		n = -n
	}

	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}

	if negative {
		digits = append([]byte{'-'}, digits...)
	}

	return string(digits)
}

