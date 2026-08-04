// backend/internal/hrm/payslips/repository.go
package payslips

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
	// Runs
	FindRuns(ctx context.Context, orgID string) ([]*PayslipRun, error)
	FindRunByRef(ctx context.Context, orgID, ref string) (*PayslipRun, error)
	FindRunByPeriod(ctx context.Context, orgID string, year, month int) (*PayslipRun, error)
	CreateRun(ctx context.Context, r *PayslipRun) error
	UpdateRun(ctx context.Context, r *PayslipRun) error
	// Payslips
	FindPayslips(ctx context.Context, orgID string, filter SlipListFilter) ([]*Payslip, error)
	CountPayslips(ctx context.Context, orgID string, filter SlipListFilter) (int, error)
	FindPayslipByRef(ctx context.Context, orgID, ref string) (*Payslip, error)
	CreatePayslip(ctx context.Context, p *Payslip) error
	CreatePayslipLines(ctx context.Context, lines []*PayslipLine) error
	LoadPayslipLines(ctx context.Context, payslipID string) ([]*PayslipLine, error)
}

type repoImpl struct{ db *pgxpool.Pool }
func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

const runSel = `id, public_id, org_id, period_year, period_month,
	description, currency, attendance_period_id,
	total_employees, total_gross_pay, total_deductions, total_net_pay,
	status, computed_at, computed_by, approved_at, approved_by, paid_at, paid_by,
	created_by, created_at, updated_at`

func scanRun(row pgx.Row) (*PayslipRun, error) {
	r := &PayslipRun{}
	err := row.Scan(
		&r.ID, &r.PublicID, &r.OrgID, &r.PeriodYear, &r.PeriodMonth,
		&r.Description, &r.Currency, &r.AttendancePeriodID,
		&r.TotalEmployees, &r.TotalGrossPay, &r.TotalDeductions, &r.TotalNetPay,
		&r.Status, &r.ComputedAt, &r.ComputedBy, &r.ApprovedAt, &r.ApprovedBy, &r.PaidAt, &r.PaidBy,
		&r.CreatedBy, &r.CreatedAt, &r.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) { return nil, nil }
	if err != nil { return nil, err }
	return r, nil
}

func (r *repoImpl) FindRuns(ctx context.Context, orgID string) ([]*PayslipRun, error) {
	rows, err := r.db.Query(ctx, `SELECT `+runSel+` FROM hrm_payslip_runs WHERE org_id=$1 ORDER BY period_year DESC, period_month DESC`, orgID)
	if err != nil { return nil, fmt.Errorf("payslips: FindRuns: %w", err) }
	defer rows.Close()
	list := make([]*PayslipRun, 0)
	for rows.Next() { run, err := scanRun(rows); if err != nil { return nil, err }; list = append(list, run) }
	return list, rows.Err()
}

func (r *repoImpl) FindRunByRef(ctx context.Context, orgID, ref string) (*PayslipRun, error) {
	return scanRun(r.db.QueryRow(ctx,
		`SELECT `+runSel+` FROM hrm_payslip_runs WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
		orgID, ref))
}

func (r *repoImpl) FindRunByPeriod(ctx context.Context, orgID string, year, month int) (*PayslipRun, error) {
	return scanRun(r.db.QueryRow(ctx,
		`SELECT `+runSel+` FROM hrm_payslip_runs WHERE org_id=$1 AND period_year=$2 AND period_month=$3`,
		orgID, year, month))
}

func (r *repoImpl) CreateRun(ctx context.Context, run *PayslipRun) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_payslip_runs (org_id, period_year, period_month, description, currency, attendance_period_id, status, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, public_id, created_at, updated_at`,
		run.OrgID, run.PeriodYear, run.PeriodMonth, run.Description, run.Currency, run.AttendancePeriodID, run.Status, run.CreatedBy,
	).Scan(&run.ID, &run.PublicID, &run.CreatedAt, &run.UpdatedAt)
}

func (r *repoImpl) UpdateRun(ctx context.Context, run *PayslipRun) error {
	return r.db.QueryRow(ctx,
		`UPDATE hrm_payslip_runs SET
		status=$1, total_employees=$2, total_gross_pay=$3, total_deductions=$4, total_net_pay=$5,
		computed_at=$6, computed_by=$7, approved_at=$8, approved_by=$9, paid_at=$10, paid_by=$11,
		updated_at=NOW()
		WHERE id=$12 AND org_id=$13 RETURNING updated_at`,
		run.Status, run.TotalEmployees, run.TotalGrossPay, run.TotalDeductions, run.TotalNetPay,
		run.ComputedAt, run.ComputedBy, run.ApprovedAt, run.ApprovedBy, run.PaidAt, run.PaidBy,
		run.ID, run.OrgID,
	).Scan(&run.UpdatedAt)
}

const slipSel = `id, public_id, org_id, employee_id, payslip_run_id,
	period_year, period_month, salary_structure_id, salary_structure_name,
	gross_pay, total_deductions, net_pay, basic_pay,
	work_days, present_days, absent_days, leave_days, holiday_days, overtime_hours,
	currency, status, payment_reference, to_char(payment_date,'YYYY-MM-DD'), paid_at,
	created_at, updated_at`

func scanSlip(row pgx.Row) (*Payslip, error) {
	p := &Payslip{}
	err := row.Scan(
		&p.ID, &p.PublicID, &p.OrgID, &p.EmployeeID, &p.PayslipRunID,
		&p.PeriodYear, &p.PeriodMonth, &p.SalaryStructureID, &p.SalaryStructureName,
		&p.GrossPay, &p.TotalDeductions, &p.NetPay, &p.BasicPay,
		&p.WorkDays, &p.PresentDays, &p.AbsentDays, &p.LeaveDays, &p.HolidayDays, &p.OvertimeHours,
		&p.Currency, &p.Status, &p.PaymentReference, &p.PaymentDate, &p.PaidAt,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) { return nil, nil }
	if err != nil { return nil, err }
	return p, nil
}

func buildPayslipsWhere(orgID string, filter SlipListFilter) (string, []any) {
	clauses := []string{"org_id = $1"}
	args := []any{orgID}
	if filter.RunID != "" {
		args = append(args, filter.RunID)
		clauses = append(clauses, fmt.Sprintf("payslip_run_id = $%d", len(args)))
	}
	if filter.EmployeeID != "" {
		args = append(args, filter.EmployeeID)
		clauses = append(clauses, fmt.Sprintf("employee_id = $%d", len(args)))
	}
	if filter.Scope != authz.ScopeAll {
		frag, scopeArgs := scope.Predicate(filter.Scope, "employee_id", len(args), orgID, filter.CallerUserID, scope.DefaultMaxDepth)
		clauses = append(clauses, frag)
		args = append(args, scopeArgs...)
	}
	return strings.Join(clauses, " AND "), args
}

func (r *repoImpl) FindPayslips(ctx context.Context, orgID string, filter SlipListFilter) ([]*Payslip, error) {
	where, args := buildPayslipsWhere(orgID, filter)
	args = append(args, filter.Limit, filter.Offset)
	q := fmt.Sprintf(`SELECT %s FROM hrm_payslips WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		slipSel, where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil { return nil, fmt.Errorf("payslips: FindPayslips: %w", err) }
	defer rows.Close()
	list := make([]*Payslip, 0)
	for rows.Next() { s, err := scanSlip(rows); if err != nil { return nil, err }; list = append(list, s) }
	return list, rows.Err()
}

func (r *repoImpl) CountPayslips(ctx context.Context, orgID string, filter SlipListFilter) (int, error) {
	where, args := buildPayslipsWhere(orgID, filter)
	var count int
	if err := r.db.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM hrm_payslips WHERE %s`, where), args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("payslips: CountPayslips: %w", err)
	}
	return count, nil
}

func (r *repoImpl) FindPayslipByRef(ctx context.Context, orgID, ref string) (*Payslip, error) {
	return scanSlip(r.db.QueryRow(ctx,
		`SELECT `+slipSel+` FROM hrm_payslips WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
		orgID, ref))
}

func (r *repoImpl) CreatePayslip(ctx context.Context, p *Payslip) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_payslips
		(org_id, employee_id, payslip_run_id, period_year, period_month,
		 salary_structure_id, salary_structure_name, gross_pay, total_deductions, net_pay, basic_pay,
		 work_days, present_days, absent_days, leave_days, holiday_days, overtime_hours,
		 currency, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
		RETURNING id, public_id, created_at, updated_at`,
		p.OrgID, p.EmployeeID, p.PayslipRunID, p.PeriodYear, p.PeriodMonth,
		p.SalaryStructureID, p.SalaryStructureName, p.GrossPay, p.TotalDeductions, p.NetPay, p.BasicPay,
		p.WorkDays, p.PresentDays, p.AbsentDays, p.LeaveDays, p.HolidayDays, p.OvertimeHours,
		p.Currency, p.Status,
	).Scan(&p.ID, &p.PublicID, &p.CreatedAt, &p.UpdatedAt)
}

func (r *repoImpl) CreatePayslipLines(ctx context.Context, lines []*PayslipLine) error {
	for _, l := range lines {
		err := r.db.QueryRow(ctx,
			`INSERT INTO hrm_payslip_lines
			(payslip_id, org_id, component_id, component_name, component_type,
			 calc_method, formula_used, computed_amount, display_order)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			RETURNING id, created_at`,
			l.PayslipID, l.OrgID, l.ComponentID, l.ComponentName, l.ComponentType,
			l.CalcMethod, l.FormulaUsed, l.ComputedAmount, l.DisplayOrder,
		).Scan(&l.ID, &l.CreatedAt)
		if err != nil { return fmt.Errorf("payslips: CreatePayslipLines: %w", err) }
	}
	return nil
}

func (r *repoImpl) LoadPayslipLines(ctx context.Context, payslipID string) ([]*PayslipLine, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, payslip_id, org_id, component_id, component_name, component_type,
		calc_method, formula_used, computed_amount, display_order, created_at
		FROM hrm_payslip_lines WHERE payslip_id=$1 ORDER BY display_order`,
		payslipID)
	if err != nil { return nil, err }
	defer rows.Close()
	list := make([]*PayslipLine, 0)
	for rows.Next() {
		l := &PayslipLine{}
		if err := rows.Scan(&l.ID, &l.PayslipID, &l.OrgID, &l.ComponentID, &l.ComponentName,
			&l.ComponentType, &l.CalcMethod, &l.FormulaUsed, &l.ComputedAmount, &l.DisplayOrder, &l.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, l)
	}
	return list, rows.Err()
}
