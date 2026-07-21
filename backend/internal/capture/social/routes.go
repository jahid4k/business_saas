package social

import "github.com/gofiber/fiber/v3"

func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn func(key string) fiber.Handler,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	pub := router.Group("/pub/social")

	// Unauthenticated inbound webhook endpoint (e.g. for Facebook Lead Ads).
	// We handle the hub.challenge (GET) and payload parsing (POST) here.
	pub.All("/:platform/webhook", handler.HandleWebhook)

	// OAuth flow routes
	pub.Get("/auth/:platform", handler.InitOAuth)
	pub.Get("/auth/:platform/callback", handler.OAuthCallback)

	// Authenticated settings routes
	org := router.Group("/organizations/:orgId/capture/social", requireAuth, requireOrgMatch, permFn("settings.view"))
	org.Get("/", handler.ListOrgSocials)
	org.Post("/", handler.CreateOrgSocial)
	org.Delete("/:id", handler.DeleteOrgSocial)
}
