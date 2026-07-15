package public

import (
	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/capture/apikeys"
	"github.com/mridha/businesssaas/internal/middleware"
)

// RegisterRoutes mounts public endpoints under /pub
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	apiKeySvc apikeys.Service,
) {
	pub := router.Group("/pub")

	// The web form capture endpoint
	// Requires an API key with the "capture:leads" scope
	pub.Post("/leads", middleware.RequireAPIKey(apiKeySvc, "capture:leads"), handler.CaptureLead)
}
