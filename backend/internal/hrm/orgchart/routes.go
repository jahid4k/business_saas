// backend/internal/hrm/orgchart/routes.go
package orgchart

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory returning permission-enforcing middleware.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts the org chart under
// /organizations/:orgId/hrm/org-chart.
//
// Chart:
//
//	GET    /organizations/:orgId/hrm/org-chart                               <- hrm.org_chart.view
//	GET    /organizations/:orgId/hrm/org-chart/chain/:employeeId             <- hrm.org_chart.view
//
// Relationships:
//
//	GET    /organizations/:orgId/hrm/org-chart/relationships                 <- hrm.org_chart.view
//	       ?employee_id=<id>  ?active=false to include ended lines (history)
//	POST   /organizations/:orgId/hrm/org-chart/relationships                 <- hrm.org_chart.manage
//	POST   /organizations/:orgId/hrm/org-chart/relationships/:relId/end      <- hrm.org_chart.manage
//
// Seats:
//
//	GET    /organizations/:orgId/hrm/org-chart/seats                         <- hrm.org_chart.view
//	       ?position_id=<id>  ?vacant=true to list unfilled headcount only
//	POST   /organizations/:orgId/hrm/org-chart/seats                         <- hrm.org_chart.manage
//	POST   /organizations/:orgId/hrm/org-chart/seats/:seatId/assign          <- hrm.org_chart.manage
//
// ⚠ These routes are NOT scope-tiered, unlike most of HRM. A chart whose
// shape depends on who is looking is a subtree rather than a chart, and both
// succession (10B) and analytics (10C) need the whole graph to compute
// anything. What is sensitive is the salary and appraisal data hanging off
// each node, and that stays behind its own already-tiered resources.
//
// .manage is separated from .view because editing a SOLID line writes
// hrm_employees.manager_id, which scope.Predicate's view_team tier resolves
// through — so re-parenting somebody silently changes who can read whose
// payroll. That is an HR-administrative act.
//
// The literal /relationships, /seats and /chain segments are their own groups
// registered before any :param route, so none is swallowed as an id.
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	base := router.Group("/organizations/:orgId/hrm/org-chart", requireAuth, requireOrgMatch)

	base.Get("", permFn("hrm.org_chart.view"), handler.GetChart)

	chain := base.Group("/chain")
	chain.Get("/:employeeId", permFn("hrm.org_chart.view"), handler.GetManagementChain)

	rels := base.Group("/relationships")
	rels.Get("", permFn("hrm.org_chart.view"), handler.ListRelationships)
	rels.Post("", permFn("hrm.org_chart.manage"), handler.CreateRelationship)
	rels.Post("/:relId/end", permFn("hrm.org_chart.manage"), handler.EndRelationship)

	seats := base.Group("/seats")
	seats.Get("", permFn("hrm.org_chart.view"), handler.ListSeats)
	seats.Post("", permFn("hrm.org_chart.manage"), handler.CreateSeat)
	seats.Post("/:seatId/assign", permFn("hrm.org_chart.manage"), handler.AssignSeat)
}
