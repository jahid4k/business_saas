package notifications

import "github.com/gofiber/fiber/v3"

// RegisterRoutes registers self-service notification endpoints. These are scoped to the
// requesting user (like /me) rather than RBAC-gated — a user always owns their own
// notifications and preferences, so requireAuth is the only guard needed.
func RegisterRoutes(router fiber.Router, handler *Handler, requireAuth fiber.Handler) {
	notif := router.Group("/notifications", requireAuth)
	notif.Get("", handler.List)
	notif.Post("/read-all", handler.MarkAllRead)
	notif.Post("/:id/read", handler.MarkRead)
	notif.Get("/preferences", handler.ListPreferences)
	notif.Patch("/preferences", handler.UpdatePreference)
}
