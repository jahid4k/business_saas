// backend/internal/middleware/permission.go
package middleware

import (
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/pkg/response"
)

// RequirePermission returns a middleware that enforces a specific permission
// within the current business context.
//
// Must run after RequireAuth and RequireBusiness — it reads user_id and
// business_id from c.Locals which those middlewares set.
//
// Permission format: "<resource>.<action>"
// Examples: "tasks.read", "tasks.delete", "members.manage"
//
// Flow:
//  1. Extract user_id and business_id from c.Locals
//  2. Split permission string into resource + action
//  3. Call authzService.Can(ctx, userID, businessID, resource, action)
//  4. Can() checks Redis cache → falls back to DB if missed
//  5. Return 403 Forbidden if permission not granted
//  6. Call c.Next() if permission granted
func RequirePermission(authzService authz.Service, permission string) fiber.Handler {
	// Parse the permission string once at route registration time — not per request
	parts := strings.SplitN(permission, ".", 2)
	if len(parts) != 2 {
		// Bad permission string — this is a programming error, not a runtime error.
		// Panic at startup so the developer sees it immediately.
		panic("middleware: RequirePermission: invalid permission format (expected resource.action): " + permission)
	}
	resource := parts[0]
	action := parts[1]

	return func(c fiber.Ctx) error {
		userID, ok := c.Locals("user_id").(string)
		if !ok || userID == "" {
			return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
		}

		businessID, ok := c.Locals("business_id").(string)
		if !ok || businessID == "" {
			return response.BadRequest(c, "NO_BUSINESS_CONTEXT", "Business context is required")
		}

		allowed, err := authzService.Can(c.Context(), userID, businessID, resource, action)
		if err != nil {
			slog.Error("permission middleware: Can() error",
				slog.String("user_id", userID),
				slog.String("business_id", businessID),
				slog.String("permission", permission),
				slog.Any("error", err),
			)
			return response.InternalServerError(c)
		}

		if !allowed {
			slog.Warn("permission denied",
				slog.String("user_id", userID),
				slog.String("business_id", businessID),
				slog.String("permission", permission),
			)
			return response.Forbidden(c,
				"PERMISSION_DENIED",
				"You do not have permission to perform this action",
			)
		}

		return c.Next()
	}
}
