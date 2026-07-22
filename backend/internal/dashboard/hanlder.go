package dashboard

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/mridha/businesssaas/pkg/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) HandleGetMetrics(c fiber.Ctx) error {
	orgID := c.Params("orgId")
	if orgID == "" {
		return response.BadRequest(c, "MISSING_ORG_ID", "orgId is required")
	}

	metrics, err := h.service.GetDashboardMetrics(c.Context(), orgID)
	if err != nil {
		slog.Error("Failed to get dashboard metrics", slog.String("error", err.Error()))
		return response.InternalServerError(c)
	}

	return response.OK(c, metrics, "Dashboard metrics retrieved successfully")
}
