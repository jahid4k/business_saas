// backend/internal/tests/integration/fnf_sources_test.go
// Phase 9B-2: the three cross-module settlement debits/credits — leave
// encashment, loan foreclosure and travel-advance recovery.
//
// Each is a consumer-owned narrow interface satisfied by the module that owns
// the data. What only a live run can prove is that the money actually reaches
// the payslip AND that the source is closed out afterwards, so nothing
// charges it twice.
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/hrm/exits"
	hrmexpenses "github.com/mridha/businesssaas/internal/hrm/expenses"
	hrmleave "github.com/mridha/businesssaas/internal/hrm/leave"
	hrmpayslips "github.com/mridha/businesssaas/internal/hrm/payslips"
)

// settlementLineFor finds the audit-trail line for one source type.
func settlementLineFor(t *testing.T, env *testEnv, fx *fnfFixture, sourceType string) *exits.SettlementLineRow {
	t.Helper()
	lines, err := env.hrmExitsSvc.ListSettlementLines(
		context.Background(), fx.orgID, hrCaller(fx.ownerID), fx.exitID)
	if err != nil {
		t.Fatalf("list settlement lines: %v", err)
	}
	for _, l := range lines {
		if l.SourceType == sourceType {
			return l
		}
	}
	return nil
}

// ============================================================
// 1. Leave encashment → a CREDIT
// ============================================================

// TestIntegration_FnFSources_EncashmentDaysBecomeMoney is the handover Phase 2
// designed for: hrm/leave records DAYS and never money, and
// encashment_rate_basis has been stored since then with the note that "a
// future F&F phase reads this". This is that phase.
func TestIntegration_FnFSources_EncashmentDaysBecomeMoney(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedFnF(t, env, "30000") // basic 30000 => daily rate 1000 at /30
	addComponent(t, env, fx.payrollFixture, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)
	seedGratuityRule(t, env, fx, "5", "30", "30", "2015-01-01")

	lt, _ := seedLeavePolicy(t, ctx, env.hrmLeaveSvc, fx.orgID, fx.ownerID,
		hrmleave.AccrualMethod("monthly"), 2, false, nil, true)
	// The policy must price against pay for F&F to be able to value a day.
	basis := hrmleave.EncashmentBasisBasicPay
	if _, err := env.db.Exec(ctx,
		`UPDATE hrm_leave_policies SET encashment_rate_basis = $2 WHERE leave_type_id = $1`,
		lt.ID, string(basis)); err != nil {
		t.Fatalf("set rate basis: %v", err)
	}
	// Give them a balance, then have HR record an encashment of 5 days.
	if _, err := env.hrmLeaveSvc.PostAdjustment(ctx, fx.orgID, fx.employeeID, lt.ID, fx.ownerID,
		hrmleave.PostAdjustmentRequest{Days: 20, Note: "opening"}); err != nil {
		t.Fatalf("seed balance: %v", err)
	}
	if _, err := env.hrmLeaveSvc.PostEncashment(ctx, fx.orgID, fx.employeeID, lt.ID, fx.ownerID,
		hrmleave.PostEncashmentRequest{Days: 5}); err != nil {
		t.Fatalf("post encashment: %v", err)
	}

	if _, err := env.hrmPayslipsSvc.ComputeRun(ctx, fx.orgID, fx.runID, fx.ownerID); err != nil {
		t.Fatalf("compute: %v", err)
	}

	line := settlementLineFor(t, env, fx, "leave_encashment")
	if line == nil {
		t.Fatal("no leave_encashment settlement line — recorded days never became money")
	}
	// 5 days x 1000/day.
	want := decimal.NewFromInt(5000)
	if !line.Amount.Equal(want) {
		t.Errorf("encashment = %s, want %s (5 days x 30000/30)", line.Amount, want)
	}
	if !line.IsCredit {
		t.Error("leave encashment recorded as a DEBIT — it is money owed TO the employee")
	}
	if line.PayslipLineID == nil {
		t.Error("the encashment line never reached the payslip")
	}
}

// TestIntegration_FnFSources_FixedRateBasisIsReportedNotGuessed — the `fixed`
// basis is a real gap: hrm_leave_policies stores the BASIS but has no column
// for the amount. Guessing a figure, or silently pricing it at zero with no
// explanation, would both be worse than saying so.
func TestIntegration_FnFSources_FixedRateBasisIsReportedNotGuessed(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedFnF(t, env, "30000")
	addComponent(t, env, fx.payrollFixture, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)

	lt, _ := seedLeavePolicy(t, ctx, env.hrmLeaveSvc, fx.orgID, fx.ownerID,
		hrmleave.AccrualMethod("monthly"), 2, false, nil, true)
	if _, err := env.db.Exec(ctx,
		`UPDATE hrm_leave_policies SET encashment_rate_basis = 'fixed' WHERE leave_type_id = $1`,
		lt.ID); err != nil {
		t.Fatalf("set fixed basis: %v", err)
	}
	if _, err := env.hrmLeaveSvc.PostAdjustment(ctx, fx.orgID, fx.employeeID, lt.ID, fx.ownerID,
		hrmleave.PostAdjustmentRequest{Days: 10, Note: "opening"}); err != nil {
		t.Fatalf("seed balance: %v", err)
	}
	if _, err := env.hrmLeaveSvc.PostEncashment(ctx, fx.orgID, fx.employeeID, lt.ID, fx.ownerID,
		hrmleave.PostEncashmentRequest{Days: 3}); err != nil {
		t.Fatalf("post encashment: %v", err)
	}

	if _, err := env.hrmPayslipsSvc.ComputeRun(ctx, fx.orgID, fx.runID, fx.ownerID); err != nil {
		t.Fatalf("compute: %v", err)
	}

	line := settlementLineFor(t, env, fx, "leave_encashment")
	if line == nil {
		t.Fatal("an unpriceable encashment vanished entirely — HR must still see the days were owed")
	}
	if !line.Amount.IsZero() {
		t.Errorf("amount = %s, want 0 — there is no column storing the fixed rate to pay", line.Amount)
	}
	if !strings.Contains(line.Description, "NOT PAID") {
		t.Errorf("description %q does not say the days went unpaid", line.Description)
	}
}

// ============================================================
// 2. Loan foreclosure → a DEBIT
// ============================================================

// seedActiveLoan creates and disburses a loan so it has a live schedule.
func seedActiveLoan(t *testing.T, env *testEnv, fx *fnfFixture, principal string, months int) string {
	t.Helper()
	ctx := context.Background()
	var loanID string
	if err := env.db.QueryRow(ctx,
		`INSERT INTO hrm_loans (org_id, employee_id, loan_type, principal_amount, interest_rate_pct,
		     tenure_months, status, disbursed_at, created_by)
		 VALUES ($1,$2,'personal',$3,0,$4,'active',NOW(),$5) RETURNING id`,
		fx.orgID, fx.employeeID, principal, months, fx.ownerID).Scan(&loanID); err != nil {
		t.Fatalf("seed loan: %v", err)
	}
	// A flat schedule: principal split evenly, all future-dated so the
	// ordinary per-installment recovery would not pick them up anyway.
	per := decimal.RequireFromString(principal).Div(decimal.NewFromInt(int64(months)))
	for i := 1; i <= months; i++ {
		if _, err := env.db.Exec(ctx,
			`INSERT INTO hrm_loan_schedules (loan_id, installment_number, due_period_year,
			     due_period_month, principal_component, interest_component, total_amount,
			     recovered_amount, status)
			 VALUES ($1,$2,2030,$3,$4,0,$4,0,'pending')`,
			loanID, i, ((i-1)%12)+1, per.String()); err != nil {
			t.Fatalf("seed schedule %d: %v", i, err)
		}
	}
	return loanID
}

// TestIntegration_FnFSources_LoanForeclosesInFullAndClosesOut proves both
// halves: the whole remaining balance is charged, and the loan is actually
// marked foreclosed so nothing recovers it a second time.
func TestIntegration_FnFSources_LoanForeclosesInFullAndClosesOut(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedFnF(t, env, "50000")
	addComponent(t, env, fx.payrollFixture, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)

	loanID := seedActiveLoan(t, env, fx, "12000", 12) // 1000 x 12 outstanding

	if _, err := env.hrmPayslipsSvc.ComputeRun(ctx, fx.orgID, fx.runID, fx.ownerID); err != nil {
		t.Fatalf("compute: %v", err)
	}

	line := settlementLineFor(t, env, fx, "loan_foreclosure")
	if line == nil {
		t.Fatal("no loan_foreclosure line — the leaver's loan was never closed out")
	}
	// The FULL outstanding, not one installment.
	if !line.Amount.Equal(decimal.NewFromInt(12000)) {
		t.Errorf("foreclosure = %s, want 12000 (the whole remaining balance, not one installment)",
			line.Amount)
	}
	if line.IsCredit {
		t.Error("loan foreclosure recorded as a CREDIT — it is money owed BY the employee")
	}

	// The source must actually be closed: an audit line alone would leave the
	// loan active for the next process to recover again.
	var status string
	var foreclosureAmount *decimal.Decimal
	if err := env.db.QueryRow(ctx,
		`SELECT status, foreclosure_amount FROM hrm_loans WHERE id = $1`, loanID).
		Scan(&status, &foreclosureAmount); err != nil {
		t.Fatalf("read loan: %v", err)
	}
	if status != "foreclosed" {
		t.Errorf("loan status = %q after settlement, want foreclosed", status)
	}
	if foreclosureAmount == nil || !foreclosureAmount.Equal(decimal.NewFromInt(12000)) {
		t.Errorf("foreclosure_amount = %v, want 12000", foreclosureAmount)
	}
	var openSchedules int
	if err := env.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrm_loan_schedules
		  WHERE loan_id = $1 AND status IN ('pending','partially_recovered')`, loanID).
		Scan(&openSchedules); err != nil {
		t.Fatalf("count schedules: %v", err)
	}
	if openSchedules != 0 {
		t.Errorf("%d schedule rows still open after foreclosure", openSchedules)
	}
}

// TestIntegration_FnFSources_NoDoubleChargeOnDueInstallment is the hazard the
// design exists to avoid. The ordinary per-installment recovery is skipped for
// fnf runs; without that, the installment due this period would be charged
// once by recovery AND again inside the foreclosed balance.
func TestIntegration_FnFSources_NoDoubleChargeOnDueInstallment(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedFnF(t, env, "50000")
	addComponent(t, env, fx.payrollFixture, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)

	loanID := seedActiveLoan(t, env, fx, "12000", 12)
	// Make one installment DUE in the run's own period — the overlap case.
	if _, err := env.db.Exec(ctx,
		`UPDATE hrm_loan_schedules SET due_period_year=$2, due_period_month=$3
		  WHERE loan_id=$1 AND installment_number=1`,
		loanID, fx.year, fx.month); err != nil {
		t.Fatalf("make installment due: %v", err)
	}

	if _, err := env.hrmPayslipsSvc.ComputeRun(ctx, fx.orgID, fx.runID, fx.ownerID); err != nil {
		t.Fatalf("compute: %v", err)
	}

	// Total loan-related deduction across the whole payslip must equal the
	// outstanding EXACTLY — 12000, not 13000.
	var loanLineTotal decimal.Decimal
	if err := env.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(l.computed_amount), 0)
		   FROM hrm_payslip_lines l
		   JOIN hrm_payslips p ON p.id = l.payslip_id
		  WHERE p.payslip_run_id = $1
		    AND (l.component_name ILIKE '%loan%' OR l.line_type = 'loan_recovery')`,
		fx.runID).Scan(&loanLineTotal); err != nil {
		t.Fatalf("sum loan lines: %v", err)
	}
	if !loanLineTotal.Equal(decimal.NewFromInt(12000)) {
		t.Errorf("loan lines total %s, want exactly 12000 — a due installment has been charged "+
			"BOTH by per-installment recovery and inside the foreclosed balance", loanLineTotal)
	}
}

// ============================================================
// 3. Travel advance recovery → a DEBIT
// ============================================================

func seedFnFAdvance(t *testing.T, env *testEnv, fx *fnfFixture, amount, currency string) string {
	t.Helper()
	var id string
	if err := env.db.QueryRow(context.Background(),
		`INSERT INTO hrm_travel_advances (org_id, employee_id, amount, currency, settled_amount,
		     status, disbursed_at, created_by)
		 VALUES ($1,$2,$3,$4,0,'disbursed',NOW(),$5) RETURNING id`,
		fx.orgID, fx.employeeID, amount, currency, fx.ownerID).Scan(&id); err != nil {
		t.Fatalf("seed advance: %v", err)
	}
	return id
}

// TestIntegration_FnFSources_AdvanceRecoveredAndMarkedSettled
func TestIntegration_FnFSources_AdvanceRecoveredAndMarkedSettled(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedFnF(t, env, "40000")
	addComponent(t, env, fx.payrollFixture, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)

	var runCurrency string
	if err := env.db.QueryRow(ctx,
		`SELECT currency FROM hrm_payslip_runs WHERE id=$1`, fx.runID).Scan(&runCurrency); err != nil {
		t.Fatalf("read run currency: %v", err)
	}
	advanceID := seedFnFAdvance(t, env, fx, "8000.00", runCurrency)

	if _, err := env.hrmPayslipsSvc.ComputeRun(ctx, fx.orgID, fx.runID, fx.ownerID); err != nil {
		t.Fatalf("compute: %v", err)
	}

	line := settlementLineFor(t, env, fx, "travel_advance")
	if line == nil {
		t.Fatal("no travel_advance line — an unsettled advance was never recovered")
	}
	if !line.Amount.Equal(decimal.NewFromInt(8000)) {
		t.Errorf("recovery = %s, want 8000", line.Amount)
	}
	if line.IsCredit {
		t.Error("advance recovery recorded as a CREDIT — it is money owed BY the employee")
	}

	// The source must be closed out, or the next settlement recovers it again.
	var settled decimal.Decimal
	var status string
	if err := env.db.QueryRow(ctx,
		`SELECT settled_amount, status FROM hrm_travel_advances WHERE id=$1`, advanceID).
		Scan(&settled, &status); err != nil {
		t.Fatalf("read advance: %v", err)
	}
	if !settled.Equal(decimal.NewFromInt(8000)) || status != string(hrmexpenses.AdvanceSettled) {
		t.Errorf("advance settled=%s status=%q, want 8000 and settled", settled, status)
	}
}

// TestIntegration_FnFSources_ForeignCurrencyAdvanceIsReportedNotGuessed —
// this org has recorded NO rate for the pair, so nothing may be converted.
//
// ⚠ The FX table now exists (11B-1), which makes this test MORE important
// rather than obsolete: giving settlement a rate source must not weaken its
// refusal to invent one. Converting at parity would mis-charge a departing
// person real money. Its counterpart —
// TestIntegration_FX_FnFForeignAdvanceIsRecoveredWhenARateExists — records a
// rate and asserts the advance does convert.
func TestIntegration_FnFSources_ForeignCurrencyAdvanceIsReportedNotGuessed(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedFnF(t, env, "40000")
	addComponent(t, env, fx.payrollFixture, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)

	var runCurrency string
	if err := env.db.QueryRow(ctx,
		`SELECT currency FROM hrm_payslip_runs WHERE id=$1`, fx.runID).Scan(&runCurrency); err != nil {
		t.Fatalf("read run currency: %v", err)
	}
	foreign := "EUR"
	if runCurrency == "EUR" {
		foreign = "JPY"
	}
	advanceID := seedFnFAdvance(t, env, fx, "500.00", foreign)

	if _, err := env.hrmPayslipsSvc.ComputeRun(ctx, fx.orgID, fx.runID, fx.ownerID); err != nil {
		t.Fatalf("compute: %v", err)
	}

	line := settlementLineFor(t, env, fx, "travel_advance")
	if line == nil {
		t.Fatal("a foreign-currency advance vanished — HR must see it is still outstanding")
	}
	if !line.Amount.IsZero() {
		t.Errorf("amount = %s, want 0 — no exchange rate exists, so nothing may be charged", line.Amount)
	}
	if !strings.Contains(line.Description, "NOT RECOVERED") {
		t.Errorf("description %q does not say it went unrecovered", line.Description)
	}

	// And it must NOT have been marked settled.
	var settled decimal.Decimal
	if err := env.db.QueryRow(ctx,
		`SELECT settled_amount FROM hrm_travel_advances WHERE id=$1`, advanceID).Scan(&settled); err != nil {
		t.Fatalf("read advance: %v", err)
	}
	if !settled.IsZero() {
		t.Errorf("settled_amount = %s — an advance nobody charged for was marked settled", settled)
	}
}

// ============================================================
// Nil-safety
// ============================================================

// TestIntegration_FnFSources_AllThreeAreNilSafe — a deployment without leave,
// loans or expenses wired must still settle, producing no line from the
// missing source rather than failing the whole run.
func TestIntegration_FnFSources_AllThreeAreNilSafe(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedFnF(t, env, "20000")
	addComponent(t, env, fx.payrollFixture, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)

	// An exits service with every cross-module source nil.
	bare := exits.NewService(exits.NewRepository(env.db), env.checklistsSvc, env.hrmScopeResolver,
		nil, nil, nil, nil, nil, nil)
	settlement, err := bare.SettlementForRun(ctx, fx.orgID, fx.runID)
	if err != nil {
		t.Fatalf("SettlementForRun with all three sources nil: %v", err)
	}
	if settlement == nil {
		t.Fatal("no settlement produced")
	}
	for _, l := range settlement.Lines {
		switch l.SourceType {
		case "leave_encashment", "loan_foreclosure", "travel_advance":
			t.Errorf("a %s line appeared with that source nil", l.SourceType)
		}
	}

	// And MarkSettled must not panic trying to consume sources it has none of.
	if err := bare.MarkSettled(ctx, fx.runID, []hrmpayslips.AppliedSettlementLine{
		{SourceType: "loan_foreclosure", SourceID: "11111111-1111-1111-1111-111111111111",
			LineID: "22222222-2222-2222-2222-222222222222", Amount: decimal.NewFromInt(10)},
	}); err != nil {
		t.Errorf("MarkSettled with nil sources: %v", err)
	}
}
