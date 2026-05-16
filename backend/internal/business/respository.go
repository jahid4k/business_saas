// backend/internal/business/repository.go
package business

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines the data access interface for business operations.
type Repository interface {
	// Create inserts a new business row inside an existing transaction.
	// The caller (service) owns the transaction and passes it in.
	CreateTx(ctx context.Context, tx pgx.Tx, b *Business) error

	// FindByID returns a business by its UUID. Returns nil, nil when not found.
	FindByID(ctx context.Context, businessID string) (*Business, error)

	// FindBySlug returns a business by its slug. Returns nil, nil when not found.
	FindBySlug(ctx context.Context, slug string) (*Business, error)

	// FindByUserID returns all businesses the user is a member of,
	// enriched with the user's role in each.
	FindByUserID(ctx context.Context, userID string) ([]*MembershipWithRole, error)

	// BeginTx starts a new database transaction.
	BeginTx(ctx context.Context) (pgx.Tx, error)
}

type repoImpl struct {
	db *pgxpool.Pool
}

// NewRepository creates a new business repository backed by a pgxpool.
func NewRepository(db *pgxpool.Pool) Repository {
	return &repoImpl{db: db}
}

// BeginTx starts a transaction. The caller must call tx.Commit() or tx.Rollback().
func (r *repoImpl) BeginTx(ctx context.Context) (pgx.Tx, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("business: BeginTx: %w", err)
	}
	return tx, nil
}

// CreateTx inserts a business row inside the given transaction.
// Populates ID, CreatedAt, UpdatedAt on the struct after insert.
func (r *repoImpl) CreateTx(ctx context.Context, tx pgx.Tx, b *Business) error {
	const q = `
		INSERT INTO businesses (name, slug, owner_id, is_active)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at`

	err := tx.QueryRow(ctx, q,
		b.Name, b.Slug, b.OwnerID, b.IsActive,
	).Scan(&b.ID, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return fmt.Errorf("business: CreateTx: %w", err)
	}
	return nil
}

// FindByID returns a business by UUID. Returns nil, nil when not found.
func (r *repoImpl) FindByID(ctx context.Context, businessID string) (*Business, error) {
	const q = `
		SELECT id, name, slug, owner_id, is_active, created_at, updated_at
		FROM businesses
		WHERE id = $1`

	b := &Business{}
	err := r.db.QueryRow(ctx, q, businessID).Scan(
		&b.ID, &b.Name, &b.Slug,
		&b.OwnerID, &b.IsActive,
		&b.CreatedAt, &b.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("business: FindByID: %w", err)
	}
	return b, nil
}

// FindBySlug returns a business by its slug. Returns nil, nil when not found.
func (r *repoImpl) FindBySlug(ctx context.Context, slug string) (*Business, error) {
	const q = `
		SELECT id, name, slug, owner_id, is_active, created_at, updated_at
		FROM businesses
		WHERE slug = $1`

	b := &Business{}
	err := r.db.QueryRow(ctx, q, slug).Scan(
		&b.ID, &b.Name, &b.Slug,
		&b.OwnerID, &b.IsActive,
		&b.CreatedAt, &b.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("business: FindBySlug: %w", err)
	}
	return b, nil
}

// FindByUserID returns all businesses the user belongs to,
// with the user's role name in each workspace.
//
// Query joins: memberships → businesses → roles
// Only returns active businesses.
func (r *repoImpl) FindByUserID(ctx context.Context, userID string) ([]*MembershipWithRole, error) {
	const q = `
		SELECT
			b.id, b.name, b.slug, b.owner_id, b.is_active, b.created_at, b.updated_at,
			r.name  AS role_name,
			m.id    AS membership_id
		FROM memberships m
		JOIN businesses b ON b.id = m.business_id
		JOIN roles      r ON r.id = m.role_id
		WHERE m.user_id   = $1
		  AND b.is_active = TRUE
		ORDER BY b.name ASC`

	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("business: FindByUserID: %w", err)
	}
	defer rows.Close()

	var results []*MembershipWithRole
	for rows.Next() {
		b := &Business{}
		mwr := &MembershipWithRole{Business: b}

		if err := rows.Scan(
			&b.ID, &b.Name, &b.Slug,
			&b.OwnerID, &b.IsActive,
			&b.CreatedAt, &b.UpdatedAt,
			&mwr.Role,
			&mwr.MemberID,
		); err != nil {
			return nil, fmt.Errorf("business: FindByUserID: scan: %w", err)
		}
		results = append(results, mwr)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("business: FindByUserID: rows: %w", err)
	}

	return results, nil
}
