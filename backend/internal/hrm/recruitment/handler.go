// backend/internal/hrm/recruitment/handler.go
package recruitment

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles HRM recruitment HTTP endpoints. Deliberately just
// {service Service} — no authz.Service, no *scope.Resolver. This module
// does not call authz.Service.ResolveScope anywhere: candidates and
// applications are not hrm_employees rows and have no employee_id for the
// Phase 1 scope tiers to resolve against. See migration 00079's header for
// the full reasoning. Route-level RBAC (permFn in routes.go) is the only
// authorization layer here.
type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// ============================================================
// Requisitions
// ============================================================

// ListRequisitions godoc
//
//	@Summary		List job requisitions
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			status	query	string	false	"Filter by status"
//	@Success		200		{object}	response.OK{data=RequisitionListResponse}
//	@Router			/organizations/{orgId}/hrm/recruitment/requisitions [get]
func (h *Handler) ListRequisitions(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	filter := RequisitionListFilter{Status: c.Query("status")}
	res, err := h.service.ListRequisitions(c.Context(), orgID, filter)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "OK")
}

// GetRequisition godoc
//
//	@Summary		Get job requisition
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path	string	true	"Organization ID"
//	@Param			requisitionId	path	string	true	"Requisition public ID"
//	@Success		200				{object}	response.OK{data=object{requisition=Requisition}}
//	@Router			/organizations/{orgId}/hrm/recruitment/requisitions/{requisitionId} [get]
func (h *Handler) GetRequisition(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	r, err := h.service.GetRequisition(c.Context(), orgID, c.Params("requisitionId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"requisition": r}, "OK")
}

// CreateRequisition godoc
//
//	@Summary		Create job requisition
//	@Tags			HRM / Recruitment
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string						true	"Organization ID"
//	@Param			body	body	CreateRequisitionRequest	true	"Requisition"
//	@Success		201		{object}	response.Created{data=object{requisition=Requisition}}
//	@Router			/organizations/{orgId}/hrm/recruitment/requisitions [post]
func (h *Handler) CreateRequisition(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateRequisitionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	r, err := h.service.CreateRequisition(c.Context(), orgID, userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"requisition": r}, "Requisition created")
}

// UpdateRequisition godoc
//
//	@Summary		Update job requisition (draft only)
//	@Tags			HRM / Recruitment
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path	string						true	"Organization ID"
//	@Param			requisitionId	path	string						true	"Requisition public ID"
//	@Param			body			body	UpdateRequisitionRequest	true	"Fields to update"
//	@Success		200				{object}	response.OK{data=object{requisition=Requisition}}
//	@Failure		409				{object}	response.Error	"WRONG_STATUS"
//	@Router			/organizations/{orgId}/hrm/recruitment/requisitions/{requisitionId} [patch]
func (h *Handler) UpdateRequisition(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateRequisitionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	r, err := h.service.UpdateRequisition(c.Context(), orgID, c.Params("requisitionId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"requisition": r}, "Requisition updated")
}

// SubmitRequisition godoc
//
//	@Summary		Submit requisition for approval
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path	string	true	"Organization ID"
//	@Param			requisitionId	path	string	true	"Requisition public ID"
//	@Success		200				{object}	response.OK{data=object{requisition=Requisition}}
//	@Router			/organizations/{orgId}/hrm/recruitment/requisitions/{requisitionId}/submit [post]
func (h *Handler) SubmitRequisition(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	r, err := h.service.SubmitRequisition(c.Context(), orgID, c.Params("requisitionId"), userID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"requisition": r}, "Requisition submitted")
}

// CloseRequisition godoc
//
//	@Summary		Close job requisition
//	@Tags			HRM / Recruitment
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path	string						true	"Organization ID"
//	@Param			requisitionId	path	string						true	"Requisition public ID"
//	@Param			body			body	CloseRequisitionRequest	true	"Reason"
//	@Success		200				{object}	response.OK{data=object{requisition=Requisition}}
//	@Router			/organizations/{orgId}/hrm/recruitment/requisitions/{requisitionId}/close [post]
func (h *Handler) CloseRequisition(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CloseRequisitionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	r, err := h.service.CloseRequisition(c.Context(), orgID, c.Params("requisitionId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"requisition": r}, "Requisition closed")
}

// ============================================================
// Postings
// ============================================================

// ListPostings godoc
//
//	@Summary		List job postings
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path	string	true	"Organization ID"
//	@Param			status			query	string	false	"Filter by status"
//	@Param			requisition_id	query	string	false	"Filter by requisition"
//	@Success		200				{object}	response.OK{data=PostingListResponse}
//	@Router			/organizations/{orgId}/hrm/recruitment/postings [get]
func (h *Handler) ListPostings(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	filter := PostingListFilter{Status: c.Query("status"), RequisitionID: c.Query("requisition_id")}
	res, err := h.service.ListPostings(c.Context(), orgID, filter)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "OK")
}

// GetPosting godoc
//
//	@Summary		Get job posting
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			postingId	path	string	true	"Posting public ID"
//	@Success		200			{object}	response.OK{data=object{posting=Posting}}
//	@Router			/organizations/{orgId}/hrm/recruitment/postings/{postingId} [get]
func (h *Handler) GetPosting(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	p, err := h.service.GetPosting(c.Context(), orgID, c.Params("postingId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"posting": p}, "OK")
}

// CreatePosting godoc
//
//	@Summary		Create job posting
//	@Description	If pipeline_id is omitted, the org's default pipeline is used.
//	@Description	public_slug is written now even though there is no public route
//	@Description	to read it in this phase — see migration 00078's design notes.
//	@Tags			HRM / Recruitment
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string					true	"Organization ID"
//	@Param			body	body	CreatePostingRequest	true	"Posting"
//	@Success		201		{object}	response.Created{data=object{posting=Posting}}
//	@Failure		409		{object}	response.Error	"SLUG_TAKEN"
//	@Router			/organizations/{orgId}/hrm/recruitment/postings [post]
func (h *Handler) CreatePosting(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreatePostingRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	p, err := h.service.CreatePosting(c.Context(), orgID, userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"posting": p}, "Posting created")
}

// UpdatePosting godoc
//
//	@Summary		Update job posting
//	@Tags			HRM / Recruitment
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string					true	"Organization ID"
//	@Param			postingId	path	string					true	"Posting public ID"
//	@Param			body		body	UpdatePostingRequest	true	"Fields to update"
//	@Success		200			{object}	response.OK{data=object{posting=Posting}}
//	@Router			/organizations/{orgId}/hrm/recruitment/postings/{postingId} [patch]
func (h *Handler) UpdatePosting(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdatePostingRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	p, err := h.service.UpdatePosting(c.Context(), orgID, c.Params("postingId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"posting": p}, "Posting updated")
}

// DeletePosting godoc
//
//	@Summary		Delete job posting
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			postingId	path	string	true	"Posting public ID"
//	@Success		204
//	@Router			/organizations/{orgId}/hrm/recruitment/postings/{postingId} [delete]
func (h *Handler) DeletePosting(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.DeletePosting(c.Context(), orgID, c.Params("postingId")); err != nil {
		return h.err(c, err)
	}
	return response.NoContent(c)
}

// PublishPosting godoc
//
//	@Summary		Publish job posting
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			postingId	path	string	true	"Posting public ID"
//	@Success		200			{object}	response.OK{data=object{posting=Posting}}
//	@Router			/organizations/{orgId}/hrm/recruitment/postings/{postingId}/publish [post]
func (h *Handler) PublishPosting(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	p, err := h.service.PublishPosting(c.Context(), orgID, c.Params("postingId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"posting": p}, "Posting published")
}

// ClosePosting godoc
//
//	@Summary		Close job posting
//	@Tags			HRM / Recruitment
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			postingId	path	string	true	"Posting public ID"
//	@Success		200			{object}	response.OK{data=object{posting=Posting}}
//	@Router			/organizations/{orgId}/hrm/recruitment/postings/{postingId}/close [post]
func (h *Handler) ClosePosting(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	p, err := h.service.ClosePosting(c.Context(), orgID, c.Params("postingId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"posting": p}, "Posting closed")
}

// ============================================================
// Shared error mapper
// ============================================================

func (h *Handler) err(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrRequisitionNotFound):
		return response.NotFound(c, "REQUISITION_NOT_FOUND", "Job requisition not found")
	case errors.Is(err, ErrPostingNotFound):
		return response.NotFound(c, "POSTING_NOT_FOUND", "Job posting not found")
	case errors.Is(err, ErrPipelineNotFound):
		return response.NotFound(c, "PIPELINE_NOT_FOUND", "Recruitment pipeline not found")
	case errors.Is(err, ErrStageNotFound):
		return response.NotFound(c, "STAGE_NOT_FOUND", "Recruitment stage not found")
	case errors.Is(err, ErrCandidateNotFound):
		return response.NotFound(c, "CANDIDATE_NOT_FOUND", "Candidate not found")
	case errors.Is(err, ErrApplicationNotFound):
		return response.NotFound(c, "APPLICATION_NOT_FOUND", "Application not found")
	case errors.Is(err, ErrNoResumeOnFile):
		return response.NotFound(c, "NO_RESUME_ON_FILE", "No resume on file for this candidate")

	case errors.Is(err, ErrTitleRequired):
		return response.BadRequest(c, "TITLE_REQUIRED", "title is required")
	case errors.Is(err, ErrFirstNameRequired):
		return response.BadRequest(c, "FIRST_NAME_REQUIRED", "first_name is required")
	case errors.Is(err, ErrInvalidEmploymentType):
		return response.BadRequest(c, "INVALID_EMPLOYMENT_TYPE", "employment_type must be one of: full_time, part_time, contractor, intern")
	case errors.Is(err, ErrInvalidSalaryRange):
		return response.BadRequest(c, "INVALID_SALARY_RANGE", "salary_max must be greater than or equal to salary_min")
	case errors.Is(err, ErrPipelineNameReq):
		return response.BadRequest(c, "NAME_REQUIRED", "name is required")
	case errors.Is(err, ErrStageNameReq):
		return response.BadRequest(c, "NAME_REQUIRED", "name is required")
	case errors.Is(err, ErrInvalidStageKind):
		return response.BadRequest(c, "INVALID_STAGE_KIND", "stage_kind must be one of: applied, in_progress, offer, hired, rejected")
	case errors.Is(err, ErrStageNotInPipeline):
		return response.Conflict(c, "STAGE_NOT_IN_PIPELINE", "Stage does not belong to this application's pipeline")
	case errors.Is(err, ErrSlugRequired):
		return response.BadRequest(c, "SLUG_REQUIRED", "public_slug is required")
	case errors.Is(err, ErrSlugTaken):
		return response.Conflict(c, "SLUG_TAKEN", "public_slug is already in use for this organization")
	case errors.Is(err, ErrRequisitionRequired):
		return response.BadRequest(c, "REQUISITION_REQUIRED", "requisition_id is required")
	case errors.Is(err, ErrPipelineRequired):
		return response.BadRequest(c, "PIPELINE_REQUIRED", "pipeline_id is required (no default pipeline configured for this org)")
	case errors.Is(err, ErrInvalidCandidateSource):
		return response.BadRequest(c, "INVALID_SOURCE", "invalid source value")
	case errors.Is(err, ErrInvalidResumeType):
		return response.BadRequest(c, "INVALID_RESUME_TYPE", "Only PDF resumes are accepted")
	case errors.Is(err, ErrResumeTooLarge):
		return response.BadRequest(c, "RESUME_TOO_LARGE", "Resume file exceeds the size limit")
	case errors.Is(err, ErrCandidateIDRequired):
		return response.BadRequest(c, "CANDIDATE_ID_REQUIRED", "candidate_id is required")
	case errors.Is(err, ErrPostingIDRequired):
		return response.BadRequest(c, "POSTING_ID_REQUIRED", "posting_id is required")
	case errors.Is(err, ErrRejectReasonRequired):
		return response.BadRequest(c, "REJECT_REASON_REQUIRED", "reason is required to reject an application")

	case errors.Is(err, ErrWrongStatus):
		return response.Conflict(c, "WRONG_STATUS", "Action not allowed in current status")
	case errors.Is(err, ErrApplicationNotActive):
		return response.Conflict(c, "APPLICATION_NOT_ACTIVE", "Action not allowed — application is not active")
	case errors.Is(err, ErrCandidateEmailExists):
		return response.Conflict(c, "CANDIDATE_EMAIL_EXISTS", "A candidate with this email already exists in this organization")
	case errors.Is(err, ErrDuplicateApplication):
		return response.Conflict(c, "DUPLICATE_APPLICATION", "This candidate already has an active application for this posting")

	default:
		log.Error("recruitment: error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}
