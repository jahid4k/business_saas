// backend/internal/hrm/analytics/routes.go
package analytics

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory returning permission-enforcing middleware.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts people analytics under
// /organizations/:orgId/hrm/analytics.
//
// Definitions:
//
//	GET    /organizations/:orgId/hrm/analytics/metrics                  <- hrm.analytics.view
//	       ?active=false to include retired definitions
//	POST   /organizations/:orgId/hrm/analytics/metrics                  <- hrm.analytics.manage
//	PATCH  /organizations/:orgId/hrm/analytics/metrics/:metricId        <- hrm.analytics.manage
//
// Metrics (all read from the fact tables):
//
//	GET    /organizations/:orgId/hrm/analytics/headcount                <- hrm.analytics.view
//	       ?from=&to=&grain=org|department
//	GET    /organizations/:orgId/hrm/analytics/attrition                <- hrm.analytics.view
//	GET    /organizations/:orgId/hrm/analytics/cohorts                  <- hrm.analytics.view
//	GET    /organizations/:orgId/hrm/analytics/diversity                <- hrm.analytics.view_dei
//	GET    /organizations/:orgId/hrm/analytics/compensation             <- hrm.analytics.view_compensation
//	       ?on=YYYY-MM-DD&grain=org|department
//	GET    /organizations/:orgId/hrm/analytics/export/attrition         <- hrm.analytics.export
//	POST   /organizations/:orgId/hrm/analytics/snapshots/run            <- hrm.analytics.manage
//	       ?on=YYYY-MM-DD to backfill a date
//
// ⚠ NOT SCOPE-TIERED. Analytics is aggregate by construction and the read
// path touches only fact tables, which carry no manager_id for
// scope.Predicate to resolve through. No ResolveScope call exists in this
// package, so TestPermissions_ScopeTiersSeeded does not fire for the
// resource.
//
// ⚠ THE THREE READ KEYS GATE DIFFERENT KINDS OF DATA, NOT DIFFERENT LEVELS
// OF ONE. view is management information; view_compensation exposes pay,
// which on a small team resolves to individuals; view_dei exposes
// demographics and is aggregate-only always. A manager holds view alone.
//
// ⚠ SUPPRESSION IS NOT LIFTED BY ANY OF THEM. /diversity always passes its
// counts through Suppress, and /compensation reads columns the nightly job
// left NULL below the threshold — the pay distribution of a small team is
// never written down, so no route can serve it.
//
// ⚠ /export deliberately carries NO demographic column. A row-level extract
// with gender on it is exactly what "aggregate-only" forbids: a spreadsheet
// the suppression rule cannot reach.
//
// The literal /metrics, /export and /snapshots segments are their own groups
// registered before any :param route, so none is swallowed as an id.
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	base := router.Group("/organizations/:orgId/hrm/analytics", requireAuth, requireOrgMatch)

	metrics := base.Group("/metrics")
	metrics.Get("", permFn("hrm.analytics.view"), handler.ListMetrics)
	metrics.Post("", permFn("hrm.analytics.manage"), handler.CreateMetric)
	metrics.Patch("/:metricId", permFn("hrm.analytics.manage"), handler.UpdateMetric)

	export := base.Group("/export")
	export.Get("/attrition", permFn("hrm.analytics.export"), handler.ExportAttrition)

	snapshots := base.Group("/snapshots")
	snapshots.Post("/run", permFn("hrm.analytics.manage"), handler.RunSnapshot)

	base.Get("/headcount", permFn("hrm.analytics.view"), handler.Headcount)
	base.Get("/attrition", permFn("hrm.analytics.view"), handler.Attrition)
	base.Get("/cohorts", permFn("hrm.analytics.view"), handler.Cohorts)
	base.Get("/diversity", permFn("hrm.analytics.view_dei"), handler.Diversity)
	base.Get("/compensation", permFn("hrm.analytics.view_compensation"), handler.Compensation)
}
