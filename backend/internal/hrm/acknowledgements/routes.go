// backend/internal/hrm/acknowledgements/routes.go
package acknowledgements

import "github.com/gofiber/fiber/v3"

type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts acknowledgement routes.
//
//	GET/POST /organizations/:orgId/hrm/acknowledgements
//	GET      /organizations/:orgId/hrm/acknowledgements/entity/:type/:id
//	GET      /organizations/:orgId/hrm/acknowledgements/:ackId
//	POST     /organizations/:orgId/hrm/acknowledgements/:ackId/acknowledge
//	POST     /organizations/:orgId/hrm/acknowledgements/:ackId/decline
func RegisterRoutes(router fiber.Router, handler *Handler, permFn PermissionFunc, requireAuth, requireOrgMatch fiber.Handler) {
	ag := router.Group("/organizations/:orgId/hrm/acknowledgements", requireAuth, requireOrgMatch)

	// Named/static sub-routes BEFORE /:ackId
	ag.Get("/entity/:type/:id", permFn("hrm.acknowledgements.view"), handler.ListByEntity)
	ag.Post("/:ackId/acknowledge", permFn("hrm.acknowledgements.respond"), handler.Respond)
	ag.Post("/:ackId/decline",    permFn("hrm.acknowledgements.respond"), handler.Decline)
	ag.Get("/",       permFn("hrm.acknowledgements.view"),   handler.List)
	ag.Post("/",      permFn("hrm.acknowledgements.manage"), handler.Create)
	ag.Get("/:ackId", permFn("hrm.acknowledgements.view"),   handler.Get)
}
