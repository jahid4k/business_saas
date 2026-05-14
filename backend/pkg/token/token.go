// Package token handles generation and hashing of opaque tokens used for:
//   - Refresh tokens
//   - Password reset tokens
//   - Email verification tokens
//
// Security model:
//   - The raw token is returned to the client once and never stored
//   - Only the SHA-256 hash of the token is stored in the database
//   - On verification: the client sends the raw token, we hash it and compare
//   - This means a database compromise does NOT expose valid tokens
package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

const (
	// DefaultTokenBytes is the number of random bytes in a generated token.
	// 32 bytes = 256 bits of entropy when base64-encoded.
	DefaultTokenBytes = 32
)

// Generate creates a cryptographically secure random opaque token.
// The token is base64url-encoded (URL-safe, no padding).
//
// Returns:
//   - rawToken: the token to return to the client (store in cookie/header)
//   - hash:     the SHA-256 hex hash to store in the database
//   - error:    if the system CSPRNG fails (extremely rare)
//
// NEVER store rawToken in the database.
// NEVER log rawToken.
// NEVER return hash to the client.
func Generate() (rawToken, hash string, err error) {
	b := make([]byte, DefaultTokenBytes)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("token: failed to generate random bytes: %w", err)
	}

	rawToken = base64.RawURLEncoding.EncodeToString(b)
	hash = Hash(rawToken)

	return rawToken, hash, nil
}

// Hash computes the SHA-256 hex digest of a raw token.
// This is the value stored in the database for lookup and comparison.
//
// SHA-256 is used (not bcrypt) because:
//   - Tokens have full 256-bit entropy (bcrypt adds no security benefit here)
//   - Lookup must be fast (we hash incoming token and look it up by hash)
//   - bcrypt's slow hashing would cause unnecessary latency on every refresh
func Hash(rawToken string) string {
	h := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(h[:])
}

// Equal compares a raw token against a stored hash in constant time.
// Returns true if they match.
func Equal(rawToken, storedHash string) bool {
	computed := Hash(rawToken)
	// Use a constant-time comparison to prevent timing attacks
	return constantTimeEqual(computed, storedHash)
}

// constantTimeEqual compares two strings in constant time.
// This is important when comparing tokens to prevent timing side-channels.
func constantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range len(a) {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
