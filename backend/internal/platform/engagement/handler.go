// backend/internal/platform/engagement/handler.go
package engagement

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles all engagement HTTP endpoints.
// The module constant "crm" is injected per-route in routes.go so this
// handler stays module-agnostic and reusable by ERP, HRM, etc.
type Handler struct {
	service Service
	module  string // "crm" | "erp" | "hrm" — set at construction time
}

// NewHandler creates a new engagement Handler bound to a specific module.
func NewHandler(service Service, module string) *Handler {
	return &Handler{service: service, module: module}
}

func orgID(c fiber.Ctx) string  { return c.Params("orgId") }
func userID(c fiber.Ctx) string { id, _ := c.Locals("user_id").(string); return id }

// ============================================================
// Timeline
// ============================================================

// GetTimeline handles GET /api/v1/organizations/:orgId/crm/<entity>/:entityId/timeline
// relatedType and relatedID come from the calling route via query params
// set by upstream middleware, or directly from path params.
func (h *Handler) GetTimeline(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	relatedType := c.Query("related_type")
	relatedID := c.Query("related_id")
	if relatedType == "" || relatedID == "" {
		return response.BadRequest(c, "MISSING_PARAMS", "related_type and related_id are required")
	}
	timeline, err := h.service.GetTimeline(c.Context(), orgID(c), relatedType, relatedID)
	if err != nil {
		log.Error("engagement: GetTimeline", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, timeline, "OK")
}

// ============================================================
// Notes
// ============================================================

func (h *Handler) ListNotesByRelated(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	notes, err := h.service.ListNotesByRelated(
		c.Context(), orgID(c),
		c.Query("related_type"), c.Query("related_id"),
	)
	if err != nil {
		log.Error("engagement: ListNotesByRelated", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"notes": notes}, "OK")
}

func (h *Handler) GetNote(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	note, err := h.service.GetNote(c.Context(), orgID(c), c.Params("noteId"))
	if err != nil {
		if errors.Is(err, ErrNoteNotFound) {
			return response.NotFound(c, "NOTE_NOT_FOUND", "Note not found")
		}
		log.Error("engagement: GetNote", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"note": note}, "OK")
}

func (h *Handler) CreateNote(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req CreateNoteRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	note, err := h.service.CreateNote(c.Context(), orgID(c), userID(c), h.module, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrContentRequired):
			return response.BadRequest(c, "CONTENT_REQUIRED", "content is required")
		case errors.Is(err, ErrRelatedTypeRequired):
			return response.BadRequest(c, "RELATED_TYPE_REQUIRED", "related_type is required")
		case errors.Is(err, ErrRelatedIDRequired):
			return response.BadRequest(c, "RELATED_ID_REQUIRED", "related_id is required")
		default:
			log.Error("engagement: CreateNote", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}
	return response.Created(c, fiber.Map{"note": note}, "Note created")
}

func (h *Handler) UpdateNote(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req UpdateNoteRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	note, err := h.service.UpdateNote(c.Context(), orgID(c), c.Params("noteId"), req)
	if err != nil {
		if errors.Is(err, ErrNoteNotFound) {
			return response.NotFound(c, "NOTE_NOT_FOUND", "Note not found")
		}
		log.Error("engagement: UpdateNote", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"note": note}, "Note updated")
}

func (h *Handler) DeleteNote(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	if err := h.service.DeleteNote(c.Context(), orgID(c), c.Params("noteId")); err != nil {
		if errors.Is(err, ErrNoteNotFound) {
			return response.NotFound(c, "NOTE_NOT_FOUND", "Note not found")
		}
		log.Error("engagement: DeleteNote", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.NoContent(c)
}

// ============================================================
// Tasks
// ============================================================

func (h *Handler) ListTasks(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	result, err := h.service.ListTasksByOrg(c.Context(), orgID(c))
	if err != nil {
		log.Error("engagement: ListTasks", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, result, "OK")
}

func (h *Handler) ListTasksByRelated(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	tasks, err := h.service.ListTasksByRelated(
		c.Context(), orgID(c),
		c.Query("related_type"), c.Query("related_id"),
	)
	if err != nil {
		log.Error("engagement: ListTasksByRelated", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"tasks": tasks}, "OK")
}

func (h *Handler) GetTask(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	task, err := h.service.GetTask(c.Context(), orgID(c), c.Params("taskId"))
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return response.NotFound(c, "TASK_NOT_FOUND", "Task not found")
		}
		log.Error("engagement: GetTask", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"task": task}, "OK")
}

func (h *Handler) CreateTask(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req CreateTaskRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	task, err := h.service.CreateTask(c.Context(), orgID(c), userID(c), h.module, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrTitleRequired):
			return response.BadRequest(c, "TITLE_REQUIRED", "title is required")
		case errors.Is(err, ErrInvalidPriority):
			return response.BadRequest(c, "INVALID_PRIORITY", "priority must be: low, medium, or high")
		default:
			log.Error("engagement: CreateTask", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}
	return response.Created(c, fiber.Map{"task": task}, "Task created")
}

func (h *Handler) UpdateTask(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req UpdateTaskRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	task, err := h.service.UpdateTask(c.Context(), orgID(c), c.Params("taskId"), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrTaskNotFound):
			return response.NotFound(c, "TASK_NOT_FOUND", "Task not found")
		case errors.Is(err, ErrInvalidPriority):
			return response.BadRequest(c, "INVALID_PRIORITY", "priority must be: low, medium, or high")
		default:
			log.Error("engagement: UpdateTask", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}
	return response.OK(c, fiber.Map{"task": task}, "Task updated")
}

func (h *Handler) DeleteTask(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	if err := h.service.DeleteTask(c.Context(), orgID(c), c.Params("taskId")); err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return response.NotFound(c, "TASK_NOT_FOUND", "Task not found")
		}
		log.Error("engagement: DeleteTask", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.NoContent(c)
}

func (h *Handler) CompleteTask(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	task, err := h.service.CompleteTask(c.Context(), orgID(c), c.Params("taskId"))
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return response.NotFound(c, "TASK_NOT_FOUND", "Task not found")
		}
		log.Error("engagement: CompleteTask", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"task": task}, "Task completed")
}

func (h *Handler) ReopenTask(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	task, err := h.service.ReopenTask(c.Context(), orgID(c), c.Params("taskId"))
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return response.NotFound(c, "TASK_NOT_FOUND", "Task not found")
		}
		log.Error("engagement: ReopenTask", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"task": task}, "Task reopened")
}

func (h *Handler) AssignTask(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req AssignTaskRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	task, err := h.service.AssignTask(c.Context(), orgID(c), c.Params("taskId"), req)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return response.NotFound(c, "TASK_NOT_FOUND", "Task not found")
		}
		log.Error("engagement: AssignTask", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"task": task}, "Task assigned")
}

// ============================================================
// Activities
// ============================================================

func (h *Handler) ListActivities(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	result, err := h.service.ListActivitiesByOrg(c.Context(), orgID(c))
	if err != nil {
		log.Error("engagement: ListActivities", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, result, "OK")
}

func (h *Handler) GetActivity(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	activity, err := h.service.GetActivity(c.Context(), orgID(c), c.Params("activityId"))
	if err != nil {
		if errors.Is(err, ErrActivityNotFound) {
			return response.NotFound(c, "ACTIVITY_NOT_FOUND", "Activity not found")
		}
		log.Error("engagement: GetActivity", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"activity": activity}, "OK")
}

func (h *Handler) CreateActivity(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req CreateActivityRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	activity, err := h.service.CreateActivity(c.Context(), orgID(c), userID(c), h.module, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrTypeRequired):
			return response.BadRequest(c, "TYPE_REQUIRED", "type is required")
		case errors.Is(err, ErrInvalidActivityType):
			return response.BadRequest(c, "INVALID_TYPE", "type must be: call, email, meeting, note, task, or other")
		case errors.Is(err, ErrSubjectRequired):
			return response.BadRequest(c, "SUBJECT_REQUIRED", "subject is required")
		default:
			log.Error("engagement: CreateActivity", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}
	return response.Created(c, fiber.Map{"activity": activity}, "Activity created")
}

func (h *Handler) UpdateActivity(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req UpdateActivityRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	activity, err := h.service.UpdateActivity(c.Context(), orgID(c), c.Params("activityId"), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrActivityNotFound):
			return response.NotFound(c, "ACTIVITY_NOT_FOUND", "Activity not found")
		case errors.Is(err, ErrInvalidActivityType):
			return response.BadRequest(c, "INVALID_TYPE", "type must be: call, email, meeting, note, task, or other")
		default:
			log.Error("engagement: UpdateActivity", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}
	return response.OK(c, fiber.Map{"activity": activity}, "Activity updated")
}

func (h *Handler) DeleteActivity(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	if err := h.service.DeleteActivity(c.Context(), orgID(c), c.Params("activityId")); err != nil {
		if errors.Is(err, ErrActivityNotFound) {
			return response.NotFound(c, "ACTIVITY_NOT_FOUND", "Activity not found")
		}
		log.Error("engagement: DeleteActivity", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.NoContent(c)
}

// ============================================================
// Email Logs
// ============================================================

func (h *Handler) ListEmailLogs(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	result, err := h.service.ListEmailLogsByOrg(c.Context(), orgID(c))
	if err != nil {
		log.Error("engagement: ListEmailLogs", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, result, "OK")
}

func (h *Handler) GetEmailLog(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	email, err := h.service.GetEmailLog(c.Context(), orgID(c), c.Params("emailId"))
	if err != nil {
		if errors.Is(err, ErrEmailLogNotFound) {
			return response.NotFound(c, "EMAIL_LOG_NOT_FOUND", "Email log not found")
		}
		log.Error("engagement: GetEmailLog", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"email": email}, "OK")
}

func (h *Handler) CreateEmailLog(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req CreateEmailLogRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	email, err := h.service.CreateEmailLog(c.Context(), orgID(c), userID(c), h.module, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrSubjectRequired):
			return response.BadRequest(c, "SUBJECT_REQUIRED", "subject is required")
		case errors.Is(err, ErrFromEmailRequired):
			return response.BadRequest(c, "FROM_EMAIL_REQUIRED", "from_email is required")
		case errors.Is(err, ErrToEmailRequired):
			return response.BadRequest(c, "TO_EMAIL_REQUIRED", "to_email is required")
		default:
			log.Error("engagement: CreateEmailLog", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}
	return response.Created(c, fiber.Map{"email": email}, "Email log created")
}

func (h *Handler) DeleteEmailLog(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	if err := h.service.DeleteEmailLog(c.Context(), orgID(c), c.Params("emailId")); err != nil {
		if errors.Is(err, ErrEmailLogNotFound) {
			return response.NotFound(c, "EMAIL_LOG_NOT_FOUND", "Email log not found")
		}
		log.Error("engagement: DeleteEmailLog", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.NoContent(c)
}
