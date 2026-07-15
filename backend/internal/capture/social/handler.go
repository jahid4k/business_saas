package social

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

// HandleWebhook receives payloads from social platforms (e.g. Facebook Lead Ads).
func (h *Handler) HandleWebhook(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	platform := c.Params("platform")

	// Meta webhook verification challenge
	if c.Method() == "GET" && platform == "facebook" {
		mode := c.Query("hub.mode")
		token := c.Query("hub.verify_token")
		challenge := c.Query("hub.challenge")
		
		// In a real app we'd verify the token against our config
		if mode == "subscribe" && token != "" {
			return c.SendString(challenge)
		}
		return response.BadRequest(c, "BAD_REQUEST", "Invalid verification request")
	}

	var payload map[string]any
	if err := c.Bind().Body(&payload); err != nil {
		return response.BadRequest(c, "BAD_REQUEST", "Invalid JSON payload")
	}

	if err := h.service.ProcessWebhook(c.Context(), platform, payload); err != nil {
		log.Error("social: HandleWebhook", slog.Any("error", err))
		return response.InternalServerError(c)
	}

	return c.SendStatus(fiber.StatusOK)
}

func (h *Handler) ListOrgSocials(c fiber.Ctx) error {
	orgID := c.Params("orgId")
	log := logger.FromCtx(c)

	socials, err := h.service.ListOrgSocials(c.Context(), orgID)
	if err != nil {
		log.Error("social: ListOrgSocials", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, socials, "OK")
}

func (h *Handler) CreateOrgSocial(c fiber.Ctx) error {
	orgID := c.Params("orgId")
	log := logger.FromCtx(c)

	var req struct {
		Platform string `json:"platform"`
		PageID   string `json:"page_id"`
	}
	if err := c.Bind().Body(&req); err != nil {
		return response.BadRequest(c, "BAD_REQUEST", "Invalid payload")
	}

	social, err := h.service.CreateOrgSocial(c.Context(), orgID, req.Platform, req.PageID)
	if err != nil {
		log.Error("social: CreateOrgSocial", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.Created(c, social, "Social integration configured")
}

func (h *Handler) DeleteOrgSocial(c fiber.Ctx) error {
	orgID := c.Params("orgId")
	id := c.Params("id")
	log := logger.FromCtx(c)

	if err := h.service.DeleteOrgSocial(c.Context(), orgID, id); err != nil {
		log.Error("social: DeleteOrgSocial", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, nil, "Social integration deleted")
}
