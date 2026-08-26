// backend/internal/hrm/compensation/routes.go
package compensation

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
// Redeclared per package to break the package <-> middleware import cycle.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts every HRM compensation route.
//
// Three conventions this file must honour, enforced by
// internal/tests/unit/architecture:
//
//   - Every registration carries a permFn("hrm....") argument whose value is
//     an INLINE STRING LITERAL — TestPermissions_AllRoutesProtected parses
//     the AST and reads Args[0].(*ast.BasicLit), so a named constant compiles
//     but fails the guard.
//
//   - `bands`, `matrix`, `cycles`, `revisions` and `bonuses` are separate
//     group variables — TestRouting_NoDuplicates normalizes every ":x"
//     segment to ":param" and keys on the receiver identifier, so
//     /:bandId and /:cellId sharing a group would collide.
//
//   - Every permFn string used below appears as the first element of an
//     INSERT tuple in migration 00099 — TestPermissions_UsedStringsExistInMigrations.
//
//     Bands         GET/POST         /organizations/:orgId/hrm/compensation/bands
//     PATCH/DELETE     .../bands/:bandId
//     Merit matrix  GET/POST         .../merit-matrix
//     DELETE           .../merit-matrix/:cellId
//     Cycles        GET/POST         .../salary-revision-cycles
//     GET              .../salary-revision-cycles/:cycleId
//     POST             .../salary-revision-cycles/:cycleId/{compute,submit,apply}
//     GET              .../salary-revision-cycles/:cycleId/revisions
//     Revisions     GET/PATCH        .../salary-revisions/:revisionId
//     Bonuses       GET/POST         .../bonuses
//     GET/POST         .../bonuses/:bonusId, .../bonuses/:bonusId/submit
//     POST             .../bonuses/:bonusId/cancel
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	base := router.Group("/organizations/:orgId/hrm/compensation", requireAuth, requireOrgMatch)

	// ── Compensation bands ──────────────────────────────────────────────
	bands := base.Group("/bands")
	bands.Get("", permFn("hrm.compensation_config.view"), handler.ListBands)
	bands.Post("", permFn("hrm.compensation_config.manage"), handler.CreateBand)
	bands.Get("/:bandId", permFn("hrm.compensation_config.view"), handler.GetBand)
	bands.Patch("/:bandId", permFn("hrm.compensation_config.manage"), handler.UpdateBand)
	bands.Delete("/:bandId", permFn("hrm.compensation_config.manage"), handler.DeleteBand)

	// ── Merit matrix ─────────────────────────────────────────────────────
	matrix := base.Group("/merit-matrix")
	matrix.Get("", permFn("hrm.compensation_config.view"), handler.ListMatrixCells)
	matrix.Post("", permFn("hrm.compensation_config.manage"), handler.CreateMatrixCell)
	matrix.Delete("/:cellId", permFn("hrm.compensation_config.manage"), handler.DeleteMatrixCell)

	// ── Salary revision cycles ───────────────────────────────────────────
	// compute/submit/apply are all hrm.salary_revisions.manage — there is no
	// self-service path here (unlike hrm.goals.set_own): an employee
	// proposes nothing about their own pay. Deciding a submitted cycle goes
	// through hrm.approvals.action, not a permission on this resource.
	cycles := base.Group("/salary-revision-cycles")
	cycles.Get("", permFn("hrm.salary_revisions.view"), handler.ListCycles)
	cycles.Post("", permFn("hrm.salary_revisions.manage"), handler.CreateCycle)
	cycles.Get("/:cycleId", permFn("hrm.salary_revisions.view"), handler.GetCycle)
	cycles.Post("/:cycleId/compute", permFn("hrm.salary_revisions.manage"), handler.ComputeCycle)
	cycles.Post("/:cycleId/submit", permFn("hrm.salary_revisions.manage"), handler.SubmitCycle)
	cycles.Post("/:cycleId/apply", permFn("hrm.salary_revisions.manage"), handler.ApplyCycle)
	cycles.Get("/:cycleId/revisions", permFn("hrm.salary_revisions.view"), handler.ListRevisions)

	// ── Salary revisions ─────────────────────────────────────────────────
	// Reads are scope-filtered by hrm.salary_revisions tiers, resolved inside
	// the handler (the payslips.GetPayslip / hrm.appraisals precedent) —
	// this is the control that stops a peer reading someone else's proposed
	// raise.
	revisions := base.Group("/salary-revisions")
	revisions.Get("/:revisionId", permFn("hrm.salary_revisions.view"), handler.GetRevision)
	revisions.Patch("/:revisionId", permFn("hrm.salary_revisions.manage"), handler.OverrideRevision)

	// ── Bonuses ──────────────────────────────────────────────────────────
	bonuses := base.Group("/bonuses")
	bonuses.Get("", permFn("hrm.bonuses.view"), handler.ListBonuses)
	bonuses.Post("", permFn("hrm.bonuses.manage"), handler.CreateBonus)
	bonuses.Get("/:bonusId", permFn("hrm.bonuses.view"), handler.GetBonus)
	bonuses.Post("/:bonusId/submit", permFn("hrm.bonuses.manage"), handler.SubmitBonus)
	bonuses.Post("/:bonusId/cancel", permFn("hrm.bonuses.manage"), handler.CancelBonus)
}
