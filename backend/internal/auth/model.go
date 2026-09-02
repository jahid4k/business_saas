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
	Email       string `json:"email"`
	Password    string `json:"password"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	DisplayName string `json:"displayName"`
	// OrganizationName and OrganizationSlug are intentionally omitted.
	// Organizations are created separately via POST /api/v1/organizations
	// after the user has signed up and logged in.
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// TokenPair is the INTERNAL token representation used between service and handler.
//
// Security contract:
//   - RefreshToken is the RAW opaque token (never stored in DB — only its hash is).
//   - RefreshToken has json:"-" so it is NEVER serialised into any JSON response.
//   - The handler reads RefreshToken to set the httpOnly cookie, then discards it.
//   - Only AccessToken and ExpiresIn travel to the client, via ClientTokenPair.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"-"`          // handler sets cookie; never in response body
	ExpiresIn    int64  `json:"expires_in"` // seconds until access token expires
}

// ClientTokenPair is the JSON shape the handler writes to the response body.
// It contains no refresh token — the httpOnly cookie carries it.
func (t *TokenPair) ToClient() *ClientTokenPair {
	return &ClientTokenPair{
		AccessToken: t.AccessToken,
		ExpiresIn:   t.ExpiresIn,
	}
}

// ClientTokenPair is what the frontend receives in the response body.
type ClientTokenPair struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

// MobileTokenPair is what MOBILE clients receive in the response body.
//
// Unlike ClientTokenPair, it deliberately DOES include the raw refresh token:
// mobile has no cookie jar, so the token has nowhere else to travel. This is
// the one place in this package a raw refresh token is allowed into a JSON
// response — used only by the /auth/mobile/* routes. The client is expected
// to move it into expo-secure-store immediately and never persist it
// anywhere else (Zustand, AsyncStorage, etc.) — see Section 14 of
// docs/Project_Instruction.md.
type MobileTokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// ToMobileClient converts the internal TokenPair into the JSON shape sent to
// mobile clients.
func (t *TokenPair) ToMobileClient() *MobileTokenPair {
	return &MobileTokenPair{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		ExpiresIn:    t.ExpiresIn,
	}
}

// MobileRefreshRequest carries the refresh token in the request body.
// Mobile has no cookie to read it from, unlike web's Refresh.
type MobileRefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// MobileLogoutRequest carries the refresh token to revoke, in the request body.
// Mobile has no cookie to read it from, unlike web's Logout.
type MobileLogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
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

// OAuthSyncResponse is the INTERNAL response from service to handler.
//
// Tokens carries the full *TokenPair (including raw RefreshToken) so the
// handler can set the httpOnly cookie. The handler then calls Tokens.ToClient()
// before writing the JSON response — the raw refresh token never reaches the wire.
//
// The json tags on Tokens use "-" so the whole TokenPair is never accidentally
// serialised. The handler builds the final response shape manually.
type OAuthSyncResponse struct {
	User    any          `json:"user"`
	Account *AuthAccount `json:"account"`
	Tokens  *TokenPair   `json:"-"` // handler extracts cookie + calls ToClient(); never serialised directly
}

// OAuthSyncClientResponse is the JSON shape the handler sends to the frontend.
// It mirrors OAuthSyncResponse but replaces *TokenPair with *ClientTokenPair.
type OAuthSyncClientResponse struct {
	User    any              `json:"user"`
	Account *AuthAccount     `json:"account"`
	Tokens  *ClientTokenPair `json:"tokens,omitempty"`
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

type VerificationToken struct {
	ID        string     `db:"id"`
	PublicID  string     `db:"public_id"`
	UserID    *string    `db:"user_id"`
	Email     *string    `db:"email"`
	TokenHash string     `db:"token_hash"`
	Type      string     `db:"type"`
	VerifiedAt *time.Time `db:"verified_at"`
	UsedAt    *time.Time `db:"used_at"`
	ExpiresAt time.Time  `db:"expires_at"`
	CreatedAt time.Time  `db:"created_at"`
}
