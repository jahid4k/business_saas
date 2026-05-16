// backend/internal/middleware/ratelimit.go
package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"

	"github.com/mridha/businesssaas/pkg/response"
)

// RateLimitConfig configures a Redis-backed sliding-window rate limiter.
type RateLimitConfig struct {
	Max           int    // maximum requests allowed in the window
	WindowSeconds int    // window size in seconds
	KeyPrefix     string // Redis key prefix — use different prefixes per route group
}

// NewAuthRateLimit returns a strict rate limiter for sensitive auth endpoints.
// Default: 10 requests per IP per 15 minutes.
// This is the main defence against brute-force login and signup spam.
func NewAuthRateLimit(redisClient *redis.Client) fiber.Handler {
	return newRateLimiter(redisClient, RateLimitConfig{
		Max:           10,
		WindowSeconds: 900, // 15 minutes
		KeyPrefix:     "rl:auth:",
	})
}

// NewAPIRateLimit returns a general rate limiter for authenticated API endpoints.
// Default: 300 requests per IP per minute.
func NewAPIRateLimit(redisClient *redis.Client) fiber.Handler {
	return newRateLimiter(redisClient, RateLimitConfig{
		Max:           300,
		WindowSeconds: 60,
		KeyPrefix:     "rl:api:",
	})
}

// newRateLimiter is the internal sliding-window rate limiter factory.
//
// Algorithm: Redis INCR + EXPIRE
//   - Key: <prefix><ip>
//   - On first request in window: INCR creates key with count=1, then EXPIRE sets TTL
//   - Subsequent requests: INCR increments count; EXPIRE is not reset (fixed window)
//   - When count > Max: return 429 with Retry-After header
//
// This is a fixed-window counter, which is simple and safe for auth rate limiting.
// It is not a true sliding window but is sufficient for Phase 1 auth protection.
// A true sliding window using a sorted set can be added in Phase 2 if needed.
func newRateLimiter(redisClient *redis.Client, cfg RateLimitConfig) fiber.Handler {
	return func(c fiber.Ctx) error {
		ip := c.IP()
		key := cfg.KeyPrefix + ip

		ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
		defer cancel()

		// Increment counter for this IP
		count, err := redisClient.Incr(ctx, key).Result()
		if err != nil {
			// If Redis is unreachable, fail open (allow the request) and log
			slog.Error("rate limiter: redis error, failing open",
				slog.String("key", key),
				slog.Any("error", err),
			)
			return c.Next()
		}

		// On the first request in this window, set the TTL
		if count == 1 {
			ttl := time.Duration(cfg.WindowSeconds) * time.Second
			if err := redisClient.Expire(ctx, key, ttl).Err(); err != nil {
				slog.Warn("rate limiter: failed to set TTL",
					slog.String("key", key),
					slog.Any("error", err),
				)
			}
		}

		// Set informational headers on every request
		remaining := cfg.Max - int(count)
		if remaining < 0 {
			remaining = 0
		}
		c.Set("X-RateLimit-Limit", strconv.Itoa(cfg.Max))
		c.Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Set("X-RateLimit-Window", fmt.Sprintf("%ds", cfg.WindowSeconds))

		// Reject if over limit
		if int(count) > cfg.Max {
			c.Set("Retry-After", strconv.Itoa(cfg.WindowSeconds))
			return response.TooManyRequests(c,
				"RATE_LIMIT_EXCEEDED",
				fmt.Sprintf("Too many requests. Please try again in %d seconds.", cfg.WindowSeconds),
			)
		}

		return c.Next()
	}
}

// AuthRateLimit is the legacy stub signature kept for any existing call sites.
// Prefer NewAuthRateLimit(redisClient) for new code.
func AuthRateLimit() fiber.Handler {
	// Falls back to a no-op if called without Redis (e.g. in tests that import the package)
	return func(c fiber.Ctx) error {
		return c.Next()
	}
}
