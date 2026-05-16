package authz

import (
	"github.com/gofiber/fiber/v3"
	"github.com/mridha/businesssaas/internal/middleware"
)

// RegisterRoutes mounts all authz routes.
//
// requireAuth and authzService are injected from main.go.
//
// Route tree:
//
//	GET  /api/v1/members              ← JWT + business + members.manage
//	GET  /api/v1/members/me           ← JWT + business (no special permission)
//	POST /api/v1/members/:userId/role ← JWT + business + members.manage
//
//	GET  /api/v1/roles                ← JWT only
//	GET  /api/v1/permissions          ← JWT only
//
// Important: /members/me must be registered BEFORE /members/:userId/role
// because Fiber matches routes in registration order. If /:userId comes first,
// "me" would be captured as a userId param.
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	requireAuth fiber.Handler,
	authzService Service,
) {
	// Member management routes
	members := router.Group("/members",
		requireAuth,
		middleware.RequireBusiness(),
	)

	// GET /members/me — no special permission required, just JWT + business context
	members.Get("/me", handler.MyMembership)

	// GET /members — list all members; requires members.manage
	members.Get("/", middleware.RequirePermission(authzService, "members.manage"), handler.ListMembers)

	// POST /members/:userId/role — change a member's role; requires members.manage
	members.Post("/:userId/role",
		middleware.RequirePermission(authzService, "members.manage"),
		handler.AssignRole,
	)

	// Role + permission lookup — JWT only, no business context required
	// (role/permission definitions are global, not per-business)
	router.Get("/roles", requireAuth, handler.ListRoles)
	router.Get("/permissions", requireAuth, handler.ListPermissions)
}
