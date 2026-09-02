// backend/internal/hrm/reimbursements/handler.go
package reimbursements

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

func requestUser(c fiber.Ctx) (string, bool) { return middleware.UserIDFromCtx(c) }
func requestOrg(c fiber.Ctx) (string, bool)  { return middleware.OrganizationIDFromCtx(c) }

func (h *Handler) err(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrNotFound):
		return response.NotFound(c, "REIMBURSEMENT_NOT_FOUND", "Reimbursement not found")
	case errors.Is(err, ErrInvalidCategory):
		return response.BadRequest(c, "INVALID_CATEGORY", err.Error())
	case errors.Is(err, ErrInvalidAmount):
		return response.BadRequest(c, "INVALID_AMOUNT", err.Error())
	case errors.Is(err, ErrWrongStatus):
		return response.Conflict(c, "WRONG_STATUS", err.Error())
	case errors.Is(err, ErrAccessDenied):
		return response.Forbidden(c, "RECORD_ACCESS_DENIED", "You do not have access to this record")
	default:
		log.Error("reimbursements: error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}

func (h *Handler) List(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.reimbursements")
	if err != nil {
		log.Error("reimbursements: List", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	filter := ListFilter{EmployeeID: c.Query("employee_id"), Scope: scopeTier, CallerUserID: userID}
	if limit, err := strconv.Atoi(c.Query("limit", "")); err == nil {
		filter.Limit = limit
	}
	if offset, err := strconv.Atoi(c.Query("offset", "")); err == nil {
		filter.Offset = offset
	}
	res, err := h.service.List(c.Context(), orgID, filter)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "OK")
}

func (h *Handler) Get(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	r, err := h.service.Get(c.Context(), orgID, c.Params("reimbursementId"))
	if err != nil {
		return h.err(c, err)
	}
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.reimbursements")
	if err != nil {
		log.Error("reimbursements: Get", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	allowed, err := h.scopeResolver.AuthorizeRecordAccess(c.Context(), scopeTier, orgID, userID, r.EmployeeID)
	if err != nil {
		log.Error("reimbursements: Get", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	if !allowed {
		return h.err(c, ErrAccessDenied)
	}
	return response.OK(c, fiber.Map{"reimbursement": r}, "OK")
}

func (h *Handler) Create(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	r, err := h.service.Create(c.Context(), orgID, userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"reimbursement": r}, "Reimbursement created")
}

func (h *Handler) Submit(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	r, err := h.service.Submit(c.Context(), orgID, c.Params("reimbursementId"), userID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"reimbursement": r}, "Reimbursement submitted for approval")
}

func (h *Handler) Cancel(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	r, err := h.service.Cancel(c.Context(), orgID, c.Params("reimbursementId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"reimbursement": r}, "Reimbursement cancelled")
}
