// backend/internal/hrm/transfers/repository.go
package transfers

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

type Repository interface {
	FindAll(ctx context.Context, orgID string, filter TransferListFilter) ([]*Transfer, error)
	Count(ctx context.Context, orgID string, filter TransferListFilter) (int, error)
	FindByRef(ctx context.Context, orgID, employeeID, ref string) (*Transfer, error)
	Create(ctx context.Context, t *Transfer) error
	Update(ctx context.Context, t *Transfer) error
	UpdateStatus(ctx context.Context, id string, status TransferStatus) error
	SetApprovalInstance(ctx context.Context, id, instanceID string, status TransferStatus) error
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

const sel = `id, public_id, org_id, employee_id, transfer_type,
	from_department_id, from_manager_employee_id, from_location,
	to_department_id, to_manager_employee_id, to_location,
	to_char(effective_date,'YYYY-MM-DD'), reason, notes,
	approval_instance_id, document_id, status,
	applied_at, applied_by, created_by, created_at, updated_at`

func scanTrf(row pgx.Row) (*Transfer, error) {
	t := &Transfer{}
	err := row.Scan(
		&t.ID, &t.PublicID, &t.OrgID, &t.EmployeeID, &t.TransferType,
		&t.FromDepartmentID, &t.FromManagerEmployeeID, &t.FromLocation,
		&t.ToDepartmentID, &t.ToManagerEmployeeID, &t.ToLocation,
		&t.EffectiveDate, &t.Reason, &t.Notes,
		&t.ApprovalInstanceID, &t.DocumentID, &t.Status,
		&t.AppliedAt, &t.AppliedBy, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return t, nil
}

func buildTransfersWhere(orgID string, filter TransferListFilter) (string, []any) {
	clauses := []string{"org_id = $1"}
	args := []any{orgID}
	if filter.EmployeeID != "" {
		args = append(args, filter.EmployeeID)
		clauses = append(clauses, fmt.Sprintf("employee_id = $%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if filter.Scope != authz.ScopeAll {
		frag, scopeArgs := scope.Predicate(filter.Scope, "employee_id", len(args), orgID, filter.CallerUserID, scope.DefaultMaxDepth)
		clauses = append(clauses, frag)
		args = append(args, scopeArgs...)
	}
	return strings.Join(clauses, " AND "), args
}

func (r *repoImpl) FindAll(ctx context.Context, orgID string, filter TransferListFilter) ([]*Transfer, error) {
	where, args := buildTransfersWhere(orgID, filter)
	args = append(args, filter.Limit, filter.Offset)
	q := fmt.Sprintf(`SELECT %s FROM hrm_transfers WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		sel, where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("transfers: FindAll: %w", err)
	}
	defer rows.Close()
	list := make([]*Transfer, 0)
	for rows.Next() {
		t, err := scanTrf(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

func (r *repoImpl) Count(ctx context.Context, orgID string, filter TransferListFilter) (int, error) {
	where, args := buildTransfersWhere(orgID, filter)
	var count int
	if err := r.db.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM hrm_transfers WHERE %s`, where), args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("transfers: Count: %w", err)
	}
	return count, nil
}

func (r *repoImpl) FindByRef(ctx context.Context, orgID, employeeID, ref string) (*Transfer, error) {
	q := `SELECT ` + sel + ` FROM hrm_transfers WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`
	args := []any{orgID, ref}
	if employeeID != "" {
		args = append(args, employeeID)
		q += fmt.Sprintf(` AND employee_id=$%d`, len(args))
	}
	return scanTrf(r.db.QueryRow(ctx, q, args...))
}

func (r *repoImpl) Create(ctx context.Context, t *Transfer) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_transfers
		(org_id, employee_id, transfer_type,
		 from_department_id, from_manager_employee_id, from_location,
		 to_department_id, to_manager_employee_id, to_location,
		 effective_date, reason, notes, approval_instance_id, document_id, status, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::date,$11,$12,$13,$14,$15,$16)
		RETURNING id, public_id, created_at, updated_at`,
		t.OrgID, t.EmployeeID, t.TransferType,
		t.FromDepartmentID, t.FromManagerEmployeeID, t.FromLocation,
		t.ToDepartmentID, t.ToManagerEmployeeID, t.ToLocation,
		t.EffectiveDate, t.Reason, t.Notes, t.ApprovalInstanceID, t.DocumentID, t.Status, t.CreatedBy,
	).Scan(&t.ID, &t.PublicID, &t.CreatedAt, &t.UpdatedAt)
}

func (r *repoImpl) Update(ctx context.Context, t *Transfer) error {
	return r.db.QueryRow(ctx,
		`UPDATE hrm_transfers SET
		to_department_id=$1, to_manager_employee_id=$2, to_location=$3,
		effective_date=$4::date, reason=$5, notes=$6, document_id=$7, updated_at=NOW()
		WHERE id=$8 AND org_id=$9 RETURNING updated_at`,
		t.ToDepartmentID, t.ToManagerEmployeeID, t.ToLocation,
		t.EffectiveDate, t.Reason, t.Notes, t.DocumentID, t.ID, t.OrgID,
	).Scan(&t.UpdatedAt)
}

func (r *repoImpl) UpdateStatus(ctx context.Context, id string, status TransferStatus) error {
	_, err := r.db.Exec(ctx, `UPDATE hrm_transfers SET status=$1, updated_at=NOW() WHERE id=$2`, status, id)
	return err
}

func (r *repoImpl) SetApprovalInstance(ctx context.Context, id, instanceID string, status TransferStatus) error {
	_, err := r.db.Exec(ctx,
		`UPDATE hrm_transfers SET approval_instance_id=$1, status=$2, updated_at=NOW() WHERE id=$3`,
		instanceID, status, id,
	)
	return err
}
