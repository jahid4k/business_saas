// backend/internal/hrm/performance/cycles_handler.go
package performance

import (
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/response"
)

// ListCycles godoc
//
//	@Summary		List goal cycles
//	@Tags			HRM / Performance
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			status	query	string	false	"Filter by status (draft|active|locked|closed)"
//	@Success		200		{object}	response.OK{data=CycleListResponse}
//	@Router			/organizations/{orgId}/hrm/performance/goal-cycles [get]
func (h *Handler) ListCycles(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	filter := CycleListFilter{Status: c.Query("status")}
	if limit, err := strconv.Atoi(c.Query("limit", "")); err == nil {
		filter.Limit = limit
	}
	if offset, err := strconv.Atoi(c.Query("offset", "")); err == nil {
		filter.Offset = offset
	}
	res, err := h.service.ListCycles(c.Context(), orgID, filter)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "OK")
}

// GetCycle godoc
//
//	@Summary		Get goal cycle
//	@Tags			HRM / Performance
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			cycleId	path	string	true	"Cycle public ID"
//	@Success		200		{object}	response.OK{data=object{cycle=GoalCycle}}
//	@Router			/organizations/{orgId}/hrm/performance/goal-cycles/{cycleId} [get]
func (h *Handler) GetCycle(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	cy, err := h.service.GetCycle(c.Context(), orgID, c.Params("cycleId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"cycle": cy}, "OK")
}

// CreateCycle godoc
//
//	@Summary		Create goal cycle
//	@Description	weight_target defaults to 100 and is the denominator every
//	@Description	employee's goal weights must total before the cycle can lock.
//	@Tags			HRM / Performance
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string				true	"Organization ID"
//	@Param			body	body	CreateCycleRequest	true	"Cycle"
//	@Success		201		{object}	response.Created{data=object{cycle=GoalCycle}}
//	@Router			/organizations/{orgId}/hrm/performance/goal-cycles [post]
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
	return response.Created(c, fiber.Map{"cycle": cy}, "Goal cycle created")
}

// UpdateCycle godoc
//
//	@Summary		Update goal cycle
//	@Tags			HRM / Performance
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string				true	"Organization ID"
//	@Param			cycleId	path	string				true	"Cycle public ID"
//	@Param			body	body	UpdateCycleRequest	true	"Fields to update"
//	@Success		200		{object}	response.OK{data=object{cycle=GoalCycle}}
//	@Router			/organizations/{orgId}/hrm/performance/goal-cycles/{cycleId} [patch]
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
	return response.OK(c, fiber.Map{"cycle": cy}, "Goal cycle updated")
}

// ActivateCycle godoc
//
//	@Summary		Activate a goal cycle (draft → active)
//	@Tags			HRM / Performance
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			cycleId	path	string	true	"Cycle public ID"
//	@Success		200		{object}	response.OK{data=object{cycle=GoalCycle}}
//	@Router			/organizations/{orgId}/hrm/performance/goal-cycles/{cycleId}/activate [post]
func (h *Handler) ActivateCycle(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	cy, err := h.service.ActivateCycle(c.Context(), orgID, c.Params("cycleId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"cycle": cy}, "Goal cycle activated")
}

// LockCycle godoc
//
//	@Summary		Lock a goal cycle (freezes goal definitions)
//	@Description	Rejected with CYCLE_WEIGHTS_INCOMPLETE unless every employee's
//	@Description	goal weights total the cycle's weight_target. Check-ins keep
//	@Description	landing after a lock — only definitions freeze.
//	@Tags			HRM / Performance
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			cycleId	path	string	true	"Cycle public ID"
//	@Success		200		{object}	response.OK{data=object{cycle=GoalCycle}}
//	@Failure		409		{object}	response.Error	"CYCLE_WEIGHTS_INCOMPLETE"
//	@Router			/organizations/{orgId}/hrm/performance/goal-cycles/{cycleId}/lock [post]
func (h *Handler) LockCycle(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	cy, err := h.service.LockCycle(c.Context(), orgID, c.Params("cycleId"), userID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"cycle": cy}, "Goal cycle locked")
}

// CloseCycle godoc
//
//	@Summary		Close a goal cycle (fully immutable)
//	@Tags			HRM / Performance
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			cycleId	path	string	true	"Cycle public ID"
//	@Success		200		{object}	response.OK{data=object{cycle=GoalCycle}}
//	@Router			/organizations/{orgId}/hrm/performance/goal-cycles/{cycleId}/close [post]
func (h *Handler) CloseCycle(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	cy, err := h.service.CloseCycle(c.Context(), orgID, c.Params("cycleId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"cycle": cy}, "Goal cycle closed")
}

// GetCycleWeightAudit godoc
//
//	@Summary		Report which employees' goal weights do not total the cycle target
//	@Description	Shares its query with the lock gate, so what this reports is
//	@Description	exactly what blocks a lock.
//	@Tags			HRM / Performance
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			cycleId	path	string	true	"Cycle public ID"
//	@Success		200		{object}	response.OK{data=CycleWeightAudit}
//	@Router			/organizations/{orgId}/hrm/performance/goal-cycles/{cycleId}/weight-audit [get]
func (h *Handler) GetCycleWeightAudit(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	audit, err := h.service.GetCycleWeightAudit(c.Context(), orgID, c.Params("cycleId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, audit, "OK")
}
