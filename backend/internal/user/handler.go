// backend/internal/user/handler.go
package user

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

type Handler struct{ service Service }

func NewHandler(service Service) *Handler { return &Handler{service: service} }

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
	photoURL := ""
	contentType := c.Get("Content-Type")
	if strings.Contains(contentType, "multipart/form-data") {
		file, err := c.FormFile("avatar")
		if err != nil {
			return response.BadRequest(c, "AVATAR_REQUIRED", "Avatar file is required")
		}
		if file.Size > 5*1024*1024 {
			return response.BadRequest(c, "AVATAR_TOO_LARGE", "Avatar file must be 5MB or smaller")
		}
		ext := strings.ToLower(filepath.Ext(file.Filename))
		switch ext {
		case ".jpg", ".jpeg", ".png", ".webp", ".gif":
		default:
			return response.BadRequest(c, "INVALID_AVATAR_TYPE", "Avatar must be jpg, png, webp, or gif")
		}
		dir := filepath.Join("uploads", "avatars")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Error("user: create avatar dir error", slog.Any("error", err))
			return response.InternalServerError(c)
		}
		filename := fmt.Sprintf("%s%s", uuid.NewString(), ext)
		path := filepath.Join(dir, filename)
		if err := c.SaveFile(file, path); err != nil {
			log.Error("user: save avatar error", slog.Any("error", err))
			return response.InternalServerError(c)
		}
		photoURL = "/" + filepath.ToSlash(path)
	} else {
		var req AvatarRequest
		if err := c.Bind().JSON(&req); err != nil {
			return response.BadRequest(c, "INVALID_BODY", "Use multipart avatar file or JSON photoURL")
		}
		photoURL = strings.TrimSpace(req.PhotoURL)
	}
	if photoURL == "" {
		return response.BadRequest(c, "AVATAR_REQUIRED", "Avatar URL or file is required")
	}
	u, err := h.service.UpdateAvatar(c.Context(), userID, photoURL)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return response.NotFound(c, "USER_NOT_FOUND", "User not found")
		}
		log.Error("user: UpdateAvatar error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"user": u.ToSafe(), "photoURL": photoURL}, "Avatar updated")
}
