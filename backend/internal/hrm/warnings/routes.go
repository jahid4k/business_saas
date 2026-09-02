// backend/internal/hrm/warnings/routes.go
package warnings

import "github.com/gofiber/fiber/v3"

type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts employee warning routes.
//
//	GET  /organizations/:orgId/hrm/warnings
//	GET/POST /organizations/:orgId/hrm/employees/:employeeId/warnings
//	GET/PATCH /organizations/:orgId/hrm/employees/:employeeId/warnings/:warningId
//	POST .../warnings/:warningId/issue|acknowledge|appeal|close|cancel
func RegisterRoutes(router fiber.Router, handler *Handler, permFn PermissionFunc, requireAuth, requireOrgMatch fiber.Handler) {
	router.Get("/organizations/:orgId/hrm/warnings",
		requireAuth, requireOrgMatch, permFn("hrm.warnings.view"), handler.ListAll)

	emp := router.Group("/organizations/:orgId/hrm/employees/:employeeId/warnings", requireAuth, requireOrgMatch)
	// Action sub-routes BEFORE /:warningId to avoid Fiber param capture
	emp.Post("/:warningId/issue", permFn("hrm.warnings.issue"), handler.Issue)
	emp.Post("/:warningId/acknowledge", permFn("hrm.warnings.acknowledge"), handler.Acknowledge)
	emp.Post("/:warningId/appeal", permFn("hrm.warnings.acknowledge"), handler.Appeal)
	emp.Post("/:warningId/close", permFn("hrm.warnings.close"), handler.Close)
	emp.Post("/:warningId/cancel", permFn("hrm.warnings.close"), handler.Cancel)
	emp.Get("/", permFn("hrm.warnings.view"), handler.ListForEmployee)
	emp.Post("/", permFn("hrm.warnings.manage"), handler.Create)
	emp.Get("/:warningId", permFn("hrm.warnings.view"), handler.Get)
	emp.Patch("/:warningId", permFn("hrm.warnings.manage"), handler.Update)
}
