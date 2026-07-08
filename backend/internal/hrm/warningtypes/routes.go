// backend/internal/hrm/warningtypes/routes.go
package warningtypes

import "github.com/gofiber/fiber/v3"

type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts HRM warning type configuration routes.
//
//	GET/POST    /organizations/:orgId/hrm/setup/warning-types
//	GET/PATCH/DELETE /organizations/:orgId/hrm/setup/warning-types/:typeId
//	GET/POST    /organizations/:orgId/hrm/setup/warning-types/escalations
//	PATCH/DELETE /organizations/:orgId/hrm/setup/warning-types/escalations/:ruleId
//
// NOTE: /escalations sub-routes registered before /:typeId to prevent
// "escalations" being parsed as a typeId param value.
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	base := router.Group("/organizations/:orgId/hrm/setup/warning-types", requireAuth, requireOrgMatch)

	// Escalation rules — must come before /:typeId
	base.Get("/escalations", permFn("hrm.warning_types.view"), handler.ListEscalationRules)
	base.Post("/escalations", permFn("hrm.warning_types.manage"), handler.CreateEscalationRule)
	base.Patch("/escalations/:ruleId", permFn("hrm.warning_types.manage"), handler.UpdateEscalationRule)
	base.Delete("/escalations/:ruleId", permFn("hrm.warning_types.manage"), handler.DeleteEscalationRule)

	// Warning type CRUD
	base.Get("/", permFn("hrm.warning_types.view"), handler.ListTypes)
	base.Post("/", permFn("hrm.warning_types.manage"), handler.CreateType)
	base.Get("/:typeId", permFn("hrm.warning_types.view"), handler.GetType)
	base.Patch("/:typeId", permFn("hrm.warning_types.manage"), handler.UpdateType)
	base.Delete("/:typeId", permFn("hrm.warning_types.manage"), handler.DeleteType)
}
