// backend/internal/hrm/shifts/routes.go
package shifts

import "github.com/gofiber/fiber/v3"

type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts HRM shift configuration and assignment routes.
//
//	GET/POST    /organizations/:orgId/hrm/setup/shifts
//	GET/PATCH/DELETE /organizations/:orgId/hrm/setup/shifts/:shiftId
//	GET/POST    /organizations/:orgId/hrm/setup/shifts/assignments
//	DELETE      /organizations/:orgId/hrm/setup/shifts/assignments/:assignmentId
func RegisterRoutes(router fiber.Router, handler *Handler, permFn PermissionFunc, requireAuth, requireOrgMatch fiber.Handler) {
	base := router.Group("/organizations/:orgId/hrm/setup/shifts", requireAuth, requireOrgMatch)

	// Assignments sub-routes before /:shiftId to avoid param capture
	base.Get("/assignments", permFn("hrm.shifts.view"), handler.ListAssignments)
	base.Post("/assignments", permFn("hrm.shifts.manage"), handler.Assign)
	base.Delete("/assignments/:assignmentId", permFn("hrm.shifts.manage"), handler.RemoveAssignment)

	base.Get("/", permFn("hrm.shifts.view"), handler.ListShifts)
	base.Post("/", permFn("hrm.shifts.manage"), handler.CreateShift)
	base.Get("/:shiftId", permFn("hrm.shifts.view"), handler.GetShift)
	base.Patch("/:shiftId", permFn("hrm.shifts.manage"), handler.UpdateShift)
	base.Delete("/:shiftId", permFn("hrm.shifts.manage"), handler.DeleteShift)
}
