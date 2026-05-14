package middleware

import "github.com/gofiber/fiber/v3"

// RequirePermission returns a middleware that enforces a specific permission
// within the current business context. Must run after RequireAuth and RequireBusiness.
//
// Permission format: "<resource>.<action>"
// Examples: "tasks.read", "tasks.delete", "members.manage"
//
// STATUS: Phase 1-D stub — calls c.Next() so routes compile and respond.
// Real permission check wired in Phase 1-D once authz.Service is implemented.
func RequirePermission(permission string) fiber.Handler {
	return func(c fiber.Ctx) error {
		// TODO (Phase 1-D): implement permission check
		// 1. Extract user_id and business_id from c.Locals
		// 2. Call authzService.Can(ctx, userID, businessID, resource, action)
		// 3. Check Redis cache first; fall back to DB query
		// 4. Return response.Forbidden(...) if not permitted
		// 5. Write audit log entry for the check result
		_ = permission // used in Phase 1-D
		return c.Next()
	}
}
