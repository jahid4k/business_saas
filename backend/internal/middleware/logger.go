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
//   - path         /api/v1/hello
//   - status       200, 404, 500, etc.
//   - latency_ms   request duration in milliseconds
//   - ip           client IP
//   - request_id   from RequestID middleware (if present)
//
// Requests to /api/v1/health are logged at DEBUG level to avoid
// polluting logs with health check noise from Docker/load balancers.
func Logger() fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()

		// Continue to the next handler
		err := c.Next()

		latency := time.Since(start)
		status := c.Response().StatusCode()
		method := c.Method()
		path := c.Path()

		attrs := []any{
			slog.String("method", method),
			slog.String("path", path),
			slog.Int("status", status),
			slog.Int64("latency_ms", latency.Milliseconds()),
			slog.String("ip", c.IP()),
		}

		// Include request ID if set by RequestID middleware
		if id, ok := c.Locals("request_id").(string); ok && id != "" {
			attrs = append(attrs, slog.String("request_id", id))
		}

		// Health checks are very frequent — log at DEBUG to reduce noise
		if path == "/api/v1/health" {
			slog.Debug("request", attrs...)
			return err
		}

		// Log errors at WARN/ERROR level, normal requests at INFO
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
