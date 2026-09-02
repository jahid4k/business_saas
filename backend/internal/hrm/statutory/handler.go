// backend/internal/hrm/statutory/handler.go
package statutory

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles HRM statutory HTTP endpoints. hrm.statutory is NOT
// scope-tiered — see migration 00103's header — so, unlike
// compensation/loans/reimbursements, this holds no authz.Service or
// scope.Resolver.
type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler { return &Handler{service: service} }

func requestUser(c fiber.Ctx) (string, bool) { return middleware.UserIDFromCtx(c) }
func requestOrg(c fiber.Ctx) (string, bool)  { return middleware.OrganizationIDFromCtx(c) }

func (h *Handler) err(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrRuleNotFound):
		return response.NotFound(c, "RULE_NOT_FOUND", "Statutory rule not found")
	case errors.Is(err, ErrInvalidRuleType):
		return response.BadRequest(c, "INVALID_RULE_TYPE", err.Error())
	case errors.Is(err, ErrInvalidBase):
		return response.BadRequest(c, "INVALID_BASE_VARIABLE", err.Error())
	case errors.Is(err, ErrInvalidAmount):
		return response.BadRequest(c, "INVALID_AMOUNT", err.Error())
	default:
		log.Error("statutory: error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}

func (h *Handler) ListRules(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	list, err := h.service.ListRules(c.Context(), orgID)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"rules": list}, "OK")
}

func (h *Handler) GetRule(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	r, err := h.service.GetRule(c.Context(), orgID, c.Params("ruleId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"rule": r}, "OK")
}

func (h *Handler) CreateRule(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateRuleRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	r, err := h.service.CreateRule(c.Context(), orgID, userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"rule": r}, "Statutory rule created")
}

func (h *Handler) ActivateRule(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	r, err := h.service.SetRuleActive(c.Context(), orgID, c.Params("ruleId"), true)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"rule": r}, "Rule activated")
}

func (h *Handler) DeactivateRule(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	r, err := h.service.SetRuleActive(c.Context(), orgID, c.Params("ruleId"), false)
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"rule": r}, "Rule deactivated")
}

func (h *Handler) ListSlabs(c fiber.Ctx) error {
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	list, err := h.service.ListSlabs(c.Context(), orgID, c.Params("ruleId"))
	if err != nil {
		return h.err(c, err)
	}
	return response.OK(c, fiber.Map{"slabs": list}, "OK")
}

func (h *Handler) CreateSlab(c fiber.Ctx) error {
	userID, ok := requestUser(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := requestOrg(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	var req CreateSlabRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	s, err := h.service.CreateSlab(c.Context(), orgID, c.Params("ruleId"), userID, req)
	if err != nil {
		return h.err(c, err)
	}
	return response.Created(c, fiber.Map{"slab": s}, "Slab created")
}
