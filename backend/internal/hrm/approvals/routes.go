// backend/internal/hrm/approvals/routes.go
package approvals

import "github.com/gofiber/fiber/v3"

type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts HRM approval template and instance routes.
//
//	GET/POST    /organizations/:orgId/hrm/setup/approvals
//	GET/PATCH/DELETE /organizations/:orgId/hrm/setup/approvals/:templateId
//	GET         /organizations/:orgId/hrm/setup/approvals/instances/:instanceId
//	POST        /organizations/:orgId/hrm/setup/approvals/instances/:instanceId/approve
//	POST        /organizations/:orgId/hrm/setup/approvals/instances/:instanceId/reject
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	base := router.Group("/organizations/:orgId/hrm/setup/approvals", requireAuth, requireOrgMatch)
	base.Get("/", permFn("hrm.approvals.view"), handler.ListTemplates)
	base.Post("/", permFn("hrm.approvals.manage"), handler.CreateTemplate)
	base.Get("/:templateId", permFn("hrm.approvals.view"), handler.GetTemplate)
	base.Patch("/:templateId", permFn("hrm.approvals.manage"), handler.UpdateTemplate)
	base.Delete("/:templateId", permFn("hrm.approvals.manage"), handler.DeleteTemplate)

	// Instance sub-actions before /:instanceId to avoid param collision
	instances := router.Group("/organizations/:orgId/hrm/setup/approvals/instances", requireAuth, requireOrgMatch)
	instances.Post("/:instanceId/approve", permFn("hrm.approvals.action"), handler.Approve)
	instances.Post("/:instanceId/reject", permFn("hrm.approvals.action"), handler.Reject)
	instances.Get("/:instanceId", permFn("hrm.approvals.view"), handler.GetInstance)
}
