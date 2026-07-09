// backend/internal/hrm/milestones/handler.go
package milestones

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

type Handler struct{ service Service }
func NewHandler(service Service) *Handler { return &Handler{service: service} }

// List godoc
//
//	@Summary		List employee milestones
//	@Description	Filter by employee_id, milestone_type, upcoming=true (future unacknowledged only).
//	@Description
//	@Description	**Required permission:** `hrm.milestones.view`
//	@Tags			HRM / Milestones
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path		string	true	"Organization ID"
//	@Param			employee_id		query		string	false	"Filter by employee UUID"
//	@Param			milestone_type	query		string	false	"work_anniversary|birthday|probation_complete|..."
//	@Param			upcoming		query		bool	false	"Only future unacknowledged milestones"
//	@Success		200				{object}	response.OK{data=MilestoneListResponse}
//	@Router			/organizations/{orgId}/hrm/milestones [get]
func (h *Handler) List(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	upcoming, _ := strconv.ParseBool(c.Query("upcoming"))
	res, err := h.service.List(c.Context(), orgID, c.Query("employee_id"), c.Query("milestone_type"), upcoming)
	if err != nil { log.Error("milestones: List", slog.Any("error", err)); return response.InternalServerError(c) }
	return response.OK(c, res, "OK")
}

// Create godoc
//
//	@Summary		Create milestone
//	@Description	Creates a milestone manually. Set `create_award`, `create_announcement`,
//	@Description	`create_calendar_event` to trigger cascade creation of linked records.
//	@Description
//	@Description	**Required permission:** `hrm.milestones.manage`
//	@Tags			HRM / Milestones
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string					true	"Organization ID"
//	@Param			body	body		CreateMilestoneRequest	true	"Milestone details"
//	@Success		201		{object}	response.Created{data=object{milestone=Milestone}}
//	@Failure		409		{object}	response.Error	"DUPLICATE — already exists for this employee+type+date"
//	@Router			/organizations/{orgId}/hrm/milestones [post]
func (h *Handler) Create(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req CreateMilestoneRequest
	if err := c.Bind().JSON(&req); err != nil { return response.BadRequest(c, "INVALID_BODY", "Invalid request body") }
	m, err := h.service.Create(c.Context(), orgID, userID, req)
	if err != nil { return h.err(c, err) }
	return response.Created(c, fiber.Map{"milestone": m}, "Milestone created")
}

func (h *Handler) Get(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	m, err := h.service.Get(c.Context(), orgID, c.Params("milestoneId"))
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"milestone": m}, "OK")
}

// Acknowledge godoc
//
//	@Summary		Acknowledge milestone
//	@Description	HR or employee marks the milestone as acknowledged/celebrated.
//	@Tags			HRM / Milestones
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			milestoneId	path	string	true	"Milestone public ID (mil_*)"
//	@Success		200			{object}	response.OK{data=object{milestone=Milestone}}
//	@Failure		409			{object}	response.Error	"ALREADY_ACKNOWLEDGED"
//	@Router			/organizations/{orgId}/hrm/milestones/{milestoneId}/acknowledge [post]
func (h *Handler) Acknowledge(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	m, err := h.service.Acknowledge(c.Context(), orgID, c.Params("milestoneId"))
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"milestone": m}, "Milestone acknowledged")
}

// Generate godoc
//
//	@Summary		Bulk-generate upcoming milestones
//	@Description	Scans active employees for hire_date anniversaries, probation completions,
//	@Description	and contract renewals within the specified year+month.
//	@Description	Idempotent — already-existing milestones are skipped (not duplicated).
//	@Description
//	@Description	**Required permission:** `hrm.milestones.generate`
//	@Tags			HRM / Milestones
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string				true	"Organization ID"
//	@Param			body	body		GenerateRequest		true	"Generation parameters"
//	@Success		200		{object}	response.OK{data=GenerateResult}
//	@Router			/organizations/{orgId}/hrm/milestones/generate [post]
func (h *Handler) Generate(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req GenerateRequest
	if err := c.Bind().JSON(&req); err != nil { return response.BadRequest(c, "INVALID_BODY", "Invalid request body") }
	result, err := h.service.GenerateUpcoming(c.Context(), orgID, userID, req)
	if err != nil { return response.BadRequest(c, "GENERATE_FAILED", err.Error()) }
	return response.OK(c, result, fmt.Sprintf("Generated %d milestones", result.Generated))
}

func (h *Handler) err(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrNotFound): return response.NotFound(c, "MILESTONE_NOT_FOUND", "Milestone not found")
	case errors.Is(err, ErrEmployeeIDRequired): return response.BadRequest(c, "EMPLOYEE_ID_REQUIRED", "employee_id is required")
	case errors.Is(err, ErrTitleRequired): return response.BadRequest(c, "TITLE_REQUIRED", "title is required")
	case errors.Is(err, ErrInvalidMilestoneType): return response.BadRequest(c, "INVALID_MILESTONE_TYPE", "invalid milestone_type")
	case errors.Is(err, ErrDateRequired): return response.BadRequest(c, "DATE_REQUIRED", "milestone_date is required (YYYY-MM-DD)")
	case errors.Is(err, ErrInvalidDate): return response.BadRequest(c, "INVALID_DATE", "date must be a valid YYYY-MM-DD")
	case errors.Is(err, ErrAlreadyAcknowledged): return response.Conflict(c, "ALREADY_ACKNOWLEDGED", "Milestone has already been acknowledged")
	case errors.Is(err, ErrDuplicate): return response.Conflict(c, "DUPLICATE", "Milestone already exists for this employee, type, and date")
	default: log.Error("milestones: error", slog.Any("error", err)); return response.InternalServerError(c)
	}
}
