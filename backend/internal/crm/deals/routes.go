// backend/internal/crm/deals/routes.go
package deals

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts all CRM deal routes under /organizations/:orgId/crm/deals.
//
//	GET    /organizations/:orgId/crm/deals                       <- crm.deals.view
//	POST   /organizations/:orgId/crm/deals                       <- crm.deals.create
//	GET    /organizations/:orgId/crm/deals/:dealId               <- crm.deals.view
//	PATCH  /organizations/:orgId/crm/deals/:dealId               <- crm.deals.update
//	DELETE /organizations/:orgId/crm/deals/:dealId               <- crm.deals.delete
//	POST   /organizations/:orgId/crm/deals/:dealId/move          <- crm.deals.move_stage
//	POST   /organizations/:orgId/crm/deals/:dealId/won           <- crm.deals.update
//	POST   /organizations/:orgId/crm/deals/:dealId/lost          <- crm.deals.update
//	GET    /organizations/:orgId/crm/deals/:dealId/board         <- crm.deals.view
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	deals := router.Group("/organizations/:orgId/crm/deals", requireAuth, requireOrgMatch)

	deals.Get("", permFn("crm.deals.view"), handler.ListDeals)
	deals.Post("", permFn("crm.deals.create"), handler.CreateDeal)
	deals.Get("/:dealId", permFn("crm.deals.view"), handler.GetDeal)
	deals.Patch("/:dealId", permFn("crm.deals.update"), handler.UpdateDeal)
	deals.Delete("/:dealId", permFn("crm.deals.delete"), handler.DeleteDeal)
	deals.Post("/:dealId/move", permFn("crm.deals.move_stage"), handler.MoveDealStage)
	deals.Post("/:dealId/won", permFn("crm.deals.update"), handler.MarkDealWon)
	deals.Post("/:dealId/lost", permFn("crm.deals.update"), handler.MarkDealLost)
	deals.Get("/:dealId/board", permFn("crm.deals.view"), handler.GetPipelineBoard)
}
