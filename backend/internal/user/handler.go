// backend/internal/user/handler.go
package user

import (
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

type Handler struct {
	service       Service
	avatarService AvatarService
}

func NewHandler(service Service, avatarService AvatarService) *Handler {
	return &Handler{service: service, avatarService: avatarService}
}

func (h *Handler) Me(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	u, err := h.service.GetByID(c.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return response.NotFound(c, "USER_NOT_FOUND", "User not found")
		}
		log.Error("user: Me error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"user": u.ToSafe()}, "OK")
}

func (h *Handler) UpdateMe(c fiber.Ctx) error {
	log := logger.FromCtx(c)
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
		log.Error("user: UpdateMe error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"user": u.ToSafe()}, "Profile updated")
}

func (h *Handler) UpdateAvatar(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}

	fileHeader, err := c.FormFile("avatar")
	if err != nil {
		return response.BadRequest(c, "AVATAR_REQUIRED", "Avatar file is required")
	}
	if fileHeader.Size > avatarMaxUpload {
		return response.BadRequest(c, "AVATAR_TOO_LARGE", "Avatar file must be 5MB or smaller")
	}

	f, err := fileHeader.Open()
	if err != nil {
		log.Error("user: open avatar upload error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	defer f.Close()

	raw := make([]byte, fileHeader.Size)
	if _, err := io.ReadFull(f, raw); err != nil {
		log.Error("user: read avatar upload error", slog.Any("error", err))
		return response.InternalServerError(c)
	}

	u, avatar, err := h.avatarService.Upload(c.Context(), userID, raw)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidImage):
			return response.BadRequest(c, "INVALID_AVATAR_TYPE", "Avatar must be a valid jpg, png, webp, or gif image")
		case errors.Is(err, ErrAvatarLimitReached):
			return response.Conflict(c, "AVATAR_LIMIT_REACHED",
				fmt.Sprintf("You can store up to %d avatars — delete one before uploading another", MaxAvatarsPerUser))
		case errors.Is(err, ErrNotFound):
			return response.NotFound(c, "USER_NOT_FOUND", "User not found")
		}
		log.Error("user: UpdateAvatar error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"user": u.ToSafe(), "avatar": avatar.ToResponse()}, "Avatar updated")
}

// ListAvatars returns all of the user's stored avatars (0 to MaxAvatarsPerUser).
func (h *Handler) ListAvatars(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	avatars, err := h.avatarService.List(c.Context(), userID)
	if err != nil {
		log.Error("user: ListAvatars error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	out := make([]*UserAvatar, len(avatars))
	for i, a := range avatars {
		out[i] = a.ToResponse()
	}
	return response.OK(c, fiber.Map{"avatars": out, "max": MaxAvatarsPerUser}, "OK")
}

// ActivateAvatar switches the user's active avatar to one already stored
// (the "quick-switch" flow — no re-upload, no new slot consumed).
func (h *Handler) ActivateAvatar(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	avatarID := c.Params("avatarId")
	u, err := h.avatarService.Activate(c.Context(), userID, avatarID)
	if err != nil {
		switch {
		case errors.Is(err, ErrAvatarNotFound):
			return response.NotFound(c, "AVATAR_NOT_FOUND", "Avatar not found")
		case errors.Is(err, ErrNotFound):
			return response.NotFound(c, "USER_NOT_FOUND", "User not found")
		}
		log.Error("user: ActivateAvatar error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"user": u.ToSafe()}, "Avatar activated")
}

// DeleteAvatar removes one stored avatar, freeing a slot. If it was the
// active one, the most recently uploaded remaining avatar (if any) takes
// over automatically — see AvatarRepository.Delete.
func (h *Handler) DeleteAvatar(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	avatarID := c.Params("avatarId")
	u, err := h.avatarService.Delete(c.Context(), userID, avatarID)
	if err != nil {
		switch {
		case errors.Is(err, ErrAvatarNotFound):
			return response.NotFound(c, "AVATAR_NOT_FOUND", "Avatar not found")
		case errors.Is(err, ErrNotFound):
			return response.NotFound(c, "USER_NOT_FOUND", "User not found")
		}
		log.Error("user: DeleteAvatar error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"user": u.ToSafe()}, "Avatar deleted")
}
