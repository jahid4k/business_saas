// backend/internal/authz/routes.go
package authz

import (
	"github.com/gofiber/fiber/v3"
)

// RegisterRoutes mounts all authz routes.
//
// requireAuth, requireOrganization, and requirePermission are all injected
// from main.go — the authz package must NOT import middleware to avoid
// the import cycle: authz → middleware → authz.
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	requireAuth fiber.Handler,
	requireOrganization fiber.Handler,
	requireMembersManage fiber.Handler,
) {
	members := router.Group("/members",
		requireAuth,
		requireOrganization,
	)

	// GET /members/me — no special permission required
	members.Get("/me", handler.MyMembership)

	// GET /members — requires roles.assign
	members.Get("", requireMembersManage, handler.ListMembers)

	// POST /members/:userId/role — requires roles.assign
	members.Post("/:userId/role", requireMembersManage, handler.AssignRole)

	// Role + permission lookup — JWT only
	router.Get("/roles", requireAuth, handler.ListRoles)
	router.Get("/permissions", requireAuth, handler.ListPermissions)
}
