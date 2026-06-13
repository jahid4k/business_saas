// backend/internal/security/routes.go
package security

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
// Same pattern as authz.PermissionFunc — breaks the import cycle.
type PermissionFunc func(permission string) fiber.Handler

func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrganization fiber.Handler,
) {
	group := router.Group("/organizations/:orgId/security", requireAuth, requireOrganization)
	group.Get("/sessions", permFn("security.sessions.view"), handler.ListSessions)
	group.Delete("/sessions/:sessionId", permFn("security.sessions.revoke"), handler.RevokeSession)
	group.Get("/login-events", permFn("security.login_events.view"), handler.ListLoginEvents)
}
