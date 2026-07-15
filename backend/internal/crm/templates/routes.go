// backend/internal/crm/templates/routes.go
package templates

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts all CRM template routes under /organizations/:orgId/crm/templates.
//
//	GET    /organizations/:orgId/crm/templates               <- crm.templates.view
//	POST   /organizations/:orgId/crm/templates               <- crm.templates.create
//	GET    /organizations/:orgId/crm/templates/:templateId   <- crm.templates.view
//	PATCH  /organizations/:orgId/crm/templates/:templateId   <- crm.templates.update
//	DELETE /organizations/:orgId/crm/templates/:templateId   <- crm.templates.delete
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	templates := router.Group("/organizations/:orgId/crm/templates", requireAuth, requireOrgMatch)

	templates.Get("", permFn("crm.templates.view"), handler.ListTemplates)
	templates.Post("", permFn("crm.templates.create"), handler.CreateTemplate)
	templates.Get("/:templateId", permFn("crm.templates.view"), handler.GetTemplate)
	templates.Patch("/:templateId", permFn("crm.templates.update"), handler.UpdateTemplate)
	templates.Delete("/:templateId", permFn("crm.templates.delete"), handler.DeleteTemplate)
}
