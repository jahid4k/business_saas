// backend/internal/crm/templates/handler.go
package templates

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles CRM template HTTP endpoints.
type Handler struct {
	service Service
}

// NewHandler creates a new templates Handler.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func orgID(c fiber.Ctx) string  { return c.Params("orgId") }
func userID(c fiber.Ctx) string { id, _ := c.Locals("user_id").(string); return id }

func (h *Handler) ListTemplates(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	result, err := h.service.ListTemplates(c.Context(), orgID(c))
	if err != nil {
		log.Error("templates: ListTemplates", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, result, "OK")
}

func (h *Handler) GetTemplate(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	template, err := h.service.GetTemplate(c.Context(), orgID(c), c.Params("templateId"))
	if err != nil {
		if errors.Is(err, ErrTemplateNotFound) {
			return response.NotFound(c, "TEMPLATE_NOT_FOUND", "Template not found")
		}
		log.Error("templates: GetTemplate", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"template": template}, "OK")
}

func (h *Handler) CreateTemplate(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req CreateTemplateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	template, err := h.service.CreateTemplate(c.Context(), orgID(c), userID(c), req)
	if err != nil {
		if err.Error() == "name is required" || err.Error() == "invalid template type" || err.Error() == "subject is required for email templates" || err.Error() == "body is required" {
			return response.BadRequest(c, "VALIDATION_FAILED", err.Error())
		}
		log.Error("templates: CreateTemplate", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.Created(c, fiber.Map{"template": template}, "Template created")
}

func (h *Handler) UpdateTemplate(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req UpdateTemplateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	template, err := h.service.UpdateTemplate(c.Context(), orgID(c), c.Params("templateId"), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrTemplateNotFound):
			return response.NotFound(c, "TEMPLATE_NOT_FOUND", "Template not found")
		default:
			log.Error("templates: UpdateTemplate", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}
	return response.OK(c, fiber.Map{"template": template}, "Template updated")
}

func (h *Handler) DeleteTemplate(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	if err := h.service.DeleteTemplate(c.Context(), orgID(c), c.Params("templateId")); err != nil {
		if errors.Is(err, ErrTemplateNotFound) {
			return response.NotFound(c, "TEMPLATE_NOT_FOUND", "Template not found")
		}
		log.Error("templates: DeleteTemplate", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.NoContent(c)
}
