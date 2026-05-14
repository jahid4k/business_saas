package auth

import (
	"context"
	"errors"
	"fmt"
)

// Repository defines the data access interface for auth operations.
// The service depends on this interface, not pgx directly.
// This keeps SQL isolated to the repository layer and makes services testable.
type Repository interface {
	CreateSession(ctx context.Context, session *Session) error
	GetSessionByTokenHash(ctx context.Context, tokenHash string) (*Session, error)
	RevokeSession(ctx context.Context, sessionID string) error
	RevokeAllUserSessions(ctx context.Context, userID string) error
}

// repoImpl is the concrete pgx implementation of Repository.
type repoImpl struct {
	// TODO (Phase 1-B): add pgxpool.Pool field
}

// NewRepository creates a new auth repository.
// Phase 1-B will wire in the actual pgxpool.Pool.
func NewRepository() Repository {
	return &repoImpl{}
}

// TODO (Phase 1-B): implement all repository methods with real pgx queries.
// All queries must use parameterized statements — never string concatenation.

func (r *repoImpl) CreateSession(_ context.Context, _ *Session) error {
	return errNotImplemented("CreateSession")
}

func (r *repoImpl) GetSessionByTokenHash(_ context.Context, _ string) (*Session, error) {
	return nil, errNotImplemented("GetSessionByTokenHash")
}

func (r *repoImpl) RevokeSession(_ context.Context, _ string) error {
	return errNotImplemented("RevokeSession")
}

func (r *repoImpl) RevokeAllUserSessions(_ context.Context, _ string) error {
	return errNotImplemented("RevokeAllUserSessions")
}

// ----------------------------------------------------------
// Shared sentinel errors for the auth package
// ----------------------------------------------------------

// ErrInvalidCredentials is returned when email/password do not match.
// IMPORTANT: this is intentionally vague — never reveal which field failed.
var ErrInvalidCredentials = errors.New("invalid credentials")

// ErrSessionNotFound is returned when a refresh token cannot be found.
var ErrSessionNotFound = errors.New("session not found")

// ErrSessionRevoked is returned when a session has been explicitly revoked.
var ErrSessionRevoked = errors.New("session revoked")

// ErrSessionExpired is returned when a session has passed its TTL.
var ErrSessionExpired = errors.New("session expired")

// ErrEmailAlreadyExists is returned during signup if the email is taken.
var ErrEmailAlreadyExists = errors.New("email already registered")

// ErrAccountLocked is returned after too many failed login attempts.
var ErrAccountLocked = errors.New("account temporarily locked")

// errNotImplemented is a helper for stub methods.
func errNotImplemented(method string) error {
	return fmt.Errorf("auth: %s: not yet implemented", method)
}
