// backend/internal/security/repository.go
package security

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// defaultSessionLimit and maxSessionLimit guard against unbounded result sets.
const (
	defaultSessionLimit = 200
	maxSessionLimit     = 1000
	defaultEventLimit   = 100
	maxEventLimit       = 500
)

type Repository interface {
	ListOrganizationSessions(ctx context.Context, organizationID string, limit int) ([]*SessionView, error)
	RevokeOrganizationSession(ctx context.Context, organizationID, sessionRef string) error
	ListOrganizationLoginEvents(ctx context.Context, organizationID string, limit int) ([]*LoginEventView, error)
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

// ListOrganizationSessions returns active and recent sessions for all members of the org.
// FIX: now accepts a limit parameter to prevent unbounded memory usage in large organizations.
func (r *repoImpl) ListOrganizationSessions(ctx context.Context, organizationID string, limit int) ([]*SessionView, error) {
	if limit <= 0 || limit > maxSessionLimit {
		limit = defaultSessionLimit
	}
	const q = `
		SELECT s.id, s.public_id, u.id, u.public_id, COALESCE(u.email, ''), u.display_name,
		       COALESCE(s.user_agent, ''), COALESCE(s.ip_address::TEXT, ''), COALESCE(s.country, ''), COALESCE(s.city, ''), COALESCE(s.region, ''),
		       s.last_activity_at, s.created_at, s.expires_at, s.revoked_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		JOIN organization_members om ON om.user_id = u.id AND om.org_id = $1
		WHERE om.status = 'active'
		ORDER BY s.last_activity_at DESC, s.created_at DESC
		LIMIT $2`
	rows, err := r.db.Query(ctx, q, organizationID, limit)
	if err != nil {
		return nil, fmt.Errorf("security: ListOrganizationSessions: %w", err)
	}
	defer rows.Close()
	var sessions []*SessionView
	now := time.Now()
	for rows.Next() {
		s := &SessionView{}
		if err := rows.Scan(&s.ID, &s.PublicID, &s.UserID, &s.UserPublicID, &s.Email, &s.DisplayName, &s.UserAgent, &s.IPAddress, &s.Country, &s.City, &s.Region, &s.LastActivityAt, &s.CreatedAt, &s.ExpiresAt, &s.RevokedAt); err != nil {
			return nil, fmt.Errorf("security: ListOrganizationSessions: scan: %w", err)
		}
		s.IsActive = s.RevokedAt == nil && now.Before(s.ExpiresAt)
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func (r *repoImpl) RevokeOrganizationSession(ctx context.Context, organizationID, sessionRef string) error {
	const q = `
		UPDATE sessions s
		SET revoked_at = NOW()
		WHERE (s.id::TEXT = $1 OR s.public_id = $1)
		  AND s.revoked_at IS NULL
		  AND EXISTS (
			SELECT 1 FROM organization_members om
			WHERE om.org_id = $2 AND om.user_id = s.user_id
		  )`
	cmd, err := r.db.Exec(ctx, q, sessionRef, organizationID)
	if err != nil {
		return fmt.Errorf("security: RevokeOrganizationSession: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrSessionNotFound
	}
	return nil
}

func (r *repoImpl) ListOrganizationLoginEvents(ctx context.Context, organizationID string, limit int) ([]*LoginEventView, error) {
	if limit <= 0 || limit > maxEventLimit {
		limit = defaultEventLimit
	}
	const q = `
		SELECT le.id, le.public_id, COALESCE(u.id::TEXT, ''), COALESCE(u.public_id, ''), COALESCE(le.email, COALESCE(u.email, '')),
		       le.provider, le.status, COALESCE(le.failure_reason, ''), COALESCE(le.ip_address::TEXT, ''), COALESCE(le.user_agent, ''),
		       COALESCE(le.country, ''), COALESCE(le.city, ''), COALESCE(le.region, ''), le.created_at
		FROM login_events le
		LEFT JOIN users u ON u.id = le.user_id
		WHERE EXISTS (
			SELECT 1 FROM organization_members om
			WHERE om.org_id = $1 AND om.user_id = le.user_id
		)
		ORDER BY le.created_at DESC
		LIMIT $2`
	rows, err := r.db.Query(ctx, q, organizationID, limit)
	if err != nil {
		return nil, fmt.Errorf("security: ListOrganizationLoginEvents: %w", err)
	}
	defer rows.Close()
	var events []*LoginEventView
	for rows.Next() {
		e := &LoginEventView{}
		if err := rows.Scan(&e.ID, &e.PublicID, &e.UserID, &e.UserPublicID, &e.Email, &e.Provider, &e.Status, &e.FailureReason, &e.IPAddress, &e.UserAgent, &e.Country, &e.City, &e.Region, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("security: ListOrganizationLoginEvents: scan: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}
