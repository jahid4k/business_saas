// backend/internal/tests/integration/fnf_settlement_test.go
// Phase 9B: full & final settlement as an off-cycle payroll run.
//
// The load-bearing claims here are that shipped payroll behaviour is
// UNCHANGED — a regular run with a negative payslip is still refused — while
// an F&F run may legitimately go negative, and that a settlement is an
// ordinary payslip with extra lines rather than a parallel calculator.
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/hrm/exits"
	hrmpayslips "github.com/mridha/businesssaas/internal/hrm/payslips"
)

// fnfFixture is an employee with a salary structure, an exit, and an F&F run
// attached to it.
type fnfFixture struct {
	*payrollFixture
	exitID string
	runID  string
}

// seedFnF builds the whole chain: payroll fixture → resignation → exit →
// F&F run attached.
func seedFnF(t *testing.T, env *testEnv, basicPay string) *fnfFixture {
	t.Helper()
	ctx := context.Background()
	pf := seedPayrollFixture(t, env, basicPay)
	caller := hrCaller(pf.ownerID)

	// A resignation to hang the exit on.
	var sourceID string
	if err := env.db.QueryRow(ctx,
		`INSERT INTO hrm_resignations (org_id, employee_id, resignation_date, notice_period_days,
		    last_working_date, reason_category, created_by)
		 VALUES ($1,$2,CURRENT_DATE,30,CURRENT_DATE,'personal',$3) RETURNING id`,
		pf.orgID, pf.employeeID, pf.ownerID).Scan(&sourceID); err != nil {
		t.Fatalf("seed resignation: %v", err)
	}

	e, err := env.hrmExitsSvc.Create(ctx, pf.orgID, caller, exits.CreateExitRequest{
		EmployeeID: pf.employeeID, SourceType: "resignation", SourceID: sourceID,
	})
	if err != nil {
		t.Fatalf("create exit: %v", err)
	}

	run, err := env.hrmPayslipsSvc.CreateRun(ctx, pf.orgID, pf.ownerID, hrmpayslips.CreateRunRequest{
		Year: pf.year, Month: pf.month, RunType: strp("fnf"),
	})
	if err != nil {
		t.Fatalf("create fnf run: %v", err)
	}
	if _, err := env.hrmExitsSvc.AttachFnFRun(ctx, pf.orgID, caller, e.ID, run.ID); err != nil {
		t.Fatalf("attach run: %v", err)
	}
	return &fnfFixture{payrollFixture: pf, exitID: e.ID, runID: run.ID}
}

func seedGratuityRule(t *testing.T, env *testEnv, fx *fnfFixture, minYears, daysPerYear, divisor, effective string) {
	t.Helper()
	_, err := env.hrmExitsSvc.CreateGratuityRule(context.Background(), fx.orgID, hrCaller(fx.ownerID),
		exits.CreateGratuityRuleRequest{
			Name: "Standard gratuity", MinYearsOfService: minYears, DaysPerYear: daysPerYear,
			MonthlyDivisor: &divisor, EffectiveDate: effective,
		})
	if err != nil {
		t.Fatalf("create gratuity rule: %v", err)
	}
}

// fnfSlip reads the single payslip an F&F run produced.
func fnfSlip(t *testing.T, env *testEnv, fx *fnfFixture) *hrmpayslips.Payslip {
	t.Helper()
	slip := &hrmpayslips.Payslip{}
	if err := env.db.QueryRow(context.Background(),
		`SELECT id::text, gross_pay, total_deductions, net_pay
		   FROM hrm_payslips WHERE payslip_run_id = $1`, fx.runID,
	).Scan(&slip.ID, &slip.GrossPay, &slip.TotalDeductions, &slip.NetPay); err != nil {
		t.Fatalf("read F&F payslip: %v", err)
	}
	slip.PayslipRunID = fx.runID
	return slip
}

// ============================================================
// The regression guard — shipped payroll behaviour is unchanged
// ============================================================

// TestIntegration_FnF_RegularRunStillRefusesNegativeNet is THE guard for this
// slice. r25 added the negative-net block as one of four money-defect fixes;
// 9B makes it run-type-aware, and the one thing that must not happen is the
// exception leaking to ordinary payroll.
//
// It duplicates part of TestIntegration_Payroll_NegativeNetIsRecordedNotSilenced
// deliberately: that test guards the r25 behaviour, this one guards the 9B
// change, and they should fail for different reasons.
func TestIntegration_FnF_RegularRunStillRefusesNegativeNet(t *testing.T) {
	env := newTestEnv(t)
	fx := seedPayrollFixture(t, env, "10000")

	addComponent(t, env, fx, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)
	addComponent(t, env, fx, "Large Recovery", "deduction", "pct_of_basic", "120", nil, 2)

	slip := computeFresh(t, env, fx) // CreateRun with no run_type => 'regular'
	if !slip.NetPay.IsNegative() {
		t.Fatalf("net = %s, want negative for this fixture", slip.NetPay)
	}

	_, err := env.hrmPayslipsSvc.ApproveRun(context.Background(), fx.orgID, slip.PayslipRunID, fx.ownerID)
	if !errors.Is(err, hrmpayslips.ErrNegativeNetPay) {
		t.Fatalf("a REGULAR run with a negative payslip returned %v, want ErrNegativeNetPay — "+
			"the F&F exception has leaked into ordinary payroll", err)
	}
}

// TestIntegration_FnF_NegativeNetIsApprovable is the other half: what an
// employee OWES on the way out can legitimately exceed what they are due, and
// the resulting negative net is a receivable to collect, not a data problem.
func TestIntegration_FnF_NegativeNetIsApprovable(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedFnF(t, env, "10000")
	addComponent(t, env, fx.payrollFixture, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)

	// A clearance debt far larger than the final salary.
	amount := "50000.00"
	if _, err := env.hrmExitsSvc.AddClearanceItem(ctx, fx.orgID, hrCaller(fx.ownerID), fx.exitID,
		exits.CreateClearanceItemRequest{
			Department: "IT", Description: "Unreturned laptop", BlockingAmount: &amount,
		}); err != nil {
		t.Fatalf("add clearance item: %v", err)
	}

	if _, err := env.hrmPayslipsSvc.ComputeRun(ctx, fx.orgID, fx.runID, fx.ownerID); err != nil {
		t.Fatalf("compute F&F run: %v", err)
	}
	slip := fnfSlip(t, env, fx)
	if !slip.NetPay.IsNegative() {
		t.Fatalf("net = %s, want negative (10,000 salary against a 50,000 debt)", slip.NetPay)
	}

	// Clearance is still open, so approval is refused for THAT reason —
	// not for the negative net.
	_, err := env.hrmPayslipsSvc.ApproveRun(ctx, fx.orgID, fx.runID, fx.ownerID)
	if !errors.Is(err, hrmpayslips.ErrClearancePending) {
		t.Fatalf("approval with open clearance returned %v, want ErrClearancePending", err)
	}

	// Resolve the item; now the negative net must NOT block approval.
	items, err := env.hrmExitsSvc.ListClearanceItems(ctx, fx.orgID, hrCaller(fx.ownerID), fx.exitID)
	if err != nil || len(items) == 0 {
		t.Fatalf("list clearance items: %v", err)
	}
	if _, err := env.hrmExitsSvc.ResolveClearanceItem(ctx, fx.orgID, hrCaller(fx.ownerID),
		fx.exitID, items[0].ID, exits.ResolveClearanceItemRequest{}); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	approved, err := env.hrmPayslipsSvc.ApproveRun(ctx, fx.orgID, fx.runID, fx.ownerID)
	if err != nil {
		t.Fatalf("an F&F run with negative net must approve: %v", err)
	}
	if approved.Status != hrmpayslips.RunApproved {
		t.Errorf("status = %s, want approved", approved.Status)
	}
}

// ============================================================
// F&F is the ADDS-ON shape, not REPLACES
// ============================================================

// TestIntegration_FnF_PaysProratedSalaryPlusSettlement proves the core design
// decision. Bonus REPLACES the salary computation; F&F must NOT — a
// settlement that paid only its own lines would omit the leaver's final
// salary, the largest credit in most settlements.
func TestIntegration_FnF_PaysProratedSalaryPlusSettlement(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedFnF(t, env, "30000")
	addComponent(t, env, fx.payrollFixture, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)

	if _, err := env.hrmPayslipsSvc.ComputeRun(ctx, fx.orgID, fx.runID, fx.ownerID); err != nil {
		t.Fatalf("compute: %v", err)
	}
	slip := fnfSlip(t, env, fx)

	// Gross must include the ordinary salary-structure earning. A REPLACES
	// implementation would show zero here.
	if !slip.GrossPay.GreaterThanOrEqual(dec(t, "30000")) {
		t.Errorf("gross = %s, want at least the 30000 final salary — "+
			"an F&F run must pay prorated salary, not only its settlement lines", slip.GrossPay)
	}

	var lineCount int
	if err := env.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrm_payslip_lines l
		   JOIN hrm_payslips p ON p.id = l.payslip_id
		  WHERE p.payslip_run_id = $1`, fx.runID).Scan(&lineCount); err != nil {
		t.Fatalf("count lines: %v", err)
	}
	if lineCount == 0 {
		t.Error("the F&F payslip has no lines at all")
	}
}

// TestIntegration_FnF_SettlesALeaverOrdinaryPayrollWouldSkip is why F&F needs
// its own employee load. The org-wide eligibility filter pays a terminated
// employee only when their termination_date falls on or after the period
// start — which is exactly the person a settlement run months later exists
// to pay.
func TestIntegration_FnF_SettlesALeaverOrdinaryPayrollWouldSkip(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedFnF(t, env, "20000")
	addComponent(t, env, fx.payrollFixture, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)

	// Mark them long gone: terminated status, termination_date well before
	// the run's period. Ordinary payroll would drop them entirely.
	var terminatedStatus string
	if err := env.db.QueryRow(ctx,
		`INSERT INTO hrm_employee_statuses (org_id, name, category)
		 VALUES ($1,'Departed','terminated') RETURNING id`, fx.orgID).Scan(&terminatedStatus); err != nil {
		t.Fatalf("seed terminated status: %v", err)
	}
	if _, err := env.db.Exec(ctx,
		`UPDATE hrm_employees SET status_id=$2, termination_date = DATE '2000-01-31' WHERE id=$1`,
		fx.employeeID, terminatedStatus); err != nil {
		t.Fatalf("mark departed: %v", err)
	}

	if _, err := env.hrmPayslipsSvc.ComputeRun(ctx, fx.orgID, fx.runID, fx.ownerID); err != nil {
		t.Fatalf("compute F&F for a departed employee: %v", err)
	}
	var n int
	if err := env.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrm_payslips WHERE payslip_run_id=$1`, fx.runID).Scan(&n); err != nil {
		t.Fatalf("count payslips: %v", err)
	}
	if n != 1 {
		t.Fatalf("F&F produced %d payslips for a long-departed employee, want 1 — "+
			"the settlement path must bypass the org-wide eligibility filter", n)
	}
}

// TestIntegration_FnF_RunWithNoExitIsReported — an F&F run that settles
// nobody is always a mistake, and computing it as an empty success would look
// like it worked.
func TestIntegration_FnF_RunWithNoExitIsReported(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedPayrollFixture(t, env, "10000")

	run, err := env.hrmPayslipsSvc.CreateRun(ctx, fx.orgID, fx.ownerID, hrmpayslips.CreateRunRequest{
		Year: fx.year, Month: fx.month, RunType: strp("fnf"),
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	_, err = env.hrmPayslipsSvc.ComputeRun(ctx, fx.orgID, run.ID, fx.ownerID)
	if !errors.Is(err, hrmpayslips.ErrNoExitForFnFRun) {
		t.Errorf("computing an unattached F&F run returned %v, want ErrNoExitForFnFRun", err)
	}
}

// ============================================================
// Gratuity
// ============================================================

// TestIntegration_FnF_GratuityRuleIsEffectiveDated — a rule dated next month
// must not alter a settlement computed today. Same claim r28 proved for
// statutory slabs and r30 for per-diem rates.
func TestIntegration_FnF_GratuityRuleIsEffectiveDated(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedFnF(t, env, "30000")
	addComponent(t, env, fx.payrollFixture, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)

	// Long tenure so gratuity actually applies.
	if _, err := env.db.Exec(ctx,
		`UPDATE hrm_employees SET hire_date = DATE '2010-01-01' WHERE id=$1`, fx.employeeID); err != nil {
		t.Fatalf("backdate hire: %v", err)
	}

	// In force: 30 days per year. Dated far in the past so it applies.
	seedGratuityRule(t, env, fx, "5", "30", "30", "2015-01-01")
	// A far-FUTURE revision at double the rate. It must not be picked up.
	seedGratuityRule(t, env, fx, "5", "60", "30", "2099-01-01")

	if _, err := env.hrmPayslipsSvc.ComputeRun(ctx, fx.orgID, fx.runID, fx.ownerID); err != nil {
		t.Fatalf("compute: %v", err)
	}

	lines, err := env.hrmExitsSvc.ListSettlementLines(ctx, fx.orgID, hrCaller(fx.ownerID), fx.exitID)
	if err != nil {
		t.Fatalf("list settlement lines: %v", err)
	}
	var gratuity *string
	var gratuityAmount string
	for _, l := range lines {
		if l.SourceType == "gratuity" {
			gratuity = &l.Description
			gratuityAmount = l.Amount.String()
		}
	}
	if gratuity == nil {
		t.Fatalf("no gratuity line produced for a 16-year tenure; lines: %d", len(lines))
	}
	// 30 days/year at a 1000 daily rate. The future rule would double it.
	if gratuityAmount == "" {
		t.Fatal("gratuity line has no amount")
	}
	t.Logf("gratuity = %s (%s)", gratuityAmount, *gratuity)

	// The in-force rule pays 30 days per completed year at a 1000 daily rate
	// (30,000 basic / 30). The 2099 revision would pay 60 days — exactly
	// double. Derive the expected figure from the SAME hire and last-working
	// dates the service used, so this asserts an exact amount rather than a
	// bound that a doubled figure could still satisfy.
	var lwd time.Time
	if err := env.db.QueryRow(ctx,
		`SELECT last_working_date FROM hrm_exits WHERE id=$1`, fx.exitID).Scan(&lwd); err != nil {
		t.Fatalf("read last working date: %v", err)
	}
	hire := time.Date(2010, time.January, 1, 0, 0, 0, 0, time.UTC)
	completedYears := int64(lwd.UTC().Sub(hire).Hours() / 24 / 365.25)
	expected := dec(t, "30").Mul(dec(t, "1000")).Mul(decimal.NewFromInt(completedYears))

	amt := dec(t, gratuityAmount)
	if !amt.Equal(expected) {
		t.Errorf("gratuity = %s, want %s (%d completed years x 30 days x 1000/day). "+
			"Double this figure means the rule dated 2099 leaked into a settlement computed today.",
			amt, expected, completedYears)
	}
}

// TestIntegration_FnF_NoGratuityRuleIsNotAnError — an org that has not set
// gratuity terms simply does not pay it, and the settlement must still run.
func TestIntegration_FnF_NoGratuityRuleIsNotAnError(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedFnF(t, env, "20000")
	addComponent(t, env, fx.payrollFixture, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)

	if _, err := env.hrmPayslipsSvc.ComputeRun(ctx, fx.orgID, fx.runID, fx.ownerID); err != nil {
		t.Fatalf("compute with no gratuity rule configured: %v", err)
	}
	lines, err := env.hrmExitsSvc.ListSettlementLines(ctx, fx.orgID, hrCaller(fx.ownerID), fx.exitID)
	if err != nil {
		t.Fatalf("list lines: %v", err)
	}
	for _, l := range lines {
		if l.SourceType == "gratuity" {
			t.Errorf("a gratuity line appeared with no rule configured: %s", l.Amount)
		}
	}
}

// ============================================================
// The audit trail
// ============================================================

// TestIntegration_FnF_SettlementLinesExplainTheFigure — six months later
// "recovered 50,000" is unanswerable without knowing which claim it was.
func TestIntegration_FnF_SettlementLinesExplainTheFigure(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedFnF(t, env, "40000")
	addComponent(t, env, fx.payrollFixture, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)
	caller := hrCaller(fx.ownerID)

	laptop := "15000.00"
	badge := "500.00"
	for _, it := range []struct{ dept, desc, amt string }{
		{"IT", "Unreturned laptop", laptop},
		{"Facilities", "Lost access badge", badge},
	} {
		a := it.amt
		if _, err := env.hrmExitsSvc.AddClearanceItem(ctx, fx.orgID, caller, fx.exitID,
			exits.CreateClearanceItemRequest{
				Department: it.dept, Description: it.desc, BlockingAmount: &a,
			}); err != nil {
			t.Fatalf("add %s item: %v", it.dept, err)
		}
	}

	if _, err := env.hrmPayslipsSvc.ComputeRun(ctx, fx.orgID, fx.runID, fx.ownerID); err != nil {
		t.Fatalf("compute: %v", err)
	}

	lines, err := env.hrmExitsSvc.ListSettlementLines(ctx, fx.orgID, caller, fx.exitID)
	if err != nil {
		t.Fatalf("list settlement lines: %v", err)
	}
	dues := 0
	for _, l := range lines {
		if l.SourceType != "clearance_due" {
			continue
		}
		dues++
		// ONE LINE PER CLAIM, not a lumped total: a departing employee has to
		// be able to dispute a specific department's specific claim.
		if l.SourceID == nil {
			t.Error("a clearance due has no source_id — it cannot be traced to the claim")
		}
		if l.IsCredit {
			t.Errorf("clearance due %s recorded as a credit", l.Description)
		}
		// Amount is ALWAYS positive; direction lives in is_credit.
		if l.Amount.IsNegative() {
			t.Errorf("amount = %s — debits must be stored positive with is_credit=false", l.Amount)
		}
		if l.PayslipLineID == nil {
			t.Errorf("clearance due %s was never linked to its payslip line", l.Description)
		}
	}
	if dues != 2 {
		t.Errorf("%d clearance_due lines, want 2 (one per claim, never lumped)", dues)
	}
}

// TestIntegration_FnF_RecomputeDoesNotDoubleTheAuditTrail — recomputing a
// draft must supersede the previous attempt, not append to it.
func TestIntegration_FnF_RecomputeDoesNotDoubleTheAuditTrail(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedFnF(t, env, "25000")
	addComponent(t, env, fx.payrollFixture, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)
	caller := hrCaller(fx.ownerID)

	amount := "5000.00"
	if _, err := env.hrmExitsSvc.AddClearanceItem(ctx, fx.orgID, caller, fx.exitID,
		exits.CreateClearanceItemRequest{
			Department: "Finance", Description: "Advance", BlockingAmount: &amount,
		}); err != nil {
		t.Fatalf("add item: %v", err)
	}

	if _, err := env.hrmPayslipsSvc.ComputeRun(ctx, fx.orgID, fx.runID, fx.ownerID); err != nil {
		t.Fatalf("first compute: %v", err)
	}
	first, err := env.hrmExitsSvc.ListSettlementLines(ctx, fx.orgID, caller, fx.exitID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(first) == 0 {
		t.Fatal("no settlement lines after the first compute")
	}

	// Re-assembling the settlement is exactly what a recompute does. Driving
	// it directly rather than resetting the run's status keeps the test about
	// the audit trail rather than about run state machinery.
	if _, err := env.hrmExitsSvc.SettlementForRun(ctx, fx.orgID, fx.runID); err != nil {
		t.Fatalf("re-assemble settlement: %v", err)
	}
	second, err := env.hrmExitsSvc.ListSettlementLines(ctx, fx.orgID, caller, fx.exitID)
	if err != nil {
		t.Fatalf("list after re-assembly: %v", err)
	}
	if len(second) != len(first) {
		t.Errorf("settlement lines went from %d to %d on re-assembly — a recompute must "+
			"supersede the previous attempt, not append to it and double every figure",
			len(first), len(second))
	}

	// One line per claim, still — the invariant that makes the trail
	// readable. MarkSettled re-stamps the payslip links on the next compute,
	// so their absence immediately after a bare re-assembly is expected.
	seen := map[string]int{}
	for _, l := range second {
		key := l.SourceType
		if l.SourceID != nil {
			key += ":" + *l.SourceID
		}
		seen[key]++
	}
	for key, n := range seen {
		if n != 1 {
			t.Errorf("source %s appears %d times on the audit trail, want exactly 1", key, n)
		}
	}
}
