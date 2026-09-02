// backend/internal/hrm/succession/routes.go
package succession

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory returning permission-enforcing middleware.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts succession under two separate base paths.
//
// Succession (confidential unless noted):
//
//	GET    /organizations/:orgId/hrm/succession/critical-positions            <- hrm.succession.view
//	       ?active=false to include retired designations
//	POST   /organizations/:orgId/hrm/succession/critical-positions            <- hrm.succession.manage
//	PATCH  /organizations/:orgId/hrm/succession/critical-positions/:cpId      <- hrm.succession.manage
//	GET    /organizations/:orgId/hrm/succession/critical-positions/:cpId/candidates
//	                                                                          <- hrm.succession.view_confidential
//	POST   /organizations/:orgId/hrm/succession/critical-positions/:cpId/candidates
//	                                                                          <- hrm.succession.manage
//	POST   /organizations/:orgId/hrm/succession/candidates/:candId/withdraw   <- hrm.succession.manage
//	GET    /organizations/:orgId/hrm/succession/assessments                   <- hrm.succession.view_confidential
//	       ?as_of=YYYY-MM-DD redraws the grid as it stood on that date
//	POST   /organizations/:orgId/hrm/succession/assessments                   <- hrm.succession.manage
//	GET    /organizations/:orgId/hrm/succession/employees/:employeeId/review  <- hrm.succession.view_confidential
//
// Development plans (the subject-visible half):
//
//	GET    /organizations/:orgId/hrm/development-plans/me                     <- hrm.development_plan.view
//	GET    /organizations/:orgId/hrm/development-plans                        <- hrm.development_plan.manage
//	       ?employee_id=<id>
//	POST   /organizations/:orgId/hrm/development-plans                        <- hrm.development_plan.manage
//	GET    /organizations/:orgId/hrm/development-plans/:planId                <- hrm.development_plan.view
//	PATCH  /organizations/:orgId/hrm/development-plans/:planId                <- hrm.development_plan.manage
//	POST   /organizations/:orgId/hrm/development-plans/:planId/items          <- hrm.development_plan.manage
//	PATCH  /organizations/:orgId/hrm/development-plans/items/:itemId          <- hrm.development_plan.view
//
// ⚠ THE TWO BASE PATHS ARE THE CONFIDENTIALITY BOUNDARY MADE VISIBLE IN THE
// URL SPACE. Everything under /succession is a judgement about a named
// person; everything under /development-plans is what that person is
// entitled to see about themselves. They are not nested inside one another
// precisely so a future route cannot be added "just under succession" and
// inherit the wrong gate.
//
// ⚠ hrm.succession.view_confidential is NOT granted to manager (00124). A
// manager may read which roles are critical — org design — but not the
// 9-box, the flight-risk signals, or who has been nominated, because the
// subject's own manager is the reader those judgements most need protecting
// from.
//
// The two routes gated on .view rather than .manage rely on the service to
// restrict them to the caller's OWN employee record: GetPlan and
// UpdatePlanItem resolve the caller's employee id and refuse anything else
// with not-found rather than forbidden, because confirming that a plan
// exists for somebody is itself information.
//
// The literal /me and /items segments are their own groups registered before
// the /:planId route, so neither is swallowed as a plan id.
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	succ := router.Group("/organizations/:orgId/hrm/succession", requireAuth, requireOrgMatch)

	crit := succ.Group("/critical-positions")
	crit.Get("", permFn("hrm.succession.view"), handler.ListCriticalPositions)
	crit.Post("", permFn("hrm.succession.manage"), handler.CreateCriticalPosition)
	crit.Patch("/:cpId", permFn("hrm.succession.manage"), handler.UpdateCriticalPosition)
	crit.Get("/:cpId/candidates", permFn("hrm.succession.view_confidential"), handler.ListCandidates)
	crit.Post("/:cpId/candidates", permFn("hrm.succession.manage"), handler.Nominate)

	cands := succ.Group("/candidates")
	cands.Post("/:candId/withdraw", permFn("hrm.succession.manage"), handler.WithdrawNomination)

	assess := succ.Group("/assessments")
	assess.Get("", permFn("hrm.succession.view_confidential"), handler.NineBoxGrid)
	assess.Post("", permFn("hrm.succession.manage"), handler.RecordAssessment)

	emps := succ.Group("/employees")
	emps.Get("/:employeeId/review", permFn("hrm.succession.view_confidential"), handler.ReviewEmployee)

	plans := router.Group("/organizations/:orgId/hrm/development-plans", requireAuth, requireOrgMatch)

	me := plans.Group("/me")
	me.Get("", permFn("hrm.development_plan.view"), handler.MyDevelopment)

	items := plans.Group("/items")
	items.Patch("/:itemId", permFn("hrm.development_plan.view"), handler.UpdatePlanItem)

	plans.Get("", permFn("hrm.development_plan.manage"), handler.ListPlans)
	plans.Post("", permFn("hrm.development_plan.manage"), handler.CreatePlan)
	plans.Get("/:planId", permFn("hrm.development_plan.view"), handler.GetPlan)
	plans.Patch("/:planId", permFn("hrm.development_plan.manage"), handler.UpdatePlan)
	plans.Post("/:planId/items", permFn("hrm.development_plan.manage"), handler.AddPlanItem)
}
