// backend/internal/auth/routes.go
package auth

import "github.com/gofiber/fiber/v3"

// RegisterRoutes mounts auth routes without rate limiting.
// Used in tests or when rate limiting is handled at the infrastructure level.
func RegisterRoutes(router fiber.Router, handler *Handler, requireAuth fiber.Handler) {
	auth := router.Group("/auth")

	auth.Post("/signup", handler.Signup)
	auth.Post("/sign-up", handler.Signup)
	auth.Post("/login", handler.Login)
	auth.Post("/sign-in", handler.Login)
	auth.Post("/refresh", handler.Refresh)
	auth.Post("/refresh-token", handler.Refresh)
	auth.Post("/password-reset/request", handler.PasswordResetRequest)
	auth.Post("/password-reset/confirm", handler.PasswordResetConfirm)
	auth.Post("/oauth/sync", handler.OAuthSync)

	auth.Get("/me", requireAuth, handler.Me)
	// Logout is intentionally not behind RequireAuth; expired access-token users can still revoke refresh tokens.
	auth.Post("/logout", handler.Logout)
	auth.Post("/sign-out", handler.Logout)
	auth.Post("/logout-all", requireAuth, handler.LogoutAll)
	auth.Post("/sign-out-all", requireAuth, handler.LogoutAll)
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
	auth.Post("/sign-up", rateLimit, handler.Signup)
	auth.Post("/login", rateLimit, handler.Login)
	auth.Post("/sign-in", rateLimit, handler.Login)
	auth.Post("/refresh", rateLimit, handler.Refresh)
	auth.Post("/refresh-token", rateLimit, handler.Refresh)
	auth.Post("/password-reset/request", rateLimit, handler.PasswordResetRequest)
	auth.Post("/password-reset/confirm", rateLimit, handler.PasswordResetConfirm)
	auth.Post("/oauth/sync", rateLimit, handler.OAuthSync)

	// Protected — JWT required
	auth.Get("/me", requireAuth, handler.Me)
	// Logout is intentionally not behind RequireAuth; expired access-token users can still revoke refresh tokens.
	auth.Post("/logout", handler.Logout)
	auth.Post("/sign-out", handler.Logout)
	auth.Post("/logout-all", requireAuth, handler.LogoutAll)
	auth.Post("/sign-out-all", requireAuth, handler.LogoutAll)
}
