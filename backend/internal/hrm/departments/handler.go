// backend/internal/hrm/departments/handler.go
package departments

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles HRM department HTTP endpoints.
type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// List handles GET /api/v1/organizations/:orgId/hrm/departments
// Requires: hrm.departments.view
// Query: active=true|false (default: all)
func (h *Handler) List(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}

	activeOnly := strings.ToLower(c.Query("active")) == "true"

	result, err := h.service.List(c.Context(), orgID, activeOnly)
	if err != nil {
		log.Error("departments: List error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, result, "OK")
}

// Create handles POST /api/v1/organizations/:orgId/hrm/departments
// Requires: hrm.departments.create
func (h *Handler) Create(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}

	var req CreateDepartmentRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	d, err := h.service.Create(c.Context(), orgID, userID, req)
	if err != nil {
		return h.deptError(c, err)
	}
	return response.Created(c, fiber.Map{"department": d}, "Department created")
}

// Get handles GET /api/v1/organizations/:orgId/hrm/departments/:deptId
// Requires: hrm.departments.view
func (h *Handler) Get(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	d, err := h.service.Get(c.Context(), orgID, c.Params("deptId"))
	if err != nil {
		return h.deptError(c, err)
	}
	return response.OK(c, fiber.Map{"department": d}, "OK")
}

// Update handles PATCH /api/v1/organizations/:orgId/hrm/departments/:deptId
// Requires: hrm.departments.update
func (h *Handler) Update(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}

	var req UpdateDepartmentRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	d, err := h.service.Update(c.Context(), orgID, c.Params("deptId"), req)
	if err != nil {
		return h.deptError(c, err)
	}
	return response.OK(c, fiber.Map{"department": d}, "Department updated")
}

// Delete handles DELETE /api/v1/organizations/:orgId/hrm/departments/:deptId
// Requires: hrm.departments.delete
func (h *Handler) Delete(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.Delete(c.Context(), orgID, c.Params("deptId")); err != nil {
		return h.deptError(c, err)
	}
	return response.NoContent(c)
}

func (h *Handler) deptError(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrDepartmentNotFound):
		return response.NotFound(c, "DEPARTMENT_NOT_FOUND", "Department not found")
	case errors.Is(err, ErrNameRequired):
		return response.BadRequest(c, "NAME_REQUIRED", "Department name is required")
	case errors.Is(err, ErrNameTooLong):
		return response.BadRequest(c, "NAME_TOO_LONG", "Name must not exceed 150 characters")
	case errors.Is(err, ErrNameConflict):
		return response.Conflict(c, "DEPARTMENT_NAME_CONFLICT", "A department with this name already exists")
	case errors.Is(err, ErrCircularParent):
		return response.BadRequest(c, "CIRCULAR_PARENT", "A department cannot be its own parent")
	default:
		log.Error("departments error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}
