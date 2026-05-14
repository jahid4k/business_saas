package business

import "context"

// Repository defines the data access interface for business operations.
type Repository interface {
	Create(ctx context.Context, b *Business) error
	FindByID(ctx context.Context, businessID string) (*Business, error)
	FindBySlug(ctx context.Context, slug string) (*Business, error)
	FindByUserID(ctx context.Context, userID string) ([]*Business, error)
}

type repoImpl struct {
	// TODO (Phase 1-C): pgxpool.Pool
}

// NewRepository creates a new business repository.
func NewRepository() Repository {
	return &repoImpl{}
}

func (r *repoImpl) Create(_ context.Context, _ *Business) error                   { return nil }
func (r *repoImpl) FindByID(_ context.Context, _ string) (*Business, error)       { return nil, nil }
func (r *repoImpl) FindBySlug(_ context.Context, _ string) (*Business, error)     { return nil, nil }
func (r *repoImpl) FindByUserID(_ context.Context, _ string) ([]*Business, error) { return nil, nil }
