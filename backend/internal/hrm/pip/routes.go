// backend/internal/hrm/pip/routes.go
package pip

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
// Redeclared per package to break the package ↔ middleware import cycle.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts every HRM performance improvement plan route.
//
// Conventions this file must honour, enforced by tests in
// internal/tests/unit/architecture:
//
//   - Every registration carries a permFn("hrm....") argument whose value is
//     an INLINE STRING LITERAL. TestPermissions_AllRoutesProtected parses the
//     AST and reads Args[0].(*ast.BasicLit), so a named constant fails even
//     though it compiles.
//   - TestRouting_NoDuplicates normalizes every ":x" segment to ":param" and
//     keys on the receiver identifier. Every path here hangs off one `plans`
//     group and no two normalize alike.
//
// ⚠ hrm.pips.close is a SEPARATE key from hrm.pips.manage and gates exactly
// one route. Closing as 'failed' is what creates the draft termination, so it
// is the moment this instrument stops being developmental. 'manager' holds
// manage and NOT close — the same reasoning that keeps hrm.appraisals.publish
// away from 'manager'.
//
// Note that hrm.pips.manage never appears below as the gate on a
// per-plan write. It cannot: the route cannot know whether the target plan
// falls inside the caller's reporting line, so the service narrows that,
// checking CanManage plus AuthorizeRecordAccess together. Routes gate on
// .view and the service refuses the write — the hrm.goals.manage precedent.
//
//	Plans  GET/POST         /organizations/:orgId/hrm/pips
//	       GET/PATCH        .../pips/:pipId
//	       POST             .../pips/:pipId/{activate,cancel,checkins,extend}
//	       POST             .../pips/:pipId/close       <- hrm.pips.close
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	plans := router.Group("/organizations/:orgId/hrm/pips", requireAuth, requireOrgMatch)

	plans.Get("", permFn("hrm.pips.view"), handler.List)
	plans.Post("", permFn("hrm.pips.manage"), handler.Create)
	plans.Get("/:pipId", permFn("hrm.pips.view"), handler.Get)
	plans.Patch("/:pipId", permFn("hrm.pips.view"), handler.Update)

	plans.Post("/:pipId/activate", permFn("hrm.pips.view"), handler.Activate)
	plans.Post("/:pipId/cancel", permFn("hrm.pips.view"), handler.Cancel)
	plans.Post("/:pipId/checkins", permFn("hrm.pips.view"), handler.AddCheckin)
	plans.Post("/:pipId/extend", permFn("hrm.pips.view"), handler.Extend)

	// The one route gated on .close. Everything above narrows on .manage in
	// the service; this additionally requires a key 'manager' does not hold.
	plans.Post("/:pipId/close", permFn("hrm.pips.close"), handler.Close)
}
