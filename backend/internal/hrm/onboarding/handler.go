// backend/internal/hrm/onboarding/handler.go
package onboarding

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/scope"
	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles the HRM-side onboarding checklist endpoints. Scope
// enforcement (view_own/view_team/view_all) lives here, not in the platform
// checklists package — platform.checklists has no concept of HRM's
// manager-chain scope tiers, and adding one would force
// TestPermissions_ScopeTiersSeeded to demand platform.checklists.view_*
// tiers nobody would hold. Reuses the tiers 00072 already seeded for
// hrm.employees, and the same AuthorizeRecordAccess resolver Phase 1 wired
// into every other HRM GET-by-ID handler.
type Handler struct {
	service       Service
	authz         authz.Service
	scopeResolver *scope.Resolver
}

func NewHandler(service Service, authzSvc authz.Service, scopeResolver *scope.Resolver) *Handler {
	return &Handler{service: service, authz: authzSvc, scopeResolver: scopeResolver}
}

// authorizeForEmployee resolves the caller's hrm.employees scope tier and
// checks record-level access to empID, writing a response and returning
// false if unauthorized or on error. Shared by List and Instantiate since
// both are keyed to the same :empId record.
func (h *Handler) authorizeForEmployee(c fiber.Ctx, log *slog.Logger, userID, orgID, empID string) (bool, error) {
	scopeTier, err := h.authz.ResolveScope(c.Context(), userID, orgID, "hrm.employees")
	if err != nil {
		log.Error("onboarding: resolve scope", slog.Any("error", err))
		return false, response.InternalServerError(c)
	}
	allowed, err := h.scopeResolver.AuthorizeRecordAccess(c.Context(), scopeTier, orgID, userID, empID)
	if err != nil {
		log.Error("onboarding: authorize record access", slog.Any("error", err))
		return false, response.InternalServerError(c)
	}
	if !allowed {
		return false, response.Forbidden(c, "RECORD_ACCESS_DENIED", "You do not have access to this record")
	}
	return true, nil
}

// List handles GET /organizations/:orgId/hrm/employees/:empId/checklists
func (h *Handler) List(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	empID := c.Params("empId")

	if allowed, resp := h.authorizeForEmployee(c, log, userID, orgID, empID); !allowed {
		return resp
	}

	result, err := h.service.ListForEmployee(c.Context(), orgID, empID)
	if err != nil {
		if errors.Is(err, ErrEmployeeNotFound) {
			return response.NotFound(c, "EMPLOYEE_NOT_FOUND", "Employee not found")
		}
		log.Error("onboarding: List", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, result, "OK")
}

// Instantiate handles POST /organizations/:orgId/hrm/employees/:empId/checklists
// — the manual retry path for the auto-hook on employees.Service.Create.
func (h *Handler) Instantiate(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	empID := c.Params("empId")

	if allowed, resp := h.authorizeForEmployee(c, log, userID, orgID, empID); !allowed {
		return resp
	}

	result, err := h.service.InstantiateForEmployee(c.Context(), orgID, empID, userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrEmployeeNotFound):
			return response.NotFound(c, "EMPLOYEE_NOT_FOUND", "Employee not found")
		case errors.Is(err, ErrNoDefaultTemplate):
			return response.NotFound(c, "NO_DEFAULT_TEMPLATE", "No default onboarding checklist template is configured for this organization")
		default:
			log.Error("onboarding: Instantiate", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}
	return response.Created(c, fiber.Map{"result": result}, "Onboarding checklist instantiated")
}
