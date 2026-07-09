// backend/internal/hrm/resignations/routes.go
package resignations

import "github.com/gofiber/fiber/v3"

type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts employee resignation routes.
//
//	GET  /organizations/:orgId/hrm/resignations
//	GET/POST /organizations/:orgId/hrm/employees/:employeeId/resignations
//	GET/PATCH /organizations/:orgId/hrm/employees/:employeeId/resignations/:resignationId
//	POST .../resignations/:resignationId/withdraw|accept|reject
func RegisterRoutes(router fiber.Router, handler *Handler, permFn PermissionFunc, requireAuth, requireOrgMatch fiber.Handler) {
	router.Get("/organizations/:orgId/hrm/resignations",
		requireAuth, requireOrgMatch, permFn("hrm.resignations.view"), handler.ListAll)

	emp := router.Group("/organizations/:orgId/hrm/employees/:employeeId/resignations", requireAuth, requireOrgMatch)
	// Action sub-routes before /:resignationId
	emp.Post("/:resignationId/withdraw", permFn("hrm.resignations.manage"), handler.Withdraw)
	emp.Post("/:resignationId/accept", permFn("hrm.resignations.process"), handler.Accept)
	emp.Post("/:resignationId/reject", permFn("hrm.resignations.process"), handler.Reject)
	emp.Get("/", permFn("hrm.resignations.view"), handler.ListForEmployee)
	emp.Post("/", permFn("hrm.resignations.manage"), handler.Submit)
	emp.Get("/:resignationId", permFn("hrm.resignations.view"), handler.Get)
	emp.Patch("/:resignationId", permFn("hrm.resignations.process"), handler.Update)
}
