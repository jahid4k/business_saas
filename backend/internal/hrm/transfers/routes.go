// backend/internal/hrm/transfers/routes.go
package transfers

import "github.com/gofiber/fiber/v3"

type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts employee transfer routes.
//
//	GET  /organizations/:orgId/hrm/transfers
//	GET/POST /organizations/:orgId/hrm/employees/:employeeId/transfers
//	GET/PATCH /organizations/:orgId/hrm/employees/:employeeId/transfers/:transferId
//	POST .../transfers/:transferId/submit|cancel|apply
func RegisterRoutes(router fiber.Router, handler *Handler, permFn PermissionFunc, requireAuth, requireOrgMatch fiber.Handler) {
	router.Get("/organizations/:orgId/hrm/transfers",
		requireAuth, requireOrgMatch, permFn("hrm.transfers.view"), handler.ListAll)

	emp := router.Group("/organizations/:orgId/hrm/employees/:employeeId/transfers", requireAuth, requireOrgMatch)
	emp.Post("/:transferId/submit", permFn("hrm.transfers.manage"), handler.Submit)
	emp.Post("/:transferId/cancel", permFn("hrm.transfers.manage"), handler.Cancel)
	emp.Post("/:transferId/apply", permFn("hrm.transfers.apply"), handler.Apply)
	emp.Get("/", permFn("hrm.transfers.view"), handler.ListForEmployee)
	emp.Post("/", permFn("hrm.transfers.manage"), handler.Create)
	emp.Get("/:transferId", permFn("hrm.transfers.view"), handler.Get)
	emp.Patch("/:transferId", permFn("hrm.transfers.manage"), handler.Update)
}
