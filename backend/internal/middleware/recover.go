package middleware

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/response"
)

// Recover catches any panic that occurs in a handler and converts it
// into a clean 500 Internal Server Error response.
//
// Without this, a panic would crash the goroutine processing the request
// and potentially bring down the server.
//
// The panic value and stack trace are logged server-side at ERROR level.
// The client only receives a generic error message — no stack traces,
// no internal details are ever exposed.
func Recover() fiber.Handler {
	return func(c fiber.Ctx) (err error) {
		defer func() {
			if r := recover(); r != nil {
				// Log the panic with full context server-side
				slog.Error("panic recovered",
					slog.Any("panic", r),
					slog.String("method", c.Method()),
					slog.String("path", c.Path()),
					slog.String("request_id", func() string {
						if id, ok := c.Locals("request_id").(string); ok {
							return id
						}
						return ""
					}()),
				)

				// Return a safe generic error to the client
				err = response.InternalServerError(c)
			}
		}()

		return c.Next()
	}
}
