// backend/internal/hrm/certifications/routes.go
package certifications

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
// Redeclared per package to break the package ↔ middleware import cycle.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts the HRM certification surface.
//
// Conventions enforced by internal/tests/unit/architecture: every permFn
// argument is an INLINE STRING LITERAL (a const fails, since the test reads
// Args[0].(*ast.BasicLit)), and `catalogue` and `credentials` are separate
// group variables because TestRouting_NoDuplicates normalizes ":x" to ":param"
// and keys on the receiver.
//
// The two halves sit on different paths for the same reason skills does:
// /certifications is the ORG CATALOGUE — no employee_id, never scope-filtered.
// /employee-certifications is PER-PERSON and scope-filtered, which is why the
// tier keys exist.
//
// hrm.certifications.manage never gates a per-record write: the route cannot
// know whether the target employee is inside the caller's reporting line, so
// the service checks CanManage together with AuthorizeRecordAccess. Those
// routes gate on .view — the hrm.goals.manage precedent.
//
// The EXPIRY SWEEP has no route at all. It is a scheduler job registered in
// main.go, running across every org in the instance — the same shape as the
// leave accrual and absence sweeps. A route would need an org, and the sweep
// deliberately has none.
//
//	Catalogue    GET/POST         /organizations/:orgId/hrm/certifications
//	             GET/PATCH/DELETE .../certifications/:certificationId
//	Credentials  GET/POST         /organizations/:orgId/hrm/employee-certifications
//	             PATCH            .../employee-certifications/:employeeCertificationId
//	             POST             .../employee-certifications/:employeeCertificationId/revoke
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	catalogue := router.Group("/organizations/:orgId/hrm/certifications", requireAuth, requireOrgMatch)
	catalogue.Get("", permFn("hrm.certifications.view"), handler.List)
	catalogue.Post("", permFn("hrm.certifications.manage"), handler.Create)
	catalogue.Get("/:certificationId", permFn("hrm.certifications.view"), handler.Get)
	catalogue.Patch("/:certificationId", permFn("hrm.certifications.manage"), handler.Update)
	catalogue.Delete("/:certificationId", permFn("hrm.certifications.manage"), handler.Delete)

	credentials := router.Group("/organizations/:orgId/hrm/employee-certifications", requireAuth, requireOrgMatch)
	credentials.Get("", permFn("hrm.certifications.view"), handler.ListEmployeeCertifications)
	credentials.Post("", permFn("hrm.certifications.view"), handler.Issue)
	credentials.Patch("/:employeeCertificationId", permFn("hrm.certifications.view"), handler.UpdateEmployeeCertification)
	credentials.Post("/:employeeCertificationId/revoke", permFn("hrm.certifications.view"), handler.Revoke)
}
