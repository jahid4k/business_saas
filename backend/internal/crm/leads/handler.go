// backend/internal/crm/leads/handler.go
package leads

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles CRM lead HTTP endpoints.
type Handler struct {
	service Service
}

// NewHandler creates a new leads Handler.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func orgID(c fiber.Ctx) string  { return c.Params("orgId") }
func userID(c fiber.Ctx) string { id, _ := c.Locals("user_id").(string); return id }

func (h *Handler) ListLeads(c fiber.Ctx) error {
	result, err := h.service.ListLeads(c.Context(), orgID(c))
	if err != nil {
		slog.Error("leads: ListLeads", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, result, "OK")
}

func (h *Handler) GetLead(c fiber.Ctx) error {
	lead, err := h.service.GetLead(c.Context(), orgID(c), c.Params("leadId"))
	if err != nil {
		if errors.Is(err, ErrLeadNotFound) {
			return response.NotFound(c, "LEAD_NOT_FOUND", "Lead not found")
		}
		slog.Error("leads: GetLead", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"lead": lead}, "OK")
}

func (h *Handler) CreateLead(c fiber.Ctx) error {
	var req CreateLeadRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	lead, err := h.service.CreateLead(c.Context(), orgID(c), userID(c), req)
	if err != nil {
		if errors.Is(err, ErrFirstNameRequired) {
			return response.BadRequest(c, "FIRST_NAME_REQUIRED", "first_name is required")
		}
		slog.Error("leads: CreateLead", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.Created(c, fiber.Map{"lead": lead}, "Lead created")
}

func (h *Handler) UpdateLead(c fiber.Ctx) error {
	var req UpdateLeadRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	lead, err := h.service.UpdateLead(c.Context(), orgID(c), c.Params("leadId"), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrLeadNotFound):
			return response.NotFound(c, "LEAD_NOT_FOUND", "Lead not found")
		case errors.Is(err, ErrInvalidStatus):
			return response.BadRequest(c, "INVALID_STATUS", "Invalid status value")
		default:
			slog.Error("leads: UpdateLead", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}
	return response.OK(c, fiber.Map{"lead": lead}, "Lead updated")
}

func (h *Handler) DeleteLead(c fiber.Ctx) error {
	if err := h.service.DeleteLead(c.Context(), orgID(c), c.Params("leadId")); err != nil {
		if errors.Is(err, ErrLeadNotFound) {
			return response.NotFound(c, "LEAD_NOT_FOUND", "Lead not found")
		}
		slog.Error("leads: DeleteLead", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.NoContent(c)
}

func (h *Handler) ConvertLead(c fiber.Ctx) error {
	var req ConvertLeadRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	result, err := h.service.ConvertLead(c.Context(), orgID(c), c.Params("leadId"), userID(c), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrLeadNotFound):
			return response.NotFound(c, "LEAD_NOT_FOUND", "Lead not found")
		case errors.Is(err, ErrLeadAlreadyConverted):
			return response.Conflict(c, "LEAD_ALREADY_CONVERTED", "This lead has already been converted")
		default:
			slog.Error("leads: ConvertLead", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}
	return response.OK(c, result, "Lead converted")
}
