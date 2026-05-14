package middleware

import "github.com/gofiber/fiber/v3"

// RateLimitConfig configures the rate limiter for a specific route group.
type RateLimitConfig struct {
	// Max is the maximum number of requests allowed in the window.
	Max int

	// WindowSeconds is the sliding window duration in seconds.
	WindowSeconds int

	// KeyPrefix is the Redis key prefix for this limiter.
	// Different route groups should use different prefixes.
	KeyPrefix string
}

// AuthRateLimit returns a rate limiter configured for sensitive auth endpoints
// (login, signup, password reset). Uses a strict limit to prevent brute force.
//
// Default: 10 requests per IP per 15 minutes.
//
// STATUS: Phase 1-B stub — Redis-backed sliding window limiter to be
// implemented once the Redis client is wired into middleware.
func AuthRateLimit() fiber.Handler {
	return rateLimiter(RateLimitConfig{
		Max:           10,
		WindowSeconds: 900, // 15 minutes
		KeyPrefix:     "rl:auth:",
	})
}

// APIRateLimit returns a rate limiter for general authenticated API endpoints.
//
// Default: 300 requests per IP per minute.
//
// STATUS: Phase 1-D stub.
func APIRateLimit() fiber.Handler {
	return rateLimiter(RateLimitConfig{
		Max:           300,
		WindowSeconds: 60,
		KeyPrefix:     "rl:api:",
	})
}

// rateLimiter is the internal factory for rate limit middleware.
func rateLimiter(_ RateLimitConfig) fiber.Handler {
	return func(c fiber.Ctx) error {
		// TODO (Phase 1-B): implement Redis sliding window rate limiter
		// 1. Build key: cfg.KeyPrefix + clientIP (or userID post-auth)
		// 2. INCR key in Redis with TTL set on first increment
		// 3. If count > cfg.Max → return response.TooManyRequests(...)
		// 4. Set X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset headers
		return c.Next()
	}
}
