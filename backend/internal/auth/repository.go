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
	FindAuthAccount(ctx context.Context, provider, providerAccountID string) (*AuthAccount, error)
	CreateAuthAccount(ctx context.Context, account *AuthAccount, accessToken, refreshToken, idToken string) error
	UpdateAuthAccount(ctx context.Context, account *AuthAccount, accessToken, refreshToken, idToken string) error
	CreateVerificationToken(ctx context.Context, vt *VerificationToken) error
	GetVerificationTokenByHash(ctx context.Context, tokenHash, tokenType string) (*VerificationToken, error)
	MarkVerificationTokenUsed(ctx context.Context, id string) error
	CreateLoginEvent(ctx context.Context, event LoginEvent) error
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

// GetSessionByTokenHash fetches a session by its token hash.
// FIX: query now filters revoked_at IS NULL AND expires_at > NOW() at the DB level
// for defence-in-depth, so callers never accidentally receive an invalid session.
func (r *repoImpl) GetSessionByTokenHash(ctx context.Context, tokenHash string) (*Session, error) {
	const q = `
		SELECT id, public_id, user_id, org_id, token_hash,
		       COALESCE(user_agent, ''), COALESCE(ip_address::TEXT, ''),
		       expires_at, revoked_at, created_at
		FROM sessions
		WHERE token_hash = $1
		  AND revoked_at IS NULL
		  AND expires_at > NOW()`
	s, err := scanSession(r.db.QueryRow(ctx, q, tokenHash))
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("auth: GetSessionByTokenHash: %w", err)
	}
	return s, nil
}

// RotateSession atomically revokes the old session and inserts a new one within
// a single transaction, using SELECT FOR UPDATE to prevent concurrent rotations.
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
		UserID: oldSession.UserID, OrgID: oldSession.OrgID,
		TokenHash: newTokenHash, UserAgent: oldSession.UserAgent,
		IPAddress: oldSession.IPAddress, ExpiresAt: expiresAt,
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

func scanAuthAccount(row pgx.Row) (*AuthAccount, error) {
	a := &AuthAccount{}
	err := row.Scan(
		&a.ID, &a.PublicID, &a.UserID, &a.Provider, &a.ProviderAccountID, &a.ProviderType,
		&a.TokenType, &a.Scope, &a.ExpiresAt, &a.ConnectedAt, &a.LastUsedAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

const accountSelect = `id, public_id, user_id, provider, provider_account_id, provider_type, COALESCE(token_type, ''), COALESCE(scope, ''), expires_at, connected_at, last_used_at, created_at, updated_at`

func (r *repoImpl) FindAuthAccount(ctx context.Context, provider, providerAccountID string) (*AuthAccount, error) {
	q := `SELECT ` + accountSelect + ` FROM auth_accounts WHERE provider = $1 AND provider_account_id = $2`
	a, err := scanAuthAccount(r.db.QueryRow(ctx, q, provider, providerAccountID))
	if err != nil {
		return nil, fmt.Errorf("auth: FindAuthAccount: %w", err)
	}
	return a, nil
}

func (r *repoImpl) CreateAuthAccount(ctx context.Context, a *AuthAccount, accessToken, refreshToken, idToken string) error {
	const q = `
		INSERT INTO auth_accounts (user_id, provider, provider_account_id, provider_type, access_token, refresh_token, id_token, token_type, scope, expires_at, last_used_at)
		VALUES ($1, $2, $3, COALESCE(NULLIF($4, ''), 'oauth'), NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), NULLIF($9, ''), $10, NOW())
		RETURNING id, public_id, connected_at, last_used_at, created_at, updated_at`
	err := r.db.QueryRow(ctx, q, a.UserID, a.Provider, a.ProviderAccountID, a.ProviderType, accessToken, refreshToken, idToken, a.TokenType, a.Scope, a.ExpiresAt).
		Scan(&a.ID, &a.PublicID, &a.ConnectedAt, &a.LastUsedAt, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return fmt.Errorf("auth: CreateAuthAccount: %w", err)
	}
	return nil
}

func (r *repoImpl) UpdateAuthAccount(ctx context.Context, a *AuthAccount, accessToken, refreshToken, idToken string) error {
	const q = `
		UPDATE auth_accounts
		SET access_token  = COALESCE(NULLIF($1, ''), access_token),
		    refresh_token = COALESCE(NULLIF($2, ''), refresh_token),
		    id_token      = COALESCE(NULLIF($3, ''), id_token),
		    token_type    = COALESCE(NULLIF($4, ''), token_type),
		    scope         = COALESCE(NULLIF($5, ''), scope),
		    expires_at    = COALESCE($6, expires_at),
		    last_used_at  = NOW(),
		    updated_at    = NOW()
		WHERE id = $7
		RETURNING last_used_at, updated_at`
	err := r.db.QueryRow(ctx, q, accessToken, refreshToken, idToken, a.TokenType, a.Scope, a.ExpiresAt, a.ID).
		Scan(&a.LastUsedAt, &a.UpdatedAt)
	if err != nil {
		return fmt.Errorf("auth: UpdateAuthAccount: %w", err)
	}
	return nil
}

func (r *repoImpl) CreateLoginEvent(ctx context.Context, e LoginEvent) error {
	const q = `
		INSERT INTO login_events (user_id, email, provider, status, failure_reason, ip_address, user_agent)
		VALUES ($1, NULLIF($2, ''), COALESCE(NULLIF($3, ''), 'credentials'), $4, NULLIF($5, ''), NULLIF($6, '')::INET, NULLIF($7, ''))`
	_, err := r.db.Exec(ctx, q, e.UserID, e.Email, e.Provider, e.Status, e.FailureReason, e.IPAddress, e.UserAgent)
	if err != nil {
		return fmt.Errorf("auth: CreateLoginEvent: %w", err)
	}
	return nil
}

var (
	ErrInvalidCredentials    = errors.New("invalid credentials")
	ErrSessionNotFound       = errors.New("session not found")
	ErrSessionRevoked        = errors.New("session revoked")
	ErrSessionExpired        = errors.New("session expired")
	ErrEmailAlreadyExists    = errors.New("email already registered")
	ErrAccountLocked         = errors.New("account temporarily locked")
	ErrAccountDisabled       = errors.New("account disabled")
	ErrPasswordLoginDisabled = errors.New("password login disabled")
	ErrOAuthProviderRequired = errors.New("oauth provider is required")
	ErrOAuthAccountIDRequired = errors.New("oauth provider account id is required")
	ErrOAuthEmailRequired    = errors.New("oauth email is required")
)

func (r *repoImpl) CreateVerificationToken(ctx context.Context, vt *VerificationToken) error {
	q := `
		INSERT INTO verification_tokens (user_id, email, token_hash, type, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, public_id, created_at
	`
	err := r.db.QueryRow(ctx, q, vt.UserID, vt.Email, vt.TokenHash, vt.Type, vt.ExpiresAt).
		Scan(&vt.ID, &vt.PublicID, &vt.CreatedAt)
	if err != nil {
		return fmt.Errorf("auth: CreateVerificationToken: %w", err)
	}
	return nil
}

func (r *repoImpl) GetVerificationTokenByHash(ctx context.Context, tokenHash, tokenType string) (*VerificationToken, error) {
	q := `
		SELECT id, public_id, user_id, email, token_hash, type, verified_at, used_at, expires_at, created_at
		FROM verification_tokens
		WHERE token_hash = $1 AND type = $2
	`
	var vt VerificationToken
	err := r.db.QueryRow(ctx, q, tokenHash, tokenType).
		Scan(&vt.ID, &vt.PublicID, &vt.UserID, &vt.Email, &vt.TokenHash, &vt.Type, &vt.VerifiedAt, &vt.UsedAt, &vt.ExpiresAt, &vt.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("auth: GetVerificationTokenByHash: %w", err)
	}
	return &vt, nil
}

func (r *repoImpl) MarkVerificationTokenUsed(ctx context.Context, id string) error {
	q := `UPDATE verification_tokens SET used_at = NOW() WHERE id = $1`
	_, err := r.db.Exec(ctx, q, id)
	return err
}
