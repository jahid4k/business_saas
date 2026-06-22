// backend/internal/platform/engagement/handler_subroutes.go
// Extra handler methods used by CRM (and future module) route adapters.
// These are sub-resource list endpoints that accept related_type + related_id
// as query params, set by the calling route's inline closure.
package engagement

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// ListTasksByRelated handles GET requests scoped to a specific entity.
// Used by: /crm/contacts/:contactId/tasks, /crm/deals/:dealId/tasks, etc.
// The calling route sets ?related_type=crm.deal&related_id=<id> before forwarding.
// func (h *Handler) ListTasksByRelated(c fiber.Ctx) error {
// 	tasks, err := h.service.ListTasksByRelated(
// 		c.Context(), orgID(c),
// 		c.Query("related_type"), c.Query("related_id"),
// 	)
// 	if err != nil {
// 		log.Error("engagement: ListTasksByRelated", slog.Any("error", err))
// 		return response.InternalServerError(c)
// 	}
// 	return response.OK(c, fiber.Map{"tasks": tasks}, "OK")
// }

// ListActivitiesByRelated handles GET requests scoped to a specific entity.
// Used by: /crm/contacts/:contactId/activities, /crm/deals/:dealId/activities, etc.
func (h *Handler) ListActivitiesByRelated(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	activities, err := h.service.ListActivitiesByRelated(
		c.Context(), orgID(c),
		c.Query("related_type"), c.Query("related_id"),
	)
	if err != nil {
		log.Error("engagement: ListActivitiesByRelated", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"activities": activities}, "OK")
}

// ListEmailLogsByRelated handles GET requests scoped to a specific entity.
// Used by: /crm/contacts/:contactId/emails, /crm/deals/:dealId/emails, etc.
func (h *Handler) ListEmailLogsByRelated(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	emails, err := h.service.ListEmailLogsByRelated(
		c.Context(), orgID(c),
		c.Query("related_type"), c.Query("related_id"),
	)
	if err != nil {
		log.Error("engagement: ListEmailLogsByRelated", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"emails": emails}, "OK")
}
