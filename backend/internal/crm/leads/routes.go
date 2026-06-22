// backend/internal/crm/leads/routes.go
package leads

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts all CRM lead routes under /organizations/:orgId/crm/leads.
//
//	GET    /organizations/:orgId/crm/leads               <- crm.leads.view
//	POST   /organizations/:orgId/crm/leads               <- crm.leads.create
//	GET    /organizations/:orgId/crm/leads/:leadId       <- crm.leads.view
//	PATCH  /organizations/:orgId/crm/leads/:leadId       <- crm.leads.update
//	DELETE /organizations/:orgId/crm/leads/:leadId       <- crm.leads.delete
//	POST   /organizations/:orgId/crm/leads/:leadId/convert <- crm.leads.convert
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	leads := router.Group("/organizations/:orgId/crm/leads", requireAuth, requireOrgMatch)

	leads.Get("", permFn("crm.leads.view"), handler.ListLeads)
	leads.Post("", permFn("crm.leads.create"), handler.CreateLead)
	leads.Get("/:leadId", permFn("crm.leads.view"), handler.GetLead)
	leads.Patch("/:leadId", permFn("crm.leads.update"), handler.UpdateLead)
	leads.Delete("/:leadId", permFn("crm.leads.delete"), handler.DeleteLead)
	leads.Post("/:leadId/convert", permFn("crm.leads.convert"), handler.ConvertLead)
}
