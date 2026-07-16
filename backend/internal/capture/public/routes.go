package public

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"

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

	// Allow all origins for public endpoints (API key middleware handles domain restrictions)
	pub.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"*"},
		AllowCredentials: false,
	}))

	// The web form capture endpoint
	// Requires an API key with the "capture:leads" scope
	pub.Post("/leads", middleware.RequireAPIKey(apiKeySvc, "capture:leads"), handler.CaptureLead)
}
