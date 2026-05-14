// Package password provides bcrypt-based password hashing and verification
// for BusinessSAAS user accounts.
//
// Security properties:
//   - Uses bcrypt with cost factor 12 (adjustable, recommended minimum: 12)
//   - Max input length is 72 bytes (bcrypt's natural limit)
//   - Constant-time comparison via bcrypt.CompareHashAndPassword
//   - Plain-text passwords are NEVER logged, stored, or returned
package password

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const (
	// DefaultCost is the bcrypt work factor.
	// Cost 12 takes ~300ms on modern hardware — fast enough for UX,
	// slow enough to resist brute force.
	// Increase to 13 or 14 for higher security requirements.
	DefaultCost = 12

	// MaxPasswordBytes is bcrypt's hard limit.
	// Passwords longer than this are silently truncated by bcrypt.
	// We reject them explicitly so the user knows their full password matters.
	MaxPasswordBytes = 72
)

// ErrPasswordTooLong is returned when the input exceeds bcrypt's limit.
var ErrPasswordTooLong = errors.New("password exceeds maximum length of 72 characters")

// Hash takes a plain-text password and returns its bcrypt hash.
// The returned hash is safe to store in the database.
//
// Returns ErrPasswordTooLong if the password exceeds 72 bytes.
func Hash(plaintext string) (string, error) {
	if len(plaintext) > MaxPasswordBytes {
		return "", ErrPasswordTooLong
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), DefaultCost)
	if err != nil {
		return "", fmt.Errorf("password: hash failed: %w", err)
	}

	return string(hash), nil
}

// Verify checks whether the plain-text password matches the stored bcrypt hash.
//
// Returns:
//   - nil if the password matches
//   - ErrMismatch if the password does not match
//   - Another error for unexpected failures
//
// Uses constant-time comparison — safe against timing attacks.
func Verify(plaintext, hash string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return ErrMismatch
	}
	if err != nil {
		return fmt.Errorf("password: verify failed: %w", err)
	}
	return nil
}

// ErrMismatch is returned when the password does not match the hash.
// Callers should map this to a generic "invalid credentials" error
// before returning anything to the client.
var ErrMismatch = errors.New("password mismatch")

// NeedsRehash returns true if the stored hash was created with a lower cost
// than the current DefaultCost. Use this to trigger transparent rehashing
// on successful login.
func NeedsRehash(hash string) bool {
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		return false
	}
	return cost < DefaultCost
}
