// backend/internal/hrm/loans/routes.go
package loans

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
// Redeclared per package to break the package <-> middleware import cycle.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts every HRM loans route.
//
//   - Every registration carries a permFn("hrm....") argument whose value is
//     an INLINE STRING LITERAL — TestPermissions_AllRoutesProtected.
//   - `loans` is its own group variable — TestRouting_NoDuplicates.
//   - Every permFn string appears as the first element of an INSERT tuple in
//     migration 00101 — TestPermissions_UsedStringsExistInMigrations.
//
// No separate .approve route — deciding a submitted loan's approval instance
// goes through hrm.approvals.action (POST .../approval-instances/:id/approve),
// the 00099 precedent.
//
//	GET/POST  /organizations/:orgId/hrm/loans
//	GET       .../loans/:loanId
//	POST      .../loans/:loanId/{submit,disburse,foreclose}
//	GET       .../loans/:loanId/schedule
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	loansGroup := router.Group("/organizations/:orgId/hrm/loans", requireAuth, requireOrgMatch)
	loansGroup.Get("", permFn("hrm.loans.view"), handler.ListLoans)
	loansGroup.Post("", permFn("hrm.loans.manage"), handler.CreateLoan)
	loansGroup.Get("/:loanId", permFn("hrm.loans.view"), handler.GetLoan)
	loansGroup.Post("/:loanId/submit", permFn("hrm.loans.manage"), handler.SubmitLoan)
	loansGroup.Post("/:loanId/disburse", permFn("hrm.loans.disburse"), handler.DisburseLoan)
	loansGroup.Post("/:loanId/foreclose", permFn("hrm.loans.foreclose"), handler.ForecloseLoan)
	loansGroup.Get("/:loanId/schedule", permFn("hrm.loans.view"), handler.ListSchedule)
}
