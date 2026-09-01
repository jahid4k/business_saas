// backend/internal/hrm/warnings/handler.go
package warnings

import (
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/scope"
	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles HRM employee warning HTTP endpoints.
type Handler struct {
	service       Service
	authz         authz.Service
	scopeResolver *scope.Resolver
}

func NewHandler(service Service, authzSvc authz.Service, scopeResolver *scope.Resolver) *Handler {
	return &Handler{service: service, authz: authzSvc, scopeResolver: scopeResolver}
}

// resolveListFilter builds the shared parts of a WarningListFilter (scope,
// pagination, status/active_only) once userID is already known — ListAll and
// ListForEmployee each add their own employee_id source (query param vs path
// param) on top. err is a plain ResolveScope failure, never a written
// response — callers log and 500 it themselves, matching every other handler
// in this file.
func (h *Handler) resolveListFilter(c fiber.Ctx, orgID, userID string) (WarningListFilter, error) {
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.warnings")
	if err != nil {
		return WarningListFilter{}, err
	}
	filter := WarningListFilter{
		Status:       c.Query("status"),
		ActiveOnly:   strings.ToLower(c.Query("active_only")) == "true",
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
//	@Summary		List all warnings (HR view)
//	@Description	Returns all warning records across the organization.
//	@Description	Filter by employee_id, status, or active_only=true for active warnings only.
//	@Description
//	@Description	**Required permission:** `hrm.warnings.view`
//	@Tags			HRM / Warnings
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			employee_id	query		string	false	"Filter by employee UUID"
//	@Param			status		query		string	false	"draft|pending_approval|issued|acknowledged|appealed|closed|cancelled"
//	@Param			active_only	query		bool	false	"When true, only return is_active=TRUE records (for escalation views)"
//	@Success		200			{object}	response.OK{data=WarningListResponse}
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/warnings [get]
func (h *Handler) ListAll(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	filter, err := h.resolveListFilter(c, orgID, userID)
	if err != nil {
		log.Error("warnings: ListAll", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	filter.EmployeeID = c.Query("employee_id")
	res, err := h.service.List(c.Context(), orgID, filter)
	if err != nil {
		log.Error("warnings: ListAll", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, res, "OK")
}

// ListForEmployee godoc
//
//	@Summary		List employee warnings
//	@Description	Returns warning records for a specific employee.
//	@Description
//	@Description	**Required permission:** `hrm.warnings.view`
//	@Tags			HRM / Warnings
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			employeeId	path		string	true	"Employee public ID (emp_*)"
//	@Param			status		query		string	false	"Filter by status"
//	@Param			active_only	query		bool	false	"Only active warnings"
//	@Success		200			{object}	response.OK{data=WarningListResponse}
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/warnings [get]
func (h *Handler) ListForEmployee(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	filter, err := h.resolveListFilter(c, orgID, userID)
	if err != nil {
		log.Error("warnings: ListForEmployee", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	filter.EmployeeID = c.Params("employeeId")
	res, err := h.service.List(c.Context(), orgID, filter)
	if err != nil {
		log.Error("warnings: ListForEmployee", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, res, "OK")
}

// Create godoc
//
//	@Summary		Create warning record
//	@Description	Creates a warning record in draft status. Warning type config
//	@Description	(severity, response window) is snapshotted automatically from A3.
//	@Description
//	@Description	Issue via `POST .../issue` to formally send to the employee.
//	@Description
//	@Description	**Required permission:** `hrm.warnings.manage`
//	@Tags			HRM / Warnings
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string				true	"Organization ID"
//	@Param			employeeId	path		string				true	"Employee public ID"
//	@Param			body		body		CreateWarningRequest	true	"Warning details"
//	@Success		201			{object}	response.Created{data=object{warning=EmployeeWarning}}
//	@Failure		400			{object}	response.Error
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		404			{object}	response.Error	"WARNING_TYPE_NOT_FOUND"
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/warnings [post]
func (h *Handler) Create(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateWarningRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	w, err := h.service.Create(c.Context(), orgID, c.Params("employeeId"), userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"warning": w}, "Warning record created")
}

// Get godoc
//
//	@Summary		Get warning record
//	@Description	Returns a single warning record.
//	@Description
//	@Description	**Required permission:** `hrm.warnings.view`
//	@Tags			HRM / Warnings
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			employeeId	path		string	true	"Employee public ID"
//	@Param			warningId	path		string	true	"Warning public ID (ew_*)"
//	@Success		200			{object}	response.OK{data=object{warning=EmployeeWarning}}
//	@Failure		404			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/warnings/{warningId} [get]
func (h *Handler) Get(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	employeeID := c.Params("employeeId")
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.warnings")
	if err != nil {
		log.Error("warnings: Get", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	allowed, err := h.scopeResolver.AuthorizeRecordAccess(c.Context(), scopeTier, orgID, userID, employeeID)
	if err != nil {
		log.Error("warnings: Get", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	if !allowed {
		return response.Forbidden(c, "RECORD_ACCESS_DENIED", "You do not have access to this record")
	}
	w, err := h.service.Get(c.Context(), orgID, employeeID, c.Params("warningId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"warning": w}, "OK")
}

// Update godoc
//
//	@Summary		Update warning record (draft only)
//	@Description	Updates a warning record. Only draft records can be updated.
//	@Description
//	@Description	**Required permission:** `hrm.warnings.manage`
//	@Tags			HRM / Warnings
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string				true	"Organization ID"
//	@Param			employeeId	path		string				true	"Employee public ID"
//	@Param			warningId	path		string				true	"Warning public ID"
//	@Param			body		body		UpdateWarningRequest	true	"Fields to update"
//	@Success		200			{object}	response.OK{data=object{warning=EmployeeWarning}}
//	@Failure		409			{object}	response.Error	"WRONG_STATUS"
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/warnings/{warningId} [patch]
func (h *Handler) Update(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateWarningRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	w, err := h.service.Update(c.Context(), orgID, c.Params("employeeId"), c.Params("warningId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"warning": w}, "Warning updated")
}

// Issue godoc
//
//	@Summary		Issue warning to employee
//	@Description	Formally issues a draft warning. Sets is_active=TRUE, computes response deadline,
//	@Description	and checks A3 escalation rules (logs HR alert if threshold reached).
//	@Description
//	@Description	**Required permission:** `hrm.warnings.issue`
//	@Tags			HRM / Warnings
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string			true	"Organization ID"
//	@Param			employeeId	path		string			true	"Employee public ID"
//	@Param			warningId	path		string			true	"Warning public ID"
//	@Param			body		body		IssueRequest	false	"Optional document ID"
//	@Success		200			{object}	response.OK{data=object{warning=EmployeeWarning}}
//	@Failure		409			{object}	response.Error	"ALREADY_ISSUED or WRONG_STATUS"
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/warnings/{warningId}/issue [post]
func (h *Handler) Issue(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req IssueRequest
	_ = c.Bind().JSON(&req)
	w, err := h.service.Issue(c.Context(), orgID, c.Params("employeeId"), c.Params("warningId"), userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"warning": w}, "Warning issued")
}

// Acknowledge godoc
//
//	@Summary		Acknowledge warning (employee action)
//	@Description	Employee acknowledges receipt of an issued warning. Optionally adds a response note.
//	@Description
//	@Description	**Required permission:** `hrm.warnings.acknowledge`
//	@Tags			HRM / Warnings
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string				true	"Organization ID"
//	@Param			employeeId	path		string				true	"Employee public ID"
//	@Param			warningId	path		string				true	"Warning public ID"
//	@Param			body		body		AcknowledgeRequest	false	"Optional response note"
//	@Success		200			{object}	response.OK{data=object{warning=EmployeeWarning}}
//	@Failure		409			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/warnings/{warningId}/acknowledge [post]
func (h *Handler) Acknowledge(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req AcknowledgeRequest
	_ = c.Bind().JSON(&req)
	w, err := h.service.Acknowledge(c.Context(), orgID, c.Params("employeeId"), c.Params("warningId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"warning": w}, "Warning acknowledged")
}

// Appeal godoc
//
//	@Summary		Appeal warning (employee action)
//	@Description	Employee contests an issued warning. Only allowed if warning_type.employee_can_respond=true.
//	@Description
//	@Description	**Required permission:** `hrm.warnings.acknowledge`
//	@Tags			HRM / Warnings
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string			true	"Organization ID"
//	@Param			employeeId	path		string			true	"Employee public ID"
//	@Param			warningId	path		string			true	"Warning public ID"
//	@Param			body		body		AppealRequest	true	"Appeal reason"
//	@Success		200			{object}	response.OK{data=object{warning=EmployeeWarning}}
//	@Failure		409			{object}	response.Error	"CANNOT_APPEAL or WRONG_STATUS"
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/warnings/{warningId}/appeal [post]
func (h *Handler) Appeal(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req AppealRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	w, err := h.service.Appeal(c.Context(), orgID, c.Params("employeeId"), c.Params("warningId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"warning": w}, "Appeal submitted")
}

// Close godoc
//
//	@Summary		Close warning (HR action)
//	@Description	HR closes an active warning. If warning is appealed, appeal_resolution is required.
//	@Description
//	@Description	**Required permission:** `hrm.warnings.close`
//	@Tags			HRM / Warnings
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string			true	"Organization ID"
//	@Param			employeeId	path		string			true	"Employee public ID"
//	@Param			warningId	path		string			true	"Warning public ID"
//	@Param			body		body		CloseRequest	false	"Appeal resolution (required if status=appealed)"
//	@Success		200			{object}	response.OK{data=object{warning=EmployeeWarning}}
//	@Failure		409			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/warnings/{warningId}/close [post]
func (h *Handler) Close(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CloseRequest
	_ = c.Bind().JSON(&req)
	w, err := h.service.Close(c.Context(), orgID, c.Params("employeeId"), c.Params("warningId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"warning": w}, "Warning closed")
}

// Cancel godoc
//
//	@Summary		Cancel warning (HR action)
//	@Description	Cancels a warning before it is closed.
//	@Description
//	@Description	**Required permission:** `hrm.warnings.close`
//	@Tags			HRM / Warnings
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			employeeId	path	string	true	"Employee public ID"
//	@Param			warningId	path	string	true	"Warning public ID"
//	@Success		200			{object}	response.OK{data=object{warning=EmployeeWarning}}
//	@Failure		409			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/warnings/{warningId}/cancel [post]
func (h *Handler) Cancel(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	w, err := h.service.Cancel(c.Context(), orgID, c.Params("employeeId"), c.Params("warningId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"warning": w}, "Warning cancelled")
}

func (h *Handler) err(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrNotFound):
		return response.NotFound(c, "WARNING_NOT_FOUND", "Warning record not found")
	case errors.Is(err, ErrWarningTypeNotFound):
		return response.NotFound(c, "WARNING_TYPE_NOT_FOUND", "Warning type not found or inactive")
	case errors.Is(err, ErrWarningTypeIDRequired):
		return response.BadRequest(c, "WARNING_TYPE_REQUIRED", "warning_type_id is required")
	case errors.Is(err, ErrTitleRequired):
		return response.BadRequest(c, "TITLE_REQUIRED", "title is required")
	case errors.Is(err, ErrDescriptionRequired):
		return response.BadRequest(c, "DESCRIPTION_REQUIRED", "description is required")
	case errors.Is(err, ErrIncidentDateRequired):
		return response.BadRequest(c, "INCIDENT_DATE_REQUIRED", "incident_date is required (YYYY-MM-DD)")
	case errors.Is(err, ErrInvalidDate):
		return response.BadRequest(c, "INVALID_DATE", "date must be a valid YYYY-MM-DD")
	case errors.Is(err, ErrWrongStatus):
		return response.Conflict(c, "WRONG_STATUS", "Action not allowed in current warning status")
	case errors.Is(err, ErrAlreadyIssued):
		return response.Conflict(c, "ALREADY_ISSUED", "Warning has already been issued")
	case errors.Is(err, ErrCannotAppeal):
		return response.Conflict(c, "CANNOT_APPEAL", "This warning type does not allow employee response")
	default:
		log.Error("warnings: error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}
