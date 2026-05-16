// backend/internal/auth/model.go
package auth

import "time"

// Session represents a persisted refresh token session in PostgreSQL.
// The raw token is NEVER stored — only its SHA-256 hash.
type Session struct {
	ID        string     `db:"id"`
	UserID    string     `db:"user_id"`
	TokenHash string     `db:"token_hash"`
	UserAgent string     `db:"user_agent"`
	IPAddress string     `db:"ip_address"`
	ExpiresAt time.Time  `db:"expires_at"`
	RevokedAt *time.Time `db:"revoked_at"`
	CreatedAt time.Time  `db:"created_at"`
}

func (s *Session) IsRevoked() bool { return s.RevokedAt != nil }
func (s *Session) IsExpired() bool { return time.Now().After(s.ExpiresAt) }
func (s *Session) IsValid() bool   { return !s.IsRevoked() && !s.IsExpired() }

// SignupRequest is the body for POST /api/v1/auth/signup.
type SignupRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// LoginRequest is the body for POST /api/v1/auth/login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RefreshRequest is the body for POST /api/v1/auth/refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// TokenPair is returned on login or token refresh.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"` // seconds
}

// PasswordResetRequestBody is the body for POST /api/v1/auth/password-reset/request.
type PasswordResetRequestBody struct {
	Email string `json:"email"`
}

// PasswordResetConfirmBody is the body for POST /api/v1/auth/password-reset/confirm.
type PasswordResetConfirmBody struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}
