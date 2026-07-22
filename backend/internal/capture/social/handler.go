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

		verifyToken := h.service.GetWebhookVerifyToken(platform)
		if mode == "subscribe" && token == verifyToken && verifyToken != "" {
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

func (h *Handler) InitOAuth(c fiber.Ctx) error {
	platform := c.Params("platform")
	orgID := c.Query("orgId")
	log := logger.FromCtx(c)

	if orgID == "" {
		return response.BadRequest(c, "BAD_REQUEST", "orgId query parameter is required")
	}

	authURL, err := h.service.GetOAuthInitURL(c.Context(), orgID, platform)
	if err != nil {
		log.Error("social: GetOAuthInitURL", slog.Any("error", err))
		return response.BadRequest(c, "BAD_REQUEST", err.Error())
	}

	return c.Redirect().To(authURL)
}

func (h *Handler) OAuthCallback(c fiber.Ctx) error {
	platform := c.Params("platform")
	code := c.Query("code")
	orgID := c.Query("state") // Assuming we pass orgId in the state parameter
	log := logger.FromCtx(c)

	if code == "" || orgID == "" {
		return response.BadRequest(c, "BAD_REQUEST", "code and state are required")
	}

	if err := h.service.HandleOAuthCallback(c.Context(), orgID, platform, code); err != nil {
		log.Error("social: HandleOAuthCallback", slog.Any("error", err))
		return response.InternalServerError(c)
	}

	// Redirect back to frontend settings page
	return c.Redirect().To("/" + orgID + "/settings/integrations?social_connected=true")
}
