// backend/internal/auth/handler.go
package auth

import (
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/mridha/businesssaas/internal/config"
	"github.com/mridha/businesssaas/pkg/logger"
	"github.com/mridha/businesssaas/pkg/response"
)

// Handler handles all auth HTTP endpoints.
type Handler struct {
	service   Service
	cookieCfg config.CookieConfig
}

// NewHandler creates a new auth Handler.
// cookieCfg is injected so the handler knows the cookie name, path,
// domain, Secure flag, and SameSite policy without importing config directly.
func NewHandler(service Service, cookieCfg config.CookieConfig) *Handler {
	return &Handler{service: service, cookieCfg: cookieCfg}
}

// -------------------------------------------------------------------
// Signup — POST /api/v1/auth/signup
// -------------------------------------------------------------------

func (h *Handler) Signup(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	var req SignupRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	if err := validateSignupRequest(req); err != nil {
		return response.BadRequest(c, "VALIDATION_ERROR", err.Error())
	}

	u, err := h.service.Signup(c.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrEmailAlreadyExists):
			// এটা expected — log করার দরকার নেই, 409 দাও
			return response.Conflict(c, "EMAIL_ALREADY_EXISTS", "An account with this email already exists")
		default:
			// এটাই real unexpected error — log করো
			log.Error("auth: signup failed",
				slog.String("layer", "handler"),
				slog.Any("error", err),
			)
			return response.InternalServerError(c)
		}
	}

	return response.Created(c, fiber.Map{"user": u}, "Account created successfully")
}

// -------------------------------------------------------------------
// Login — POST /api/v1/auth/login
//
// On success:
//   - Sets the httpOnly refresh token cookie (bsaas_refresh).
//   - Returns only the access token in the JSON body.
//     The refresh token is NEVER written to the response body.
// -------------------------------------------------------------------

func (h *Handler) Login(c fiber.Ctx) error {
	log := logger.FromCtx(c)
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
		case errors.Is(err, ErrAccountDisabled):
			return response.Unauthorized(c, "ACCOUNT_DISABLED", "Your account has been disabled.")
		default:
			log.Error("auth: login error", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}

	// Write the refresh token into the httpOnly cookie.
	// The browser will automatically include this cookie on all future
	// requests to /api/v1/auth/* — JavaScript cannot read it.
	h.setRefreshCookie(c, pair.RefreshToken, pair.ExpiresIn)

	// Return only the access token in the body.
	return response.OK(c, pair.ToClient(), "Login successful")
}

// -------------------------------------------------------------------
// OAuthSync — POST /api/v1/auth/oauth/sync
//
// Called by next-auth after a successful Google / Facebook OAuth sign-in.
// If IssueTokens=true, the handler issues a token pair and sets the
// refresh token cookie exactly like Login.
// -------------------------------------------------------------------

// OAuthSync handles POST /api/v1/auth/oauth/sync.
//
// Called by next-auth after a successful Google / Facebook OAuth sign-in.
// If IssueTokens=true the service issues a full TokenPair. The handler:
//  1. Reads the raw refresh token from result.Tokens (never serialised directly).
//  2. Sets the httpOnly refresh cookie — same as Login.
//  3. Sends OAuthSyncClientResponse to the frontend (access_token only, no raw refresh).
func (h *Handler) OAuthSync(c fiber.Ctx) error {
	log := logger.FromCtx(c)
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
			log.Error("auth: oauth sync error", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}

	// Build the client-safe response. If tokens were issued:
	//   - set the httpOnly cookie with the raw refresh token
	//   - send only the access token in the body (ToClient strips RefreshToken)
	clientResp := &OAuthSyncClientResponse{
		User:    result.User,
		Account: result.Account,
	}
	if result.Tokens != nil {
		h.setRefreshCookie(c, result.Tokens.RefreshToken, result.Tokens.ExpiresIn)
		clientResp.Tokens = result.Tokens.ToClient()
	}

	return response.OK(c, clientResp, "OAuth account synced")
}

// -------------------------------------------------------------------
// Refresh — POST /api/v1/auth/refresh
//
// Reads the refresh token from the httpOnly cookie (not from the body).
// On success:
//   - Rotates the refresh token (old cookie → new cookie).
//   - Returns a new access token in the response body.
//
// Why cookie-only and not body?
//   If we accepted the token from the body, any XSS script that captured
//   the token (e.g. from localStorage) could silently refresh it. By
//   reading only from the httpOnly cookie, the refresh path is only
//   accessible to the browser itself, not to injected scripts.
// -------------------------------------------------------------------

func (h *Handler) Refresh(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	// Read refresh token exclusively from the httpOnly cookie.
	rawToken := c.Cookies(h.cookieCfg.Name)
	if strings.TrimSpace(rawToken) == "" {
		// Clear any stale / malformed cookie to avoid confusion.
		h.clearRefreshCookie(c)
		return response.Unauthorized(c, "MISSING_TOKEN", "Refresh token is required")
	}

	pair, err := h.service.Refresh(c.Context(), rawToken)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials),
			errors.Is(err, ErrSessionNotFound),
			errors.Is(err, ErrSessionRevoked),
			errors.Is(err, ErrSessionExpired):
			// Token is invalid or expired — clear the cookie so the
			// browser does not keep sending a dead token.
			h.clearRefreshCookie(c)
			return response.Unauthorized(c, "INVALID_TOKEN", "Invalid or expired refresh token")
		default:
			log.Error("auth: refresh error", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}

	// Rotate: replace the old cookie with the new refresh token.
	h.setRefreshCookie(c, pair.RefreshToken, pair.ExpiresIn)

	return response.OK(c, pair.ToClient(), "Token refreshed")
}

// -------------------------------------------------------------------
// Logout — POST /api/v1/auth/logout
//
// Revokes the refresh session in the database and clears the cookie.
// Does NOT require a valid access token — an expired access token user
// should still be able to revoke their refresh token.
// Always returns 204 regardless of outcome (logout must never fail visibly).
// -------------------------------------------------------------------

func (h *Handler) Logout(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	rawToken := c.Cookies(h.cookieCfg.Name)
	if strings.TrimSpace(rawToken) != "" {
		if err := h.service.Logout(c.Context(), rawToken); err != nil {
			// Log but do not surface — logout must always appear to succeed.
			log.Error("auth: logout error", slog.Any("error", err))
		}
	}

	// Clear the cookie unconditionally — even if the token was already
	// invalid or missing, we want to ensure the browser removes it.
	h.clearRefreshCookie(c)

	return response.NoContent(c)
}

// -------------------------------------------------------------------
// LogoutAll — POST /api/v1/auth/logout-all
//
// Requires a valid access token (requireAuth middleware).
// Revokes ALL refresh sessions for the authenticated user and
// clears the current browser's cookie.
// -------------------------------------------------------------------

func (h *Handler) LogoutAll(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}

	if err := h.service.LogoutAll(c.Context(), userID); err != nil {
		log.Error("auth: logout-all error", slog.Any("error", err))
		return response.InternalServerError(c)
	}

	// Clear the cookie on this browser too.
	h.clearRefreshCookie(c)

	return response.NoContent(c)
}

// -------------------------------------------------------------------
// Me — GET /api/v1/auth/me
// -------------------------------------------------------------------

func (h *Handler) Me(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Unauthorized(c, "UNAUTHORIZED", "Authentication required")
	}

	u, err := h.service.Me(c.Context(), userID)
	if err != nil {
		log.Error("auth: me error", slog.Any("error", err))
		return response.InternalServerError(c)
	}

	return response.OK(c, fiber.Map{"user": u}, "OK")
}

// -------------------------------------------------------------------
// PasswordResetRequest — POST /api/v1/auth/password-reset/request
// -------------------------------------------------------------------

func (h *Handler) PasswordResetRequest(c fiber.Ctx) error {
	var req PasswordResetRequestBody
	// Parse but always return the same response — never reveal whether email exists.
	if err := c.Bind().JSON(&req); err == nil && req.Email != "" {
		_ = h.service.RequestPasswordReset(c.Context(), req.Email)
	}
	return response.OK(c, nil, "If that email is registered, a reset link has been sent")
}

// -------------------------------------------------------------------
// PasswordResetConfirm — POST /api/v1/auth/password-reset/confirm
// -------------------------------------------------------------------

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

// -------------------------------------------------------------------
// Cookie helpers
// -------------------------------------------------------------------

// setRefreshCookie writes the refresh token into the httpOnly cookie.
// expiresInSeconds is the access token TTL (used to derive the Max-Age).
// The cookie Max-Age is derived from the config's RefreshTokenTTL, not
// from expiresInSeconds — the access token TTL is much shorter and should
// not limit the cookie lifetime.
func (h *Handler) setRefreshCookie(c fiber.Ctx, rawToken string, _ int64) {
	maxAgeSecs := int(h.cookieCfg.MaxAge.Seconds())

	cookie := &fiber.Cookie{
		Name:     h.cookieCfg.Name,
		Value:    rawToken,
		Domain:   h.cookieCfg.Domain,
		Path:     h.cookieCfg.Path,
		MaxAge:   maxAgeSecs,
		Secure:   h.cookieCfg.Secure,
		HTTPOnly: h.cookieCfg.HTTPOnly, // always true
		SameSite: h.cookieCfg.SameSite,
	}

	c.Cookie(cookie)
}

// clearRefreshCookie immediately expires the refresh cookie.
// The browser removes an expired cookie automatically.
func (h *Handler) clearRefreshCookie(c fiber.Ctx) {
	cookie := &fiber.Cookie{
		Name:     h.cookieCfg.Name,
		Value:    "",
		Domain:   h.cookieCfg.Domain,
		Path:     h.cookieCfg.Path,
		MaxAge:   -1,              // negative MaxAge = delete immediately
		Expires:  time.Unix(0, 0), // belt-and-suspenders: also set past expiry date
		Secure:   h.cookieCfg.Secure,
		HTTPOnly: h.cookieCfg.HTTPOnly,
		SameSite: h.cookieCfg.SameSite,
	}

	c.Cookie(cookie)
}

// -------------------------------------------------------------------
// Validation
// -------------------------------------------------------------------

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
	// First/last name are optional — OAuth and SaaS onboarding flows
	// may initially provide only email + displayName.
	return nil
}
