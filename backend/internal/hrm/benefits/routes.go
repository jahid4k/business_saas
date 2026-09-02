// backend/internal/hrm/benefits/routes.go
package benefits

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
// Redeclared per package to break the package <-> middleware import cycle.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts every HRM benefits route.
//
//   - Every registration carries a permFn("hrm....") argument whose value is
//     an INLINE STRING LITERAL — TestPermissions_AllRoutesProtected.
//   - `plans`, `enrollments` and `dependents` are separate group variables —
//     TestRouting_NoDuplicates.
//   - Every permFn string appears as the first element of an INSERT tuple in
//     migration 00105 — TestPermissions_UsedStringsExistInMigrations.
//
// enroll_self, not manage, gates POST .../enrollments — the route cannot
// express "for yourself only", so Service.EnrollSelf narrows it (the
// hrm.goals.set_own precedent).
//
//	GET/POST  /organizations/:orgId/hrm/benefits/plans
//	GET/POST  .../plans/:planId/tiers
//	GET/POST  .../benefits/enrollments
//	GET       .../enrollments/:enrollmentId
//	POST      .../enrollments/:enrollmentId/waive
//	GET/POST  .../benefits/dependents
//	POST      .../dependents/:dependentId/verify
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	base := router.Group("/organizations/:orgId/hrm/benefits", requireAuth, requireOrgMatch)

	plans := base.Group("/plans")
	plans.Get("", permFn("hrm.benefit_plans.view"), handler.ListPlans)
	plans.Post("", permFn("hrm.benefit_plans.manage"), handler.CreatePlan)
	plans.Get("/:planId/tiers", permFn("hrm.benefit_plans.view"), handler.ListTiers)
	plans.Post("/:planId/tiers", permFn("hrm.benefit_plans.manage"), handler.CreateTier)

	enrollments := base.Group("/enrollments")
	enrollments.Get("", permFn("hrm.benefit_enrollments.view"), handler.ListEnrollments)
	enrollments.Post("", permFn("hrm.benefit_enrollments.enroll_self"), handler.EnrollSelf)
	enrollments.Get("/:enrollmentId", permFn("hrm.benefit_enrollments.view"), handler.GetEnrollment)
	enrollments.Post("/:enrollmentId/waive", permFn("hrm.benefit_enrollments.enroll_self"), handler.WaiveEnrollment)

	dependents := base.Group("/dependents")
	dependents.Get("", permFn("hrm.benefit_enrollments.view"), handler.ListDependents)
	dependents.Post("", permFn("hrm.benefit_enrollments.enroll_self"), handler.CreateDependent)
	dependents.Post("/:dependentId/verify", permFn("hrm.benefit_enrollments.verify_dependent"), handler.VerifyDependent)
}
