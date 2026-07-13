// backend/internal/hrm/leave/routes.go
package leave

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts all HRM leave routes under /organizations/:orgId/hrm/leave.
//
// Leave Types:
//
//	GET    /organizations/:orgId/hrm/leave/types           <- hrm.leave.view
//	POST   /organizations/:orgId/hrm/leave/types           <- hrm.leave.create
//	GET    /organizations/:orgId/hrm/leave/types/:typeId   <- hrm.leave.view
//	PATCH  /organizations/:orgId/hrm/leave/types/:typeId   <- hrm.leave.update
//	DELETE /organizations/:orgId/hrm/leave/types/:typeId   <- hrm.leave.delete
//
// Leave Requests:
//
//	GET    /organizations/:orgId/hrm/leave/requests                    <- hrm.leave.view
//	POST   /organizations/:orgId/hrm/leave/requests                    <- hrm.leave.request
//	GET    /organizations/:orgId/hrm/leave/requests/:reqId             <- hrm.leave.view
//	POST   /organizations/:orgId/hrm/leave/requests/:reqId/approve     <- hrm.leave.approve
//	POST   /organizations/:orgId/hrm/leave/requests/:reqId/reject      <- hrm.leave.approve
//	POST   /organizations/:orgId/hrm/leave/requests/:reqId/cancel      <- hrm.leave.request
//	DELETE /organizations/:orgId/hrm/leave/requests/:reqId             <- hrm.leave.delete
//
// NOTE: approve/reject/cancel sub-routes are registered BEFORE /:reqId
// to prevent Fiber matching the literal strings "approve", "reject", "cancel"
// as the :reqId path parameter.
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	leave := router.Group("/organizations/:orgId/hrm/leave", requireAuth, requireOrgMatch)

	// ── Leave Types ──────────────────────────────────────────────────────────
	types := leave.Group("/types")
	types.Get("/", permFn("hrm.leave.view"), handler.ListLeaveTypes)
	types.Post("/", permFn("hrm.leave.create"), handler.CreateLeaveType)
	types.Get("/:typeId", permFn("hrm.leave.view"), handler.GetLeaveType)
	types.Patch("/:typeId", permFn("hrm.leave.update"), handler.UpdateLeaveType)
	types.Delete("/:typeId", permFn("hrm.leave.delete"), handler.DeleteLeaveType)

	// ── Leave Requests ───────────────────────────────────────────────────────
	requests := leave.Group("/requests")
	requests.Get("/", permFn("hrm.leave.view"), handler.ListRequests)
	requests.Post("/", permFn("hrm.leave.request"), handler.CreateRequest)

	// Sub-actions MUST be registered before /:reqId — otherwise Fiber
	// resolves "approve", "reject", "cancel" as the :reqId param value.
	requests.Post("/:reqId/approve", permFn("hrm.leave.approve"), handler.ApproveRequest)
	requests.Post("/:reqId/reject", permFn("hrm.leave.approve"), handler.RejectRequest)
	requests.Post("/:reqId/cancel", permFn("hrm.leave.request"), handler.CancelRequest)

	requests.Get("/:reqId", permFn("hrm.leave.view"), handler.GetRequest)
	requests.Delete("/:reqId", permFn("hrm.leave.delete"), handler.DeleteRequest)
}
