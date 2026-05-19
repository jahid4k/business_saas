// backend/internal/business/routes.go
// Package name is kept as business for backward compatibility. Routes expose organizations.
package organizations

import "github.com/gofiber/fiber/v3"

func RegisterRoutes(router fiber.Router, handler *Handler, requireAuth fiber.Handler) {
	// Preferred route group.
	organizations := router.Group("/organizations", requireAuth)
	organizations.Post("", handler.Create)
	organizations.Get("", handler.List)
	organizations.Get("/:id", handler.Get)
	organizations.Post("/:id/switch", handler.Switch)

	// Backward-compatible aliases while older frontend code is migrated.
	businesses := router.Group("/businesses", requireAuth)
	businesses.Post("", handler.Create)
	businesses.Get("", handler.List)
	businesses.Get("/:id", handler.Get)
	businesses.Post("/:id/switch", handler.Switch)
}
