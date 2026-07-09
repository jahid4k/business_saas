// backend/internal/hrm/employeedocs/routes.go
package employeedocs

import "github.com/gofiber/fiber/v3"

type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts employee document routes.
//
//	GET  /organizations/:orgId/hrm/documents                          ← HR view all
//	GET/POST /organizations/:orgId/hrm/employees/:employeeId/documents
//	GET .../documents/:documentId
//	POST .../documents/:documentId/send|acknowledge|decline|withdraw
func RegisterRoutes(router fiber.Router, handler *Handler, permFn PermissionFunc, requireAuth, requireOrgMatch fiber.Handler) {
	router.Get("/organizations/:orgId/hrm/documents",
		requireAuth, requireOrgMatch, permFn("hrm.documents.view"), handler.ListAll)

	emp := router.Group("/organizations/:orgId/hrm/employees/:employeeId/documents", requireAuth, requireOrgMatch)
	emp.Post("/:documentId/send",        permFn("hrm.documents.manage"),      handler.Send)
	emp.Post("/:documentId/acknowledge", permFn("hrm.documents.acknowledge"), handler.Acknowledge)
	emp.Post("/:documentId/decline",     permFn("hrm.documents.acknowledge"), handler.Decline)
	emp.Post("/:documentId/withdraw",    permFn("hrm.documents.manage"),      handler.Withdraw)
	emp.Get("/",            permFn("hrm.documents.view"),   handler.List)
	emp.Post("/",           permFn("hrm.documents.manage"), handler.Create)
	emp.Get("/:documentId", permFn("hrm.documents.view"),   handler.Get)
}
