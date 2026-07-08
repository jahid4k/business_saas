// backend/internal/hrm/contracts/routes.go
package contracts

import "github.com/gofiber/fiber/v3"

type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts employee contract routes (employee-scoped, not setup).
//
//	GET/POST    /organizations/:orgId/hrm/employees/:employeeId/contracts
//	GET/PATCH   /organizations/:orgId/hrm/employees/:employeeId/contracts/:contractId
//	POST        /organizations/:orgId/hrm/employees/:employeeId/contracts/:contractId/deactivate
func RegisterRoutes(router fiber.Router, handler *Handler, permFn PermissionFunc, requireAuth, requireOrgMatch fiber.Handler) {
	base := router.Group("/organizations/:orgId/hrm/employees/:employeeId/contracts", requireAuth, requireOrgMatch)
	base.Get("/", permFn("hrm.contracts.view"), handler.List)
	base.Post("/", permFn("hrm.contracts.manage"), handler.Create)
	// deactivate before /:contractId to avoid param capture
	base.Post("/:contractId/deactivate", permFn("hrm.contracts.manage"), handler.Deactivate)
	base.Get("/:contractId", permFn("hrm.contracts.view"), handler.Get)
	base.Patch("/:contractId", permFn("hrm.contracts.manage"), handler.Update)
}
