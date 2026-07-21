package dashboard

import (
	"github.com/gofiber/fiber/v3"
)

func RegisterRoutes(router fiber.Router, handler *Handler, requireAuth, requireOrgMatch fiber.Handler) {
	// Group under /api/v1/orgs/:orgId/dashboard
	group := router.Group("/orgs/:orgId/dashboard")

	// Apply common org authentication/authorization middlewares
	group.Use(requireAuth)
	group.Use(requireOrgMatch)

	// Fetch metrics
	group.Get("/", handler.HandleGetMetrics)
}
