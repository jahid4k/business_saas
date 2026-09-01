// backend/internal/hrm/orgchart/handler.go
package orgchart

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler serves the org chart's HTTP endpoints.
//
// It resolves the caller's manage authority once per request and hands it to
// the service on a Caller value. There is no scope tier here — the chart is
// org-wide, deliberately (see the service doc comment and migration 00122).
type Handler struct {
	service Service
	authz   authz.Service
}

func NewHandler(service Service, authzSvc authz.Service) *Handler {
	return &Handler{service: service, authz: authzSvc}
}

func orgID(c fiber.Ctx) string { return c.Params("orgId") }

var errUnauthenticated = errors.New("unauthenticated")

// caller resolves identity and manage authority.
// ⚠ Can takes the FULL dotted resource ("hrm.org_chart") — authz builds its
// key as resource+"."+action, and a bare name denies everything silently.
func (h *Handler) caller(c fiber.Ctx) (Caller, error) {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return Caller{}, errUnauthenticated
	}
	canManage, err := h.authz.Can(c.Context(), userID, orgID(c), "hrm.org_chart", "manage")
	if err != nil {
		return Caller{}, err
	}
	return Caller{UserID: userID, CanManage: canManage}, nil
}

func mapError(c fiber.Ctx, log *slog.Logger, op string, err error) error {
	switch {
	case errors.Is(err, ErrRelationshipNotFound):
		return response.NotFound(c, "RELATIONSHIP_NOT_FOUND", "Reporting relationship not found")
	case errors.Is(err, ErrSeatNotFound):
		return response.NotFound(c, "SEAT_NOT_FOUND", "Position seat not found")
	case errors.Is(err, ErrEmployeeNotFound):
		return response.NotFound(c, "EMPLOYEE_NOT_FOUND", "Employee not found in this organization")
	case errors.Is(err, ErrInvalidType):
		return response.BadRequest(c, "INVALID_RELATIONSHIP_TYPE",
			"relationship_type must be one of solid, dotted, functional, project")
	case errors.Is(err, ErrSelfManagement):
		return response.BadRequest(c, "SELF_MANAGEMENT", "An employee cannot report to themselves")
	case errors.Is(err, ErrWouldCreateCycle):
		return response.Conflict(c, "WOULD_CREATE_CYCLE",
			"This reporting line would create a cycle in the management chain")
	case errors.Is(err, ErrDuplicateSolid):
		return response.Conflict(c, "DUPLICATE_SOLID_LINE",
			"This employee already has an active solid-line manager; end it first")
	case errors.Is(err, ErrAlreadyEnded):
		return response.Conflict(c, "ALREADY_ENDED", "This reporting relationship has already ended")
	case errors.Is(err, ErrSeatOccupied):
		return response.Conflict(c, "SEAT_OCCUPIED", "This employee already occupies another active seat")
	case errors.Is(err, ErrAccessDenied):
		return response.Forbidden(c, "ACCESS_DENIED", "You do not have access to this resource")
	case errors.Is(err, errUnauthenticated):
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	default:
		log.Error("orgchart: "+op, slog.Any("error", err))
		return response.InternalServerError(c)
	}
}

// ============================================================
// Relationships
// ============================================================

// CreateRelationship handles POST /organizations/:orgId/hrm/org-chart/relationships
func (h *Handler) CreateRelationship(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "CreateRelationship", err)
	}
	var req CreateRelationshipRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	rel, err := h.service.CreateRelationship(c.Context(), orgID(c), caller, req)
	if err != nil {
		return mapError(c, log, "CreateRelationship", err)
	}
	return response.Created(c, fiber.Map{"relationship": rel}, "Reporting relationship created")
}

// ListRelationships handles GET /organizations/:orgId/hrm/org-chart/relationships
func (h *Handler) ListRelationships(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "ListRelationships", err)
	}
	list, err := h.service.ListRelationships(c.Context(), orgID(c), caller,
		strings.TrimSpace(c.Query("employee_id")), c.Query("active") != "false")
	if err != nil {
		return mapError(c, log, "ListRelationships", err)
	}
	return response.OK(c, fiber.Map{"relationships": list}, "OK")
}

// EndRelationship handles POST /organizations/:orgId/hrm/org-chart/relationships/:relId/end
func (h *Handler) EndRelationship(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "EndRelationship", err)
	}
	var req EndRelationshipRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	rel, err := h.service.EndRelationship(c.Context(), orgID(c), caller, c.Params("relId"), req)
	if err != nil {
		return mapError(c, log, "EndRelationship", err)
	}
	return response.OK(c, fiber.Map{"relationship": rel}, "Reporting relationship ended")
}

// ============================================================
// Chart
// ============================================================

// GetChart handles GET /organizations/:orgId/hrm/org-chart
func (h *Handler) GetChart(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "GetChart", err)
	}
	nodes, err := h.service.GetChart(c.Context(), orgID(c), caller)
	if err != nil {
		return mapError(c, log, "GetChart", err)
	}
	return response.OK(c, fiber.Map{"nodes": nodes}, "OK")
}

// GetManagementChain handles GET /organizations/:orgId/hrm/org-chart/chain/:employeeId
func (h *Handler) GetManagementChain(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "GetManagementChain", err)
	}
	chain, err := h.service.ManagementChain(c.Context(), orgID(c), caller, c.Params("employeeId"))
	if err != nil {
		return mapError(c, log, "GetManagementChain", err)
	}
	return response.OK(c, fiber.Map{"chain": chain}, "OK")
}

// ============================================================
// Seats
// ============================================================

// CreateSeat handles POST /organizations/:orgId/hrm/org-chart/seats
func (h *Handler) CreateSeat(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "CreateSeat", err)
	}
	var req CreateSeatRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	seat, err := h.service.CreateSeat(c.Context(), orgID(c), caller, req)
	if err != nil {
		return mapError(c, log, "CreateSeat", err)
	}
	return response.Created(c, fiber.Map{"seat": seat}, "Position seat created")
}

// ListSeats handles GET /organizations/:orgId/hrm/org-chart/seats
func (h *Handler) ListSeats(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "ListSeats", err)
	}
	seats, err := h.service.ListSeats(c.Context(), orgID(c), caller,
		strings.TrimSpace(c.Query("position_id")), c.Query("vacant") == "true")
	if err != nil {
		return mapError(c, log, "ListSeats", err)
	}
	return response.OK(c, fiber.Map{"seats": seats}, "OK")
}

// AssignSeat handles POST /organizations/:orgId/hrm/org-chart/seats/:seatId/assign
func (h *Handler) AssignSeat(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	caller, err := h.caller(c)
	if err != nil {
		return mapError(c, log, "AssignSeat", err)
	}
	var req AssignSeatRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	seat, err := h.service.AssignSeat(c.Context(), orgID(c), caller, c.Params("seatId"), req)
	if err != nil {
		return mapError(c, log, "AssignSeat", err)
	}
	return response.OK(c, fiber.Map{"seat": seat}, "Seat assignment updated")
}
