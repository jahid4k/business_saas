// backend/internal/hrm/complaints/handler.go
package complaints

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

// resolveListFilter builds the shared parts of a ComplaintListFilter (scope,
// pagination, status) once userID is already known — ListAll and
// ListForEmployee each add their own employee_id source (query param vs path
// param) on top. err is a plain ResolveScope failure, never a written
// response — callers log and 500 it themselves.
func (h *Handler) resolveListFilter(c fiber.Ctx, orgID, userID string) (ComplaintListFilter, error) {
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.complaints")
	if err != nil {
		return ComplaintListFilter{}, err
	}
	filter := ComplaintListFilter{
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

func (h *Handler) ListAll(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	filter, err := h.resolveListFilter(c, orgID, userID)
	if err != nil { log.Error("complaints: ListAll", slog.Any("error", err)); return response.InternalServerError(c) }
	filter.EmployeeID = c.Query("employee_id")
	res, err := h.service.List(c.Context(), orgID, filter)
	if err != nil { log.Error("complaints: ListAll", slog.Any("error", err)); return response.InternalServerError(c) }
	return response.OK(c, res, "OK")
}

func (h *Handler) ListForEmployee(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	filter, err := h.resolveListFilter(c, orgID, userID)
	if err != nil { log.Error("complaints: ListForEmployee", slog.Any("error", err)); return response.InternalServerError(c) }
	filter.EmployeeID = c.Params("employeeId")
	res, err := h.service.List(c.Context(), orgID, filter)
	if err != nil { log.Error("complaints: ListForEmployee", slog.Any("error", err)); return response.InternalServerError(c) }
	return response.OK(c, res, "OK")
}

// Create godoc
//
//	@Summary		Submit complaint
//	@Description	Employee submits a complaint or grievance. Set is_anonymous=true to hide identity from non-HR users.
//	@Description
//	@Description	**Required permission:** `hrm.complaints.manage`
//	@Tags			HRM / Complaints
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path		string					true	"Organization ID"
//	@Param			employeeId	path		string					true	"Employee public ID"
//	@Param			body		body		CreateComplaintRequest	true	"Complaint details"
//	@Success		201			{object}	response.Created{data=object{complaint=Complaint}}
//	@Failure		400			{object}	response.Error
//	@Failure		401			{object}	response.Error
//	@Failure		403			{object}	response.Error
//	@Failure		500			{object}	response.Error
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/complaints [post]
func (h *Handler) Create(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req CreateComplaintRequest
	if err := c.Bind().JSON(&req); err != nil { return response.BadRequest(c, "INVALID_BODY", "Invalid request body") }
	cmp, err := h.service.Create(c.Context(), orgID, c.Params("employeeId"), userID, req)
	if err != nil { return h.err(c, err) }
	return response.Created(c, fiber.Map{"complaint": cmp}, "Complaint submitted")
}

func (h *Handler) Get(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	employeeID := c.Params("employeeId")
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.complaints")
	if err != nil { log.Error("complaints: Get", slog.Any("error", err)); return response.InternalServerError(c) }
	allowed, err := h.scopeResolver.AuthorizeRecordAccess(c.Context(), scopeTier, orgID, userID, employeeID)
	if err != nil { log.Error("complaints: Get", slog.Any("error", err)); return response.InternalServerError(c) }
	if !allowed { return response.Forbidden(c, "RECORD_ACCESS_DENIED", "You do not have access to this record") }
	cmp, err := h.service.Get(c.Context(), orgID, employeeID, c.Params("complaintId"))
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"complaint": cmp}, "OK")
}

func (h *Handler) Update(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req UpdateComplaintRequest
	if err := c.Bind().JSON(&req); err != nil { return response.BadRequest(c, "INVALID_BODY", "Invalid request body") }
	cmp, err := h.service.Update(c.Context(), orgID, c.Params("employeeId"), c.Params("complaintId"), req)
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"complaint": cmp}, "Complaint updated")
}

func (h *Handler) StartReview(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	cmp, err := h.service.StartReview(c.Context(), orgID, c.Params("employeeId"), c.Params("complaintId"))
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"complaint": cmp}, "Complaint under review")
}

func (h *Handler) Assign(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req AssignRequest
	if err := c.Bind().JSON(&req); err != nil { return response.BadRequest(c, "INVALID_BODY", "Invalid request body") }
	cmp, err := h.service.Assign(c.Context(), orgID, c.Params("employeeId"), c.Params("complaintId"), req)
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"complaint": cmp}, "Investigator assigned")
}

func (h *Handler) Resolve(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req ResolveRequest
	if err := c.Bind().JSON(&req); err != nil { return response.BadRequest(c, "INVALID_BODY", "Invalid request body") }
	cmp, err := h.service.Resolve(c.Context(), orgID, c.Params("employeeId"), c.Params("complaintId"), userID, req)
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"complaint": cmp}, "Complaint resolved")
}

func (h *Handler) Dismiss(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok { return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required") }
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	var req DismissRequest
	if err := c.Bind().JSON(&req); err != nil { return response.BadRequest(c, "INVALID_BODY", "Invalid request body") }
	cmp, err := h.service.Dismiss(c.Context(), orgID, c.Params("employeeId"), c.Params("complaintId"), userID, req)
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"complaint": cmp}, "Complaint dismissed")
}

func (h *Handler) Withdraw(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok { return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required") }
	cmp, err := h.service.Withdraw(c.Context(), orgID, c.Params("employeeId"), c.Params("complaintId"))
	if err != nil { return h.err(c, err) }
	return response.OK(c, fiber.Map{"complaint": cmp}, "Complaint withdrawn")
}

func (h *Handler) err(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrNotFound): return response.NotFound(c, "COMPLAINT_NOT_FOUND", "Complaint not found")
	case errors.Is(err, ErrTitleRequired): return response.BadRequest(c, "TITLE_REQUIRED", "title is required")
	case errors.Is(err, ErrDescriptionRequired): return response.BadRequest(c, "DESCRIPTION_REQUIRED", "description is required")
	case errors.Is(err, ErrInvalidType): return response.BadRequest(c, "INVALID_TYPE", "Invalid complaint_type")
	case errors.Is(err, ErrInvalidDate): return response.BadRequest(c, "INVALID_DATE", "date must be a valid YYYY-MM-DD")
	case errors.Is(err, ErrWrongStatus): return response.Conflict(c, "WRONG_STATUS", "Action not allowed in current complaint status")
	case errors.Is(err, ErrInvestigatorRequired): return response.BadRequest(c, "INVESTIGATOR_REQUIRED", "investigator_id is required")
	case errors.Is(err, ErrResolutionRequired): return response.BadRequest(c, "RESOLUTION_REQUIRED", "resolution text is required")
	default: log.Error("complaints: error", slog.Any("error", err)); return response.InternalServerError(c)
	}
}
