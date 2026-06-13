package unit

import (
	"strings"
	"testing"

	"github.com/mridha/businesssaas/pkg/password"
)

// TestHash_ValidPassword verifies that hashing a normal password succeeds
// and returns a non-empty, non-plaintext value.
func TestHash_ValidPassword(t *testing.T) {
	plain := "correct-horse-battery-staple"

	hash, err := password.Hash(plain)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if hash == plain {
		t.Fatal("hash must not equal plaintext")
	}
	// bcrypt hashes start with "$2a$" or "$2b$"
	if !strings.HasPrefix(hash, "$2") {
		t.Fatalf("expected bcrypt hash, got: %s", hash)
	}
}

// TestHash_TooLong verifies that passwords exceeding 72 bytes are rejected.
func TestHash_TooLong(t *testing.T) {
	tooLong := strings.Repeat("a", 73)

	_, err := password.Hash(tooLong)
	if err == nil {
		t.Fatal("expected error for password > 72 bytes, got nil")
	}
	if err != password.ErrPasswordTooLong {
		t.Fatalf("expected ErrPasswordTooLong, got: %v", err)
	}
}

// TestVerify_Correct verifies that Verify returns nil for the correct password.
func TestVerify_Correct(t *testing.T) {
	plain := "my-secure-password-123"

	hash, err := password.Hash(plain)
	if err != nil {
		t.Fatalf("hash failed: %v", err)
	}

	if err := password.Verify(plain, hash); err != nil {
		t.Fatalf("expected nil error for correct password, got: %v", err)
	}
}

// TestVerify_Wrong verifies that Verify returns ErrMismatch for a wrong password.
func TestVerify_Wrong(t *testing.T) {
	hash, _ := password.Hash("correct-password")

	err := password.Verify("wrong-password", hash)
	if err == nil {
		t.Fatal("expected error for wrong password, got nil")
	}
	if err != password.ErrMismatch {
		t.Fatalf("expected ErrMismatch, got: %v", err)
	}
}

// TestVerify_Empty verifies that an empty password does not match a real hash.
func TestVerify_Empty(t *testing.T) {
	hash, _ := password.Hash("some-password")

	if err := password.Verify("", hash); err == nil {
		t.Fatal("expected error for empty password, got nil")
	}
}

// TestHash_Idempotent verifies that hashing the same password twice
// produces different hashes (bcrypt uses random salts).
func TestHash_Idempotent(t *testing.T) {
	plain := "same-password"

	hash1, _ := password.Hash(plain)
	hash2, _ := password.Hash(plain)

	if hash1 == hash2 {
		t.Fatal("expected different hashes for the same password (salt must be random)")
	}

	// Both hashes must still verify correctly
	if err := password.Verify(plain, hash1); err != nil {
		t.Fatalf("hash1 verify failed: %v", err)
	}
	if err := password.Verify(plain, hash2); err != nil {
		t.Fatalf("hash2 verify failed: %v", err)
	}
}
