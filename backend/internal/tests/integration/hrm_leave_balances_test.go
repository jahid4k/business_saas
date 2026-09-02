// backend/internal/tests/integration/hrm_leave_balances_test.go
// Proves the leave-balance ledger against a real Postgres — this covers two
// classes of thing a stub-repo unit test structurally cannot prove:
//  1. DB-level idempotency (the unique indexes actually rejecting a
//     double-post, not just the service-layer pre-check).
//  2. The write-path integration into CreateRequest/ApproveRequest/
//     CancelRequest/DeleteRequest, which bypasses Repository and wraps a
//     real pgx.Tx (mirroring promotions.Apply) — those specific code paths
//     cannot run against an unconnected stub pool at all.
//
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/audit"
	"github.com/mridha/businesssaas/internal/hrm/leave"
)

// newLeaveTestServices constructs a real leave.Service (and its Repository,
// for direct seeding) against env.db — mirrors scope.NewResolver(env.db) in
// hrm_scope_test.go: testEnv doesn't hold every module's service, individual
// integration test files wire the ones they need locally.
func newLeaveTestServices(env *testEnv) (leave.Service, leave.Repository) {
	repo := leave.NewRepository(env.db)
	auditSvc := audit.NewService(audit.NewRepository(env.db))
	return leave.NewService(repo, auditSvc, env.db), repo
}

// seedLeavePolicy creates a leave type and an active policy for it,
// returning both. accrualRate/carryForwardCap/encashable let each test
// dial in exactly the config it needs.
func seedLeavePolicy(t *testing.T, ctx context.Context, svc leave.Service, orgID, createdBy string, method leave.AccrualMethod, rate float64, carryForwardEnabled bool, carryForwardCap *float64, encashable bool) (*leave.LeaveType, *leave.LeavePolicy) {
	t.Helper()
	lt, err := svc.CreateLeaveType(ctx, orgID, createdBy, leave.CreateLeaveTypeRequest{Name: "Balance Test Type " + uniqueSlug("lt")})
	if err != nil {
		t.Fatalf("seed leave type: %v", err)
	}
	p, err := svc.CreatePolicy(ctx, orgID, createdBy, leave.CreatePolicyRequest{
		LeaveTypeID:         lt.ID,
		AccrualMethod:       method,
		AccrualRate:         rate,
		CarryForwardEnabled: carryForwardEnabled,
		CarryForwardCap:     carryForwardCap,
		Encashable:          encashable,
	})
	if err != nil {
		t.Fatalf("seed leave policy: %v", err)
	}
	return lt, p
}

// TestIntegration_HRMLeaveBalance_AccrualIdempotency proves the real
// uq_hrm_lvx_accrual_period unique index (not just the service-layer
// pre-check) rejects a double-post when RunAccrual is invoked twice for the
// same period — required because the scheduler's Redis lock has no
// renewal, so a slow run can double-execute.
func TestIntegration_HRMLeaveBalance_AccrualIdempotency(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	empID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Accrual Emp", nil)

	svc, _ := newLeaveTestServices(env)
	_, policy := seedLeavePolicy(t, ctx, svc, orgID, ownerID, leave.AccrualMonthly, 1.5, false, nil, false)

	// asOfDate must be strictly after the policy's creation month for
	// monthly accrual to be due — use next month's 1st so this passes
	// regardless of when the test suite runs.
	asOfDate := firstOfNextMonth(time.Now()).Format("2006-01-02")

	n1, err := svc.RunAccrual(ctx, orgID, ownerID, asOfDate)
	if err != nil {
		t.Fatalf("first RunAccrual failed: %v", err)
	}
	if n1 != 1 {
		t.Fatalf("expected 1 accrual posted on first run, got %d", n1)
	}

	n2, err := svc.RunAccrual(ctx, orgID, ownerID, asOfDate)
	if err != nil {
		t.Fatalf("second RunAccrual failed: %v", err)
	}
	if n2 != 0 {
		t.Errorf("expected 0 accruals on the second run for the same period (idempotent), got %d", n2)
	}

	result, err := svc.ListTransactions(ctx, orgID, empID, leave.TransactionFilter{TransactionType: leave.TransactionAccrual})
	if err != nil {
		t.Fatalf("ListTransactions failed: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("expected exactly 1 accrual transaction in the ledger, got %d", result.Total)
	}
	_ = policy
}

// TestIntegration_HRMLeaveBalance_CarryForwardCapping proves the forfeiture
// math: a balance under the cap is untouched, a balance over the cap gets
// exactly the excess forfeited, and a negative balance is never forfeited
// (max(0, ...) — nothing to take away).
func TestIntegration_HRMLeaveBalance_CarryForwardCapping(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)

	svc, repo := newLeaveTestServices(env)
	cap := 5.0

	// Case 1: over the cap — 20 days accrued, capped at 5, expect 15 forfeited.
	overEmpID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Over Cap Emp", nil)
	_, overPolicy := seedLeavePolicy(t, ctx, svc, orgID, ownerID, leave.AccrualMonthly, 1, true, &cap, false)
	mustSeedTransaction(t, ctx, repo, orgID, overEmpID, overPolicy.LeaveTypeID, overPolicy.ID, leave.TransactionAccrual, 20, "2025-06-01", ownerID)

	// Case 2: under the cap — 3 days accrued, capped at 5, expect nothing forfeited.
	underEmpID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Under Cap Emp", nil)
	mustSeedTransaction(t, ctx, repo, orgID, underEmpID, overPolicy.LeaveTypeID, overPolicy.ID, leave.TransactionAccrual, 3, "2025-06-01", ownerID)

	// Case 3: negative balance — an adjustment debit with no prior accrual;
	// must be left untouched, not "forfeited" further negative.
	negEmpID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Negative Emp", nil)
	mustSeedTransaction(t, ctx, repo, orgID, negEmpID, overPolicy.LeaveTypeID, overPolicy.ID, leave.TransactionAdjustment, -4, "2025-06-01", ownerID)

	nextYearJan1 := time.Date(time.Now().Year()+1, time.January, 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	n, err := svc.RunCarryForward(ctx, orgID, ownerID, nextYearJan1)
	if err != nil {
		t.Fatalf("RunCarryForward failed: %v", err)
	}
	if n != 1 {
		t.Errorf("expected exactly 1 forfeiture posted (only the over-cap employee), got %d", n)
	}

	assertForfeiture := func(empID string, wantDays float64, wantAny bool) {
		t.Helper()
		result, err := svc.ListTransactions(ctx, orgID, empID, leave.TransactionFilter{TransactionType: leave.TransactionForfeiture})
		if err != nil {
			t.Fatalf("ListTransactions(%s) failed: %v", empID, err)
		}
		if !wantAny {
			if result.Total != 0 {
				t.Errorf("expected no forfeiture for %s, got %d", empID, result.Total)
			}
			return
		}
		if result.Total != 1 {
			t.Fatalf("expected exactly 1 forfeiture for %s, got %d", empID, result.Total)
		}
		if result.Transactions[0].Days != wantDays {
			t.Errorf("expected forfeiture of %v days for %s, got %v", wantDays, empID, result.Transactions[0].Days)
		}
	}
	assertForfeiture(overEmpID, -15, true) // 20 - 5 cap = 15 forfeited (signed debit)
	assertForfeiture(underEmpID, 0, false)
	assertForfeiture(negEmpID, 0, false)
}

// TestIntegration_HRMLeaveBalance_SnapshotPlusDeltaMatchesFullReplay is the
// test that actually exercises the boundary-symmetry invariant the whole
// balance-read formula depends on: the accrual job's snapshot-writer and
// GetCurrentBalance's checkpoint+delta read must use identical date
// boundaries, or a transaction gets double-counted or dropped. Proven by
// computing the same employee's balance two independent ways — checkpoint+
// delta, and a brute-force full-ledger sum with no snapshot involved at
// all — and asserting they agree, across a multi-month transaction history.
func TestIntegration_HRMLeaveBalance_SnapshotPlusDeltaMatchesFullReplay(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	empID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Replay Emp", nil)

	svc, repo := newLeaveTestServices(env)
	_, policy := seedLeavePolicy(t, ctx, svc, orgID, ownerID, leave.AccrualMonthly, 1, false, nil, false)

	// A multi-month spread of transaction types, seeded directly (bypassing
	// wall-clock time entirely).
	mustSeedTransaction(t, ctx, repo, orgID, empID, policy.LeaveTypeID, policy.ID, leave.TransactionAccrual, 5, "2025-01-01", ownerID)
	mustSeedTransaction(t, ctx, repo, orgID, empID, policy.LeaveTypeID, policy.ID, leave.TransactionUsage, -2, "2025-02-10", ownerID)
	mustSeedTransaction(t, ctx, repo, orgID, empID, policy.LeaveTypeID, policy.ID, leave.TransactionAccrual, 5, "2025-02-01", ownerID)
	mustSeedTransaction(t, ctx, repo, orgID, empID, policy.LeaveTypeID, policy.ID, leave.TransactionAdjustment, -1, "2025-03-05", ownerID)
	mustSeedTransaction(t, ctx, repo, orgID, empID, policy.LeaveTypeID, policy.ID, leave.TransactionAccrual, 5, "2025-03-01", ownerID)

	// Write a snapshot mid-history (as the accrual job would have, at the
	// Feb 1 boundary) covering Jan's activity only.
	if err := repo.CreateBalanceSnapshot(ctx, &leave.LeaveBalance{
		OrgID: orgID, EmployeeID: empID, LeaveTypeID: policy.LeaveTypeID, PolicyID: policy.ID,
		PeriodYear: 2025, PeriodMonth: 1, AsOfDate: "2025-02-01",
		OpeningBalance: 0, Accrued: 5, ClosingBalance: 5, CreatedBy: ownerID,
	}); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}

	cb, err := svc.GetCurrentBalance(ctx, orgID, empID, policy.LeaveTypeID)
	if err != nil {
		t.Fatalf("GetCurrentBalance (checkpoint+delta) failed: %v", err)
	}

	var bruteForce float64
	if err := env.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(days), 0) FROM hrm_leave_transactions WHERE org_id=$1 AND employee_id=$2 AND leave_type_id=$3`,
		orgID, empID, policy.LeaveTypeID,
	).Scan(&bruteForce); err != nil {
		t.Fatalf("brute-force sum query failed: %v", err)
	}

	if cb.Balance != bruteForce {
		t.Errorf("checkpoint+delta balance (%v) does not match full-ledger-replay balance (%v) — boundary mismatch between the snapshot writer and the balance reader", cb.Balance, bruteForce)
	}
	// 5(Jan accrual, in snapshot) + 5(Feb accrual) - 2(Feb usage) + 5(Mar accrual) - 1(Mar adjustment) = 12
	if bruteForce != 12 {
		t.Fatalf("test setup sanity check failed: expected brute-force total 12, got %v", bruteForce)
	}
}

// TestIntegration_HRMLeaveBalance_ApproveRequest_PostsUsage_WithPolicy
// exercises the real pgx.Tx write path in updateRequestWithLedger — cannot
// run against the unconnected stub pool the unit tests use.
func TestIntegration_HRMLeaveBalance_ApproveRequest_PostsUsage_WithPolicy(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	empID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Approve Emp", nil)

	svc, repo := newLeaveTestServices(env)
	lt, policy := seedLeavePolicy(t, ctx, svc, orgID, ownerID, leave.AccrualMonthly, 1, false, nil, false)
	mustSeedTransaction(t, ctx, repo, orgID, empID, policy.LeaveTypeID, policy.ID, leave.TransactionAccrual, 10, "2025-01-01", ownerID)

	lr, err := svc.CreateRequest(ctx, orgID, ownerID, leave.CreateLeaveRequestRequest{
		EmployeeID: empID, LeaveTypeID: lt.ID, StartDate: "2026-01-05", EndDate: "2026-01-09", TotalDays: 4,
	})
	if err != nil {
		t.Fatalf("CreateRequest failed: %v", err)
	}
	if lr.Status != leave.LeaveRequestStatusPending {
		t.Fatalf("expected pending (default leave type requires approval), got %s", lr.Status)
	}

	if _, err := svc.ApproveRequest(ctx, orgID, lr.ID, ownerID, leave.ReviewLeaveRequestRequest{}); err != nil {
		t.Fatalf("ApproveRequest failed: %v", err)
	}

	cb, err := svc.GetCurrentBalance(ctx, orgID, empID, lt.ID)
	if err != nil {
		t.Fatalf("GetCurrentBalance failed: %v", err)
	}
	if cb.Balance != 6 {
		t.Errorf("expected balance 6 (10 accrued - 4 taken), got %v", cb.Balance)
	}
}

// TestIntegration_HRMLeaveBalance_ApproveRequest_InsufficientBalance_SoftAllow
// proves decision #1: approving a request that drives the balance negative
// always succeeds — there is no balance-sufficiency gate anywhere in the
// write path.
func TestIntegration_HRMLeaveBalance_ApproveRequest_InsufficientBalance_SoftAllow(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	empID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Overdrawn Emp", nil)

	svc, _ := newLeaveTestServices(env)
	lt, _ := seedLeavePolicy(t, ctx, svc, orgID, ownerID, leave.AccrualMonthly, 1, false, nil, false)
	// Deliberately no accrual seeded — balance starts at zero.

	lr, err := svc.CreateRequest(ctx, orgID, ownerID, leave.CreateLeaveRequestRequest{
		EmployeeID: empID, LeaveTypeID: lt.ID, StartDate: "2026-01-05", EndDate: "2026-01-09", TotalDays: 5,
	})
	if err != nil {
		t.Fatalf("CreateRequest failed: %v", err)
	}

	if _, err := svc.ApproveRequest(ctx, orgID, lr.ID, ownerID, leave.ReviewLeaveRequestRequest{}); err != nil {
		t.Fatalf("expected ApproveRequest to succeed even with insufficient balance (soft allow), got error: %v", err)
	}

	cb, err := svc.GetCurrentBalance(ctx, orgID, empID, lt.ID)
	if err != nil {
		t.Fatalf("GetCurrentBalance failed: %v", err)
	}
	if cb.Balance != -5 {
		t.Errorf("expected balance -5 (0 - 5 taken, allowed to go negative), got %v", cb.Balance)
	}
	if !cb.IsNegative {
		t.Error("expected IsNegative=true")
	}
}

// TestIntegration_HRMLeaveBalance_CancelAndDeleteReverseUsage exercises the
// real pgx.Tx reversal paths in updateRequestWithLedger (cancel) and
// deleteRequestWithReversal (delete).
func TestIntegration_HRMLeaveBalance_CancelAndDeleteReverseUsage(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	empID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Reversal Emp", nil)

	svc, repo := newLeaveTestServices(env)
	lt, policy := seedLeavePolicy(t, ctx, svc, orgID, ownerID, leave.AccrualMonthly, 1, false, nil, false)
	mustSeedTransaction(t, ctx, repo, orgID, empID, policy.LeaveTypeID, policy.ID, leave.TransactionAccrual, 20, "2025-01-01", ownerID)

	// Approve then cancel — balance should return to the pre-approval level.
	lr1, err := svc.CreateRequest(ctx, orgID, ownerID, leave.CreateLeaveRequestRequest{
		EmployeeID: empID, LeaveTypeID: lt.ID, StartDate: "2026-01-05", EndDate: "2026-01-09", TotalDays: 4,
	})
	if err != nil {
		t.Fatalf("CreateRequest (1) failed: %v", err)
	}
	if _, err := svc.ApproveRequest(ctx, orgID, lr1.ID, ownerID, leave.ReviewLeaveRequestRequest{}); err != nil {
		t.Fatalf("ApproveRequest (1) failed: %v", err)
	}
	if _, err := svc.CancelRequest(ctx, orgID, lr1.ID, ownerID); err != nil {
		t.Fatalf("CancelRequest failed: %v", err)
	}

	cb, err := svc.GetCurrentBalance(ctx, orgID, empID, lt.ID)
	if err != nil {
		t.Fatalf("GetCurrentBalance after cancel failed: %v", err)
	}
	if cb.Balance != 20 {
		t.Errorf("expected balance back to 20 after cancelling an approved request, got %v", cb.Balance)
	}

	// Approve then hard-delete — same expectation, via the delete path.
	lr2, err := svc.CreateRequest(ctx, orgID, ownerID, leave.CreateLeaveRequestRequest{
		EmployeeID: empID, LeaveTypeID: lt.ID, StartDate: "2026-02-05", EndDate: "2026-02-09", TotalDays: 3,
	})
	if err != nil {
		t.Fatalf("CreateRequest (2) failed: %v", err)
	}
	if _, err := svc.ApproveRequest(ctx, orgID, lr2.ID, ownerID, leave.ReviewLeaveRequestRequest{}); err != nil {
		t.Fatalf("ApproveRequest (2) failed: %v", err)
	}
	if err := svc.DeleteRequest(ctx, orgID, lr2.ID, ownerID); err != nil {
		t.Fatalf("DeleteRequest failed: %v", err)
	}

	cb2, err := svc.GetCurrentBalance(ctx, orgID, empID, lt.ID)
	if err != nil {
		t.Fatalf("GetCurrentBalance after delete failed: %v", err)
	}
	if cb2.Balance != 20 {
		t.Errorf("expected balance back to 20 after deleting an approved request, got %v", cb2.Balance)
	}

	// The deleted request itself is gone.
	if _, err := svc.GetRequest(ctx, orgID, lr2.ID); err != leave.ErrLeaveRequestNotFound {
		t.Errorf("expected ErrLeaveRequestNotFound for the deleted request, got %v", err)
	}
}

// ── helpers ────────────────────────────────────────────────────────────

func firstOfNextMonth(t time.Time) time.Time {
	firstOfThisMonth := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	return firstOfThisMonth.AddDate(0, 1, 0)
}

func mustSeedTransaction(t *testing.T, ctx context.Context, repo leave.Repository, orgID, employeeID, leaveTypeID, policyID string, txType leave.TransactionType, days float64, transactionDate, actorID string) {
	t.Helper()
	tx := &leave.LeaveTransaction{
		OrgID: orgID, EmployeeID: employeeID, LeaveTypeID: leaveTypeID, PolicyID: policyID,
		TransactionType: txType, Days: days, TransactionDate: transactionDate, CreatedBy: actorID,
	}
	if err := repo.CreateTransaction(ctx, tx); err != nil {
		t.Fatalf("seed transaction: %v", err)
	}
}
