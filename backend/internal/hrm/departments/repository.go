// backend/internal/hrm/departments/repository.go
package departments

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines data access for HRM departments.
//
// TENANT ISOLATION RULE: every query MUST include org_id in the WHERE clause.
// FindByRef returns nil, nil for both "not found" and "belongs to another org".
type Repository interface {
	FindAll(ctx context.Context, orgID string, activeOnly bool) ([]*Department, error)
	Count(ctx context.Context, orgID string, activeOnly bool) (int, error)
	FindByRef(ctx context.Context, orgID, ref string) (*Department, error)
	Create(ctx context.Context, d *Department) error
	Update(ctx context.Context, d *Department) error
	Delete(ctx context.Context, orgID, ref string) error

	// ExistsByName returns true when an active department with the given name
	// (case-insensitive) already exists in the org, ignoring excludeID.
	ExistsByName(ctx context.Context, orgID, name, excludeID string) (bool, error)
}

type repoImpl struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repoImpl{db: db}
}

const deptSelect = `id, public_id, org_id, name, description, parent_department_id, head_employee_id, is_active, created_by, created_at, updated_at`

func scanDepartment(row pgx.Row) (*Department, error) {
	d := &Department{}
	err := row.Scan(
		&d.ID, &d.PublicID, &d.OrgID, &d.Name, &d.Description,
		&d.ParentDepartmentID, &d.HeadEmployeeID, &d.IsActive,
		&d.CreatedBy, &d.CreatedAt, &d.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (r *repoImpl) FindAll(ctx context.Context, orgID string, activeOnly bool) ([]*Department, error) {
	where := "org_id = $1"
	args := []any{orgID}
	if activeOnly {
		where += " AND is_active = TRUE"
	}
	q := fmt.Sprintf(`SELECT %s FROM hrm_departments WHERE %s ORDER BY name ASC`, deptSelect, where)
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("departments: FindAll: %w", err)
	}
	defer rows.Close()

	var list []*Department
	for rows.Next() {
		d, err := scanDepartment(rows)
		if err != nil {
			return nil, fmt.Errorf("departments: FindAll: scan: %w", err)
		}
		list = append(list, d)
	}
	return list, rows.Err()
}

func (r *repoImpl) Count(ctx context.Context, orgID string, activeOnly bool) (int, error) {
	where := "org_id = $1"
	args := []any{orgID}
	if activeOnly {
		where += " AND is_active = TRUE"
	}
	var count int
	if err := r.db.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM hrm_departments WHERE %s`, where), args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("departments: Count: %w", err)
	}
	return count, nil
}

func (r *repoImpl) FindByRef(ctx context.Context, orgID, ref string) (*Department, error) {
	q := `SELECT ` + deptSelect + `
		FROM hrm_departments
		WHERE org_id = $1 AND (id::TEXT = $2 OR public_id = $2)`
	d, err := scanDepartment(r.db.QueryRow(ctx, q, orgID, strings.TrimSpace(ref)))
	if err != nil {
		return nil, fmt.Errorf("departments: FindByRef: %w", err)
	}
	return d, nil
}

func (r *repoImpl) Create(ctx context.Context, d *Department) error {
	const q = `
		INSERT INTO hrm_departments
		    (org_id, name, description, parent_department_id, head_employee_id, is_active, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING ` + deptSelect

	created, err := scanDepartment(r.db.QueryRow(ctx, q,
		d.OrgID, d.Name, d.Description, d.ParentDepartmentID,
		d.HeadEmployeeID, d.IsActive, d.CreatedBy,
	))
	if err != nil {
		return fmt.Errorf("departments: Create: %w", err)
	}
	*d = *created
	return nil
}

func (r *repoImpl) Update(ctx context.Context, d *Department) error {
	const q = `
		UPDATE hrm_departments
		SET name = $1, description = $2, parent_department_id = $3,
		    head_employee_id = $4, is_active = $5, updated_at = NOW()
		WHERE id = $6 AND org_id = $7
		RETURNING ` + deptSelect

	updated, err := scanDepartment(r.db.QueryRow(ctx, q,
		d.Name, d.Description, d.ParentDepartmentID,
		d.HeadEmployeeID, d.IsActive, d.ID, d.OrgID,
	))
	if err != nil {
		return fmt.Errorf("departments: Update: %w", err)
	}
	if updated == nil {
		return ErrDepartmentNotFound
	}
	*d = *updated
	return nil
}

func (r *repoImpl) Delete(ctx context.Context, orgID, ref string) error {
	cmd, err := r.db.Exec(ctx,
		`DELETE FROM hrm_departments WHERE org_id = $1 AND (id::TEXT = $2 OR public_id = $2)`,
		orgID, strings.TrimSpace(ref),
	)
	if err != nil {
		return fmt.Errorf("departments: Delete: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrDepartmentNotFound
	}
	return nil
}

func (r *repoImpl) ExistsByName(ctx context.Context, orgID, name, excludeID string) (bool, error) {
	q := `SELECT EXISTS(
		SELECT 1 FROM hrm_departments
		WHERE org_id = $1 AND LOWER(name) = LOWER($2) AND is_active = TRUE AND id::TEXT != $3
	)`
	var exists bool
	if err := r.db.QueryRow(ctx, q, orgID, strings.TrimSpace(name), excludeID).Scan(&exists); err != nil {
		return false, fmt.Errorf("departments: ExistsByName: %w", err)
	}
	return exists, nil
}
