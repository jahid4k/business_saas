// Package middleware contains all Fiber middleware used by BusinessSAAS.
// Each middleware is a single file with a single responsibility.
package middleware

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// RequestID injects a unique request ID into every request.
//
// The ID is set in two places:
//   - c.Locals("request_id") — available to all subsequent handlers
//   - X-Request-ID response header — visible to API clients and logs
//
// If the client sends an X-Request-ID header, that value is used.
// Otherwise, a new UUID v4 is generated.
//
// The request ID is included in every API response envelope via
// pkg/response helpers, making it easy to correlate client errors
// with server logs.
func RequestID() fiber.Handler {
	return func(c fiber.Ctx) error {
		// Honour client-provided request ID (useful for distributed tracing)
		id := c.Get("X-Request-ID")
		if id == "" {
			id = uuid.New().String()
		}

		// Make available to downstream handlers and response helpers
		c.Locals("request_id", id)

		// Echo back in response header
		c.Set("X-Request-ID", id)

		return c.Next()
	}
}
