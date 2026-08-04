// backend/internal/hrm/transfers/handler.go
package transfers

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

// resolveListFilter builds the shared parts of a TransferListFilter (scope,
// pagination, status) once userID is already known — ListAll and
// ListForEmployee each add their own employee_id source (query param vs path
// param) on top. err is a plain ResolveScope failure, never a written
// response — callers log and 500 it themselves.
func (h *Handler) resolveListFilter(c fiber.Ctx, orgID, userID string) (TransferListFilter, error) {
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.transfers")
	if err != nil {
		return TransferListFilter{}, err
	}
	filter := TransferListFilter{
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
//	@Summary		List all transfers (HR view)
//	@Description	Returns all transfer records across the organization.
//	@Description	Transfers are always HR or manager-initiated (ADR-0014: not employee self-service).
//	@Description
//	@Description	**Required permission:** `hrm.transfers.view`
//	@Tags			HRM / Transfers
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			status		query		string	false	"Filter by status"
//	@Param			employee_id	query		string	false	"Filter by employee UUID"
//	@Success		200			{object}	response.OK{data=TransferListResponse}
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/transfers [get]
func (h *Handler) ListAll(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	filter, err := h.resolveListFilter(c, orgID, userID)
	if err != nil { log.Error("transfers: ListAll", slog.Any("error", err)); return response.InternalServerError(c) }
	filter.EmployeeID = c.Query("employee_id")
	res, err := h.service.List(c.Context(), orgID, filter)
	if err != nil { log.Error("transfers: ListAll", slog.Any("error", err)); return response.InternalServerError(c) }
	return response.OK(c, res, "OK")
}

// ListForEmployee godoc
//
//	@Summary		List employee transfers
//	@Description	Returns transfer records for a specific employee.
//	@Description
//	@Description	**Required permission:** `hrm.transfers.view`
//	@Tags			HRM / Transfers
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			employeeId	path		string	true	"Employee public ID"
//	@Success		200			{object}	response.OK{data=TransferListResponse}
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/transfers [get]
func (h *Handler) ListForEmployee(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	filter, err := h.resolveListFilter(c, orgID, userID)
	if err != nil { log.Error("transfers: ListForEmployee", slog.Any("error", err)); return response.InternalServerError(c) }
	filter.EmployeeID = c.Params("employeeId")
	res, err := h.service.List(c.Context(), orgID, filter)
	if err != nil { log.Error("transfers: ListForEmployee", slog.Any("error", err)); return response.InternalServerError(c) }
	return response.OK(c, res, "OK")
}

// Create godoc
//
//	@Summary		Create transfer record
//	@Description	Creates a transfer record. Current department/manager snapshotted automatically.
//	@Description
//	@Description	**Required permission:** `hrm.transfers.manage`
//	@Tags			HRM / Transfers
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string					true	"Organization ID"
//	@Param			employeeId	path		string					true	"Employee public ID"
//	@Param			body		body		CreateTransferRequest	true	"Transfer details"
//	@Success		201			{object}	response.Created{data=object{transfer=Transfer}}
//	@Failure		400			{object}	response.Error
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/transfers [post]
func (h *Handler) Create(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req CreateTransferRequest
	if err := c.Bind().JSON(&req); err != nil { return response.BadRequest(c, "INVALID_BODY", "Invalid request body") }
	t, err := h.service.Create(c.Context(), orgID, c.Params("employeeId"), userID, req)
	if err != nil { return h.err(c, err) }
	return response.Created(c, fiber.Map{"transfer": t}, "Transfer record created")
}

// Get godoc
//
//	@Summary		Get transfer record
//	@Tags			HRM / Transfers
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			employeeId	path		string	true	"Employee public ID"
//	@Param			transferId	path		string	true	"Transfer public ID (trf_*)"
//	@Success		200			{object}	response.OK{data=object{transfer=Transfer}}
//	@Failure		404			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/transfers/{transferId} [get]
func (h *Handler) Get(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	employeeID := c.Params("employeeId")
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.transfers")
	if err != nil { log.Error("transfers: Get", slog.Any("error", err)); return response.InternalServerError(c) }
	allowed, err := h.scopeResolver.AuthorizeRecordAccess(c.Context(), scopeTier, orgID, userID, employeeID)
	if err != nil { log.Error("transfers: Get", slog.Any("error", err)); return response.InternalServerError(c) }
	if !allowed { return response.Forbidden(c, "RECORD_ACCESS_DENIED", "You do not have access to this record") }
	t, err := h.service.Get(c.Context(), orgID, employeeID, c.Params("transferId"))
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"transfer": t}, "OK")
}

// Update godoc
//
//	@Summary		Update transfer record (draft only)
//	@Tags			HRM / Transfers
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string					true	"Organization ID"
//	@Param			employeeId	path		string					true	"Employee public ID"
//	@Param			transferId	path		string					true	"Transfer public ID"
//	@Param			body		body		UpdateTransferRequest	true	"Fields to update"
//	@Success		200			{object}	response.OK{data=object{transfer=Transfer}}
//	@Failure		409			{object}	response.Error	"WRONG_STATUS"
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/transfers/{transferId} [patch]
func (h *Handler) Update(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req UpdateTransferRequest
	if err := c.Bind().JSON(&req); err != nil { return response.BadRequest(c, "INVALID_BODY", "Invalid request body") }
	t, err := h.service.Update(c.Context(), orgID, c.Params("employeeId"), c.Params("transferId"), req)
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"transfer": t}, "Transfer updated")
}

// Submit godoc
//
//	@Summary		Submit transfer for approval
//	@Tags			HRM / Transfers
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			employeeId	path	string	true	"Employee public ID"
//	@Param			transferId	path	string	true	"Transfer public ID"
//	@Success		200			{object}	response.OK{data=object{transfer=Transfer}}
//	@Failure		409			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/transfers/{transferId}/submit [post]
func (h *Handler) Submit(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	t, err := h.service.Submit(c.Context(), orgID, c.Params("employeeId"), c.Params("transferId"), userID)
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"transfer": t}, "Transfer submitted")
}

// Cancel godoc
//
//	@Summary		Cancel transfer
//	@Tags			HRM / Transfers
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			employeeId	path	string	true	"Employee public ID"
//	@Param			transferId	path	string	true	"Transfer public ID"
//	@Success		200			{object}	response.OK{data=object{transfer=Transfer}}
//	@Failure		409			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/transfers/{transferId}/cancel [post]
func (h *Handler) Cancel(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	t, err := h.service.Cancel(c.Context(), orgID, c.Params("employeeId"), c.Params("transferId"))
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"transfer": t}, "Transfer cancelled")
}

// Apply godoc
//
//	@Summary		Apply transfer to employee record
//	@Description	Updates employee.department_id and/or employee.manager_id in one transaction.
//	@Description	**Required permission:** `hrm.transfers.apply`
//	@Tags			HRM / Transfers
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			employeeId	path	string	true	"Employee public ID"
//	@Param			transferId	path	string	true	"Transfer public ID"
//	@Success		200			{object}	response.OK{data=object{transfer=Transfer}}
//	@Failure		409			{object}	response.Error	"ALREADY_APPLIED or NOT_APPROVED"
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/transfers/{transferId}/apply [post]
func (h *Handler) Apply(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	t, err := h.service.Apply(c.Context(), orgID, c.Params("employeeId"), c.Params("transferId"), userID)
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"transfer": t}, "Transfer applied")
}

func (h *Handler) err(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrNotFound):
		return response.NotFound(c, "TRANSFER_NOT_FOUND", "Transfer record not found")
	case errors.Is(err, ErrInvalidTransferType):
		return response.BadRequest(c, "INVALID_TRANSFER_TYPE", "transfer_type must be: department, location, reporting, or full")
	case errors.Is(err, ErrEffectiveDateReq):
		return response.BadRequest(c, "EFFECTIVE_DATE_REQUIRED", "effective_date is required (YYYY-MM-DD)")
	case errors.Is(err, ErrInvalidDate):
		return response.BadRequest(c, "INVALID_DATE", "effective_date must be a valid YYYY-MM-DD")
	case errors.Is(err, ErrWrongStatus):
		return response.Conflict(c, "WRONG_STATUS", "Action not allowed in current transfer status")
	case errors.Is(err, ErrAlreadyApplied):
		return response.Conflict(c, "ALREADY_APPLIED", "Transfer has already been applied")
	case errors.Is(err, ErrNotApproved):
		return response.Conflict(c, "NOT_APPROVED", "Transfer must be approved before applying")
	default:
		log.Error("transfers: error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}
