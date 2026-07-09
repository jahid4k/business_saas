// backend/internal/hrm/reports/handler.go
package reports

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles HRM report HTTP endpoints.
type Handler struct {
	service Service
}

// NewHandler creates a new HRM reports Handler.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// orgID is a local helper to extract the :orgId param cleanly.
func orgID(c fiber.Ctx) string { return c.Params("orgId") }

// GetOverview handles GET /api/v1/organizations/:orgId/hrm/reports/overview
// Returns the top-level HRM KPI summary in one response.
// Requires: hrm.reports.view
func (h *Handler) GetOverview(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	summary, err := h.service.GetSummary(c.Context(), orgID(c))
	if err != nil {
		log.Error("hrm reports: GetOverview", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"summary": summary}, "OK")
}

// GetHeadcount handles GET /api/v1/organizations/:orgId/hrm/reports/headcount
// Returns active employee counts broken down by department.
// Requires: hrm.reports.view
func (h *Handler) GetHeadcount(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	result, err := h.service.GetHeadcountByDepartment(c.Context(), orgID(c))
	if err != nil {
		log.Error("hrm reports: GetHeadcount", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"headcount_by_department": result}, "OK")
}

// GetLeaveSummary handles GET /api/v1/organizations/:orgId/hrm/reports/leave-summary
// Returns leave request statistics grouped by leave type.
// Requires: hrm.reports.view
func (h *Handler) GetLeaveSummary(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	result, err := h.service.GetLeaveSummary(c.Context(), orgID(c))
	if err != nil {
		log.Error("hrm reports: GetLeaveSummary", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"leave_summary": result}, "OK")
}
