// backend/internal/hrm/complaints/routes.go
package complaints

import "github.com/gofiber/fiber/v3"

type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts employee complaint routes.
//
//	GET  /organizations/:orgId/hrm/complaints
//	GET/POST /organizations/:orgId/hrm/employees/:employeeId/complaints
//	GET/PATCH .../complaints/:complaintId
//	POST .../complaints/:complaintId/start-review|assign|resolve|dismiss|withdraw
func RegisterRoutes(router fiber.Router, handler *Handler, permFn PermissionFunc, requireAuth, requireOrgMatch fiber.Handler) {
	router.Get("/organizations/:orgId/hrm/complaints",
		requireAuth, requireOrgMatch, permFn("hrm.complaints.view"), handler.ListAll)

	emp := router.Group("/organizations/:orgId/hrm/employees/:employeeId/complaints", requireAuth, requireOrgMatch)
	emp.Post("/:complaintId/start-review", permFn("hrm.complaints.process"), handler.StartReview)
	emp.Post("/:complaintId/assign", permFn("hrm.complaints.process"), handler.Assign)
	emp.Post("/:complaintId/resolve", permFn("hrm.complaints.process"), handler.Resolve)
	emp.Post("/:complaintId/dismiss", permFn("hrm.complaints.process"), handler.Dismiss)
	emp.Post("/:complaintId/withdraw", permFn("hrm.complaints.manage"), handler.Withdraw)
	emp.Get("/", permFn("hrm.complaints.view"), handler.ListForEmployee)
	emp.Post("/", permFn("hrm.complaints.manage"), handler.Create)
	emp.Get("/:complaintId", permFn("hrm.complaints.view"), handler.Get)
	emp.Patch("/:complaintId", permFn("hrm.complaints.manage"), handler.Update)
}
