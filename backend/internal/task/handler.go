package task

import (
	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles task CRUD endpoints.
// Each route is protected by RequirePermission middleware — the handler
// itself does NOT re-check permissions. Middleware is the enforcement layer.
type Handler struct {
	service Service
}

// NewHandler creates a new task Handler.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// List handles GET /api/v1/tasks
// Requires: tasks.read
// STATUS: Phase 1-E stub.
func (h *Handler) List(c fiber.Ctx) error {
	// TODO (Phase 1-E):
	// 1. Extract business_id from c.Locals("business_id")
	// 2. Call h.service.List(ctx, businessID)
	// 3. Return TaskListResponse
	return response.NotImplemented(c)
}

// Create handles POST /api/v1/tasks
// Requires: tasks.create
// STATUS: Phase 1-E stub.
func (h *Handler) Create(c fiber.Ctx) error {
	// TODO (Phase 1-E):
	// 1. Extract user_id and business_id from c.Locals
	// 2. Parse and validate CreateTaskRequest
	// 3. Call h.service.Create(ctx, businessID, userID, req)
	// 4. Return 201 Created with the new task
	return response.NotImplemented(c)
}

// Get handles GET /api/v1/tasks/:id
// Requires: tasks.read
// STATUS: Phase 1-E stub.
func (h *Handler) Get(c fiber.Ctx) error {
	// TODO (Phase 1-E):
	// 1. Extract business_id from c.Locals
	// 2. Get task ID from c.Params("id")
	// 3. Call h.service.GetByID(ctx, businessID, taskID)
	// 4. Return 404 if not found OR if business_id does not match (tenant isolation)
	return response.NotImplemented(c)
}

// Update handles PATCH /api/v1/tasks/:id
// Requires: tasks.update
// STATUS: Phase 1-E stub.
func (h *Handler) Update(c fiber.Ctx) error {
	// TODO (Phase 1-E):
	// 1. Extract business_id from c.Locals
	// 2. Get task ID from c.Params("id")
	// 3. Parse and validate UpdateTaskRequest
	// 4. Call h.service.Update(ctx, businessID, taskID, req)
	// 5. Verify task belongs to business before updating (tenant isolation)
	return response.NotImplemented(c)
}

// Delete handles DELETE /api/v1/tasks/:id
// Requires: tasks.delete
// STATUS: Phase 1-E stub.
func (h *Handler) Delete(c fiber.Ctx) error {
	// TODO (Phase 1-E):
	// 1. Extract business_id from c.Locals
	// 2. Get task ID from c.Params("id")
	// 3. Call h.service.Delete(ctx, businessID, taskID)
	// 4. Verify task belongs to business before deleting (tenant isolation)
	// 5. Return 204 No Content
	return response.NotImplemented(c)
}
