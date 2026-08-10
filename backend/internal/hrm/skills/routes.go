// backend/internal/hrm/skills/routes.go
package skills

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
// Redeclared per package to break the package ↔ middleware import cycle.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts the HRM skills taxonomy.
//
// Conventions enforced by internal/tests/unit/architecture: every permFn
// argument is an INLINE STRING LITERAL (a const fails, since the test reads
// Args[0].(*ast.BasicLit)), and `skills` and `employeeSkills` are separate
// group variables because TestRouting_NoDuplicates normalizes ":x" to ":param"
// and keys on the receiver.
//
// The two halves sit on different paths on purpose. /skills is the ORG
// TAXONOMY — no employee_id, never scope-filtered, readable by anyone who can
// see the catalogue. /employee-skills is PER-PERSON and scope-filtered, which
// is why the tier keys exist. Mounting employee skills under
// /skills/:skillId/employees would have implied the taxonomy owns them.
//
// hrm.skills.manage never gates a per-record write: the route cannot know
// whether the target employee falls inside the caller's reporting line, so the
// service checks CanManage together with AuthorizeRecordAccess. Those routes
// gate on .view — the hrm.goals.manage precedent.
//
//	Taxonomy    GET/POST         /organizations/:orgId/hrm/skills
//	            GET/PATCH/DELETE .../skills/:skillId
//	Employee    GET/POST         /organizations/:orgId/hrm/employee-skills
//	            PATCH/DELETE     .../employee-skills/:employeeSkillId
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	skills := router.Group("/organizations/:orgId/hrm/skills", requireAuth, requireOrgMatch)
	skills.Get("", permFn("hrm.skills.view"), handler.ListSkills)
	skills.Post("", permFn("hrm.skills.manage"), handler.CreateSkill)
	skills.Get("/:skillId", permFn("hrm.skills.view"), handler.GetSkill)
	skills.Patch("/:skillId", permFn("hrm.skills.manage"), handler.UpdateSkill)
	skills.Delete("/:skillId", permFn("hrm.skills.manage"), handler.DeleteSkill)

	employeeSkills := router.Group("/organizations/:orgId/hrm/employee-skills", requireAuth, requireOrgMatch)
	employeeSkills.Get("", permFn("hrm.skills.view"), handler.ListEmployeeSkills)
	employeeSkills.Post("", permFn("hrm.skills.view"), handler.GrantSkill)
	employeeSkills.Patch("/:employeeSkillId", permFn("hrm.skills.view"), handler.UpdateEmployeeSkill)
	employeeSkills.Delete("/:employeeSkillId", permFn("hrm.skills.view"), handler.RevokeSkill)
}
