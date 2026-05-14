package user

import (
	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
)

// RegisterRoutes mounts all user routes onto the given Fiber router group.
//
// Route tree:
//
//	GET   /api/v1/users/me   ← requires JWT
//	PATCH /api/v1/users/me   ← requires JWT
func RegisterRoutes(router fiber.Router, handler *Handler) {
	users := router.Group("/users", middleware.RequireAuth())
	users.Get("/me", handler.Me)
	users.Patch("/me", handler.UpdateMe)
}
