package scheduler

import (
	"github.com/gofiber/fiber/v3"
)

func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	requireAuth fiber.Handler,
	requirePermission func(permission string) fiber.Handler,
) {
	group := router.Group("/platform/scheduler", requireAuth)

	// Admin routes
	group.Get("/jobs", requirePermission("platform.scheduler.view"), handler.ListJobs)
	group.Get("/jobs/:name/runs", requirePermission("platform.scheduler.view"), handler.ListJobRuns)
	group.Post("/jobs/:name/run", requirePermission("platform.scheduler.manage"), handler.TriggerJob)
}
