package email

import "github.com/gofiber/fiber/v3"

func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn func(key string) fiber.Handler,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	pub := router.Group("/pub/email")
	
	// Unauthenticated inbound webhook endpoint.
	// We rely on the "to" address to lookup the correct organization securely.
	pub.Post("/webhook", handler.HandleWebhook)

	// Authenticated settings routes
	org := router.Group("/organizations/:orgId/capture/email", requireAuth, requireOrgMatch, permFn("settings.view"))
	org.Get("/", handler.ListOrgEmails)
	org.Post("/", handler.CreateOrgEmail)
	org.Delete("/:id", handler.DeleteOrgEmail)
}
