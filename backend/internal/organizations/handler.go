// backend/internal/business/handler.go
// Package name is kept as business for backward compatibility. Handler now exposes organization semantics.
package organizations

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/response"
)

type Handler struct{ service Service }

func NewHandler(service Service) *Handler { return &Handler{service: service} }

func (h *Handler) Create(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	var req CreateBusinessRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	org, err := h.service.Create(c.Context(), userID, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrSlugTaken):
			return response.Conflict(c, "SLUG_TAKEN", "That slug is already in use. Please choose another.")
		case errors.Is(err, ErrInvalidSlug):
			return response.BadRequest(c, "INVALID_SLUG", "Slug must be lowercase letters, digits, and hyphens only (e.g. my-company)")
		case errors.Is(err, ErrInvalidName):
			return response.BadRequest(c, "INVALID_NAME", "Organization name must be between 2 and 100 characters")
		default:
			slog.Error("organization: Create error", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}
	return response.Created(c, fiber.Map{"organization": org}, "Organization created successfully")
}

func (h *Handler) List(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	orgs, err := h.service.ListForUser(c.Context(), userID)
	if err != nil {
		slog.Error("organization: List error", slog.Any("error", err))
		return response.InternalServerError(c)
	}
	return response.OK(c, fiber.Map{"organizations": orgs}, "OK")
}

func (h *Handler) Get(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	organizationID := c.Params("id")
	if organizationID == "" {
		return response.BadRequest(c, "MISSING_ID", "Organization ID is required")
	}
	org, err := h.service.GetByID(c.Context(), organizationID, userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound), errors.Is(err, ErrNotMember):
			return response.NotFound(c, "ORGANIZATION_NOT_FOUND", "Organization not found")
		default:
			slog.Error("organization: Get error", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}
	return response.OK(c, fiber.Map{"organization": org}, "OK")
}

func (h *Handler) Switch(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}
	organizationID := c.Params("id")
	if organizationID == "" {
		return response.BadRequest(c, "MISSING_ID", "Organization ID is required")
	}
	accessToken, role, err := h.service.Switch(c.Context(), organizationID, userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			return response.NotFound(c, "ORGANIZATION_NOT_FOUND", "Organization not found")
		case errors.Is(err, ErrNotMember):
			return response.Forbidden(c, "NOT_A_MEMBER", "You are not a member of this organization")
		default:
			slog.Error("organization: Switch error", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}
	return response.OK(c, fiber.Map{"access_token": accessToken, "role": role, "organization_id": organizationID}, "Organization context switched")
}
