// backend/internal/middleware/context.go
package middleware

import "github.com/gofiber/fiber/v3"

// UserIDFromCtx reads the authenticated user's id, set by RequireAuth.
func UserIDFromCtx(c fiber.Ctx) (string, bool) {
	userID, ok := c.Locals("user_id").(string)
	return userID, ok && userID != ""
}

// OrganizationIDFromCtx reads the active organization id, checking both
// "organization_id" (set by RequireOrganizationParam from :orgId) and
// "business_id" (set by RequireAuth from the JWT's bid claim), so handlers
// work regardless of which middleware supplied the context.
func OrganizationIDFromCtx(c fiber.Ctx) (string, bool) {
	orgID, _ := c.Locals("organization_id").(string)
	if orgID == "" {
		orgID, _ = c.Locals("business_id").(string)
	}
	return orgID, orgID != ""
}
