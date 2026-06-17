// backend/internal/crm/pipeline/routes.go
package pipeline

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts pipeline and stage routes under /organizations/:orgId/crm/pipelines.
//
// Pipelines:
//
//	GET    /organizations/:orgId/crm/pipelines                                  <- crm.deals.view
//	POST   /organizations/:orgId/crm/pipelines                                  <- crm.deals.create
//	GET    /organizations/:orgId/crm/pipelines/:pipelineId                      <- crm.deals.view
//	PATCH  /organizations/:orgId/crm/pipelines/:pipelineId                      <- crm.deals.update
//	DELETE /organizations/:orgId/crm/pipelines/:pipelineId                      <- crm.deals.delete
//
// Stages (nested under pipeline):
//
//	GET    /organizations/:orgId/crm/pipelines/:pipelineId/stages               <- crm.deals.view
//	POST   /organizations/:orgId/crm/pipelines/:pipelineId/stages               <- crm.deals.create
//	PATCH  /organizations/:orgId/crm/pipelines/:pipelineId/stages/:stageId      <- crm.deals.update
//	DELETE /organizations/:orgId/crm/pipelines/:pipelineId/stages/:stageId      <- crm.deals.delete
//	POST   /organizations/:orgId/crm/pipelines/:pipelineId/stages/reorder       <- crm.deals.update
//
// Note: /reorder is registered before /:stageId to prevent param shadowing
// (Fiber StrictRouting + CaseSensitive is on, but literal segments still
// need to come before param segments on the same level).
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	pipelines := router.Group("/organizations/:orgId/crm/pipelines", requireAuth, requireOrgMatch)

	pipelines.Get("", permFn("crm.deals.view"), handler.ListPipelines)
	pipelines.Post("", permFn("crm.deals.create"), handler.CreatePipeline)
	pipelines.Get("/:pipelineId", permFn("crm.deals.view"), handler.GetPipeline)
	pipelines.Patch("/:pipelineId", permFn("crm.deals.update"), handler.UpdatePipeline)
	pipelines.Delete("/:pipelineId", permFn("crm.deals.delete"), handler.DeletePipeline)

	stages := pipelines.Group("/:pipelineId/stages")
	stages.Get("", permFn("crm.deals.view"), handler.ListStages)
	stages.Post("", permFn("crm.deals.create"), handler.CreateStage)
	// /reorder must be registered before /:stageId — literal wins over param
	stages.Post("/reorder", permFn("crm.deals.update"), handler.ReorderStages)
	stages.Patch("/:stageId", permFn("crm.deals.update"), handler.UpdateStage)
	stages.Delete("/:stageId", permFn("crm.deals.delete"), handler.DeleteStage)
}
