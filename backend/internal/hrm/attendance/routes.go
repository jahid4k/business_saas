// backend/internal/hrm/attendance/routes.go
package attendance

import "github.com/gofiber/fiber/v3"

type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts attendance routes.
//
// Records:
//
//	GET/POST  /organizations/:orgId/hrm/attendance
//	GET       /organizations/:orgId/hrm/attendance/:recordId
//	POST      /organizations/:orgId/hrm/attendance/:recordId/approve|reject|regularize
//
// Periods:
//
//	GET       /organizations/:orgId/hrm/attendance/periods
//	POST      /organizations/:orgId/hrm/attendance/periods
//	POST      /organizations/:orgId/hrm/attendance/periods/:year/:month/finalize
//	POST      /organizations/:orgId/hrm/attendance/periods/:year/:month/lock
func RegisterRoutes(router fiber.Router, handler *Handler, permFn PermissionFunc, requireAuth, requireOrgMatch fiber.Handler) {
	att := router.Group("/organizations/:orgId/hrm/attendance", requireAuth, requireOrgMatch)

	// Period sub-routes BEFORE record routes (static prefix before /:recordId)
	att.Get("/periods", permFn("hrm.attendance.view"), handler.ListPeriods)
	att.Post("/periods", permFn("hrm.attendance.manage"), handler.GetOrCreatePeriod)
	att.Post("/periods/:year/:month/finalize", permFn("hrm.attendance.finalize"), handler.FinalizePeriod)
	att.Post("/periods/:year/:month/lock", permFn("hrm.attendance.finalize"), handler.LockPeriod)

	// Action routes before /:recordId
	att.Post("/:recordId/approve", permFn("hrm.attendance.approve"), handler.Approve)
	att.Post("/:recordId/reject", permFn("hrm.attendance.approve"), handler.Reject)
	att.Post("/:recordId/regularize", permFn("hrm.attendance.manage"), handler.Regularize)

	// CRUD
	att.Get("/", permFn("hrm.attendance.view"), handler.ListRecords)
	att.Post("/", permFn("hrm.attendance.manage"), handler.Record)
	att.Get("/:recordId", permFn("hrm.attendance.view"), handler.GetRecord)
}
