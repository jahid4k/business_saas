// Package jwt handles issuing and parsing JWT access tokens for BusinessSAAS.
//
// Token design:
//   - Algorithm: HS256 (HMAC-SHA256 with a symmetric secret)
//   - Signed with JWT_SECRET from config
//   - Short TTL (default 15 minutes)
//   - Claims include: user_id, business_id (if selected), email, role
//
// The refresh token is a separate opaque token (see pkg/token).
// JWTs are NOT stored in the database — they are stateless.
// Session state (revocation) is tracked via opaque refresh tokens in PostgreSQL.
package jwt

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims defines the payload of a BusinessSAAS access token.
// Embeds jwt.RegisteredClaims for standard fields (exp, iat, iss).
type Claims struct {
	jwt.RegisteredClaims

	// UserID is the authenticated user's UUID.
	UserID string `json:"uid"`

	// BusinessID is the active workspace UUID.
	// Empty string means no business context is selected yet
	// (e.g. after signup, before creating or selecting a workspace).
	BusinessID string `json:"bid,omitempty"`

	// Email is included for convenience in frontend display.
	// It is NOT used for authentication decisions.
	Email string `json:"email"`

	// Role is the user's role in the current business.
	// Empty when BusinessID is empty.
	// Used for coarse client-side UI decisions only.
	// All permission checks on the backend use the database, not this field.
	Role string `json:"role,omitempty"`
}

// Manager handles JWT operations using a shared secret.
type Manager struct {
	secret    []byte
	accessTTL time.Duration
	issuer    string
}

// NewManager creates a JWT Manager from the given secret and TTL.
func NewManager(secret string, accessTTL time.Duration) *Manager {
	return &Manager{
		secret:    []byte(secret),
		accessTTL: accessTTL,
		issuer:    "businesssaas",
	}
}

// IssueAccessToken creates and signs a new access token for the given user.
func (m *Manager) IssueAccessToken(userID, email, businessID, role string) (string, error) {
	now := time.Now()

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
		},
		UserID:     userID,
		BusinessID: businessID,
		Email:      email,
		Role:       role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("jwt: failed to sign token: %w", err)
	}

	return signed, nil
}

// Parse validates the token string and returns the claims.
//
// Returns:
//   - *Claims if the token is valid and not expired
//   - ErrTokenExpired if the token has expired
//   - ErrTokenInvalid for any other validation failure
func (m *Manager) Parse(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(t *jwt.Token) (any, error) {
			// Verify signing algorithm — prevent algorithm substitution attacks
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("jwt: unexpected signing method: %v", t.Header["alg"])
			}
			return m.secret, nil
		},
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
	)

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrTokenInvalid
	}

	return claims, nil
}

// ----------------------------------------------------------
// Sentinel errors
// ----------------------------------------------------------

// ErrTokenExpired is returned when the JWT has passed its expiry time.
var ErrTokenExpired = errors.New("jwt: token expired")

// ErrTokenInvalid is returned for any other JWT validation failure.
// This is intentionally vague — clients should only know that the token is bad.
var ErrTokenInvalid = errors.New("jwt: token invalid")
