// backend/internal/hrm/employees/routes.go
package employees

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts all HRM employee routes.
//
//	GET    /organizations/:orgId/hrm/employees                         <- hrm.employees.view
//	POST   /organizations/:orgId/hrm/employees                         <- hrm.employees.create
//	GET    /organizations/:orgId/hrm/employees/:empId                  <- hrm.employees.view
//	PATCH  /organizations/:orgId/hrm/employees/:empId                  <- hrm.employees.update
//	DELETE /organizations/:orgId/hrm/employees/:empId                  <- hrm.employees.delete
//	POST   /organizations/:orgId/hrm/employees/:empId/terminate        <- hrm.employees.terminate
//
//	GET    /organizations/:orgId/hrm/employee-statuses                 <- hrm.employees.view
//	POST   /organizations/:orgId/hrm/employee-statuses                 <- hrm.employees.manage_setup
//	PATCH  /organizations/:orgId/hrm/employee-statuses/:id             <- hrm.employees.manage_setup
//	DELETE /organizations/:orgId/hrm/employee-statuses/:id             <- hrm.employees.manage_setup
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	emps := router.Group("/organizations/:orgId/hrm/employees", requireAuth, requireOrgMatch)

	emps.Get("", permFn("hrm.employees.view"), handler.List)
	emps.Post("", permFn("hrm.employees.create"), handler.Create)
	emps.Get("/:empId", permFn("hrm.employees.view"), handler.Get)
	emps.Patch("/:empId", permFn("hrm.employees.update"), handler.Update)
	emps.Delete("/:empId", permFn("hrm.employees.delete"), handler.Delete)

	// Terminate must be registered before /:empId matches catch all sub-paths
	emps.Post("/:empId/terminate", permFn("hrm.employees.terminate"), handler.Terminate)

	statuses := router.Group("/organizations/:orgId/hrm/employee-statuses", requireAuth, requireOrgMatch)
	statuses.Get("", permFn("hrm.employees.view"), handler.ListStatuses)
	// We use hrm.employees.update for setup for now. (Or a setup specific permission if it exists, but hrm.employees.update makes sense for now).
	statuses.Post("", permFn("hrm.employees.update"), handler.CreateStatus)
	statuses.Patch("/:id", permFn("hrm.employees.update"), handler.UpdateStatus)
	statuses.Delete("/:id", permFn("hrm.employees.update"), handler.DeleteStatus)
}
