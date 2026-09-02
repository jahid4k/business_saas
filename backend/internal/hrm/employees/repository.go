// backend/internal/hrm/employees/repository.go
package employees

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/scope"
)

// Repository defines data access for HRM employees.
//
// TENANT ISOLATION RULE: every query MUST include org_id in the WHERE clause.
// FindByRef returns nil, nil for "not found" OR "wrong org" — callers must
// not be able to distinguish the two cases.
type Repository interface {
	FindAll(ctx context.Context, orgID string, filter ListFilter) ([]*Employee, error)
	Count(ctx context.Context, orgID string, filter ListFilter) (int, error)
	FindByRef(ctx context.Context, orgID, ref string) (*Employee, error)
	Create(ctx context.Context, e *Employee) error
	// CreateTx inserts an employee inside an existing transaction — used by
	// recruitment.HireApplication to keep the employee insert, the
	// application's converted_employee_id, and the requisition's
	// filled_count atomic. The interface accepts pgx.Tx, not pgxpool.Pool,
	// so the caller owns the transaction (the contacts.CreateContactTx
	// precedent).
	CreateTx(ctx context.Context, tx pgx.Tx, e *Employee) error
	Update(ctx context.Context, e *Employee) error
	Delete(ctx context.Context, orgID, ref string) error
	GetDefaultStatusID(ctx context.Context, orgID string, category EmployeeStatusCategory) (string, error)
	ListStatuses(ctx context.Context, orgID string) ([]*EmployeeStatusModel, error)
	CreateStatus(ctx context.Context, s *EmployeeStatusModel) error
	UpdateStatus(ctx context.Context, s *EmployeeStatusModel) error
	DeleteStatus(ctx context.Context, orgID, statusID string) error

	// ExistsByEmployeeNumber returns true when another active employee already
	// holds the same employee_number within the org, ignoring excludeID.
	ExistsByEmployeeNumber(ctx context.Context, orgID, number, excludeID string) (bool, error)
}

type repoImpl struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repoImpl{db: db}
}

const empSelect = `
	id, public_id, org_id, user_id, employee_number,
	first_name, last_name, email, work_email, phone, work_phone,
	date_of_birth, gender, avatar_url,
	hire_date, termination_date, employment_type, status_id,
	department_id, position_id, manager_id,
	address, city, country, notes, source_candidate_id,
	created_by, created_at, updated_at`

func scanEmployee(row pgx.Row) (*Employee, error) {
	e := &Employee{}
	err := row.Scan(
		&e.ID, &e.PublicID, &e.OrgID, &e.UserID, &e.EmployeeNumber,
		&e.FirstName, &e.LastName, &e.Email, &e.WorkEmail, &e.Phone, &e.WorkPhone,
		&e.DateOfBirth, &e.Gender, &e.AvatarURL,
		&e.HireDate, &e.TerminationDate, &e.EmploymentType, &e.StatusID,
		&e.DepartmentID, &e.PositionID, &e.ManagerID,
		&e.Address, &e.City, &e.Country, &e.Notes, &e.SourceCandidateID,
		&e.CreatedBy, &e.CreatedAt, &e.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return e, nil
}

// buildListWhere builds the WHERE clause and args for FindAll/Count.
func buildListWhere(orgID string, filter ListFilter) (string, []any) {
	clauses := []string{"org_id = $1"}
	args := []any{orgID}

	if filter.StatusID != "" {
		args = append(args, filter.StatusID)
		clauses = append(clauses, fmt.Sprintf("status_id = $%d", len(args)))
	}
	if filter.EmploymentType != "" {
		args = append(args, string(filter.EmploymentType))
		clauses = append(clauses, fmt.Sprintf("employment_type = $%d", len(args)))
	}
	if filter.DepartmentID != "" {
		args = append(args, filter.DepartmentID)
		clauses = append(clauses, fmt.Sprintf("department_id::TEXT = $%d", len(args)))
	}
	if filter.ManagerID != "" {
		args = append(args, filter.ManagerID)
		clauses = append(clauses, fmt.Sprintf("manager_id::TEXT = $%d", len(args)))
	}
	if filter.Search != "" {
		// Case-insensitive substring match on name, email, and employee_number.
		search := "%" + strings.ToLower(strings.TrimSpace(filter.Search)) + "%"
		args = append(args, search)
		n := len(args)
		clauses = append(clauses, fmt.Sprintf(
			`(LOWER(first_name) LIKE $%d OR LOWER(COALESCE(last_name,'')) LIKE $%d
			  OR LOWER(COALESCE(email,'')) LIKE $%d OR LOWER(COALESCE(employee_number,'')) LIKE $%d)`,
			n, n, n, n,
		))
	}
	if filter.Scope != authz.ScopeAll {
		frag, scopeArgs := scope.Predicate(filter.Scope, "id", len(args), orgID, filter.CallerUserID, scope.DefaultMaxDepth)
		clauses = append(clauses, frag)
		args = append(args, scopeArgs...)
	}
	return strings.Join(clauses, " AND "), args
}

func (r *repoImpl) FindAll(ctx context.Context, orgID string, filter ListFilter) ([]*Employee, error) {
	where, args := buildListWhere(orgID, filter)
	args = append(args, filter.Limit, filter.Offset)
	q := fmt.Sprintf(`
		SELECT %s FROM hrm_employees
		WHERE %s
		ORDER BY first_name ASC, last_name ASC, id DESC
		LIMIT $%d OFFSET $%d`,
		empSelect, where, len(args)-1, len(args),
	)
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("employees: FindAll: %w", err)
	}
	defer rows.Close()

	var list []*Employee
	for rows.Next() {
		e, err := scanEmployee(rows)
		if err != nil {
			return nil, fmt.Errorf("employees: FindAll: scan: %w", err)
		}
		list = append(list, e)
	}
	return list, rows.Err()
}

func (r *repoImpl) Count(ctx context.Context, orgID string, filter ListFilter) (int, error) {
	where, args := buildListWhere(orgID, filter)
	var count int
	if err := r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM hrm_employees WHERE %s`, where), args...,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("employees: Count: %w", err)
	}
	return count, nil
}

func (r *repoImpl) FindByRef(ctx context.Context, orgID, ref string) (*Employee, error) {
	q := `SELECT ` + empSelect + `
		FROM hrm_employees
		WHERE org_id = $1 AND (id::TEXT = $2 OR public_id = $2)`
	e, err := scanEmployee(r.db.QueryRow(ctx, q, orgID, strings.TrimSpace(ref)))
	if err != nil {
		return nil, fmt.Errorf("employees: FindByRef: %w", err)
	}
	return e, nil
}

func (r *repoImpl) GetDefaultStatusID(ctx context.Context, orgID string, category EmployeeStatusCategory) (string, error) {
	var id string
	q := `SELECT id FROM hrm_employee_statuses WHERE org_id = $1 AND category = $2 ORDER BY created_at ASC LIMIT 1`
	if err := r.db.QueryRow(ctx, q, orgID, string(category)).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (r *repoImpl) ListStatuses(ctx context.Context, orgID string) ([]*EmployeeStatusModel, error) {
	q := `SELECT id, org_id, name, category, color, created_at, updated_at 
		  FROM hrm_employee_statuses 
		  WHERE org_id = $1 
		  ORDER BY created_at ASC`
	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("statuses: ListStatuses: %w", err)
	}
	defer rows.Close()

	var list []*EmployeeStatusModel
	for rows.Next() {
		s := &EmployeeStatusModel{}
		if err := rows.Scan(&s.ID, &s.OrgID, &s.Name, &s.Category, &s.Color, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

func (r *repoImpl) CreateStatus(ctx context.Context, s *EmployeeStatusModel) error {
	const q = `
		INSERT INTO hrm_employee_statuses (org_id, name, category, color)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at`
	return r.db.QueryRow(ctx, q, s.OrgID, s.Name, string(s.Category), s.Color).
		Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
}

func (r *repoImpl) UpdateStatus(ctx context.Context, s *EmployeeStatusModel) error {
	const q = `
		UPDATE hrm_employee_statuses
		SET name = $1, category = $2, color = $3, updated_at = NOW()
		WHERE id = $4 AND org_id = $5
		RETURNING updated_at`
	return r.db.QueryRow(ctx, q, s.Name, string(s.Category), s.Color, s.ID, s.OrgID).
		Scan(&s.UpdatedAt)
}

func (r *repoImpl) DeleteStatus(ctx context.Context, orgID, statusID string) error {
	const q = `DELETE FROM hrm_employee_statuses WHERE id = $1 AND org_id = $2`
	res, err := r.db.Exec(ctx, q, statusID, orgID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// insertEmployeeSQL is shared by Create and CreateTx — the contacts
// insertContactSQL precedent, so the two paths can never drift out of sync.
const insertEmployeeSQL = `
	INSERT INTO hrm_employees (
		org_id, user_id, employee_number,
		first_name, last_name, email, work_email, phone, work_phone,
		date_of_birth, gender, avatar_url,
		hire_date, employment_type, status_id,
		department_id, position_id, manager_id,
		address, city, country, notes, source_candidate_id, created_by
	) VALUES (
		$1, $2, $3,
		$4, $5, $6, $7, $8, $9,
		$10, $11, $12,
		$13, $14, $15,
		$16, $17, $18,
		$19, $20, $21, $22, $23, $24
	) RETURNING ` + empSelect

func employeeInsertArgs(e *Employee) []any {
	return []any{
		e.OrgID, e.UserID, e.EmployeeNumber,
		e.FirstName, e.LastName, e.Email, e.WorkEmail, e.Phone, e.WorkPhone,
		e.DateOfBirth, e.Gender, e.AvatarURL,
		e.HireDate, e.EmploymentType, e.StatusID,
		e.DepartmentID, e.PositionID, e.ManagerID,
		e.Address, e.City, e.Country, e.Notes, e.SourceCandidateID, e.CreatedBy,
	}
}

func (r *repoImpl) Create(ctx context.Context, e *Employee) error {
	created, err := scanEmployee(r.db.QueryRow(ctx, insertEmployeeSQL, employeeInsertArgs(e)...))
	if err != nil {
		return fmt.Errorf("employees: Create: %w", err)
	}
	*e = *created
	return nil
}

// CreateTx inserts an employee within an existing pgx.Tx. The caller is
// responsible for committing or rolling back the transaction.
func (r *repoImpl) CreateTx(ctx context.Context, tx pgx.Tx, e *Employee) error {
	created, err := scanEmployee(tx.QueryRow(ctx, insertEmployeeSQL, employeeInsertArgs(e)...))
	if err != nil {
		return fmt.Errorf("employees: CreateTx: %w", err)
	}
	*e = *created
	return nil
}

func (r *repoImpl) Update(ctx context.Context, e *Employee) error {
	const q = `
		UPDATE hrm_employees
		SET user_id = $1, employee_number = $2,
		    first_name = $3, last_name = $4, email = $5, work_email = $6,
		    phone = $7, work_phone = $8,
		    date_of_birth = $9, gender = $10, avatar_url = $11,
		    hire_date = $12, termination_date = $13, employment_type = $14, status_id = $15,
		    department_id = $16, position_id = $17, manager_id = $18,
		    address = $19, city = $20, country = $21, notes = $22,
		    updated_at = NOW()
		WHERE id = $23 AND org_id = $24
		RETURNING ` + empSelect

	updated, err := scanEmployee(r.db.QueryRow(ctx, q,
		e.UserID, e.EmployeeNumber,
		e.FirstName, e.LastName, e.Email, e.WorkEmail,
		e.Phone, e.WorkPhone,
		e.DateOfBirth, e.Gender, e.AvatarURL,
		e.HireDate, e.TerminationDate, e.EmploymentType, e.StatusID,
		e.DepartmentID, e.PositionID, e.ManagerID,
		e.Address, e.City, e.Country, e.Notes,
		e.ID, e.OrgID,
	))
	if err != nil {
		return fmt.Errorf("employees: Update: %w", err)
	}
	if updated == nil {
		return ErrEmployeeNotFound
	}
	*e = *updated
	return nil
}

func (r *repoImpl) Delete(ctx context.Context, orgID, ref string) error {
	cmd, err := r.db.Exec(ctx,
		`DELETE FROM hrm_employees WHERE org_id = $1 AND (id::TEXT = $2 OR public_id = $2)`,
		orgID, strings.TrimSpace(ref),
	)
	if err != nil {
		return fmt.Errorf("employees: Delete: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrEmployeeNotFound
	}
	return nil
}

func (r *repoImpl) ExistsByEmployeeNumber(ctx context.Context, orgID, number, excludeID string) (bool, error) {
	q := `SELECT EXISTS(
		SELECT 1 FROM hrm_employees
		WHERE org_id = $1 AND LOWER(employee_number) = LOWER($2) AND id::TEXT != $3
	)`
	var exists bool
	if err := r.db.QueryRow(ctx, q, orgID, strings.TrimSpace(number), excludeID).Scan(&exists); err != nil {
		return false, fmt.Errorf("employees: ExistsByEmployeeNumber: %w", err)
	}
	return exists, nil
}
