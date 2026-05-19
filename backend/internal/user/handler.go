// backend/internal/user/handler.go
package user

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/response"
)

type Handler struct{ service Service }

func NewHandler(service Service) *Handler { return &Handler{service: service} }

// Me handles GET /api/v1/users/me and GET /api/v1/me.
func (h *Handler) Me(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}

	u, err := h.service.GetByID(c.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return response.NotFound(c, "USER_NOT_FOUND", "User not found")
		}
		slog.Error("user: Me error", slog.Any("error", err))
		return response.InternalServerError(c)
	}

	return response.OK(c, fiber.Map{"user": u.ToSafe()}, "OK")
}

// UpdateMe handles PATCH /api/v1/users/me and PATCH /api/v1/me.
func (h *Handler) UpdateMe(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}

	var req UpdateProfileRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	u, err := h.service.UpdateProfile(c.Context(), userID, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return response.NotFound(c, "USER_NOT_FOUND", "User not found")
		}
		slog.Error("user: UpdateMe error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"user": u.ToSafe()}, "Profile updated")
}
