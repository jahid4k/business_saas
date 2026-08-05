// backend/internal/hrm/recruitment/pipelines_handler.go
package recruitment

import (
	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/response"
)

// ListPipelines godoc
//
//	@Summary		List recruitment pipelines
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Success		200		{object}	response.OK{data=object{pipelines=[]Pipeline}}
//	@Router			/organizations/{orgId}/hrm/recruitment/pipelines [get]
func (h *Handler) ListPipelines(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	list, err := h.service.ListPipelines(c.Context(), orgID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"pipelines": list}, "OK")
}

// GetPipeline godoc
//
//	@Summary		Get recruitment pipeline (with stages)
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			pipelineId	path	string	true	"Pipeline public ID"
//	@Success		200			{object}	response.OK{data=object{pipeline=Pipeline}}
//	@Router			/organizations/{orgId}/hrm/recruitment/pipelines/{pipelineId} [get]
func (h *Handler) GetPipeline(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	p, err := h.service.GetPipeline(c.Context(), orgID, c.Params("pipelineId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"pipeline": p}, "OK")
}

// CreatePipeline godoc
//
//	@Summary		Create recruitment pipeline
//	@Tags			HRM / Recruitment
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string					true	"Organization ID"
//	@Param			body	body	CreatePipelineRequest	true	"Pipeline"
//	@Success		201		{object}	response.Created{data=object{pipeline=Pipeline}}
//	@Router			/organizations/{orgId}/hrm/recruitment/pipelines [post]
func (h *Handler) CreatePipeline(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreatePipelineRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	p, err := h.service.CreatePipeline(c.Context(), orgID, userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"pipeline": p}, "Pipeline created")
}

// UpdatePipeline godoc
//
//	@Summary		Update recruitment pipeline
//	@Tags			HRM / Recruitment
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string					true	"Organization ID"
//	@Param			pipelineId	path	string					true	"Pipeline public ID"
//	@Param			body		body	UpdatePipelineRequest	true	"Fields to update"
//	@Success		200			{object}	response.OK{data=object{pipeline=Pipeline}}
//	@Router			/organizations/{orgId}/hrm/recruitment/pipelines/{pipelineId} [patch]
func (h *Handler) UpdatePipeline(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdatePipelineRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	p, err := h.service.UpdatePipeline(c.Context(), orgID, c.Params("pipelineId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"pipeline": p}, "Pipeline updated")
}

// DeletePipeline godoc
//
//	@Summary		Delete recruitment pipeline
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			pipelineId	path	string	true	"Pipeline public ID"
//	@Success		204
//	@Router			/organizations/{orgId}/hrm/recruitment/pipelines/{pipelineId} [delete]
func (h *Handler) DeletePipeline(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.DeletePipeline(c.Context(), orgID, c.Params("pipelineId")); err != nil {
		return h.err(c, err)
	}
	return response.NoContent(c)
}

// ============================================================
// Stages
// ============================================================

// ListStages godoc
//
//	@Summary		List stages within a pipeline
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			pipelineId	path	string	true	"Pipeline public ID"
//	@Success		200			{object}	response.OK{data=object{stages=[]Stage}}
//	@Router			/organizations/{orgId}/hrm/recruitment/pipelines/{pipelineId}/stages [get]
func (h *Handler) ListStages(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	list, err := h.service.ListStages(c.Context(), orgID, c.Params("pipelineId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"stages": list}, "OK")
}

// CreateStage godoc
//
//	@Summary		Create a stage within a pipeline
//	@Tags			HRM / Recruitment
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string				true	"Organization ID"
//	@Param			pipelineId	path	string				true	"Pipeline public ID"
//	@Param			body		body	CreateStageRequest	true	"Stage"
//	@Success		201			{object}	response.Created{data=object{stage=Stage}}
//	@Router			/organizations/{orgId}/hrm/recruitment/pipelines/{pipelineId}/stages [post]
func (h *Handler) CreateStage(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateStageRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	st, err := h.service.CreateStage(c.Context(), orgID, c.Params("pipelineId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"stage": st}, "Stage created")
}

// ReorderStages godoc
//
//	@Summary		Reorder stages within a pipeline
//	@Tags			HRM / Recruitment
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string					true	"Organization ID"
//	@Param			pipelineId	path	string					true	"Pipeline public ID"
//	@Param			body		body	ReorderStagesRequest	true	"Ordered list of stage IDs"
//	@Success		200			{object}	response.OK
//	@Router			/organizations/{orgId}/hrm/recruitment/pipelines/{pipelineId}/stages/reorder [post]
func (h *Handler) ReorderStages(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req ReorderStagesRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	if err := h.service.ReorderStages(c.Context(), orgID, c.Params("pipelineId"), req); err != nil {
		return h.err(c, err)
	}
	return response.OK(c, nil, "Stages reordered")
}

// UpdateStage godoc
//
//	@Summary		Update a stage
//	@Tags			HRM / Recruitment
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string				true	"Organization ID"
//	@Param			pipelineId	path	string				true	"Pipeline public ID"
//	@Param			stageId		path	string				true	"Stage public ID"
//	@Param			body		body	UpdateStageRequest	true	"Fields to update"
//	@Success		200			{object}	response.OK{data=object{stage=Stage}}
//	@Router			/organizations/{orgId}/hrm/recruitment/pipelines/{pipelineId}/stages/{stageId} [patch]
func (h *Handler) UpdateStage(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateStageRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	st, err := h.service.UpdateStage(c.Context(), orgID, c.Params("pipelineId"), c.Params("stageId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"stage": st}, "Stage updated")
}

// DeleteStage godoc
//
//	@Summary		Delete a stage
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			pipelineId	path	string	true	"Pipeline public ID"
//	@Param			stageId		path	string	true	"Stage public ID"
//	@Success		204
//	@Router			/organizations/{orgId}/hrm/recruitment/pipelines/{pipelineId}/stages/{stageId} [delete]
func (h *Handler) DeleteStage(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.DeleteStage(c.Context(), orgID, c.Params("pipelineId"), c.Params("stageId")); err != nil {
		return h.err(c, err)
	}
	return response.NoContent(c)
}
