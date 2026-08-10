// backend/internal/hrm/performance/appraisals_handler.go
package performance

import (
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// ── Rating scales ────────────────────────────────────────────────────────────

// ListScales godoc
//
//	@Summary		List rating scales
//	@Tags			HRM / Performance
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Success		200		{object}	response.OK{data=object{scales=[]RatingScale}}
//	@Router			/organizations/{orgId}/hrm/performance/rating-scales [get]
func (h *Handler) ListScales(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	list, err := h.service.ListScales(c.Context(), orgID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"scales": list}, "OK")
}

// GetScale godoc
//
//	@Summary		Get a rating scale with its levels
//	@Tags			HRM / Performance
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			scaleId	path	string	true	"Scale public ID"
//	@Success		200		{object}	response.OK{data=object{scale=RatingScale}}
//	@Router			/organizations/{orgId}/hrm/performance/rating-scales/{scaleId} [get]
func (h *Handler) GetScale(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	sc, err := h.service.GetScale(c.Context(), orgID, c.Params("scaleId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"scale": sc}, "OK")
}

// CreateScale godoc
//
//	@Summary		Create a rating scale
//	@Tags			HRM / Performance
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string				true	"Organization ID"
//	@Param			body	body	CreateScaleRequest	true	"Scale"
//	@Success		201		{object}	response.Created{data=object{scale=RatingScale}}
//	@Router			/organizations/{orgId}/hrm/performance/rating-scales [post]
func (h *Handler) CreateScale(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateScaleRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	sc, err := h.service.CreateScale(c.Context(), orgID, userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"scale": sc}, "Rating scale created")
}

// UpdateScale godoc
//
//	@Summary		Update a rating scale
//	@Tags			HRM / Performance
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string				true	"Organization ID"
//	@Param			scaleId	path	string				true	"Scale public ID"
//	@Param			body	body	UpdateScaleRequest	true	"Fields to update"
//	@Success		200		{object}	response.OK{data=object{scale=RatingScale}}
//	@Router			/organizations/{orgId}/hrm/performance/rating-scales/{scaleId} [patch]
func (h *Handler) UpdateScale(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateScaleRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	sc, err := h.service.UpdateScale(c.Context(), orgID, c.Params("scaleId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"scale": sc}, "Rating scale updated")
}

// DeleteScale godoc
//
//	@Summary		Delete a rating scale
//	@Description	Refused while an appraisal cycle references it.
//	@Tags			HRM / Performance
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			scaleId	path	string	true	"Scale public ID"
//	@Success		204
//	@Failure		409		{object}	response.Error	"SCALE_IN_USE"
//	@Router			/organizations/{orgId}/hrm/performance/rating-scales/{scaleId} [delete]
func (h *Handler) DeleteScale(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.DeleteScale(c.Context(), orgID, c.Params("scaleId")); err != nil {
		return h.err(c, err)
	}
	return response.NoContent(c)
}

// CreateLevel godoc
//
//	@Summary		Add a level to a rating scale
//	@Tags			HRM / Performance
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string				true	"Organization ID"
//	@Param			scaleId	path	string				true	"Scale public ID"
//	@Param			body	body	CreateLevelRequest	true	"Level"
//	@Success		201		{object}	response.Created{data=object{level=RatingLevel}}
//	@Router			/organizations/{orgId}/hrm/performance/rating-scales/{scaleId}/levels [post]
func (h *Handler) CreateLevel(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateLevelRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	l, err := h.service.CreateLevel(c.Context(), orgID, c.Params("scaleId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"level": l}, "Rating level created")
}

// UpdateLevel godoc
//
//	@Summary		Update a rating level
//	@Tags			HRM / Performance
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string				true	"Organization ID"
//	@Param			levelId	path	string				true	"Level public ID"
//	@Param			body	body	UpdateLevelRequest	true	"Fields to update"
//	@Success		200		{object}	response.OK{data=object{level=RatingLevel}}
//	@Router			/organizations/{orgId}/hrm/performance/rating-levels/{levelId} [patch]
func (h *Handler) UpdateLevel(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateLevelRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	l, err := h.service.UpdateLevel(c.Context(), orgID, c.Params("levelId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"level": l}, "Rating level updated")
}

// DeleteLevel godoc
//
//	@Summary		Delete a rating level
//	@Description	Appraisals already rated at this level keep their label and
//	@Description	value snapshot — the FK is ON DELETE SET NULL precisely so
//	@Description	historical ratings survive.
//	@Tags			HRM / Performance
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			levelId	path	string	true	"Level public ID"
//	@Success		204
//	@Router			/organizations/{orgId}/hrm/performance/rating-levels/{levelId} [delete]
func (h *Handler) DeleteLevel(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.DeleteLevel(c.Context(), orgID, c.Params("levelId")); err != nil {
		return h.err(c, err)
	}
	return response.NoContent(c)
}

// ── Appraisal cycles ─────────────────────────────────────────────────────────

// ListAppraisalCycles godoc
//
//	@Summary		List appraisal cycles
//	@Tags			HRM / Performance
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			status	query	string	false	"Filter by status"
//	@Success		200		{object}	response.OK{data=AppraisalCycleListResponse}
//	@Router			/organizations/{orgId}/hrm/performance/appraisal-cycles [get]
func (h *Handler) ListAppraisalCycles(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	filter := AppraisalCycleListFilter{Status: c.Query("status")}
	if limit, err := strconv.Atoi(c.Query("limit", "")); err == nil {
		filter.Limit = limit
	}
	if offset, err := strconv.Atoi(c.Query("offset", "")); err == nil {
		filter.Offset = offset
	}
	res, err := h.service.ListAppraisalCycles(c.Context(), orgID, filter)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "OK")
}

// GetAppraisalCycle godoc
//
//	@Summary		Get an appraisal cycle
//	@Tags			HRM / Performance
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			cycleId	path	string	true	"Cycle public ID"
//	@Success		200		{object}	response.OK{data=object{cycle=AppraisalCycle}}
//	@Router			/organizations/{orgId}/hrm/performance/appraisal-cycles/{cycleId} [get]
func (h *Handler) GetAppraisalCycle(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	cy, err := h.service.GetAppraisalCycle(c.Context(), orgID, c.Params("cycleId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"cycle": cy}, "OK")
}

// CreateAppraisalCycle godoc
//
//	@Summary		Create an appraisal cycle
//	@Description	Requires a rating scale that already has levels, and at least
//	@Description	one form template. goal_cycle_id optionally links a Phase 5A
//	@Description	goal cycle, which makes goal attainment part of the appraisal.
//	@Tags			HRM / Performance
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string						true	"Organization ID"
//	@Param			body	body	CreateAppraisalCycleRequest	true	"Cycle"
//	@Success		201		{object}	response.Created{data=object{cycle=AppraisalCycle}}
//	@Router			/organizations/{orgId}/hrm/performance/appraisal-cycles [post]
func (h *Handler) CreateAppraisalCycle(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateAppraisalCycleRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	cy, err := h.service.CreateAppraisalCycle(c.Context(), orgID, userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"cycle": cy}, "Appraisal cycle created")
}

// UpdateAppraisalCycle godoc
//
//	@Summary		Update an appraisal cycle
//	@Tags			HRM / Performance
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string						true	"Organization ID"
//	@Param			cycleId	path	string						true	"Cycle public ID"
//	@Param			body	body	UpdateAppraisalCycleRequest	true	"Fields to update"
//	@Success		200		{object}	response.OK{data=object{cycle=AppraisalCycle}}
//	@Router			/organizations/{orgId}/hrm/performance/appraisal-cycles/{cycleId} [patch]
func (h *Handler) UpdateAppraisalCycle(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req UpdateAppraisalCycleRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	cy, err := h.service.UpdateAppraisalCycle(c.Context(), orgID, c.Params("cycleId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"cycle": cy}, "Appraisal cycle updated")
}

// ActivateAppraisalCycle godoc
//
//	@Summary		Activate an appraisal cycle (draft → active)
//	@Tags			HRM / Performance
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			cycleId	path	string	true	"Cycle public ID"
//	@Success		200		{object}	response.OK{data=object{cycle=AppraisalCycle}}
//	@Router			/organizations/{orgId}/hrm/performance/appraisal-cycles/{cycleId}/activate [post]
func (h *Handler) ActivateAppraisalCycle(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	cy, err := h.service.ActivateAppraisalCycle(c.Context(), orgID, c.Params("cycleId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"cycle": cy}, "Appraisal cycle activated")
}

// CloseAppraisalCycle godoc
//
//	@Summary		Close an appraisal cycle
//	@Tags			HRM / Performance
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			cycleId	path	string	true	"Cycle public ID"
//	@Success		200		{object}	response.OK{data=object{cycle=AppraisalCycle}}
//	@Router			/organizations/{orgId}/hrm/performance/appraisal-cycles/{cycleId}/close [post]
func (h *Handler) CloseAppraisalCycle(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	cy, err := h.service.CloseAppraisalCycle(c.Context(), orgID, c.Params("cycleId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"cycle": cy}, "Appraisal cycle closed")
}

// InstantiateAppraisal godoc
//
//	@Summary		Create one employee's appraisal within a cycle
//	@Description	Freezes the employee's manager and instantiates the cycle's
//	@Description	self and manager forms. This is the module-owned endpoint the
//	@Description	form engine deliberately does not expose generically.
//	@Tags			HRM / Performance
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string						true	"Organization ID"
//	@Param			cycleId	path	string						true	"Cycle public ID"
//	@Param			body	body	InstantiateAppraisalRequest	true	"Employee"
//	@Success		201		{object}	response.Created{data=object{appraisal=AppraisalDetail}}
//	@Failure		409		{object}	response.Error	"APPRAISAL_EXISTS or CYCLE_NOT_ACTIVE"
//	@Router			/organizations/{orgId}/hrm/performance/appraisal-cycles/{cycleId}/appraisals [post]
func (h *Handler) InstantiateAppraisal(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req InstantiateAppraisalRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	a, err := h.service.InstantiateAppraisal(c.Context(), orgID, c.Params("cycleId"), userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"appraisal": a}, "Appraisal created")
}

// ── Appraisals ───────────────────────────────────────────────────────────────

// ListAppraisals godoc
//
//	@Summary		List appraisals
//	@Description	Scope-filtered: view_own returns the caller's, view_team adds
//	@Description	their direct reports', view_all returns the organization's.
//	@Description	This is the control that prevents appraisal draft leakage.
//	@Tags			HRM / Performance
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			cycle_id	query	string	false	"Filter by cycle"
//	@Param			employee_id	query	string	false	"Filter by employee"
//	@Param			phase		query	string	false	"Filter by phase"
//	@Success		200			{object}	response.OK{data=AppraisalListResponse}
//	@Router			/organizations/{orgId}/hrm/performance/appraisals [get]
func (h *Handler) ListAppraisals(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	tier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.appraisals")
	if err != nil {
		log.Error("performance: ListAppraisals", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	filter := AppraisalListFilter{
		CycleID: c.Query("cycle_id"), EmployeeID: c.Query("employee_id"), Phase: c.Query("phase"),
		Scope: tier, CallerUserID: userID,
	}
	if limit, err := strconv.Atoi(c.Query("limit", "")); err == nil {
		filter.Limit = limit
	}
	if offset, err := strconv.Atoi(c.Query("offset", "")); err == nil {
		filter.Offset = offset
	}
	res, err := h.service.ListAppraisals(c.Context(), orgID, filter)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "OK")
}

// resolveAppraisalCaller builds the Caller for appraisal routes. The tier is
// resolved against "hrm.appraisals" rather than "hrm.goals", so the two
// modules' visibility can be granted independently.
func (h *Handler) resolveAppraisalCaller(c fiber.Ctx, orgID string) (Caller, error) {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return Caller{}, errUnauthenticated
	}
	tier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.appraisals")
	if err != nil {
		return Caller{}, err
	}
	canManage, err := h.authz.Can(c.Context(), userID, orgID, "hrm.appraisals", "manage")
	if err != nil {
		return Caller{}, err
	}
	return Caller{UserID: userID, Tier: tier, CanManage: canManage}, nil
}

// GetAppraisal godoc
//
//	@Summary		Get an appraisal with its scores, history and legal next phases
//	@Description	Scores are read live before publish and from the frozen
//	@Description	snapshot afterwards, so a published appraisal reports the same
//	@Description	numbers forever.
//	@Tags			HRM / Performance
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			appraisalId	path	string	true	"Appraisal public ID"
//	@Success		200			{object}	response.OK{data=object{appraisal=AppraisalDetail}}
//	@Failure		403			{object}	response.Error	"APPRAISAL_ACCESS_DENIED"
//	@Router			/organizations/{orgId}/hrm/performance/appraisals/{appraisalId} [get]
func (h *Handler) GetAppraisal(c fiber.Ctx) error {
	return h.appraisalAction(c, "GetAppraisal", func(orgID string, caller Caller) (*AppraisalDetail, error) {
		return h.service.GetAppraisal(c.Context(), orgID, c.Params("appraisalId"), caller)
	}, "OK")
}

// AdvancePhase godoc
//
//	@Summary		Move an appraisal to another phase
//	@Description	The target phase is explicit because two phases can legally
//	@Description	move backwards — manager_review can return to self_review and
//	@Description	calibration can return to manager_review.
//	@Tags			HRM / Performance
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string				true	"Organization ID"
//	@Param			appraisalId	path	string				true	"Appraisal public ID"
//	@Param			body		body	AdvancePhaseRequest	true	"Target phase"
//	@Success		200			{object}	response.OK{data=object{appraisal=AppraisalDetail}}
//	@Failure		409			{object}	response.Error	"ILLEGAL_PHASE_TRANSITION"
//	@Router			/organizations/{orgId}/hrm/performance/appraisals/{appraisalId}/phase [post]
func (h *Handler) AdvancePhase(c fiber.Ctx) error {
	var req AdvancePhaseRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	return h.appraisalAction(c, "AdvancePhase", func(orgID string, caller Caller) (*AppraisalDetail, error) {
		return h.service.AdvancePhase(c.Context(), orgID, c.Params("appraisalId"), caller, req)
	}, "Appraisal phase updated")
}

// SetRating godoc
//
//	@Summary		Set the manager's proposed final rating
//	@Tags			HRM / Performance
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string				true	"Organization ID"
//	@Param			appraisalId	path	string				true	"Appraisal public ID"
//	@Param			body		body	SetRatingRequest	true	"Rating level"
//	@Success		200			{object}	response.OK{data=object{appraisal=AppraisalDetail}}
//	@Router			/organizations/{orgId}/hrm/performance/appraisals/{appraisalId}/rating [post]
func (h *Handler) SetRating(c fiber.Ctx) error {
	var req SetRatingRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	return h.appraisalAction(c, "SetRating", func(orgID string, caller Caller) (*AppraisalDetail, error) {
		return h.service.SetRating(c.Context(), orgID, c.Params("appraisalId"), caller, req)
	}, "Rating set")
}

// Calibrate godoc
//
//	@Summary		Adjust a rating during calibration
//	@Description	The note is mandatory — an unexplained override of a manager's
//	@Description	assessment is what the audit trail exists to prevent. Both the
//	@Description	previous and new rating are recorded in phase history.
//	@Tags			HRM / Performance
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string				true	"Organization ID"
//	@Param			appraisalId	path	string				true	"Appraisal public ID"
//	@Param			body		body	CalibrateRequest	true	"Rating and mandatory note"
//	@Success		200			{object}	response.OK{data=object{appraisal=AppraisalDetail}}
//	@Failure		400			{object}	response.Error	"CALIBRATION_NOTE_REQUIRED"
//	@Failure		409			{object}	response.Error	"NOT_IN_CALIBRATION"
//	@Router			/organizations/{orgId}/hrm/performance/appraisals/{appraisalId}/calibrate [post]
func (h *Handler) Calibrate(c fiber.Ctx) error {
	var req CalibrateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	return h.appraisalAction(c, "Calibrate", func(orgID string, caller Caller) (*AppraisalDetail, error) {
		return h.service.Calibrate(c.Context(), orgID, c.Params("appraisalId"), caller, req)
	}, "Rating calibrated")
}

// PublishAppraisal godoc
//
//	@Summary		Publish an appraisal (irreversible)
//	@Description	Freezes the self score, manager score and goal attainment onto
//	@Description	the record. The only move out of published is acknowledged.
//	@Tags			HRM / Performance
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			appraisalId	path	string	true	"Appraisal public ID"
//	@Success		200			{object}	response.OK{data=object{appraisal=AppraisalDetail}}
//	@Failure		409			{object}	response.Error	"RATING_REQUIRED_TO_PUBLISH"
//	@Router			/organizations/{orgId}/hrm/performance/appraisals/{appraisalId}/publish [post]
func (h *Handler) PublishAppraisal(c fiber.Ctx) error {
	return h.appraisalAction(c, "PublishAppraisal", func(orgID string, caller Caller) (*AppraisalDetail, error) {
		return h.service.PublishAppraisal(c.Context(), orgID, c.Params("appraisalId"), caller)
	}, "Appraisal published")
}

// appraisalAction factors the org/caller resolution every appraisal route
// shares, so each handler is its own two-line function.
func (h *Handler) appraisalAction(c fiber.Ctx, op string, fn func(orgID string, caller Caller) (*AppraisalDetail, error), msg string) error {
	log := logger.FromCtx(c)
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveAppraisalCaller(c, orgID)
	if err != nil {
		if err == errUnauthenticated {
			return h.err(c, err)
		}
		log.Error("performance: "+op, slog.Any("error", err))
		return response.InternalServerError(c)
	}
	a, err := fn(orgID, caller)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"appraisal": a}, msg)
}
