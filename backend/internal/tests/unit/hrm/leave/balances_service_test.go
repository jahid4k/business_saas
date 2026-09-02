// backend/internal/tests/unit/hrm/leave/balances_service_test.go
package leave_test

import (
	"context"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/hrm/leave"
)

// Note on scope: CreatePolicy (incl. its backfill call), PostAdjustment,
// PostEncashment, and every balance/transaction read path only ever touch
// Repository — they're fully covered here with the stub. The write-path
// integration into CreateRequest/ApproveRequest/CancelRequest/DeleteRequest
// (usage posting and reversal when a policy exists) bypasses Repository and
// wraps a real pgx.Tx (mirroring promotions.Apply), so those specific
// scenarios — including the insufficient-balance soft-allow behavior — are
// only provable against a real Postgres connection and live in
// internal/tests/integration/hrm_leave_balances_test.go instead.

func newLeaveTypeAndOrg() (orgID, createdBy string) {
	return "org_bal_1", "user_bal_1"
}

func TestCreatePolicy_HappyPath(t *testing.T) {
	repo := newStubRepo()
	svc := leave.NewService(repo, &mockAudit{}, newDummyPool())
	ctx := context.Background()
	orgID, createdBy := newLeaveTypeAndOrg()

	lt, err := svc.CreateLeaveType(ctx, orgID, createdBy, leave.CreateLeaveTypeRequest{Name: "Annual"})
	if err != nil {
		t.Fatalf("CreateLeaveType failed: %v", err)
	}

	cap := 5.0
	basis := leave.EncashmentBasisBasicPay
	p, err := svc.CreatePolicy(ctx, orgID, createdBy, leave.CreatePolicyRequest{
		LeaveTypeID:         lt.ID,
		AccrualMethod:       leave.AccrualMonthly,
		AccrualRate:         1.5,
		CarryForwardEnabled: true,
		CarryForwardCap:     &cap,
		Encashable:          true,
		EncashmentRateBasis: &basis,
	})
	if err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}
	if p.LeaveTypeID != lt.ID {
		t.Errorf("expected LeaveTypeID %s, got %s", lt.ID, p.LeaveTypeID)
	}
	if !p.IsActive {
		t.Error("expected new policy to be active")
	}
	if p.CarryForwardCap == nil || *p.CarryForwardCap != 5.0 {
		t.Errorf("expected CarryForwardCap 5.0, got %v", p.CarryForwardCap)
	}
}

func TestCreatePolicy_InvalidAccrualMethod(t *testing.T) {
	repo := newStubRepo()
	svc := leave.NewService(repo, &mockAudit{}, newDummyPool())
	ctx := context.Background()
	orgID, createdBy := newLeaveTypeAndOrg()

	lt, _ := svc.CreateLeaveType(ctx, orgID, createdBy, leave.CreateLeaveTypeRequest{Name: "Sick"})
	_, err := svc.CreatePolicy(ctx, orgID, createdBy, leave.CreatePolicyRequest{
		LeaveTypeID:   lt.ID,
		AccrualMethod: leave.AccrualMethod("weekly"), // not a valid method
	})
	if err != leave.ErrInvalidAccrualMethod {
		t.Errorf("expected ErrInvalidAccrualMethod, got %v", err)
	}
}

func TestCreatePolicy_DuplicateActiveConflict(t *testing.T) {
	repo := newStubRepo()
	svc := leave.NewService(repo, &mockAudit{}, newDummyPool())
	ctx := context.Background()
	orgID, createdBy := newLeaveTypeAndOrg()

	lt, _ := svc.CreateLeaveType(ctx, orgID, createdBy, leave.CreateLeaveTypeRequest{Name: "Casual"})
	req := leave.CreatePolicyRequest{LeaveTypeID: lt.ID, AccrualMethod: leave.AccrualMonthly, AccrualRate: 1}

	if _, err := svc.CreatePolicy(ctx, orgID, createdBy, req); err != nil {
		t.Fatalf("first CreatePolicy failed: %v", err)
	}
	_, err := svc.CreatePolicy(ctx, orgID, createdBy, req)
	if err != leave.ErrPolicyAlreadyExists {
		t.Errorf("expected ErrPolicyAlreadyExists, got %v", err)
	}
}

// TestCreatePolicy_BackfillsHistoricalUsage seeds pre-existing approved
// requests directly into the stub (simulating history from before this
// leave type had a policy), then asserts CreatePolicy replays exactly one
// 'usage' transaction per historical request, dated at each request's
// original approval date — and leaves requests for OTHER leave types, or
// non-approved requests, untouched (decision #2: replay taken-days only).
func TestCreatePolicy_BackfillsHistoricalUsage(t *testing.T) {
	repo := newStubRepo()
	svc := leave.NewService(repo, &mockAudit{}, newDummyPool())
	ctx := context.Background()
	orgID, createdBy := newLeaveTypeAndOrg()

	lt, _ := svc.CreateLeaveType(ctx, orgID, createdBy, leave.CreateLeaveTypeRequest{Name: "Backfill Type"})
	// Seeded directly with an explicit, guaranteed-distinct ID rather than a
	// second back-to-back svc.CreateLeaveType call — the stub's fake-ID
	// generator uses millisecond-precision timestamps, which two calls this
	// close together can collide on.
	otherLt := &leave.LeaveType{ID: "lt_other_fixed", PublicID: "lt_other_fixed", OrgID: orgID, Name: "Other Type", IsActive: true, CreatedBy: createdBy}
	repo.leaveTypes[otherLt.ID] = otherLt

	reviewedAt := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	approvedReq := &leave.LeaveRequest{
		ID: "req_hist_1", PublicID: "req_hist_1", OrgID: orgID, EmployeeID: "emp_1",
		LeaveTypeID: lt.ID, TotalDays: 3, Status: leave.LeaveRequestStatusApproved,
		ReviewedAt: &reviewedAt, CreatedBy: createdBy, CreatedAt: time.Now(),
	}
	pendingReq := &leave.LeaveRequest{
		ID: "req_hist_2", PublicID: "req_hist_2", OrgID: orgID, EmployeeID: "emp_2",
		LeaveTypeID: lt.ID, TotalDays: 2, Status: leave.LeaveRequestStatusPending,
		CreatedBy: createdBy, CreatedAt: time.Now(),
	}
	otherTypeReq := &leave.LeaveRequest{
		ID: "req_hist_3", PublicID: "req_hist_3", OrgID: orgID, EmployeeID: "emp_3",
		LeaveTypeID: otherLt.ID, TotalDays: 4, Status: leave.LeaveRequestStatusApproved,
		CreatedBy: createdBy, CreatedAt: time.Now(),
	}
	// Reach into the stub directly — this is pre-existing history the
	// service itself never wrote (it predates the policy).
	seedRequest(repo, approvedReq)
	seedRequest(repo, pendingReq)
	seedRequest(repo, otherTypeReq)

	_, err := svc.CreatePolicy(ctx, orgID, createdBy, leave.CreatePolicyRequest{
		LeaveTypeID: lt.ID, AccrualMethod: leave.AccrualMonthly, AccrualRate: 1,
	})
	if err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}

	result, err := svc.ListTransactions(ctx, orgID, "emp_1", leave.TransactionFilter{})
	if err != nil {
		t.Fatalf("ListTransactions failed: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("expected exactly 1 backfilled transaction for emp_1, got %d", result.Total)
	}
	tx := result.Transactions[0]
	if tx.TransactionType != leave.TransactionUsage {
		t.Errorf("expected TransactionUsage, got %s", tx.TransactionType)
	}
	if tx.Days != -3 {
		t.Errorf("expected Days -3 (signed debit), got %v", tx.Days)
	}
	if tx.TransactionDate != "2025-06-15" {
		t.Errorf("expected backfilled date 2025-06-15 (reviewed_at), got %s", tx.TransactionDate)
	}
	if tx.LeaveRequestID == nil || *tx.LeaveRequestID != approvedReq.ID {
		t.Errorf("expected transaction to reference the original request")
	}

	// The pending request (emp_2) and the other-leave-type request (emp_3)
	// must not have been backfilled.
	for _, empID := range []string{"emp_2", "emp_3"} {
		r, err := svc.ListTransactions(ctx, orgID, empID, leave.TransactionFilter{})
		if err != nil {
			t.Fatalf("ListTransactions(%s) failed: %v", empID, err)
		}
		if r.Total != 0 {
			t.Errorf("expected zero transactions for %s, got %d", empID, r.Total)
		}
	}
}

func TestPostAdjustment_RequiresNote(t *testing.T) {
	repo := newStubRepo()
	svc := leave.NewService(repo, &mockAudit{}, newDummyPool())
	ctx := context.Background()
	orgID, createdBy := newLeaveTypeAndOrg()

	lt, _ := svc.CreateLeaveType(ctx, orgID, createdBy, leave.CreateLeaveTypeRequest{Name: "Adj Type"})
	svc.CreatePolicy(ctx, orgID, createdBy, leave.CreatePolicyRequest{LeaveTypeID: lt.ID, AccrualMethod: leave.AccrualMonthly, AccrualRate: 1})

	_, err := svc.PostAdjustment(ctx, orgID, "emp_1", lt.ID, createdBy, leave.PostAdjustmentRequest{Days: 2, Note: "  "})
	if err != leave.ErrAdjustmentNoteRequired {
		t.Errorf("expected ErrAdjustmentNoteRequired, got %v", err)
	}
}

func TestPostAdjustment_RequiresNonZeroDays(t *testing.T) {
	repo := newStubRepo()
	svc := leave.NewService(repo, &mockAudit{}, newDummyPool())
	ctx := context.Background()
	orgID, createdBy := newLeaveTypeAndOrg()

	lt, _ := svc.CreateLeaveType(ctx, orgID, createdBy, leave.CreateLeaveTypeRequest{Name: "Adj Type 2"})
	svc.CreatePolicy(ctx, orgID, createdBy, leave.CreatePolicyRequest{LeaveTypeID: lt.ID, AccrualMethod: leave.AccrualMonthly, AccrualRate: 1})

	_, err := svc.PostAdjustment(ctx, orgID, "emp_1", lt.ID, createdBy, leave.PostAdjustmentRequest{Days: 0, Note: "correction"})
	if err != leave.ErrAdjustmentDaysZero {
		t.Errorf("expected ErrAdjustmentDaysZero, got %v", err)
	}
}

func TestPostAdjustment_RequiresActivePolicy(t *testing.T) {
	repo := newStubRepo()
	svc := leave.NewService(repo, &mockAudit{}, newDummyPool())
	ctx := context.Background()
	orgID, createdBy := newLeaveTypeAndOrg()

	lt, _ := svc.CreateLeaveType(ctx, orgID, createdBy, leave.CreateLeaveTypeRequest{Name: "No Policy Type"})
	_, err := svc.PostAdjustment(ctx, orgID, "emp_1", lt.ID, createdBy, leave.PostAdjustmentRequest{Days: 2, Note: "correction"})
	if err != leave.ErrNoActivePolicy {
		t.Errorf("expected ErrNoActivePolicy, got %v", err)
	}
}

func TestPostAdjustment_Success(t *testing.T) {
	repo := newStubRepo()
	svc := leave.NewService(repo, &mockAudit{}, newDummyPool())
	ctx := context.Background()
	orgID, createdBy := newLeaveTypeAndOrg()

	lt, _ := svc.CreateLeaveType(ctx, orgID, createdBy, leave.CreateLeaveTypeRequest{Name: "Adj Type 3"})
	svc.CreatePolicy(ctx, orgID, createdBy, leave.CreatePolicyRequest{LeaveTypeID: lt.ID, AccrualMethod: leave.AccrualMonthly, AccrualRate: 1})

	tx, err := svc.PostAdjustment(ctx, orgID, "emp_1", lt.ID, createdBy, leave.PostAdjustmentRequest{Days: -2.5, Note: "correcting an overgrant"})
	if err != nil {
		t.Fatalf("PostAdjustment failed: %v", err)
	}
	if tx.TransactionType != leave.TransactionAdjustment {
		t.Errorf("expected TransactionAdjustment, got %s", tx.TransactionType)
	}
	if tx.Days != -2.5 {
		t.Errorf("expected Days -2.5 (signed, as given), got %v", tx.Days)
	}

	cb, err := svc.GetCurrentBalance(ctx, orgID, "emp_1", lt.ID)
	if err != nil {
		t.Fatalf("GetCurrentBalance failed: %v", err)
	}
	if cb.Balance != -2.5 {
		t.Errorf("expected balance -2.5, got %v", cb.Balance)
	}
}

func TestPostEncashment_RequiresPositiveDays(t *testing.T) {
	repo := newStubRepo()
	svc := leave.NewService(repo, &mockAudit{}, newDummyPool())
	ctx := context.Background()
	orgID, createdBy := newLeaveTypeAndOrg()

	lt, _ := svc.CreateLeaveType(ctx, orgID, createdBy, leave.CreateLeaveTypeRequest{Name: "Encash Type"})
	svc.CreatePolicy(ctx, orgID, createdBy, leave.CreatePolicyRequest{LeaveTypeID: lt.ID, AccrualMethod: leave.AccrualMonthly, AccrualRate: 1, Encashable: true})

	_, err := svc.PostEncashment(ctx, orgID, "emp_1", lt.ID, createdBy, leave.PostEncashmentRequest{Days: 0})
	if err != leave.ErrEncashmentDaysInvalid {
		t.Errorf("expected ErrEncashmentDaysInvalid, got %v", err)
	}
}

func TestPostEncashment_RequiresEncashablePolicy(t *testing.T) {
	repo := newStubRepo()
	svc := leave.NewService(repo, &mockAudit{}, newDummyPool())
	ctx := context.Background()
	orgID, createdBy := newLeaveTypeAndOrg()

	lt, _ := svc.CreateLeaveType(ctx, orgID, createdBy, leave.CreateLeaveTypeRequest{Name: "Non-Encash Type"})
	svc.CreatePolicy(ctx, orgID, createdBy, leave.CreatePolicyRequest{LeaveTypeID: lt.ID, AccrualMethod: leave.AccrualMonthly, AccrualRate: 1, Encashable: false})

	_, err := svc.PostEncashment(ctx, orgID, "emp_1", lt.ID, createdBy, leave.PostEncashmentRequest{Days: 3})
	if err != leave.ErrEncashmentNotAllowed {
		t.Errorf("expected ErrEncashmentNotAllowed, got %v", err)
	}
}

// TestPostEncashment_Success also asserts, per decision #3, that this phase
// never computes or stores a currency amount — only the days debit.
func TestPostEncashment_Success(t *testing.T) {
	repo := newStubRepo()
	svc := leave.NewService(repo, &mockAudit{}, newDummyPool())
	ctx := context.Background()
	orgID, createdBy := newLeaveTypeAndOrg()

	lt, _ := svc.CreateLeaveType(ctx, orgID, createdBy, leave.CreateLeaveTypeRequest{Name: "Encash Type 2"})
	svc.CreatePolicy(ctx, orgID, createdBy, leave.CreatePolicyRequest{LeaveTypeID: lt.ID, AccrualMethod: leave.AccrualMonthly, AccrualRate: 1, Encashable: true})

	tx, err := svc.PostEncashment(ctx, orgID, "emp_1", lt.ID, createdBy, leave.PostEncashmentRequest{Days: 4})
	if err != nil {
		t.Fatalf("PostEncashment failed: %v", err)
	}
	if tx.TransactionType != leave.TransactionEncashment {
		t.Errorf("expected TransactionEncashment, got %s", tx.TransactionType)
	}
	if tx.Days != -4 {
		t.Errorf("expected Days -4 (negated to a debit), got %v", tx.Days)
	}
}

func TestGetCurrentBalance_NoPolicy(t *testing.T) {
	repo := newStubRepo()
	svc := leave.NewService(repo, &mockAudit{}, newDummyPool())
	ctx := context.Background()
	orgID, createdBy := newLeaveTypeAndOrg()

	lt, _ := svc.CreateLeaveType(ctx, orgID, createdBy, leave.CreateLeaveTypeRequest{Name: "Untracked Type"})
	cb, err := svc.GetCurrentBalance(ctx, orgID, "emp_1", lt.ID)
	if err != nil {
		t.Fatalf("GetCurrentBalance failed: %v", err)
	}
	if cb.HasPolicy {
		t.Error("expected HasPolicy=false for a leave type with no policy")
	}
	if cb.Balance != 0 {
		t.Errorf("expected zero balance, got %v", cb.Balance)
	}
}

// TestGetCurrentBalance_SnapshotPlusDeltaBoundary proves the checkpoint+delta
// formula's boundary: a snapshot's AsOfDate is the EXCLUSIVE upper bound of
// what it covers (it represents "closing state as of the instant this date
// begins"), so a transaction dated exactly on the snapshot's AsOfDate was
// NOT yet folded into it and must still count in the delta — only
// transactions strictly BEFORE the snapshot's AsOfDate are already covered.
// (This boundary was gotten backwards in an earlier draft and caught by
// TestIntegration_HRMLeaveBalance_SnapshotPlusDeltaMatchesFullReplay
// against a real ledger — this unit test now matches the corrected,
// integration-proven semantics.)
func TestGetCurrentBalance_SnapshotPlusDeltaBoundary(t *testing.T) {
	repo := newStubRepo()
	svc := leave.NewService(repo, &mockAudit{}, newDummyPool())
	ctx := context.Background()
	orgID, createdBy := newLeaveTypeAndOrg()

	lt, _ := svc.CreateLeaveType(ctx, orgID, createdBy, leave.CreateLeaveTypeRequest{Name: "Snapshot Type"})
	policy, _ := svc.CreatePolicy(ctx, orgID, createdBy, leave.CreatePolicyRequest{LeaveTypeID: lt.ID, AccrualMethod: leave.AccrualMonthly, AccrualRate: 1})

	// Seed a snapshot directly (simulating what the accrual job would have
	// written): AsOfDate="2026-02-01" means "covers everything before Feb 1"
	// — i.e. all of January.
	seedBalanceSnapshot(repo, &leave.LeaveBalance{
		OrgID: orgID, EmployeeID: "emp_1", LeaveTypeID: lt.ID, PolicyID: policy.ID,
		PeriodYear: 2026, PeriodMonth: 1, AsOfDate: "2026-02-01",
		ClosingBalance: 10, CreatedBy: createdBy,
	})
	// Before-boundary transaction — already reflected in the snapshot's
	// ClosingBalance, must NOT also count in the delta.
	seedTransaction(repo, &leave.LeaveTransaction{
		OrgID: orgID, EmployeeID: "emp_1", LeaveTypeID: lt.ID, PolicyID: policy.ID,
		TransactionType: leave.TransactionAccrual, Days: 10, TransactionDate: "2026-01-01", CreatedBy: createdBy,
	})
	// On-boundary transaction (dated exactly the snapshot's AsOfDate) — NOT
	// yet covered by the snapshot, must count in the delta.
	seedTransaction(repo, &leave.LeaveTransaction{
		OrgID: orgID, EmployeeID: "emp_1", LeaveTypeID: lt.ID, PolicyID: policy.ID,
		TransactionType: leave.TransactionAccrual, Days: 4, TransactionDate: "2026-02-01", CreatedBy: createdBy,
	})
	// After-boundary transaction — must count.
	seedTransaction(repo, &leave.LeaveTransaction{
		OrgID: orgID, EmployeeID: "emp_1", LeaveTypeID: lt.ID, PolicyID: policy.ID,
		TransactionType: leave.TransactionUsage, Days: -3, TransactionDate: "2026-02-15", CreatedBy: createdBy,
	})

	cb, err := svc.GetCurrentBalance(ctx, orgID, "emp_1", lt.ID)
	if err != nil {
		t.Fatalf("GetCurrentBalance failed: %v", err)
	}
	if cb.SnapshotClosing != 10 {
		t.Errorf("expected SnapshotClosing 10, got %v", cb.SnapshotClosing)
	}
	if cb.DeltaSinceSnapshot != 1 {
		t.Errorf("expected DeltaSinceSnapshot 1 (the on-boundary +4 accrual and the after-boundary -3 usage, excluding the before-boundary +10), got %v", cb.DeltaSinceSnapshot)
	}
	if cb.Balance != 11 {
		t.Errorf("expected Balance 11 (10 + 4 - 3), got %v", cb.Balance)
	}
	if cb.IsNegative {
		t.Error("expected IsNegative=false for a positive balance")
	}
}

// ── stub-seeding helpers (reach directly into stubRepo's maps) ──────────

func seedRequest(repo *stubRepo, r *leave.LeaveRequest) {
	repo.requests[r.ID] = r
}

func seedBalanceSnapshot(repo *stubRepo, b *leave.LeaveBalance) {
	b.ID = "lvb_seed_" + b.AsOfDate + "_" + b.EmployeeID
	repo.balances[b.ID] = b
}

func seedTransaction(repo *stubRepo, t *leave.LeaveTransaction) {
	t.ID = "lvx_seed_" + t.TransactionDate + "_" + t.EmployeeID + "_" + string(t.TransactionType)
	repo.transactions[t.ID] = t
}
