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

type Repository interface {
	CreateSession(ctx context.Context, session *Session) error
	GetSessionByTokenHash(ctx context.Context, tokenHash string) (*Session, error)
	RotateSession(ctx context.Context, oldTokenHash, newTokenHash string, expiresAt time.Time) (*Session, error)
	RevokeSession(ctx context.Context, sessionID string) error
	RevokeAllUserSessions(ctx context.Context, userID string) error
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

func (r *repoImpl) CreateSession(ctx context.Context, s *Session) error {
	const q = `
		INSERT INTO sessions (user_id, org_id, token_hash, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, '')::INET, $6)
		RETURNING id, public_id, created_at`
	err := r.db.QueryRow(ctx, q, s.UserID, s.OrgID, s.TokenHash, s.UserAgent, s.IPAddress, s.ExpiresAt).
		Scan(&s.ID, &s.PublicID, &s.CreatedAt)
	if err != nil {
		return fmt.Errorf("auth: CreateSession: %w", err)
	}
	return nil
}

func scanSession(row pgx.Row) (*Session, error) {
	s := &Session{}
	err := row.Scan(&s.ID, &s.PublicID, &s.UserID, &s.OrgID, &s.TokenHash, &s.UserAgent, &s.IPAddress, &s.ExpiresAt, &s.RevokedAt, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *repoImpl) GetSessionByTokenHash(ctx context.Context, tokenHash string) (*Session, error) {
	const q = `
		SELECT id, public_id, user_id, org_id, token_hash,
		       COALESCE(user_agent, ''), COALESCE(ip_address::TEXT, ''),
		       expires_at, revoked_at, created_at
		FROM sessions
		WHERE token_hash = $1`
	s, err := scanSession(r.db.QueryRow(ctx, q, tokenHash))
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("auth: GetSessionByTokenHash: %w", err)
	}
	return s, nil
}

// RotateSession revokes a refresh token and creates the next token atomically.
func (r *repoImpl) RotateSession(ctx context.Context, oldTokenHash, newTokenHash string, expiresAt time.Time) (*Session, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("auth: RotateSession: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const selectQ = `
		SELECT id, public_id, user_id, org_id, token_hash,
		       COALESCE(user_agent, ''), COALESCE(ip_address::TEXT, ''),
		       expires_at, revoked_at, created_at
		FROM sessions
		WHERE token_hash = $1
		FOR UPDATE`
	oldSession, err := scanSession(tx.QueryRow(ctx, selectQ, oldTokenHash))
	if err != nil {
		return nil, err
	}
	if oldSession.IsRevoked() {
		return nil, ErrSessionRevoked
	}
	if oldSession.IsExpired() {
		return nil, ErrSessionExpired
	}

	const revokeQ = `UPDATE sessions SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`
	cmd, err := tx.Exec(ctx, revokeQ, oldSession.ID)
	if err != nil {
		return nil, fmt.Errorf("auth: RotateSession: revoke: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return nil, ErrSessionRevoked
	}

	newSession := &Session{
		UserID: oldSession.UserID, OrgID: oldSession.OrgID, TokenHash: newTokenHash,
		UserAgent: oldSession.UserAgent, IPAddress: oldSession.IPAddress, ExpiresAt: expiresAt,
	}
	const insertQ = `
		INSERT INTO sessions (user_id, org_id, token_hash, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, '')::INET, $6)
		RETURNING id, public_id, created_at`
	if err := tx.QueryRow(ctx, insertQ, newSession.UserID, newSession.OrgID, newSession.TokenHash, newSession.UserAgent, newSession.IPAddress, newSession.ExpiresAt).
		Scan(&newSession.ID, &newSession.PublicID, &newSession.CreatedAt); err != nil {
		return nil, fmt.Errorf("auth: RotateSession: insert: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("auth: RotateSession: commit: %w", err)
	}
	return newSession, nil
}

func (r *repoImpl) RevokeSession(ctx context.Context, sessionID string) error {
	const q = `UPDATE sessions SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`
	_, err := r.db.Exec(ctx, q, sessionID)
	if err != nil {
		return fmt.Errorf("auth: RevokeSession: %w", err)
	}
	return nil
}

func (r *repoImpl) RevokeAllUserSessions(ctx context.Context, userID string) error {
	const q = `UPDATE sessions SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`
	_, err := r.db.Exec(ctx, q, userID)
	if err != nil {
		return fmt.Errorf("auth: RevokeAllUserSessions: %w", err)
	}
	return nil
}

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrSessionNotFound = errors.New("session not found")
var ErrSessionRevoked = errors.New("session revoked")
var ErrSessionExpired = errors.New("session expired")
var ErrEmailAlreadyExists = errors.New("email already registered")
var ErrAccountLocked = errors.New("account temporarily locked")
var ErrAccountDisabled = errors.New("account disabled")
var ErrPasswordLoginDisabled = errors.New("password login disabled")
