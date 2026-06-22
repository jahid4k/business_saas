// backend/internal/crm/pipeline/handler.go
package pipeline

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles pipeline and stage HTTP endpoints.
type Handler struct {
	service Service
}

// NewHandler creates a new pipeline Handler.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func orgID(c fiber.Ctx) string  { return c.Params("orgId") }
func userID(c fiber.Ctx) string { id, _ := c.Locals("user_id").(string); return id }

// ============================================================
// Pipelines
// ============================================================

func (h *Handler) ListPipelines(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	result, err := h.service.ListPipelines(c.Context(), orgID(c))
	if err != nil {
		log.Error("pipeline: ListPipelines", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, result, "OK")
}

func (h *Handler) GetPipeline(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	p, err := h.service.GetPipeline(c.Context(), orgID(c), c.Params("pipelineId"))
	if err != nil {
		if errors.Is(err, ErrPipelineNotFound) {
			return response.NotFound(c, "PIPELINE_NOT_FOUND", "Pipeline not found")
		}
		log.Error("pipeline: GetPipeline", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"pipeline": p}, "OK")
}

func (h *Handler) CreatePipeline(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req CreatePipelineRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	p, err := h.service.CreatePipeline(c.Context(), orgID(c), userID(c), req)
	if err != nil {
		if errors.Is(err, ErrNameRequired) {
			return response.BadRequest(c, "NAME_REQUIRED", "name is required")
		}
		log.Error("pipeline: CreatePipeline", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.Created(c, fiber.Map{"pipeline": p}, "Pipeline created")
}

func (h *Handler) UpdatePipeline(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req UpdatePipelineRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	p, err := h.service.UpdatePipeline(c.Context(), orgID(c), c.Params("pipelineId"), req)
	if err != nil {
		if errors.Is(err, ErrPipelineNotFound) {
			return response.NotFound(c, "PIPELINE_NOT_FOUND", "Pipeline not found")
		}
		log.Error("pipeline: UpdatePipeline", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"pipeline": p}, "Pipeline updated")
}

func (h *Handler) DeletePipeline(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	if err := h.service.DeletePipeline(c.Context(), orgID(c), c.Params("pipelineId")); err != nil {
		if errors.Is(err, ErrPipelineNotFound) {
			return response.NotFound(c, "PIPELINE_NOT_FOUND", "Pipeline not found")
		}
		log.Error("pipeline: DeletePipeline", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.NoContent(c)
}

// ============================================================
// Stages
// ============================================================

func (h *Handler) ListStages(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	result, err := h.service.ListStages(c.Context(), orgID(c), c.Params("pipelineId"))
	if err != nil {
		if errors.Is(err, ErrPipelineNotFound) {
			return response.NotFound(c, "PIPELINE_NOT_FOUND", "Pipeline not found")
		}
		log.Error("pipeline: ListStages", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, result, "OK")
}

func (h *Handler) CreateStage(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req CreateStageRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	st, err := h.service.CreateStage(c.Context(), orgID(c), c.Params("pipelineId"), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrNameRequired):
			return response.BadRequest(c, "NAME_REQUIRED", "name is required")
		case errors.Is(err, ErrPipelineNotFound):
			return response.NotFound(c, "PIPELINE_NOT_FOUND", "Pipeline not found")
		default:
			log.Error("pipeline: CreateStage", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}
	return response.Created(c, fiber.Map{"stage": st}, "Stage created")
}

func (h *Handler) UpdateStage(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req UpdateStageRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	st, err := h.service.UpdateStage(c.Context(), orgID(c), c.Params("pipelineId"), c.Params("stageId"), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrStageNotFound):
			return response.NotFound(c, "STAGE_NOT_FOUND", "Stage not found")
		case errors.Is(err, ErrStageNotInPipeline):
			return response.BadRequest(c, "STAGE_NOT_IN_PIPELINE", "Stage does not belong to this pipeline")
		default:
			log.Error("pipeline: UpdateStage", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}
	return response.OK(c, fiber.Map{"stage": st}, "Stage updated")
}

func (h *Handler) DeleteStage(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	if err := h.service.DeleteStage(c.Context(), orgID(c), c.Params("pipelineId"), c.Params("stageId")); err != nil {
		switch {
		case errors.Is(err, ErrStageNotFound):
			return response.NotFound(c, "STAGE_NOT_FOUND", "Stage not found")
		case errors.Is(err, ErrStageNotInPipeline):
			return response.BadRequest(c, "STAGE_NOT_IN_PIPELINE", "Stage does not belong to this pipeline")
		default:
			log.Error("pipeline: DeleteStage", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}
	return response.NoContent(c)
}

func (h *Handler) ReorderStages(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req ReorderStagesRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	if err := h.service.ReorderStages(c.Context(), orgID(c), c.Params("pipelineId"), req); err != nil {
		log.Error("pipeline: ReorderStages", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, nil, "Stages reordered")
}
