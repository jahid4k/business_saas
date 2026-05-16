// backend/internal/user/routes.go
package user

import "github.com/gofiber/fiber/v3"

// RegisterRoutes mounts user routes.
// requireAuth is injected from main.go.
func RegisterRoutes(router fiber.Router, handler *Handler, requireAuth fiber.Handler) {
	users := router.Group("/users", requireAuth)
	users.Get("/me", handler.Me)
	users.Patch("/me", handler.UpdateMe)
}
