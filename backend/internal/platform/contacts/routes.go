// backend/internal/platform/contacts/routes.go
package contacts

import "github.com/gofiber/fiber/v3"

// PermissionFunc is a factory that returns permission-enforcing middleware.
// Same pattern as authz, task, security — breaks the contacts ↔ middleware import cycle.
type PermissionFunc func(permission string) fiber.Handler

// RegisterRoutes mounts contact and company routes under /organizations/:orgId/crm.
//
// Contacts:
//
//	GET    /organizations/:orgId/crm/contacts              <- crm.contacts.view
//	POST   /organizations/:orgId/crm/contacts              <- crm.contacts.create
//	GET    /organizations/:orgId/crm/contacts/:contactId   <- crm.contacts.view
//	PATCH  /organizations/:orgId/crm/contacts/:contactId   <- crm.contacts.update
//	DELETE /organizations/:orgId/crm/contacts/:contactId   <- crm.contacts.delete
//
// Companies:
//
//	GET    /organizations/:orgId/crm/companies                              <- crm.companies.view
//	POST   /organizations/:orgId/crm/companies                              <- crm.companies.create
//	GET    /organizations/:orgId/crm/companies/:companyId                   <- crm.companies.view
//	PATCH  /organizations/:orgId/crm/companies/:companyId                   <- crm.companies.update
//	DELETE /organizations/:orgId/crm/companies/:companyId                   <- crm.companies.delete
//	GET    /organizations/:orgId/crm/companies/:companyId/contacts          <- crm.contacts.view
func RegisterRoutes(
	router fiber.Router,
	handler *Handler,
	permFn PermissionFunc,
	requireAuth fiber.Handler,
	requireOrgMatch fiber.Handler,
) {
	crm := router.Group("/organizations/:orgId/crm", requireAuth, requireOrgMatch)

	// Contacts
	contacts := crm.Group("/contacts")
	contacts.Get("", permFn("crm.contacts.view"), handler.ListContacts)
	contacts.Post("", permFn("crm.contacts.create"), handler.CreateContact)
	contacts.Get("/:contactId", permFn("crm.contacts.view"), handler.GetContact)
	contacts.Patch("/:contactId", permFn("crm.contacts.update"), handler.UpdateContact)
	contacts.Delete("/:contactId", permFn("crm.contacts.delete"), handler.DeleteContact)

	// Companies
	companies := crm.Group("/companies")
	companies.Get("", permFn("crm.companies.view"), handler.ListCompanies)
	companies.Post("", permFn("crm.companies.create"), handler.CreateCompany)
	companies.Get("/:companyId", permFn("crm.companies.view"), handler.GetCompany)
	companies.Patch("/:companyId", permFn("crm.companies.update"), handler.UpdateCompany)
	companies.Delete("/:companyId", permFn("crm.companies.delete"), handler.DeleteCompany)

	// Sub-resource: contacts belonging to a company
	companies.Get("/:companyId/contacts", permFn("crm.contacts.view"), handler.GetContactsByCompany)
}
