// backend/internal/middleware/auth.go
package middleware

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"

	jwtpkg "github.com/mridha/businesssaas/pkg/jwt"
	"github.com/mridha/businesssaas/pkg/response"
)

// RequireAuth returns a middleware that validates the JWT access token.
//
// Expects: Authorization: Bearer <access_token>
//
// On success, sets in c.Locals:
//   - "user_id"     string
//   - "email"       string
//   - "business_id" string (may be empty before workspace selection)
//   - "role"        string (may be empty before workspace selection)
//
// On failure, returns 401. Token details are NEVER exposed to the client.
func RequireAuth(jwtManager *jwtpkg.Manager) fiber.Handler {
	return func(c fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return response.Unauthorized(c, "MISSING_TOKEN", "Authentication required")
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return response.Unauthorized(c, "INVALID_TOKEN_FORMAT", "Authentication required")
		}

		tokenString := strings.TrimSpace(parts[1])
		if tokenString == "" {
			return response.Unauthorized(c, "MISSING_TOKEN", "Authentication required")
		}

		claims, err := jwtManager.Parse(tokenString)
		if err != nil {
			switch {
			case errors.Is(err, jwtpkg.ErrTokenExpired):
				return response.Unauthorized(c, "TOKEN_EXPIRED", "Access token has expired")
			default:
				slog.Debug("auth middleware: invalid token", slog.Any("error", err))
				return response.Unauthorized(c, "INVALID_TOKEN", "Authentication required")
			}
		}

		c.Locals("user_id", claims.UserID)
		c.Locals("email", claims.Email)
		c.Locals("business_id", claims.BusinessID)
		c.Locals("role", claims.Role)

		return c.Next()
	}
}

// RequireBusiness validates that the JWT contains a non-empty business_id.
// Must run after RequireAuth.
func RequireBusiness() fiber.Handler {
	return func(c fiber.Ctx) error {
		businessID, _ := c.Locals("business_id").(string)
		if businessID == "" {
			return response.BadRequest(c,
				"NO_ORGANIZATION_CONTEXT",
				"An organization context is required. Select a workspace first.",
			)
		}
		return c.Next()
	}
}
