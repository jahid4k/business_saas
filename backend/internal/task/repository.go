// backend/internal/task/repository.go
package task

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines the data access interface for task operations.
//
// TENANT ISOLATION RULE: Every query MUST include business_id in the WHERE
// clause. This is the application-level enforcement of multi-tenancy.
// A task that exists but belongs to a different business must return nil, nil
// — exactly the same as a task that does not exist at all.
// Never return a task to a caller whose business_id does not match.
type Repository interface {
	FindAll(ctx context.Context, businessID string) ([]*Task, error)
	FindByID(ctx context.Context, businessID, taskID string) (*Task, error)
	Create(ctx context.Context, t *Task) error
	Update(ctx context.Context, t *Task) error
	Delete(ctx context.Context, businessID, taskID string) error
	Count(ctx context.Context, businessID string) (int, error)
}

type repoImpl struct {
	db *pgxpool.Pool
}

// NewRepository creates a new task repository backed by a pgxpool.
func NewRepository(db *pgxpool.Pool) Repository {
	return &repoImpl{db: db}
}

// FindAll returns all tasks for a business, ordered by created_at DESC.
// Always filters by business_id — no cross-tenant leakage possible.
func (r *repoImpl) FindAll(ctx context.Context, businessID string) ([]*Task, error) {
	const q = `
		SELECT id, business_id, title, description, status,
		       created_by, created_at, updated_at
		FROM tasks
		WHERE business_id = $1
		ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, q, businessID)
	if err != nil {
		return nil, fmt.Errorf("task: FindAll: %w", err)
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		t := &Task{}
		if err := rows.Scan(
			&t.ID, &t.BusinessID, &t.Title, &t.Description,
			&t.Status, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("task: FindAll: scan: %w", err)
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("task: FindAll: rows: %w", err)
	}

	return tasks, nil
}

// FindByID returns a task only if it belongs to the given business.
// Returns nil, nil when not found OR when the task belongs to a different business.
// This is intentional — callers cannot distinguish "not found" from "wrong tenant".
func (r *repoImpl) FindByID(ctx context.Context, businessID, taskID string) (*Task, error) {
	const q = `
		SELECT id, business_id, title, description, status,
		       created_by, created_at, updated_at
		FROM tasks
		WHERE id          = $1
		  AND business_id = $2`

	t := &Task{}
	err := r.db.QueryRow(ctx, q, taskID, businessID).Scan(
		&t.ID, &t.BusinessID, &t.Title, &t.Description,
		&t.Status, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("task: FindByID: %w", err)
	}
	return t, nil
}

// Create inserts a new task row and populates ID, CreatedAt, UpdatedAt.
func (r *repoImpl) Create(ctx context.Context, t *Task) error {
	const q = `
		INSERT INTO tasks (business_id, title, description, status, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`

	err := r.db.QueryRow(ctx, q,
		t.BusinessID, t.Title, t.Description, t.Status, t.CreatedBy,
	).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return fmt.Errorf("task: Create: %w", err)
	}
	return nil
}

// Update saves changes to an existing task.
// The business_id clause prevents cross-tenant modification — if the task
// belongs to a different business the UPDATE affects 0 rows and we return nil
// (the service layer will have already verified ownership via FindByID).
func (r *repoImpl) Update(ctx context.Context, t *Task) error {
	const q = `
		UPDATE tasks
		SET title       = $1,
		    description = $2,
		    status      = $3,
		    updated_at  = NOW()
		WHERE id          = $4
		  AND business_id = $5
		RETURNING updated_at`

	err := r.db.QueryRow(ctx, q,
		t.Title, t.Description, t.Status,
		t.ID, t.BusinessID,
	).Scan(&t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("task: Update: task not found or tenant mismatch")
	}
	if err != nil {
		return fmt.Errorf("task: Update: %w", err)
	}
	return nil
}

// Delete removes a task. Both taskID and businessID must match.
// Returns nil even if no row was deleted — the service layer verifies existence
// via FindByID before calling Delete.
func (r *repoImpl) Delete(ctx context.Context, businessID, taskID string) error {
	const q = `
		DELETE FROM tasks
		WHERE id          = $1
		  AND business_id = $2`

	_, err := r.db.Exec(ctx, q, taskID, businessID)
	if err != nil {
		return fmt.Errorf("task: Delete: %w", err)
	}
	return nil
}

// Count returns the total number of tasks for a business.
// Used to populate the Total field in TaskListResponse.
func (r *repoImpl) Count(ctx context.Context, businessID string) (int, error) {
	const q = `SELECT COUNT(*) FROM tasks WHERE business_id = $1`

	var count int
	if err := r.db.QueryRow(ctx, q, businessID).Scan(&count); err != nil {
		return 0, fmt.Errorf("task: Count: %w", err)
	}
	return count, nil
}
