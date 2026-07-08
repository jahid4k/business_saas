// backend/internal/hrm/salary/routes.go
package salary

import "github.com/gofiber/fiber/v3"

// PermissionFunc returns permission-enforcing middleware for a given key.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts all HRM salary routes.
//
// Setup routes (config — HR admin):
//
//	GET/POST    /organizations/:orgId/hrm/setup/salary/components
//	GET/PATCH/DELETE /organizations/:orgId/hrm/setup/salary/components/:compId
//	POST        /organizations/:orgId/hrm/setup/salary/formula/test
//	GET/POST    /organizations/:orgId/hrm/setup/salary/structures
//	GET/PATCH/DELETE /organizations/:orgId/hrm/setup/salary/structures/:structId
//	POST        /organizations/:orgId/hrm/setup/salary/structures/:structId/components
//	DELETE      /organizations/:orgId/hrm/setup/salary/structures/:structId/components/:compId
//
// Employee-scoped routes:
//
//	GET/POST    /organizations/:orgId/hrm/employees/:employeeId/salary
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	// ── Setup: components ────────────────────────────────────────────────────
	comps := router.Group("/organizations/:orgId/hrm/setup/salary/components",
		requireAuth, requireOrgMatch)
	comps.Get("/", permFn("hrm.salary.view"), handler.ListComponents)
	comps.Post("/", permFn("hrm.salary.manage"), handler.CreateComponent)
	comps.Get("/:compId", permFn("hrm.salary.view"), handler.GetComponent)
	comps.Patch("/:compId", permFn("hrm.salary.manage"), handler.UpdateComponent)
	comps.Delete("/:compId", permFn("hrm.salary.manage"), handler.DeleteComponent)

	// ── Setup: formula test ───────────────────────────────────────────────────
	router.Post("/organizations/:orgId/hrm/setup/salary/formula/test",
		requireAuth, requireOrgMatch,
		permFn("hrm.salary.manage"), handler.TestFormula)

	// ── Setup: structures ─────────────────────────────────────────────────────
	structs := router.Group("/organizations/:orgId/hrm/setup/salary/structures",
		requireAuth, requireOrgMatch)
	structs.Get("/", permFn("hrm.salary.view"), handler.ListStructures)
	structs.Post("/", permFn("hrm.salary.manage"), handler.CreateStructure)
	structs.Get("/:structId", permFn("hrm.salary.view"), handler.GetStructure)
	structs.Patch("/:structId", permFn("hrm.salary.manage"), handler.UpdateStructure)
	structs.Delete("/:structId", permFn("hrm.salary.manage"), handler.DeleteStructure)

	// Structure components — sub-actions registered before /:compId to avoid param collision
	structs.Post("/:structId/components", permFn("hrm.salary.manage"), handler.AddComponentToStructure)
	structs.Delete("/:structId/components/:compId", permFn("hrm.salary.manage"), handler.RemoveComponentFromStructure)

	// ── Employee salary records ───────────────────────────────────────────────
	empSalary := router.Group("/organizations/:orgId/hrm/employees/:employeeId/salary",
		requireAuth, requireOrgMatch)
	empSalary.Get("/", permFn("hrm.salary.employee.view"), handler.GetSalaryHistory)
	empSalary.Post("/", permFn("hrm.salary.employee.manage"), handler.AssignSalary)
}
