// backend/internal/task/handler.go
package task

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles task CRUD endpoints.
//
// Permission enforcement is done entirely by RequirePermission middleware
// before the handler is called. The handler itself trusts that the middleware
// has already verified the permission and focuses only on HTTP concerns.
type Handler struct {
	service Service
}

// NewHandler creates a new task Handler.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// List handles GET /api/v1/tasks
// Requires: tasks.read
func (h *Handler) List(c fiber.Ctx) error {
	businessID, ok := c.Locals("business_id").(string)
	if !ok || businessID == "" {
		return response.BadRequest(c, "NO_BUSINESS_CONTEXT", "Business context is required")
	}

	result, err := h.service.List(c.Context(), businessID)
	if err != nil {
		slog.Error("task: List error", slog.Any("error", err))
		return response.InternalServerError(c)
	}

	return response.OK(c, result, "OK")
}

// Create handles POST /api/v1/tasks
// Requires: tasks.create
func (h *Handler) Create(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}

	businessID, ok := c.Locals("business_id").(string)
	if !ok || businessID == "" {
		return response.BadRequest(c, "NO_BUSINESS_CONTEXT", "Business context is required")
	}

	var req CreateTaskRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	t, err := h.service.Create(c.Context(), businessID, userID, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrTitleRequired):
			return response.BadRequest(c, "TITLE_REQUIRED", "Title is required")
		case errors.Is(err, ErrTitleTooLong):
			return response.BadRequest(c, "TITLE_TOO_LONG", "Title must not exceed 255 characters")
		case errors.Is(err, ErrDescriptionTooLong):
			return response.BadRequest(c, "DESCRIPTION_TOO_LONG", "Description must not exceed 2000 characters")
		case errors.Is(err, ErrInvalidStatus):
			return response.BadRequest(c, "INVALID_STATUS", "Status must be one of: todo, in_progress, done")
		default:
			slog.Error("task: Create error", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}

	return response.Created(c, fiber.Map{"task": t}, "Task created")
}

// Get handles GET /api/v1/tasks/:id
// Requires: tasks.read
func (h *Handler) Get(c fiber.Ctx) error {
	businessID, ok := c.Locals("business_id").(string)
	if !ok || businessID == "" {
		return response.BadRequest(c, "NO_BUSINESS_CONTEXT", "Business context is required")
	}

	taskID := c.Params("id")
	if taskID == "" {
		return response.BadRequest(c, "MISSING_ID", "Task ID is required")
	}

	t, err := h.service.GetByID(c.Context(), businessID, taskID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return response.NotFound(c, "TASK_NOT_FOUND", "Task not found")
		}
		slog.Error("task: Get error", slog.Any("error", err))
		return response.InternalServerError(c)
	}

	return response.OK(c, fiber.Map{"task": t}, "OK")
}

// Update handles PATCH /api/v1/tasks/:id
// Requires: tasks.update
func (h *Handler) Update(c fiber.Ctx) error {
	businessID, ok := c.Locals("business_id").(string)
	if !ok || businessID == "" {
		return response.BadRequest(c, "NO_BUSINESS_CONTEXT", "Business context is required")
	}

	taskID := c.Params("id")
	if taskID == "" {
		return response.BadRequest(c, "MISSING_ID", "Task ID is required")
	}

	var req UpdateTaskRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	t, err := h.service.Update(c.Context(), businessID, taskID, req)
	if err != nil {
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
			return response.BadRequest(c, "INVALID_STATUS", "Status must be one of: todo, in_progress, done")
		default:
			slog.Error("task: Update error", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}

	return response.OK(c, fiber.Map{"task": t}, "Task updated")
}

// Delete handles DELETE /api/v1/tasks/:id
// Requires: tasks.delete
func (h *Handler) Delete(c fiber.Ctx) error {
	businessID, ok := c.Locals("business_id").(string)
	if !ok || businessID == "" {
		return response.BadRequest(c, "NO_BUSINESS_CONTEXT", "Business context is required")
	}

	taskID := c.Params("id")
	if taskID == "" {
		return response.BadRequest(c, "MISSING_ID", "Task ID is required")
	}

	if err := h.service.Delete(c.Context(), businessID, taskID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return response.NotFound(c, "TASK_NOT_FOUND", "Task not found")
		}
		slog.Error("task: Delete error", slog.Any("error", err))
		return response.InternalServerError(c)
	}

	return response.NoContent(c)
}
