// backend/internal/hrm/performance/checkins_handler.go
package performance

import (
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// ListCheckins godoc
//
//	@Summary		List a goal's check-in history
//	@Description	Append-only, newest first. Each row carries the progress
//	@Description	percent as reported at the time, unclamped.
//	@Tags			HRM / Performance
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string	true	"Organization ID"
//	@Param			goalId	path	string	true	"Goal public ID"
//	@Success		200		{object}	response.OK{data=CheckinListResponse}
//	@Failure		403		{object}	response.Error	"GOAL_ACCESS_DENIED"
//	@Router			/organizations/{orgId}/hrm/performance/goals/{goalId}/checkins [get]
func (h *Handler) ListCheckins(c fiber.Ctx) error {
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
		log.Error("performance: ListCheckins", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	limit, _ := strconv.Atoi(c.Query("limit", ""))
	offset, _ := strconv.Atoi(c.Query("offset", ""))

	res, err := h.service.ListCheckins(c.Context(), orgID, c.Params("goalId"), caller, limit, offset)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "OK")
}

// CreateCheckin godoc
//
//	@Summary		Record progress on a goal
//	@Description	The only way a goal's current_value moves. Still permitted
//	@Description	while the cycle is locked — locking freezes goal definitions,
//	@Description	not progress reporting.
//	@Tags			HRM / Performance
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			orgId	path	string					true	"Organization ID"
//	@Param			goalId	path	string					true	"Goal public ID"
//	@Param			body	body	CreateCheckinRequest	true	"Progress"
//	@Success		201		{object}	response.Created{data=CreateCheckinResult}
//	@Failure		409		{object}	response.Error	"GOAL_NOT_OPEN"
//	@Router			/organizations/{orgId}/hrm/performance/goals/{goalId}/checkins [post]
func (h *Handler) CreateCheckin(c fiber.Ctx) error {
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
		log.Error("performance: CreateCheckin", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	var req CreateCheckinRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	res, err := h.service.CreateCheckin(c.Context(), orgID, c.Params("goalId"), caller, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, res, "Check-in recorded")
}
