// backend/internal/hrm/leave/balances_service.go
package leave

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/mridha/businesssaas/internal/audit"
)

// ─────────────────────────────────────────────────────────
// Policies
// ─────────────────────────────────────────────────────────

func (s *serviceImpl) ListPolicies(ctx context.Context, orgID string) (*PolicyListResponse, error) {
	list, err := s.repo.FindAllPolicies(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("leave: ListPolicies: %w", err)
	}
	if list == nil {
		list = []*LeavePolicy{}
	}
	return &PolicyListResponse{Policies: list, Total: len(list)}, nil
}

func (s *serviceImpl) GetPolicy(ctx context.Context, orgID, ref string) (*LeavePolicy, error) {
	p, err := s.repo.FindPolicyByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("leave: GetPolicy: %w", err)
	}
	if p == nil {
		return nil, ErrPolicyNotFound
	}
	return p, nil
}

// CreatePolicy activates balance tracking for a leave type. It synchronously
// backfills historical usage from any pre-existing approved requests for
// this leave type (decision #2: replay taken-days only, no retroactive
// accrual simulation).
func (s *serviceImpl) CreatePolicy(ctx context.Context, orgID, createdBy string, req CreatePolicyRequest) (*LeavePolicy, error) {
	leaveTypeID := strings.TrimSpace(req.LeaveTypeID)
	if leaveTypeID == "" {
		return nil, ErrLeaveTypeIDRequired
	}
	lt, err := s.repo.FindLeaveTypeByRef(ctx, orgID, leaveTypeID)
	if err != nil {
		return nil, fmt.Errorf("leave: CreatePolicy: type check: %w", err)
	}
	if lt == nil {
		return nil, ErrLeaveTypeNotFound
	}

	if !req.AccrualMethod.IsValid() {
		return nil, ErrInvalidAccrualMethod
	}
	if req.AccrualRate < 0 {
		return nil, ErrInvalidAccrualRate
	}
	if req.CarryForwardCap != nil && *req.CarryForwardCap < 0 {
		return nil, ErrInvalidCarryForwardCap
	}
	if req.EncashmentRateBasis != nil && !req.EncashmentRateBasis.IsValid() {
		return nil, ErrInvalidEncashmentBasis
	}

	existing, err := s.repo.FindPolicyByLeaveType(ctx, orgID, lt.ID)
	if err != nil {
		return nil, fmt.Errorf("leave: CreatePolicy: existing check: %w", err)
	}
	if existing != nil {
		return nil, ErrPolicyAlreadyExists
	}

	cap := req.CarryForwardCap
	if !req.CarryForwardEnabled {
		cap = nil
	}
	basis := req.EncashmentRateBasis
	if !req.Encashable {
		basis = nil
	}

	p := &LeavePolicy{
		OrgID:               orgID,
		LeaveTypeID:         lt.ID,
		AccrualMethod:       req.AccrualMethod,
		AccrualRate:         req.AccrualRate,
		CarryForwardEnabled: req.CarryForwardEnabled,
		CarryForwardCap:     cap,
		Encashable:          req.Encashable,
		EncashmentRateBasis: basis,
		IsActive:            true,
		CreatedBy:           createdBy,
	}
	if err := s.repo.CreatePolicy(ctx, p); err != nil {
		return nil, fmt.Errorf("leave: CreatePolicy: %w", err)
	}

	s.backfillHistoricalUsage(ctx, orgID, lt.ID, p.ID, createdBy)

	return p, nil
}

// backfillHistoricalUsage replays pre-existing approved leave requests into
// the ledger as 'usage' transactions, dated at each request's original
// approval date. Each insert is independent and guarded by the
// uq_hrm_lvx_request_type unique index, so a failure on one historical row
// is skipped rather than aborting the rest — this mirrors the per-item
// isolation already used by attendance.RunAbsenceSweep and
// milestones.GenerateUpcoming. No balance snapshot is written here; the
// balance is simply queryable via the checkpoint+delta formula with no
// prior snapshot (decision #2: balances may start low/negative).
func (s *serviceImpl) backfillHistoricalUsage(ctx context.Context, orgID, leaveTypeID, policyID, actorID string) {
	requests, err := s.repo.FindApprovedRequestsWithoutUsageEntry(ctx, orgID, leaveTypeID)
	if err != nil {
		return
	}
	note := "Backfilled from a pre-existing approved leave request"
	for _, lr := range requests {
		txDate := lr.CreatedAt
		if lr.ReviewedAt != nil {
			txDate = *lr.ReviewedAt
		}
		t := &LeaveTransaction{
			OrgID: orgID, EmployeeID: lr.EmployeeID, LeaveTypeID: leaveTypeID, PolicyID: policyID,
			TransactionType: TransactionUsage, Days: -lr.TotalDays,
			TransactionDate: txDate.Format(dateLayout),
			LeaveRequestID:  &lr.ID, Note: &note, CreatedBy: actorID,
		}
		_ = s.repo.CreateTransaction(ctx, t)
	}
}

// UpdatePolicy never re-triggers backfill — that only happens at creation time.
func (s *serviceImpl) UpdatePolicy(ctx context.Context, orgID, ref string, req UpdatePolicyRequest) (*LeavePolicy, error) {
	p, err := s.repo.FindPolicyByRef(ctx, orgID, ref)
	if err != nil {
		return nil, fmt.Errorf("leave: UpdatePolicy: %w", err)
	}
	if p == nil {
		return nil, ErrPolicyNotFound
	}

	if req.AccrualMethod != nil {
		if !req.AccrualMethod.IsValid() {
			return nil, ErrInvalidAccrualMethod
		}
		p.AccrualMethod = *req.AccrualMethod
	}
	if req.AccrualRate != nil {
		if *req.AccrualRate < 0 {
			return nil, ErrInvalidAccrualRate
		}
		p.AccrualRate = *req.AccrualRate
	}
	if req.CarryForwardEnabled != nil {
		p.CarryForwardEnabled = *req.CarryForwardEnabled
	}
	if req.CarryForwardCap != nil {
		if *req.CarryForwardCap < 0 {
			return nil, ErrInvalidCarryForwardCap
		}
		p.CarryForwardCap = req.CarryForwardCap
	}
	if !p.CarryForwardEnabled {
		p.CarryForwardCap = nil
	}
	if req.Encashable != nil {
		p.Encashable = *req.Encashable
	}
	if req.EncashmentRateBasis != nil {
		if !req.EncashmentRateBasis.IsValid() {
			return nil, ErrInvalidEncashmentBasis
		}
		p.EncashmentRateBasis = req.EncashmentRateBasis
	}
	if !p.Encashable {
		p.EncashmentRateBasis = nil
	}
	if req.IsActive != nil {
		p.IsActive = *req.IsActive
	}

	if err := s.repo.UpdatePolicy(ctx, p); err != nil {
		return nil, fmt.Errorf("leave: UpdatePolicy: %w", err)
	}
	return p, nil
}

// ─────────────────────────────────────────────────────────
// Balance reads
// ─────────────────────────────────────────────────────────

// balanceAsOf computes the balance as it stood at a specific past date —
// bounded by throughDate on both the snapshot lookup and the transaction
// sum — as opposed to currentBalanceFor's unrestricted "right now" reading.
// Used by the carry-forward job: forfeiture must be computed against the
// balance as of year-end, not against whatever has posted since (which
// could include days that already belong to the new year by the time the
// job actually runs).
func (s *serviceImpl) balanceAsOf(ctx context.Context, orgID, employeeID, leaveTypeID, throughDate string) (float64, error) {
	snapshot, err := s.repo.FindLatestBalanceSnapshotAsOf(ctx, orgID, employeeID, leaveTypeID, throughDate)
	if err != nil {
		return 0, fmt.Errorf("snapshot lookup: %w", err)
	}

	var opening float64
	var sinceDate *string
	if snapshot != nil {
		opening = snapshot.ClosingBalance
		asOf := snapshot.AsOfDate
		sinceDate = &asOf
	}

	sums, err := s.repo.SumTransactionsByType(ctx, orgID, employeeID, leaveTypeID, sinceDate, throughDate)
	if err != nil {
		return 0, fmt.Errorf("sum transactions: %w", err)
	}
	return opening + sums.Accrued - sums.Taken - sums.Encashed + sums.CarriedForward + sums.Adjusted, nil
}

// currentBalanceFor computes "what does this employee have available right
// now" for one leave type: the latest hrm_leave_balances snapshot's
// ClosingBalance + every ledger transaction posted after that snapshot's
// AsOfDate. This boundary (strictly after AsOfDate) must exactly match the
// boundary writeMonthlySnapshot uses when computing each period's sums —
// see the integration test that proves this (hrm_leave_balances_test.go).
func (s *serviceImpl) currentBalanceFor(ctx context.Context, orgID, employeeID string, lt *LeaveType) (*CurrentBalance, error) {
	cb := &CurrentBalance{LeaveTypeID: lt.ID, LeaveTypeName: lt.Name}

	policy, err := s.repo.FindPolicyByLeaveType(ctx, orgID, lt.ID)
	if err != nil {
		return nil, fmt.Errorf("policy lookup: %w", err)
	}
	if policy == nil {
		return cb, nil
	}
	cb.HasPolicy = true
	cb.PolicyID = &policy.ID

	snapshot, err := s.repo.FindLatestBalanceSnapshot(ctx, orgID, employeeID, lt.ID)
	if err != nil {
		return nil, fmt.Errorf("snapshot lookup: %w", err)
	}

	var sinceDate *string
	if snapshot != nil {
		asOf := snapshot.AsOfDate
		cb.SnapshotAsOfDate = &asOf
		cb.SnapshotClosing = snapshot.ClosingBalance
		sinceDate = &asOf
	}

	delta, err := s.repo.SumTransactionsSince(ctx, orgID, employeeID, lt.ID, sinceDate)
	if err != nil {
		return nil, fmt.Errorf("sum transactions: %w", err)
	}
	cb.DeltaSinceSnapshot = delta
	cb.Balance = cb.SnapshotClosing + delta
	cb.IsNegative = cb.Balance < 0
	return cb, nil
}

func (s *serviceImpl) GetCurrentBalance(ctx context.Context, orgID, employeeID, leaveTypeID string) (*CurrentBalance, error) {
	lt, err := s.repo.FindLeaveTypeByRef(ctx, orgID, leaveTypeID)
	if err != nil {
		return nil, fmt.Errorf("leave: GetCurrentBalance: %w", err)
	}
	if lt == nil {
		return nil, ErrLeaveTypeNotFound
	}
	return s.currentBalanceFor(ctx, orgID, employeeID, lt)
}

func (s *serviceImpl) ListCurrentBalances(ctx context.Context, orgID, employeeID string) ([]*CurrentBalance, error) {
	types, err := s.repo.FindAllLeaveTypes(ctx, orgID, true)
	if err != nil {
		return nil, fmt.Errorf("leave: ListCurrentBalances: %w", err)
	}
	out := make([]*CurrentBalance, 0, len(types))
	for _, lt := range types {
		cb, err := s.currentBalanceFor(ctx, orgID, employeeID, lt)
		if err != nil {
			return nil, fmt.Errorf("leave: ListCurrentBalances: %w", err)
		}
		out = append(out, cb)
	}
	return out, nil
}

func (s *serviceImpl) ListBalanceHistory(ctx context.Context, orgID, employeeID, leaveTypeID string, limit, offset int) (*BalanceHistoryResponse, error) {
	lt, err := s.repo.FindLeaveTypeByRef(ctx, orgID, leaveTypeID)
	if err != nil {
		return nil, fmt.Errorf("leave: ListBalanceHistory: %w", err)
	}
	if lt == nil {
		return nil, ErrLeaveTypeNotFound
	}
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	if offset < 0 {
		offset = 0
	}
	list, total, err := s.repo.FindBalanceHistory(ctx, orgID, employeeID, lt.ID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("leave: ListBalanceHistory: %w", err)
	}
	if list == nil {
		list = []*LeaveBalance{}
	}
	return &BalanceHistoryResponse{Balances: list, Total: total, Limit: limit, Offset: offset}, nil
}

func (s *serviceImpl) ListTransactions(ctx context.Context, orgID, employeeID string, filter TransactionFilter) (*TransactionListResponse, error) {
	filter.Normalise()
	list, err := s.repo.FindTransactions(ctx, orgID, employeeID, filter)
	if err != nil {
		return nil, fmt.Errorf("leave: ListTransactions: %w", err)
	}
	if list == nil {
		list = []*LeaveTransaction{}
	}
	total, err := s.repo.CountTransactions(ctx, orgID, employeeID, filter)
	if err != nil {
		return nil, fmt.Errorf("leave: ListTransactions: count: %w", err)
	}
	return &TransactionListResponse{Transactions: list, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

// ─────────────────────────────────────────────────────────
// Manual balance actions (adjustment, encashment)
// ─────────────────────────────────────────────────────────

func (s *serviceImpl) PostAdjustment(ctx context.Context, orgID, employeeID, leaveTypeID, actorID string, req PostAdjustmentRequest) (*LeaveTransaction, error) {
	if req.Days == 0 {
		return nil, ErrAdjustmentDaysZero
	}
	note := strings.TrimSpace(req.Note)
	if note == "" {
		return nil, ErrAdjustmentNoteRequired
	}
	lt, err := s.repo.FindLeaveTypeByRef(ctx, orgID, leaveTypeID)
	if err != nil {
		return nil, fmt.Errorf("leave: PostAdjustment: %w", err)
	}
	if lt == nil {
		return nil, ErrLeaveTypeNotFound
	}
	policy, err := s.repo.FindPolicyByLeaveType(ctx, orgID, lt.ID)
	if err != nil {
		return nil, fmt.Errorf("leave: PostAdjustment: policy: %w", err)
	}
	if policy == nil {
		return nil, ErrNoActivePolicy
	}

	t := &LeaveTransaction{
		OrgID: orgID, EmployeeID: employeeID, LeaveTypeID: lt.ID, PolicyID: policy.ID,
		TransactionType: TransactionAdjustment, Days: req.Days,
		TransactionDate: time.Now().Format(dateLayout),
		Note:            &note, CreatedBy: actorID,
	}
	if err := s.repo.CreateTransaction(ctx, t); err != nil {
		return nil, fmt.Errorf("leave: PostAdjustment: %w", err)
	}

	s.audit.Log(ctx, audit.EventHRMLeaveBalanceAdjusted, actorID, orgID, "", "", map[string]any{
		"employee_id": employeeID, "leave_type_id": lt.ID, "days": req.Days,
	})
	return t, nil
}

// PostEncashment records days encashed only — it never computes a currency
// amount (decision #3). encashment_rate_basis is stored config a future
// F&F phase reads; this phase does not evaluate it.
func (s *serviceImpl) PostEncashment(ctx context.Context, orgID, employeeID, leaveTypeID, actorID string, req PostEncashmentRequest) (*LeaveTransaction, error) {
	if req.Days <= 0 {
		return nil, ErrEncashmentDaysInvalid
	}
	lt, err := s.repo.FindLeaveTypeByRef(ctx, orgID, leaveTypeID)
	if err != nil {
		return nil, fmt.Errorf("leave: PostEncashment: %w", err)
	}
	if lt == nil {
		return nil, ErrLeaveTypeNotFound
	}
	policy, err := s.repo.FindPolicyByLeaveType(ctx, orgID, lt.ID)
	if err != nil {
		return nil, fmt.Errorf("leave: PostEncashment: policy: %w", err)
	}
	if policy == nil {
		return nil, ErrNoActivePolicy
	}
	if !policy.Encashable {
		return nil, ErrEncashmentNotAllowed
	}

	t := &LeaveTransaction{
		OrgID: orgID, EmployeeID: employeeID, LeaveTypeID: lt.ID, PolicyID: policy.ID,
		TransactionType: TransactionEncashment, Days: -float64(req.Days),
		TransactionDate: time.Now().Format(dateLayout),
		Note:            req.Note, CreatedBy: actorID,
	}
	if err := s.repo.CreateTransaction(ctx, t); err != nil {
		return nil, fmt.Errorf("leave: PostEncashment: %w", err)
	}

	s.audit.Log(ctx, audit.EventHRMLeaveEncashed, actorID, orgID, "", "", map[string]any{
		"employee_id": employeeID, "leave_type_id": lt.ID, "days": req.Days,
	})
	return t, nil
}

// ─────────────────────────────────────────────────────────
// Transactional write-path helpers for CreateRequest/ApproveRequest/
// CancelRequest/DeleteRequest — bypass Repository for these specific
// multi-statement writes, following promotions.Apply's exact existing
// pattern (raw SQL via a caller-scoped pgx.Tx). Only called when an active
// policy exists for the request's leave type; the no-policy path in
// service.go is untouched and goes through Repository as before.
// ─────────────────────────────────────────────────────────

// insertLedgerTx inserts one ledger row within an already-open tx.
func insertLedgerTx(ctx context.Context, tx pgx.Tx, orgID, employeeID, leaveTypeID, policyID string, txType TransactionType, days float64, txDate string, leaveRequestID *string, actorID string) error {
	const q = `
		INSERT INTO hrm_leave_transactions
		    (org_id, employee_id, leave_type_id, policy_id, transaction_type, days, transaction_date, leave_request_id, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7::date, $8, $9)`
	_, err := tx.Exec(ctx, q, orgID, employeeID, leaveTypeID, policyID, txType, days, txDate, leaveRequestID, actorID)
	if err != nil {
		return fmt.Errorf("insert %s transaction: %w", txType, err)
	}
	return nil
}

// createRequestWithUsage inserts a new leave request already in `approved`
// status and posts its usage debit atomically — used only by CreateRequest's
// auto-approve branch (lt.RequiresApproval=false) when a policy exists.
func (s *serviceImpl) createRequestWithUsage(ctx context.Context, lr *LeaveRequest, policy *LeavePolicy, actorID string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	const insertQ = `
		INSERT INTO hrm_leave_requests
		    (org_id, employee_id, leave_type_id, start_date, end_date, total_days, reason, status, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING ` + lrSelect

	created, err := scanLeaveRequest(tx.QueryRow(ctx, insertQ,
		lr.OrgID, lr.EmployeeID, lr.LeaveTypeID, lr.StartDate, lr.EndDate, lr.TotalDays,
		lr.Reason, lr.Status, lr.CreatedBy,
	))
	if err != nil {
		return fmt.Errorf("insert request: %w", err)
	}
	*lr = *created

	if err := insertLedgerTx(ctx, tx, lr.OrgID, lr.EmployeeID, lr.LeaveTypeID, policy.ID,
		TransactionUsage, -lr.TotalDays, time.Now().Format(dateLayout), &lr.ID, actorID); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// updateRequestWithLedger applies lr's already-mutated status/review fields
// (set by the caller before invoking this) and posts a ledger transaction
// atomically — used by ApproveRequest (usage debit) and CancelRequest
// (usage_reversal credit) when a policy exists.
func (s *serviceImpl) updateRequestWithLedger(ctx context.Context, lr *LeaveRequest, policy *LeavePolicy, txType TransactionType, days float64, actorID string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	const updateQ = `
		UPDATE hrm_leave_requests
		SET status = $1, reviewed_by = $2, reviewed_at = $3, review_note = $4, updated_at = NOW()
		WHERE id = $5 AND org_id = $6
		RETURNING ` + lrSelect

	updated, err := scanLeaveRequest(tx.QueryRow(ctx, updateQ,
		lr.Status, lr.ReviewedBy, lr.ReviewedAt, lr.ReviewNote, lr.ID, lr.OrgID,
	))
	if err != nil {
		return fmt.Errorf("update request: %w", err)
	}
	if updated == nil {
		return ErrLeaveRequestNotFound
	}
	*lr = *updated

	if err := insertLedgerTx(ctx, tx, lr.OrgID, lr.EmployeeID, lr.LeaveTypeID, policy.ID,
		txType, days, time.Now().Format(dateLayout), &lr.ID, actorID); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// deleteRequestWithReversal posts a usage_reversal (dated at delete time)
// before hard-deleting an approved, usage-posted request — the ledger row
// survives with leave_request_id nulled by ON DELETE SET NULL, so the
// history stays intact even once the request itself is gone.
func (s *serviceImpl) deleteRequestWithReversal(ctx context.Context, lr *LeaveRequest, policy *LeavePolicy, actorID string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := insertLedgerTx(ctx, tx, lr.OrgID, lr.EmployeeID, lr.LeaveTypeID, policy.ID,
		TransactionUsageReversal, lr.TotalDays, time.Now().Format(dateLayout), &lr.ID, actorID); err != nil {
		return err
	}

	cmd, err := tx.Exec(ctx, `DELETE FROM hrm_leave_requests WHERE org_id = $1 AND id = $2`, lr.OrgID, lr.ID)
	if err != nil {
		return fmt.Errorf("delete request: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrLeaveRequestNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// ─────────────────────────────────────────────────────────
// Scheduler jobs
// ─────────────────────────────────────────────────────────

// activeEmployeeRef is the minimal per-employee data the two jobs need,
// fetched inline against s.db rather than through Repository — the same
// convention attendance.RunAbsenceSweep and warnings.serviceImpl already
// use for ad-hoc "all active employees in an org" queries.
type activeEmployeeRef struct {
	ID       string
	HireDate time.Time
}

func (s *serviceImpl) activeEmployeesForAccrual(ctx context.Context, orgID string) ([]activeEmployeeRef, error) {
	rows, err := s.db.Query(ctx,
		`SELECT e.id::text, e.hire_date
		FROM hrm_employees e
		JOIN hrm_employee_statuses st ON st.id = e.status_id
		WHERE e.org_id = $1 AND st.category = 'active'`,
		orgID)
	if err != nil {
		return nil, fmt.Errorf("list active employees: %w", err)
	}
	defer rows.Close()

	var list []activeEmployeeRef
	for rows.Next() {
		var e activeEmployeeRef
		if err := rows.Scan(&e.ID, &e.HireDate); err != nil {
			return nil, fmt.Errorf("scan employee: %w", err)
		}
		list = append(list, e)
	}
	return list, rows.Err()
}

// isAccrualDue is a pure function (no DB) so the date-branching per
// accrual_method is table-driven-testable without Postgres. v1 scope
// boundaries, stated not assumed: no first-year proration for annual, no
// partial-month proration for monthly on new hires, annual fires on the
// POLICY's creation anniversary (shared by every employee under it), not
// each employee's individual hire-date anniversary.
func isAccrualDue(method AccrualMethod, policyCreatedAt, hireDate, asOfDate time.Time, alreadyAccrued bool) bool {
	switch method {
	case AccrualMonthly:
		if asOfDate.Day() != 1 {
			return false
		}
		policyMonth := time.Date(policyCreatedAt.Year(), policyCreatedAt.Month(), 1, 0, 0, 0, 0, time.UTC)
		asOfMonth := time.Date(asOfDate.Year(), asOfDate.Month(), 1, 0, 0, 0, 0, time.UTC)
		return asOfMonth.After(policyMonth)
	case AccrualAnnual:
		return asOfDate.Month() == policyCreatedAt.Month() && asOfDate.Day() == policyCreatedAt.Day()
	case AccrualOnJoining:
		if alreadyAccrued {
			return false
		}
		hireDay := time.Date(hireDate.Year(), hireDate.Month(), hireDate.Day(), 0, 0, 0, 0, time.UTC)
		policyDay := time.Date(policyCreatedAt.Year(), policyCreatedAt.Month(), policyCreatedAt.Day(), 0, 0, 0, 0, time.UTC)
		return !hireDay.Before(policyDay)
	default:
		return false
	}
}

// postAccrualIfDue posts one accrual transaction for (policy, emp) if due on
// asOfDate. Idempotent two ways: a pre-check against the ledger, and the
// uq_hrm_lvx_accrual_period unique index as the hard backstop — required
// because the scheduler's Redis lock has no renewal, so a run exceeding its
// TTL could double-execute.
func (s *serviceImpl) postAccrualIfDue(ctx context.Context, orgID string, policy *LeavePolicy, emp activeEmployeeRef, asOfDate time.Time, actorID string) bool {
	var alreadyAccrued bool
	if policy.AccrualMethod == AccrualOnJoining {
		exists, err := s.repo.ExistsAnyAccrual(ctx, emp.ID, policy.LeaveTypeID)
		if err != nil {
			return false
		}
		alreadyAccrued = exists
	}

	if !isAccrualDue(policy.AccrualMethod, policy.CreatedAt, emp.HireDate, asOfDate, alreadyAccrued) {
		return false
	}

	dateStr := asOfDate.Format(dateLayout)
	exists, err := s.repo.ExistsAccrualForPeriod(ctx, emp.ID, policy.LeaveTypeID, dateStr)
	if err != nil || exists {
		return false
	}

	t := &LeaveTransaction{
		OrgID: orgID, EmployeeID: emp.ID, LeaveTypeID: policy.LeaveTypeID, PolicyID: policy.ID,
		TransactionType: TransactionAccrual, Days: policy.AccrualRate,
		TransactionDate: dateStr, CreatedBy: actorID,
	}
	if err := s.repo.CreateTransaction(ctx, t); err != nil {
		return false
	}
	return true
}

// writeMonthlySnapshot rolls this period's ledger activity into one
// hrm_leave_balances row per (employee, policy). Runs once per calendar
// month (the 1st), regardless of whether an accrual event fired this
// period — a month with zero accrual (e.g. an annual-method policy in a
// non-anniversary month) still gets a snapshot with Accrued=0, so balance
// history stays contiguous month over month.
// writeMonthlySnapshot is invoked on the 1st of month M (asOfDate) and
// closes out the PERIOD THAT JUST ENDED — month M-1 — not month M itself.
// A snapshot's AsOfDate is the exclusive upper bound of what it covers
// (SumTransactionsSince/SumTransactionsByType's doc comments), so the row
// written here with AsOfDate=asOfDate (e.g. Feb 1) correctly represents
// "closing state of January" and is labeled PeriodMonth=1 (January), not 2
// — labeling it by asOfDate's own month would conflate this run's brand-new
// accrual (also dated asOfDate, e.g. February's grant posted Feb 1) with
// January's closing number, when it hasn't happened yet from January's
// point of view.
func (s *serviceImpl) writeMonthlySnapshot(ctx context.Context, orgID string, policy *LeavePolicy, emp activeEmployeeRef, asOfDate time.Time, actorID string) {
	periodDate := asOfDate.AddDate(0, -1, 0)

	existing, err := s.repo.FindLatestBalanceSnapshot(ctx, orgID, emp.ID, policy.LeaveTypeID)
	if err != nil {
		return
	}
	if existing != nil && existing.PeriodYear == periodDate.Year() && existing.PeriodMonth == int(periodDate.Month()) {
		return // already written this period — idempotent no-op
	}

	var opening float64
	var sinceDate *string
	if existing != nil {
		opening = existing.ClosingBalance
		asOf := existing.AsOfDate
		sinceDate = &asOf
	}

	sums, err := s.repo.SumTransactionsByType(ctx, orgID, emp.ID, policy.LeaveTypeID, sinceDate, asOfDate.Format(dateLayout))
	if err != nil {
		return
	}

	closing := opening + sums.Accrued - sums.Taken - sums.Encashed + sums.CarriedForward + sums.Adjusted

	b := &LeaveBalance{
		OrgID: orgID, EmployeeID: emp.ID, LeaveTypeID: policy.LeaveTypeID, PolicyID: policy.ID,
		PeriodYear: periodDate.Year(), PeriodMonth: int(periodDate.Month()), AsOfDate: asOfDate.Format(dateLayout),
		OpeningBalance: opening, Accrued: sums.Accrued, Taken: sums.Taken, Encashed: sums.Encashed,
		CarriedForward: sums.CarriedForward, Adjusted: sums.Adjusted, ClosingBalance: closing,
		CreatedBy: actorID,
	}
	_ = s.repo.CreateBalanceSnapshot(ctx, b)
}

// RunAccrual posts due accrual transactions and, on the 1st of the month,
// rolls the period into a hrm_leave_balances snapshot per (employee,
// policy). Registered as the "leave.accrue_and_snapshot" scheduler job,
// daily — daily (not monthly-only) because on_joining grants need to fire
// promptly for new hires; monthly/annual policies simply no-op on days
// they're not due.
func (s *serviceImpl) RunAccrual(ctx context.Context, orgID, systemUserID, asOfDate string) (int, error) {
	today, err := time.Parse(dateLayout, asOfDate)
	if err != nil {
		return 0, fmt.Errorf("leave: RunAccrual: invalid asOfDate: %w", err)
	}

	policies, err := s.repo.FindAllPolicies(ctx, orgID)
	if err != nil {
		return 0, fmt.Errorf("leave: RunAccrual: list policies: %w", err)
	}
	employees, err := s.activeEmployeesForAccrual(ctx, orgID)
	if err != nil {
		return 0, fmt.Errorf("leave: RunAccrual: list employees: %w", err)
	}

	isSnapshotDay := today.Day() == 1
	total := 0
	for _, policy := range policies {
		if !policy.IsActive {
			continue
		}
		for _, emp := range employees {
			if s.postAccrualIfDue(ctx, orgID, policy, emp, today, systemUserID) {
				total++
			}
			if isSnapshotDay {
				s.writeMonthlySnapshot(ctx, orgID, policy, emp, today, systemUserID)
			}
		}
	}
	return total, nil
}

// RunCarryForward runs annually (Jan 1) and forfeits any balance in excess
// of a policy's carry_forward_cap. There are no year-scoped buckets in this
// running-ledger design, so carry-forward reduces to forfeiture-only: an
// uncapped or under-cap balance simply persists into the new year untouched
// — nothing needs to be credited for it. A negative balance is never
// forfeited (max(0, ...) — nothing to take away). Registered as the
// "leave.year_end_carry_forward" scheduler job.
func (s *serviceImpl) RunCarryForward(ctx context.Context, orgID, systemUserID, asOfDate string) (int, error) {
	today, err := time.Parse(dateLayout, asOfDate)
	if err != nil {
		return 0, fmt.Errorf("leave: RunCarryForward: invalid asOfDate: %w", err)
	}

	policies, err := s.repo.FindAllPolicies(ctx, orgID)
	if err != nil {
		return 0, fmt.Errorf("leave: RunCarryForward: list policies: %w", err)
	}
	employees, err := s.activeEmployeesForAccrual(ctx, orgID)
	if err != nil {
		return 0, fmt.Errorf("leave: RunCarryForward: list employees: %w", err)
	}

	// The forfeiture transaction itself is dated Dec 31 of the year just
	// ending, for business readability — but the BALANCE query boundary
	// passed to forfeitIfOverCap is asOfDate (Jan 1) itself, the correct
	// exclusive upper bound covering all of the closing year inclusive (see
	// SumTransactionsSince's doc comment). Using Dec 31 as the query bound
	// would incorrectly exclude anything dated exactly Dec 31.
	closingYearEnd := time.Date(today.Year()-1, time.December, 31, 0, 0, 0, 0, time.UTC)
	forfeitureDate := closingYearEnd.Format(dateLayout)

	total := 0
	for _, policy := range policies {
		if !policy.IsActive || !policy.CarryForwardEnabled {
			continue
		}
		for _, emp := range employees {
			if s.forfeitIfOverCap(ctx, orgID, policy, emp, asOfDate, forfeitureDate, systemUserID) {
				total++
			}
		}
	}
	return total, nil
}

// forfeitIfOverCap is idempotent via the uq_hrm_lvx_forfeiture_period unique
// index alone (no service-layer pre-check) — a second call for the same
// (employee, leave_type, date) simply fails the INSERT and returns false,
// which is sufficient given this job runs once a year. balanceBoundary is
// the exclusive upper bound for the balance query (asOfDate, Jan 1);
// forfeitureDate is the transaction's own business-readable date (Dec 31).
func (s *serviceImpl) forfeitIfOverCap(ctx context.Context, orgID string, policy *LeavePolicy, emp activeEmployeeRef, balanceBoundary, forfeitureDate, actorID string) bool {
	balance, err := s.balanceAsOf(ctx, orgID, emp.ID, policy.LeaveTypeID, balanceBoundary)
	if err != nil {
		return false
	}

	cap := policy.CarryForwardCap
	var forfeit float64
	if cap != nil {
		forfeit = balance - *cap
	}
	if forfeit <= 0 {
		return false
	}

	t := &LeaveTransaction{
		OrgID: orgID, EmployeeID: emp.ID, LeaveTypeID: policy.LeaveTypeID, PolicyID: policy.ID,
		TransactionType: TransactionForfeiture, Days: -forfeit,
		TransactionDate: forfeitureDate, CreatedBy: actorID,
	}
	if err := s.repo.CreateTransaction(ctx, t); err != nil {
		// uq_hrm_lvx_forfeiture_period rejects a double-post here — the
		// scheduler's Redis lock has no renewal, so this is the hard backstop.
		return false
	}
	return true
}
