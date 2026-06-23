// backend/internal/security/handler.go
package security

import (
	"errors"
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

type Handler struct{ service Service }

func NewHandler(service Service) *Handler { return &Handler{service: service} }

func (h *Handler) ListSessions(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := organizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	limit, _ := strconv.Atoi(c.Query("limit", "200"))
	sessions, err := h.service.ListSessions(c.Context(), orgID, limit)
	if err != nil {
		log.Error("security: ListSessions error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"sessions": sessions}, "OK")
}

func (h *Handler) RevokeSession(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := organizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	if err := h.service.RevokeSession(c.Context(), orgID, c.Params("sessionId")); err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return response.NotFound(c, "SESSION_NOT_FOUND", "Session not found")
		}
		log.Error("security: RevokeSession error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.NoContent(c)
}

func (h *Handler) ListLoginEvents(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := organizationIDFromCtx(c)
	if !ok {
		return response.BadRequest(c, "NO_ORGANIZATION_CONTEXT", "Organization context is required")
	}
	limit, _ := strconv.Atoi(c.Query("limit", "100"))
	events, err := h.service.ListLoginEvents(c.Context(), orgID, limit)
	if err != nil {
		log.Error("security: ListLoginEvents error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"loginEvents": events}, "OK")
}

func organizationIDFromCtx(c fiber.Ctx) (string, bool) {
	orgID, _ := c.Locals("organization_id").(string)
	if orgID == "" {
		orgID, _ = c.Locals("business_id").(string)
	}
	return orgID, orgID != ""
}
