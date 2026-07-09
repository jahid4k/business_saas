// backend/internal/hrm/terminations/routes.go
package terminations

import "github.com/gofiber/fiber/v3"

// PermissionFunc returns permission-enforcing middleware for a given key.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts employee termination routes.
//
// HR list view:
//
//	GET /organizations/:orgId/hrm/terminations
//
// Employee-scoped:
//
//	POST        /organizations/:orgId/hrm/employees/:employeeId/terminations
//	GET/PATCH   /organizations/:orgId/hrm/employees/:employeeId/terminations/:terminationId
//	POST        /organizations/:orgId/hrm/employees/:employeeId/terminations/:terminationId/submit
//	POST        /organizations/:orgId/hrm/employees/:employeeId/terminations/:terminationId/cancel
//	POST        /organizations/:orgId/hrm/employees/:employeeId/terminations/:terminationId/apply
//
// NOTE: There is intentionally no employee-scoped list endpoint — employees cannot
// see their own termination records. HR views all via the org-level list.
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	// HR org-level list
	router.Get("/organizations/:orgId/hrm/terminations",
		requireAuth, requireOrgMatch, permFn("hrm.terminations.view"), handler.ListAll)

	// Employee-scoped routes
	emp := router.Group(
		"/organizations/:orgId/hrm/employees/:employeeId/terminations",
		requireAuth, requireOrgMatch,
	)

	// Action sub-routes BEFORE /:terminationId to avoid Fiber param capture
	emp.Post("/:terminationId/submit", permFn("hrm.terminations.manage"), handler.Submit)
	emp.Post("/:terminationId/cancel", permFn("hrm.terminations.manage"), handler.Cancel)
	emp.Post("/:terminationId/apply", permFn("hrm.terminations.apply"), handler.Apply)

	emp.Post("/", permFn("hrm.terminations.manage"), handler.Create)
	emp.Get("/:terminationId", permFn("hrm.terminations.view"), handler.Get)
	emp.Patch("/:terminationId", permFn("hrm.terminations.manage"), handler.Update)
}
