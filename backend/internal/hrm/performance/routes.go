// backend/internal/hrm/performance/routes.go
package performance

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
// Redeclared per package to break the package ↔ middleware import cycle.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts every HRM performance route.
//
// Two conventions this file must honour, both enforced by tests in
// internal/tests/unit/architecture:
//
//   - Every registration carries a permFn("hrm....") argument whose value is
//     an INLINE STRING LITERAL. TestPermissions_AllRoutesProtected parses the
//     AST and reads Args[0].(*ast.BasicLit), so a named constant fails even
//     though it compiles.
//   - `cycles`, `goals` and `empGoals` are separate group variables.
//     TestRouting_NoDuplicates normalizes every ":x" segment to ":param" and
//     keys on the receiver identifier, so registering /:cycleId and /:goalId
//     on one shared group would collide.
//
// Note that hrm.goals.manage never appears below. It cannot gate a route,
// because the route cannot know whether the target goal belongs to the caller
// — the service narrows that, checking manage via authzSvc.Can. This is the
// platform.checklists.complete and hrm.interviews.scorecard precedent.
//
//	Goal cycles  GET/POST         /organizations/:orgId/hrm/performance/goal-cycles
//	             GET/PATCH        .../goal-cycles/:cycleId
//	             GET              .../goal-cycles/:cycleId/weight-audit
//	             POST             .../goal-cycles/:cycleId/{activate,lock,close}
//	Goals        GET/POST         .../goals
//	             GET/PATCH/DELETE .../goals/:goalId
//	             POST             .../goals/:goalId/{submit,complete,cancel}
//	             GET/POST         .../goals/:goalId/checkins
//	Employee     GET              /organizations/:orgId/hrm/employees/:employeeId/goals
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	base := router.Group("/organizations/:orgId/hrm/performance", requireAuth, requireOrgMatch)

	// ── Goal cycles ─────────────────────────────────────────────────────
	cycles := base.Group("/goal-cycles")
	cycles.Get("", permFn("hrm.goal_cycles.view"), handler.ListCycles)
	cycles.Post("", permFn("hrm.goal_cycles.manage"), handler.CreateCycle)
	cycles.Get("/:cycleId", permFn("hrm.goal_cycles.view"), handler.GetCycle)
	cycles.Patch("/:cycleId", permFn("hrm.goal_cycles.manage"), handler.UpdateCycle)
	cycles.Get("/:cycleId/weight-audit", permFn("hrm.goal_cycles.view"), handler.GetCycleWeightAudit)
	cycles.Post("/:cycleId/activate", permFn("hrm.goal_cycles.manage"), handler.ActivateCycle)
	cycles.Post("/:cycleId/lock", permFn("hrm.goal_cycles.manage"), handler.LockCycle)
	cycles.Post("/:cycleId/close", permFn("hrm.goal_cycles.manage"), handler.CloseCycle)

	// ── Goals ───────────────────────────────────────────────────────────
	// Writes are gated on set_own, which is granted through 'member'. The
	// service then narrows: writing your own goal needs nothing more, while
	// writing someone else's additionally requires hrm.goals.manage AND that
	// they fall inside the caller's scope tier.
	goals := base.Group("/goals")
	goals.Get("", permFn("hrm.goals.view"), handler.ListGoals)
	goals.Post("", permFn("hrm.goals.set_own"), handler.CreateGoal)
	goals.Get("/:goalId", permFn("hrm.goals.view"), handler.GetGoal)
	goals.Patch("/:goalId", permFn("hrm.goals.set_own"), handler.UpdateGoal)
	goals.Delete("/:goalId", permFn("hrm.goals.set_own"), handler.DeleteGoal)
	goals.Post("/:goalId/submit", permFn("hrm.goals.set_own"), handler.SubmitGoal)
	goals.Post("/:goalId/complete", permFn("hrm.goals.set_own"), handler.CompleteGoal)
	goals.Post("/:goalId/cancel", permFn("hrm.goals.set_own"), handler.CancelGoal)
	goals.Get("/:goalId/checkins", permFn("hrm.goals.view"), handler.ListCheckins)
	goals.Post("/:goalId/checkins", permFn("hrm.goals.set_own"), handler.CreateCheckin)

	// ── Rating scales ───────────────────────────────────────────────────
	// Own group variable per sub-feature: TestRouting_NoDuplicates normalizes
	// every ":x" to ":param" and keys on the receiver, so /:scaleId and
	// /:goalId sharing a group would collide.
	scales := base.Group("/rating-scales")
	scales.Get("", permFn("hrm.rating_scales.view"), handler.ListScales)
	scales.Post("", permFn("hrm.rating_scales.manage"), handler.CreateScale)
	scales.Get("/:scaleId", permFn("hrm.rating_scales.view"), handler.GetScale)
	scales.Patch("/:scaleId", permFn("hrm.rating_scales.manage"), handler.UpdateScale)
	scales.Delete("/:scaleId", permFn("hrm.rating_scales.manage"), handler.DeleteScale)
	scales.Post("/:scaleId/levels", permFn("hrm.rating_scales.manage"), handler.CreateLevel)

	levels := base.Group("/rating-levels")
	levels.Patch("/:levelId", permFn("hrm.rating_scales.manage"), handler.UpdateLevel)
	levels.Delete("/:levelId", permFn("hrm.rating_scales.manage"), handler.DeleteLevel)

	// ── Appraisal cycles ────────────────────────────────────────────────
	appraisalCycles := base.Group("/appraisal-cycles")
	appraisalCycles.Get("", permFn("hrm.appraisals.view"), handler.ListAppraisalCycles)
	appraisalCycles.Post("", permFn("hrm.appraisals.manage"), handler.CreateAppraisalCycle)
	appraisalCycles.Get("/:cycleId", permFn("hrm.appraisals.view"), handler.GetAppraisalCycle)
	appraisalCycles.Patch("/:cycleId", permFn("hrm.appraisals.manage"), handler.UpdateAppraisalCycle)
	appraisalCycles.Post("/:cycleId/activate", permFn("hrm.appraisals.manage"), handler.ActivateAppraisalCycle)
	appraisalCycles.Post("/:cycleId/close", permFn("hrm.appraisals.manage"), handler.CloseAppraisalCycle)
	appraisalCycles.Post("/:cycleId/appraisals", permFn("hrm.appraisals.manage"), handler.InstantiateAppraisal)

	// ── Appraisals ──────────────────────────────────────────────────────
	// Reads are scope-filtered by the service against hrm.appraisals tiers;
	// this is the control that prevents appraisal draft leakage.
	//
	// calibrate and publish carry their own permissions because both override
	// or finalize someone else's assessment: 'manager' holds neither.
	appraisals := base.Group("/appraisals")
	appraisals.Get("", permFn("hrm.appraisals.view"), handler.ListAppraisals)
	appraisals.Get("/:appraisalId", permFn("hrm.appraisals.view"), handler.GetAppraisal)
	appraisals.Post("/:appraisalId/phase", permFn("hrm.appraisals.respond"), handler.AdvancePhase)
	appraisals.Post("/:appraisalId/rating", permFn("hrm.appraisals.review"), handler.SetRating)
	appraisals.Post("/:appraisalId/calibrate", permFn("hrm.appraisals.calibrate"), handler.Calibrate)
	appraisals.Post("/:appraisalId/publish", permFn("hrm.appraisals.publish"), handler.PublishAppraisal)

	// ── Employee-scoped convenience read ────────────────────────────────
	// Mounted under /hrm/employees rather than /hrm/performance so it sits
	// alongside the other per-employee HRM reads.
	empGoals := router.Group("/organizations/:orgId/hrm/employees/:employeeId/goals", requireAuth, requireOrgMatch)
	empGoals.Get("", permFn("hrm.goals.view"), handler.ListGoalsForEmployee)
}
