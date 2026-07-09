// backend/internal/hrm/calendar/routes.go
package calendar

import "github.com/gofiber/fiber/v3"

type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts HR calendar routes.
//
//	GET/POST /organizations/:orgId/hrm/calendar
//	GET/PATCH /organizations/:orgId/hrm/calendar/:eventId
//	POST .../calendar/:eventId/cancel|rsvp
func RegisterRoutes(router fiber.Router, handler *Handler, permFn PermissionFunc, requireAuth, requireOrgMatch fiber.Handler) {
	cal := router.Group("/organizations/:orgId/hrm/calendar", requireAuth, requireOrgMatch)
	cal.Post("/:eventId/cancel", permFn("hrm.calendar.manage"), handler.Cancel)
	cal.Post("/:eventId/rsvp",   permFn("hrm.calendar.manage"), handler.RequestRSVP)
	cal.Get("/",         permFn("hrm.calendar.view"),   handler.List)
	cal.Post("/",        permFn("hrm.calendar.manage"), handler.Create)
	cal.Get("/:eventId",  permFn("hrm.calendar.view"),   handler.Get)
	cal.Patch("/:eventId",permFn("hrm.calendar.manage"), handler.Update)
}
