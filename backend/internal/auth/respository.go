// backend/internal/auth/repository.go
package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines the data access interface for auth session operations.
type Repository interface {
	CreateSession(ctx context.Context, session *Session) error
	GetSessionByTokenHash(ctx context.Context, tokenHash string) (*Session, error)
	RevokeSession(ctx context.Context, sessionID string) error
	RevokeAllUserSessions(ctx context.Context, userID string) error
}

type repoImpl struct {
	db *pgxpool.Pool
}

// NewRepository creates a new auth repository backed by a pgxpool.
func NewRepository(db *pgxpool.Pool) Repository {
	return &repoImpl{db: db}
}

// CreateSession inserts a new session row.
// NEVER pass the raw token — only pass the SHA-256 hash in session.TokenHash.
func (r *repoImpl) CreateSession(ctx context.Context, s *Session) error {
	const q = `
		INSERT INTO sessions (user_id, token_hash, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`

	err := r.db.QueryRow(ctx, q,
		s.UserID, s.TokenHash, s.UserAgent, s.IPAddress, s.ExpiresAt,
	).Scan(&s.ID, &s.CreatedAt)
	if err != nil {
		return fmt.Errorf("auth: CreateSession: %w", err)
	}
	return nil
}

// GetSessionByTokenHash looks up a session by its SHA-256 token hash.
func (r *repoImpl) GetSessionByTokenHash(ctx context.Context, tokenHash string) (*Session, error) {
	const q = `
		SELECT id, user_id, token_hash, user_agent, ip_address,
		       expires_at, revoked_at, created_at
		FROM sessions
		WHERE token_hash = $1`

	s := &Session{}
	err := r.db.QueryRow(ctx, q, tokenHash).Scan(
		&s.ID, &s.UserID, &s.TokenHash,
		&s.UserAgent, &s.IPAddress,
		&s.ExpiresAt, &s.RevokedAt, &s.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("auth: GetSessionByTokenHash: %w", err)
	}
	return s, nil
}

// RevokeSession marks a single session revoked by setting revoked_at = NOW().
func (r *repoImpl) RevokeSession(ctx context.Context, sessionID string) error {
	const q = `
		UPDATE sessions
		SET revoked_at = $1
		WHERE id = $2 AND revoked_at IS NULL`

	_, err := r.db.Exec(ctx, q, time.Now(), sessionID)
	if err != nil {
		return fmt.Errorf("auth: RevokeSession: %w", err)
	}
	return nil
}

// RevokeAllUserSessions marks every active session for the user revoked.
// Used by logout-all and password-change flows.
func (r *repoImpl) RevokeAllUserSessions(ctx context.Context, userID string) error {
	const q = `
		UPDATE sessions
		SET revoked_at = $1
		WHERE user_id = $2 AND revoked_at IS NULL`

	_, err := r.db.Exec(ctx, q, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("auth: RevokeAllUserSessions: %w", err)
	}
	return nil
}

// ----------------------------------------------------------
// Sentinel errors
// ----------------------------------------------------------

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrSessionNotFound = errors.New("session not found")
var ErrSessionRevoked = errors.New("session revoked")
var ErrSessionExpired = errors.New("session expired")
var ErrEmailAlreadyExists = errors.New("email already registered")
var ErrAccountLocked = errors.New("account temporarily locked")
