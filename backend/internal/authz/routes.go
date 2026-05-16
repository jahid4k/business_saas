// backend/internal/authz/routes.go
package authz

import (
	"github.com/gofiber/fiber/v3"
)

// RegisterRoutes mounts all authz routes.
//
// requireAuth, requireBusiness, and requirePermission are all injected
// from main.go — the authz package must NOT import middleware to avoid
// the import cycle: authz → middleware → authz.
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	requireAuth fiber.Handler,
	requireBusiness fiber.Handler,
	requireMembersManage fiber.Handler,
) {
	members := router.Group("/members",
		requireAuth,
		requireBusiness,
	)

	// GET /members/me — no special permission required
	members.Get("/me", handler.MyMembership)

	// GET /members — requires members.manage
	members.Get("/", requireMembersManage, handler.ListMembers)

	// POST /members/:userId/role — requires members.manage
	members.Post("/:userId/role", requireMembersManage, handler.AssignRole)

	// Role + permission lookup — JWT only
	router.Get("/roles", requireAuth, handler.ListRoles)
	router.Get("/permissions", requireAuth, handler.ListPermissions)
}
