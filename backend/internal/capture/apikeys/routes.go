package apikeys

import "github.com/gofiber/fiber/v3"

type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts API Key management routes under /organizations/:orgId/capture/apikeys
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	group := router.Group("/organizations/:orgId/capture/apikeys", requireAuth, requireOrgMatch)

	group.Get("", permFn("capture.apikeys.view"), handler.ListKeys)
	group.Post("", permFn("capture.apikeys.create"), handler.CreateKey)
	group.Delete("/:keyId", permFn("capture.apikeys.delete"), handler.RevokeKey)
}
