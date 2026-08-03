package visitors

import (
	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/capture/apikeys"
	"github.com/mridha/businesssaas/internal/middleware"
)

func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	apiKeySvc apikeys.Service,
	permFn func(key string) fiber.Handler,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	pub := router.Group("/pub/visitors")

	// Endpoint to identify or track website visitors
	// Requires a valid API key with the "capture:visitors" scope
	pub.Post("/identify", middleware.RequireAPIKey(apiKeySvc, "capture:visitors"), handler.Identify)

	// Authenticated dashboard routes
	org := router.Group("/organizations/:orgId/capture/visitors", requireAuth, requireOrgMatch, permFn("capture.visitors.view"))
	org.Get("/", handler.ListVisitors)
}
