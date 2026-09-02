// backend/internal/hrm/compensation/revisions_handler.go
package compensation

import (
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

func (h *Handler) ListCycles(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	list, err := h.service.ListCycles(c.Context(), orgID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"cycles": list}, "OK")
}

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

func (h *Handler) CreateCycle(c fiber.Ctx) error {
	userID, ok := requestUser(c)
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
	return response.Created(c, fiber.Map{"cycle": cy}, "Salary revision cycle created")
}

func (h *Handler) ComputeCycle(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	cy, err := h.service.ComputeCycle(c.Context(), orgID, c.Params("cycleId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"cycle": cy}, "Cycle computed")
}

func (h *Handler) SubmitCycle(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	cy, err := h.service.SubmitCycle(c.Context(), orgID, c.Params("cycleId"), userID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"cycle": cy}, "Cycle submitted for approval")
}

func (h *Handler) ApplyCycle(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	cy, err := h.service.ApplyCycle(c.Context(), orgID, c.Params("cycleId"), userID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"cycle": cy}, "Cycle applied")
}

func (h *Handler) ListRevisions(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.salary_revisions")
	if err != nil {
		log.Error("compensation: ListRevisions", slog.Any("error", err))
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
	res, err := h.service.ListRevisions(c.Context(), orgID, c.Params("cycleId"), filter)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, res, "OK")
}

func (h *Handler) GetRevision(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	rv, err := h.service.GetRevision(c.Context(), orgID, c.Params("revisionId"))
	if err != nil {
		return h.err(c, err)
	}
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.salary_revisions")
	if err != nil {
		log.Error("compensation: GetRevision", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	allowed, err := h.scopeResolver.AuthorizeRecordAccess(c.Context(), scopeTier, orgID, userID, rv.EmployeeID)
	if err != nil {
		log.Error("compensation: GetRevision", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	if !allowed {
		return h.err(c, ErrAccessDenied)
	}
	return response.OK(c, fiber.Map{"revision": rv}, "OK")
}

func (h *Handler) OverrideRevision(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req OverrideRevisionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	rv, err := h.service.OverrideRevision(c.Context(), orgID, c.Params("revisionId"), req)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"revision": rv}, "Revision overridden")
}
