// backend/internal/hrm/payslips/routes.go
package payslips

import "github.com/gofiber/fiber/v3"

type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts payroll routes.
//
// Runs:
//   GET/POST /organizations/:orgId/hrm/payroll/runs
//   GET      /organizations/:orgId/hrm/payroll/runs/:runId
//   POST     /organizations/:orgId/hrm/payroll/runs/:runId/compute|approve|pay|cancel
//
// Payslips:
//   GET  /organizations/:orgId/hrm/payroll/payslips                  ← filter by run_id / employee_id
//   GET  /organizations/:orgId/hrm/payroll/payslips/:payslipId       ← with component lines
func RegisterRoutes(router fiber.Router, handler *Handler, permFn PermissionFunc, requireAuth, requireOrgMatch fiber.Handler) {
	pr := router.Group("/organizations/:orgId/hrm/payroll", requireAuth, requireOrgMatch)

	// Runs — action routes before /:runId
	pr.Get("/runs",                  permFn("hrm.payroll.view"),    handler.ListRuns)
	pr.Post("/runs",                 permFn("hrm.payroll.manage"),  handler.CreateRun)
	pr.Post("/runs/:runId/compute",  permFn("hrm.payroll.compute"), handler.ComputeRun)
	// The dry run. Its own permission because it persists nothing — safe to
	// grant to reviewers who must never be able to commit a run.
	pr.Post("/runs/:runId/preview",  permFn("hrm.payroll.preview"), handler.PreviewRun)
	pr.Post("/runs/:runId/approve",  permFn("hrm.payroll.approve"), handler.ApproveRun)
	pr.Post("/runs/:runId/pay",      permFn("hrm.payroll.pay"),     handler.MarkPaid)
	pr.Post("/runs/:runId/cancel",   permFn("hrm.payroll.manage"),  handler.CancelRun)
	pr.Get("/runs/:runId",           permFn("hrm.payroll.view"),    handler.GetRun)

	// Payslips (read-only via this route; created by ComputeRun)
	pr.Get("/payslips",              permFn("hrm.payroll.view"), handler.ListPayslips)
	pr.Get("/payslips/:payslipId",   permFn("hrm.payroll.view"), handler.GetPayslip)
}
