// backend/internal/hrm/doctemplates/routes.go
package doctemplates

import "github.com/gofiber/fiber/v3"

type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts HRM document template routes.
//
//	GET/POST    /organizations/:orgId/hrm/setup/document-templates
//	GET/PATCH/DELETE /organizations/:orgId/hrm/setup/document-templates/:templateId
//	POST        /organizations/:orgId/hrm/setup/document-templates/:templateId/preview
func RegisterRoutes(router fiber.Router, handler *Handler, permFn PermissionFunc, requireAuth, requireOrgMatch fiber.Handler) {
	base := router.Group("/organizations/:orgId/hrm/setup/document-templates", requireAuth, requireOrgMatch)
	base.Get("/", permFn("hrm.doc_templates.view"), handler.List)
	base.Post("/", permFn("hrm.doc_templates.manage"), handler.Create)
	// preview before /:templateId to avoid param capture
	base.Post("/:templateId/preview", permFn("hrm.doc_templates.view"), handler.Preview)
	base.Get("/:templateId", permFn("hrm.doc_templates.view"), handler.Get)
	base.Patch("/:templateId", permFn("hrm.doc_templates.manage"), handler.Update)
	base.Delete("/:templateId", permFn("hrm.doc_templates.manage"), handler.Delete)
}
