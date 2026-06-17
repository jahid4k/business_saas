// backend/internal/crm/reports/routes.go
package reports

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts all CRM report routes under /organizations/:orgId/crm/reports.
//
//	GET /organizations/:orgId/crm/reports/overview          <- crm.reports.view
//	GET /organizations/:orgId/crm/reports/summary           <- crm.reports.view
//	GET /organizations/:orgId/crm/reports/deals/by-stage    <- crm.reports.view
//	GET /organizations/:orgId/crm/reports/deals/by-owner    <- crm.reports.view
//	GET /organizations/:orgId/crm/reports/leads/by-source   <- crm.reports.view
//	GET /organizations/:orgId/crm/reports/tasks/overdue     <- crm.reports.view
//	GET /organizations/:orgId/crm/reports/activities/stats  <- crm.reports.view
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	reports := router.Group("/organizations/:orgId/crm/reports", requireAuth, requireOrgMatch)

	reports.Get("/overview", permFn("crm.reports.view"), handler.GetOverview)
	reports.Get("/summary", permFn("crm.reports.view"), handler.GetSummary)
	reports.Get("/deals/by-stage", permFn("crm.reports.view"), handler.GetDealsByStage)
	reports.Get("/deals/by-owner", permFn("crm.reports.view"), handler.GetDealsByOwner)
	reports.Get("/leads/by-source", permFn("crm.reports.view"), handler.GetLeadsBySource)
	reports.Get("/tasks/overdue", permFn("crm.reports.view"), handler.GetOverdueTasks)
	reports.Get("/activities/stats", permFn("crm.reports.view"), handler.GetActivityStats)
}
