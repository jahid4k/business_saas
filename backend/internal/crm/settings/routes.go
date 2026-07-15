package settings

import (
	"github.com/gofiber/fiber/v3"
)

type PermissionFunc func(permission string) fiber.Handler

func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	settings := router.Group("/organizations/:orgId/crm/settings", requireAuth, requireOrgMatch)

	// Same permission as global settings for now, or crm.settings.view/update if we define them.
	// We'll use settings.update since CRM settings affect org.
	settings.Get("", permFn("settings.view"), handler.GetSettings)
	settings.Patch("", permFn("settings.update"), handler.UpdateSettings)
}
