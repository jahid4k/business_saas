// backend/internal/platform/tickets/handler.go
package tickets

import (
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler serves the ticket engine's HTTP endpoints.
//
// It holds the service and nothing else — no authz field. Every access
// decision belongs to the service, which resolves it from the
// AccessDirectory; a handler that also checked would be a second, divergent
// copy of the rules. The platform/checklists.Handler shape.
//
// Deliberately absent: any route reaching MarkConverted. Conversion is
// initiated from the HRM side, which creates the complaint and calls back —
// an HTTP endpoint would have to trust a client-supplied converted_to_id and
// could point a ticket at a complaint that does not exist. Same reasoning as
// checklists having no generic instantiate route.
type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler { return &Handler{service: service} }

func orgID(c fiber.Ctx) string { return c.Params("orgId") }

func callerUserID(c fiber.Ctx) (string, error) {
	id, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return "", errUnauthenticated
	}
	return id, nil
}

var errUnauthenticated = errors.New("unauthenticated")

func mapError(c fiber.Ctx, log *slog.Logger, op string, err error) error {
	switch {
	case errors.Is(err, ErrTicketNotFound):
		return response.NotFound(c, "TICKET_NOT_FOUND", "Ticket not found")
	case errors.Is(err, ErrCategoryNotFound):
		return response.NotFound(c, "TICKET_CATEGORY_NOT_FOUND", "Ticket category not found")
	case errors.Is(err, ErrPolicyNotFound):
		return response.NotFound(c, "SLA_POLICY_NOT_FOUND", "SLA policy not found")
	case errors.Is(err, ErrCommentNotFound):
		return response.NotFound(c, "TICKET_COMMENT_NOT_FOUND", "Ticket comment not found")
	case errors.Is(err, ErrInvalidPriority):
		return response.BadRequest(c, "INVALID_PRIORITY", "priority must be one of low, normal, high, urgent")
	case errors.Is(err, ErrInvalidRequesterType):
		return response.BadRequest(c, "INVALID_REQUESTER_TYPE", "Invalid requester_type")
	case errors.Is(err, ErrInvalidSLAMinutes):
		return response.BadRequest(c, "INVALID_SLA_MINUTES", "SLA minutes must be positive and resolution must not precede first response")
	case errors.Is(err, ErrRestrictedRoleMissing):
		return response.BadRequest(c, "RESTRICTED_ROLE_REQUIRED", "A sensitive category requires a restricted_role")
	case errors.Is(err, ErrSensitiveCategoryRole):
		return response.Forbidden(c, "SENSITIVE_CATEGORY_ROLE", "This category is restricted to a specific role; the assignee does not hold it")
	case errors.Is(err, ErrAlreadyPaused):
		return response.Conflict(c, "ALREADY_PAUSED", "The SLA clock is already paused")
	case errors.Is(err, ErrNotPaused):
		return response.Conflict(c, "NOT_PAUSED", "The SLA clock is not paused")
	case errors.Is(err, ErrAlreadyConverted):
		return response.Conflict(c, "ALREADY_CONVERTED", "This ticket has already been converted")
	case errors.Is(err, ErrWrongStatus):
		return response.Conflict(c, "WRONG_STATUS", "Not allowed in the ticket's current status")
	case errors.Is(err, ErrAccessDenied):
		return response.Forbidden(c, "ACCESS_DENIED", "You do not have access to this resource")
	case errors.Is(err, errUnauthenticated):
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	default:
		log.Error("tickets: "+op, slog.Any("error", err))
		return response.InternalServerError(c)
	}
}

// ============================================================
// Categories
// ============================================================

// ListCategories handles GET /organizations/:orgId/tickets/categories
func (h *Handler) ListCategories(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "ListCategories", err)
	}
	activeOnly := c.Query("active") != "false"
	list, err := h.service.ListCategories(c.Context(), orgID(c), userID, activeOnly)
	if err != nil {
		return mapError(c, log, "ListCategories", err)
	}
	return response.OK(c, fiber.Map{"categories": list}, "OK")
}

// CreateCategory handles POST /organizations/:orgId/tickets/categories
func (h *Handler) CreateCategory(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "CreateCategory", err)
	}
	var req CreateCategoryRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	cat, err := h.service.CreateCategory(c.Context(), orgID(c), userID, req)
	if err != nil {
		return mapError(c, log, "CreateCategory", err)
	}
	return response.Created(c, fiber.Map{"category": cat}, "Ticket category created")
}

// UpdateCategory handles PATCH /organizations/:orgId/tickets/categories/:categoryId
func (h *Handler) UpdateCategory(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "UpdateCategory", err)
	}
	var body struct {
		CreateCategoryRequest
		IsActive *bool `json:"is_active"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	cat, err := h.service.UpdateCategory(c.Context(), orgID(c), userID,
		c.Params("categoryId"), body.CreateCategoryRequest, body.IsActive)
	if err != nil {
		return mapError(c, log, "UpdateCategory", err)
	}
	return response.OK(c, fiber.Map{"category": cat}, "Ticket category updated")
}

// ============================================================
// SLA policies
// ============================================================

// ListPolicies handles GET /organizations/:orgId/tickets/sla-policies
func (h *Handler) ListPolicies(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "ListPolicies", err)
	}
	list, err := h.service.ListPolicies(c.Context(), orgID(c), userID)
	if err != nil {
		return mapError(c, log, "ListPolicies", err)
	}
	return response.OK(c, fiber.Map{"policies": list}, "OK")
}

// CreatePolicy handles POST /organizations/:orgId/tickets/sla-policies
func (h *Handler) CreatePolicy(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "CreatePolicy", err)
	}
	var req CreateSLAPolicyRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	p, err := h.service.CreatePolicy(c.Context(), orgID(c), userID, req)
	if err != nil {
		return mapError(c, log, "CreatePolicy", err)
	}
	return response.Created(c, fiber.Map{"policy": p}, "SLA policy created")
}

// UpdatePolicy handles PATCH /organizations/:orgId/tickets/sla-policies/:policyId
func (h *Handler) UpdatePolicy(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "UpdatePolicy", err)
	}
	var req CreateSLAPolicyRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	p, err := h.service.UpdatePolicy(c.Context(), orgID(c), userID, c.Params("policyId"), req)
	if err != nil {
		return mapError(c, log, "UpdatePolicy", err)
	}
	return response.OK(c, fiber.Map{"policy": p}, "SLA policy updated")
}

// ============================================================
// Tickets
// ============================================================

// CreateTicket handles POST /organizations/:orgId/tickets
func (h *Handler) CreateTicket(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "CreateTicket", err)
	}
	var req CreateTicketRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	t, err := h.service.Create(c.Context(), orgID(c), userID, req)
	if err != nil {
		return mapError(c, log, "CreateTicket", err)
	}
	return response.Created(c, fiber.Map{"ticket": t}, "Ticket created")
}

// ListTickets handles GET /organizations/:orgId/tickets
func (h *Handler) ListTickets(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "ListTickets", err)
	}
	// ViewerUserID and CanViewAll are deliberately NOT read from the query —
	// the service sets both from the authenticated caller.
	f := ListFilter{
		Status:         strings.TrimSpace(c.Query("status")),
		Priority:       strings.TrimSpace(c.Query("priority")),
		CategoryID:     strings.TrimSpace(c.Query("category_id")),
		AssigneeUserID: strings.TrimSpace(c.Query("assignee_user_id")),
		Limit:          atoiOr(c.Query("limit"), 0),
		Offset:         atoiOr(c.Query("offset"), 0),
	}
	res, err := h.service.List(c.Context(), orgID(c), userID, f)
	if err != nil {
		return mapError(c, log, "ListTickets", err)
	}
	return response.OK(c, res, "OK")
}

func atoiOr(raw string, fallback int) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return v
}

// GetTicket handles GET /organizations/:orgId/tickets/:ticketId
func (h *Handler) GetTicket(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "GetTicket", err)
	}
	t, err := h.service.Get(c.Context(), orgID(c), userID, c.Params("ticketId"))
	if err != nil {
		return mapError(c, log, "GetTicket", err)
	}
	return response.OK(c, fiber.Map{"ticket": t}, "OK")
}

// AssignTicket handles POST /organizations/:orgId/tickets/:ticketId/assign
func (h *Handler) AssignTicket(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "AssignTicket", err)
	}
	var req AssignTicketRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	t, err := h.service.Assign(c.Context(), orgID(c), userID, c.Params("ticketId"), req)
	if err != nil {
		return mapError(c, log, "AssignTicket", err)
	}
	return response.OK(c, fiber.Map{"ticket": t}, "Ticket assigned")
}

// ResolveTicket handles POST /organizations/:orgId/tickets/:ticketId/resolve
func (h *Handler) ResolveTicket(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "ResolveTicket", err)
	}
	t, err := h.service.Resolve(c.Context(), orgID(c), userID, c.Params("ticketId"))
	if err != nil {
		return mapError(c, log, "ResolveTicket", err)
	}
	return response.OK(c, fiber.Map{"ticket": t}, "Ticket resolved")
}

// CloseTicket handles POST /organizations/:orgId/tickets/:ticketId/close
func (h *Handler) CloseTicket(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "CloseTicket", err)
	}
	t, err := h.service.Close(c.Context(), orgID(c), userID, c.Params("ticketId"))
	if err != nil {
		return mapError(c, log, "CloseTicket", err)
	}
	return response.OK(c, fiber.Map{"ticket": t}, "Ticket closed")
}

// CancelTicket handles POST /organizations/:orgId/tickets/:ticketId/cancel
func (h *Handler) CancelTicket(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "CancelTicket", err)
	}
	t, err := h.service.Cancel(c.Context(), orgID(c), userID, c.Params("ticketId"))
	if err != nil {
		return mapError(c, log, "CancelTicket", err)
	}
	return response.OK(c, fiber.Map{"ticket": t}, "Ticket cancelled")
}

// PauseTicket handles POST /organizations/:orgId/tickets/:ticketId/pause
func (h *Handler) PauseTicket(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "PauseTicket", err)
	}
	var req PauseTicketRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	t, err := h.service.Pause(c.Context(), orgID(c), userID, c.Params("ticketId"), req)
	if err != nil {
		return mapError(c, log, "PauseTicket", err)
	}
	return response.OK(c, fiber.Map{"ticket": t}, "SLA clock paused")
}

// ResumeTicket handles POST /organizations/:orgId/tickets/:ticketId/resume
func (h *Handler) ResumeTicket(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "ResumeTicket", err)
	}
	t, err := h.service.Resume(c.Context(), orgID(c), userID, c.Params("ticketId"))
	if err != nil {
		return mapError(c, log, "ResumeTicket", err)
	}
	return response.OK(c, fiber.Map{"ticket": t}, "SLA clock resumed")
}

// ============================================================
// Comments
// ============================================================

// ListComments handles GET /organizations/:orgId/tickets/:ticketId/comments
func (h *Handler) ListComments(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "ListComments", err)
	}
	list, err := h.service.ListComments(c.Context(), orgID(c), userID, c.Params("ticketId"))
	if err != nil {
		return mapError(c, log, "ListComments", err)
	}
	return response.OK(c, fiber.Map{"comments": list}, "OK")
}

// AddComment handles POST /organizations/:orgId/tickets/:ticketId/comments
func (h *Handler) AddComment(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, err := callerUserID(c)
	if err != nil {
		return mapError(c, log, "AddComment", err)
	}
	var req CreateCommentRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	cm, err := h.service.AddComment(c.Context(), orgID(c), userID, c.Params("ticketId"), req)
	if err != nil {
		return mapError(c, log, "AddComment", err)
	}
	return response.Created(c, fiber.Map{"comment": cm}, "Comment added")
}
