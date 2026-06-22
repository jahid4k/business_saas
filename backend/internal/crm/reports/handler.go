// backend/internal/crm/reports/handler.go
package reports

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles CRM report HTTP endpoints.
type Handler struct {
	service Service
}

// NewHandler creates a new reports Handler.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func orgID(c fiber.Ctx) string { return c.Params("orgId") }

func (h *Handler) GetSummary(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	summary, err := h.service.GetSummary(c.Context(), orgID(c))
	if err != nil {
		log.Error("reports: GetSummary", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"summary": summary}, "OK")
}

func (h *Handler) GetDealsByStage(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	result, err := h.service.GetDealsByStage(c.Context(), orgID(c))
	if err != nil {
		log.Error("reports: GetDealsByStage", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"deals_by_stage": result}, "OK")
}

func (h *Handler) GetDealsByOwner(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	result, err := h.service.GetDealsByOwner(c.Context(), orgID(c))
	if err != nil {
		log.Error("reports: GetDealsByOwner", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"deals_by_owner": result}, "OK")
}

func (h *Handler) GetLeadsBySource(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	result, err := h.service.GetLeadsBySource(c.Context(), orgID(c))
	if err != nil {
		log.Error("reports: GetLeadsBySource", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"leads_by_source": result}, "OK")
}

func (h *Handler) GetOverdueTasks(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	result, err := h.service.GetOverdueTasks(c.Context(), orgID(c))
	if err != nil {
		log.Error("reports: GetOverdueTasks", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"overdue_tasks": result}, "OK")
}

func (h *Handler) GetActivityStats(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	result, err := h.service.GetActivityStats(c.Context(), orgID(c))
	if err != nil {
		log.Error("reports: GetActivityStats", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"activity_stats": result}, "OK")
}

func (h *Handler) GetOverview(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	summary, err := h.service.GetSummary(c.Context(), orgID(c))
	if err != nil {
		log.Error("reports: GetOverview summary", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	recentDeals, err := h.service.GetRecentDeals(c.Context(), orgID(c))
	if err != nil {
		log.Error("reports: GetOverview recent deals", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{
		"summary":      summary,
		"recent_deals": recentDeals,
	}, "OK")
}
