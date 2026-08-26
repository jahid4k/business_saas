// backend/internal/hrm/statutory/routes.go
package statutory

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
// Redeclared per package to break the package <-> middleware import cycle.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts every HRM statutory route.
//
//   - Every registration carries a permFn("hrm....") argument whose value is
//     an INLINE STRING LITERAL — TestPermissions_AllRoutesProtected.
//
//   - Slabs are mounted as a sub-path of the single `rules` group (no second
//     top-level group needed — nothing else in this package uses :ruleId).
//
//   - Every permFn string appears as the first element of an INSERT tuple in
//     migration 00103 — TestPermissions_UsedStringsExistInMigrations.
//
//     GET/POST  /organizations/:orgId/hrm/statutory/rules
//     GET       .../rules/:ruleId
//     POST      .../rules/:ruleId/{activate,deactivate}
//     GET/POST  .../rules/:ruleId/slabs
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	rules := router.Group("/organizations/:orgId/hrm/statutory/rules", requireAuth, requireOrgMatch)
	rules.Get("", permFn("hrm.statutory.view"), handler.ListRules)
	rules.Post("", permFn("hrm.statutory.manage"), handler.CreateRule)
	rules.Get("/:ruleId", permFn("hrm.statutory.view"), handler.GetRule)
	rules.Post("/:ruleId/activate", permFn("hrm.statutory.manage"), handler.ActivateRule)
	rules.Post("/:ruleId/deactivate", permFn("hrm.statutory.manage"), handler.DeactivateRule)
	rules.Get("/:ruleId/slabs", permFn("hrm.statutory.view"), handler.ListSlabs)
	rules.Post("/:ruleId/slabs", permFn("hrm.statutory.manage"), handler.CreateSlab)
}
