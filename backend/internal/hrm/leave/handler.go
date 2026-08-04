// backend/internal/hrm/leave/handler.go
package leave

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

// Handler handles all HRM leave endpoints — both leave types and leave requests.
type Handler struct {
	service       Service
	authz         authz.Service
	scopeResolver *scope.Resolver
}

func NewHandler(service Service, authzSvc authz.Service, scopeResolver *scope.Resolver) *Handler {
	return &Handler{service: service, authz: authzSvc, scopeResolver: scopeResolver}
}

// ─────────────────────────────────────────────────────────
// Leave Type handlers
// ─────────────────────────────────────────────────────────

// ListLeaveTypes handles GET /api/v1/organizations/:orgId/hrm/leave/types
// Requires: hrm.leave.view
func (h *Handler) ListLeaveTypes(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	activeOnly := strings.ToLower(c.Query("active")) == "true"
	result, err := h.service.ListLeaveTypes(c.Context(), orgID, activeOnly)
	if err != nil {
		log.Error("leave: ListLeaveTypes error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, result, "OK")
}

// CreateLeaveType handles POST /api/v1/organizations/:orgId/hrm/leave/types
// Requires: hrm.leave.create
func (h *Handler) CreateLeaveType(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateLeaveTypeRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	lt, err := h.service.CreateLeaveType(c.Context(), orgID, userID, req)
	if err != nil {
		return h.leaveTypeError(c, err)
	}
	return response.Created(c, fiber.Map{"leave_type": lt}, "Leave type created")
}

// GetLeaveType handles GET /api/v1/organizations/:orgId/hrm/leave/types/:typeId
// Requires: hrm.leave.view
func (h *Handler) GetLeaveType(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	lt, err := h.service.GetLeaveType(c.Context(), orgID, c.Params("typeId"))
	if err != nil {
		return h.leaveTypeError(c, err)
	}
	return response.OK(c, fiber.Map{"leave_type": lt}, "OK")
}

// UpdateLeaveType handles PATCH /api/v1/organizations/:orgId/hrm/leave/types/:typeId
// Requires: hrm.leave.update
func (h *Handler) UpdateLeaveType(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateLeaveTypeRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	lt, err := h.service.UpdateLeaveType(c.Context(), orgID, c.Params("typeId"), req)
	if err != nil {
		return h.leaveTypeError(c, err)
	}
	return response.OK(c, fiber.Map{"leave_type": lt}, "Leave type updated")
}

// DeleteLeaveType handles DELETE /api/v1/organizations/:orgId/hrm/leave/types/:typeId
// Requires: hrm.leave.delete
func (h *Handler) DeleteLeaveType(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.DeleteLeaveType(c.Context(), orgID, c.Params("typeId")); err != nil {
		return h.leaveTypeError(c, err)
	}
	return response.NoContent(c)
}

// ─────────────────────────────────────────────────────────
// Leave Request handlers
// ─────────────────────────────────────────────────────────

// ListRequests handles GET /api/v1/organizations/:orgId/hrm/leave/requests
// Requires: hrm.leave.view
// Query: employee_id, leave_type_id, status, limit, offset
func (h *Handler) ListRequests(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}

	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.leave")
	if err != nil {
		log.Error("leave: ListRequests: resolve scope", slog.Any("error", err))
		return response.InternalServerError(c)
	}

	filter := LeaveRequestFilter{Scope: scopeTier, CallerUserID: userID}
	filter.EmployeeID = strings.TrimSpace(c.Query("employee_id"))
	filter.LeaveTypeID = strings.TrimSpace(c.Query("leave_type_id"))

	if st := strings.TrimSpace(c.Query("status")); st != "" {
		s := LeaveRequestStatus(st)
		if !s.IsValid() {
			return response.BadRequest(c, "INVALID_STATUS",
				"status must be one of: pending, approved, rejected, cancelled")
		}
		filter.Status = s
	}
	if limit, err := strconv.Atoi(c.Query("limit", "")); err == nil {
		filter.Limit = limit
	}
	if offset, err := strconv.Atoi(c.Query("offset", "")); err == nil {
		filter.Offset = offset
	}

	result, err := h.service.ListRequests(c.Context(), orgID, filter)
	if err != nil {
		log.Error("leave: ListRequests error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, result, "OK")
}

// CreateRequest handles POST /api/v1/organizations/:orgId/hrm/leave/requests
// Requires: hrm.leave.request
func (h *Handler) CreateRequest(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateLeaveRequestRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	lr, err := h.service.CreateRequest(c.Context(), orgID, userID, req)
	if err != nil {
		return h.leaveRequestError(c, err)
	}
	return response.Created(c, fiber.Map{"leave_request": lr}, "Leave request submitted")
}

// GetRequest handles GET /api/v1/organizations/:orgId/hrm/leave/requests/:reqId
// Requires: hrm.leave.view
func (h *Handler) GetRequest(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	lr, err := h.service.GetRequest(c.Context(), orgID, c.Params("reqId"))
	if err != nil {
		return h.leaveRequestError(c, err)
	}
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.leave")
	if err != nil {
		log.Error("leave: GetRequest", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	allowed, err := h.scopeResolver.AuthorizeRecordAccess(c.Context(), scopeTier, orgID, userID, lr.EmployeeID)
	if err != nil {
		log.Error("leave: GetRequest", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	if !allowed {
		return response.Forbidden(c, "RECORD_ACCESS_DENIED", "You do not have access to this record")
	}
	return response.OK(c, fiber.Map{"leave_request": lr}, "OK")
}

// ApproveRequest handles POST /api/v1/organizations/:orgId/hrm/leave/requests/:reqId/approve
// Requires: hrm.leave.approve
func (h *Handler) ApproveRequest(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req ReviewLeaveRequestRequest
	_ = c.Bind().JSON(&req) // note field is optional
	lr, err := h.service.ApproveRequest(c.Context(), orgID, c.Params("reqId"), userID, req)
	if err != nil {
		return h.leaveRequestError(c, err)
	}
	return response.OK(c, fiber.Map{"leave_request": lr}, "Leave request approved")
}

// RejectRequest handles POST /api/v1/organizations/:orgId/hrm/leave/requests/:reqId/reject
// Requires: hrm.leave.approve
func (h *Handler) RejectRequest(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req ReviewLeaveRequestRequest
	_ = c.Bind().JSON(&req)
	lr, err := h.service.RejectRequest(c.Context(), orgID, c.Params("reqId"), userID, req)
	if err != nil {
		return h.leaveRequestError(c, err)
	}
	return response.OK(c, fiber.Map{"leave_request": lr}, "Leave request rejected")
}

// CancelRequest handles POST /api/v1/organizations/:orgId/hrm/leave/requests/:reqId/cancel
// Requires: hrm.leave.request (any member can cancel their own)
func (h *Handler) CancelRequest(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	lr, err := h.service.CancelRequest(c.Context(), orgID, c.Params("reqId"), userID)
	if err != nil {
		return h.leaveRequestError(c, err)
	}
	return response.OK(c, fiber.Map{"leave_request": lr}, "Leave request cancelled")
}

// DeleteRequest handles DELETE /api/v1/organizations/:orgId/hrm/leave/requests/:reqId
// Requires: hrm.leave.delete
func (h *Handler) DeleteRequest(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.DeleteRequest(c.Context(), orgID, c.Params("reqId"), userID); err != nil {
		return h.leaveRequestError(c, err)
	}
	return response.NoContent(c)
}

// ─────────────────────────────────────────────────────────
// Error mappers
// ─────────────────────────────────────────────────────────

func (h *Handler) leaveTypeError(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrLeaveTypeNotFound):
		return response.NotFound(c, "LEAVE_TYPE_NOT_FOUND", "Leave type not found")
	case errors.Is(err, ErrLeaveTypeNameReq):
		return response.BadRequest(c, "LEAVE_TYPE_NAME_REQUIRED", "Leave type name is required")
	case errors.Is(err, ErrLeaveTypeNameLong):
		return response.BadRequest(c, "LEAVE_TYPE_NAME_TOO_LONG", "Name must not exceed 100 characters")
	case errors.Is(err, ErrLeaveTypeConflict):
		return response.Conflict(c, "LEAVE_TYPE_NAME_CONFLICT", "A leave type with this name already exists")
	default:
		log.Error("leave type error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}

func (h *Handler) leaveRequestError(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrLeaveRequestNotFound):
		return response.NotFound(c, "LEAVE_REQUEST_NOT_FOUND", "Leave request not found")
	case errors.Is(err, ErrLeaveTypeNotFound):
		return response.BadRequest(c, "LEAVE_TYPE_NOT_FOUND", "The specified leave type does not exist")
	case errors.Is(err, ErrLeaveTypeInactive):
		return response.BadRequest(c, "LEAVE_TYPE_INACTIVE", "The selected leave type is inactive")
	case errors.Is(err, ErrEmployeeIDRequired):
		return response.BadRequest(c, "EMPLOYEE_ID_REQUIRED", "employee_id is required")
	case errors.Is(err, ErrLeaveTypeIDRequired):
		return response.BadRequest(c, "LEAVE_TYPE_ID_REQUIRED", "leave_type_id is required")
	case errors.Is(err, ErrStartDateRequired):
		return response.BadRequest(c, "START_DATE_REQUIRED", "start_date is required")
	case errors.Is(err, ErrEndDateRequired):
		return response.BadRequest(c, "END_DATE_REQUIRED", "end_date is required")
	case errors.Is(err, ErrInvalidStartDate):
		return response.BadRequest(c, "INVALID_START_DATE", "start_date must be a valid date in YYYY-MM-DD format")
	case errors.Is(err, ErrInvalidEndDate):
		return response.BadRequest(c, "INVALID_END_DATE", "end_date must be a valid date in YYYY-MM-DD format")
	case errors.Is(err, ErrEndBeforeStart):
		return response.BadRequest(c, "END_BEFORE_START", "end_date cannot be before start_date")
	case errors.Is(err, ErrInvalidTotalDays):
		return response.BadRequest(c, "INVALID_TOTAL_DAYS", "total_days must be greater than 0")
	case errors.Is(err, ErrNotPending):
		return response.Conflict(c, "NOT_PENDING", "Only pending leave requests can be approved or rejected")
	case errors.Is(err, ErrAlreadyCancelled):
		return response.Conflict(c, "ALREADY_CANCELLED", "Leave request is already cancelled")
	default:
		log.Error("leave request error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}
