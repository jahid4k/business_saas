package notifications

import (
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles self-service notification endpoints (mirrors /me: requireAuth only,
// scoped to the requesting user, no RBAC permission check).
type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func requestUserID(c fiber.Ctx) (uuid.UUID, bool) {
	idStr, ok := c.Locals("user_id").(string)
	if !ok || idStr == "" {
		return uuid.UUID{}, false
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.UUID{}, false
	}
	return id, true
}

// List handles GET /api/v1/notifications
func (h *Handler) List(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	uid, ok := requestUserID(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}

	limit, offset := 50, 0
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	result, err := h.service.ListInApp(c.Context(), uid, limit, offset)
	if err != nil {
		log.Error("notifications: List", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, result, "OK")
}

// MarkRead handles POST /api/v1/notifications/:id/read
func (h *Handler) MarkRead(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	uid, ok := requestUserID(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	notifID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "invalid notification id")
	}
	if err := h.service.MarkRead(c.Context(), uid, notifID); err != nil {
		log.Error("notifications: MarkRead", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, nil, "Marked as read")
}

// MarkAllRead handles POST /api/v1/notifications/read-all
func (h *Handler) MarkAllRead(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	uid, ok := requestUserID(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	if err := h.service.MarkAllRead(c.Context(), uid); err != nil {
		log.Error("notifications: MarkAllRead", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, nil, "All marked as read")
}

// ListPreferences handles GET /api/v1/notifications/preferences
func (h *Handler) ListPreferences(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	uid, ok := requestUserID(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	prefs, err := h.service.ListPreferences(c.Context(), uid)
	if err != nil {
		log.Error("notifications: ListPreferences", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"preferences": prefs}, "OK")
}

type updatePreferenceRequest struct {
	EventType string `json:"event_type"`
	Channel   string `json:"channel"`
	IsEnabled bool   `json:"is_enabled"`
}

// UpdatePreference handles PATCH /api/v1/notifications/preferences
func (h *Handler) UpdatePreference(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	uid, ok := requestUserID(c)
	if !ok {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	var req updatePreferenceRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "invalid request body")
	}
	if req.EventType == "" || req.Channel == "" {
		return response.BadRequest(c, "VALIDATION_ERROR", "event_type and channel are required")
	}
	if err := h.service.UpdatePreference(c.Context(), uid, req.EventType, req.Channel, req.IsEnabled); err != nil {
		log.Error("notifications: UpdatePreference", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, nil, "Preference updated")
}
