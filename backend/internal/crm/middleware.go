// backend/internal/crm/middleware.go
package crm

import (
	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/response"
)

// RequireOrgMatch validates that the :orgId URL parameter matches the
// business_id stored in the JWT claims (set in Locals by RequireAuth).
//
// This is the CRM tenant isolation guard. Every CRM route is scoped under
// /organizations/:orgId/crm/... — this middleware ensures a user cannot
// access another organization's data by changing the URL param.
//
// Must run after RequireAuth.
func RequireOrgMatch() fiber.Handler {
	return func(c fiber.Ctx) error {
		paramOrgID := c.Params("orgId")
		jwtOrgID, _ := c.Locals("business_id").(string)

		if jwtOrgID == "" {
			return response.BadRequest(c,
				"NO_BUSINESS_CONTEXT",
				"A business context is required. Select a workspace first.",
			)
		}
		if paramOrgID != jwtOrgID {
			return response.Forbidden(c,
				"ORG_ACCESS_DENIED",
				"You do not have access to this organization",
			)
		}
		return c.Next()
	}
}
