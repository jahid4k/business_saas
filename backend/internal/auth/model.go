// backend/internal/auth/model.go
package auth

import "time"

// Session represents a persisted refresh token session in PostgreSQL.
// The raw token is NEVER stored — only its SHA-256 hash.
type Session struct {
	ID        string     `db:"id"`
	PublicID  string     `db:"public_id"`
	UserID    string     `db:"user_id"`
	OrgID     *string    `db:"org_id"`
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

// SignupRequest is the body for POST /api/v1/auth/signup or /sign-up.
type SignupRequest struct {
	Email            string `json:"email"`
	Password         string `json:"password"`
	FirstName        string `json:"first_name"`
	LastName         string `json:"last_name"`
	DisplayName      string `json:"displayName"`
	OrganizationName string `json:"organizationName"`
	OrganizationSlug string `json:"organizationSlug"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type PasswordResetRequestBody struct {
	Email string `json:"email"`
}

type PasswordResetConfirmBody struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}
