package business

import (
	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
)

// RegisterRoutes mounts all business routes onto the given Fiber router group.
//
// Route tree:
//
//	POST /api/v1/businesses              ← requires JWT
//	GET  /api/v1/businesses              ← requires JWT
//	GET  /api/v1/businesses/:id          ← requires JWT
//	POST /api/v1/businesses/:id/switch   ← requires JWT
func RegisterRoutes(router fiber.Router, handler *Handler) {
	businesses := router.Group("/businesses", middleware.RequireAuth())

	businesses.Post("/", handler.Create)
	businesses.Get("/", handler.List)
	businesses.Get("/:id", handler.Get)
	businesses.Post("/:id/switch", handler.Switch)
}
