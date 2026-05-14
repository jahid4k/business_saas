package auth

import (
	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
)

// RegisterRoutes mounts all auth routes onto the given Fiber router group.
//
// Route tree:
//
//	POST /api/v1/auth/signup
//	POST /api/v1/auth/login
//	POST /api/v1/auth/refresh
//	POST /api/v1/auth/logout          ← requires valid JWT
//	POST /api/v1/auth/logout-all      ← requires valid JWT
//	POST /api/v1/auth/password-reset/request
//	POST /api/v1/auth/password-reset/confirm
//
// Rate limiting is applied to all auth routes.
// Additional rate limiting is applied to login and password reset (Phase 1-B).
func RegisterRoutes(router fiber.Router, handler *Handler) {
	auth := router.Group("/auth")

	// Public auth routes — no JWT required
	// Rate limit middleware is a stub until Phase 1-B
	auth.Post("/signup", middleware.AuthRateLimit(), handler.Signup)
	auth.Post("/login", middleware.AuthRateLimit(), handler.Login)
	auth.Post("/refresh", middleware.AuthRateLimit(), handler.Refresh)
	auth.Post("/password-reset/request", middleware.AuthRateLimit(), handler.PasswordResetRequest)
	auth.Post("/password-reset/confirm", middleware.AuthRateLimit(), handler.PasswordResetConfirm)

	// Authenticated auth routes — JWT required
	auth.Post("/logout", middleware.RequireAuth(), handler.Logout)
	auth.Post("/logout-all", middleware.RequireAuth(), handler.LogoutAll)
}
