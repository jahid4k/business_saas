// backend/internal/crm/deals/handler.go
package deals

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles CRM deal HTTP endpoints.
type Handler struct {
	service Service
}

// NewHandler creates a new deals Handler.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func orgID(c fiber.Ctx) string  { return c.Params("orgId") }
func userID(c fiber.Ctx) string { id, _ := c.Locals("user_id").(string); return id }

func (h *Handler) ListDeals(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	result, err := h.service.ListDeals(c.Context(), orgID(c))
	if err != nil {
		log.Error("deals: ListDeals", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, result, "OK")
}

func (h *Handler) GetDeal(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	deal, err := h.service.GetDeal(c.Context(), orgID(c), c.Params("dealId"))
	if err != nil {
		if errors.Is(err, ErrDealNotFound) {
			return response.NotFound(c, "DEAL_NOT_FOUND", "Deal not found")
		}
		log.Error("deals: GetDeal", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"deal": deal}, "OK")
}

func (h *Handler) CreateDeal(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req CreateDealRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	deal, err := h.service.CreateDeal(c.Context(), orgID(c), userID(c), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrTitleRequired):
			return response.BadRequest(c, "TITLE_REQUIRED", "title is required")
		case errors.Is(err, ErrPipelineRequired):
			return response.BadRequest(c, "PIPELINE_REQUIRED", "pipeline_id is required")
		case errors.Is(err, ErrStageRequired):
			return response.BadRequest(c, "STAGE_REQUIRED", "stage_id is required")
		default:
			log.Error("deals: CreateDeal", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}
	return response.Created(c, fiber.Map{"deal": deal}, "Deal created")
}

func (h *Handler) UpdateDeal(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req UpdateDealRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	deal, err := h.service.UpdateDeal(c.Context(), orgID(c), c.Params("dealId"), req)
	if err != nil {
		if errors.Is(err, ErrDealNotFound) {
			return response.NotFound(c, "DEAL_NOT_FOUND", "Deal not found")
		}
		log.Error("deals: UpdateDeal", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"deal": deal}, "Deal updated")
}

func (h *Handler) DeleteDeal(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	if err := h.service.DeleteDeal(c.Context(), orgID(c), c.Params("dealId")); err != nil {
		if errors.Is(err, ErrDealNotFound) {
			return response.NotFound(c, "DEAL_NOT_FOUND", "Deal not found")
		}
		log.Error("deals: DeleteDeal", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.NoContent(c)
}

func (h *Handler) MoveDealStage(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req MoveDealStageRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	deal, err := h.service.MoveDealStage(c.Context(), orgID(c), c.Params("dealId"), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrDealNotFound):
			return response.NotFound(c, "DEAL_NOT_FOUND", "Deal not found")
		case errors.Is(err, ErrStageNotFound):
			return response.NotFound(c, "STAGE_NOT_FOUND", "Stage not found")
		case errors.Is(err, ErrStageNotInPipeline):
			return response.BadRequest(c, "STAGE_NOT_IN_PIPELINE", "Stage does not belong to this deal's pipeline")
		case errors.Is(err, ErrStageRequired):
			return response.BadRequest(c, "STAGE_REQUIRED", "stage_id is required")
		default:
			log.Error("deals: MoveDealStage", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}
	return response.OK(c, fiber.Map{"deal": deal}, "Deal stage updated")
}

func (h *Handler) MarkDealWon(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	deal, err := h.service.MarkDealWon(c.Context(), orgID(c), c.Params("dealId"))
	if err != nil {
		if errors.Is(err, ErrDealNotFound) {
			return response.NotFound(c, "DEAL_NOT_FOUND", "Deal not found")
		}
		log.Error("deals: MarkDealWon", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"deal": deal}, "Deal marked as won")
}

func (h *Handler) MarkDealLost(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req MarkLostRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	deal, err := h.service.MarkDealLost(c.Context(), orgID(c), c.Params("dealId"), req)
	if err != nil {
		if errors.Is(err, ErrDealNotFound) {
			return response.NotFound(c, "DEAL_NOT_FOUND", "Deal not found")
		}
		log.Error("deals: MarkDealLost", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"deal": deal}, "Deal marked as lost")
}

func (h *Handler) GetPipelineBoard(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	board, err := h.service.GetPipelineBoard(c.Context(), orgID(c), c.Params("pipelineId"))
	if err != nil {
		log.Error("deals: GetPipelineBoard", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"board": board}, "OK")
}
