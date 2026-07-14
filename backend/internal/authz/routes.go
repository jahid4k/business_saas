// backend/internal/authz/routes.go
package authz

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns a Fiber middleware enforcing a named permission.
// This indirection breaks the authz ↔ middleware import cycle: authz/routes.go needs to
// create per-route permission middleware, but middleware/permission.go imports authz for
// the Service interface. By accepting a factory instead of importing middleware directly,
// authz/routes.go stays cycle-free.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts all authz/RBAC routes.
// permFn is typically middleware.RequirePermission(authzSvc, ...) partially applied via a closure.
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireBusiness fiber.Handler,
	requireOrgParam fiber.Handler,
) {
	members := router.Group("/members", requireAuth, requireBusiness)
	members.Get("/me", handler.MyMembership)
	members.Get("", permFn("members.view"), handler.ListMembers)
	members.Post("/:userId/role", permFn("members.update"), handler.AssignRole)

	router.Get("/roles", requireAuth, handler.ListRoles)
	router.Get("/permissions", requireAuth, handler.ListPermissions)

	orgs := router.Group("/organizations/:orgId", requireAuth, requireOrgParam)

	orgMembers := orgs.Group("/members")
	orgMembers.Get("", permFn("members.view"), handler.ListMembers)
	orgMembers.Post("/invite", permFn("members.invite"), handler.InviteMember)
	orgMembers.Get("/:memberId", permFn("members.view"), handler.GetMember)
	orgMembers.Patch("/:memberId", permFn("members.update"), handler.UpdateMember)
	orgMembers.Patch("/:memberId/role", permFn("members.update"), handler.AssignRole)
	orgMembers.Patch("/:memberId/status", permFn("members.update"), handler.UpdateMember)
	orgMembers.Post("/:memberId/reset-password", permFn("members.password_reset"), handler.ResetMemberPassword)

	orgInvitations := orgs.Group("/invitations")
	orgInvitations.Post("/:invitationId/resend", permFn("members.invite"), handler.ResendInvitation)
	orgInvitations.Delete("/:invitationId", permFn("members.remove"), handler.RevokeInvitation)

	rbac := orgs.Group("/rbac")
	rbac.Get("/permissions", permFn("roles.view"), handler.ListPermissions)
	rbac.Get("/permissions/grouped", permFn("roles.view"), handler.ListPermissions)
	rbac.Get("/permissions/matrix", permFn("roles.view"), handler.PermissionMatrix)
	rbac.Post("/check", permFn("roles.view"), handler.CheckMember)
	rbac.Post("/check-member", permFn("roles.view"), handler.CheckMember)

	roles := rbac.Group("/roles")
	roles.Get("", permFn("roles.view"), handler.ListOrganizationRoles)
	roles.Post("", permFn("roles.create"), handler.CreateRole)
	roles.Get("/:roleId", permFn("roles.view"), handler.GetRole)
	roles.Patch("/:roleId", permFn("roles.update"), handler.UpdateRole)
	roles.Delete("/:roleId", permFn("roles.delete"), handler.DeleteRole)
	roles.Patch("/:roleId/permissions", permFn("roles.permissions.update"), handler.UpdateRolePermissions)
	roles.Post("/:roleId/clone", permFn("roles.clone"), handler.CloneRole)

	memberPerms := rbac.Group("/members/:memberId/permissions")
	memberPerms.Get("", permFn("members.permissions.view"), handler.GetMemberPermissions)
	memberPerms.Patch("", permFn("members.permissions.update"), handler.UpdateMemberPermissions)

	// Accepting an invitation requires identity, but not existing organization membership.
	router.Post("/organizations/:orgId/invitations/:token/accept", requireAuth, handler.AcceptInvitation)
}
