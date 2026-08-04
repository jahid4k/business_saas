// backend/internal/platform/checklists/routes.go
package checklists

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
// Same pattern as authz, task, contacts — breaks the checklists ↔ middleware
// import cycle.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts checklist engine routes under
// /organizations/:orgId/checklists. Prefix choice: contacts' auth shape
// (requireAuth + requireOrgMatch) minus the "crm" segment that doesn't apply
// here — /platform/scheduler has no :orgId/requireOrgMatch (a tenant
// isolation hole if copied) and /organizations/:orgId/crm/... has the right
// auth but a module segment that doesn't fit a shared platform primitive.
//
// Templates:
//
//	GET    /organizations/:orgId/checklists/templates                              <- platform.checklists.view
//	POST   /organizations/:orgId/checklists/templates                              <- platform.checklists.manage
//	GET    /organizations/:orgId/checklists/templates/:templateId                  <- platform.checklists.view
//	PATCH  /organizations/:orgId/checklists/templates/:templateId                  <- platform.checklists.manage
//	DELETE /organizations/:orgId/checklists/templates/:templateId                  <- platform.checklists.manage
//
// Template items:
//
//	GET    /organizations/:orgId/checklists/templates/:templateId/items            <- platform.checklists.view
//	POST   /organizations/:orgId/checklists/templates/:templateId/items            <- platform.checklists.manage
//	PATCH  /organizations/:orgId/checklists/templates/:templateId/items/:itemId    <- platform.checklists.manage
//	DELETE /organizations/:orgId/checklists/templates/:templateId/items/:itemId    <- platform.checklists.manage
//
// Instances:
//
//	GET    /organizations/:orgId/checklists/instances                              <- platform.checklists.view
//	GET    /organizations/:orgId/checklists/instances/:instanceId                  <- platform.checklists.view
//	POST   /organizations/:orgId/checklists/instances/:instanceId/cancel           <- platform.checklists.manage
//
// There is deliberately NO generic POST .../instances route — see the
// Handler doc comment. Instantiation is reachable only through module-owned
// endpoints (e.g. internal/hrm/onboarding) that resolve the subject
// server-side.
//
// Items:
//
//	GET    /organizations/:orgId/checklists/items/mine                             <- platform.checklists.view
//	POST   /organizations/:orgId/checklists/items/:itemId/complete                 <- platform.checklists.complete
//	POST   /organizations/:orgId/checklists/items/:itemId/reopen                   <- platform.checklists.complete
//	POST   /organizations/:orgId/checklists/items/:itemId/skip                     <- platform.checklists.manage
//
// /items/mine is registered before /items/:itemId's sub-paths (the
// /companies/enrich precedent) so the literal segment wins. .complete is
// granted broadly by migration 00077 then narrowed per-item by the service
// (assignee, or matching role holder, or .manage) — the route gate cannot
// express "is this your own item", so it does not try.
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	base := router.Group("/organizations/:orgId/checklists", requireAuth, requireOrgMatch)

	templates := base.Group("/templates")
	templates.Get("", permFn("platform.checklists.view"), handler.ListTemplates)
	templates.Post("", permFn("platform.checklists.manage"), handler.CreateTemplate)
	templates.Get("/:templateId", permFn("platform.checklists.view"), handler.GetTemplate)
	templates.Patch("/:templateId", permFn("platform.checklists.manage"), handler.UpdateTemplate)
	templates.Delete("/:templateId", permFn("platform.checklists.manage"), handler.DeleteTemplate)

	templates.Get("/:templateId/items", permFn("platform.checklists.view"), handler.ListTemplateItems)
	templates.Post("/:templateId/items", permFn("platform.checklists.manage"), handler.CreateTemplateItem)
	templates.Patch("/:templateId/items/:itemId", permFn("platform.checklists.manage"), handler.UpdateTemplateItem)
	templates.Delete("/:templateId/items/:itemId", permFn("platform.checklists.manage"), handler.DeleteTemplateItem)

	instances := base.Group("/instances")
	instances.Get("", permFn("platform.checklists.view"), handler.ListInstances)
	instances.Get("/:instanceId", permFn("platform.checklists.view"), handler.GetInstance)
	instances.Post("/:instanceId/cancel", permFn("platform.checklists.manage"), handler.CancelInstance)

	items := base.Group("/items")
	items.Get("/mine", permFn("platform.checklists.view"), handler.ListMyItems)
	items.Post("/:itemId/complete", permFn("platform.checklists.complete"), handler.CompleteItem)
	items.Post("/:itemId/reopen", permFn("platform.checklists.complete"), handler.ReopenItem)
	items.Post("/:itemId/skip", permFn("platform.checklists.manage"), handler.SkipItem)
}
