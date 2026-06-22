// backend/internal/platform/contacts/handler.go
package contacts

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/pagination"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles contact and company HTTP endpoints.
type Handler struct {
	service Service
}

// NewHandler creates a new contacts Handler.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func orgID(c fiber.Ctx) string  { return c.Params("orgId") }
func userID(c fiber.Ctx) string { id, _ := c.Locals("user_id").(string); return id }

// ============================================================
// Contacts
// ============================================================

// ListContacts handles GET /api/v1/organizations/:orgId/crm/contacts
// Accepts ?limit= and ?offset= query params (defaults: limit=50, max=200).
func (h *Handler) ListContacts(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	p := pagination.FromCtx(c)
	result, err := h.service.ListContacts(c.Context(), orgID(c), p)
	if err != nil {
		log.Error("contacts: ListContacts", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, result, "OK")
}

// GetContact handles GET /api/v1/organizations/:orgId/crm/contacts/:contactId
func (h *Handler) GetContact(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	contact, err := h.service.GetContact(c.Context(), orgID(c), c.Params("contactId"))
	if err != nil {
		if errors.Is(err, ErrContactNotFound) {
			return response.NotFound(c, "CONTACT_NOT_FOUND", "Contact not found")
		}
		log.Error("contacts: GetContact", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"contact": contact}, "OK")
}

// CreateContact handles POST /api/v1/organizations/:orgId/crm/contacts
func (h *Handler) CreateContact(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req CreateContactRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	contact, err := h.service.CreateContact(c.Context(), orgID(c), userID(c), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrFirstNameRequired):
			return response.BadRequest(c, "FIRST_NAME_REQUIRED", "first_name is required")
		case errors.Is(err, ErrInvalidEmail):
			return response.BadRequest(c, "INVALID_EMAIL", "Invalid email address")
		default:
			log.Error("contacts: CreateContact", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}
	return response.Created(c, fiber.Map{"contact": contact}, "Contact created")
}

// UpdateContact handles PATCH /api/v1/organizations/:orgId/crm/contacts/:contactId
func (h *Handler) UpdateContact(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req UpdateContactRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	contact, err := h.service.UpdateContact(c.Context(), orgID(c), c.Params("contactId"), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrContactNotFound):
			return response.NotFound(c, "CONTACT_NOT_FOUND", "Contact not found")
		case errors.Is(err, ErrInvalidStatus):
			return response.BadRequest(c, "INVALID_STATUS", "Invalid status value")
		case errors.Is(err, ErrInvalidEmail):
			return response.BadRequest(c, "INVALID_EMAIL", "Invalid email address")
		default:
			log.Error("contacts: UpdateContact", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}
	return response.OK(c, fiber.Map{"contact": contact}, "Contact updated")
}

// DeleteContact handles DELETE /api/v1/organizations/:orgId/crm/contacts/:contactId
func (h *Handler) DeleteContact(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	if err := h.service.DeleteContact(c.Context(), orgID(c), c.Params("contactId")); err != nil {
		if errors.Is(err, ErrContactNotFound) {
			return response.NotFound(c, "CONTACT_NOT_FOUND", "Contact not found")
		}
		log.Error("contacts: DeleteContact", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.NoContent(c)
}

// GetContactsByCompany handles GET /api/v1/organizations/:orgId/crm/companies/:companyId/contacts
func (h *Handler) GetContactsByCompany(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	contacts, err := h.service.GetContactsByCompany(c.Context(), orgID(c), c.Params("companyId"))
	if err != nil {
		log.Error("contacts: GetContactsByCompany", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"contacts": contacts}, "OK")
}

// ============================================================
// Companies
// ============================================================

// ListCompanies handles GET /api/v1/organizations/:orgId/crm/companies
// Accepts ?limit= and ?offset= query params (defaults: limit=50, max=200).
func (h *Handler) ListCompanies(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	p := pagination.FromCtx(c)
	result, err := h.service.ListCompanies(c.Context(), orgID(c), p)
	if err != nil {
		log.Error("contacts: ListCompanies", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, result, "OK")
}

// GetCompany handles GET /api/v1/organizations/:orgId/crm/companies/:companyId
func (h *Handler) GetCompany(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	company, err := h.service.GetCompany(c.Context(), orgID(c), c.Params("companyId"))
	if err != nil {
		if errors.Is(err, ErrCompanyNotFound) {
			return response.NotFound(c, "COMPANY_NOT_FOUND", "Company not found")
		}
		log.Error("contacts: GetCompany", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"company": company}, "OK")
}

// CreateCompany handles POST /api/v1/organizations/:orgId/crm/companies
func (h *Handler) CreateCompany(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req CreateCompanyRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	company, err := h.service.CreateCompany(c.Context(), orgID(c), userID(c), req)
	if err != nil {
		if errors.Is(err, ErrNameRequired) {
			return response.BadRequest(c, "NAME_REQUIRED", "name is required")
		}
		log.Error("contacts: CreateCompany", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.Created(c, fiber.Map{"company": company}, "Company created")
}

// UpdateCompany handles PATCH /api/v1/organizations/:orgId/crm/companies/:companyId
func (h *Handler) UpdateCompany(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req UpdateCompanyRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	company, err := h.service.UpdateCompany(c.Context(), orgID(c), c.Params("companyId"), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrCompanyNotFound):
			return response.NotFound(c, "COMPANY_NOT_FOUND", "Company not found")
		case errors.Is(err, ErrInvalidStatus):
			return response.BadRequest(c, "INVALID_STATUS", "Invalid status value")
		default:
			log.Error("contacts: UpdateCompany", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}
	return response.OK(c, fiber.Map{"company": company}, "Company updated")
}

// DeleteCompany handles DELETE /api/v1/organizations/:orgId/crm/companies/:companyId
func (h *Handler) DeleteCompany(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	if err := h.service.DeleteCompany(c.Context(), orgID(c), c.Params("companyId")); err != nil {
		if errors.Is(err, ErrCompanyNotFound) {
			return response.NotFound(c, "COMPANY_NOT_FOUND", "Company not found")
		}
		log.Error("contacts: DeleteCompany", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.NoContent(c)
}
