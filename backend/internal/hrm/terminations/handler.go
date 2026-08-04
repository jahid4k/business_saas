// backend/internal/hrm/terminations/handler.go
package terminations

import (
	"errors"
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/scope"
	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles HRM termination HTTP endpoints.
// Terminations are always HR-initiated — employees use /resignations.
type Handler struct {
	service       Service
	authz         authz.Service
	scopeResolver *scope.Resolver
}

func NewHandler(service Service, authzSvc authz.Service, scopeResolver *scope.Resolver) *Handler {
	return &Handler{service: service, authz: authzSvc, scopeResolver: scopeResolver}
}

// ListAll godoc
//
//	@Summary		List all terminations (HR view)
//	@Description	Returns all termination records in the organization.
//	@Description	Terminations are always HR-initiated — use /resignations for employee-initiated departures.
//	@Description
//	@Description	**Required permission:** `hrm.terminations.view`
//	@Tags			HRM / Terminations
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			status		query		string	false	"Filter by status (draft|pending_approval|approved|cancelled|applied)"
//	@Param			employee_id	query		string	false	"Filter by employee UUID"
//	@Success		200			{object}	response.OK{data=TerminationListResponse}
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/terminations [get]
func (h *Handler) ListAll(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.terminations")
	if err != nil { log.Error("terminations: ListAll", slog.Any("error", err)); return response.InternalServerError(c) }
	filter := TerminationListFilter{
		EmployeeID:   c.Query("employee_id"),
		Status:       c.Query("status"),
		Scope:        scopeTier,
		CallerUserID: userID,
	}
	if limit, err := strconv.Atoi(c.Query("limit", "")); err == nil { filter.Limit = limit }
	if offset, err := strconv.Atoi(c.Query("offset", "")); err == nil { filter.Offset = offset }
	res, err := h.service.List(c.Context(), orgID, filter)
	if err != nil { log.Error("terminations: ListAll", slog.Any("error", err)); return response.InternalServerError(c) }
	return response.OK(c, res, "OK")
}

// Create godoc
//
//	@Summary		Create termination record
//	@Description	HR creates a termination record in draft status.
//	@Description
//	@Description	**Note:** `is_rehire_eligible` defaults to true. Set false for gross misconduct.
//	@Description	`severance_amount` is informational — disbursed via payroll/accounting.
//	@Description
//	@Description	**Required permission:** `hrm.terminations.manage`
//	@Description
//	@Description	**Error codes:**
//	@Description	- `INVALID_TERMINATION_TYPE`
//	@Description	- `TERMINATION_DATE_REQUIRED` / `LAST_WORKING_DATE_REQUIRED`
//	@Description	- `ALREADY_ACTIVE_TERMINATION` — cancel existing record first
//	@Tags			HRM / Terminations
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string						true	"Organization ID"
//	@Param			employeeId	path		string						true	"Employee public ID"
//	@Param			body		body		CreateTerminationRequest	true	"Termination details"
//	@Success		201			{object}	response.Created{data=object{termination=Termination}}
//	@Failure		400			{object}	response.Error
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		409			{object}	response.Error	"ALREADY_ACTIVE_TERMINATION"
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/terminations [post]
func (h *Handler) Create(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req CreateTerminationRequest
	if err := c.Bind().JSON(&req); err != nil { return response.BadRequest(c, "INVALID_BODY", "Invalid request body") }
	t, err := h.service.Create(c.Context(), orgID, c.Params("employeeId"), userID, req)
	if err != nil { return h.err(c, err) }
	return response.Created(c, fiber.Map{"termination": t}, "Termination record created")
}

// Get godoc
//
//	@Summary		Get termination record
//	@Description	Returns a single termination record by its public ID.
//	@Description
//	@Description	**Required permission:** `hrm.terminations.view`
//	@Tags			HRM / Terminations
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path		string	true	"Organization ID"
//	@Param			employeeId		path		string	true	"Employee public ID"
//	@Param			terminationId	path		string	true	"Termination public ID (term_*)"
//	@Success		200				{object}	response.OK{data=object{termination=Termination}}
//	@Failure		401				{object}	response.Error
//	@Failure		403				{object}	response.Error
//	@Failure		404				{object}	response.Error	"TERMINATION_NOT_FOUND"
//	@Failure		500				{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/terminations/{terminationId} [get]
func (h *Handler) Get(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	employeeID := c.Params("employeeId")
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.terminations")
	if err != nil { log.Error("terminations: Get", slog.Any("error", err)); return response.InternalServerError(c) }
	allowed, err := h.scopeResolver.AuthorizeRecordAccess(c.Context(), scopeTier, orgID, userID, employeeID)
	if err != nil { log.Error("terminations: Get", slog.Any("error", err)); return response.InternalServerError(c) }
	if !allowed { return response.Forbidden(c, "RECORD_ACCESS_DENIED", "You do not have access to this record") }
	t, err := h.service.Get(c.Context(), orgID, employeeID, c.Params("terminationId"))
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"termination": t}, "OK")
}

// Update godoc
//
//	@Summary		Update termination record
//	@Description	Partially updates a termination record. Not allowed once applied/cancelled/rejected.
//	@Description
//	@Description	**Required permission:** `hrm.terminations.manage`
//	@Tags			HRM / Terminations
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path		string					true	"Organization ID"
//	@Param			employeeId		path		string					true	"Employee public ID"
//	@Param			terminationId	path		string					true	"Termination public ID"
//	@Param			body			body		UpdateTerminationRequest	true	"Fields to update"
//	@Success		200				{object}	response.OK{data=object{termination=Termination}}
//	@Failure		400				{object}	response.Error
//	@Failure		401				{object}	response.Error
//	@Failure		403				{object}	response.Error
//	@Failure		404				{object}	response.Error
//	@Failure		409				{object}	response.Error	"WRONG_STATUS"
//	@Failure		500				{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/terminations/{terminationId} [patch]
func (h *Handler) Update(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req UpdateTerminationRequest
	if err := c.Bind().JSON(&req); err != nil { return response.BadRequest(c, "INVALID_BODY", "Invalid request body") }
	t, err := h.service.Update(c.Context(), orgID, c.Params("employeeId"), c.Params("terminationId"), req)
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"termination": t}, "Termination updated")
}

// Submit godoc
//
//	@Summary		Submit termination for approval
//	@Description	Moves draft → approved (or pending_approval if approval chain configured).
//	@Description
//	@Description	**Required permission:** `hrm.terminations.manage`
//	@Tags			HRM / Terminations
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path	string	true	"Organization ID"
//	@Param			employeeId		path	string	true	"Employee public ID"
//	@Param			terminationId	path	string	true	"Termination public ID"
//	@Success		200				{object}	response.OK{data=object{termination=Termination}}
//	@Failure		401				{object}	response.Error
//	@Failure		403				{object}	response.Error
//	@Failure		404				{object}	response.Error
//	@Failure		409				{object}	response.Error	"WRONG_STATUS"
//	@Failure		500				{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/terminations/{terminationId}/submit [post]
func (h *Handler) Submit(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	t, err := h.service.Submit(c.Context(), orgID, c.Params("employeeId"), c.Params("terminationId"), userID)
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"termination": t}, "Termination submitted")
}

// Cancel godoc
//
//	@Summary		Cancel termination
//	@Description	Cancels a termination that has not yet been applied.
//	@Description
//	@Description	**Required permission:** `hrm.terminations.manage`
//	@Tags			HRM / Terminations
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path	string	true	"Organization ID"
//	@Param			employeeId		path	string	true	"Employee public ID"
//	@Param			terminationId	path	string	true	"Termination public ID"
//	@Success		200				{object}	response.OK{data=object{termination=Termination}}
//	@Failure		401				{object}	response.Error
//	@Failure		403				{object}	response.Error
//	@Failure		404				{object}	response.Error
//	@Failure		409				{object}	response.Error
//	@Failure		500				{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/terminations/{terminationId}/cancel [post]
func (h *Handler) Cancel(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	t, err := h.service.Cancel(c.Context(), orgID, c.Params("employeeId"), c.Params("terminationId"))
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"termination": t}, "Termination cancelled")
}

// Apply godoc
//
//	@Summary		Apply termination to employee record
//	@Description	Executes the termination atomically:
//	@Description	- Sets `employee.status = 'terminated'`
//	@Description	- Sets `employee.termination_date = last_working_date`
//	@Description
//	@Description	**This operation is irreversible.** Verify all details before applying.
//	@Description	**Prerequisites:** status must be `approved`.
//	@Description	**Required permission:** `hrm.terminations.apply`
//	@Tags			HRM / Terminations
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path	string	true	"Organization ID"
//	@Param			employeeId		path	string	true	"Employee public ID"
//	@Param			terminationId	path	string	true	"Termination public ID (term_*)"
//	@Success		200				{object}	response.OK{data=object{termination=Termination}}
//	@Failure		401				{object}	response.Error
//	@Failure		403				{object}	response.Error
//	@Failure		404				{object}	response.Error
//	@Failure		409				{object}	response.Error	"ALREADY_APPLIED or NOT_APPROVED"
//	@Failure		500				{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/terminations/{terminationId}/apply [post]
func (h *Handler) Apply(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	t, err := h.service.Apply(c.Context(), orgID, c.Params("employeeId"), c.Params("terminationId"), userID)
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"termination": t}, "Termination applied — employee is now terminated")
}

func (h *Handler) err(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrNotFound):
		return response.NotFound(c, "TERMINATION_NOT_FOUND", "Termination record not found")
	case errors.Is(err, ErrAlreadyActiveTermination):
		return response.Conflict(c, "ALREADY_ACTIVE_TERMINATION", "Employee already has an active termination record — cancel it first")
	case errors.Is(err, ErrInvalidTerminationType):
		return response.BadRequest(c, "INVALID_TERMINATION_TYPE",
			"termination_type must be: voluntary, involuntary, layoff, retirement, contract_end, or probation_fail")
	case errors.Is(err, ErrTerminationDateRequired):
		return response.BadRequest(c, "TERMINATION_DATE_REQUIRED", "termination_date is required (YYYY-MM-DD)")
	case errors.Is(err, ErrLastWorkingDateRequired):
		return response.BadRequest(c, "LAST_WORKING_DATE_REQUIRED", "last_working_date is required (YYYY-MM-DD)")
	case errors.Is(err, ErrInvalidDate):
		return response.BadRequest(c, "INVALID_DATE", "date must be a valid YYYY-MM-DD")
	case errors.Is(err, ErrWrongStatus):
		return response.Conflict(c, "WRONG_STATUS", "Action not allowed in current termination status")
	case errors.Is(err, ErrAlreadyApplied):
		return response.Conflict(c, "ALREADY_APPLIED", "Termination has already been applied")
	case errors.Is(err, ErrNotApproved):
		return response.Conflict(c, "NOT_APPROVED", "Termination must be approved before applying")
	default:
		log.Error("terminations: error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}
