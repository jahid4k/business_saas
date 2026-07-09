// backend/internal/hrm/departments/routes.go
package departments

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
// Breaks the departments <-> middleware import cycle, same pattern as all
// other modules in this codebase.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts all HRM department routes.
//
//	GET    /organizations/:orgId/hrm/departments           <- hrm.departments.view
//	POST   /organizations/:orgId/hrm/departments           <- hrm.departments.create
//	GET    /organizations/:orgId/hrm/departments/:deptId   <- hrm.departments.view
//	PATCH  /organizations/:orgId/hrm/departments/:deptId   <- hrm.departments.update
//	DELETE /organizations/:orgId/hrm/departments/:deptId   <- hrm.departments.delete
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	depts := router.Group("/organizations/:orgId/hrm/departments", requireAuth, requireOrgMatch)

	depts.Get("", permFn("hrm.departments.view"), handler.List)
	depts.Post("", permFn("hrm.departments.create"), handler.Create)
	depts.Get("/:deptId", permFn("hrm.departments.view"), handler.Get)
	depts.Patch("/:deptId", permFn("hrm.departments.update"), handler.Update)
	depts.Delete("/:deptId", permFn("hrm.departments.delete"), handler.Delete)
}
