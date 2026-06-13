// backend/internal/tests/unit/jwt_test.go
package unit

import (
	"errors"
	"testing"
	"time"

	jwtpkg "github.com/mridha/businesssaas/pkg/jwt"
)

func TestJWT_IssueAndParse(t *testing.T) {
	mgr := jwtpkg.NewManager("test-secret-that-is-long-enough", 15*time.Minute)
	tok, err := mgr.IssueAccessToken("user-1", "alice@example.com", "org-1", "admin")
	if err != nil {
		t.Fatalf("IssueAccessToken() error: %v", err)
	}
	if tok == "" {
		t.Fatal("expected non-empty token")
	}
	claims, err := mgr.Parse(tok)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if claims.UserID != "user-1" {
		t.Errorf("UserID: got %q, want %q", claims.UserID, "user-1")
	}
	if claims.BusinessID != "org-1" {
		t.Errorf("BusinessID: got %q, want %q", claims.BusinessID, "org-1")
	}
	if claims.Email != "alice@example.com" {
		t.Errorf("Email: got %q, want %q", claims.Email, "alice@example.com")
	}
}

func TestJWT_ExpiredToken(t *testing.T) {
	mgr := jwtpkg.NewManager("test-secret-that-is-long-enough", -1*time.Second) // already expired
	tok, err := mgr.IssueAccessToken("user-2", "bob@example.com", "", "")
	if err != nil {
		t.Fatalf("IssueAccessToken() error: %v", err)
	}
	_, parseErr := mgr.Parse(tok)
	if parseErr == nil {
		t.Fatal("expected error for expired token, got nil")
	}
	if !errors.Is(parseErr, jwtpkg.ErrTokenExpired) {
		t.Errorf("expected ErrTokenExpired, got: %v", parseErr)
	}
}

func TestJWT_InvalidToken(t *testing.T) {
	mgr := jwtpkg.NewManager("test-secret-that-is-long-enough", 15*time.Minute)
	_, err := mgr.Parse("this.is.not.a.valid.jwt")
	if err == nil {
		t.Fatal("expected error for garbage token, got nil")
	}
}

func TestJWT_WrongSecret(t *testing.T) {
	issuer := jwtpkg.NewManager("secret-A-long-enough-padding", 15*time.Minute)
	verifier := jwtpkg.NewManager("secret-B-long-enough-padding", 15*time.Minute)

	tok, _ := issuer.IssueAccessToken("user-3", "carol@example.com", "", "")
	_, err := verifier.Parse(tok)
	if err == nil {
		t.Fatal("expected error when verifying with wrong secret, got nil")
	}
}
