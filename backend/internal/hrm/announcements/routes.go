// backend/internal/hrm/announcements/routes.go
package announcements

import "github.com/gofiber/fiber/v3"

type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts announcement routes.
//
//	GET/POST /organizations/:orgId/hrm/announcements
//	GET/PATCH /organizations/:orgId/hrm/announcements/:announcementId
//	POST .../announcements/:announcementId/publish|schedule|archive
func RegisterRoutes(router fiber.Router, handler *Handler, permFn PermissionFunc, requireAuth, requireOrgMatch fiber.Handler) {
	ann := router.Group("/organizations/:orgId/hrm/announcements", requireAuth, requireOrgMatch)
	ann.Post("/:announcementId/publish",  permFn("hrm.announcements.publish"), handler.Publish)
	ann.Post("/:announcementId/schedule", permFn("hrm.announcements.publish"), handler.Schedule)
	ann.Post("/:announcementId/archive",  permFn("hrm.announcements.publish"), handler.Archive)
	ann.Get("/",               permFn("hrm.announcements.view"),   handler.List)
	ann.Post("/",              permFn("hrm.announcements.manage"), handler.Create)
	ann.Get("/:announcementId",  permFn("hrm.announcements.view"),   handler.Get)
	ann.Patch("/:announcementId",permFn("hrm.announcements.manage"), handler.Update)
}
