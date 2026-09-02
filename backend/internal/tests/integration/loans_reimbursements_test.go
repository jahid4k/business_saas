// backend/internal/tests/integration/loans_reimbursements_test.go
// hrm/loans and hrm/reimbursements against real Postgres, including their
// payout through payslips.LoanSource / ReimbursementSource — neither
// reachable from a unit test, since computePayslips reaches *pgxpool.Pool
// directly (the r25/r26 precedent). Gate: INTEGRATION=1
package integration

import (
	"context"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/hrm/loans"
	hrmpayslips "github.com/mridha/businesssaas/internal/hrm/payslips"
	"github.com/mridha/businesssaas/internal/hrm/reimbursements"
)

// nextMonth returns the (year, month) one calendar month after (year, month).
func nextMonth(year, month int) (int, int) {
	if month == 12 {
		return year + 1, 1
	}
	return year, month + 1
}

func computeRunFor(t *testing.T, env *testEnv, orgID, ownerID string, year, month int) *hrmpayslips.PayslipRun {
	t.Helper()
	ctx := context.Background()
	run, err := env.hrmPayslipsSvc.CreateRun(ctx, orgID, ownerID, hrmpayslips.CreateRunRequest{Year: year, Month: month})
	if err != nil {
		t.Fatalf("create run for %d-%02d: %v", year, month, err)
	}
	computed, err := env.hrmPayslipsSvc.ComputeRun(ctx, orgID, run.ID, ownerID)
	if err != nil {
		t.Fatalf("compute run for %d-%02d: %v", year, month, err)
	}
	return computed
}

func readPayslip(t *testing.T, env *testEnv, runID, employeeID string) (found bool, gross, deductions, net string) {
	t.Helper()
	ctx := context.Background()
	// hrm_payslips' totals are NUMERIC(18,4) (r24), so a bare ::text cast
	// renders four decimal places — to_char pins it to two, matching every
	// literal comparison below.
	var g, d, n string
	err := env.db.QueryRow(ctx,
		`SELECT to_char(gross_pay,'FM999999999990.00'),
		        to_char(total_deductions,'FM999999999990.00'),
		        to_char(net_pay,'FM999999999990.00')
		   FROM hrm_payslips WHERE payslip_run_id=$1 AND employee_id=$2`,
		runID, employeeID).Scan(&g, &d, &n)
	if err != nil {
		return false, "", "", ""
	}
	return true, g, d, n
}

// ============================================================
// Loans
// ============================================================

func TestIntegration_Loans_FullLifecycle_EndToEnd(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedPayrollFixture(t, env, "10000")
	addComponent(t, env, fx, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)

	l, err := env.hrmLoansSvc.CreateLoan(ctx, fx.orgID, fx.ownerID, loans.CreateLoanRequest{
		EmployeeID: fx.employeeID, LoanType: "personal",
		PrincipalAmount: "6000", InterestRatePct: "0", TenureMonths: 6,
	})
	if err != nil {
		t.Fatalf("create loan: %v", err)
	}

	submitted, err := env.hrmLoansSvc.SubmitLoan(ctx, fx.orgID, l.ID, fx.ownerID)
	if err != nil {
		t.Fatalf("submit loan: %v", err)
	}
	if submitted.Status != loans.LoanApproved {
		t.Fatalf("expected approved (no template configured), got %s", submitted.Status)
	}

	disbursed, err := env.hrmLoansSvc.DisburseLoan(ctx, fx.orgID, l.ID, fx.ownerID)
	if err != nil {
		t.Fatalf("disburse loan: %v", err)
	}
	if disbursed.Status != loans.LoanActive {
		t.Fatalf("expected active, got %s", disbursed.Status)
	}
	if disbursed.InstallmentAmount == nil || !disbursed.InstallmentAmount.Equal(dec(t, "1000")) {
		t.Fatalf("expected installment_amount 1000, got %v", disbursed.InstallmentAmount)
	}

	schedule, err := env.hrmLoansSvc.ListSchedule(ctx, fx.orgID, l.ID)
	if err != nil {
		t.Fatalf("list schedule: %v", err)
	}
	if len(schedule) != 6 {
		t.Fatalf("expected 6 installments, got %d", len(schedule))
	}
	for _, row := range schedule {
		if !row.TotalAmount.Equal(dec(t, "1000")) {
			t.Errorf("installment %d total = %s, want 1000", row.InstallmentNumber, row.TotalAmount)
		}
		if row.Status != loans.SchedulePending {
			t.Errorf("installment %d status = %s, want pending", row.InstallmentNumber, row.Status)
		}
	}

	// The first installment is due the month AFTER disbursement.
	dueYear, dueMonth := nextMonth(time.Now().Year(), int(time.Now().Month()))

	run := computeRunFor(t, env, fx.orgID, fx.ownerID, dueYear, dueMonth)
	if !run.TotalNetPay.Equal(dec(t, "9000")) {
		t.Errorf("run net = %s, want 9000 (10000 gross - 1000 loan recovery)", run.TotalNetPay)
	}

	found, gross, ded, net := readPayslip(t, env, run.ID, fx.employeeID)
	if !found {
		t.Fatal("expected a payslip for the employee")
	}
	if gross != "10000.00" {
		t.Errorf("gross = %s, want 10000.00", gross)
	}
	if ded != "1000.00" {
		t.Errorf("deductions = %s, want 1000.00 (the loan recovery)", ded)
	}
	if net != "9000.00" {
		t.Errorf("net = %s, want 9000.00", net)
	}

	// The schedule row and the ledger must both reflect the recovery.
	updated, err := env.hrmLoansSvc.ListSchedule(ctx, fx.orgID, l.ID)
	if err != nil {
		t.Fatalf("list schedule after recovery: %v", err)
	}
	if updated[0].Status != loans.ScheduleRecovered {
		t.Errorf("installment 1 status = %s, want recovered", updated[0].Status)
	}
	if !updated[0].RecoveredAmount.Equal(dec(t, "1000")) {
		t.Errorf("installment 1 recovered_amount = %s, want 1000", updated[0].RecoveredAmount)
	}

	var eventCount int
	if err := env.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrm_loan_recovery_events WHERE schedule_id=$1`, updated[0].ID).Scan(&eventCount); err != nil {
		t.Fatalf("count recovery events: %v", err)
	}
	if eventCount != 1 {
		t.Errorf("expected 1 recovery event, got %d", eventCount)
	}
}

func TestIntegration_Loans_ZeroNetPayCapsRecovery_AndCarriesForward(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	// Thin headroom: 1000 basic pay, a 700 flat deduction, leaving only 300
	// of headroom before any loan recovery — well short of a 1000 installment.
	fx := seedPayrollFixture(t, env, "1000")
	addComponent(t, env, fx, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)
	addComponent(t, env, fx, "Fixed Deduction", "deduction", "fixed", "700", nil, 2)

	l, err := env.hrmLoansSvc.CreateLoan(ctx, fx.orgID, fx.ownerID, loans.CreateLoanRequest{
		EmployeeID: fx.employeeID, LoanType: "emergency",
		PrincipalAmount: "1000", InterestRatePct: "0", TenureMonths: 1,
	})
	if err != nil {
		t.Fatalf("create loan: %v", err)
	}
	if _, err := env.hrmLoansSvc.SubmitLoan(ctx, fx.orgID, l.ID, fx.ownerID); err != nil {
		t.Fatalf("submit loan: %v", err)
	}
	if _, err := env.hrmLoansSvc.DisburseLoan(ctx, fx.orgID, l.ID, fx.ownerID); err != nil {
		t.Fatalf("disburse loan: %v", err)
	}

	y1, m1 := nextMonth(time.Now().Year(), int(time.Now().Month()))
	run1 := computeRunFor(t, env, fx.orgID, fx.ownerID, y1, m1)

	found, _, ded, net := readPayslip(t, env, run1.ID, fx.employeeID)
	if !found {
		t.Fatal("expected a payslip")
	}
	// headroom = 1000 - 700 = 300; the 1000 installment must be CAPPED to
	// that 300, never driving net negative.
	if ded != "1000.00" {
		t.Errorf("deductions = %s, want 1000.00 (700 fixed + 300 capped loan recovery)", ded)
	}
	if net != "0.00" {
		t.Errorf("net = %s, want exactly 0.00 — recovery must not push it negative", net)
	}

	schedule, err := env.hrmLoansSvc.ListSchedule(ctx, fx.orgID, l.ID)
	if err != nil {
		t.Fatalf("list schedule: %v", err)
	}
	if schedule[0].Status != loans.SchedulePartiallyRecovered {
		t.Errorf("status = %s, want partially_recovered", schedule[0].Status)
	}
	if !schedule[0].RecoveredAmount.Equal(dec(t, "300")) {
		t.Errorf("recovered_amount = %s, want 300", schedule[0].RecoveredAmount)
	}
	if !schedule[0].RemainingOwed().Equal(dec(t, "700")) {
		t.Errorf("remaining owed = %s, want 700", schedule[0].RemainingOwed())
	}

	// Next period has full headroom (no other deductions this time is not
	// true — the same Fixed Deduction component recurs every run — so the
	// remaining 700 is again capped: headroom is still only 300/period).
	y2, m2 := nextMonth(y1, m1)
	run2 := computeRunFor(t, env, fx.orgID, fx.ownerID, y2, m2)
	found2, _, ded2, net2 := readPayslip(t, env, run2.ID, fx.employeeID)
	if !found2 {
		t.Fatal("expected a second payslip")
	}
	if net2 != "0.00" {
		t.Errorf("period 2 net = %s, want 0.00", net2)
	}
	if ded2 != "1000.00" {
		t.Errorf("period 2 deductions = %s, want 1000.00", ded2)
	}

	final, err := env.hrmLoansSvc.ListSchedule(ctx, fx.orgID, l.ID)
	if err != nil {
		t.Fatalf("list schedule after period 2: %v", err)
	}
	if !final[0].RecoveredAmount.Equal(dec(t, "600")) {
		t.Errorf("recovered_amount after 2 periods = %s, want 600 (300+300, still short of 1000)", final[0].RecoveredAmount)
	}
	if final[0].Status != loans.SchedulePartiallyRecovered {
		t.Errorf("status after 2 periods = %s, want still partially_recovered", final[0].Status)
	}
}

func TestIntegration_Loans_Foreclosure(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedPayrollFixture(t, env, "10000")
	addComponent(t, env, fx, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)

	l, err := env.hrmLoansSvc.CreateLoan(ctx, fx.orgID, fx.ownerID, loans.CreateLoanRequest{
		EmployeeID: fx.employeeID, LoanType: "personal",
		PrincipalAmount: "3000", InterestRatePct: "0", TenureMonths: 3,
	})
	if err != nil {
		t.Fatalf("create loan: %v", err)
	}
	if _, err := env.hrmLoansSvc.SubmitLoan(ctx, fx.orgID, l.ID, fx.ownerID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := env.hrmLoansSvc.DisburseLoan(ctx, fx.orgID, l.ID, fx.ownerID); err != nil {
		t.Fatalf("disburse: %v", err)
	}

	foreclosed, err := env.hrmLoansSvc.ForecloseLoan(ctx, fx.orgID, l.ID, fx.ownerID, loans.ForecloseLoanRequest{ForeclosureAmount: "2900"})
	if err != nil {
		t.Fatalf("foreclose: %v", err)
	}
	if foreclosed.Status != loans.LoanForeclosed {
		t.Fatalf("expected foreclosed, got %s", foreclosed.Status)
	}

	schedule, err := env.hrmLoansSvc.ListSchedule(ctx, fx.orgID, l.ID)
	if err != nil {
		t.Fatalf("list schedule: %v", err)
	}
	for _, row := range schedule {
		if row.Status != loans.ScheduleForeclosed {
			t.Errorf("installment %d status = %s, want foreclosed", row.InstallmentNumber, row.Status)
		}
	}

	// A payroll run for the first due period must recover NOTHING — the
	// loan is closed, not merely paused.
	y1, m1 := nextMonth(time.Now().Year(), int(time.Now().Month()))
	run := computeRunFor(t, env, fx.orgID, fx.ownerID, y1, m1)
	_, _, ded, net := readPayslip(t, env, run.ID, fx.employeeID)
	if ded != "0.00" {
		t.Errorf("deductions = %s, want 0.00 — a foreclosed loan must not recover", ded)
	}
	if net != "10000.00" {
		t.Errorf("net = %s, want 10000.00", net)
	}
}

// TestIntegration_Loans_ResignedEmployeeStopsAccruingRecovery is the
// "resignation" edge case the build plan names. A loan schedule item due
// after an employee has fully left the org is never recovered — the employee
// is simply not in that period's eligible-employee set, so no payslip is
// produced for them and the debt stays a receivable, neither silently
// recovered nor silently written off.
func TestIntegration_Loans_ResignedEmployeeStopsAccruingRecovery(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedPayrollFixture(t, env, "6000")
	addComponent(t, env, fx, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)

	l, err := env.hrmLoansSvc.CreateLoan(ctx, fx.orgID, fx.ownerID, loans.CreateLoanRequest{
		EmployeeID: fx.employeeID, LoanType: "personal",
		PrincipalAmount: "6000", InterestRatePct: "0", TenureMonths: 3,
	})
	if err != nil {
		t.Fatalf("create loan: %v", err)
	}
	if _, err := env.hrmLoansSvc.SubmitLoan(ctx, fx.orgID, l.ID, fx.ownerID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := env.hrmLoansSvc.DisburseLoan(ctx, fx.orgID, l.ID, fx.ownerID); err != nil {
		t.Fatalf("disburse: %v", err)
	}

	y1, m1 := nextMonth(time.Now().Year(), int(time.Now().Month()))
	y3, m3 := nextMonth(y1, m1)
	y3b, m3b := nextMonth(y3, m3) // the 3rd installment's due period

	// Terminate the employee well before the 3rd installment's period, and
	// give a terminated status category (not just a status_id swap) — the
	// "who gets paid" rule (r25) reads est.category.
	terminatedStatusID := statusIDFor(t, env, fx.orgID, "Terminated", "terminated")
	termDate := time.Date(y1, time.Month(m1), 15, 0, 0, 0, 0, time.UTC) // leaves during period 1
	if _, err := env.db.Exec(ctx,
		`UPDATE hrm_employees SET status_id=$1, termination_date=$2 WHERE id=$3`,
		terminatedStatusID, termDate, fx.employeeID); err != nil {
		t.Fatalf("terminate employee: %v", err)
	}

	// Period 1: the employee worked PART of it (termination_date >= period
	// start), so they are still paid and their first installment still
	// recovers normally.
	run1 := computeRunFor(t, env, fx.orgID, fx.ownerID, y1, m1)
	found1, _, ded1, _ := readPayslip(t, env, run1.ID, fx.employeeID)
	if !found1 {
		t.Fatal("expected a partial-period payslip for the exit month")
	}
	if ded1 != "2000.00" {
		t.Errorf("exit-month deductions = %s, want 2000.00 (installment 1 recovers normally)", ded1)
	}

	// Period 3: the employee is long gone — no payslip at all — so
	// installment 3 must stay untouched, not silently recovered or written off.
	run3 := computeRunFor(t, env, fx.orgID, fx.ownerID, y3b, m3b)
	if run3.TotalEmployees != 0 {
		t.Errorf("expected 0 payslips for a period after the employee fully left, got %d", run3.TotalEmployees)
	}

	schedule, err := env.hrmLoansSvc.ListSchedule(ctx, fx.orgID, l.ID)
	if err != nil {
		t.Fatalf("list schedule: %v", err)
	}
	if schedule[2].Status != loans.SchedulePending {
		t.Errorf("installment 3 status = %s, want still pending — a debt the org must settle out of band, not written off silently", schedule[2].Status)
	}
	if !schedule[2].RecoveredAmount.IsZero() {
		t.Errorf("installment 3 recovered_amount = %s, want 0", schedule[2].RecoveredAmount)
	}
}

// ============================================================
// Reimbursements
// ============================================================

func TestIntegration_Reimbursements_PaidThroughRegularRun(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedPayrollFixture(t, env, "5000")
	addComponent(t, env, fx, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)

	r, err := env.hrmReimbursementsSvc.Create(ctx, fx.orgID, fx.ownerID, reimbursements.CreateRequest{
		EmployeeID: fx.employeeID, Category: "travel", Amount: "450",
	})
	if err != nil {
		t.Fatalf("create reimbursement: %v", err)
	}
	submitted, err := env.hrmReimbursementsSvc.Submit(ctx, fx.orgID, r.ID, fx.ownerID)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if submitted.Status != reimbursements.StatusApproved {
		t.Fatalf("expected approved (no template configured), got %s", submitted.Status)
	}

	run := computeRunFor(t, env, fx.orgID, fx.ownerID, fx.year, fx.month)
	found, gross, ded, net := readPayslip(t, env, run.ID, fx.employeeID)
	if !found {
		t.Fatal("expected a payslip")
	}
	if gross != "5000.00" {
		t.Errorf("gross = %s, want 5000.00 (reimbursement must NOT inflate gross)", gross)
	}
	if ded != "0.00" {
		t.Errorf("deductions = %s, want 0.00", ded)
	}
	if net != "5450.00" {
		t.Errorf("net = %s, want 5450.00 (5000 salary + 450 reimbursement)", net)
	}

	after, err := env.hrmReimbursementsSvc.Get(ctx, fx.orgID, r.ID)
	if err != nil {
		t.Fatalf("get reimbursement: %v", err)
	}
	if after.Status != reimbursements.StatusPaid {
		t.Fatalf("expected paid, got %s", after.Status)
	}
	if after.PayslipRunID == nil || *after.PayslipRunID != run.ID {
		t.Error("expected payslip_run_id to point at the run")
	}
}

func TestIntegration_Loans_And_Reimbursements_CoexistInOneRun(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedPayrollFixture(t, env, "8000")
	addComponent(t, env, fx, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)

	l, err := env.hrmLoansSvc.CreateLoan(ctx, fx.orgID, fx.ownerID, loans.CreateLoanRequest{
		EmployeeID: fx.employeeID, LoanType: "personal",
		PrincipalAmount: "2400", InterestRatePct: "0", TenureMonths: 4,
	})
	if err != nil {
		t.Fatalf("create loan: %v", err)
	}
	if _, err := env.hrmLoansSvc.SubmitLoan(ctx, fx.orgID, l.ID, fx.ownerID); err != nil {
		t.Fatalf("submit loan: %v", err)
	}
	if _, err := env.hrmLoansSvc.DisburseLoan(ctx, fx.orgID, l.ID, fx.ownerID); err != nil {
		t.Fatalf("disburse loan: %v", err)
	}

	r, err := env.hrmReimbursementsSvc.Create(ctx, fx.orgID, fx.ownerID, reimbursements.CreateRequest{
		EmployeeID: fx.employeeID, Category: "medical", Amount: "300",
	})
	if err != nil {
		t.Fatalf("create reimbursement: %v", err)
	}
	if _, err := env.hrmReimbursementsSvc.Submit(ctx, fx.orgID, r.ID, fx.ownerID); err != nil {
		t.Fatalf("submit reimbursement: %v", err)
	}

	y1, m1 := nextMonth(time.Now().Year(), int(time.Now().Month()))
	run := computeRunFor(t, env, fx.orgID, fx.ownerID, y1, m1)
	found, gross, ded, net := readPayslip(t, env, run.ID, fx.employeeID)
	if !found {
		t.Fatal("expected a payslip")
	}
	// 8000 gross, 600 loan installment (2400/4), +300 reimbursement:
	// net = 8000 - 600 + 300 = 7700.
	if gross != "8000.00" {
		t.Errorf("gross = %s, want 8000.00", gross)
	}
	if ded != "600.00" {
		t.Errorf("deductions = %s, want 600.00", ded)
	}
	if net != "7700.00" {
		t.Errorf("net = %s, want 7700.00", net)
	}
}
