// backend/internal/hrm/loans/repository.go
package loans

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
	ListLoans(ctx context.Context, orgID string, filter ListFilter) ([]*Loan, int, error)
	FindLoanByRef(ctx context.Context, orgID, ref string) (*Loan, error)
	FindLoanByApprovalInstance(ctx context.Context, orgID, instanceID string) (*Loan, error)
	CreateLoan(ctx context.Context, l *Loan) error
	UpdateLoan(ctx context.Context, l *Loan) error

	// CreateSchedule persists a freshly-amortized schedule for a loan in one
	// transaction. Called exactly once, at disbursement — see migration
	// 00100's header.
	CreateSchedule(ctx context.Context, loanID string, rows []*ScheduleRow) error
	ListScheduleByLoan(ctx context.Context, loanID string) ([]*ScheduleRow, error)
	// PendingInstallmentsForEmployee returns installments due ON OR BEFORE
	// the given period across ALL of the employee's active loans, oldest
	// first — a backlog from a prior run's zero-net-pay capping is caught up
	// before anything newer.
	PendingInstallmentsForEmployee(ctx context.Context, orgID, employeeID string, year, month int) ([]*ScheduleRow, error)
	// RecordRecoveryEvents applies a batch of recoveries atomically: one
	// hrm_loan_recovery_events row plus a recovered_amount/status update per
	// schedule row, in a single transaction — see payslips.LoanSource's doc
	// comment on why partial failure here must not happen.
	RecordRecoveryEvents(ctx context.Context, runID string, applications []RecoveryApplicationInput) error

	// ForecloseLoan marks a loan foreclosed and every remaining
	// pending/partially_recovered schedule row 'foreclosed' — those rows are
	// never deleted, only stopped, preserving the amortization as history.
	ForecloseLoan(ctx context.Context, orgID, loanID, foreclosedBy string, foreclosureAmount string) error
}

// RecoveryApplicationInput is the repository-layer shape of a recovery
// application, decoupled from payslips.RecoveryApplication so this package
// need not import payslips for its own repository interface (only the
// service layer, satisfying payslips.LoanSource, imports payslips).
type RecoveryApplicationInput struct {
	ScheduleID    string
	LineID        string
	AmountApplied string
}

type repoImpl struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) Repository { return &repoImpl{db: db} }

const loanSel = `id, public_id, org_id, employee_id, loan_type, principal_amount, interest_rate_pct,
	tenure_months, installment_amount, status, approval_instance_id,
	disbursed_at, disbursed_by, foreclosed_at, foreclosure_amount,
	created_by, created_at, updated_at`

func scanLoan(row pgx.Row) (*Loan, error) {
	l := &Loan{}
	err := row.Scan(&l.ID, &l.PublicID, &l.OrgID, &l.EmployeeID, &l.LoanType,
		&l.PrincipalAmount, &l.InterestRatePct, &l.TenureMonths, &l.InstallmentAmount,
		&l.Status, &l.ApprovalInstanceID,
		&l.DisbursedAt, &l.DisbursedBy, &l.ForeclosedAt, &l.ForeclosureAmount,
		&l.CreatedBy, &l.CreatedAt, &l.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return l, nil
}

func (r *repoImpl) ListLoans(ctx context.Context, orgID string, filter ListFilter) ([]*Loan, int, error) {
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
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM hrm_loans WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("loans: ListLoans: count: %w", err)
	}

	args = append(args, filter.Limit, filter.Offset)
	q := fmt.Sprintf(`SELECT %s FROM hrm_loans WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		loanSel, where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("loans: ListLoans: %w", err)
	}
	defer rows.Close()
	list := make([]*Loan, 0)
	for rows.Next() {
		l, err := scanLoan(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, l)
	}
	return list, total, rows.Err()
}

func (r *repoImpl) FindLoanByRef(ctx context.Context, orgID, ref string) (*Loan, error) {
	return scanLoan(r.db.QueryRow(ctx,
		`SELECT `+loanSel+` FROM hrm_loans WHERE org_id=$1 AND (id::text=$2 OR public_id=$2)`,
		orgID, ref))
}

func (r *repoImpl) FindLoanByApprovalInstance(ctx context.Context, orgID, instanceID string) (*Loan, error) {
	return scanLoan(r.db.QueryRow(ctx,
		`SELECT `+loanSel+` FROM hrm_loans WHERE org_id=$1 AND approval_instance_id=$2::uuid`,
		orgID, instanceID))
}

func (r *repoImpl) CreateLoan(ctx context.Context, l *Loan) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO hrm_loans (org_id, employee_id, loan_type, principal_amount, interest_rate_pct, tenure_months, status, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id, public_id, created_at, updated_at`,
		l.OrgID, l.EmployeeID, l.LoanType, l.PrincipalAmount, l.InterestRatePct, l.TenureMonths, l.Status, l.CreatedBy,
	).Scan(&l.ID, &l.PublicID, &l.CreatedAt, &l.UpdatedAt)
}

func (r *repoImpl) UpdateLoan(ctx context.Context, l *Loan) error {
	ct, err := r.db.Exec(ctx,
		`UPDATE hrm_loans
		    SET status=$3, approval_instance_id=$4, installment_amount=$5,
		        disbursed_at=$6, disbursed_by=$7, foreclosed_at=$8, foreclosure_amount=$9, updated_at=NOW()
		  WHERE org_id=$1 AND id=$2::uuid`,
		l.OrgID, l.ID, l.Status, l.ApprovalInstanceID, l.InstallmentAmount,
		l.DisbursedAt, l.DisbursedBy, l.ForeclosedAt, l.ForeclosureAmount)
	if err != nil {
		return fmt.Errorf("loans: UpdateLoan: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrLoanNotFound
	}
	return nil
}

const scheduleSel = `id, public_id, loan_id, installment_number, due_period_year, due_period_month,
	principal_component, interest_component, total_amount, recovered_amount, status, created_at, updated_at`

func scanSchedule(row pgx.Row) (*ScheduleRow, error) {
	s := &ScheduleRow{}
	err := row.Scan(&s.ID, &s.PublicID, &s.LoanID, &s.InstallmentNumber, &s.DuePeriodYear, &s.DuePeriodMonth,
		&s.PrincipalComponent, &s.InterestComponent, &s.TotalAmount, &s.RecoveredAmount, &s.Status,
		&s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *repoImpl) CreateSchedule(ctx context.Context, loanID string, rows []*ScheduleRow) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("loans: CreateSchedule: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, row := range rows {
		if err := tx.QueryRow(ctx,
			`INSERT INTO hrm_loan_schedules
			    (loan_id, installment_number, due_period_year, due_period_month,
			     principal_component, interest_component, total_amount)
			 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id, public_id, created_at, updated_at`,
			loanID, row.InstallmentNumber, row.DuePeriodYear, row.DuePeriodMonth,
			row.PrincipalComponent, row.InterestComponent, row.TotalAmount,
		).Scan(&row.ID, &row.PublicID, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return fmt.Errorf("loans: CreateSchedule: installment %d: %w", row.InstallmentNumber, err)
		}
	}
	return tx.Commit(ctx)
}

func (r *repoImpl) ListScheduleByLoan(ctx context.Context, loanID string) ([]*ScheduleRow, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+scheduleSel+` FROM hrm_loan_schedules WHERE loan_id=$1::uuid ORDER BY installment_number`, loanID)
	if err != nil {
		return nil, fmt.Errorf("loans: ListScheduleByLoan: %w", err)
	}
	defer rows.Close()
	list := make([]*ScheduleRow, 0)
	for rows.Next() {
		s, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

const scheduleSelQualified = `s.id, s.public_id, s.loan_id, s.installment_number, s.due_period_year, s.due_period_month,
	s.principal_component, s.interest_component, s.total_amount, s.recovered_amount, s.status, s.created_at, s.updated_at`

func (r *repoImpl) PendingInstallmentsForEmployee(ctx context.Context, orgID, employeeID string, year, month int) ([]*ScheduleRow, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+scheduleSelQualified+`
		   FROM hrm_loan_schedules s
		   JOIN hrm_loans l ON l.id = s.loan_id
		  WHERE l.org_id=$1 AND l.employee_id=$2 AND l.status='active'
		    AND s.status IN ('pending','partially_recovered')
		    AND make_date(s.due_period_year, s.due_period_month, 1) <= make_date($3,$4,1)
		  ORDER BY s.due_period_year, s.due_period_month, s.installment_number`,
		orgID, employeeID, year, month)
	if err != nil {
		return nil, fmt.Errorf("loans: PendingInstallmentsForEmployee: %w", err)
	}
	defer rows.Close()
	list := make([]*ScheduleRow, 0)
	for rows.Next() {
		s, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

func (r *repoImpl) RecordRecoveryEvents(ctx context.Context, runID string, applications []RecoveryApplicationInput) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("loans: RecordRecoveryEvents: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, a := range applications {
		var loanID string
		if err := tx.QueryRow(ctx, `SELECT loan_id FROM hrm_loan_schedules WHERE id=$1::uuid`, a.ScheduleID).Scan(&loanID); err != nil {
			return fmt.Errorf("loans: RecordRecoveryEvents: resolve loan for schedule %s: %w", a.ScheduleID, err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO hrm_loan_recovery_events (loan_id, schedule_id, payslip_run_id, payslip_line_id, amount_recovered)
			 VALUES ($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5)`,
			loanID, a.ScheduleID, runID, a.LineID, a.AmountApplied); err != nil {
			return fmt.Errorf("loans: RecordRecoveryEvents: insert event for schedule %s: %w", a.ScheduleID, err)
		}
		ct, err := tx.Exec(ctx,
			`UPDATE hrm_loan_schedules
			    SET recovered_amount = recovered_amount + $2,
			        status = CASE WHEN recovered_amount + $2 >= total_amount THEN 'recovered' ELSE 'partially_recovered' END,
			        updated_at = NOW()
			  WHERE id=$1::uuid`,
			a.ScheduleID, a.AmountApplied)
		if err != nil {
			return fmt.Errorf("loans: RecordRecoveryEvents: update schedule %s: %w", a.ScheduleID, err)
		}
		if ct.RowsAffected() == 0 {
			return fmt.Errorf("loans: RecordRecoveryEvents: schedule %s not found", a.ScheduleID)
		}
	}

	// A loan whose every installment is now fully recovered is complete.
	// Scoped to loans touched by this batch, not a full-table scan.
	if _, err := tx.Exec(ctx,
		`UPDATE hrm_loans SET status='completed', updated_at=NOW()
		  WHERE status='active' AND id IN (
		      SELECT DISTINCT s.loan_id FROM hrm_loan_schedules s
		      WHERE s.id = ANY($1::uuid[])
		  ) AND NOT EXISTS (
		      SELECT 1 FROM hrm_loan_schedules s2
		       WHERE s2.loan_id = hrm_loans.id AND s2.status IN ('pending','partially_recovered')
		  )`,
		scheduleIDs(applications)); err != nil {
		return fmt.Errorf("loans: RecordRecoveryEvents: complete finished loans: %w", err)
	}

	return tx.Commit(ctx)
}

func scheduleIDs(applications []RecoveryApplicationInput) []string {
	ids := make([]string, len(applications))
	for i, a := range applications {
		ids[i] = a.ScheduleID
	}
	return ids
}

func (r *repoImpl) ForecloseLoan(ctx context.Context, orgID, loanID, foreclosedBy string, foreclosureAmount string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("loans: ForecloseLoan: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	ct, err := tx.Exec(ctx,
		`UPDATE hrm_loans
		    SET status='foreclosed', foreclosed_at=NOW(), foreclosure_amount=$3, updated_at=NOW()
		  WHERE org_id=$1 AND id=$2::uuid AND status='active'`,
		orgID, loanID, foreclosureAmount)
	if err != nil {
		return fmt.Errorf("loans: ForecloseLoan: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrWrongLoanStatus
	}
	if _, err := tx.Exec(ctx,
		`UPDATE hrm_loan_schedules SET status='foreclosed', updated_at=NOW()
		  WHERE loan_id=$1::uuid AND status IN ('pending','partially_recovered')`,
		loanID); err != nil {
		return fmt.Errorf("loans: ForecloseLoan: close schedule: %w", err)
	}
	return tx.Commit(ctx)
}
