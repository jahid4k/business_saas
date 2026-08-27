// backend/internal/hrm/expenses/routes.go
package expenses

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
// Redeclared per package to break the package <-> middleware import cycle.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts every HRM travel and expense route.
//
// Conventions enforced by internal/tests/unit/architecture:
//
//   - permFn("hrm....") with an INLINE STRING LITERAL —
//     TestPermissions_AllRoutesProtected reads Args[0].(*ast.BasicLit).
//   - `policies`, `perDiem`, `mileage`, `travel`, `advances` and `claims` are
//     separate group variables — TestRouting_NoDuplicates normalizes ":x" to
//     ":param" and keys on the receiver, so /:travelId and /:claimId sharing
//     a group would collide.
//   - Every permFn string is the first element of an INSERT tuple in
//     migration 00109 — TestPermissions_UsedStringsExistInMigrations.
//
// .submit is granted through 'member' and cannot express "for yourself only";
// Service.CreateTravel and Service.CreateClaim narrow it by resolving the
// caller's own employeeID. The hrm.goals.set_own precedent.
//
// .approve_lines gates the per-line decision — the money call this module
// exists for, and the one a manager holds. Deciding the claim's approval
// INSTANCE still goes through hrm.approvals.action, as everywhere else.
//
// Settlement is gated on .manage, not .approve_lines: deciding what a claim
// is worth and releasing the money are different authorities, the
// hrm.loans.disburse / compensation.ApplyCycle split.
//
//	Config    GET/POST  /organizations/:orgId/hrm/expense-policies
//	          GET/POST  /organizations/:orgId/hrm/per-diem-rates
//	          GET/POST  /organizations/:orgId/hrm/mileage-rates
//	Travel    GET/POST  /organizations/:orgId/hrm/travel-requests
//	          GET       .../travel-requests/:travelId
//	          POST      .../travel-requests/:travelId/submit
//	          GET/POST  .../travel-requests/:travelId/itinerary
//	Advances  GET/POST  /organizations/:orgId/hrm/travel-advances
//	          POST      .../travel-advances/:advanceId/disburse
//	Claims    GET/POST  /organizations/:orgId/hrm/expense-claims
//	          GET       .../expense-claims/:claimId
//	          POST      .../expense-claims/:claimId/{lines,submit,settle}
//	          POST      .../expense-claims/:claimId/lines/:lineId/approve
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	// ── Config (hrm.expense_config, untiered) ───────────────────────────
	policies := router.Group("/organizations/:orgId/hrm/expense-policies", requireAuth, requireOrgMatch)
	policies.Get("", permFn("hrm.expense_config.view"), handler.ListPolicies)
	policies.Post("", permFn("hrm.expense_config.manage"), handler.CreatePolicy)

	perDiem := router.Group("/organizations/:orgId/hrm/per-diem-rates", requireAuth, requireOrgMatch)
	perDiem.Get("", permFn("hrm.expense_config.view"), handler.ListPerDiemRates)
	perDiem.Post("", permFn("hrm.expense_config.manage"), handler.CreatePerDiemRate)

	mileage := router.Group("/organizations/:orgId/hrm/mileage-rates", requireAuth, requireOrgMatch)
	mileage.Get("", permFn("hrm.expense_config.view"), handler.ListMileageRates)
	mileage.Post("", permFn("hrm.expense_config.manage"), handler.CreateMileageRate)

	// ── Travel (hrm.travel, scope-tiered) ───────────────────────────────
	travel := router.Group("/organizations/:orgId/hrm/travel-requests", requireAuth, requireOrgMatch)
	travel.Get("", permFn("hrm.travel.view"), handler.ListTravel)
	travel.Post("", permFn("hrm.travel.submit"), handler.CreateTravel)
	travel.Get("/:travelId", permFn("hrm.travel.view"), handler.GetTravel)
	travel.Post("/:travelId/submit", permFn("hrm.travel.submit"), handler.SubmitTravel)
	travel.Get("/:travelId/itinerary", permFn("hrm.travel.view"), handler.ListItinerary)
	travel.Post("/:travelId/itinerary", permFn("hrm.travel.submit"), handler.AddItineraryItem)

	// ── Advances (hrm.travel) ───────────────────────────────────────────
	// Creating an advance is .manage — an employee does not grant themselves
	// money up front — and releasing it is its own key again.
	advances := router.Group("/organizations/:orgId/hrm/travel-advances", requireAuth, requireOrgMatch)
	advances.Get("", permFn("hrm.travel.view"), handler.ListAdvances)
	advances.Post("", permFn("hrm.travel.manage"), handler.CreateAdvance)
	advances.Post("/:advanceId/disburse", permFn("hrm.travel.disburse_advance"), handler.DisburseAdvance)

	// ── Claims (hrm.expenses, scope-tiered) ─────────────────────────────
	claims := router.Group("/organizations/:orgId/hrm/expense-claims", requireAuth, requireOrgMatch)
	claims.Get("", permFn("hrm.expenses.view"), handler.ListClaims)
	claims.Post("", permFn("hrm.expenses.submit"), handler.CreateClaim)
	claims.Get("/:claimId", permFn("hrm.expenses.view"), handler.GetClaim)
	claims.Post("/:claimId/lines", permFn("hrm.expenses.submit"), handler.AddLine)
	claims.Post("/:claimId/submit", permFn("hrm.expenses.submit"), handler.SubmitClaim)
	claims.Post("/:claimId/lines/:lineId/approve", permFn("hrm.expenses.approve_lines"), handler.ApproveLine)
	claims.Post("/:claimId/settle", permFn("hrm.expenses.manage"), handler.SettleClaim)
}
