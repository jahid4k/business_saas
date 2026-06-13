package middleware

import (
	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/response"
)

// RequireOrganizationParam sets organization context from a route parameter such as :orgId.
// It intentionally does not authorize by itself; route-level permission middleware does that.
func RequireOrganizationParam(paramName string) fiber.Handler {
	return func(c fiber.Ctx) error {
		orgID := c.Params(paramName)
		if orgID == "" {
			return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
		}
		c.Locals("organization_id", orgID)
		c.Locals("business_id", orgID)
		return c.Next()
	}
}
