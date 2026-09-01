// backend/internal/hrm/succession/handler.go
package succession

import (
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler serves succession's HTTP endpoints.
type Handler struct {
	service Service
	authz   authz.Service
}

func NewHandler(service Service, authzSvc authz.Service) *Handler {
	return &Handler{service: service, authz: authzSvc}
}

func orgID(c fiber.Ctx) string { return c.Params("orgId") }

var errUnauthenticated = errors.New("unauthenticated")

// caller resolves identity and the three authorities this package
// distinguishes.
//
// ⚠ Can takes the FULL dotted resource — authz builds its key as
// resource+"."+action, and a bare name denies everything silently (8C).
//
// CanViewConfidential is resolved here as well as gated on the route so the
// service can re-check it. A route gate protects the HTTP path only; the
// service check is what protects a future scheduler or report generator.
func (h *Handler) caller(c fiber.Ctx) (Caller, error) {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return Caller{}, errUnauthenticated
	}
	org := orgID(c)
	canManage, err := h.authz.Can(c.Context(), userID, org, "hrm.succession", "manage")
	if err != nil {
		return Caller{}, err
	}
	canConfidential, err := h.authz.Can(c.Context(), userID, org, "hrm.succession", "view_confidential")
	if err != nil {
		return Caller{}, err
	}
	canManagePlans, err := h.authz.Can(c.Context(), userID, org, "hrm.development_plan", "manage")
	if err != nil {
		return Caller{}, err
	}
	return Caller{
		UserID:              userID,
		CanManage:           canManage,
		CanViewConfidential: canConfidential,
		CanManagePlans:      canManagePlans,
	}, nil
}

func mapError(c fiber.Ctx, log *slog.Logger, op string, err error) error {
	switch {
	case errors.Is(err, ErrCriticalNotFound):
		return response.NotFound(c, "CRITICAL_POSITION_NOT_FOUND", "Critical position not found")
	case errors.Is(err, ErrCandidateNotFound):
		return response.NotFound(c, "CANDIDATE_NOT_FOUND", "Succession candidate not found")
	case errors.Is(err, ErrPlanNotFound):
		return response.NotFound(c, "PLAN_NOT_FOUND", "Development plan not found")
	case errors.Is(err, ErrItemNotFound):
		return response.NotFound(c, "PLAN_ITEM_NOT_FOUND", "Development plan item not found")
	case errors.Is(err, ErrEmployeeNotFound):
		return response.NotFound(c, "EMPLOYEE_NOT_FOUND", "Employee not found in this organization")
	case errors.Is(err, ErrPositionNotFound):
		return response.NotFound(c, "POSITION_NOT_FOUND", "Position not found in this organization")
	case errors.Is(err, ErrNoEmployeeRecord):
		return response.NotFound(c, "NO_EMPLOYEE_RECORD",
			"You have no employee record in this organization")
	case errors.Is(err, ErrAlreadyDesignated):
		return response.Conflict(c, "ALREADY_DESIGNATED",
			"This position is already designated critical; retire the existing designation first")
	case errors.Is(err, ErrAlreadyNominated):
		return response.Conflict(c, "ALREADY_NOMINATED",
			"This employee is already an active candidate for this position")
	case errors.Is(err, ErrAlreadyWithdrawn):
		return response.Conflict(c, "ALREADY_WITHDRAWN", "This nomination is no longer active")
	case errors.Is(err, ErrInvalidBand):
		return response.BadRequest(c, "INVALID_BAND",
			"performance_band and potential_band must each be low, medium or high")
	case errors.Is(err, ErrRationaleRequired):
		return response.BadRequest(c, "RATIONALE_REQUIRED",
			"A potential band must state its rationale — an unexplained rating is not an assessment")
	case errors.Is(err, ErrInvalidReadiness):
		return response.BadRequest(c, "INVALID_READINESS",
			"readiness must be ready_now, ready_1_2_years, ready_3_5_years or emergency_cover")
	case errors.Is(err, ErrInvalidCriticality):
		return response.BadRequest(c, "INVALID_CRITICALITY",
			"criticality_level must be mission_critical, high or moderate and vacancy_risk high, medium or low")
	case errors.Is(err, ErrTitleRequired):
		return response.BadRequest(c, "TITLE_REQUIRED", "A development plan needs a title")
	case errors.Is(err, ErrDescriptionRequired):
		return response.BadRequest(c, "DESCRIPTION_REQUIRED", "A development plan item needs a description")
	case errors.Is(err, ErrInvalidStatus):
		return response.BadRequest(c, "INVALID_STATUS", "Invalid status value")
	case errors.Is(err, ErrAccessDenied):
		return response.Forbidden(c, "ACCESS_DENIED", "You do not have access to this resource")
	case errors.Is(err, errUnauthenticated):
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	default:
		log.Error("succession: "+op, slog.Any("error", err))
		return response.InternalServerError(c)
	}
}

// ============================================================
// Critical positions
// ============================================================

// ListCriticalPositions handles GET /organizations/:orgId/hrm/succession/critical-positions
func (h *Handler) ListCriticalPositions(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "ListCriticalPositions", err)
	}
	list, err := h.service.ListCriticalPositions(c.Context(), orgID(c), caller, c.Query("active") != "false")
	if err != nil {
		return mapError(c, log, "ListCriticalPositions", err)
	}
	return response.OK(c, fiber.Map{"critical_positions": list}, "OK")
}

// CreateCriticalPosition handles POST /organizations/:orgId/hrm/succession/critical-positions
func (h *Handler) CreateCriticalPosition(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "CreateCriticalPosition", err)
	}
	var req CreateCriticalPositionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	cp, err := h.service.CreateCriticalPosition(c.Context(), orgID(c), caller, req)
	if err != nil {
		return mapError(c, log, "CreateCriticalPosition", err)
	}
	return response.Created(c, fiber.Map{"critical_position": cp}, "Critical position designated")
}

// UpdateCriticalPosition handles PATCH /organizations/:orgId/hrm/succession/critical-positions/:cpId
func (h *Handler) UpdateCriticalPosition(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "UpdateCriticalPosition", err)
	}
	var req UpdateCriticalPositionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	cp, err := h.service.UpdateCriticalPosition(c.Context(), orgID(c), caller, c.Params("cpId"), req)
	if err != nil {
		return mapError(c, log, "UpdateCriticalPosition", err)
	}
	return response.OK(c, fiber.Map{"critical_position": cp}, "Critical position updated")
}

// ============================================================
// Candidates and assessments — CONFIDENTIAL
// ============================================================

// ListCandidates handles GET .../critical-positions/:cpId/candidates
func (h *Handler) ListCandidates(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "ListCandidates", err)
	}
	list, err := h.service.ListCandidates(c.Context(), orgID(c), caller,
		c.Params("cpId"), c.Query("active") != "false")
	if err != nil {
		return mapError(c, log, "ListCandidates", err)
	}
	return response.OK(c, fiber.Map{"candidates": list}, "OK")
}

// Nominate handles POST .../critical-positions/:cpId/candidates
func (h *Handler) Nominate(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "Nominate", err)
	}
	var req NominateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	cand, err := h.service.Nominate(c.Context(), orgID(c), caller, c.Params("cpId"), req)
	if err != nil {
		return mapError(c, log, "Nominate", err)
	}
	return response.Created(c, fiber.Map{"candidate": cand}, "Successor nominated")
}

// WithdrawNomination handles POST .../candidates/:candId/withdraw
func (h *Handler) WithdrawNomination(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "WithdrawNomination", err)
	}
	var req WithdrawRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	cand, err := h.service.WithdrawNomination(c.Context(), orgID(c), caller, c.Params("candId"), req)
	if err != nil {
		return mapError(c, log, "WithdrawNomination", err)
	}
	return response.OK(c, fiber.Map{"candidate": cand}, "Nomination withdrawn")
}

// RecordAssessment handles POST /organizations/:orgId/hrm/succession/assessments
func (h *Handler) RecordAssessment(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "RecordAssessment", err)
	}
	var req RecordAssessmentRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	a, err := h.service.RecordAssessment(c.Context(), orgID(c), caller, req)
	if err != nil {
		return mapError(c, log, "RecordAssessment", err)
	}
	box, _ := Box(a.PerformanceBand, a.PotentialBand)
	return response.Created(c, fiber.Map{"assessment": a, "nine_box": box}, "Talent assessment recorded")
}

// NineBoxGrid handles GET /organizations/:orgId/hrm/succession/assessments
func (h *Handler) NineBoxGrid(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "NineBoxGrid", err)
	}
	var asOf *time.Time
	if raw := strings.TrimSpace(c.Query("as_of")); raw != "" {
		d, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return response.BadRequest(c, "INVALID_AS_OF", "as_of must be YYYY-MM-DD")
		}
		asOf = &d
	}
	grid, err := h.service.NineBoxGrid(c.Context(), orgID(c), caller, asOf)
	if err != nil {
		return mapError(c, log, "NineBoxGrid", err)
	}
	return response.OK(c, fiber.Map{"grid": grid}, "OK")
}

// ReviewEmployee handles GET .../succession/employees/:employeeId/review
func (h *Handler) ReviewEmployee(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "ReviewEmployee", err)
	}
	view, err := h.service.ReviewEmployee(c.Context(), orgID(c), caller, c.Params("employeeId"))
	if err != nil {
		return mapError(c, log, "ReviewEmployee", err)
	}
	return response.OK(c, fiber.Map{"review": view}, "OK")
}

// ============================================================
// Development plans — SUBJECT-VISIBLE
// ============================================================

// MyDevelopment handles GET /organizations/:orgId/hrm/development-plans/me
func (h *Handler) MyDevelopment(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "MyDevelopment", err)
	}
	view, err := h.service.MyDevelopment(c.Context(), orgID(c), caller)
	if err != nil {
		return mapError(c, log, "MyDevelopment", err)
	}
	return response.OK(c, fiber.Map{"development": view}, "OK")
}

// ListPlans handles GET /organizations/:orgId/hrm/development-plans
func (h *Handler) ListPlans(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "ListPlans", err)
	}
	list, err := h.service.ListPlans(c.Context(), orgID(c), caller, c.Query("employee_id"))
	if err != nil {
		return mapError(c, log, "ListPlans", err)
	}
	return response.OK(c, fiber.Map{"plans": list}, "OK")
}

// CreatePlan handles POST /organizations/:orgId/hrm/development-plans
func (h *Handler) CreatePlan(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "CreatePlan", err)
	}
	var req CreatePlanRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	p, err := h.service.CreatePlan(c.Context(), orgID(c), caller, req)
	if err != nil {
		return mapError(c, log, "CreatePlan", err)
	}
	return response.Created(c, fiber.Map{"plan": p}, "Development plan created")
}

// GetPlan handles GET /organizations/:orgId/hrm/development-plans/:planId
func (h *Handler) GetPlan(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "GetPlan", err)
	}
	p, err := h.service.GetPlan(c.Context(), orgID(c), caller, c.Params("planId"))
	if err != nil {
		return mapError(c, log, "GetPlan", err)
	}
	return response.OK(c, fiber.Map{"plan": p}, "OK")
}

// UpdatePlan handles PATCH /organizations/:orgId/hrm/development-plans/:planId
func (h *Handler) UpdatePlan(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "UpdatePlan", err)
	}
	var req UpdatePlanRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	p, err := h.service.UpdatePlan(c.Context(), orgID(c), caller, c.Params("planId"), req)
	if err != nil {
		return mapError(c, log, "UpdatePlan", err)
	}
	return response.OK(c, fiber.Map{"plan": p}, "Development plan updated")
}

// AddPlanItem handles POST /organizations/:orgId/hrm/development-plans/:planId/items
func (h *Handler) AddPlanItem(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "AddPlanItem", err)
	}
	var req CreateItemRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	it, err := h.service.AddPlanItem(c.Context(), orgID(c), caller, c.Params("planId"), req)
	if err != nil {
		return mapError(c, log, "AddPlanItem", err)
	}
	return response.Created(c, fiber.Map{"item": it}, "Development plan item added")
}

// UpdatePlanItem handles PATCH /organizations/:orgId/hrm/development-plans/items/:itemId
func (h *Handler) UpdatePlanItem(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "UpdatePlanItem", err)
	}
	var req UpdateItemRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	it, err := h.service.UpdatePlanItem(c.Context(), orgID(c), caller, c.Params("itemId"), req)
	if err != nil {
		return mapError(c, log, "UpdatePlanItem", err)
	}
	return response.OK(c, fiber.Map{"item": it}, "Development plan item updated")
}
