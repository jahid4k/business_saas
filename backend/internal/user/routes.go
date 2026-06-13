// backend/internal/user/routes.go
package user

import "github.com/gofiber/fiber/v3"

func RegisterRoutes(router fiber.Router, handler *Handler, requireAuth fiber.Handler) {
	// Backward-compatible group.
	users := router.Group("/users", requireAuth)
	users.Get("/me", handler.Me)
	users.Patch("/me", handler.UpdateMe)

	// Preferred SaaS API shape.
	me := router.Group("/me", requireAuth)
	me.Get("", handler.Me)
	me.Patch("", handler.UpdateMe)
	me.Patch("/settings", handler.UpdateMe)
	me.Patch("/preferences", handler.UpdateMe)
	me.Post("/avatar", handler.UpdateAvatar)
}
