// backend/internal/auth/handler.go
package auth

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles all auth HTTP endpoints.
type Handler struct {
	service Service
}

// NewHandler creates a new auth Handler.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// Signup handles POST /api/v1/auth/signup
func (h *Handler) Signup(c fiber.Ctx) error {
	var req SignupRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	if err := validateSignupRequest(req); err != nil {
		return response.BadRequest(c, "VALIDATION_ERROR", err.Error())
	}

	u, err := h.service.Signup(c.Context(), req)
	if err != nil {
		if errors.Is(err, ErrEmailAlreadyExists) {
			// Generic message — prevent email enumeration
			return response.BadRequest(c, "SIGNUP_FAILED", "Unable to create account with the provided details")
		}
		slog.Error("auth: signup error", slog.Any("error", err))
		return response.InternalServerError(c)
	}

	return response.Created(c, fiber.Map{"user": u}, "Account created successfully")
}

// Login handles POST /api/v1/auth/login
func (h *Handler) Login(c fiber.Ctx) error {
	var req LoginRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.Unauthorized(c, "INVALID_CREDENTIALS", "Invalid email or password")
	}

	if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.Password) == "" {
		return response.Unauthorized(c, "INVALID_CREDENTIALS", "Invalid email or password")
	}

	ip := c.IP()
	userAgent := string(c.Request().Header.Peek("User-Agent"))

	pair, err := h.service.Login(c.Context(), req, ip, userAgent)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			return response.Unauthorized(c, "INVALID_CREDENTIALS", "Invalid email or password")
		case errors.Is(err, ErrAccountLocked):
			return response.Unauthorized(c, "ACCOUNT_LOCKED", "Account temporarily locked. Please try again later.")
		default:
			slog.Error("auth: login error", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}

	return response.OK(c, pair, "Login successful")
}

// OAuthSync handles POST /api/v1/auth/oauth/sync.
func (h *Handler) OAuthSync(c fiber.Ctx) error {
	var req OAuthSyncRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	result, err := h.service.OAuthSync(c.Context(), req, c.IP(), string(c.Request().Header.Peek("User-Agent")))
	if err != nil {
		switch {
		case errors.Is(err, ErrOAuthProviderRequired):
			return response.BadRequest(c, "PROVIDER_REQUIRED", "OAuth provider is required")
		case errors.Is(err, ErrOAuthAccountIDRequired):
			return response.BadRequest(c, "PROVIDER_ACCOUNT_ID_REQUIRED", "Provider account ID is required")
		case errors.Is(err, ErrOAuthEmailRequired):
			return response.BadRequest(c, "EMAIL_REQUIRED", "Email is required when linking a new OAuth account")
		case errors.Is(err, ErrAccountDisabled):
			return response.Unauthorized(c, "ACCOUNT_DISABLED", "Account is disabled")
		default:
			slog.Error("auth: oauth sync error", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}
	return response.OK(c, result, "OAuth account synced")
}

// Refresh handles POST /api/v1/auth/refresh
func (h *Handler) Refresh(c fiber.Ctx) error {
	var req RefreshRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.Unauthorized(c, "INVALID_TOKEN", "Invalid or expired refresh token")
	}

	if strings.TrimSpace(req.RefreshToken) == "" {
		return response.Unauthorized(c, "MISSING_TOKEN", "Refresh token is required")
	}

	pair, err := h.service.Refresh(c.Context(), req.RefreshToken)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials),
			errors.Is(err, ErrSessionNotFound),
			errors.Is(err, ErrSessionRevoked),
			errors.Is(err, ErrSessionExpired):
			return response.Unauthorized(c, "INVALID_TOKEN", "Invalid or expired refresh token")
		default:
			slog.Error("auth: refresh error", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}

	return response.OK(c, pair, "Token refreshed")
}

// Logout handles POST /api/v1/auth/logout
func (h *Handler) Logout(c fiber.Ctx) error {
	var req RefreshRequest
	if err := c.Bind().JSON(&req); err == nil && req.RefreshToken != "" {
		if err := h.service.Logout(c.Context(), req.RefreshToken); err != nil {
			slog.Error("auth: logout error", slog.Any("error", err))
		}
	}
	// Always return 204 — logout should always appear to succeed
	return response.NoContent(c)
}

// LogoutAll handles POST /api/v1/auth/logout-all
func (h *Handler) LogoutAll(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}

	if err := h.service.LogoutAll(c.Context(), userID); err != nil {
		slog.Error("auth: logout-all error", slog.Any("error", err))
		return response.InternalServerError(c)
	}

	return response.NoContent(c)
}

// Me handles GET /api/v1/auth/me
func (h *Handler) Me(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}

	u, err := h.service.Me(c.Context(), userID)
	if err != nil {
		slog.Error("auth: me error", slog.Any("error", err))
		return response.InternalServerError(c)
	}

	return response.OK(c, fiber.Map{"user": u}, "OK")
}

// PasswordResetRequest handles POST /api/v1/auth/password-reset/request
func (h *Handler) PasswordResetRequest(c fiber.Ctx) error {
	var req PasswordResetRequestBody
	// Parse but always return same response — never reveal whether email exists
	if err := c.Bind().JSON(&req); err == nil && req.Email != "" {
		_ = h.service.RequestPasswordReset(c.Context(), req.Email)
	}
	return response.OK(c, nil, "If that email is registered, a reset link has been sent")
}

// PasswordResetConfirm handles POST /api/v1/auth/password-reset/confirm
func (h *Handler) PasswordResetConfirm(c fiber.Ctx) error {
	var req PasswordResetConfirmBody
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	if req.Token == "" || req.NewPassword == "" {
		return response.BadRequest(c, "MISSING_FIELDS", "Token and new password are required")
	}
	if err := h.service.ConfirmPasswordReset(c.Context(), req.Token, req.NewPassword); err != nil {
		if errors.Is(err, ErrNotImplemented) {
			return response.NotImplemented(c)
		}
		return response.BadRequest(c, "RESET_FAILED", "Invalid or expired reset token")
	}
	return response.OK(c, nil, "Password reset successful")
}

// ----------------------------------------------------------
// Validation
// ----------------------------------------------------------

func validateSignupRequest(req SignupRequest) error {
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" {
		return errors.New("email is required")
	}
	if len(req.Email) > 255 {
		return errors.New("email must not exceed 255 characters")
	}
	if req.Password == "" {
		return errors.New("password is required")
	}
	if len(req.Password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	if len(req.Password) > 72 {
		return errors.New("password must not exceed 72 characters")
	}
	// First/last name are optional because OAuth and SaaS onboarding flows
	// may initially provide only email + displayName.
	return nil
}
