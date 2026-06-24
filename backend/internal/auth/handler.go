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
func NewHandler(service Service, cookieCfg config.CookieConfig) *Handler {
	return &Handler{service: service, cookieCfg: cookieCfg}
}

// Signup godoc
//
//	@Summary		Sign up
//	@Description	Creates a new user account. Returns the safe user profile on success.
//	@Description
//	@Description	**Error codes:**
//	@Description	- `VALIDATION_ERROR` — missing or invalid fields
//	@Description	- `EMAIL_ALREADY_EXISTS` — account with this email already exists
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		SignupRequest	true	"Registration details"
//	@Success		201		{object}	response.Created{data=SignupResponseData}
//	@Failure		400		{object}	response.Error	"VALIDATION_ERROR"
//	@Failure		409		{object}	response.Error	"EMAIL_ALREADY_EXISTS"
//	@Failure		429		{object}	response.Error	"RATE_LIMITED"
//	@Failure		500		{object}	response.Error
//	@Router			/auth/signup [post]
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
			return response.Conflict(c, "EMAIL_ALREADY_EXISTS", "An account with this email already exists")
		default:
			log.Error("auth: signup failed",
				slog.String("layer", "handler"),
				slog.Any("error", err),
			)
			return response.InternalServerError(c)
		}
	}

	return response.Created(c, fiber.Map{"user": u}, "Account created successfully")
}

// Login godoc
//
//	@Summary		Log in
//	@Description	Authenticates with email + password.
//	@Description
//	@Description	**On success:**
//	@Description	- Returns `access_token` + `expires_in` in the response body
//	@Description	- Sets an httpOnly refresh cookie (`bsaas_refresh`, path `/api/v1/auth`)
//	@Description	- The refresh token is NEVER in the response body
//	@Description
//	@Description	**Error codes:**
//	@Description	- `INVALID_CREDENTIALS` — wrong email or password (deliberately generic to prevent enumeration)
//	@Description	- `ACCOUNT_LOCKED` — too many failed attempts; try again later
//	@Description	- `ACCOUNT_DISABLED` — account suspended by an admin
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		LoginRequest					true	"Login credentials"
//	@Success		200		{object}	response.OK{data=LoginResponseData}	"Access token issued"
//	@Failure		400		{object}	response.Error					"INVALID_BODY"
//	@Failure		401		{object}	response.Error					"INVALID_CREDENTIALS / ACCOUNT_LOCKED / ACCOUNT_DISABLED"
//	@Failure		423		{object}	response.Error					"ACCOUNT_LOCKED"
//	@Failure		429		{object}	response.Error					"RATE_LIMITED"
//	@Failure		500		{object}	response.Error
//	@Router			/auth/login [post]
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

	h.setRefreshCookie(c, pair.RefreshToken, pair.ExpiresIn)
	return response.OK(c, pair.ToClient(), "Login successful")
}

// OAuthSync godoc
//
//	@Summary		OAuth sync
//	@Description	Called by the Next.js auth adapter after a successful OAuth sign-in (Google, GitHub, etc.).
//	@Description
//	@Description	If `issueTokens=true` the backend issues a full token pair:
//	@Description	- Access token in the response body
//	@Description	- Refresh token set as an httpOnly cookie (same contract as Login)
//	@Description
//	@Description	**Error codes:**
//	@Description	- `PROVIDER_REQUIRED`, `PROVIDER_ACCOUNT_ID_REQUIRED`, `EMAIL_REQUIRED`, `ACCOUNT_DISABLED`
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		OAuthSyncRequest					true	"OAuth provider data"
//	@Success		200		{object}	response.OK{data=OAuthSyncClientResponse}	"Account synced"
//	@Failure		400		{object}	response.Error
//	@Failure		401		{object}	response.Error	"ACCOUNT_DISABLED"
//	@Failure		500		{object}	response.Error
//	@Router			/auth/oauth/sync [post]
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

// Refresh godoc
//
//	@Summary		Refresh access token
//	@Description	Issues a new access token using the httpOnly refresh cookie (`bsaas_refresh`).
//	@Description
//	@Description	**The refresh token is read from the cookie — NOT the request body.**
//	@Description	The cookie is rotated on every successful refresh (old token revoked, new one set).
//	@Description
//	@Description	**Why cookie-only?**
//	@Description	If the token were accepted from the body, an XSS script that captured it from
//	@Description	localStorage could silently refresh it. The httpOnly cookie is inaccessible to JS.
//	@Description
//	@Description	**Error codes:**
//	@Description	- `MISSING_TOKEN` — no refresh cookie present
//	@Description	- `INVALID_TOKEN` — token expired, revoked, or not found
//	@Tags			Auth
//	@Produce		json
//	@Success		200		{object}	response.OK{data=ClientTokenPair}	"New access token"
//	@Failure		401		{object}	response.Error	"MISSING_TOKEN / INVALID_TOKEN"
//	@Failure		500		{object}	response.Error
//	@Router			/auth/refresh [post]
func (h *Handler) Refresh(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	rawToken := c.Cookies(h.cookieCfg.Name)
	if strings.TrimSpace(rawToken) == "" {
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
			h.clearRefreshCookie(c)
			return response.Unauthorized(c, "INVALID_TOKEN", "Invalid or expired refresh token")
		default:
			log.Error("auth: refresh error", slog.Any("error", err))
			return response.InternalServerError(c)
		}
	}

	h.setRefreshCookie(c, pair.RefreshToken, pair.ExpiresIn)
	return response.OK(c, pair.ToClient(), "Token refreshed")
}

// Logout godoc
//
//	@Summary		Log out
//	@Description	Revokes the current refresh session and clears the httpOnly cookie.
//	@Description
//	@Description	**Always returns 204** — even if the token was already invalid or missing.
//	@Description	Logout must never fail visibly to the user.
//	@Description
//	@Description	Does NOT require a valid access token — an expired-token user should still be
//	@Description	able to revoke their refresh session.
//	@Tags			Auth
//	@Produce		json
//	@Success		204	"Session revoked and cookie cleared"
//	@Failure		500	{object}	response.Error
//	@Router			/auth/logout [post]
func (h *Handler) Logout(c fiber.Ctx) error {
	log := logger.FromCtx(c)
	rawToken := c.Cookies(h.cookieCfg.Name)
	if strings.TrimSpace(rawToken) != "" {
		if err := h.service.Logout(c.Context(), rawToken); err != nil {
			log.Error("auth: logout error", slog.Any("error", err))
		}
	}

	h.clearRefreshCookie(c)
	return response.NoContent(c)
}

// LogoutAll godoc
//
//	@Summary		Log out all sessions
//	@Description	Revokes ALL active refresh sessions for the authenticated user across all devices.
//	@Description	Also clears the current browser's refresh cookie.
//	@Description
//	@Description	Requires a valid access token (`Authorization: Bearer <token>`).
//	@Tags			Auth
//	@Produce		json
//	@Security		BearerAuth
//	@Success		204	"All sessions revoked"
//	@Failure		401	{object}	response.Error	"UNAUTHORIZED"
//	@Failure		500	{object}	response.Error
//	@Router			/auth/logout-all [post]
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

	h.clearRefreshCookie(c)
	return response.NoContent(c)
}

// Me godoc
//
//	@Summary		Get authenticated user
//	@Description	Returns the full safe user profile for the currently authenticated user.
//	@Description	Safe = `password_hash` is never included.
//	@Tags			Auth
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	response.OK{data=MeResponseData}	"User profile"
//	@Failure		401	{object}	response.Error	"UNAUTHORIZED"
//	@Failure		500	{object}	response.Error
//	@Router			/auth/me [get]
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

// PasswordResetRequest godoc
//
//	@Summary		Request password reset
//	@Description	Initiates a password reset flow for the given email address.
//	@Description
//	@Description	**Always returns 200** — even if the email is not registered.
//	@Description	This prevents email enumeration: an attacker cannot distinguish registered
//	@Description	from unregistered emails by the response.
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		PasswordResetRequestBody	true	"Email address"
//	@Success		200		{object}	response.OK					"Reset request processed"
//	@Failure		500		{object}	response.Error
//	@Router			/auth/password-reset/request [post]
func (h *Handler) PasswordResetRequest(c fiber.Ctx) error {
	var req PasswordResetRequestBody
	if err := c.Bind().JSON(&req); err == nil && req.Email != "" {
		_ = h.service.RequestPasswordReset(c.Context(), req.Email)
	}
	return response.OK(c, nil, "If that email is registered, a reset link has been sent")
}

// PasswordResetConfirm godoc
//
//	@Summary		Confirm password reset
//	@Description	Sets a new password using the reset token from the email link.
//	@Description
//	@Description	**Error codes:**
//	@Description	- `MISSING_FIELDS` — token or new_password is empty
//	@Description	- `RESET_FAILED` — invalid or expired reset token
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		PasswordResetConfirmBody	true	"Reset token and new password"
//	@Success		200		{object}	response.OK					"Password reset successful"
//	@Failure		400		{object}	response.Error				"MISSING_FIELDS / RESET_FAILED"
//	@Failure		500		{object}	response.Error
//	@Router			/auth/password-reset/confirm [post]
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

func (h *Handler) clearRefreshCookie(c fiber.Ctx) {
	cookie := &fiber.Cookie{
		Name:     h.cookieCfg.Name,
		Value:    "",
		Domain:   h.cookieCfg.Domain,
		Path:     h.cookieCfg.Path,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
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
	return nil
}

// -------------------------------------------------------------------
// Swagger response wrapper types
// These types only exist for documentation generation — not used at runtime.
// They describe the shape of fiber.Map{} responses.
// -------------------------------------------------------------------

// SignupResponseData is the data field of the signup 201 response.
type SignupResponseData struct {
	User any `json:"user" swaggertype:"object"` // SafeUser
}

// LoginResponseData is the data field of the login 200 response.
type LoginResponseData = ClientTokenPair

// MeResponseData is the data field of the /me 200 response.
type MeResponseData struct {
	User any `json:"user" swaggertype:"object"` // SafeUser
}
