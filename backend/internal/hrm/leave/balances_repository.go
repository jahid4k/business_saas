// backend/internal/hrm/leave/balances_repository.go
package leave

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// BalanceRepository defines data access for leave policies, balance
// snapshots, and the transaction ledger. Embedded into Repository so
// repoImpl (repository.go) picks these up automatically.
//
// TENANT ISOLATION RULE: every query MUST include org_id in the WHERE clause.
type BalanceRepository interface {
	// Policies
	FindPolicyByLeaveType(ctx context.Context, orgID, leaveTypeID string) (*LeavePolicy, error) // active-only
	FindPolicyByRef(ctx context.Context, orgID, ref string) (*LeavePolicy, error)
	FindAllPolicies(ctx context.Context, orgID string) ([]*LeavePolicy, error)
	CreatePolicy(ctx context.Context, p *LeavePolicy) error
	UpdatePolicy(ctx context.Context, p *LeavePolicy) error

	// Balance snapshots
	FindLatestBalanceSnapshot(ctx context.Context, orgID, employeeID, leaveTypeID string) (*LeaveBalance, error)
	// FindLatestBalanceSnapshotAsOf is FindLatestBalanceSnapshot bounded by
	// AsOfDate <= throughDate — used to compute a balance as it stood at a
	// past point in time (the carry-forward job's year-end forfeiture math),
	// as opposed to the unrestricted "right now" reading GetCurrentBalance uses.
	FindLatestBalanceSnapshotAsOf(ctx context.Context, orgID, employeeID, leaveTypeID, throughDate string) (*LeaveBalance, error)
	FindBalanceHistory(ctx context.Context, orgID, employeeID, leaveTypeID string, limit, offset int) ([]*LeaveBalance, int, error)
	CreateBalanceSnapshot(ctx context.Context, b *LeaveBalance) error

	// Transactions (ledger)
	SumTransactionsSince(ctx context.Context, orgID, employeeID, leaveTypeID string, sinceDate *string) (float64, error)
	SumTransactionsByType(ctx context.Context, orgID, employeeID, leaveTypeID string, sinceDate *string, throughDate string) (*PeriodTransactionSums, error)
	FindTransactions(ctx context.Context, orgID, employeeID string, filter TransactionFilter) ([]*LeaveTransaction, error)
	CountTransactions(ctx context.Context, orgID, employeeID string, filter TransactionFilter) (int, error)
	CreateTransaction(ctx context.Context, t *LeaveTransaction) error
	ExistsAccrualForPeriod(ctx context.Context, employeeID, leaveTypeID, transactionDate string) (bool, error)
	// ExistsAnyAccrual reports whether ANY accrual transaction has ever been
	// posted for (employeeID, leaveTypeID), regardless of date — used for the
	// on_joining accrual method's one-time-ever idempotency check, distinct
	// from ExistsAccrualForPeriod's specific-date check used by monthly/annual.
	ExistsAnyAccrual(ctx context.Context, employeeID, leaveTypeID string) (bool, error)
	FindApprovedRequestsWithoutUsageEntry(ctx context.Context, orgID, leaveTypeID string) ([]*LeaveRequest, error)
}

// ─────────────────────────────────────────────────────────
// Policies
// ─────────────────────────────────────────────────────────

const lvpSelect = `id, public_id, org_id, leave_type_id, accrual_method, accrual_rate,
	carry_forward_enabled, carry_forward_cap, encashable, encashment_rate_basis,
	is_active, created_by, created_at, updated_at`

func scanPolicy(row pgx.Row) (*LeavePolicy, error) {
	p := &LeavePolicy{}
	err := row.Scan(
		&p.ID, &p.PublicID, &p.OrgID, &p.LeaveTypeID, &p.AccrualMethod, &p.AccrualRate,
		&p.CarryForwardEnabled, &p.CarryForwardCap, &p.Encashable, &p.EncashmentRateBasis,
		&p.IsActive, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *repoImpl) FindPolicyByLeaveType(ctx context.Context, orgID, leaveTypeID string) (*LeavePolicy, error) {
	q := `SELECT ` + lvpSelect + ` FROM hrm_leave_policies WHERE org_id = $1 AND leave_type_id = $2 AND is_active = TRUE`
	p, err := scanPolicy(r.db.QueryRow(ctx, q, orgID, leaveTypeID))
	if err != nil {
		return nil, fmt.Errorf("leave: FindPolicyByLeaveType: %w", err)
	}
	return p, nil
}

func (r *repoImpl) FindPolicyByRef(ctx context.Context, orgID, ref string) (*LeavePolicy, error) {
	q := `SELECT ` + lvpSelect + ` FROM hrm_leave_policies WHERE org_id = $1 AND (id::TEXT = $2 OR public_id = $2)`
	p, err := scanPolicy(r.db.QueryRow(ctx, q, orgID, strings.TrimSpace(ref)))
	if err != nil {
		return nil, fmt.Errorf("leave: FindPolicyByRef: %w", err)
	}
	return p, nil
}

func (r *repoImpl) FindAllPolicies(ctx context.Context, orgID string) ([]*LeavePolicy, error) {
	q := `SELECT ` + lvpSelect + ` FROM hrm_leave_policies WHERE org_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("leave: FindAllPolicies: %w", err)
	}
	defer rows.Close()

	var list []*LeavePolicy
	for rows.Next() {
		p, err := scanPolicy(rows)
		if err != nil {
			return nil, fmt.Errorf("leave: FindAllPolicies: scan: %w", err)
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (r *repoImpl) CreatePolicy(ctx context.Context, p *LeavePolicy) error {
	const q = `
		INSERT INTO hrm_leave_policies
		    (org_id, leave_type_id, accrual_method, accrual_rate, carry_forward_enabled,
		     carry_forward_cap, encashable, encashment_rate_basis, is_active, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING ` + lvpSelect

	created, err := scanPolicy(r.db.QueryRow(ctx, q,
		p.OrgID, p.LeaveTypeID, p.AccrualMethod, p.AccrualRate, p.CarryForwardEnabled,
		p.CarryForwardCap, p.Encashable, p.EncashmentRateBasis, p.IsActive, p.CreatedBy,
	))
	if err != nil {
		return fmt.Errorf("leave: CreatePolicy: %w", err)
	}
	*p = *created
	return nil
}

func (r *repoImpl) UpdatePolicy(ctx context.Context, p *LeavePolicy) error {
	const q = `
		UPDATE hrm_leave_policies
		SET accrual_method = $1, accrual_rate = $2, carry_forward_enabled = $3,
		    carry_forward_cap = $4, encashable = $5, encashment_rate_basis = $6,
		    is_active = $7, updated_at = NOW()
		WHERE id = $8 AND org_id = $9
		RETURNING ` + lvpSelect

	updated, err := scanPolicy(r.db.QueryRow(ctx, q,
		p.AccrualMethod, p.AccrualRate, p.CarryForwardEnabled,
		p.CarryForwardCap, p.Encashable, p.EncashmentRateBasis,
		p.IsActive, p.ID, p.OrgID,
	))
	if err != nil {
		return fmt.Errorf("leave: UpdatePolicy: %w", err)
	}
	if updated == nil {
		return ErrPolicyNotFound
	}
	*p = *updated
	return nil
}

// ─────────────────────────────────────────────────────────
// Balance snapshots
// ─────────────────────────────────────────────────────────

const lvbSelect = `id, public_id, org_id, employee_id, leave_type_id, policy_id,
	period_year, period_month, to_char(as_of_date,'YYYY-MM-DD'),
	opening_balance, accrued, taken, encashed, carried_forward, adjusted, closing_balance,
	created_by, created_at`

func scanBalance(row pgx.Row) (*LeaveBalance, error) {
	b := &LeaveBalance{}
	err := row.Scan(
		&b.ID, &b.PublicID, &b.OrgID, &b.EmployeeID, &b.LeaveTypeID, &b.PolicyID,
		&b.PeriodYear, &b.PeriodMonth, &b.AsOfDate,
		&b.OpeningBalance, &b.Accrued, &b.Taken, &b.Encashed, &b.CarriedForward, &b.Adjusted, &b.ClosingBalance,
		&b.CreatedBy, &b.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (r *repoImpl) FindLatestBalanceSnapshot(ctx context.Context, orgID, employeeID, leaveTypeID string) (*LeaveBalance, error) {
	q := `SELECT ` + lvbSelect + ` FROM hrm_leave_balances
		WHERE org_id = $1 AND employee_id = $2 AND leave_type_id = $3
		ORDER BY period_year DESC, period_month DESC LIMIT 1`
	b, err := scanBalance(r.db.QueryRow(ctx, q, orgID, employeeID, leaveTypeID))
	if err != nil {
		return nil, fmt.Errorf("leave: FindLatestBalanceSnapshot: %w", err)
	}
	return b, nil
}

func (r *repoImpl) FindLatestBalanceSnapshotAsOf(ctx context.Context, orgID, employeeID, leaveTypeID, throughDate string) (*LeaveBalance, error) {
	q := `SELECT ` + lvbSelect + ` FROM hrm_leave_balances
		WHERE org_id = $1 AND employee_id = $2 AND leave_type_id = $3 AND as_of_date <= $4::date
		ORDER BY period_year DESC, period_month DESC LIMIT 1`
	b, err := scanBalance(r.db.QueryRow(ctx, q, orgID, employeeID, leaveTypeID, throughDate))
	if err != nil {
		return nil, fmt.Errorf("leave: FindLatestBalanceSnapshotAsOf: %w", err)
	}
	return b, nil
}

func (r *repoImpl) FindBalanceHistory(ctx context.Context, orgID, employeeID, leaveTypeID string, limit, offset int) ([]*LeaveBalance, int, error) {
	q := `SELECT ` + lvbSelect + ` FROM hrm_leave_balances
		WHERE org_id = $1 AND employee_id = $2 AND leave_type_id = $3
		ORDER BY period_year DESC, period_month DESC LIMIT $4 OFFSET $5`
	rows, err := r.db.Query(ctx, q, orgID, employeeID, leaveTypeID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("leave: FindBalanceHistory: %w", err)
	}
	defer rows.Close()

	var list []*LeaveBalance
	for rows.Next() {
		b, err := scanBalance(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("leave: FindBalanceHistory: scan: %w", err)
		}
		list = append(list, b)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var total int
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrm_leave_balances WHERE org_id = $1 AND employee_id = $2 AND leave_type_id = $3`,
		orgID, employeeID, leaveTypeID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("leave: FindBalanceHistory: count: %w", err)
	}
	return list, total, nil
}

func (r *repoImpl) CreateBalanceSnapshot(ctx context.Context, b *LeaveBalance) error {
	const q = `
		INSERT INTO hrm_leave_balances
		    (org_id, employee_id, leave_type_id, policy_id, period_year, period_month, as_of_date,
		     opening_balance, accrued, taken, encashed, carried_forward, adjusted, closing_balance, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7::date, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING ` + lvbSelect

	created, err := scanBalance(r.db.QueryRow(ctx, q,
		b.OrgID, b.EmployeeID, b.LeaveTypeID, b.PolicyID, b.PeriodYear, b.PeriodMonth, b.AsOfDate,
		b.OpeningBalance, b.Accrued, b.Taken, b.Encashed, b.CarriedForward, b.Adjusted, b.ClosingBalance, b.CreatedBy,
	))
	if err != nil {
		return fmt.Errorf("leave: CreateBalanceSnapshot: %w", err)
	}
	*b = *created
	return nil
}

// ─────────────────────────────────────────────────────────
// Transactions (ledger)
// ─────────────────────────────────────────────────────────

const lvxSelect = `id, public_id, org_id, employee_id, leave_type_id, policy_id,
	transaction_type, days, to_char(transaction_date,'YYYY-MM-DD'),
	leave_request_id, note, created_by, created_at`

func scanTransaction(row pgx.Row) (*LeaveTransaction, error) {
	t := &LeaveTransaction{}
	err := row.Scan(
		&t.ID, &t.PublicID, &t.OrgID, &t.EmployeeID, &t.LeaveTypeID, &t.PolicyID,
		&t.TransactionType, &t.Days, &t.TransactionDate,
		&t.LeaveRequestID, &t.Note, &t.CreatedBy, &t.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return t, nil
}

// SumTransactionsSince sums every transaction not yet covered by the
// snapshot whose AsOfDate is sinceDate. A snapshot's AsOfDate is the
// EXCLUSIVE upper bound of what it covers (it represents "closing state as
// of the instant this date begins" — everything strictly before it), so a
// transaction dated exactly sinceDate has NOT been folded into that
// snapshot and must count here: the comparison is >=, not >.
func (r *repoImpl) SumTransactionsSince(ctx context.Context, orgID, employeeID, leaveTypeID string, sinceDate *string) (float64, error) {
	q := `SELECT COALESCE(SUM(days), 0) FROM hrm_leave_transactions
		WHERE org_id = $1 AND employee_id = $2 AND leave_type_id = $3
		AND ($4::date IS NULL OR transaction_date >= $4::date)`
	var sum float64
	if err := r.db.QueryRow(ctx, q, orgID, employeeID, leaveTypeID, sinceDate).Scan(&sum); err != nil {
		return 0, fmt.Errorf("leave: SumTransactionsSince: %w", err)
	}
	return sum, nil
}

// SumTransactionsByType sums the half-open range [sinceDate, throughDate) —
// see SumTransactionsSince's doc comment for why the lower bound is
// inclusive; the upper bound is exclusive to match (a transaction dated
// exactly throughDate belongs to the NEXT period's snapshot, not this one).
func (r *repoImpl) SumTransactionsByType(ctx context.Context, orgID, employeeID, leaveTypeID string, sinceDate *string, throughDate string) (*PeriodTransactionSums, error) {
	q := `SELECT
		COALESCE(SUM(days) FILTER (WHERE transaction_type = 'accrual'), 0),
		COALESCE(-SUM(days) FILTER (WHERE transaction_type IN ('usage', 'usage_reversal')), 0),
		COALESCE(-SUM(days) FILTER (WHERE transaction_type = 'encashment'), 0),
		COALESCE(SUM(days) FILTER (WHERE transaction_type IN ('carry_forward', 'forfeiture')), 0),
		COALESCE(SUM(days) FILTER (WHERE transaction_type = 'adjustment'), 0)
		FROM hrm_leave_transactions
		WHERE org_id = $1 AND employee_id = $2 AND leave_type_id = $3
		AND ($4::date IS NULL OR transaction_date >= $4::date)
		AND transaction_date < $5::date`
	s := &PeriodTransactionSums{}
	if err := r.db.QueryRow(ctx, q, orgID, employeeID, leaveTypeID, sinceDate, throughDate).Scan(
		&s.Accrued, &s.Taken, &s.Encashed, &s.CarriedForward, &s.Adjusted,
	); err != nil {
		return nil, fmt.Errorf("leave: SumTransactionsByType: %w", err)
	}
	return s, nil
}

func buildTransactionWhere(orgID, employeeID string, filter TransactionFilter) (string, []any) {
	clauses := []string{"org_id = $1", "employee_id = $2"}
	args := []any{orgID, employeeID}

	if filter.LeaveTypeID != "" {
		args = append(args, filter.LeaveTypeID)
		clauses = append(clauses, fmt.Sprintf("leave_type_id::TEXT = $%d", len(args)))
	}
	if filter.TransactionType != "" {
		args = append(args, string(filter.TransactionType))
		clauses = append(clauses, fmt.Sprintf("transaction_type = $%d", len(args)))
	}
	return strings.Join(clauses, " AND "), args
}

func (r *repoImpl) FindTransactions(ctx context.Context, orgID, employeeID string, filter TransactionFilter) ([]*LeaveTransaction, error) {
	where, args := buildTransactionWhere(orgID, employeeID, filter)
	args = append(args, filter.Limit, filter.Offset)
	q := fmt.Sprintf(`
		SELECT %s FROM hrm_leave_transactions
		WHERE %s
		ORDER BY transaction_date DESC, created_at DESC
		LIMIT $%d OFFSET $%d`,
		lvxSelect, where, len(args)-1, len(args),
	)
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("leave: FindTransactions: %w", err)
	}
	defer rows.Close()

	var list []*LeaveTransaction
	for rows.Next() {
		t, err := scanTransaction(rows)
		if err != nil {
			return nil, fmt.Errorf("leave: FindTransactions: scan: %w", err)
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

func (r *repoImpl) CountTransactions(ctx context.Context, orgID, employeeID string, filter TransactionFilter) (int, error) {
	where, args := buildTransactionWhere(orgID, employeeID, filter)
	var count int
	if err := r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM hrm_leave_transactions WHERE %s`, where), args...,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("leave: CountTransactions: %w", err)
	}
	return count, nil
}

func (r *repoImpl) CreateTransaction(ctx context.Context, t *LeaveTransaction) error {
	const q = `
		INSERT INTO hrm_leave_transactions
		    (org_id, employee_id, leave_type_id, policy_id, transaction_type, days,
		     transaction_date, leave_request_id, note, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7::date, $8, $9, $10)
		RETURNING ` + lvxSelect

	created, err := scanTransaction(r.db.QueryRow(ctx, q,
		t.OrgID, t.EmployeeID, t.LeaveTypeID, t.PolicyID, t.TransactionType, t.Days,
		t.TransactionDate, t.LeaveRequestID, t.Note, t.CreatedBy,
	))
	if err != nil {
		return fmt.Errorf("leave: CreateTransaction: %w", err)
	}
	*t = *created
	return nil
}

func (r *repoImpl) ExistsAccrualForPeriod(ctx context.Context, employeeID, leaveTypeID, transactionDate string) (bool, error) {
	q := `SELECT EXISTS(
		SELECT 1 FROM hrm_leave_transactions
		WHERE employee_id = $1 AND leave_type_id = $2 AND transaction_type = 'accrual' AND transaction_date = $3::date
	)`
	var exists bool
	if err := r.db.QueryRow(ctx, q, employeeID, leaveTypeID, transactionDate).Scan(&exists); err != nil {
		return false, fmt.Errorf("leave: ExistsAccrualForPeriod: %w", err)
	}
	return exists, nil
}

func (r *repoImpl) ExistsAnyAccrual(ctx context.Context, employeeID, leaveTypeID string) (bool, error) {
	q := `SELECT EXISTS(
		SELECT 1 FROM hrm_leave_transactions
		WHERE employee_id = $1 AND leave_type_id = $2 AND transaction_type = 'accrual'
	)`
	var exists bool
	if err := r.db.QueryRow(ctx, q, employeeID, leaveTypeID).Scan(&exists); err != nil {
		return false, fmt.Errorf("leave: ExistsAnyAccrual: %w", err)
	}
	return exists, nil
}

// FindApprovedRequestsWithoutUsageEntry finds every approved hrm_leave_requests
// row for (orgID, leaveTypeID) with no matching 'usage' ledger transaction yet
// — used once, at policy-creation time, to backfill historical usage.
func (r *repoImpl) FindApprovedRequestsWithoutUsageEntry(ctx context.Context, orgID, leaveTypeID string) ([]*LeaveRequest, error) {
	q := `SELECT ` + lrSelect + ` FROM hrm_leave_requests lr
		WHERE lr.org_id = $1 AND lr.leave_type_id = $2 AND lr.status = 'approved'
		AND NOT EXISTS (
			SELECT 1 FROM hrm_leave_transactions lx
			WHERE lx.leave_request_id = lr.id AND lx.transaction_type = 'usage'
		)`
	rows, err := r.db.Query(ctx, q, orgID, leaveTypeID)
	if err != nil {
		return nil, fmt.Errorf("leave: FindApprovedRequestsWithoutUsageEntry: %w", err)
	}
	defer rows.Close()

	var list []*LeaveRequest
	for rows.Next() {
		lr, err := scanLeaveRequest(rows)
		if err != nil {
			return nil, fmt.Errorf("leave: FindApprovedRequestsWithoutUsageEntry: scan: %w", err)
		}
		list = append(list, lr)
	}
	return list, rows.Err()
}
