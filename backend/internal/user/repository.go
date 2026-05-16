// backend/internal/user/repository.go
package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines the data access interface for user operations.
type Repository interface {
	FindByID(ctx context.Context, userID string) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	Create(ctx context.Context, u *User) error
	Update(ctx context.Context, u *User) error
}

type repoImpl struct {
	db *pgxpool.Pool
}

// NewRepository creates a new user repository backed by a pgxpool.
func NewRepository(db *pgxpool.Pool) Repository {
	return &repoImpl{db: db}
}

// FindByID looks up a user by UUID. Returns nil, nil when not found.
func (r *repoImpl) FindByID(ctx context.Context, userID string) (*User, error) {
	const q = `
		SELECT id, email, password_hash, first_name, last_name,
		       is_verified, is_active, failed_logins, locked_until,
		       created_at, updated_at
		FROM users
		WHERE id = $1`

	u := &User{}
	err := r.db.QueryRow(ctx, q, userID).Scan(
		&u.ID, &u.Email, &u.PasswordHash,
		&u.FirstName, &u.LastName,
		&u.IsVerified, &u.IsActive,
		&u.FailedLogins, &u.LockedUntil,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("user: FindByID: %w", err)
	}
	return u, nil
}

// FindByEmail looks up a user by normalised email. Returns nil, nil when not found.
func (r *repoImpl) FindByEmail(ctx context.Context, email string) (*User, error) {
	const q = `
		SELECT id, email, password_hash, first_name, last_name,
		       is_verified, is_active, failed_logins, locked_until,
		       created_at, updated_at
		FROM users
		WHERE LOWER(email) = LOWER($1)`

	u := &User{}
	err := r.db.QueryRow(ctx, q, email).Scan(
		&u.ID, &u.Email, &u.PasswordHash,
		&u.FirstName, &u.LastName,
		&u.IsVerified, &u.IsActive,
		&u.FailedLogins, &u.LockedUntil,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("user: FindByEmail: %w", err)
	}
	return u, nil
}

// Create inserts a new user and populates ID, CreatedAt, UpdatedAt.
func (r *repoImpl) Create(ctx context.Context, u *User) error {
	const q = `
		INSERT INTO users (email, password_hash, first_name, last_name, is_verified, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at`

	err := r.db.QueryRow(ctx, q,
		u.Email, u.PasswordHash,
		u.FirstName, u.LastName,
		u.IsVerified, u.IsActive,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return fmt.Errorf("user: Create: %w", err)
	}
	return nil
}

// Update saves mutable fields for an existing user.
func (r *repoImpl) Update(ctx context.Context, u *User) error {
	const q = `
		UPDATE users
		SET password_hash = $1,
		    first_name    = $2,
		    last_name     = $3,
		    is_verified   = $4,
		    is_active     = $5,
		    failed_logins = $6,
		    locked_until  = $7,
		    updated_at    = NOW()
		WHERE id = $8
		RETURNING updated_at`

	err := r.db.QueryRow(ctx, q,
		u.PasswordHash, u.FirstName, u.LastName,
		u.IsVerified, u.IsActive,
		u.FailedLogins, u.LockedUntil,
		u.ID,
	).Scan(&u.UpdatedAt)
	if err != nil {
		return fmt.Errorf("user: Update: %w", err)
	}
	return nil
}
