// backend/internal/hrm/feedback/handler.go
package feedback

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles HRM 360 feedback HTTP endpoints.
//
// It holds authz.Service because this module resolves THREE authorization
// facts per request — the caller's scope tier, whether they may coordinate,
// and whether they may manage — and hands all three to the service on a
// Caller value. The performance.Handler precedent.
//
// hrm.feedback.coordinate is the one that matters: it is what separates the
// identity-bearing read path from the content-bearing one, and no single
// permission grants both.
type Handler struct {
	service Service
	authz   authz.Service
}

func NewHandler(service Service, authzSvc authz.Service) *Handler {
	return &Handler{service: service, authz: authzSvc}
}

var errUnauthenticated = errors.New("authentication required")

func requestOrg(c fiber.Ctx) (string, bool) {
	return middleware.OrganizationIDFromCtx(c)
}

func (h *Handler) resolveCaller(c fiber.Ctx, orgID string) (Caller, error) {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return Caller{}, errUnauthenticated
	}
	tier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.feedback")
	if err != nil {
		return Caller{}, err
	}
	canCoordinate, err := h.authz.Can(c.Context(), userID, orgID, "hrm.feedback", "coordinate")
	if err != nil {
		return Caller{}, err
	}
	canManage, err := h.authz.Can(c.Context(), userID, orgID, "hrm.feedback", "manage")
	if err != nil {
		return Caller{}, err
	}
	return Caller{UserID: userID, Tier: tier, CanCoordinate: canCoordinate, CanManage: canManage}, nil
}

func atoiOr(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}

// err maps every sentinel this package raises onto an HTTP response. Anything
// unmapped is logged and 500s rather than leaking an internal message.
func (h *Handler) err(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	// ── Not found ────────────────────────────────────────────────────────
	case errors.Is(err, ErrCycleNotFound):
		return response.NotFound(c, "FEEDBACK_CYCLE_NOT_FOUND", "Feedback cycle not found")
	case errors.Is(err, ErrRequestNotFound):
		return response.NotFound(c, "FEEDBACK_REQUEST_NOT_FOUND", "Feedback request not found")
	case errors.Is(err, ErrEmployeeNotFound):
		return response.NotFound(c, "EMPLOYEE_NOT_FOUND", "Employee not found in this organization")

	// ── Forbidden ────────────────────────────────────────────────────────
	case errors.Is(err, ErrAccessDenied):
		return response.Forbidden(c, "FEEDBACK_ACCESS_DENIED", "You do not have access to this feedback")
	case errors.Is(err, ErrNotRespondent):
		return response.Forbidden(c, "NOT_RESPONDENT", "This feedback request is not addressed to you")
	case errors.Is(err, errUnauthenticated):
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")

	// ── Conflict ─────────────────────────────────────────────────────────
	case errors.Is(err, ErrCycleNameTaken):
		return response.Conflict(c, "FEEDBACK_CYCLE_NAME_TAKEN", err.Error())
	case errors.Is(err, ErrCycleNotActive):
		return response.Conflict(c, "FEEDBACK_CYCLE_NOT_ACTIVE", err.Error())
	case errors.Is(err, ErrCycleClosed):
		return response.Conflict(c, "FEEDBACK_CYCLE_CLOSED", err.Error())
	case errors.Is(err, ErrCycleStatus):
		return response.Conflict(c, "FEEDBACK_CYCLE_STATUS", err.Error())
	case errors.Is(err, ErrDuplicateRequest):
		return response.Conflict(c, "DUPLICATE_FEEDBACK_REQUEST", err.Error())
	case errors.Is(err, ErrAlreadySubmitted):
		return response.Conflict(c, "FEEDBACK_ALREADY_SUBMITTED", err.Error())
	case errors.Is(err, ErrRequestClosed):
		return response.Conflict(c, "FEEDBACK_REQUEST_CLOSED", err.Error())

	// ── Bad request ──────────────────────────────────────────────────────
	case errors.Is(err, ErrCycleNameRequired):
		return response.BadRequest(c, "NAME_REQUIRED", err.Error())
	case errors.Is(err, ErrTemplateRequired):
		return response.BadRequest(c, "FORM_TEMPLATE_REQUIRED", err.Error())
	case errors.Is(err, ErrInvalidPeriod):
		return response.BadRequest(c, "INVALID_PERIOD", err.Error())
	case errors.Is(err, ErrInvalidDate):
		return response.BadRequest(c, "INVALID_DATE", err.Error())
	case errors.Is(err, ErrMinResponses):
		return response.BadRequest(c, "INVALID_MIN_RESPONSES", err.Error())
	case errors.Is(err, ErrNoRespondents):
		return response.BadRequest(c, "NO_RESPONDENTS", err.Error())
	case errors.Is(err, ErrInvalidRelationship):
		return response.BadRequest(c, "INVALID_RELATIONSHIP", err.Error())
	case errors.Is(err, ErrRespondentRequired):
		return response.BadRequest(c, "RESPONDENT_REQUIRED", err.Error())
	case errors.Is(err, ErrSelfMismatch):
		return response.BadRequest(c, "SELF_RELATIONSHIP_MISMATCH", err.Error())
	}

	log.Error("feedback error", "error", err)
	return response.InternalServerError(c)
}

// ── Cycles ───────────────────────────────────────────────────────────────────

// ListCycles godoc
//
//	@Summary		List 360 feedback cycles
//	@Tags			HRM - 360 Feedback
//	@Security		BearerAuth
//	@Param			orgId	path		string	true	"Organization ID"
//	@Param			status	query		string	false	"draft | active | closed"
//	@Success		200		{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/feedback/cycles [get]
func (h *Handler) ListCycles(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	res, err := h.service.ListCycles(c.Context(), orgID, CycleListFilter{
		Status: c.Query("status"),
		Limit:  atoiOr(c.Query("limit"), 0),
		Offset: atoiOr(c.Query("offset"), 0),
	})
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "Feedback cycles retrieved")
}

// GetCycle godoc
//
//	@Summary		Get a 360 feedback cycle
//	@Tags			HRM - 360 Feedback
//	@Security		BearerAuth
//	@Param			orgId	path		string	true	"Organization ID"
//	@Param			cycleId	path		string	true	"Cycle ID"
//	@Success		200		{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/feedback/cycles/{cycleId} [get]
func (h *Handler) GetCycle(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	cy, err := h.service.GetCycle(c.Context(), orgID, c.Params("cycleId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"cycle": cy}, "Feedback cycle retrieved")
}

// CreateCycle godoc
//
//	@Summary		Create a 360 feedback cycle
//	@Tags			HRM - 360 Feedback
//	@Security		BearerAuth
//	@Param			orgId	path		string				true	"Organization ID"
//	@Param			body	body		CreateCycleRequest	true	"Cycle"
//	@Success		201		{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/feedback/cycles [post]
func (h *Handler) CreateCycle(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateCycleRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	cy, err := h.service.CreateCycle(c.Context(), orgID, userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"cycle": cy}, "Feedback cycle created")
}

// UpdateCycle godoc
//
//	@Summary		Update a 360 feedback cycle
//	@Tags			HRM - 360 Feedback
//	@Security		BearerAuth
//	@Param			orgId	path		string				true	"Organization ID"
//	@Param			cycleId	path		string				true	"Cycle ID"
//	@Param			body	body		UpdateCycleRequest	true	"Changes"
//	@Success		200		{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/feedback/cycles/{cycleId} [patch]
func (h *Handler) UpdateCycle(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateCycleRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	cy, err := h.service.UpdateCycle(c.Context(), orgID, c.Params("cycleId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"cycle": cy}, "Feedback cycle updated")
}

// ActivateCycle godoc
//
//	@Summary		Activate a 360 feedback cycle
//	@Tags			HRM - 360 Feedback
//	@Security		BearerAuth
//	@Param			orgId	path		string	true	"Organization ID"
//	@Param			cycleId	path		string	true	"Cycle ID"
//	@Success		200		{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/feedback/cycles/{cycleId}/activate [post]
func (h *Handler) ActivateCycle(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	cy, err := h.service.ActivateCycle(c.Context(), orgID, c.Params("cycleId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"cycle": cy}, "Feedback cycle activated")
}

// CloseCycle godoc
//
//	@Summary		Close a 360 feedback cycle
//	@Tags			HRM - 360 Feedback
//	@Security		BearerAuth
//	@Param			orgId	path		string	true	"Organization ID"
//	@Param			cycleId	path		string	true	"Cycle ID"
//	@Success		200		{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/feedback/cycles/{cycleId}/close [post]
func (h *Handler) CloseCycle(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	cy, err := h.service.CloseCycle(c.Context(), orgID, c.Params("cycleId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"cycle": cy}, "Feedback cycle closed")
}

// ── Requests: coordination ───────────────────────────────────────────────────

// CreateRequests godoc
//
//	@Summary		Ask a batch of people for feedback about one subject
//	@Description	Returns summaries only — never a form instance reference.
//	@Tags			HRM - 360 Feedback
//	@Security		BearerAuth
//	@Param			orgId	path		string					true	"Organization ID"
//	@Param			cycleId	path		string					true	"Cycle ID"
//	@Param			body	body		CreateRequestsRequest	true	"Subject and respondents"
//	@Success		201		{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/feedback/cycles/{cycleId}/requests [post]
func (h *Handler) CreateRequests(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateRequestsRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	out, err := h.service.CreateRequests(c.Context(), orgID, c.Params("cycleId"), userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"requests": out}, "Feedback requests created")
}

// ListRequests godoc
//
//	@Summary		List who was asked and who has responded
//	@Description	The COORDINATION view: identity and status, never answer content.
//	@Tags			HRM - 360 Feedback
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			cycle_id	query		string	false	"Filter by cycle"
//	@Param			employee_id	query		string	false	"Filter by subject"
//	@Param			status		query		string	false	"pending | submitted | declined | cancelled"
//	@Success		200			{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/feedback/requests [get]
func (h *Handler) ListRequests(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	res, err := h.service.ListRequests(c.Context(), orgID, caller, RequestListFilter{
		CycleID:           c.Query("cycle_id"),
		SubjectEmployeeID: c.Query("employee_id"),
		Status:            c.Query("status"),
		Limit:             atoiOr(c.Query("limit"), 0),
		Offset:            atoiOr(c.Query("offset"), 0),
	})
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "Feedback requests retrieved")
}

// ── Requests: responding ─────────────────────────────────────────────────────

// ListMyRequests godoc
//
//	@Summary		List feedback requests addressed to you
//	@Description	The only response type carrying a form instance id, because the caller is the person meant to fill it in.
//	@Tags			HRM - 360 Feedback
//	@Security		BearerAuth
//	@Param			orgId	path		string	true	"Organization ID"
//	@Success		200		{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/feedback/requests/mine [get]
func (h *Handler) ListMyRequests(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	res, err := h.service.ListMyRequests(c.Context(), orgID, userID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "Your feedback requests retrieved")
}

// SubmitResponse godoc
//
//	@Summary		Mark a feedback request answered
//	@Tags			HRM - 360 Feedback
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			requestId	path		string	true	"Request ID"
//	@Success		200			{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/feedback/requests/{requestId}/submit [post]
func (h *Handler) SubmitResponse(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	out, err := h.service.SubmitResponse(c.Context(), orgID, c.Params("requestId"), caller)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"request": out}, "Feedback submitted")
}

// DeclineRequest godoc
//
//	@Summary		Decline a feedback request addressed to you
//	@Tags			HRM - 360 Feedback
//	@Security		BearerAuth
//	@Param			orgId		path		string			true	"Organization ID"
//	@Param			requestId	path		string			true	"Request ID"
//	@Param			body		body		DeclineRequest	false	"Reason"
//	@Success		200			{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/feedback/requests/{requestId}/decline [post]
func (h *Handler) DeclineRequest(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	var req DeclineRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	out, err := h.service.DeclineRequest(c.Context(), orgID, c.Params("requestId"), caller, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"request": out}, "Feedback request declined")
}

// ── The content path ─────────────────────────────────────────────────────────

// GetAggregate godoc
//
//	@Summary		Read anonymised 360 feedback about an employee
//	@Description	Anonymous relationship groups are SUPPRESSED below the cycle's min_responses threshold, for every role including admins. Never returns a respondent or a form instance reference.
//	@Tags			HRM - 360 Feedback
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			cycleId		path		string	true	"Cycle ID"
//	@Param			employeeId	path		string	true	"Subject employee ID"
//	@Success		200			{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/feedback/cycles/{cycleId}/employees/{employeeId}/aggregate [get]
func (h *Handler) GetAggregate(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	agg, err := h.service.GetAggregate(c.Context(), orgID, c.Params("cycleId"), c.Params("employeeId"), caller)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"aggregate": agg}, "Feedback aggregate retrieved")
}
