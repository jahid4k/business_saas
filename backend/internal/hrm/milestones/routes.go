// backend/internal/hrm/milestones/routes.go
package milestones

import "github.com/gofiber/fiber/v3"

type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts milestone routes.
//
//	GET/POST  /organizations/:orgId/hrm/milestones
//	POST      /organizations/:orgId/hrm/milestones/generate
//	GET       /organizations/:orgId/hrm/milestones/:milestoneId
//	POST      /organizations/:orgId/hrm/milestones/:milestoneId/acknowledge
func RegisterRoutes(router fiber.Router, handler *Handler, permFn PermissionFunc, requireAuth, requireOrgMatch fiber.Handler) {
	mil := router.Group("/organizations/:orgId/hrm/milestones", requireAuth, requireOrgMatch)
	// Static sub-routes BEFORE /:milestoneId
	mil.Post("/generate",                   permFn("hrm.milestones.generate"), handler.Generate)
	mil.Post("/:milestoneId/acknowledge",   permFn("hrm.milestones.manage"),   handler.Acknowledge)
	mil.Get("/",            permFn("hrm.milestones.view"),   handler.List)
	mil.Post("/",           permFn("hrm.milestones.manage"), handler.Create)
	mil.Get("/:milestoneId",permFn("hrm.milestones.view"),   handler.Get)
}
