// backend/internal/auth/routes.go
package auth

import "github.com/gofiber/fiber/v3"

// RegisterRoutes mounts auth routes without rate limiting.
// Used in tests or when rate limiting is handled at the infrastructure level.
func RegisterRoutes(router fiber.Router, handler *Handler, requireAuth fiber.Handler) {
	auth := router.Group("/auth")

	auth.Post("/signup", handler.Signup)
	auth.Post("/login", handler.Login)
	auth.Post("/refresh", handler.Refresh)
	auth.Post("/password-reset/request", handler.PasswordResetRequest)
	auth.Post("/password-reset/confirm", handler.PasswordResetConfirm)

	auth.Get("/me", requireAuth, handler.Me)
	auth.Post("/logout", requireAuth, handler.Logout)
	auth.Post("/logout-all", requireAuth, handler.LogoutAll)
}

// RegisterRoutesWithRateLimit mounts auth routes with Redis-backed rate limiting
// on all public endpoints.
func RegisterRoutesWithRateLimit(
	router fiber.Router,
	handler *Handler,
	requireAuth fiber.Handler,
	rateLimit fiber.Handler,
) {
	auth := router.Group("/auth")

	// Public — rate limited
	auth.Post("/signup", rateLimit, handler.Signup)
	auth.Post("/login", rateLimit, handler.Login)
	auth.Post("/refresh", rateLimit, handler.Refresh)
	auth.Post("/password-reset/request", rateLimit, handler.PasswordResetRequest)
	auth.Post("/password-reset/confirm", rateLimit, handler.PasswordResetConfirm)

	// Protected — JWT required
	auth.Get("/me", requireAuth, handler.Me)
	auth.Post("/logout", requireAuth, handler.Logout)
	auth.Post("/logout-all", requireAuth, handler.LogoutAll)
}
