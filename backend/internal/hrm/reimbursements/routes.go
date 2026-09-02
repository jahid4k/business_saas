// backend/internal/hrm/reimbursements/routes.go
package reimbursements

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
// Redeclared per package to break the package <-> middleware import cycle.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts every HRM reimbursements route.
//
//   - Every registration carries a permFn("hrm....") argument whose value is
//     an INLINE STRING LITERAL — TestPermissions_AllRoutesProtected.
//
//   - `reimbursements` is its own group variable — TestRouting_NoDuplicates.
//
//   - Every permFn string appears as the first element of an INSERT tuple in
//     migration 00101 — TestPermissions_UsedStringsExistInMigrations.
//
//     GET/POST  /organizations/:orgId/hrm/reimbursements
//     GET       .../reimbursements/:reimbursementId
//     POST      .../reimbursements/:reimbursementId/{submit,cancel}
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	reimbursementsGroup := router.Group("/organizations/:orgId/hrm/reimbursements", requireAuth, requireOrgMatch)
	reimbursementsGroup.Get("", permFn("hrm.reimbursements.view"), handler.List)
	reimbursementsGroup.Post("", permFn("hrm.reimbursements.manage"), handler.Create)
	reimbursementsGroup.Get("/:reimbursementId", permFn("hrm.reimbursements.view"), handler.Get)
	reimbursementsGroup.Post("/:reimbursementId/submit", permFn("hrm.reimbursements.manage"), handler.Submit)
	reimbursementsGroup.Post("/:reimbursementId/cancel", permFn("hrm.reimbursements.manage"), handler.Cancel)
}
