package task

import (
	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
)

// RegisterRoutes mounts all task routes onto the given Fiber router group.
//
// Every task route requires:
//  1. Valid JWT (RequireAuth)
//  2. Valid business context (RequireBusiness)
//  3. Specific permission (RequirePermission)
//
// Route tree:
//
//	GET    /api/v1/tasks        ← tasks.read
//	POST   /api/v1/tasks        ← tasks.create
//	GET    /api/v1/tasks/:id    ← tasks.read
//	PATCH  /api/v1/tasks/:id    ← tasks.update
//	DELETE /api/v1/tasks/:id    ← tasks.delete
func RegisterRoutes(router fiber.Router, handler *Handler) {
	// All task routes require authentication + business context
	tasks := router.Group("/tasks",
		middleware.RequireAuth(),
		middleware.RequireBusiness(),
	)

	tasks.Get("/", middleware.RequirePermission("tasks.read"), handler.List)
	tasks.Post("/", middleware.RequirePermission("tasks.create"), handler.Create)
	tasks.Get("/:id", middleware.RequirePermission("tasks.read"), handler.Get)
	tasks.Patch("/:id", middleware.RequirePermission("tasks.update"), handler.Update)
	tasks.Delete("/:id", middleware.RequirePermission("tasks.delete"), handler.Delete)
}
