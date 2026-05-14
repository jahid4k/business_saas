package task

import "context"

// Repository defines the data access interface for task operations.
// All queries are scoped to business_id — this is how tenant isolation is enforced.
type Repository interface {
	FindAll(ctx context.Context, businessID string) ([]*Task, error)
	FindByID(ctx context.Context, businessID, taskID string) (*Task, error)
	Create(ctx context.Context, t *Task) error
	Update(ctx context.Context, t *Task) error
	Delete(ctx context.Context, businessID, taskID string) error
	Count(ctx context.Context, businessID string) (int, error)
}

type repoImpl struct {
	// TODO (Phase 1-E): pgxpool.Pool
}

// NewRepository creates a new task repository.
func NewRepository() Repository {
	return &repoImpl{}
}

func (r *repoImpl) FindAll(_ context.Context, _ string) ([]*Task, error)   { return nil, nil }
func (r *repoImpl) FindByID(_ context.Context, _, _ string) (*Task, error) { return nil, nil }
func (r *repoImpl) Create(_ context.Context, _ *Task) error                { return nil }
func (r *repoImpl) Update(_ context.Context, _ *Task) error                { return nil }
func (r *repoImpl) Delete(_ context.Context, _, _ string) error            { return nil }
func (r *repoImpl) Count(_ context.Context, _ string) (int, error)         { return 0, nil }
