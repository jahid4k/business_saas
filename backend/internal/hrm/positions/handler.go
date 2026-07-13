// backend/internal/hrm/positions/handler.go
package positions

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles HRM position HTTP endpoints.
type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// List handles GET /api/v1/organizations/:orgId/hrm/positions
// Requires: hrm.positions.view
// Query: department_id (filter), active=true|false
func (h *Handler) List(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}

	departmentID := strings.TrimSpace(c.Query("department_id"))
	activeOnly := strings.ToLower(c.Query("active")) == "true"

	result, err := h.service.List(c.Context(), orgID, departmentID, activeOnly)
	if err != nil {
		log.Error("positions: List error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, result, "OK")
}

// Create handles POST /api/v1/organizations/:orgId/hrm/positions
// Requires: hrm.positions.create
func (h *Handler) Create(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}

	var req CreatePositionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	p, err := h.service.Create(c.Context(), orgID, userID, req)
	if err != nil {
		return h.posError(c, err)
	}
	return response.Created(c, fiber.Map{"position": p}, "Position created")
}

// Get handles GET /api/v1/organizations/:orgId/hrm/positions/:posId
// Requires: hrm.positions.view
func (h *Handler) Get(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	p, err := h.service.Get(c.Context(), orgID, c.Params("posId"))
	if err != nil {
		return h.posError(c, err)
	}
	return response.OK(c, fiber.Map{"position": p}, "OK")
}

// Update handles PATCH /api/v1/organizations/:orgId/hrm/positions/:posId
// Requires: hrm.positions.update
func (h *Handler) Update(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}

	var req UpdatePositionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	p, err := h.service.Update(c.Context(), orgID, c.Params("posId"), req)
	if err != nil {
		return h.posError(c, err)
	}
	return response.OK(c, fiber.Map{"position": p}, "Position updated")
}

// Delete handles DELETE /api/v1/organizations/:orgId/hrm/positions/:posId
// Requires: hrm.positions.delete
func (h *Handler) Delete(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.Delete(c.Context(), orgID, c.Params("posId")); err != nil {
		return h.posError(c, err)
	}
	return response.NoContent(c)
}

func (h *Handler) posError(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrPositionNotFound):
		return response.NotFound(c, "POSITION_NOT_FOUND", "Position not found")
	case errors.Is(err, ErrTitleRequired):
		return response.BadRequest(c, "TITLE_REQUIRED", "Position title is required")
	case errors.Is(err, ErrTitleTooLong):
		return response.BadRequest(c, "TITLE_TOO_LONG", "Title must not exceed 150 characters")
	case errors.Is(err, ErrTitleConflict):
		return response.Conflict(c, "POSITION_TITLE_CONFLICT", "A position with this title already exists")
	default:
		log.Error("positions error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}
