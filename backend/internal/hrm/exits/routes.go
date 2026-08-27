// backend/internal/hrm/exits/routes.go
package exits

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory returning permission-enforcing middleware.
// Same pattern as every other HRM package — breaks the exits ↔ middleware
// import cycle.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts exit management under /organizations/:orgId/hrm/exits.
//
// Exits:
//
//	POST   /organizations/:orgId/hrm/exits                                    <- hrm.exits.manage
//	GET    /organizations/:orgId/hrm/exits                                    <- hrm.exits.view_own
//	GET    /organizations/:orgId/hrm/exits/:exitId                            <- hrm.exits.view_own
//	PATCH  /organizations/:orgId/hrm/exits/:exitId                            <- hrm.exits.manage
//	POST   /organizations/:orgId/hrm/exits/:exitId/cancel                     <- hrm.exits.manage
//	POST   /organizations/:orgId/hrm/exits/:exitId/clearance/start            <- hrm.exits.manage
//
// Clearance:
//
//	GET    /organizations/:orgId/hrm/exits/:exitId/clearance                  <- hrm.exits.view_own
//	POST   /organizations/:orgId/hrm/exits/:exitId/clearance                  <- hrm.exits.clear
//	POST   /organizations/:orgId/hrm/exits/:exitId/clearance/:itemId/resolve  <- hrm.exits.clear
//
// Settlement (9B):
//
//	GET    /organizations/:orgId/hrm/exits/:exitId/settlement                 <- hrm.exits.view_own
//	POST   /organizations/:orgId/hrm/exits/:exitId/settlement/run             <- hrm.exits.settle
//
// Gratuity rules (9B):
//
//	GET    /organizations/:orgId/hrm/exits/gratuity-rules                     <- hrm.gratuity.view
//	POST   /organizations/:orgId/hrm/exits/gratuity-rules                     <- hrm.gratuity.manage
//
// Rehire:
//
//	GET    /organizations/:orgId/hrm/exits/rehire/:employeeId                 <- hrm.exits.view_own
//	PUT    /organizations/:orgId/hrm/exits/rehire/:employeeId                 <- hrm.exits.decide_rehire
//
// The READ routes gate on the LOWEST tier, `view_own`, and the service then
// narrows what they actually return by the caller's resolved tier — the
// established HRM shape. A route gate cannot express "your own exit", and
// gating reads on view_all would lock a departing employee out of watching
// their own clearance, which is the frustration this module exists to remove.
//
// `hrm.exits.settle` gates attaching a payroll run to an exit, and separately
// gates F&F APPROVAL on the payroll run itself. Separation of duties: running
// clearance is HR-operational work, approving the money that leaves with the
// employee is a finance authority.
//
// The literal /rehire segment is registered as its own group BEFORE the
// /:exitId group, so a request for /rehire/... is not swallowed as an exit id
// (the /companies/enrich and /instances/mine precedent).
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	base := router.Group("/organizations/:orgId/hrm/exits", requireAuth, requireOrgMatch)

	// Literal segments first, so /gratuity-rules and /rehire are not
	// swallowed as an :exitId (the /companies/enrich precedent).
	gratuity := base.Group("/gratuity-rules")
	gratuity.Get("", permFn("hrm.gratuity.view"), handler.ListGratuityRules)
	gratuity.Post("", permFn("hrm.gratuity.manage"), handler.CreateGratuityRule)

	rehire := base.Group("/rehire")
	rehire.Get("/:employeeId", permFn("hrm.exits.view_own"), handler.GetRehire)
	rehire.Put("/:employeeId", permFn("hrm.exits.decide_rehire"), handler.DecideRehire)

	base.Post("", permFn("hrm.exits.manage"), handler.CreateExit)
	base.Get("", permFn("hrm.exits.view_own"), handler.ListExits)

	items := base.Group("/:exitId")
	items.Get("", permFn("hrm.exits.view_own"), handler.GetExit)
	items.Patch("", permFn("hrm.exits.manage"), handler.UpdateExit)
	items.Post("/cancel", permFn("hrm.exits.manage"), handler.CancelExit)
	items.Post("/clearance/start", permFn("hrm.exits.manage"), handler.StartClearance)

	items.Get("/clearance", permFn("hrm.exits.view_own"), handler.ListClearanceItems)
	items.Post("/clearance", permFn("hrm.exits.clear"), handler.AddClearanceItem)
	items.Post("/clearance/:itemId/resolve", permFn("hrm.exits.clear"), handler.ResolveClearanceItem)

	items.Get("/settlement", permFn("hrm.exits.view_own"), handler.ListSettlementLines)
	items.Post("/settlement/run", permFn("hrm.exits.settle"), handler.AttachFnFRun)
}
