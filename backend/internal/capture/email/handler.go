package email

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// HandleWebhook receives inbound parsed emails (e.g., from SendGrid).
func (h *Handler) HandleWebhook(c fiber.Ctx) error {
	log := logger.FromCtx(c)

	var payload map[string]any
	// Webhooks might send multipart/form-data (like SendGrid), or application/json (like Postmark).
	// We'll try to bind standard body, and if it fails or is empty, we fall back to manual form parsing.
	if err := c.Bind().Body(&payload); err != nil {
		// Try parsing form fields directly
		form, formErr := c.MultipartForm()
		if formErr == nil && form != nil {
			payload = make(map[string]any)
			for k, v := range form.Value {
				if len(v) > 0 {
					payload[k] = v[0]
				}
			}
		} else {
			return response.BadRequest(c, "BAD_REQUEST", "Unable to parse webhook payload")
		}
	}

	if err := h.service.ProcessInboundWebhook(c.Context(), payload); err != nil {
		log.Error("email: HandleWebhook", slog.Any("error", err))
		return response.InternalServerError(c)
	}

	return c.SendStatus(fiber.StatusOK)
}

func (h *Handler) ListOrgEmails(c fiber.Ctx) error {
	orgID := c.Params("orgId")
	log := logger.FromCtx(c)

	emails, err := h.service.ListOrgEmails(c.Context(), orgID)
	if err != nil {
		log.Error("email: ListOrgEmails", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, emails, "OK")
}

func (h *Handler) CreateOrgEmail(c fiber.Ctx) error {
	orgID := c.Params("orgId")
	log := logger.FromCtx(c)

	var req struct {
		Address string `json:"address"`
	}
	if err := c.Bind().Body(&req); err != nil {
		return response.BadRequest(c, "BAD_REQUEST", "Invalid payload")
	}

	email, err := h.service.CreateOrgEmail(c.Context(), orgID, req.Address)
	if err != nil {
		log.Error("email: CreateOrgEmail", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.Created(c, email, "Email configured")
}

func (h *Handler) DeleteOrgEmail(c fiber.Ctx) error {
	orgID := c.Params("orgId")
	id := c.Params("id")
	log := logger.FromCtx(c)

	if err := h.service.DeleteOrgEmail(c.Context(), orgID, id); err != nil {
		log.Error("email: DeleteOrgEmail", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, nil, "Email configuration deleted")
}
