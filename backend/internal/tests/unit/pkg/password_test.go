// backend/internal/tests/unit/pkg/password_test.go
package pkg

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
	"github.com/mridha/businesssaas/pkg/password"
)

func TestPassword_HashAndVerify_Success(t *testing.T) {
	plaintext := "my-secure-password"

	hash, err := password.Hash(plaintext)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if hash == "" {
		t.Fatal("expected hash string to not be empty")
	}

	err = password.Verify(plaintext, hash)
	if err != nil {
		t.Fatalf("expected no error verifying valid password, got %v", err)
	}
}

func TestPassword_Hash_TooLong(t *testing.T) {
	plaintext := strings.Repeat("a", 73)

	_, err := password.Hash(plaintext)
	if !errors.Is(err, password.ErrPasswordTooLong) {
		t.Fatalf("expected ErrPasswordTooLong, got %v", err)
	}
}

func TestPassword_Verify_Mismatch(t *testing.T) {
	plaintext := "correct-horse-battery-staple"
	wrongPassword := "wrong-password"

	hash, err := password.Hash(plaintext)
	if err != nil {
		t.Fatalf("expected no error hashing, got %v", err)
	}

	err = password.Verify(wrongPassword, hash)
	if !errors.Is(err, password.ErrMismatch) {
		t.Fatalf("expected ErrMismatch, got %v", err)
	}
}

func TestPassword_Verify_InvalidHash(t *testing.T) {
	err := password.Verify("any-password", "not-a-bcrypt-hash")
	if err == nil {
		t.Fatal("expected error verifying against invalid hash")
	}
	if errors.Is(err, password.ErrMismatch) {
		t.Fatalf("expected a format error, got ErrMismatch")
	}
}

func TestPassword_NeedsRehash(t *testing.T) {
	// Generate a hash with a low cost to simulate an old hash
	lowCostHashBytes, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("expected no error generating low cost hash, got %v", err)
	}
	lowCostHash := string(lowCostHashBytes)

	if !password.NeedsRehash(lowCostHash) {
		t.Error("expected NeedsRehash to be true for low cost hash")
	}

	// Generate a hash with default cost
	defaultCostHash, err := password.Hash("password")
	if err != nil {
		t.Fatalf("expected no error generating default cost hash, got %v", err)
	}

	if password.NeedsRehash(defaultCostHash) {
		t.Error("expected NeedsRehash to be false for default cost hash")
	}
}
