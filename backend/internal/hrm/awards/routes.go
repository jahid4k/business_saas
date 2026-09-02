// backend/internal/hrm/awards/routes.go
package awards

import "github.com/gofiber/fiber/v3"

type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts award routes.
//
//	GET/POST /organizations/:orgId/hrm/awards
//	GET/PATCH /organizations/:orgId/hrm/awards/:awardId
//	POST .../awards/:awardId/submit|issue|cancel
func RegisterRoutes(router fiber.Router, handler *Handler, permFn PermissionFunc, requireAuth, requireOrgMatch fiber.Handler) {
	aw := router.Group("/organizations/:orgId/hrm/awards", requireAuth, requireOrgMatch)
	aw.Post("/:awardId/submit", permFn("hrm.awards.approve"), handler.Submit)
	aw.Post("/:awardId/issue", permFn("hrm.awards.issue"), handler.Issue)
	aw.Post("/:awardId/cancel", permFn("hrm.awards.manage"), handler.Cancel)
	aw.Get("/", permFn("hrm.awards.view"), handler.List)
	aw.Post("/", permFn("hrm.awards.manage"), handler.Create)
	aw.Get("/:awardId", permFn("hrm.awards.view"), handler.Get)
	aw.Patch("/:awardId", permFn("hrm.awards.manage"), handler.Update)
}
