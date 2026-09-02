// backend/internal/hrm/performance/goals_handler.go
package performance

import (
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// buildGoalListFilter reads query params and attaches the caller's resolved
// scope tier. The tier is set HERE and never in the service or repository —
// the Phase 1 convention. Its zero value means "no rows", not "no filter".
func (h *Handler) buildGoalListFilter(c fiber.Ctx, orgID, userID string) (GoalListFilter, error) {
	tier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.goals")
	if err != nil {
		return GoalListFilter{}, err
	}
	filter := GoalListFilter{
		CycleID:      c.Query("cycle_id"),
		EmployeeID:   c.Query("employee_id"),
		Status:       c.Query("status"),
		GoalLevel:    c.Query("goal_level"),
		ParentID:     c.Query("parent_goal_id"),
		Scope:        tier,
		CallerUserID: userID,
	}
	if limit, err := strconv.Atoi(c.Query("limit", "")); err == nil {
		filter.Limit = limit
	}
	if offset, err := strconv.Atoi(c.Query("offset", "")); err == nil {
		filter.Offset = offset
	}
	return filter, nil
}

// ListGoals godoc
//
//	@Summary		List goals
//	@Description	Scope-filtered: view_own returns the caller's goals, view_team
//	@Description	adds their direct reports', view_all returns the organization's.
//	@Tags			HRM / Performance
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId			path	string	true	"Organization ID"
//	@Param			cycle_id		query	string	false	"Filter by cycle"
//	@Param			employee_id		query	string	false	"Filter by employee"
//	@Param			status			query	string	false	"Filter by status"
//	@Param			goal_level		query	string	false	"Filter by level"
//	@Param			parent_goal_id	query	string	false	"Filter by alignment parent"
//	@Success		200				{object}	response.OK{data=GoalListResponse}
//	@Router			/organizations/{orgId}/hrm/performance/goals [get]
func (h *Handler) ListGoals(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	filter, err := h.buildGoalListFilter(c, orgID, userID)
	if err != nil {
		log.Error("performance: ListGoals", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	res, err := h.service.ListGoals(c.Context(), orgID, filter)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "OK")
}

// ListGoalsForEmployee godoc
//
//	@Summary		List one employee's goals
//	@Description	Same scope filtering as the collection route — an out-of-scope
//	@Description	employee simply yields no rows.
//	@Tags			HRM / Performance
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			employeeId	path	string	true	"Employee public ID"
//	@Param			cycle_id	query	string	false	"Filter by cycle"
//	@Success		200			{object}	response.OK{data=GoalListResponse}
//	@Router			/organizations/{orgId}/hrm/employees/{employeeId}/goals [get]
func (h *Handler) ListGoalsForEmployee(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	filter, err := h.buildGoalListFilter(c, orgID, userID)
	if err != nil {
		log.Error("performance: ListGoalsForEmployee", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	filter.EmployeeID = c.Params("employeeId")
	res, err := h.service.ListGoals(c.Context(), orgID, filter)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "OK")
}

// GetGoal godoc
//
//	@Summary		Get goal
//	@Description	Includes computed progress, the alignment parent as a
//	@Description	title-only reference, and the mean progress of aligned children.
//	@Tags			HRM / Performance
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			goalId	path	string	true	"Goal public ID"
//	@Success		200		{object}	response.OK{data=object{goal=GoalDetail}}
//	@Failure		403		{object}	response.Error	"GOAL_ACCESS_DENIED"
//	@Router			/organizations/{orgId}/hrm/performance/goals/{goalId} [get]
func (h *Handler) GetGoal(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		if err == errUnauthenticated {
			return h.err(c, err)
		}
		log.Error("performance: GetGoal", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	g, err := h.service.GetGoal(c.Context(), orgID, c.Params("goalId"), caller)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"goal": g}, "OK")
}

// CreateGoal godoc
//
//	@Summary		Create a goal
//	@Description	Omit employee_id to set a goal on your own record, which
//	@Description	needs only hrm.goals.set_own. Targeting another employee
//	@Description	additionally requires hrm.goals.manage and that they fall
//	@Description	within your scope tier.
//	@Tags			HRM / Performance
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string				true	"Organization ID"
//	@Param			body	body	CreateGoalRequest	true	"Goal"
//	@Success		201		{object}	response.Created{data=object{goal=Goal}}
//	@Failure		409		{object}	response.Error	"WEIGHT_EXCEEDS_CYCLE_TARGET or CYCLE_NOT_ACTIVE"
//	@Router			/organizations/{orgId}/hrm/performance/goals [post]
func (h *Handler) CreateGoal(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		if err == errUnauthenticated {
			return h.err(c, err)
		}
		log.Error("performance: CreateGoal", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	var req CreateGoalRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	g, err := h.service.CreateGoal(c.Context(), orgID, caller, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"goal": g}, "Goal created")
}

// UpdateGoal godoc
//
//	@Summary		Update a goal
//	@Description	current_value is deliberately not updatable here — progress
//	@Description	moves only through a check-in.
//	@Tags			HRM / Performance
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string				true	"Organization ID"
//	@Param			goalId	path	string				true	"Goal public ID"
//	@Param			body	body	UpdateGoalRequest	true	"Fields to update"
//	@Success		200		{object}	response.OK{data=object{goal=Goal}}
//	@Router			/organizations/{orgId}/hrm/performance/goals/{goalId} [patch]
func (h *Handler) UpdateGoal(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		if err == errUnauthenticated {
			return h.err(c, err)
		}
		log.Error("performance: UpdateGoal", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	var req UpdateGoalRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	g, err := h.service.UpdateGoal(c.Context(), orgID, c.Params("goalId"), caller, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"goal": g}, "Goal updated")
}

// DeleteGoal godoc
//
//	@Summary		Delete a goal
//	@Description	Refused once the goal has check-ins or aligned goals — cancel
//	@Description	it instead, which preserves history and frees its weight.
//	@Tags			HRM / Performance
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			goalId	path	string	true	"Goal public ID"
//	@Success		204
//	@Failure		409		{object}	response.Error	"GOAL_HAS_HISTORY"
//	@Router			/organizations/{orgId}/hrm/performance/goals/{goalId} [delete]
func (h *Handler) DeleteGoal(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		if err == errUnauthenticated {
			return h.err(c, err)
		}
		log.Error("performance: DeleteGoal", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	if err := h.service.DeleteGoal(c.Context(), orgID, c.Params("goalId"), caller); err != nil {
		return h.err(c, err)
	}
	return response.NoContent(c)
}

// SubmitGoal godoc
//
//	@Summary		Submit a goal (draft → active)
//	@Tags			HRM / Performance
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			goalId	path	string	true	"Goal public ID"
//	@Success		200		{object}	response.OK{data=object{goal=Goal}}
//	@Router			/organizations/{orgId}/hrm/performance/goals/{goalId}/submit [post]
func (h *Handler) SubmitGoal(c fiber.Ctx) error {
	return h.goalStatusAction(c, "SubmitGoal", func(orgID string, caller Caller) (*Goal, error) {
		return h.service.SubmitGoal(c.Context(), orgID, c.Params("goalId"), caller)
	}, "Goal submitted")
}

// CompleteGoal godoc
//
//	@Summary		Complete a goal
//	@Description	outcome is recorded, never inferred from progress.
//	@Tags			HRM / Performance
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string					true	"Organization ID"
//	@Param			goalId	path	string					true	"Goal public ID"
//	@Param			body	body	CompleteGoalRequest		true	"Outcome"
//	@Success		200		{object}	response.OK{data=object{goal=Goal}}
//	@Router			/organizations/{orgId}/hrm/performance/goals/{goalId}/complete [post]
func (h *Handler) CompleteGoal(c fiber.Ctx) error {
	var req CompleteGoalRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	return h.goalStatusAction(c, "CompleteGoal", func(orgID string, caller Caller) (*Goal, error) {
		return h.service.CompleteGoal(c.Context(), orgID, c.Params("goalId"), caller, req)
	}, "Goal completed")
}

// CancelGoal godoc
//
//	@Summary		Cancel a goal
//	@Description	Frees the goal's weight back to the employee's cycle budget.
//	@Tags			HRM / Performance
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string				true	"Organization ID"
//	@Param			goalId	path	string				true	"Goal public ID"
//	@Param			body	body	CancelGoalRequest	true	"Reason"
//	@Success		200		{object}	response.OK{data=object{goal=Goal}}
//	@Router			/organizations/{orgId}/hrm/performance/goals/{goalId}/cancel [post]
func (h *Handler) CancelGoal(c fiber.Ctx) error {
	var req CancelGoalRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	return h.goalStatusAction(c, "CancelGoal", func(orgID string, caller Caller) (*Goal, error) {
		return h.service.CancelGoal(c.Context(), orgID, c.Params("goalId"), caller, req)
	}, "Goal cancelled")
}

// goalStatusAction factors the org/caller resolution the three status
// transitions share, so each one is its own two-line handler.
func (h *Handler) goalStatusAction(c fiber.Ctx, op string, fn func(orgID string, caller Caller) (*Goal, error), msg string) error {
	log := logger.FromCtx(c)
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	caller, err := h.resolveCaller(c, orgID)
	if err != nil {
		if err == errUnauthenticated {
			return h.err(c, err)
		}
		log.Error("performance: "+op, slog.Any("error", err))
		return response.InternalServerError(c)
	}
	g, err := fn(orgID, caller)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"goal": g}, msg)
}
