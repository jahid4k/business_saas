// backend/internal/hrm/onboarding/routes.go
package onboarding

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts the HRM onboarding checklist routes, nested under
// the existing employee resource.
//
//	GET  /organizations/:orgId/hrm/employees/:empId/checklists   <- hrm.employees.view
//	POST /organizations/:orgId/hrm/employees/:empId/checklists   <- hrm.employees.update
//
// These reuse hrm.employees.view/.update rather than minting hrm.onboarding.*
// keys — new keys would need their own migration seeding, and until every
// role is granted them a route gated on an unseeded key silently 403s admins
// who'd expect employee-management permissions to cover this. Verified:
// TestPermissions_AllRoutesProtected (internal/tests/unit/architecture)
// hard-fails any permFn(...) string under internal/hrm not prefixed "hrm.",
// so onboarding could not mint a platform.* key here even if it wanted to.
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	checklists := router.Group("/organizations/:orgId/hrm/employees/:empId/checklists", requireAuth, requireOrgMatch)
	checklists.Get("", permFn("hrm.employees.view"), handler.List)
	checklists.Post("", permFn("hrm.employees.update"), handler.Instantiate)
}
