// backend/internal/tests/unit/pkg/jwt_test.go
package pkg

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	bsjwt "github.com/mridha/businesssaas/pkg/jwt"
)

func TestJWTManager_IssueAndParse_Success(t *testing.T) {
	secret := "test-secret"
	ttl := 15 * time.Minute
	manager := bsjwt.NewManager(secret, ttl)

	userID := "user-123"
	email := "test@example.com"
	businessID := "biz-456"
	role := "admin"

	tokenStr, err := manager.IssueAccessToken(userID, email, businessID, role)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if tokenStr == "" {
		t.Fatal("expected token string to not be empty")
	}

	claims, err := manager.Parse(tokenStr)
	if err != nil {
		t.Fatalf("expected no error parsing valid token, got %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("expected userID %q, got %q", userID, claims.UserID)
	}
	if claims.Email != email {
		t.Errorf("expected email %q, got %q", email, claims.Email)
	}
	if claims.BusinessID != businessID {
		t.Errorf("expected businessID %q, got %q", businessID, claims.BusinessID)
	}
	if claims.Role != role {
		t.Errorf("expected role %q, got %q", role, claims.Role)
	}
	if claims.Issuer != "businesssaas" {
		t.Errorf("expected issuer 'businesssaas', got %q", claims.Issuer)
	}
	if claims.Subject != userID {
		t.Errorf("expected subject %q, got %q", userID, claims.Subject)
	}
}

func TestJWTManager_Parse_ExpiredToken(t *testing.T) {
	secret := "test-secret"
	// Create a manager with negative TTL to generate expired tokens instantly
	ttl := -1 * time.Minute
	manager := bsjwt.NewManager(secret, ttl)

	tokenStr, err := manager.IssueAccessToken("u1", "e@example.com", "b1", "user")
	if err != nil {
		t.Fatalf("expected no error issuing token, got %v", err)
	}

	_, err = manager.Parse(tokenStr)
	if !errors.Is(err, bsjwt.ErrTokenExpired) {
		t.Fatalf("expected ErrTokenExpired, got %v", err)
	}
}

func TestJWTManager_Parse_WrongSignature(t *testing.T) {
	manager1 := bsjwt.NewManager("secret-one", 15*time.Minute)
	manager2 := bsjwt.NewManager("secret-two", 15*time.Minute)

	tokenStr, err := manager1.IssueAccessToken("u1", "e@example.com", "b1", "user")
	if err != nil {
		t.Fatalf("expected no error issuing token, got %v", err)
	}

	_, err = manager2.Parse(tokenStr)
	if !errors.Is(err, bsjwt.ErrTokenInvalid) {
		t.Fatalf("expected ErrTokenInvalid when using wrong secret, got %v", err)
	}
}

func TestJWTManager_Parse_InvalidTokenFormat(t *testing.T) {
	manager := bsjwt.NewManager("secret", 15*time.Minute)

	_, err := manager.Parse("not.a.valid.token")
	if !errors.Is(err, bsjwt.ErrTokenInvalid) {
		t.Fatalf("expected ErrTokenInvalid for bad format, got %v", err)
	}
}

func TestJWTManager_Parse_WrongSigningMethod(t *testing.T) {
	// A token signed with a different algorithm (e.g. none) should be rejected
	manager := bsjwt.NewManager("secret", 15*time.Minute)

	now := time.Now()
	claims := bsjwt.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "businesssaas",
			Subject:   "u1",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
		},
		UserID: "u1",
	}

	// Create a token with "none" signing method
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenStr, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("expected no error signing with none, got %v", err)
	}

	_, err = manager.Parse(tokenStr)
	if !errors.Is(err, bsjwt.ErrTokenInvalid) {
		t.Fatalf("expected ErrTokenInvalid for wrong signing method, got %v", err)
	}
}
