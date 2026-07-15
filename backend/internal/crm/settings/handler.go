package settings

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

func orgID(c fiber.Ctx) string {
	return c.Params("orgId")
}

func (h *Handler) GetSettings(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	settings, err := h.service.GetSettings(c.Context(), orgID(c))
	if err != nil {
		log.Error("settings: GetSettings", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, settings, "OK")
}

func (h *Handler) UpdateSettings(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req UpdateCRMSettingsRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_PAYLOAD", "Invalid request payload")
	}

	settings, err := h.service.UpdateSettings(c.Context(), orgID(c), req)
	if err != nil {
		log.Error("settings: UpdateSettings", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, settings, "Settings updated successfully")
}
