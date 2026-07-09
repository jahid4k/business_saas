// backend/internal/hrm/positions/repository.go
package positions

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines data access for HRM positions.
//
// TENANT ISOLATION RULE: every query MUST include org_id in the WHERE clause.
type Repository interface {
	FindAll(ctx context.Context, orgID string, departmentID string, activeOnly bool) ([]*Position, error)
	Count(ctx context.Context, orgID string, departmentID string, activeOnly bool) (int, error)
	FindByRef(ctx context.Context, orgID, ref string) (*Position, error)
	Create(ctx context.Context, p *Position) error
	Update(ctx context.Context, p *Position) error
	Delete(ctx context.Context, orgID, ref string) error
	ExistsByTitle(ctx context.Context, orgID, title, excludeID string) (bool, error)
}

type repoImpl struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repoImpl{db: db}
}

const posSelect = `id, public_id, org_id, department_id, title, description, is_active, created_by, created_at, updated_at`

func scanPosition(row pgx.Row) (*Position, error) {
	p := &Position{}
	err := row.Scan(
		&p.ID, &p.PublicID, &p.OrgID, &p.DepartmentID,
		&p.Title, &p.Description, &p.IsActive,
		&p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *repoImpl) FindAll(ctx context.Context, orgID, departmentID string, activeOnly bool) ([]*Position, error) {
	clauses := []string{"org_id = $1"}
	args := []any{orgID}
	if departmentID != "" {
		args = append(args, departmentID)
		clauses = append(clauses, fmt.Sprintf("department_id::TEXT = $%d", len(args)))
	}
	if activeOnly {
		clauses = append(clauses, "is_active = TRUE")
	}
	where := strings.Join(clauses, " AND ")
	q := fmt.Sprintf(`SELECT %s FROM hrm_positions WHERE %s ORDER BY title ASC`, posSelect, where)
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("positions: FindAll: %w", err)
	}
	defer rows.Close()

	var list []*Position
	for rows.Next() {
		p, err := scanPosition(rows)
		if err != nil {
			return nil, fmt.Errorf("positions: FindAll: scan: %w", err)
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (r *repoImpl) Count(ctx context.Context, orgID, departmentID string, activeOnly bool) (int, error) {
	clauses := []string{"org_id = $1"}
	args := []any{orgID}
	if departmentID != "" {
		args = append(args, departmentID)
		clauses = append(clauses, fmt.Sprintf("department_id::TEXT = $%d", len(args)))
	}
	if activeOnly {
		clauses = append(clauses, "is_active = TRUE")
	}
	var count int
	if err := r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM hrm_positions WHERE %s`, strings.Join(clauses, " AND ")),
		args...,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("positions: Count: %w", err)
	}
	return count, nil
}

func (r *repoImpl) FindByRef(ctx context.Context, orgID, ref string) (*Position, error) {
	q := `SELECT ` + posSelect + `
		FROM hrm_positions
		WHERE org_id = $1 AND (id::TEXT = $2 OR public_id = $2)`
	p, err := scanPosition(r.db.QueryRow(ctx, q, orgID, strings.TrimSpace(ref)))
	if err != nil {
		return nil, fmt.Errorf("positions: FindByRef: %w", err)
	}
	return p, nil
}

func (r *repoImpl) Create(ctx context.Context, p *Position) error {
	const q = `
		INSERT INTO hrm_positions (org_id, department_id, title, description, is_active, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING ` + posSelect

	created, err := scanPosition(r.db.QueryRow(ctx, q,
		p.OrgID, p.DepartmentID, p.Title, p.Description, p.IsActive, p.CreatedBy,
	))
	if err != nil {
		return fmt.Errorf("positions: Create: %w", err)
	}
	*p = *created
	return nil
}

func (r *repoImpl) Update(ctx context.Context, p *Position) error {
	const q = `
		UPDATE hrm_positions
		SET department_id = $1, title = $2, description = $3, is_active = $4, updated_at = NOW()
		WHERE id = $5 AND org_id = $6
		RETURNING ` + posSelect

	updated, err := scanPosition(r.db.QueryRow(ctx, q,
		p.DepartmentID, p.Title, p.Description, p.IsActive, p.ID, p.OrgID,
	))
	if err != nil {
		return fmt.Errorf("positions: Update: %w", err)
	}
	if updated == nil {
		return ErrPositionNotFound
	}
	*p = *updated
	return nil
}

func (r *repoImpl) Delete(ctx context.Context, orgID, ref string) error {
	cmd, err := r.db.Exec(ctx,
		`DELETE FROM hrm_positions WHERE org_id = $1 AND (id::TEXT = $2 OR public_id = $2)`,
		orgID, strings.TrimSpace(ref),
	)
	if err != nil {
		return fmt.Errorf("positions: Delete: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrPositionNotFound
	}
	return nil
}

func (r *repoImpl) ExistsByTitle(ctx context.Context, orgID, title, excludeID string) (bool, error) {
	q := `SELECT EXISTS(
		SELECT 1 FROM hrm_positions
		WHERE org_id = $1 AND LOWER(title) = LOWER($2) AND is_active = TRUE AND id::TEXT != $3
	)`
	var exists bool
	if err := r.db.QueryRow(ctx, q, orgID, strings.TrimSpace(title), excludeID).Scan(&exists); err != nil {
		return false, fmt.Errorf("positions: ExistsByTitle: %w", err)
	}
	return exists, nil
}
