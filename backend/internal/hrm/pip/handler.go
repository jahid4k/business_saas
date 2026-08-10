// backend/internal/hrm/pip/handler.go
package pip

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles HRM performance improvement plan HTTP endpoints.
//
// It holds authz.Service because this module resolves THREE authorization
// facts per request — the scope tier, hrm.pips.manage and hrm.pips.close —
// and hands all three to the service on a Caller value. The
// performance.Handler precedent.
//
// close is separate from manage because closing as 'failed' triggers the
// draft-termination handoff. 'manager' holds manage and not close.
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
	tier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.pips")
	if err != nil {
		return Caller{}, err
	}
	canManage, err := h.authz.Can(c.Context(), userID, orgID, "hrm.pips", "manage")
	if err != nil {
		return Caller{}, err
	}
	canClose, err := h.authz.Can(c.Context(), userID, orgID, "hrm.pips", "close")
	if err != nil {
		return Caller{}, err
	}
	return Caller{UserID: userID, Tier: tier, CanManage: canManage, CanClose: canClose}, nil
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
	case errors.Is(err, ErrNotFound):
		return response.NotFound(c, "PIP_NOT_FOUND", "Performance improvement plan not found")
	case errors.Is(err, ErrEmployeeNotFound):
		return response.NotFound(c, "EMPLOYEE_NOT_FOUND", "Employee not found in this organization")

	case errors.Is(err, ErrAccessDenied):
		return response.Forbidden(c, "PIP_ACCESS_DENIED", err.Error())
	case errors.Is(err, ErrCloseDenied):
		return response.Forbidden(c, "PIP_CLOSE_DENIED", err.Error())
	case errors.Is(err, errUnauthenticated):
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")

	case errors.Is(err, ErrAlreadyOpen):
		return response.Conflict(c, "PIP_ALREADY_OPEN", err.Error())
	case errors.Is(err, ErrNotOpen):
		return response.Conflict(c, "PIP_NOT_OPEN", err.Error())
	case errors.Is(err, ErrNotActive):
		return response.Conflict(c, "PIP_NOT_ACTIVE", err.Error())
	case errors.Is(err, ErrAlreadyClosed):
		return response.Conflict(c, "PIP_ALREADY_CLOSED", err.Error())

	case errors.Is(err, ErrTitleRequired):
		return response.BadRequest(c, "TITLE_REQUIRED", err.Error())
	case errors.Is(err, ErrConcernsRequired):
		return response.BadRequest(c, "CONCERNS_REQUIRED", err.Error())
	case errors.Is(err, ErrCriteriaRequired):
		return response.BadRequest(c, "SUCCESS_CRITERIA_REQUIRED", err.Error())
	case errors.Is(err, ErrNoteRequired):
		return response.BadRequest(c, "NOTE_REQUIRED", err.Error())
	case errors.Is(err, ErrInvalidDate):
		return response.BadRequest(c, "INVALID_DATE", err.Error())
	case errors.Is(err, ErrInvalidPeriod):
		return response.BadRequest(c, "INVALID_PERIOD", err.Error())
	case errors.Is(err, ErrInvalidProgress):
		return response.BadRequest(c, "INVALID_PROGRESS", err.Error())
	case errors.Is(err, ErrInvalidOutcome):
		return response.BadRequest(c, "INVALID_OUTCOME", err.Error())
	case errors.Is(err, ErrOutcomeRequired):
		return response.BadRequest(c, "OUTCOME_REQUIRED", err.Error())
	case errors.Is(err, ErrExtensionBackwards):
		return response.BadRequest(c, "EXTENSION_BACKWARDS", err.Error())
	}

	log.Error("pip error", "error", err)
	return response.InternalServerError(c)
}

// List godoc
//
//	@Summary		List performance improvement plans
//	@Tags			HRM - PIP
//	@Security		BearerAuth
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			employee_id	query		string	false	"Filter by employee"
//	@Param			status		query		string	false	"draft | active | extended | closed | cancelled"
//	@Param			outcome		query		string	false	"successful | failed | abandoned"
//	@Success		200			{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/pips [get]
func (h *Handler) List(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	res, err := h.service.List(c.Context(), orgID, caller, ListFilter{
		EmployeeID: c.Query("employee_id"),
		Status:     c.Query("status"),
		Outcome:    c.Query("outcome"),
		Limit:      atoiOr(c.Query("limit"), 0),
		Offset:     atoiOr(c.Query("offset"), 0),
	})
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "Performance improvement plans retrieved")
}

// Get godoc
//
//	@Summary		Get a performance improvement plan
//	@Tags			HRM - PIP
//	@Security		BearerAuth
//	@Param			orgId	path		string	true	"Organization ID"
//	@Param			pipId	path		string	true	"PIP ID"
//	@Success		200		{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/pips/{pipId} [get]
func (h *Handler) Get(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	p, err := h.service.Get(c.Context(), orgID, c.Params("pipId"), caller)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"pip": p}, "Performance improvement plan retrieved")
}

// Create godoc
//
//	@Summary		Open a performance improvement plan
//	@Tags			HRM - PIP
//	@Security		BearerAuth
//	@Param			orgId	path		string			true	"Organization ID"
//	@Param			body	body		CreateRequest	true	"Plan"
//	@Success		201		{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/pips [post]
func (h *Handler) Create(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	var req CreateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	p, err := h.service.Create(c.Context(), orgID, userID, caller, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"pip": p}, "Performance improvement plan created")
}

// Update godoc
//
//	@Summary		Update a performance improvement plan
//	@Description	end_date is deliberately not editable here — it moves through /extend, which forces a written reason.
//	@Tags			HRM - PIP
//	@Security		BearerAuth
//	@Param			orgId	path		string			true	"Organization ID"
//	@Param			pipId	path		string			true	"PIP ID"
//	@Param			body	body		UpdateRequest	true	"Changes"
//	@Success		200		{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/pips/{pipId} [patch]
func (h *Handler) Update(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	var req UpdateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	p, err := h.service.Update(c.Context(), orgID, c.Params("pipId"), caller, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"pip": p}, "Performance improvement plan updated")
}

// Activate godoc
//
//	@Summary		Activate a performance improvement plan
//	@Tags			HRM - PIP
//	@Security		BearerAuth
//	@Param			orgId	path		string	true	"Organization ID"
//	@Param			pipId	path		string	true	"PIP ID"
//	@Success		200		{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/pips/{pipId}/activate [post]
func (h *Handler) Activate(c fiber.Ctx) error {
	return h.transition(c, func(orgID, ref string, caller Caller) (*Detail, error) {
		return h.service.Activate(c.Context(), orgID, ref, caller)
	}, "Performance improvement plan activated")
}

// Cancel godoc
//
//	@Summary		Cancel a performance improvement plan
//	@Description	Distinct from closing as 'abandoned': cancelling says the plan should not have been opened, abandoning says it ran and was dropped.
//	@Tags			HRM - PIP
//	@Security		BearerAuth
//	@Param			orgId	path		string	true	"Organization ID"
//	@Param			pipId	path		string	true	"PIP ID"
//	@Success		200		{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/pips/{pipId}/cancel [post]
func (h *Handler) Cancel(c fiber.Ctx) error {
	return h.transition(c, func(orgID, ref string, caller Caller) (*Detail, error) {
		return h.service.Cancel(c.Context(), orgID, ref, caller)
	}, "Performance improvement plan cancelled")
}

// transition factors the identical preamble out of the no-body state changes.
func (h *Handler) transition(c fiber.Ctx, fn func(orgID, ref string, caller Caller) (*Detail, error), msg string) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	p, err := fn(orgID, c.Params("pipId"), caller)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"pip": p}, msg)
}

// AddCheckin godoc
//
//	@Summary		Add a review entry to a performance improvement plan
//	@Tags			HRM - PIP
//	@Security		BearerAuth
//	@Param			orgId	path		string			true	"Organization ID"
//	@Param			pipId	path		string			true	"PIP ID"
//	@Param			body	body		CheckinRequest	true	"Review"
//	@Success		201		{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/pips/{pipId}/checkins [post]
func (h *Handler) AddCheckin(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	var req CheckinRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	p, err := h.service.AddCheckin(c.Context(), orgID, c.Params("pipId"), caller, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"pip": p}, "Review recorded")
}

// Extend godoc
//
//	@Summary		Extend a performance improvement plan's end date
//	@Description	The extension and its written reason land in one transaction; neither can exist without the other.
//	@Tags			HRM - PIP
//	@Security		BearerAuth
//	@Param			orgId	path		string			true	"Organization ID"
//	@Param			pipId	path		string			true	"PIP ID"
//	@Param			body	body		ExtendRequest	true	"New end date and reason"
//	@Success		200		{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/pips/{pipId}/extend [post]
func (h *Handler) Extend(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	var req ExtendRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	p, err := h.service.Extend(c.Context(), orgID, c.Params("pipId"), caller, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"pip": p}, "Performance improvement plan extended")
}

// Close godoc
//
//	@Summary		Close a performance improvement plan with an outcome
//	@Description	A 'failed' outcome creates a DRAFT termination and stops — submitting it for approval and applying it stay on the termination endpoints.
//	@Tags			HRM - PIP
//	@Security		BearerAuth
//	@Param			orgId	path		string			true	"Organization ID"
//	@Param			pipId	path		string			true	"PIP ID"
//	@Param			body	body		CloseRequest	true	"Outcome and note"
//	@Success		200		{object}	response.Response
//	@Router			/organizations/{orgId}/hrm/pips/{pipId}/close [post]
func (h *Handler) Close(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		return h.err(c, err)
	}
	var req CloseRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	p, err := h.service.Close(c.Context(), orgID, c.Params("pipId"), caller, req)

	// The handoff failure is a PARTIAL success: the plan closed, the draft
	// termination did not appear. Reporting it as a plain error would tell
	// the caller nothing happened, which is false and would invite a retry
	// that then fails with ErrAlreadyClosed.
	if IsHandoffFailure(err) && p != nil {
		logger.FromCtx(c).Error("pip: termination handoff failed", "error", err, "pip_id", p.ID)
		return response.OK(c, fiber.Map{"pip": p, "termination_draft_created": false},
			"Plan closed as failed, but the draft termination could not be created — create it manually")
	}
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"pip": p, "termination_draft_created": p.TerminationID != nil},
		"Performance improvement plan closed")
}
