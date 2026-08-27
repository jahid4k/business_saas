// backend/internal/platform/tickets/routes.go
package tickets

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory returning permission-enforcing middleware.
// Same pattern as authz, checklists, forms — breaks the tickets ↔ middleware
// import cycle.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts the ticket engine under /organizations/:orgId/tickets,
// the platform/checklists prefix shape (requireAuth + requireOrgMatch, no
// module segment, because this is a shared platform primitive rather than an
// HRM feature).
//
// Configuration:
//
//	GET    /organizations/:orgId/tickets/categories                   <- platform.tickets.view
//	POST   /organizations/:orgId/tickets/categories                   <- platform.ticket_config.manage
//	PATCH  /organizations/:orgId/tickets/categories/:categoryId       <- platform.ticket_config.manage
//	GET    /organizations/:orgId/tickets/sla-policies                 <- platform.ticket_config.view
//	POST   /organizations/:orgId/tickets/sla-policies                 <- platform.ticket_config.manage
//	PATCH  /organizations/:orgId/tickets/sla-policies/:policyId       <- platform.ticket_config.manage
//
// Tickets:
//
//	POST   /organizations/:orgId/tickets                              <- platform.tickets.create
//	GET    /organizations/:orgId/tickets                              <- platform.tickets.view
//	GET    /organizations/:orgId/tickets/:ticketId                    <- platform.tickets.view
//	POST   /organizations/:orgId/tickets/:ticketId/assign             <- platform.tickets.assign
//	POST   /organizations/:orgId/tickets/:ticketId/resolve            <- platform.tickets.resolve
//	POST   /organizations/:orgId/tickets/:ticketId/close              <- platform.tickets.view
//	POST   /organizations/:orgId/tickets/:ticketId/cancel             <- platform.tickets.view
//	POST   /organizations/:orgId/tickets/:ticketId/pause              <- platform.tickets.pause
//	POST   /organizations/:orgId/tickets/:ticketId/resume             <- platform.tickets.pause
//
// Comments:
//
//	GET    /organizations/:orgId/tickets/:ticketId/comments           <- platform.tickets.view
//	POST   /organizations/:orgId/tickets/:ticketId/comments           <- platform.tickets.comment
//
// GET /categories is gated on tickets.view, not ticket_config.view: anyone
// who may raise a ticket has to see the categories to pick one, and the
// service reads it through the same permission.
//
// close and cancel gate on tickets.view because a requester may close or
// cancel their OWN ticket without being an agent; the service enforces
// "yours, or you hold .resolve". The route gate cannot express ownership, so
// it does not try — the platform.checklists.complete precedent.
//
// platform.tickets.view_all and platform.tickets.comment_internal have no
// route of their own by design. They are read INSIDE the service to widen
// what an existing route returns — view_all lifts the list's own-tickets
// narrowing, comment_internal permits an internal note on the ordinary
// comment route.
//
// There is deliberately NO conversion route — see the Handler doc comment.
//
// The literal /categories and /sla-policies segments are registered as their
// own groups BEFORE the /:ticketId group, so a request for /categories is
// not swallowed as a ticket id (the /companies/enrich precedent).
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	base := router.Group("/organizations/:orgId/tickets", requireAuth, requireOrgMatch)

	categories := base.Group("/categories")
	categories.Get("", permFn("platform.tickets.view"), handler.ListCategories)
	categories.Post("", permFn("platform.ticket_config.manage"), handler.CreateCategory)
	categories.Patch("/:categoryId", permFn("platform.ticket_config.manage"), handler.UpdateCategory)

	policies := base.Group("/sla-policies")
	policies.Get("", permFn("platform.ticket_config.view"), handler.ListPolicies)
	policies.Post("", permFn("platform.ticket_config.manage"), handler.CreatePolicy)
	policies.Patch("/:policyId", permFn("platform.ticket_config.manage"), handler.UpdatePolicy)

	base.Post("", permFn("platform.tickets.create"), handler.CreateTicket)
	base.Get("", permFn("platform.tickets.view"), handler.ListTickets)

	items := base.Group("/:ticketId")
	items.Get("", permFn("platform.tickets.view"), handler.GetTicket)
	items.Post("/assign", permFn("platform.tickets.assign"), handler.AssignTicket)
	items.Post("/resolve", permFn("platform.tickets.resolve"), handler.ResolveTicket)
	items.Post("/close", permFn("platform.tickets.view"), handler.CloseTicket)
	items.Post("/cancel", permFn("platform.tickets.view"), handler.CancelTicket)
	items.Post("/pause", permFn("platform.tickets.pause"), handler.PauseTicket)
	items.Post("/resume", permFn("platform.tickets.pause"), handler.ResumeTicket)

	items.Get("/comments", permFn("platform.tickets.view"), handler.ListComments)
	items.Post("/comments", permFn("platform.tickets.comment"), handler.AddComment)
}
