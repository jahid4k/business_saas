package auth

import (
	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles all auth HTTP endpoints.
// It depends only on the Service interface — no direct DB or Redis access.
type Handler struct {
	service Service
}

// NewHandler creates a new auth Handler.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// Signup handles POST /api/v1/auth/signup
// Creates a new user account.
// STATUS: Phase 1-B stub.
func (h *Handler) Signup(c fiber.Ctx) error {
	// TODO (Phase 1-B):
	// 1. Parse and validate SignupRequest body
	// 2. Call h.service.Signup(ctx, req)
	// 3. Return 201 Created with sanitised user data
	return response.NotImplemented(c)
}

// Login handles POST /api/v1/auth/login
// Authenticates user and returns JWT access + opaque refresh token.
// STATUS: Phase 1-B stub.
func (h *Handler) Login(c fiber.Ctx) error {
	// TODO (Phase 1-B):
	// 1. Parse and validate LoginRequest body
	// 2. Call h.service.Login(ctx, req, ip, userAgent)
	// 3. Return generic error for any auth failure (never reveal which field failed)
	// 4. Return 200 OK with TokenPair on success
	return response.NotImplemented(c)
}

// Refresh handles POST /api/v1/auth/refresh
// Exchanges a valid opaque refresh token for a new token pair.
// STATUS: Phase 1-B stub.
func (h *Handler) Refresh(c fiber.Ctx) error {
	// TODO (Phase 1-B):
	// 1. Parse RefreshRequest body
	// 2. Call h.service.Refresh(ctx, refreshToken)
	// 3. Rotate the refresh token (revoke old, issue new)
	// 4. Return new TokenPair
	return response.NotImplemented(c)
}

// Logout handles POST /api/v1/auth/logout
// Revokes the current session.
// STATUS: Phase 1-B stub.
func (h *Handler) Logout(c fiber.Ctx) error {
	// TODO (Phase 1-B):
	// 1. Extract refresh token from request body
	// 2. Call h.service.Logout(ctx, refreshToken)
	// 3. Return 204 No Content
	return response.NotImplemented(c)
}

// LogoutAll handles POST /api/v1/auth/logout-all
// Revokes all sessions for the authenticated user.
// STATUS: Phase 1-B stub.
func (h *Handler) LogoutAll(c fiber.Ctx) error {
	// TODO (Phase 1-B):
	// 1. Extract user_id from c.Locals (set by RequireAuth middleware)
	// 2. Call h.service.LogoutAll(ctx, userID)
	// 3. Return 204 No Content
	return response.NotImplemented(c)
}

// PasswordResetRequest handles POST /api/v1/auth/password-reset/request
// Sends a password reset email with a single-use time-limited token.
// STATUS: Phase 1-B stub.
func (h *Handler) PasswordResetRequest(c fiber.Ctx) error {
	// TODO (Phase 1-B):
	// 1. Parse email from body
	// 2. Call h.service.RequestPasswordReset(ctx, email)
	// 3. Always return 200 OK regardless of whether email exists (prevent enumeration)
	return response.NotImplemented(c)
}

// PasswordResetConfirm handles POST /api/v1/auth/password-reset/confirm
// Validates the reset token and sets a new password.
// STATUS: Phase 1-B stub.
func (h *Handler) PasswordResetConfirm(c fiber.Ctx) error {
	// TODO (Phase 1-B):
	// 1. Parse token + new_password
	// 2. Call h.service.ConfirmPasswordReset(ctx, token, newPassword)
	// 3. Invalidate all existing sessions after password change
	// 4. Return 200 OK
	return response.NotImplemented(c)
}
