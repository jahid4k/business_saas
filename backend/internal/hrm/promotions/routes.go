// backend/internal/hrm/promotions/routes.go
package promotions

import "github.com/gofiber/fiber/v3"

// PermissionFunc returns permission-enforcing middleware for a given key.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts employee promotion routes.
//
// HR list view:
//
//	GET /organizations/:orgId/hrm/promotions
//
// Employee-scoped:
//
//	GET/POST    /organizations/:orgId/hrm/employees/:employeeId/promotions
//	GET/PATCH   /organizations/:orgId/hrm/employees/:employeeId/promotions/:promotionId
//	POST        /organizations/:orgId/hrm/employees/:employeeId/promotions/:promotionId/submit
//	POST        /organizations/:orgId/hrm/employees/:employeeId/promotions/:promotionId/cancel
//	POST        /organizations/:orgId/hrm/employees/:employeeId/promotions/:promotionId/apply
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	// HR org-level list — registered before employee-scoped group
	router.Get("/organizations/:orgId/hrm/promotions",
		requireAuth, requireOrgMatch, permFn("hrm.promotions.view"), handler.ListAll)

	// Employee-scoped routes
	emp := router.Group(
		"/organizations/:orgId/hrm/employees/:employeeId/promotions",
		requireAuth, requireOrgMatch,
	)

	// Action sub-routes BEFORE /:promotionId to avoid Fiber param capture
	emp.Post("/:promotionId/submit", permFn("hrm.promotions.manage"), handler.Submit)
	emp.Post("/:promotionId/cancel", permFn("hrm.promotions.manage"), handler.Cancel)
	emp.Post("/:promotionId/apply", permFn("hrm.promotions.apply"), handler.Apply)

	emp.Get("/", permFn("hrm.promotions.view"), handler.ListForEmployee)
	emp.Post("/", permFn("hrm.promotions.manage"), handler.Create)
	emp.Get("/:promotionId", permFn("hrm.promotions.view"), handler.Get)
	emp.Patch("/:promotionId", permFn("hrm.promotions.manage"), handler.Update)
}
