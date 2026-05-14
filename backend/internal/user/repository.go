package user

import (
	"context"
)

// Repository defines the data access interface for user operations.
type Repository interface {
	FindByID(ctx context.Context, userID string) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	Create(ctx context.Context, u *User) error
	Update(ctx context.Context, u *User) error
}

type repoImpl struct {
	// TODO (Phase 1-B): pgxpool.Pool
}

// NewRepository creates a new user repository.
func NewRepository() Repository {
	return &repoImpl{}
}

func (r *repoImpl) FindByID(_ context.Context, _ string) (*User, error) {
	return nil, nil // TODO Phase 1-B
}

func (r *repoImpl) FindByEmail(_ context.Context, _ string) (*User, error) {
	return nil, nil // TODO Phase 1-B
}

func (r *repoImpl) Create(_ context.Context, _ *User) error {
	return nil // TODO Phase 1-B
}

func (r *repoImpl) Update(_ context.Context, _ *User) error {
	return nil // TODO Phase 1-B
}
