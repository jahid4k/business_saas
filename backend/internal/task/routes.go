// backend/internal/task/routes.go
package task

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
// Same pattern as authz.PermissionFunc and security.PermissionFunc — breaks
// the task <-> middleware import cycle.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts all task routes under /organizations/:orgId/tasks.
//
//	GET    /organizations/:orgId/tasks         <- tasks.view
//	POST   /organizations/:orgId/tasks         <- tasks.create
//	GET    /organizations/:orgId/tasks/:taskId <- tasks.view
//	PATCH  /organizations/:orgId/tasks/:taskId <- tasks.update
//	DELETE /organizations/:orgId/tasks/:taskId <- tasks.delete
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrganization fiber.Handler,
) {
	tasks := router.Group("/organizations/:orgId/tasks", requireAuth, requireOrganization)

	tasks.Get("", permFn("tasks.view"), handler.List)
	tasks.Post("", permFn("tasks.create"), handler.Create)
	tasks.Get("/:taskId", permFn("tasks.view"), handler.Get)
	tasks.Patch("/:taskId", permFn("tasks.update"), handler.Update)
	tasks.Delete("/:taskId", permFn("tasks.delete"), handler.Delete)
}
