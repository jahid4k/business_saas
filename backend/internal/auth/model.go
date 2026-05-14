// Package auth handles all authentication logic for BusinessSAAS:
// signup, login, token refresh, logout, and password reset.
package auth

import "time"

// Session represents a persisted refresh token session in PostgreSQL.
// The raw token is NEVER stored — only its bcrypt hash.
type Session struct {
	ID        string     `db:"id"`
	UserID    string     `db:"user_id"`
	TokenHash string     `db:"token_hash"` // bcrypt hash of the opaque refresh token
	UserAgent string     `db:"user_agent"`
	IPAddress string     `db:"ip_address"`
	ExpiresAt time.Time  `db:"expires_at"`
	RevokedAt *time.Time `db:"revoked_at"` // nil = active session
	CreatedAt time.Time  `db:"created_at"`
}

// IsRevoked returns true if the session has been explicitly revoked.
func (s *Session) IsRevoked() bool {
	return s.RevokedAt != nil
}

// IsExpired returns true if the session has passed its expiry time.
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// IsValid returns true if the session is neither revoked nor expired.
func (s *Session) IsValid() bool {
	return !s.IsRevoked() && !s.IsExpired()
}

// SignupRequest is the request body for POST /api/v1/auth/signup.
type SignupRequest struct {
	Email     string `json:"email"     validate:"required,email,max=255"`
	Password  string `json:"password"  validate:"required,min=8,max=72"`
	FirstName string `json:"first_name" validate:"required,min=1,max=100"`
	LastName  string `json:"last_name"  validate:"required,min=1,max=100"`
}

// LoginRequest is the request body for POST /api/v1/auth/login.
type LoginRequest struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// RefreshRequest is the request body for POST /api/v1/auth/refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// TokenPair is returned on successful login or token refresh.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"` // access token TTL in seconds
}

// PasswordResetRequestBody is the request body for POST /api/v1/auth/password-reset/request.
type PasswordResetRequestBody struct {
	Email string `json:"email" validate:"required,email"`
}

// PasswordResetConfirmBody is the request body for POST /api/v1/auth/password-reset/confirm.
type PasswordResetConfirmBody struct {
	Token       string `json:"token"        validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8,max=72"`
}
