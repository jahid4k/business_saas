// backend/internal/task/routes.go
package task

import (
	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/middleware"
)

// RegisterRoutes mounts all task routes.
//
// requireAuth and authzService are injected from main.go.
//
// Every task route requires:
//
//  1. Valid JWT                            (requireAuth)
//
//  2. Non-empty business_id in JWT        (RequireBusiness)
//
//  3. Specific permission (DB + cached)   (RequirePermission with authzService)
//
//     GET    /api/v1/tasks        ← tasks.read
//     POST   /api/v1/tasks        ← tasks.create
//     GET    /api/v1/tasks/:id    ← tasks.read
//     PATCH  /api/v1/tasks/:id    ← tasks.update
//     DELETE /api/v1/tasks/:id    ← tasks.delete
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	requireAuth fiber.Handler,
	authzService authz.Service,
) {
	tasks := router.Group("/tasks",
		requireAuth,
		middleware.RequireBusiness(),
	)

	tasks.Get("/", middleware.RequirePermission(authzService, "tasks.read"), handler.List)
	tasks.Post("/", middleware.RequirePermission(authzService, "tasks.create"), handler.Create)
	tasks.Get("/:id", middleware.RequirePermission(authzService, "tasks.read"), handler.Get)
	tasks.Patch("/:id", middleware.RequirePermission(authzService, "tasks.update"), handler.Update)
	tasks.Delete("/:id", middleware.RequirePermission(authzService, "tasks.delete"), handler.Delete)
}
