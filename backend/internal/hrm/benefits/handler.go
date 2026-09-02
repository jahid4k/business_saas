// backend/internal/hrm/benefits/handler.go
package benefits

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

// Handler handles HRM benefits HTTP endpoints. hrm.benefit_enrollments is
// scope-tiered (00105), so this mirrors compensation/loans/reimbursements'
// Handler shape.
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
	case errors.Is(err, ErrPlanNotFound):
		return response.NotFound(c, "PLAN_NOT_FOUND", "Benefit plan not found")
	case errors.Is(err, ErrTierNotFound):
		return response.NotFound(c, "TIER_NOT_FOUND", "Benefit tier not found")
	case errors.Is(err, ErrEnrollmentNotFound):
		return response.NotFound(c, "ENROLLMENT_NOT_FOUND", "Enrollment not found")
	case errors.Is(err, ErrDependentNotFound):
		return response.NotFound(c, "DEPENDENT_NOT_FOUND", "Dependent not found")
	case errors.Is(err, ErrInvalidPlanType):
		return response.BadRequest(c, "INVALID_PLAN_TYPE", err.Error())
	case errors.Is(err, ErrInvalidWindowType):
		return response.BadRequest(c, "INVALID_WINDOW_TYPE", err.Error())
	case errors.Is(err, ErrInvalidRelationship):
		return response.BadRequest(c, "INVALID_RELATIONSHIP", err.Error())
	case errors.Is(err, ErrInvalidAmount):
		return response.BadRequest(c, "INVALID_AMOUNT", err.Error())
	case errors.Is(err, ErrAlreadyEnrolled):
		return response.Conflict(c, "ALREADY_ENROLLED", err.Error())
	case errors.Is(err, ErrWrongStatus):
		return response.Conflict(c, "WRONG_STATUS", err.Error())
	case errors.Is(err, ErrAccessDenied):
		return response.Forbidden(c, "RECORD_ACCESS_DENIED", "You do not have access to this record")
	default:
		log.Error("benefits: error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}

func (h *Handler) ListPlans(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	list, err := h.service.ListPlans(c.Context(), orgID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"plans": list}, "OK")
}

func (h *Handler) CreatePlan(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreatePlanRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	p, err := h.service.CreatePlan(c.Context(), orgID, userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"plan": p}, "Plan created")
}

func (h *Handler) ListTiers(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	list, err := h.service.ListTiers(c.Context(), orgID, c.Params("planId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"tiers": list}, "OK")
}

func (h *Handler) CreateTier(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateTierRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	t, err := h.service.CreateTier(c.Context(), orgID, c.Params("planId"), userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"tier": t}, "Tier created")
}

func (h *Handler) ListEnrollments(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.benefit_enrollments")
	if err != nil {
		log.Error("benefits: ListEnrollments", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	filter := ListFilter{EmployeeID: c.Query("employee_id"), Scope: scopeTier, CallerUserID: userID}
	if limit, err := strconv.Atoi(c.Query("limit", "")); err == nil {
		filter.Limit = limit
	}
	if offset, err := strconv.Atoi(c.Query("offset", "")); err == nil {
		filter.Offset = offset
	}
	res, err := h.service.ListEnrollments(c.Context(), orgID, filter)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "OK")
}

func (h *Handler) GetEnrollment(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	e, err := h.service.GetEnrollment(c.Context(), orgID, c.Params("enrollmentId"))
	if err != nil {
		return h.err(c, err)
	}
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.benefit_enrollments")
	if err != nil {
		log.Error("benefits: GetEnrollment", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	allowed, err := h.scopeResolver.AuthorizeRecordAccess(c.Context(), scopeTier, orgID, userID, e.EmployeeID)
	if err != nil {
		log.Error("benefits: GetEnrollment", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	if !allowed {
		return h.err(c, ErrAccessDenied)
	}
	return response.OK(c, fiber.Map{"enrollment": e}, "OK")
}

// EnrollSelf enrolls the caller's OWN employee record — enroll_self cannot,
// by itself, enroll anyone else. Service.EnrollSelf resolves the caller's
// own employeeID from userID before creating anything.
func (h *Handler) EnrollSelf(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateEnrollmentRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	e, err := h.service.EnrollSelf(c.Context(), orgID, userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"enrollment": e}, "Enrolled")
}

func (h *Handler) WaiveEnrollment(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	e, err := h.service.WaiveEnrollment(c.Context(), orgID, c.Params("enrollmentId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"enrollment": e}, "Enrollment waived")
}

func (h *Handler) ListDependents(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	list, err := h.service.ListDependents(c.Context(), orgID, c.Query("employee_id"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"dependents": list}, "OK")
}

func (h *Handler) CreateDependent(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateDependentRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	d, err := h.service.CreateDependent(c.Context(), orgID, userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"dependent": d}, "Dependent created")
}

func (h *Handler) VerifyDependent(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	d, err := h.service.VerifyDependent(c.Context(), orgID, c.Params("dependentId"), userID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"dependent": d}, "Dependent verified")
}
