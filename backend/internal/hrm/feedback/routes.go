// backend/internal/hrm/feedback/routes.go
package feedback

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
// Redeclared per package to break the package ↔ middleware import cycle.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts every HRM 360 feedback route.
//
// Conventions this file must honour, all enforced by tests in
// internal/tests/unit/architecture:
//
//   - Every registration carries a permFn("hrm....") argument whose value is
//     an INLINE STRING LITERAL. TestPermissions_AllRoutesProtected parses the
//     AST and reads Args[0].(*ast.BasicLit), so a named constant fails even
//     though it compiles.
//   - `cycles` and `requests` are separate group variables.
//     TestRouting_NoDuplicates normalizes every ":x" segment to ":param" and
//     keys on the receiver identifier, so /:cycleId and /:requestId on one
//     shared group would collide.
//
// ⚠ The permission split is the module, not decoration:
//
//	.coordinate gates the route that returns WHO WAS ASKED.
//	.view       gates the route that returns WHAT WAS SAID.
//
// No route is gated on a permission that yields both, because no such
// permission exists. An HR admin chasing non-responders is coordinating; the
// same admin reading the feedback gets the same suppressed aggregate the
// subject gets. Adding a "coordinator sees attributed content" route would
// make the promise to respondents false — and a promise of anonymity that is
// false for one role is false.
//
// /requests/mine registers BEFORE /requests/:requestId — a literal segment
// loses to a param when registered after it (the /instances/mine precedent).
//
//	Cycles     GET/POST   /organizations/:orgId/hrm/feedback/cycles
//	           GET/PATCH  .../cycles/:cycleId
//	           POST       .../cycles/:cycleId/{activate,close}
//	           POST       .../cycles/:cycleId/requests
//	           GET        .../cycles/:cycleId/employees/:employeeId/aggregate
//	Requests   GET        .../requests                  <- coordinate
//	           GET        .../requests/mine             <- respond
//	           POST       .../requests/:requestId/{submit,decline}
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	base := router.Group("/organizations/:orgId/hrm/feedback", requireAuth, requireOrgMatch)

	// ── Cycles ──────────────────────────────────────────────────────────
	cycles := base.Group("/cycles")
	cycles.Get("", permFn("hrm.feedback.view"), handler.ListCycles)
	cycles.Post("", permFn("hrm.feedback.manage"), handler.CreateCycle)
	cycles.Get("/:cycleId", permFn("hrm.feedback.view"), handler.GetCycle)
	cycles.Patch("/:cycleId", permFn("hrm.feedback.manage"), handler.UpdateCycle)
	cycles.Post("/:cycleId/activate", permFn("hrm.feedback.manage"), handler.ActivateCycle)
	cycles.Post("/:cycleId/close", permFn("hrm.feedback.manage"), handler.CloseCycle)
	// Creating requests is running the campaign, so it gates on .manage.
	cycles.Post("/:cycleId/requests", permFn("hrm.feedback.manage"), handler.CreateRequests)

	// The content path. Gated on .view and narrowed by the scope tier per
	// record; suppression is applied in the service and applies to EVERY
	// tier, including view_all. Stays on the `cycles` group because
	// /:cycleId/employees/:employeeId/aggregate normalizes to
	// /:param/employees/:param/aggregate, which collides with nothing above.
	cycles.Get("/:cycleId/employees/:employeeId/aggregate", permFn("hrm.feedback.view"), handler.GetAggregate)

	// ── Requests ────────────────────────────────────────────────────────
	requests := base.Group("/requests")
	requests.Get("", permFn("hrm.feedback.coordinate"), handler.ListRequests)
	requests.Get("/mine", permFn("hrm.feedback.respond"), handler.ListMyRequests)
	requests.Post("/:requestId/submit", permFn("hrm.feedback.respond"), handler.SubmitResponse)
	requests.Post("/:requestId/decline", permFn("hrm.feedback.respond"), handler.DeclineRequest)
}
