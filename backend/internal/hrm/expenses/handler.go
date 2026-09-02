// backend/internal/hrm/expenses/handler.go
package expenses

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

// Handler handles HRM travel and expense HTTP endpoints. Both hrm.travel and
// hrm.expenses are scope-tiered (00109).
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
	case errors.Is(err, ErrTravelNotFound):
		return response.NotFound(c, "TRAVEL_NOT_FOUND", "Travel request not found")
	case errors.Is(err, ErrItineraryNotFound):
		return response.NotFound(c, "ITINERARY_NOT_FOUND", "Itinerary item not found")
	case errors.Is(err, ErrAdvanceNotFound):
		return response.NotFound(c, "ADVANCE_NOT_FOUND", "Travel advance not found")
	case errors.Is(err, ErrClaimNotFound):
		return response.NotFound(c, "CLAIM_NOT_FOUND", "Expense claim not found")
	case errors.Is(err, ErrLineNotFound):
		return response.NotFound(c, "LINE_NOT_FOUND", "Expense line not found")
	case errors.Is(err, ErrPolicyNotFound):
		return response.NotFound(c, "POLICY_NOT_FOUND", "Expense policy not found")
	case errors.Is(err, ErrInvalidAmount):
		return response.BadRequest(c, "INVALID_AMOUNT", err.Error())
	case errors.Is(err, ErrInvalidCategory):
		return response.BadRequest(c, "INVALID_CATEGORY", err.Error())
	case errors.Is(err, ErrInvalidItemType):
		return response.BadRequest(c, "INVALID_ITEM_TYPE", err.Error())
	case errors.Is(err, ErrInvalidDateRange):
		return response.BadRequest(c, "INVALID_DATE_RANGE", err.Error())
	case errors.Is(err, ErrInvalidExchangeRate):
		return response.BadRequest(c, "INVALID_EXCHANGE_RATE", err.Error())
	case errors.Is(err, ErrApprovedExceedsClaimed):
		return response.BadRequest(c, "APPROVED_EXCEEDS_CLAIMED", err.Error())
	case errors.Is(err, ErrClaimHasNoLines):
		return response.Conflict(c, "CLAIM_HAS_NO_LINES", err.Error())
	case errors.Is(err, ErrLinesUndecided):
		return response.Conflict(c, "LINES_UNDECIDED", err.Error())
	case errors.Is(err, ErrAlreadySettled):
		return response.Conflict(c, "ALREADY_SETTLED", err.Error())
	case errors.Is(err, ErrWrongStatus):
		return response.Conflict(c, "WRONG_STATUS", err.Error())
	case errors.Is(err, ErrAccessDenied):
		return response.Forbidden(c, "RECORD_ACCESS_DENIED", "You do not have access to this record")
	default:
		log.Error("expenses: error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}

// resolveFilter builds a scope-aware filter against the given resource —
// hrm.travel and hrm.expenses are separate resources with separate tiers.
func (h *Handler) resolveFilter(c fiber.Ctx, resource string) (ListFilter, string, error) {
	userID, ok := requestUser(c)
	if !ok {
		return ListFilter{}, "", errors.New("unauthenticated")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return ListFilter{}, "", errors.New("no org context")
	}
	tier, err := h.authz.ResolveScope(c.Context(), userID, orgID, resource)
	if err != nil {
		return ListFilter{}, "", err
	}
	f := ListFilter{
		EmployeeID: c.Query("employee_id"), Status: c.Query("status"),
		Scope: tier, CallerUserID: userID,
	}
	if limit, err := strconv.Atoi(c.Query("limit", "")); err == nil {
		f.Limit = limit
	}
	if offset, err := strconv.Atoi(c.Query("offset", "")); err == nil {
		f.Offset = offset
	}
	return f, orgID, nil
}

// authorizeRecord narrows a single-record read to the caller's tier.
func (h *Handler) authorizeRecord(c fiber.Ctx, resource, employeeID string) error {
	userID, _ := requestUser(c)
	orgID, _ := requestOrg(c)
	tier, err := h.authz.ResolveScope(c.Context(), userID, orgID, resource)
	if err != nil {
		return err
	}
	allowed, err := h.scopeResolver.AuthorizeRecordAccess(c.Context(), tier, orgID, userID, employeeID)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrAccessDenied
	}
	return nil
}

// ── Config ───────────────────────────────────────────────────────────────────

func (h *Handler) ListPolicies(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	list, err := h.service.ListPolicies(c.Context(), orgID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"policies": list}, "OK")
}

func (h *Handler) CreatePolicy(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, _ := requestOrg(c)
	var req CreatePolicyRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	p, err := h.service.CreatePolicy(c.Context(), orgID, userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"policy": p}, "Expense policy created")
}

func (h *Handler) ListPerDiemRates(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	list, err := h.service.ListPerDiemRates(c.Context(), orgID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"per_diem_rates": list}, "OK")
}

func (h *Handler) CreatePerDiemRate(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, _ := requestOrg(c)
	var req CreatePerDiemRateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	r, err := h.service.CreatePerDiemRate(c.Context(), orgID, userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"per_diem_rate": r}, "Per-diem rate created")
}

func (h *Handler) ListMileageRates(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	list, err := h.service.ListMileageRates(c.Context(), orgID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"mileage_rates": list}, "OK")
}

func (h *Handler) CreateMileageRate(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, _ := requestOrg(c)
	var req CreateMileageRateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	m, err := h.service.CreateMileageRate(c.Context(), orgID, userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"mileage_rate": m}, "Mileage rate created")
}

// ── Travel ───────────────────────────────────────────────────────────────────

func (h *Handler) ListTravel(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	filter, orgID, err := h.resolveFilter(c, "hrm.travel")
	if err != nil {
		log.Error("expenses: ListTravel", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	res, err := h.service.ListTravel(c.Context(), orgID, filter)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "OK")
}

func (h *Handler) GetTravel(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	t, err := h.service.GetTravel(c.Context(), orgID, c.Params("travelId"))
	if err != nil {
		return h.err(c, err)
	}
	if err := h.authorizeRecord(c, "hrm.travel", t.EmployeeID); err != nil {
		if errors.Is(err, ErrAccessDenied) {
			return h.err(c, err)
		}
		log.Error("expenses: GetTravel", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"travel_request": t}, "OK")
}

func (h *Handler) CreateTravel(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, _ := requestOrg(c)
	var req CreateTravelRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	t, err := h.service.CreateTravel(c.Context(), orgID, userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"travel_request": t}, "Travel request created")
}

func (h *Handler) SubmitTravel(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, _ := requestOrg(c)
	t, err := h.service.SubmitTravel(c.Context(), orgID, c.Params("travelId"), userID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"travel_request": t}, "Travel request submitted")
}

func (h *Handler) ListItinerary(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	list, err := h.service.ListItinerary(c.Context(), orgID, c.Params("travelId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"itinerary": list}, "OK")
}

func (h *Handler) AddItineraryItem(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateItineraryItemRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	i, err := h.service.AddItineraryItem(c.Context(), orgID, c.Params("travelId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"itinerary_item": i}, "Itinerary item added")
}

// ── Advances ─────────────────────────────────────────────────────────────────

func (h *Handler) ListAdvances(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	filter, orgID, err := h.resolveFilter(c, "hrm.travel")
	if err != nil {
		log.Error("expenses: ListAdvances", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	res, err := h.service.ListAdvances(c.Context(), orgID, filter)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "OK")
}

func (h *Handler) CreateAdvance(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, _ := requestOrg(c)
	var req CreateAdvanceRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	a, err := h.service.CreateAdvance(c.Context(), orgID, userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"advance": a}, "Travel advance created")
}

func (h *Handler) DisburseAdvance(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, _ := requestOrg(c)
	a, err := h.service.DisburseAdvance(c.Context(), orgID, c.Params("advanceId"), userID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"advance": a}, "Travel advance disbursed")
}

// ── Claims ───────────────────────────────────────────────────────────────────

func (h *Handler) ListClaims(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	filter, orgID, err := h.resolveFilter(c, "hrm.expenses")
	if err != nil {
		log.Error("expenses: ListClaims", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	res, err := h.service.ListClaims(c.Context(), orgID, filter)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "OK")
}

func (h *Handler) GetClaim(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	cl, err := h.service.GetClaim(c.Context(), orgID, c.Params("claimId"))
	if err != nil {
		return h.err(c, err)
	}
	if err := h.authorizeRecord(c, "hrm.expenses", cl.EmployeeID); err != nil {
		if errors.Is(err, ErrAccessDenied) {
			return h.err(c, err)
		}
		log.Error("expenses: GetClaim", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"claim": cl}, "OK")
}

func (h *Handler) CreateClaim(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, _ := requestOrg(c)
	var req CreateClaimRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	cl, err := h.service.CreateClaim(c.Context(), orgID, userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"claim": cl}, "Expense claim created")
}

func (h *Handler) AddLine(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateLineRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	l, err := h.service.AddLine(c.Context(), orgID, c.Params("claimId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"line": l}, "Expense line added")
}

func (h *Handler) SubmitClaim(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, _ := requestOrg(c)
	cl, err := h.service.SubmitClaim(c.Context(), orgID, c.Params("claimId"), userID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"claim": cl}, "Expense claim submitted")
}

// ApproveLine is the line-level approval endpoint — the one this module is
// shaped around.
func (h *Handler) ApproveLine(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req ApproveLineRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	cl, err := h.service.ApproveLine(c.Context(), orgID, c.Params("claimId"), c.Params("lineId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"claim": cl}, "Expense line decided")
}

func (h *Handler) SettleClaim(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, _ := requestOrg(c)
	res, err := h.service.SettleClaim(c.Context(), orgID, c.Params("claimId"), userID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "Expense claim settled")
}
