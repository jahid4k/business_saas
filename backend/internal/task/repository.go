// backend/internal/task/repository.go
package task

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines the data access interface for task operations.
//
// TENANT ISOLATION RULE: every query MUST include org_id in the WHERE clause.
// FindByRef returns nil, nil for both "does not exist" and "exists in a
// different organization" — callers must not be able to distinguish the two.
type Repository interface {
	FindAll(ctx context.Context, orgID string, filter ListFilter) ([]*Task, error)
	Count(ctx context.Context, orgID string, filter ListFilter) (int, error)
	FindByRef(ctx context.Context, orgID, taskRef string) (*Task, error)
	Create(ctx context.Context, t *Task) error
	Update(ctx context.Context, t *Task) error
	Delete(ctx context.Context, orgID, taskRef string) error

	// ResolveOrgMember resolves a user reference (UUID, public_id, or email) to
	// that user's internal UUID, but only if they are an active member of orgID.
	// Returns ErrAssigneeNotFound if no active member matches.
	ResolveOrgMember(ctx context.Context, orgID, userRef string) (string, error)
}

type repoImpl struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repoImpl{db: db}
}

const taskSelect = `id, public_id, org_id, title, description, status, due_date, created_by, assigned_to, created_at, updated_at`

func scanTask(row pgx.Row) (*Task, error) {
	t := &Task{}
	err := row.Scan(&t.ID, &t.PublicID, &t.OrgID, &t.Title, &t.Description, &t.Status, &t.DueDate, &t.CreatedBy, &t.AssignedTo, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return t, nil
}

// sortColumn maps a validated SortField to its SQL column. ListFilter.Normalise
// guarantees SortBy is always one of these keys before reaching here.
var sortColumn = map[SortField]string{
	SortByCreatedAt: "created_at",
	SortByUpdatedAt: "updated_at",
	SortByDueDate:   "due_date",
	SortByTitle:     "title",
	SortByStatus:    "status",
}

// buildListWhere returns the WHERE clause (without "WHERE") and its args for
// FindAll/Count, scoped by org_id plus any optional filter fields.
func buildListWhere(orgID string, filter ListFilter) (string, []any) {
	clauses := []string{"org_id = $1"}
	args := []any{orgID}

	if filter.Status != "" {
		args = append(args, filter.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if filter.AssignedTo != "" {
		args = append(args, filter.AssignedTo)
		clauses = append(clauses, fmt.Sprintf("assigned_to = $%d", len(args)))
	}
	return strings.Join(clauses, " AND "), args
}

// FindAll returns tasks for an organization, filtered/sorted/paginated per
// filter. filter must already be normalised (see ListFilter.Normalise).
func (r *repoImpl) FindAll(ctx context.Context, orgID string, filter ListFilter) ([]*Task, error) {
	where, args := buildListWhere(orgID, filter)

	dir := "DESC"
	if !filter.SortDesc {
		dir = "ASC"
	}
	col := sortColumn[filter.SortBy]

	args = append(args, filter.Limit, filter.Offset)
	q := fmt.Sprintf(`
		SELECT %s
		FROM tasks
		WHERE %s
		ORDER BY %s %s, id DESC
		LIMIT $%d OFFSET $%d`,
		taskSelect, where, col, dir, len(args)-1, len(args),
	)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("task: FindAll: %w", err)
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("task: FindAll: scan: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// Count returns the number of tasks matching filter (ignoring Limit/Offset/Sort)
// — used to populate TaskListResponse.Total for pagination.
func (r *repoImpl) Count(ctx context.Context, orgID string, filter ListFilter) (int, error) {
	where, args := buildListWhere(orgID, filter)
	q := fmt.Sprintf(`SELECT COUNT(*) FROM tasks WHERE %s`, where)

	var count int
	if err := r.db.QueryRow(ctx, q, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("task: Count: %w", err)
	}
	return count, nil
}

// FindByRef returns a task by internal UUID or public_id, scoped to orgID.
// Returns nil, nil if not found OR if it belongs to a different
// organization — intentional, see TENANT ISOLATION RULE above.
func (r *repoImpl) FindByRef(ctx context.Context, orgID, taskRef string) (*Task, error) {
	q := `SELECT ` + taskSelect + `
		FROM tasks
		WHERE org_id = $1 AND (id::TEXT = $2 OR public_id = $2)`
	t, err := scanTask(r.db.QueryRow(ctx, q, orgID, strings.TrimSpace(taskRef)))
	if err != nil {
		return nil, fmt.Errorf("task: FindByRef: %w", err)
	}
	return t, nil
}

// Create inserts a new task row and populates generated fields.
func (r *repoImpl) Create(ctx context.Context, t *Task) error {
	const q = `
		INSERT INTO tasks (org_id, title, description, status, due_date, created_by, assigned_to)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING ` + taskSelect

	created, err := scanTask(r.db.QueryRow(ctx, q, t.OrgID, t.Title, t.Description, t.Status, t.DueDate, t.CreatedBy, t.AssignedTo))
	if err != nil {
		return fmt.Errorf("task: Create: %w", err)
	}
	*t = *created
	return nil
}

// Update saves changes to an existing task, identified by t.ID (the service
// has already resolved a ref via FindByRef). The org_id clause is defence in
// depth: a mismatched org_id here means 0 rows returned -> ErrNotFound, never
// a cross-tenant write.
func (r *repoImpl) Update(ctx context.Context, t *Task) error {
	const q = `
		UPDATE tasks
		SET title = $1, description = $2, status = $3, due_date = $4, assigned_to = $5, updated_at = NOW()
		WHERE id = $6 AND org_id = $7
		RETURNING ` + taskSelect

	updated, err := scanTask(r.db.QueryRow(ctx, q, t.Title, t.Description, t.Status, t.DueDate, t.AssignedTo, t.ID, t.OrgID))
	if err != nil {
		return fmt.Errorf("task: Update: %w", err)
	}
	if updated == nil {
		return ErrNotFound
	}
	*t = *updated
	return nil
}

// Delete removes a task by ref, scoped to orgID. Returns ErrNotFound if no
// row matched (does not exist, or belongs to a different organization).
func (r *repoImpl) Delete(ctx context.Context, orgID, taskRef string) error {
	const q = `DELETE FROM tasks WHERE org_id = $1 AND (id::TEXT = $2 OR public_id = $2)`
	cmd, err := r.db.Exec(ctx, q, orgID, strings.TrimSpace(taskRef))
	if err != nil {
		return fmt.Errorf("task: Delete: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ResolveOrgMember resolves userRef (internal UUID, public_id, or email) to an
// active member's user UUID within orgID. Mirrors authz.GetMemberByRef's
// ref-matching so the same identifier formats work everywhere in the API.
func (r *repoImpl) ResolveOrgMember(ctx context.Context, orgID, userRef string) (string, error) {
	const q = `
		SELECT u.id::TEXT
		FROM users u
		JOIN organization_members om ON om.user_id = u.id
		WHERE om.org_id = $1 AND om.status = 'active'
		  AND (u.id::TEXT = $2 OR u.public_id = $2 OR LOWER(u.email) = LOWER($2))`
	var userID string
	err := r.db.QueryRow(ctx, q, orgID, strings.TrimSpace(userRef)).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrAssigneeNotFound
	}
	if err != nil {
		return "", fmt.Errorf("task: ResolveOrgMember: %w", err)
	}
	return userID, nil
}
