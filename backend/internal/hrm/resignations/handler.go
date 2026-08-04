// backend/internal/hrm/resignations/handler.go
package resignations

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

type Handler struct {
	service       Service
	authz         authz.Service
	scopeResolver *scope.Resolver
}

func NewHandler(service Service, authzSvc authz.Service, scopeResolver *scope.Resolver) *Handler {
	return &Handler{service: service, authz: authzSvc, scopeResolver: scopeResolver}
}

// resolveListFilter builds the shared parts of a ResignationListFilter
// (scope, pagination, status) once userID is already known — ListAll and
// ListForEmployee each add their own employee_id source (query param vs path
// param) on top. err is a plain ResolveScope failure, never a written
// response — callers log and 500 it themselves.
func (h *Handler) resolveListFilter(c fiber.Ctx, orgID, userID string) (ResignationListFilter, error) {
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.resignations")
	if err != nil {
		return ResignationListFilter{}, err
	}
	filter := ResignationListFilter{
		Status:       c.Query("status"),
		Scope:        scopeTier,
		CallerUserID: userID,
	}
	if limit, err := strconv.Atoi(c.Query("limit", "")); err == nil {
		filter.Limit = limit
	}
	if offset, err := strconv.Atoi(c.Query("offset", "")); err == nil {
		filter.Offset = offset
	}
	return filter, nil
}

// ListAll godoc
//
//	@Summary		List all resignations (HR view)
//	@Description	Returns all resignation records in the organization.
//	@Description
//	@Description	**Required permission:** `hrm.resignations.view`
//	@Tags			HRM / Resignations
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			status		query		string	false	"submitted | accepted | withdrawn | rejected"
//	@Param			employee_id	query		string	false	"Filter by employee UUID"
//	@Success		200			{object}	response.OK{data=ResignationListResponse}
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/resignations [get]
func (h *Handler) ListAll(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	filter, err := h.resolveListFilter(c, orgID, userID)
	if err != nil { log.Error("resignations: ListAll", slog.Any("error", err)); return response.InternalServerError(c) }
	filter.EmployeeID = c.Query("employee_id")
	res, err := h.service.List(c.Context(), orgID, filter)
	if err != nil { log.Error("resignations: ListAll", slog.Any("error", err)); return response.InternalServerError(c) }
	return response.OK(c, res, "OK")
}

// ListForEmployee godoc
//
//	@Summary		List employee resignations
//	@Tags			HRM / Resignations
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			employeeId	path		string	true	"Employee public ID"
//	@Success		200			{object}	response.OK{data=ResignationListResponse}
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/resignations [get]
func (h *Handler) ListForEmployee(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	filter, err := h.resolveListFilter(c, orgID, userID)
	if err != nil { log.Error("resignations: ListForEmployee", slog.Any("error", err)); return response.InternalServerError(c) }
	filter.EmployeeID = c.Params("employeeId")
	res, err := h.service.List(c.Context(), orgID, filter)
	if err != nil { log.Error("resignations: ListForEmployee", slog.Any("error", err)); return response.InternalServerError(c) }
	return response.OK(c, res, "OK")
}

// Submit godoc
//
//	@Summary		Submit resignation
//	@Description	Employee submits a resignation. Notice period is auto-computed from the active
//	@Description	contract. Supply `last_working_date` to override.
//	@Description
//	@Description	**Required permission:** `hrm.resignations.manage`
//	@Description
//	@Description	**Error codes:** `RESIGNATION_DATE_REQUIRED` · `INVALID_DATE` ·
//	@Description	`ALREADY_ACTIVE` (existing pending resignation exists)
//	@Tags			HRM / Resignations
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string						true	"Organization ID"
//	@Param			employeeId	path		string						true	"Employee public ID"
//	@Param			body		body		SubmitResignationRequest	true	"Resignation details"
//	@Success		201			{object}	response.Created{data=object{resignation=Resignation}}
//	@Failure		400			{object}	response.Error
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		409			{object}	response.Error	"ALREADY_ACTIVE"
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/resignations [post]
func (h *Handler) Submit(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req SubmitResignationRequest
	if err := c.Bind().JSON(&req); err != nil { return response.BadRequest(c, "INVALID_BODY", "Invalid request body") }
	r, err := h.service.Submit(c.Context(), orgID, c.Params("employeeId"), userID, req)
	if err != nil { return h.err(c, err) }
	return response.Created(c, fiber.Map{"resignation": r}, "Resignation submitted")
}

// Get godoc
//
//	@Summary		Get resignation record
//	@Tags			HRM / Resignations
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path		string	true	"Organization ID"
//	@Param			employeeId		path		string	true	"Employee public ID"
//	@Param			resignationId	path		string	true	"Resignation public ID (res_*)"
//	@Success		200				{object}	response.OK{data=object{resignation=Resignation}}
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/resignations/{resignationId} [get]
func (h *Handler) Get(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	employeeID := c.Params("employeeId")
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.resignations")
	if err != nil { log.Error("resignations: Get", slog.Any("error", err)); return response.InternalServerError(c) }
	allowed, err := h.scopeResolver.AuthorizeRecordAccess(c.Context(), scopeTier, orgID, userID, employeeID)
	if err != nil { log.Error("resignations: Get", slog.Any("error", err)); return response.InternalServerError(c) }
	if !allowed { return response.Forbidden(c, "RECORD_ACCESS_DENIED", "You do not have access to this record") }
	r, err := h.service.Get(c.Context(), orgID, employeeID, c.Params("resignationId"))
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"resignation": r}, "OK")
}

// Update godoc
//
//	@Summary		Update resignation record (HR action)
//	@Description	HR updates exit clearance tracking and optionally overrides last_working_date.
//	@Tags			HRM / Resignations
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path		string					true	"Organization ID"
//	@Param			employeeId		path		string					true	"Employee public ID"
//	@Param			resignationId	path		string					true	"Resignation public ID"
//	@Param			body			body		UpdateResignationRequest	true	"Fields to update"
//	@Success		200				{object}	response.OK{data=object{resignation=Resignation}}
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/resignations/{resignationId} [patch]
func (h *Handler) Update(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req UpdateResignationRequest
	if err := c.Bind().JSON(&req); err != nil { return response.BadRequest(c, "INVALID_BODY", "Invalid request body") }
	r, err := h.service.Update(c.Context(), orgID, c.Params("employeeId"), c.Params("resignationId"), req)
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"resignation": r}, "Resignation updated")
}

// Withdraw godoc
//
//	@Summary		Withdraw resignation
//	@Description	Employee withdraws a submitted (not yet accepted) resignation.
//	@Description	**Required permission:** `hrm.resignations.manage`
//	@Tags			HRM / Resignations
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path	string	true	"Organization ID"
//	@Param			employeeId		path	string	true	"Employee public ID"
//	@Param			resignationId	path	string	true	"Resignation public ID"
//	@Success		200				{object}	response.OK{data=object{resignation=Resignation}}
//	@Failure		409				{object}	response.Error	"WRONG_STATUS"
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/resignations/{resignationId}/withdraw [post]
func (h *Handler) Withdraw(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	r, err := h.service.Withdraw(c.Context(), orgID, c.Params("employeeId"), c.Params("resignationId"))
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"resignation": r}, "Resignation withdrawn")
}

// Accept godoc
//
//	@Summary		Accept resignation (HR action)
//	@Description	HR accepts the resignation. Updates `employee.status = 'resigned'` and sets
//	@Description	`employee.termination_date = last_working_date`. Both changes are atomic.
//	@Description
//	@Description	**Required permission:** `hrm.resignations.process`
//	@Tags			HRM / Resignations
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path	string	true	"Organization ID"
//	@Param			employeeId		path	string	true	"Employee public ID"
//	@Param			resignationId	path	string	true	"Resignation public ID"
//	@Success		200				{object}	response.OK{data=object{resignation=Resignation}}
//	@Failure		409				{object}	response.Error	"ALREADY_ACCEPTED or WRONG_STATUS"
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/resignations/{resignationId}/accept [post]
func (h *Handler) Accept(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	r, err := h.service.Accept(c.Context(), orgID, c.Params("employeeId"), c.Params("resignationId"), userID)
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"resignation": r}, "Resignation accepted")
}

// Reject godoc
//
//	@Summary		Reject resignation (HR action)
//	@Description	HR rejects the resignation. Employee may resubmit.
//	@Description	**Required permission:** `hrm.resignations.process`
//	@Tags			HRM / Resignations
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path	string	true	"Organization ID"
//	@Param			employeeId		path	string	true	"Employee public ID"
//	@Param			resignationId	path	string	true	"Resignation public ID"
//	@Success		200				{object}	response.OK{data=object{resignation=Resignation}}
//	@Failure		409				{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/resignations/{resignationId}/reject [post]
func (h *Handler) Reject(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	r, err := h.service.Reject(c.Context(), orgID, c.Params("employeeId"), c.Params("resignationId"))
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"resignation": r}, "Resignation rejected")
}

func (h *Handler) err(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrNotFound):
		return response.NotFound(c, "RESIGNATION_NOT_FOUND", "Resignation record not found")
	case errors.Is(err, ErrAlreadyActive):
		return response.Conflict(c, "ALREADY_ACTIVE", "Employee already has an active resignation — withdraw it first")
	case errors.Is(err, ErrResignationDateReq):
		return response.BadRequest(c, "RESIGNATION_DATE_REQUIRED", "resignation_date is required (YYYY-MM-DD)")
	case errors.Is(err, ErrInvalidDate):
		return response.BadRequest(c, "INVALID_DATE", "date must be a valid YYYY-MM-DD")
	case errors.Is(err, ErrInvalidReasonCategory):
		return response.BadRequest(c, "INVALID_REASON_CATEGORY", "Invalid reason_category value")
	case errors.Is(err, ErrWrongStatus):
		return response.Conflict(c, "WRONG_STATUS", "Action not allowed in current resignation status")
	case errors.Is(err, ErrAlreadyAccepted):
		return response.Conflict(c, "ALREADY_ACCEPTED", "Resignation has already been accepted")
	default:
		log.Error("resignations: error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}
