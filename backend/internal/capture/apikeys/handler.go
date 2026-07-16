package apikeys

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

func orgID(c fiber.Ctx) string  { return c.Params("orgId") }
func userID(c fiber.Ctx) string { id, _ := c.Locals("user_id").(string); return id }

func (h *Handler) CreateKey(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req CreateKeyRequest
	if err := c.Bind().Body(&req); err != nil {
		return response.BadRequest(c, "BAD_REQUEST", "Invalid JSON payload")
	}

	res, err := h.service.GenerateKey(c.Context(), orgID(c), userID(c), req)
	if err != nil {
		log.Error("apikeys: CreateKey", slog.Any("error", err))
		return response.InternalServerError(c)
	}

	return response.Created(c, res, "API Key created successfully")
}

func (h *Handler) ListKeys(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	keys, err := h.service.ListKeys(c.Context(), orgID(c))
	if err != nil {
		log.Error("apikeys: ListKeys", slog.Any("error", err))
		return response.InternalServerError(c)
	}

	return response.OK(c, keys, "OK")
}

func (h *Handler) RevokeKey(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	keyID := c.Params("keyId")

	if err := h.service.RevokeKey(c.Context(), orgID(c), keyID); err != nil {
		if err == ErrKeyNotFound {
			return response.NotFound(c, "NOT_FOUND", "API Key not found or already revoked")
		}
		log.Error("apikeys: RevokeKey", slog.Any("error", err))
		return response.InternalServerError(c)
	}

	return response.OK(c, nil, "API Key revoked successfully")
}
