// backend/internal/task/handler.go
package task

import (
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles task CRUD endpoints.
//
// Permission enforcement is done entirely by RequirePermission middleware
// before the handler is called — the handler trusts that and focuses only
// on HTTP concerns.
type Handler struct {
	service  Service
	authzSvc authz.Service
}

func NewHandler(service Service, authzSvc authz.Service) *Handler {
	return &Handler{service: service, authzSvc: authzSvc}
}

// List handles GET /api/v1/organizations/:orgId/tasks
// Requires: tasks.view
// Query params: status, assignedTo, sort, order (asc|desc), limit, offset
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

	filter := ListFilter{SortDesc: true}

	// Check if user has view_all permission. If not, restrict to their own tasks.
	hasViewAll, err := h.authzSvc.Can(c.Context(), userID, orgID, "tasks", "view_all")
	if err != nil {
		log.Error("task: List: failed to check view_all permission", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	if !hasViewAll {
		filter.InvolvedUserID = userID
	}

	if status := strings.TrimSpace(c.Query("status")); status != "" {
		s := TaskStatus(status)
		if !s.IsValid() {
			return response.BadRequest(c, "INVALID_STATUS", "status must be one of: todo, in_progress, done, cancelled")
		}
		filter.Status = s
	}
	filter.AssignedTo = strings.TrimSpace(c.Query("assignedTo"))

	if sort := strings.TrimSpace(c.Query("sort")); sort != "" {
		sf := SortField(sort)
		if !sf.IsValid() {
			return response.BadRequest(c, "INVALID_SORT", "sort must be one of: created_at, updated_at, due_date, title, status")
		}
		filter.SortBy = sf
	}
	if order := strings.ToLower(strings.TrimSpace(c.Query("order"))); order == "asc" {
		filter.SortDesc = false
	}

	if limit, err := strconv.Atoi(c.Query("limit", "")); err == nil {
		filter.Limit = limit
	}
	if offset, err := strconv.Atoi(c.Query("offset", "")); err == nil {
		filter.Offset = offset
	}

	result, err := h.service.List(c.Context(), orgID, filter)
	if err != nil {
		log.Error("task: List error", slog.Any("error", err))
		return response.InternalServerError(c)
	}

	return response.OK(c, result, "OK")
}

// Create handles POST /api/v1/organizations/:orgId/tasks
// Requires: tasks.create
func (h *Handler) Create(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}

	var req CreateTaskRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	t, err := h.service.Create(c.Context(), orgID, userID, req)
	if err != nil {
		return h.taskError(c, err)
	}

	return response.Created(c, fiber.Map{"task": t}, "Task created")
}

// Get handles GET /api/v1/organizations/:orgId/tasks/:taskId
// Requires: tasks.view
func (h *Handler) Get(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}

	t, err := h.service.Get(c.Context(), orgID, c.Params("taskId"))
	if err != nil {
		return h.taskError(c, err)
	}

	return response.OK(c, fiber.Map{"task": t}, "OK")
}

// Update handles PATCH /api/v1/organizations/:orgId/tasks/:taskId
// Requires: tasks.update
func (h *Handler) Update(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}

	var req UpdateTaskRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	t, err := h.service.Update(c.Context(), orgID, c.Params("taskId"), req)
	if err != nil {
		return h.taskError(c, err)
	}

	return response.OK(c, fiber.Map{"task": t}, "Task updated")
}

// Delete handles DELETE /api/v1/organizations/:orgId/tasks/:taskId
// Requires: tasks.delete
func (h *Handler) Delete(c fiber.Ctx) error {
	orgID, ok := middleware.OrganizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}

	if err := h.service.Delete(c.Context(), orgID, c.Params("taskId")); err != nil {
		return h.taskError(c, err)
	}

	return response.NoContent(c)
}

// taskError maps service-layer sentinel errors to HTTP responses. Centralised
// here so List/Get/Create/Update/Delete all map consistently — mirrors
// authz.Handler.authzError.
func (h *Handler) taskError(c fiber.Ctx, err error) error {
	log := logger.FromCtx(c)
	switch {
	case errors.Is(err, ErrNotFound):
		return response.NotFound(c, "TASK_NOT_FOUND", "Task not found")
	case errors.Is(err, ErrTitleRequired):
		return response.BadRequest(c, "TITLE_REQUIRED", "Title is required")
	case errors.Is(err, ErrTitleTooLong):
		return response.BadRequest(c, "TITLE_TOO_LONG", "Title must not exceed 255 characters")
	case errors.Is(err, ErrDescriptionTooLong):
		return response.BadRequest(c, "DESCRIPTION_TOO_LONG", "Description must not exceed 2000 characters")
	case errors.Is(err, ErrInvalidStatus):
		return response.BadRequest(c, "INVALID_STATUS", "Status must be one of: todo, in_progress, done, cancelled")
	case errors.Is(err, ErrInvalidDueDate):
		return response.BadRequest(c, "INVALID_DUE_DATE", "dueDate must be a valid RFC3339 timestamp")
	case errors.Is(err, ErrAssigneeNotFound):
		return response.BadRequest(c, "ASSIGNEE_NOT_FOUND", "assignedTo must be an active member of this organization")
	default:
		log.Error("task error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
}
