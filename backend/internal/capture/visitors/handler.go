package visitors

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// Identify receives frontend telemetry (like Segment Identify/Page).
func (h *Handler) Identify(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	orgID, ok := c.Locals("org_id").(string)
	if !ok || orgID == "" {
		return response.Unauthorized(c, "UNAUTHORIZED", "Missing org identifier")
	}

	var req IdentifyRequest
	if err := c.Bind().Body(&req); err != nil {
		return response.BadRequest(c, "BAD_REQUEST", "Invalid JSON payload")
	}

	if req.SessionID == "" {
		return response.BadRequest(c, "BAD_REQUEST", "session_id is required")
	}

	ip := c.IP()
	userAgent := string(c.Request().Header.UserAgent())

	if err := h.service.IdentifyVisitor(c.Context(), orgID, ip, userAgent, req); err != nil {
		log.Error("visitors: Identify", slog.Any("error", err))
		return response.InternalServerError(c)
	}

	return c.SendStatus(fiber.StatusOK)
}

func (h *Handler) ListVisitors(c fiber.Ctx) error {
	orgID := c.Params("orgId")
	log := logger.FromCtx(c)

	visitors, err := h.service.ListVisitors(c.Context(), orgID)
	if err != nil {
		log.Error("visitors: ListVisitors", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, visitors, "OK")
}
