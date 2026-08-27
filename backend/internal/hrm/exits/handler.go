// backend/internal/hrm/exits/handler.go
package exits

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

// Handler serves exit management's HTTP endpoints.
//
// It resolves the caller's scope tier and manage permission once per request
// and hands both to the service on a Caller value — the hrm/performance
// shape. The service holds no authz.Service of its own, so there is exactly
// one place tier resolution happens.
type Handler struct {
	service       Service
	authz         authz.Service
	scopeResolver *scope.Resolver
}

func NewHandler(service Service, authzSvc authz.Service, scopeResolver *scope.Resolver) *Handler {
	return &Handler{service: service, authz: authzSvc, scopeResolver: scopeResolver}
}

func orgID(c fiber.Ctx) string { return c.Params("orgId") }

var errUnauthenticated = errors.New("unauthenticated")

// caller resolves identity, scope tier and manage authority in one place.
// ⚠ ResolveScope takes the FULL dotted resource ("hrm.exits") — authz builds
// its permission key as resource+"."+action, and a bare name denies
// everything silently and uniformly.
func (h *Handler) caller(c fiber.Ctx) (Caller, error) {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return Caller{}, errUnauthenticated
	}
	tier, err := h.authz.ResolveScope(c.Context(), userID, orgID(c), "hrm.exits")
	if err != nil {
		return Caller{}, err
	}
	canManage, err := h.authz.Can(c.Context(), userID, orgID(c), "hrm.exits", "manage")
	if err != nil {
		return Caller{}, err
	}
	return Caller{UserID: userID, Scope: tier, CanManage: canManage}, nil
}

func mapError(c fiber.Ctx, log *slog.Logger, op string, err error) error {
	switch {
	case errors.Is(err, ErrExitNotFound):
		return response.NotFound(c, "EXIT_NOT_FOUND", "Exit record not found")
	case errors.Is(err, ErrClearanceItemNotFound):
		return response.NotFound(c, "CLEARANCE_ITEM_NOT_FOUND", "Clearance item not found")
	case errors.Is(err, ErrSourceNotFound):
		return response.NotFound(c, "SOURCE_NOT_FOUND", "The referenced resignation or termination does not exist")
	case errors.Is(err, ErrSourceMismatch):
		return response.BadRequest(c, "SOURCE_MISMATCH", "The referenced decision record belongs to a different employee")
	case errors.Is(err, ErrEmployeeRequired):
		return response.BadRequest(c, "EMPLOYEE_REQUIRED", "employee_id is required")
	case errors.Is(err, ErrInvalidSourceType):
		return response.BadRequest(c, "INVALID_SOURCE_TYPE", "source_type must be one of resignation, termination")
	case errors.Is(err, ErrInvalidAmount):
		return response.BadRequest(c, "INVALID_AMOUNT", "blocking_amount must be a non-negative decimal")
	case errors.Is(err, ErrInvalidGratuityRule):
		return response.BadRequest(c, "INVALID_GRATUITY_RULE",
			"A gratuity rule needs a positive days_per_year, a non-negative minimum service period, a positive divisor and a valid effective_date")
	case errors.Is(err, ErrNameRequired):
		return response.BadRequest(c, "NAME_REQUIRED", "name is required")
	case errors.Is(err, ErrInvalidRehireStatus):
		return response.BadRequest(c, "INVALID_REHIRE_STATUS", "status must be one of eligible, not_eligible, conditional")
	case errors.Is(err, ErrExitAlreadyOpen):
		return response.Conflict(c, "EXIT_ALREADY_OPEN", "This employee already has an exit in progress")
	case errors.Is(err, ErrAlreadyResolved):
		return response.Conflict(c, "ALREADY_RESOLVED", "Clearance item is already resolved")
	case errors.Is(err, ErrWrongStatus):
		return response.Conflict(c, "WRONG_STATUS", "Not allowed in the exit's current status")
	case errors.Is(err, ErrAccessDenied):
		return response.Forbidden(c, "ACCESS_DENIED", "You do not have access to this resource")
	case errors.Is(err, errUnauthenticated):
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	default:
		log.Error("exits: "+op, slog.Any("error", err))
		return response.InternalServerError(c)
	}
}

func atoiOr(raw string, fallback int) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return v
}

// ============================================================
// Exits
// ============================================================

// CreateExit handles POST /organizations/:orgId/hrm/exits
func (h *Handler) CreateExit(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "CreateExit", err)
	}
	var req CreateExitRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	e, err := h.service.Create(c.Context(), orgID(c), caller, req)
	if err != nil {
		return mapError(c, log, "CreateExit", err)
	}
	return response.Created(c, fiber.Map{"exit": e}, "Exit record created")
}

// ListExits handles GET /organizations/:orgId/hrm/exits
func (h *Handler) ListExits(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "ListExits", err)
	}
	f := ListFilter{
		Status:     strings.TrimSpace(c.Query("status")),
		EmployeeID: strings.TrimSpace(c.Query("employee_id")),
		Limit:      atoiOr(c.Query("limit"), 0),
		Offset:     atoiOr(c.Query("offset"), 0),
	}
	res, err := h.service.List(c.Context(), orgID(c), caller, f)
	if err != nil {
		return mapError(c, log, "ListExits", err)
	}
	return response.OK(c, res, "OK")
}

// GetExit handles GET /organizations/:orgId/hrm/exits/:exitId
func (h *Handler) GetExit(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "GetExit", err)
	}
	e, err := h.service.Get(c.Context(), orgID(c), caller, c.Params("exitId"))
	if err != nil {
		return mapError(c, log, "GetExit", err)
	}
	return response.OK(c, fiber.Map{"exit": e}, "OK")
}

// UpdateExit handles PATCH /organizations/:orgId/hrm/exits/:exitId
func (h *Handler) UpdateExit(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "UpdateExit", err)
	}
	var req UpdateExitRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	e, err := h.service.Update(c.Context(), orgID(c), caller, c.Params("exitId"), req)
	if err != nil {
		return mapError(c, log, "UpdateExit", err)
	}
	return response.OK(c, fiber.Map{"exit": e}, "Exit record updated")
}

// CancelExit handles POST /organizations/:orgId/hrm/exits/:exitId/cancel
func (h *Handler) CancelExit(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "CancelExit", err)
	}
	e, err := h.service.Cancel(c.Context(), orgID(c), caller, c.Params("exitId"))
	if err != nil {
		return mapError(c, log, "CancelExit", err)
	}
	return response.OK(c, fiber.Map{"exit": e}, "Exit record cancelled")
}

// StartClearance handles POST /organizations/:orgId/hrm/exits/:exitId/clearance/start
func (h *Handler) StartClearance(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "StartClearance", err)
	}
	e, err := h.service.StartClearance(c.Context(), orgID(c), caller, c.Params("exitId"))
	if err != nil {
		return mapError(c, log, "StartClearance", err)
	}
	return response.OK(c, fiber.Map{"exit": e}, "Clearance started")
}

// ============================================================
// Clearance items
// ============================================================

// ListClearanceItems handles GET /organizations/:orgId/hrm/exits/:exitId/clearance
func (h *Handler) ListClearanceItems(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "ListClearanceItems", err)
	}
	items, err := h.service.ListClearanceItems(c.Context(), orgID(c), caller, c.Params("exitId"))
	if err != nil {
		return mapError(c, log, "ListClearanceItems", err)
	}
	return response.OK(c, fiber.Map{"clearance_items": items}, "OK")
}

// AddClearanceItem handles POST /organizations/:orgId/hrm/exits/:exitId/clearance
func (h *Handler) AddClearanceItem(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "AddClearanceItem", err)
	}
	var req CreateClearanceItemRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	item, err := h.service.AddClearanceItem(c.Context(), orgID(c), caller, c.Params("exitId"), req)
	if err != nil {
		return mapError(c, log, "AddClearanceItem", err)
	}
	return response.Created(c, fiber.Map{"clearance_item": item}, "Clearance item added")
}

// ResolveClearanceItem handles POST /organizations/:orgId/hrm/exits/:exitId/clearance/:itemId/resolve
func (h *Handler) ResolveClearanceItem(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "ResolveClearanceItem", err)
	}
	var req ResolveClearanceItemRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	item, err := h.service.ResolveClearanceItem(c.Context(), orgID(c), caller,
		c.Params("exitId"), c.Params("itemId"), req)
	if err != nil {
		return mapError(c, log, "ResolveClearanceItem", err)
	}
	return response.OK(c, fiber.Map{"clearance_item": item}, "Clearance item resolved")
}

// ============================================================
// Rehire eligibility
// ============================================================

// GetRehire handles GET /organizations/:orgId/hrm/exits/rehire/:employeeId
func (h *Handler) GetRehire(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "GetRehire", err)
	}
	re, err := h.service.GetRehire(c.Context(), orgID(c), caller, c.Params("employeeId"))
	if err != nil {
		return mapError(c, log, "GetRehire", err)
	}
	return response.OK(c, fiber.Map{"rehire_eligibility": re}, "OK")
}

// DecideRehire handles PUT /organizations/:orgId/hrm/exits/rehire/:employeeId
func (h *Handler) DecideRehire(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "DecideRehire", err)
	}
	var req DecideRehireRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	re, err := h.service.DecideRehire(c.Context(), orgID(c), caller, c.Params("employeeId"), req)
	if err != nil {
		return mapError(c, log, "DecideRehire", err)
	}
	return response.OK(c, fiber.Map{"rehire_eligibility": re}, "Rehire eligibility recorded")
}

// ============================================================
// Settlement (9B)
// ============================================================

// AttachFnFRun handles POST /organizations/:orgId/hrm/exits/:exitId/settlement/run
func (h *Handler) AttachFnFRun(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "AttachFnFRun", err)
	}
	var req struct {
		PayslipRunID string `json:"payslip_run_id"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	if strings.TrimSpace(req.PayslipRunID) == "" {
		return response.BadRequest(c, "RUN_REQUIRED", "payslip_run_id is required")
	}
	e, err := h.service.AttachFnFRun(c.Context(), orgID(c), caller, c.Params("exitId"), req.PayslipRunID)
	if err != nil {
		return mapError(c, log, "AttachFnFRun", err)
	}
	return response.OK(c, fiber.Map{"exit": e}, "Settlement run attached")
}

// ListSettlementLines handles GET /organizations/:orgId/hrm/exits/:exitId/settlement
func (h *Handler) ListSettlementLines(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "ListSettlementLines", err)
	}
	lines, err := h.service.ListSettlementLines(c.Context(), orgID(c), caller, c.Params("exitId"))
	if err != nil {
		return mapError(c, log, "ListSettlementLines", err)
	}
	return response.OK(c, fiber.Map{"settlement_lines": lines}, "OK")
}

// ============================================================
// Gratuity rules
// ============================================================

// ListGratuityRules handles GET /organizations/:orgId/hrm/exits/gratuity-rules
func (h *Handler) ListGratuityRules(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "ListGratuityRules", err)
	}
	rules, err := h.service.ListGratuityRules(c.Context(), orgID(c), caller)
	if err != nil {
		return mapError(c, log, "ListGratuityRules", err)
	}
	return response.OK(c, fiber.Map{"gratuity_rules": rules}, "OK")
}

// CreateGratuityRule handles POST /organizations/:orgId/hrm/exits/gratuity-rules
func (h *Handler) CreateGratuityRule(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "CreateGratuityRule", err)
	}
	var req CreateGratuityRuleRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	rule, err := h.service.CreateGratuityRule(c.Context(), orgID(c), caller, req)
	if err != nil {
		return mapError(c, log, "CreateGratuityRule", err)
	}
	return response.Created(c, fiber.Map{"gratuity_rule": rule}, "Gratuity rule created")
}
