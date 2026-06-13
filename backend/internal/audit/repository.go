// backend/internal/audit/repository.go
package audit

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type repoImpl struct{ db *pgxpool.Pool }

// NewRepository returns a production audit repository that writes to the audit_logs table.
// FIX: replaces the no-op placeholder — audit events are now actually persisted.
func NewRepository(db *pgxpool.Pool) Repository {
	return &repoImpl{db: db}
}

func (r *repoImpl) Insert(ctx context.Context, e *Event) error {
	const q = `
		INSERT INTO audit_logs (user_id, business_id, event_type, metadata, ip_address, user_agent)
		VALUES ($1, $2, $3, $4, NULLIF($5, '')::INET, NULLIF($6, ''))`
	_, err := r.db.Exec(ctx, q, e.UserID, e.BusinessID, string(e.EventType), e.Metadata, e.IPAddress, e.UserAgent)
	if err != nil {
		return fmt.Errorf("audit: Insert: %w", err)
	}
	return nil
}

// noopRepo is used for testing only.
type noopRepo struct{}

func (n *noopRepo) Insert(_ context.Context, _ *Event) error { return nil }

// NewNoopRepository returns a no-op audit repository for tests.
func NewNoopRepository() Repository { return &noopRepo{} }
