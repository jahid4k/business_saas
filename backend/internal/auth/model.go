// backend/internal/auth/model.go
package auth

import "time"

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

type AuthAccount struct {
	ID                string     `json:"id"`
	PublicID          string     `json:"publicId"`
	UserID            string     `json:"userId"`
	Provider          string     `json:"provider"`
	ProviderAccountID string     `json:"providerAccountId"`
	ProviderType      string     `json:"providerType"`
	TokenType         string     `json:"tokenType,omitempty"`
	Scope             string     `json:"scope,omitempty"`
	ExpiresAt         *time.Time `json:"expiresAt,omitempty"`
	ConnectedAt       time.Time  `json:"connectedAt"`
	LastUsedAt        *time.Time `json:"lastUsedAt,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

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

type OAuthSyncRequest struct {
	Provider          string     `json:"provider"`
	ProviderAccountID string     `json:"providerAccountId"`
	ProviderType      string     `json:"providerType"`
	Email             string     `json:"email"`
	EmailVerified     bool       `json:"emailVerified"`
	DisplayName       string     `json:"displayName"`
	FirstName         string     `json:"firstName"`
	LastName          string     `json:"lastName"`
	PhotoURL          string     `json:"photoURL"`
	AccessToken       string     `json:"accessToken"`
	RefreshToken      string     `json:"refreshToken"`
	IDToken           string     `json:"idToken"`
	TokenType         string     `json:"tokenType"`
	Scope             string     `json:"scope"`
	ExpiresAt         *time.Time `json:"expiresAt"`
	IssueTokens       *bool      `json:"issueTokens"`
}

type OAuthSyncResponse struct {
	User    any          `json:"user"`
	Account *AuthAccount `json:"account"`
	Tokens  *TokenPair   `json:"tokens,omitempty"`
}

type LoginEvent struct {
	UserID        *string
	Email         string
	Provider      string
	Status        string
	FailureReason string
	IPAddress     string
	UserAgent     string
}
