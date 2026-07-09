// backend/internal/hrm/reports/routes.go
package reports

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts all HRM report routes under /organizations/:orgId/hrm/reports.
//
//	GET /organizations/:orgId/hrm/reports/overview       <- hrm.reports.view
//	GET /organizations/:orgId/hrm/reports/headcount      <- hrm.reports.view
//	GET /organizations/:orgId/hrm/reports/leave-summary  <- hrm.reports.view
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	reports := router.Group("/organizations/:orgId/hrm/reports", requireAuth, requireOrgMatch)

	reports.Get("/overview", permFn("hrm.reports.view"), handler.GetOverview)
	reports.Get("/headcount", permFn("hrm.reports.view"), handler.GetHeadcount)
	reports.Get("/leave-summary", permFn("hrm.reports.view"), handler.GetLeaveSummary)
}
