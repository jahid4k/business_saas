// backend/internal/business/handler.go
package business

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles business/workspace HTTP endpoints.
type Handler struct {
	service Service
}

// NewHandler creates a new business Handler.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// Create handles POST /api/v1/businesses
//
// Creates a new workspace. The authenticated user automatically becomes the Owner.
// The business + owner membership are created in a single transaction.
func (h *Handler) Create(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}

	var req CreateBusinessRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	b, err := h.service.Create(c.Context(), userID, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrSlugTaken):
			return response.Conflict(c, "SLUG_TAKEN", "That slug is already in use. Please choose another.")
		case errors.Is(err, ErrInvalidSlug):
			return response.BadRequest(c, "INVALID_SLUG", "Slug must be lowercase letters, digits, and hyphens only (e.g. my-company)")
		case errors.Is(err, ErrInvalidName):
			return response.BadRequest(c, "INVALID_NAME", "Business name must be between 2 and 100 characters")
		default:
			slog.Error("business: Create error", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}

	return response.Created(c, fiber.Map{"business": b}, "Workspace created successfully")
}

// List handles GET /api/v1/businesses
//
// Returns all workspaces the authenticated user belongs to,
// including the user's role in each workspace.
func (h *Handler) List(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}

	businesses, err := h.service.ListForUser(c.Context(), userID)
	if err != nil {
		slog.Error("business: List error", slog.Any("error", err))
		return response.InternalServerError(c)
	}

	return response.OK(c, fiber.Map{"businesses": businesses}, "OK")
}

// Get handles GET /api/v1/businesses/:id
//
// Returns a single workspace. The user must be a member of the workspace.
func (h *Handler) Get(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}

	businessID := c.Params("id")
	if businessID == "" {
		return response.BadRequest(c, "MISSING_ID", "Business ID is required")
	}

	b, err := h.service.GetByID(c.Context(), businessID, userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			return response.NotFound(c, "BUSINESS_NOT_FOUND", "Workspace not found")
		case errors.Is(err, ErrNotMember):
			// Return 404 to prevent business ID enumeration
			return response.NotFound(c, "BUSINESS_NOT_FOUND", "Workspace not found")
		default:
			slog.Error("business: Get error", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}

	return response.OK(c, fiber.Map{"business": b}, "OK")
}

// Switch handles POST /api/v1/businesses/:id/switch
//
// Verifies the user is a member of the business and returns a new JWT
// access token with business_id and role embedded.
//
// The client must replace their current access token with the returned one.
// All subsequent business-scoped requests must use this new token.
func (h *Handler) Switch(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}

	businessID := c.Params("id")
	if businessID == "" {
		return response.BadRequest(c, "MISSING_ID", "Business ID is required")
	}

	accessToken, role, err := h.service.Switch(c.Context(), businessID, userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			return response.NotFound(c, "BUSINESS_NOT_FOUND", "Workspace not found")
		case errors.Is(err, ErrNotMember):
			return response.Forbidden(c, "NOT_A_MEMBER", "You are not a member of this workspace")
		default:
			slog.Error("business: Switch error", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}

	return response.OK(c, fiber.Map{
		"access_token": accessToken,
		"role":         role,
		"business_id":  businessID,
	}, "Business context switched")
}
