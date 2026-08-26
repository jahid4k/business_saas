// backend/internal/hrm/reimbursements/repository.go
package reimbursements

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
	List(ctx context.Context, orgID string, filter ListFilter) ([]*Reimbursement, int, error)
	FindByRef(ctx context.Context, orgID, ref string) (*Reimbursement, error)
	FindByApprovalInstance(ctx context.Context, orgID, instanceID string) (*Reimbursement, error)
	Create(ctx context.Context, r *Reimbursement) error
	Update(ctx context.Context, r *Reimbursement) error
	// PendingForEmployee returns approved, unpaid reimbursements for an
	// employee. Unlike loan installments, reimbursements carry no due
	// period — an approved reimbursement is payable in ANY subsequent run.
	PendingForEmployee(ctx context.Context, orgID, employeeID string) ([]*Reimbursement, error)
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

const sel = `id, public_id, org_id, employee_id, category, description, amount, currency,
	status, approval_instance_id, payslip_run_id, payslip_line_id, paid_at,
	created_by, created_at, updated_at`

func scanRow(row pgx.Row) (*Reimbursement, error) {
	r := &Reimbursement{}
	err := row.Scan(&r.ID, &r.PublicID, &r.OrgID, &r.EmployeeID, &r.Category, &r.Description,
		&r.Amount, &r.Currency, &r.Status, &r.ApprovalInstanceID,
		&r.PayslipRunID, &r.PayslipLineID, &r.PaidAt,
		&r.CreatedBy, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (rp *repoImpl) List(ctx context.Context, orgID string, filter ListFilter) ([]*Reimbursement, int, error) {
	clauses := []string{"org_id = $1"}
	args := []any{orgID}
	if filter.EmployeeID != "" {
		args = append(args, filter.EmployeeID)
		clauses = append(clauses, fmt.Sprintf("employee_id = $%d", len(args)))
	}
	if filter.Scope != authz.ScopeAll {
		frag, scopeArgs := scope.Predicate(filter.Scope, "employee_id", len(args), orgID, filter.CallerUserID, scope.DefaultMaxDepth)
		clauses = append(clauses, frag)
		args = append(args, scopeArgs...)
	}
	where := strings.Join(clauses, " AND ")

	var total int
	if err := rp.db.QueryRow(ctx, `SELECT COUNT(*) FROM hrm_reimbursements WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("reimbursements: List: count: %w", err)
	}

	args = append(args, filter.Limit, filter.Offset)
	q := fmt.Sprintf(`SELECT %s FROM hrm_reimbursements WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		sel, where, len(args)-1, len(args))
	rows, err := rp.db.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("reimbursements: List: %w", err)
	}
	defer rows.Close()
	list := make([]*Reimbursement, 0)
	for rows.Next() {
		r, err := scanRow(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, r)
	}
	return list, total, rows.Err()
}

func (rp *repoImpl) FindByRef(ctx context.Context, orgID, ref string) (*Reimbursement, error) {
	return scanRow(rp.db.QueryRow(ctx,
		`SELECT `+sel+` FROM hrm_reimbursements WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
		orgID, ref))
}

func (rp *repoImpl) FindByApprovalInstance(ctx context.Context, orgID, instanceID string) (*Reimbursement, error) {
	return scanRow(rp.db.QueryRow(ctx,
		`SELECT `+sel+` FROM hrm_reimbursements WHERE org_id=$1 AND approval_instance_id=$2::uuid`,
		orgID, instanceID))
}

func (rp *repoImpl) Create(ctx context.Context, r *Reimbursement) error {
	return rp.db.QueryRow(ctx,
		`INSERT INTO hrm_reimbursements (org_id, employee_id, category, description, amount, currency, status, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id, public_id, created_at, updated_at`,
		r.OrgID, r.EmployeeID, r.Category, r.Description, r.Amount, r.Currency, r.Status, r.CreatedBy,
	).Scan(&r.ID, &r.PublicID, &r.CreatedAt, &r.UpdatedAt)
}

func (rp *repoImpl) Update(ctx context.Context, r *Reimbursement) error {
	ct, err := rp.db.Exec(ctx,
		`UPDATE hrm_reimbursements
		    SET status=$3, approval_instance_id=$4, payslip_run_id=$5, payslip_line_id=$6, paid_at=$7, updated_at=NOW()
		  WHERE org_id=$1 AND id=$2::uuid`,
		r.OrgID, r.ID, r.Status, r.ApprovalInstanceID, r.PayslipRunID, r.PayslipLineID, r.PaidAt)
	if err != nil {
		return fmt.Errorf("reimbursements: Update: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (rp *repoImpl) PendingForEmployee(ctx context.Context, orgID, employeeID string) ([]*Reimbursement, error) {
	rows, err := rp.db.Query(ctx,
		`SELECT `+sel+` FROM hrm_reimbursements WHERE org_id=$1 AND employee_id=$2 AND status='approved'`,
		orgID, employeeID)
	if err != nil {
		return nil, fmt.Errorf("reimbursements: PendingForEmployee: %w", err)
	}
	defer rows.Close()
	list := make([]*Reimbursement, 0)
	for rows.Next() {
		r, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, rows.Err()
}
