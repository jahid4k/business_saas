// backend/internal/hrm/compensation/bonuses_repository.go
package compensation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/scope"
)

// BonusRepository covers hrm_bonuses. Marking bonuses paid after a bonus
// payroll run (payslips.BonusSource.MarkBonusesPaid) is NOT here — it needs
// to update several bonuses atomically in one transaction, which does not fit
// this interface's one-row-per-call shape, so bonuses_service.go does it
// directly against *pgxpool.Pool. See that method's doc comment.
type BonusRepository interface {
	ListBonuses(ctx context.Context, orgID string, filter ListFilter) ([]*Bonus, int, error)
	FindBonusByRef(ctx context.Context, orgID, ref string) (*Bonus, error)
	FindBonusByApprovalInstance(ctx context.Context, orgID, instanceID string) (*Bonus, error)
	CreateBonus(ctx context.Context, b *Bonus) error
	UpdateBonus(ctx context.Context, b *Bonus) error

	// PendingForPeriod returns approved, unpaid bonuses for orgID whose
	// (period_year, period_month) matches — month-less (annual) bonuses match
	// any month in that year, so they are not silently skipped by every
	// monthly bonus run.
	PendingForPeriod(ctx context.Context, orgID string, year, month int) ([]*Bonus, error)
}

const bonusSel = `id, public_id, org_id, employee_id, bonus_type, description, period_year, period_month,
	amount, currency, calculation_snapshot, status, approval_instance_id,
	payslip_run_id, payslip_line_id, paid_at, created_by, created_at, updated_at`

func scanBonus(row pgx.Row) (*Bonus, error) {
	b := &Bonus{}
	err := row.Scan(&b.ID, &b.PublicID, &b.OrgID, &b.EmployeeID, &b.BonusType, &b.Description,
		&b.PeriodYear, &b.PeriodMonth, &b.Amount, &b.Currency, &b.CalculationSnapshot,
		&b.Status, &b.ApprovalInstanceID, &b.PayslipRunID, &b.PayslipLineID, &b.PaidAt,
		&b.CreatedBy, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (r *repoImpl) ListBonuses(ctx context.Context, orgID string, filter ListFilter) ([]*Bonus, int, error) {
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
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM hrm_bonuses WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("compensation: ListBonuses: count: %w", err)
	}

	args = append(args, filter.Limit, filter.Offset)
	q := fmt.Sprintf(`SELECT %s FROM hrm_bonuses WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		bonusSel, where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("compensation: ListBonuses: %w", err)
	}
	defer rows.Close()
	list := make([]*Bonus, 0)
	for rows.Next() {
		b, err := scanBonus(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, b)
	}
	return list, total, rows.Err()
}

func (r *repoImpl) FindBonusByRef(ctx context.Context, orgID, ref string) (*Bonus, error) {
	return scanBonus(r.db.QueryRow(ctx,
		`SELECT `+bonusSel+` FROM hrm_bonuses WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
		orgID, ref))
}

func (r *repoImpl) FindBonusByApprovalInstance(ctx context.Context, orgID, instanceID string) (*Bonus, error) {
	return scanBonus(r.db.QueryRow(ctx,
		`SELECT `+bonusSel+` FROM hrm_bonuses WHERE org_id=$1 AND approval_instance_id=$2::uuid`,
		orgID, instanceID))
}

func (r *repoImpl) CreateBonus(ctx context.Context, b *Bonus) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_bonuses (org_id, employee_id, bonus_type, description, period_year, period_month,
		     amount, currency, calculation_snapshot, status, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id, public_id, created_at, updated_at`,
		b.OrgID, b.EmployeeID, b.BonusType, b.Description, b.PeriodYear, b.PeriodMonth,
		b.Amount, b.Currency, b.CalculationSnapshot, b.Status, b.CreatedBy,
	).Scan(&b.ID, &b.PublicID, &b.CreatedAt, &b.UpdatedAt)
}

func (r *repoImpl) UpdateBonus(ctx context.Context, b *Bonus) error {
	ct, err := r.db.Exec(ctx,
		`UPDATE hrm_bonuses
		    SET status=$3, approval_instance_id=$4, payslip_run_id=$5, payslip_line_id=$6,
		        paid_at=$7, updated_at=NOW()
		  WHERE org_id=$1 AND id=$2::uuid`,
		b.OrgID, b.ID, b.Status, b.ApprovalInstanceID, b.PayslipRunID, b.PayslipLineID, b.PaidAt)
	if err != nil {
		return fmt.Errorf("compensation: UpdateBonus: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrBonusNotFound
	}
	return nil
}

func (r *repoImpl) PendingForPeriod(ctx context.Context, orgID string, year, month int) ([]*Bonus, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+bonusSel+` FROM hrm_bonuses
		  WHERE org_id=$1 AND status='approved' AND period_year=$2
		    AND (period_month IS NULL OR period_month=$3)`,
		orgID, year, month)
	if err != nil {
		return nil, fmt.Errorf("compensation: PendingForPeriod: %w", err)
	}
	defer rows.Close()
	list := make([]*Bonus, 0)
	for rows.Next() {
		b, err := scanBonus(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, b)
	}
	return list, rows.Err()
}
