// backend/internal/hrm/compensation/bonuses_handler.go
package compensation

import (
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

func (h *Handler) ListBonuses(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.bonuses")
	if err != nil {
		log.Error("compensation: ListBonuses", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	filter := ListFilter{
		EmployeeID: c.Query("employee_id"), Scope: scopeTier, CallerUserID: userID,
	}
	if limit, err := strconv.Atoi(c.Query("limit", "")); err == nil {
		filter.Limit = limit
	}
	if offset, err := strconv.Atoi(c.Query("offset", "")); err == nil {
		filter.Offset = offset
	}
	res, err := h.service.ListBonuses(c.Context(), orgID, filter)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "OK")
}

func (h *Handler) GetBonus(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	b, err := h.service.GetBonus(c.Context(), orgID, c.Params("bonusId"))
	if err != nil {
		return h.err(c, err)
	}
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.bonuses")
	if err != nil {
		log.Error("compensation: GetBonus", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	allowed, err := h.scopeResolver.AuthorizeRecordAccess(c.Context(), scopeTier, orgID, userID, b.EmployeeID)
	if err != nil {
		log.Error("compensation: GetBonus", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	if !allowed {
		return h.err(c, ErrAccessDenied)
	}
	return response.OK(c, fiber.Map{"bonus": b}, "OK")
}

func (h *Handler) CreateBonus(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateBonusRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	b, err := h.service.CreateBonus(c.Context(), orgID, userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"bonus": b}, "Bonus created")
}

func (h *Handler) SubmitBonus(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	b, err := h.service.SubmitBonus(c.Context(), orgID, c.Params("bonusId"), userID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"bonus": b}, "Bonus submitted for approval")
}

func (h *Handler) CancelBonus(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	b, err := h.service.CancelBonus(c.Context(), orgID, c.Params("bonusId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"bonus": b}, "Bonus cancelled")
}
