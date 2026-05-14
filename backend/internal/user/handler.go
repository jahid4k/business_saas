package user

import (
	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles user profile endpoints.
type Handler struct {
	service Service
}

// NewHandler creates a new user Handler.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// Me handles GET /api/v1/users/me
// Returns the authenticated user's profile.
// STATUS: Phase 1-B stub.
func (h *Handler) Me(c fiber.Ctx) error {
	// TODO (Phase 1-B):
	// 1. Extract user_id from c.Locals("user_id")
	// 2. Call h.service.GetByID(ctx, userID)
	// 3. Return user.ToSafe()
	return response.NotImplemented(c)
}

// UpdateMe handles PATCH /api/v1/users/me
// Updates the authenticated user's profile.
// STATUS: Phase 1-B stub.
func (h *Handler) UpdateMe(c fiber.Ctx) error {
	// TODO (Phase 1-B):
	// 1. Extract user_id from c.Locals("user_id")
	// 2. Parse and validate UpdateProfileRequest
	// 3. Call h.service.UpdateProfile(ctx, userID, req)
	// 4. Return updated SafeUser
	return response.NotImplemented(c)
}
