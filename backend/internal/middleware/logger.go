// backend/internal/middleware/logger.go
package middleware

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
)

// Logger returns a Fiber middleware that logs every HTTP request using
// the stdlib structured logger (log/slog).
//
// Log fields per request:
//   - method       GET, POST, etc.
//   - path         /api/v1/tasks
//   - status       200, 404, 500, etc.
//   - latency_ms   request duration in milliseconds
//   - ip           client IP
//   - request_id   from RequestID middleware
//   - user_id      from RequireAuth middleware (empty on public routes)
//   - business_id  from RequireAuth middleware (empty before workspace select)
//
// This means every log line is self-contained — you can grep by request_id,
// user_id, or business_id and immediately see what happened and who triggered it.
//
// Health check requests are logged at DEBUG level to reduce noise.
func Logger() fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()

		// Run the rest of the chain first — we log after response is ready.
		err := c.Next()

		latency := time.Since(start)
		status := c.Response().StatusCode()

		attrs := []any{
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
			slog.Int("status", status),
			slog.Int64("latency_ms", latency.Milliseconds()),
			slog.String("ip", c.IP()),
		}

		// request_id — set by RequestID middleware, always present.
		if id, ok := c.Locals("request_id").(string); ok && id != "" {
			attrs = append(attrs, slog.String("request_id", id))
		}

		// user_id — set by RequireAuth. Empty on public routes (login, signup).
		// Lets you answer: "which user triggered this 500?"
		if uid, ok := c.Locals("user_id").(string); ok && uid != "" {
			attrs = append(attrs, slog.String("user_id", uid))
		}

		// business_id — set by RequireAuth from JWT bid claim.
		// Empty before workspace selection. Lets you answer:
		// "which tenant had this error?" — critical for multi-tenant debugging.
		if bid, ok := c.Locals("business_id").(string); ok && bid != "" {
			attrs = append(attrs, slog.String("business_id", bid))
		}

		// Health checks are very frequent — log at DEBUG to avoid noise.
		if c.Path() == "/api/v1/health" {
			slog.Debug("request", attrs...)
			return err
		}

		switch {
		case status >= 500:
			slog.Error("request", attrs...)
		case status >= 400:
			slog.Warn("request", attrs...)
		default:
			slog.Info("request", attrs...)
		}

		return err
	}
}
