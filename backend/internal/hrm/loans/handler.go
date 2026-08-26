// backend/internal/hrm/loans/handler.go
package loans

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

// Handler handles HRM loans HTTP endpoints. hrm.loans is scope-tiered
// (00101), so this mirrors payslips.Handler / compensation.Handler.
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
	case errors.Is(err, ErrLoanNotFound):
		return response.NotFound(c, "LOAN_NOT_FOUND", "Loan not found")
	case errors.Is(err, ErrScheduleNotFound):
		return response.NotFound(c, "SCHEDULE_NOT_FOUND", "Loan schedule not found")
	case errors.Is(err, ErrInvalidLoanType):
		return response.BadRequest(c, "INVALID_LOAN_TYPE", err.Error())
	case errors.Is(err, ErrInvalidAmount):
		return response.BadRequest(c, "INVALID_AMOUNT", err.Error())
	case errors.Is(err, ErrInvalidTenure):
		return response.BadRequest(c, "INVALID_TENURE", err.Error())
	case errors.Is(err, ErrWrongLoanStatus):
		return response.Conflict(c, "WRONG_LOAN_STATUS", err.Error())
	case errors.Is(err, ErrAccessDenied):
		return response.Forbidden(c, "RECORD_ACCESS_DENIED", "You do not have access to this record")
	default:
		log.Error("loans: error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}

func (h *Handler) ListLoans(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.loans")
	if err != nil {
		log.Error("loans: ListLoans", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	filter := ListFilter{EmployeeID: c.Query("employee_id"), Scope: scopeTier, CallerUserID: userID}
	if limit, err := strconv.Atoi(c.Query("limit", "")); err == nil {
		filter.Limit = limit
	}
	if offset, err := strconv.Atoi(c.Query("offset", "")); err == nil {
		filter.Offset = offset
	}
	res, err := h.service.ListLoans(c.Context(), orgID, filter)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "OK")
}

func (h *Handler) GetLoan(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	l, err := h.service.GetLoan(c.Context(), orgID, c.Params("loanId"))
	if err != nil {
		return h.err(c, err)
	}
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.loans")
	if err != nil {
		log.Error("loans: GetLoan", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	allowed, err := h.scopeResolver.AuthorizeRecordAccess(c.Context(), scopeTier, orgID, userID, l.EmployeeID)
	if err != nil {
		log.Error("loans: GetLoan", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	if !allowed {
		return h.err(c, ErrAccessDenied)
	}
	return response.OK(c, fiber.Map{"loan": l}, "OK")
}

func (h *Handler) CreateLoan(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateLoanRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	l, err := h.service.CreateLoan(c.Context(), orgID, userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"loan": l}, "Loan created")
}

func (h *Handler) SubmitLoan(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	l, err := h.service.SubmitLoan(c.Context(), orgID, c.Params("loanId"), userID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"loan": l}, "Loan submitted for approval")
}

func (h *Handler) DisburseLoan(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	l, err := h.service.DisburseLoan(c.Context(), orgID, c.Params("loanId"), userID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"loan": l}, "Loan disbursed")
}

func (h *Handler) ListSchedule(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	rows, err := h.service.ListSchedule(c.Context(), orgID, c.Params("loanId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"schedule": rows}, "OK")
}

func (h *Handler) ForecloseLoan(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req ForecloseLoanRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	l, err := h.service.ForecloseLoan(c.Context(), orgID, c.Params("loanId"), userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"loan": l}, "Loan foreclosed")
}
