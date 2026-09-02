// backend/internal/hrm/shifts/handler.go
package shifts

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

type Handler struct{ service Service }

func NewHandler(service Service) *Handler { return &Handler{service: service} }

// ListShifts godoc
//
//	@Summary		List shifts
//	@Description	Returns all shift definitions for the organization.
//	@Description
//	@Description	**Required permission:** `hrm.shifts.view`
//	@Tags			HRM / Shifts
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string	true	"Organization ID"
//	@Param			active	query		bool	false	"When true, return only active shifts"
//	@Success		200		{object}	response.OK{data=ShiftListResponse}
//	@Failure		401		{object}	response.Error
//	@Failure		403		{object}	response.Error
//	@Failure		500		{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/shifts [get]
func (h *Handler) ListShifts(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	activeOnly := strings.ToLower(c.Query("active")) == "true"
	res, err := h.service.List(c.Context(), orgID, activeOnly)
	if err != nil {
		log.Error("shifts: List", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, res, "OK")
}

// CreateShift godoc
//
//	@Summary		Create shift
//	@Description	Creates a new shift definition.
//	@Description
//	@Description	- `shift_type: fixed` requires `start_time` and `end_time`
//	@Description	- `shift_type: flexible` requires `weekly_hours_target`
//	@Description	- `working_days` defaults to `[mon,tue,wed,thu,fri]` if omitted
//	@Description
//	@Description	**Required permission:** `hrm.shifts.manage`
//	@Tags			HRM / Shifts
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string				true	"Organization ID"
//	@Param			body	body		CreateShiftRequest	true	"Shift definition"
//	@Success		201		{object}	response.Created{data=object{shift=Shift}}
//	@Failure		400		{object}	response.Error
//	@Failure		401		{object}	response.Error
//	@Failure		403		{object}	response.Error
//	@Failure		409		{object}	response.Error
//	@Failure		500		{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/shifts [post]
func (h *Handler) CreateShift(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateShiftRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	sh, err := h.service.Create(c.Context(), orgID, userID, req)
	if err != nil {
		return h.shiftError(c, err)
	}
	return response.Created(c, fiber.Map{"shift": sh}, "Shift created")
}

// GetShift godoc
//
//	@Summary		Get shift
//	@Description	Returns a single shift by its public ID.
//	@Description
//	@Description	**Required permission:** `hrm.shifts.view`
//	@Tags			HRM / Shifts
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string	true	"Organization ID"
//	@Param			shiftId	path		string	true	"Shift public ID (sh_*)"
//	@Success		200		{object}	response.OK{data=object{shift=Shift}}
//	@Failure		401		{object}	response.Error
//	@Failure		403		{object}	response.Error
//	@Failure		404		{object}	response.Error	"SHIFT_NOT_FOUND"
//	@Failure		500		{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/shifts/{shiftId} [get]
func (h *Handler) GetShift(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	sh, err := h.service.Get(c.Context(), orgID, c.Params("shiftId"))
	if err != nil {
		return h.shiftError(c, err)
	}
	return response.OK(c, fiber.Map{"shift": sh}, "OK")
}

// UpdateShift godoc
//
//	@Summary		Update shift
//	@Description	Partially updates a shift definition.
//	@Description
//	@Description	**Required permission:** `hrm.shifts.manage`
//	@Tags			HRM / Shifts
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string				true	"Organization ID"
//	@Param			shiftId	path		string				true	"Shift public ID (sh_*)"
//	@Param			body	body		UpdateShiftRequest	true	"Fields to update"
//	@Success		200		{object}	response.OK{data=object{shift=Shift}}
//	@Failure		400		{object}	response.Error
//	@Failure		401		{object}	response.Error
//	@Failure		403		{object}	response.Error
//	@Failure		404		{object}	response.Error
//	@Failure		409		{object}	response.Error
//	@Failure		500		{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/shifts/{shiftId} [patch]
func (h *Handler) UpdateShift(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateShiftRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	sh, err := h.service.Update(c.Context(), orgID, c.Params("shiftId"), req)
	if err != nil {
		return h.shiftError(c, err)
	}
	return response.OK(c, fiber.Map{"shift": sh}, "Shift updated")
}

// DeleteShift godoc
//
//	@Summary		Delete shift
//	@Description	Permanently deletes a shift.
//	@Description
//	@Description	**Required permission:** `hrm.shifts.manage`
//	@Tags			HRM / Shifts
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			shiftId	path	string	true	"Shift public ID (sh_*)"
//	@Success		204		"Shift deleted"
//	@Failure		401		{object}	response.Error
//	@Failure		403		{object}	response.Error
//	@Failure		404		{object}	response.Error
//	@Failure		500		{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/shifts/{shiftId} [delete]
func (h *Handler) DeleteShift(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.Delete(c.Context(), orgID, c.Params("shiftId")); err != nil {
		return h.shiftError(c, err)
	}
	return response.NoContent(c)
}

// ListAssignments godoc
//
//	@Summary		List shift assignments
//	@Description	Returns shift assignments. Filter by assignee_type and assignee_id to narrow results.
//	@Description
//	@Description	**Required permission:** `hrm.shifts.view`
//	@Tags			HRM / Shifts
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path		string	true	"Organization ID"
//	@Param			assignee_type	query		string	false	"organization | department | employee"
//	@Param			assignee_id		query		string	false	"UUID of the assignee"
//	@Success		200				{object}	response.OK{data=AssignmentListResponse}
//	@Failure		401				{object}	response.Error
//	@Failure		403				{object}	response.Error
//	@Failure		500				{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/shifts/assignments [get]
func (h *Handler) ListAssignments(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	res, err := h.service.ListAssignments(c.Context(), orgID, c.Query("assignee_type"), c.Query("assignee_id"))
	if err != nil {
		log.Error("shifts: ListAssignments", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, res, "OK")
}

// Assign godoc
//
//	@Summary		Assign shift
//	@Description	Assigns a shift to an organization, department, or employee.
//	@Description	Lookup priority: employee > department > organization.
//	@Description
//	@Description	**Required permission:** `hrm.shifts.manage`
//	@Tags			HRM / Shifts
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path		string				true	"Organization ID"
//	@Param			body	body		AssignShiftRequest	true	"Assignment details"
//	@Success		201		{object}	response.Created{data=object{assignment=WorkScheduleAssignment}}
//	@Failure		400		{object}	response.Error
//	@Failure		401		{object}	response.Error
//	@Failure		403		{object}	response.Error
//	@Failure		404		{object}	response.Error	"SHIFT_NOT_FOUND"
//	@Failure		500		{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/shifts/assignments [post]
func (h *Handler) Assign(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req AssignShiftRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	a, err := h.service.Assign(c.Context(), orgID, userID, req)
	if err != nil {
		return h.shiftError(c, err)
	}
	return response.Created(c, fiber.Map{"assignment": a}, "Shift assigned")
}

// RemoveAssignment godoc
//
//	@Summary		Remove shift assignment
//	@Description	Deletes a work schedule assignment.
//	@Description
//	@Description	**Required permission:** `hrm.shifts.manage`
//	@Tags			HRM / Shifts
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path	string	true	"Organization ID"
//	@Param			assignmentId	path	string	true	"Assignment public ID (wsa_*)"
//	@Success		204				"Assignment removed"
//	@Failure		401				{object}	response.Error
//	@Failure		403				{object}	response.Error
//	@Failure		404				{object}	response.Error
//	@Failure		500				{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/setup/shifts/assignments/{assignmentId} [delete]
func (h *Handler) RemoveAssignment(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.RemoveAssignment(c.Context(), orgID, c.Params("assignmentId")); err != nil {
		return h.shiftError(c, err)
	}
	return response.NoContent(c)
}

func (h *Handler) shiftError(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrShiftNotFound):
		return response.NotFound(c, "SHIFT_NOT_FOUND", "Shift not found")
	case errors.Is(err, ErrAssignmentNotFound):
		return response.NotFound(c, "ASSIGNMENT_NOT_FOUND", "Assignment not found")
	case errors.Is(err, ErrNameRequired):
		return response.BadRequest(c, "NAME_REQUIRED", "Name is required")
	case errors.Is(err, ErrNameConflict):
		return response.Conflict(c, "SHIFT_NAME_CONFLICT", "A shift with this name already exists")
	case errors.Is(err, ErrInvalidShiftType):
		return response.BadRequest(c, "INVALID_SHIFT_TYPE", "shift_type must be: fixed or flexible")
	case errors.Is(err, ErrFixedTimeRequired):
		return response.BadRequest(c, "FIXED_TIME_REQUIRED", "start_time and end_time are required for fixed shifts")
	case errors.Is(err, ErrFlexHoursRequired):
		return response.BadRequest(c, "FLEX_HOURS_REQUIRED", "weekly_hours_target is required for flexible shifts")
	case errors.Is(err, ErrInvalidAssigneeType):
		return response.BadRequest(c, "INVALID_ASSIGNEE_TYPE", "assignee_type must be: organization, department, or employee")
	case errors.Is(err, ErrAssigneeIDRequired):
		return response.BadRequest(c, "ASSIGNEE_ID_REQUIRED", "assignee_id is required")
	case errors.Is(err, ErrEffectiveDateRequired):
		return response.BadRequest(c, "EFFECTIVE_DATE_REQUIRED", "effective_date must be a valid YYYY-MM-DD date")
	default:
		log.Error("shifts: error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}
