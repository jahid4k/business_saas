// backend/internal/hrm/positions/routes.go
package positions

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts all HRM position routes.
//
//	GET    /organizations/:orgId/hrm/positions          <- hrm.positions.view
//	POST   /organizations/:orgId/hrm/positions          <- hrm.positions.create
//	GET    /organizations/:orgId/hrm/positions/:posId   <- hrm.positions.view
//	PATCH  /organizations/:orgId/hrm/positions/:posId   <- hrm.positions.update
//	DELETE /organizations/:orgId/hrm/positions/:posId   <- hrm.positions.delete
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	pos := router.Group("/organizations/:orgId/hrm/positions", requireAuth, requireOrgMatch)

	pos.Get("", permFn("hrm.positions.view"), handler.List)
	pos.Post("", permFn("hrm.positions.create"), handler.Create)
	pos.Get("/:posId", permFn("hrm.positions.view"), handler.Get)
	pos.Patch("/:posId", permFn("hrm.positions.update"), handler.Update)
	pos.Delete("/:posId", permFn("hrm.positions.delete"), handler.Delete)
}
