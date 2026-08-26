// backend/internal/tests/integration/payroll_compute_test.go
// The payroll compute engine against real Postgres. ComputeRun had NO
// integration coverage and effectively no unit coverage before this file —
// only ComputeSlab's arithmetic was tested — which is how three money bugs
// survived in it:
//
//  1. net pay was silently clamped to zero, so deductions exceeding gross
//     made the shortfall vanish with no error and no record;
//  2. individual line amounts were clamped the same way;
//  3. pct_of_gross / formula / slab read a MID-LOOP accumulating gross, so
//     reordering display_order silently changed everyone's pay.
//
// ComputeRun reaches for *pgxpool.Pool directly, so none of this is reachable
// from a stub-repo unit test. Gate: INTEGRATION=1
package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	hrmpayslips "github.com/mridha/businesssaas/internal/hrm/payslips"
)

// payrollFixture is one org with a salary structure an employee is attached to.
type payrollFixture struct {
	orgID       string
	statusID    string
	ownerID     string
	employeeID  string
	structureID string
	year        int
	month       int
}

func seedPayrollFixture(t *testing.T, env *testEnv, basicPay string) *payrollFixture {
	t.Helper()
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	empID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Payee", nil)

	var structureID string
	if err := env.db.QueryRow(ctx,
		`INSERT INTO hrm_salary_structures (org_id, name, created_by)
		 VALUES ($1, $2, $3) RETURNING id`,
		orgID, "Structure "+uniqueSlug("s"), ownerID).Scan(&structureID); err != nil {
		t.Fatalf("create salary structure: %v", err)
	}

	if _, err := env.db.Exec(ctx,
		`INSERT INTO hrm_employee_salary_records
		    (org_id, employee_id, structure_id, basic_pay, effective_date, change_reason, created_by)
		 VALUES ($1,$2,$3,$4,'2020-01-01','joining',$5)`,
		orgID, empID, structureID, basicPay, ownerID); err != nil {
		t.Fatalf("create salary record: %v", err)
	}

	now := time.Now().UTC()
	return &payrollFixture{
		orgID: orgID, statusID: statusID, ownerID: ownerID,
		employeeID: empID, structureID: structureID,
		year: now.Year(), month: int(now.Month()),
	}
}

// addComponent attaches one component to the fixture's structure at the given
// display order.
func addComponent(t *testing.T, env *testEnv, fx *payrollFixture,
	name, compType, calcMethod, fixedValue string, formula *string, displayOrder int) string {
	t.Helper()
	ctx := context.Background()

	var compID string
	if err := env.db.QueryRow(ctx,
		`INSERT INTO hrm_salary_components
		    (org_id, name, component_type, calc_method, fixed_value, formula_expression, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		fx.orgID, name, compType, calcMethod, fixedValue, formula, fx.ownerID).Scan(&compID); err != nil {
		t.Fatalf("create component %s: %v", name, err)
	}
	if _, err := env.db.Exec(ctx,
		`INSERT INTO hrm_salary_structure_components (structure_id, component_id, display_order)
		 VALUES ($1,$2,$3)`, fx.structureID, compID, displayOrder); err != nil {
		t.Fatalf("attach component %s: %v", name, err)
	}
	return compID
}

// computeFresh creates a run for the fixture's period and computes it,
// returning the single payslip produced.
func computeFresh(t *testing.T, env *testEnv, fx *payrollFixture) *hrmpayslips.Payslip {
	t.Helper()
	ctx := context.Background()

	run, err := env.hrmPayslipsSvc.CreateRun(ctx, fx.orgID, fx.ownerID, hrmpayslips.CreateRunRequest{
		Year: fx.year, Month: fx.month,
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if _, err := env.hrmPayslipsSvc.ComputeRun(ctx, fx.orgID, run.ID, fx.ownerID); err != nil {
		t.Fatalf("compute run: %v", err)
	}

	slip := &hrmpayslips.Payslip{}
	if err := env.db.QueryRow(ctx,
		`SELECT id::text, gross_pay, total_deductions, net_pay
		   FROM hrm_payslips WHERE payslip_run_id = $1 AND employee_id = $2`,
		run.ID, fx.employeeID,
	).Scan(&slip.ID, &slip.GrossPay, &slip.TotalDeductions, &slip.NetPay); err != nil {
		t.Fatalf("read payslip: %v", err)
	}
	slip.PayslipRunID = run.ID
	return slip
}

func dec(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	if err != nil {
		t.Fatalf("bad decimal literal %q: %v", s, err)
	}
	return d
}

// ============================================================
// Defect 3 — order-dependent gross
// ============================================================

// TestIntegration_Payroll_ComponentOrderDoesNotChangePay is the headline test.
//
// Basic 10,000 with two earnings and one deduction expressed as a share of
// gross. Under the old single-loop engine the deduction's value depended on
// how many earnings happened to precede it in display_order, so an admin
// dragging a row in the UI changed take-home pay. Under staged evaluation every
// permutation must produce identical figures.
func TestIntegration_Payroll_ComponentOrderDoesNotChangePay(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedPayrollFixture(t, env, "10000")

	// Two earnings totalling 15,000 gross, and a 10%-of-gross deduction.
	houseRent := addComponent(t, env, fx, "House Rent", "earning", "pct_of_basic", "50", nil, 1)
	transport := addComponent(t, env, fx, "Transport", "earning", "fixed", "0", nil, 2)
	if _, err := env.db.Exec(ctx,
		`UPDATE hrm_salary_components SET fixed_value = 10000 WHERE id = $1`, transport); err != nil {
		t.Fatalf("set transport value: %v", err)
	}
	tax := addComponent(t, env, fx, "Tax", "deduction", "pct_of_gross", "10", nil, 3)

	baseline := computeFresh(t, env, fx)

	// Gross = basic-derived 5,000 + fixed 10,000 = 15,000; tax = 10% = 1,500.
	if !baseline.GrossPay.Equal(dec(t, "15000")) {
		t.Fatalf("gross = %s, want 15000", baseline.GrossPay)
	}
	if !baseline.TotalDeductions.Equal(dec(t, "1500")) {
		t.Fatalf("deductions = %s, want 1500 (10%% of the FINAL gross)", baseline.TotalDeductions)
	}
	if !baseline.NetPay.Equal(dec(t, "13500")) {
		t.Fatalf("net = %s, want 13500", baseline.NetPay)
	}

	// Now permute display_order and recompute. Every permutation must agree.
	permutations := []struct {
		name                string
		taxOrder, rentOrder int
		transportOrder      int
	}{
		{"deduction first", 1, 2, 3},
		{"deduction between earnings", 2, 1, 3},
		{"reversed", 3, 2, 1},
	}

	for _, p := range permutations {
		t.Run(p.name, func(t *testing.T) {
			for compID, order := range map[string]int{
				tax: p.taxOrder, houseRent: p.rentOrder, transport: p.transportOrder,
			} {
				if _, err := env.db.Exec(ctx,
					`UPDATE hrm_salary_structure_components SET display_order = $1
					   WHERE structure_id = $2 AND component_id = $3`,
					order, fx.structureID, compID); err != nil {
					t.Fatalf("reorder: %v", err)
				}
			}
			// A fresh run for a later period, since one run per org per month.
			fx.month = fx.month%12 + 1
			if fx.month == 1 {
				fx.year++
			}
			got := computeFresh(t, env, fx)

			if !got.GrossPay.Equal(baseline.GrossPay) {
				t.Errorf("gross changed with component order: %s vs %s", got.GrossPay, baseline.GrossPay)
			}
			if !got.TotalDeductions.Equal(baseline.TotalDeductions) {
				t.Errorf("deductions changed with component order: %s vs %s — "+
					"a pct_of_gross line is reading a partial gross",
					got.TotalDeductions, baseline.TotalDeductions)
			}
			if !got.NetPay.Equal(baseline.NetPay) {
				t.Errorf("NET PAY CHANGED with component order: %s vs %s", got.NetPay, baseline.NetPay)
			}
		})
	}
}

// ============================================================
// Defects 1 and 2 — silent clamping
// ============================================================

// TestIntegration_Payroll_NegativeNetIsRecordedNotSilenced. Deductions of
// 12,000 against a gross of 10,000 must produce net = -2,000, not 0. The old
// engine reported 0 while the line items still totalled -2,000, so a payslip
// disagreed with its own arithmetic and nothing recorded that money had gone
// missing.
func TestIntegration_Payroll_NegativeNetIsRecordedNotSilenced(t *testing.T) {
	env := newTestEnv(t)
	fx := seedPayrollFixture(t, env, "10000")

	addComponent(t, env, fx, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)
	addComponent(t, env, fx, "Large Recovery", "deduction", "pct_of_basic", "120", nil, 2)

	slip := computeFresh(t, env, fx)

	if !slip.GrossPay.Equal(dec(t, "10000")) {
		t.Fatalf("gross = %s, want 10000", slip.GrossPay)
	}
	if !slip.TotalDeductions.Equal(dec(t, "12000")) {
		t.Fatalf("deductions = %s, want 12000", slip.TotalDeductions)
	}
	if !slip.NetPay.Equal(dec(t, "-2000")) {
		t.Errorf("net = %s, want -2000 — a clamped zero hides the shortfall", slip.NetPay)
	}

	// And the run cannot be approved while it holds a negative payslip.
	_, err := env.hrmPayslipsSvc.ApproveRun(context.Background(), fx.orgID, slip.PayslipRunID, fx.ownerID)
	if !errors.Is(err, hrmpayslips.ErrNegativeNetPay) {
		t.Errorf("expected ErrNegativeNetPay to block approval, got %v", err)
	}
}

// TestIntegration_Payroll_NegativeLineAmountSurvives. A formula producing a
// negative adjustment — a correction or clawback — was zeroed on the way in.
func TestIntegration_Payroll_NegativeLineAmountSurvives(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedPayrollFixture(t, env, "10000")

	addComponent(t, env, fx, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)
	negative := "0 - 500"
	addComponent(t, env, fx, "Prior Overpayment Correction", "earning", "formula", "0", &negative, 2)

	slip := computeFresh(t, env, fx)

	var amount decimal.Decimal
	if err := env.db.QueryRow(ctx,
		`SELECT computed_amount FROM hrm_payslip_lines
		   WHERE payslip_id = $1 AND component_name = 'Prior Overpayment Correction'`,
		slip.ID).Scan(&amount); err != nil {
		t.Fatalf("read correction line: %v", err)
	}
	if !amount.Equal(dec(t, "-500")) {
		t.Errorf("correction line = %s, want -500 — clamping erased a real adjustment", amount)
	}
	// 10,000 earning minus a 500 negative earning.
	if !slip.GrossPay.Equal(dec(t, "9500")) {
		t.Errorf("gross = %s, want 9500", slip.GrossPay)
	}
}

// TestIntegration_Payroll_ApproveSucceedsWhenNetIsPositive is the other half of
// the guard: it must not block ordinary runs.
func TestIntegration_Payroll_ApproveSucceedsWhenNetIsPositive(t *testing.T) {
	env := newTestEnv(t)
	fx := seedPayrollFixture(t, env, "10000")

	addComponent(t, env, fx, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)
	addComponent(t, env, fx, "Tax", "deduction", "pct_of_gross", "10", nil, 2)

	slip := computeFresh(t, env, fx)
	if !slip.NetPay.Equal(dec(t, "9000")) {
		t.Fatalf("net = %s, want 9000", slip.NetPay)
	}

	approved, err := env.hrmPayslipsSvc.ApproveRun(
		context.Background(), fx.orgID, slip.PayslipRunID, fx.ownerID)
	if err != nil {
		t.Fatalf("a positive-net run must approve cleanly: %v", err)
	}
	if approved.Status != hrmpayslips.RunApproved {
		t.Errorf("status = %s, want approved", approved.Status)
	}
}

// TestIntegration_Payroll_GrossDependentEarningUsesStageOneGross pins the
// stage-2 rule: two earnings expressed as a share of gross are both evaluated
// against the stage-1 total, so neither can influence the other and their
// relative order is irrelevant.
func TestIntegration_Payroll_GrossDependentEarningUsesStageOneGross(t *testing.T) {
	env := newTestEnv(t)
	fx := seedPayrollFixture(t, env, "10000")

	addComponent(t, env, fx, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)
	addComponent(t, env, fx, "Bonus A", "earning", "pct_of_gross", "10", nil, 2)
	addComponent(t, env, fx, "Bonus B", "earning", "pct_of_gross", "10", nil, 3)

	slip := computeFresh(t, env, fx)

	// Stage 1 gross = 10,000. Each bonus is 10% OF THAT (1,000), not compounded
	// on top of one another. Final gross = 12,000.
	if !slip.GrossPay.Equal(dec(t, "12000")) {
		t.Errorf("gross = %s, want 12000 — both gross-share earnings must use the "+
			"stage-1 gross, not compound on each other", slip.GrossPay)
	}
}

// ============================================================
// 7A — run types and the dry-run preview
// ============================================================

// TestIntegration_Payroll_RunTypesCoexistInOnePeriod proves the replaced
// constraint actually permits what run_type promises.
//
// uq_hrm_pr_org_month was UNIQUE (org_id, period_year, period_month) — one run
// per org per month OF ANY TYPE — so run_type would have been a column that
// could never hold a second value in a period. It is now a PARTIAL unique
// index over regular runs only.
func TestIntegration_Payroll_RunTypesCoexistInOnePeriod(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedPayrollFixture(t, env, "10000")
	addComponent(t, env, fx, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)

	regular := "regular"
	if _, err := env.hrmPayslipsSvc.CreateRun(ctx, fx.orgID, fx.ownerID, hrmpayslips.CreateRunRequest{
		Year: fx.year, Month: fx.month, RunType: &regular,
	}); err != nil {
		t.Fatalf("create regular run: %v", err)
	}

	// A second REGULAR run in the same period is still refused.
	if _, err := env.hrmPayslipsSvc.CreateRun(ctx, fx.orgID, fx.ownerID, hrmpayslips.CreateRunRequest{
		Year: fx.year, Month: fx.month, RunType: &regular,
	}); !errors.Is(err, hrmpayslips.ErrDuplicateRun) {
		t.Errorf("expected ErrDuplicateRun for a second regular run, got %v", err)
	}

	// Every other type may sit alongside it, and repeat.
	for _, rt := range []string{"bonus", "off_cycle", "arrears", "fnf", "bonus"} {
		runType := rt
		if _, err := env.hrmPayslipsSvc.CreateRun(ctx, fx.orgID, fx.ownerID, hrmpayslips.CreateRunRequest{
			Year: fx.year, Month: fx.month, RunType: &runType,
		}); err != nil {
			t.Fatalf("create %s run alongside regular: %v", rt, err)
		}
	}

	var n int
	if err := env.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrm_payslip_runs
		   WHERE org_id=$1 AND period_year=$2 AND period_month=$3`,
		fx.orgID, fx.year, fx.month).Scan(&n); err != nil {
		t.Fatalf("count runs: %v", err)
	}
	if n != 6 {
		t.Errorf("expected 6 runs in the period (1 regular + 5 others), got %d", n)
	}

	// And an unknown type is rejected rather than reaching the CHECK.
	bogus := "not_a_run_type"
	if _, err := env.hrmPayslipsSvc.CreateRun(ctx, fx.orgID, fx.ownerID, hrmpayslips.CreateRunRequest{
		Year: fx.year, Month: fx.month, RunType: &bogus,
	}); !errors.Is(err, hrmpayslips.ErrInvalidRunType) {
		t.Errorf("expected ErrInvalidRunType, got %v", err)
	}
}

// TestIntegration_Payroll_PreviewPersistsNothing is the dry run's whole
// contract. It must produce the same figures ComputeRun would, write no
// payslips, and leave the run in draft.
func TestIntegration_Payroll_PreviewPersistsNothing(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedPayrollFixture(t, env, "10000")
	addComponent(t, env, fx, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)
	addComponent(t, env, fx, "Tax", "deduction", "pct_of_gross", "10", nil, 2)

	run, err := env.hrmPayslipsSvc.CreateRun(ctx, fx.orgID, fx.ownerID, hrmpayslips.CreateRunRequest{
		Year: fx.year, Month: fx.month,
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	preview, err := env.hrmPayslipsSvc.PreviewRun(ctx, fx.orgID, run.ID)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.TotalEmployees != 1 {
		t.Errorf("preview covered %d employees, want 1", preview.TotalEmployees)
	}
	if !preview.TotalNetPay.Equal(dec(t, "9000")) {
		t.Errorf("preview net = %s, want 9000", preview.TotalNetPay)
	}
	if preview.NegativeNetCount != 0 {
		t.Errorf("negative count = %d, want 0", preview.NegativeNetCount)
	}

	// Nothing was written and nothing moved.
	var slipCount int
	if err := env.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrm_payslips WHERE payslip_run_id = $1`, run.ID).Scan(&slipCount); err != nil {
		t.Fatalf("count payslips: %v", err)
	}
	if slipCount != 0 {
		t.Errorf("preview persisted %d payslips — a dry run must write nothing", slipCount)
	}
	reread, err := env.hrmPayslipsSvc.GetRun(ctx, fx.orgID, run.ID)
	if err != nil {
		t.Fatalf("re-read run: %v", err)
	}
	if reread.Status != hrmpayslips.RunDraft {
		t.Errorf("run status = %s after preview, want draft", reread.Status)
	}

	// The real compute must then agree with what the preview promised.
	if _, err := env.hrmPayslipsSvc.ComputeRun(ctx, fx.orgID, run.ID, fx.ownerID); err != nil {
		t.Fatalf("compute: %v", err)
	}
	committed, err := env.hrmPayslipsSvc.GetRun(ctx, fx.orgID, run.ID)
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if !committed.TotalNetPay.Equal(preview.TotalNetPay) {
		t.Errorf("compute produced %s but preview promised %s — the two paths have diverged",
			committed.TotalNetPay, preview.TotalNetPay)
	}
}

// TestIntegration_Payroll_PreviewSurfacesNegativeNetBeforeApproval. Finding out
// at approval that a run cannot be approved is the failure the dry run exists
// to prevent.
func TestIntegration_Payroll_PreviewSurfacesNegativeNet(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedPayrollFixture(t, env, "10000")
	addComponent(t, env, fx, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)
	addComponent(t, env, fx, "Large Recovery", "deduction", "pct_of_basic", "120", nil, 2)

	run, err := env.hrmPayslipsSvc.CreateRun(ctx, fx.orgID, fx.ownerID, hrmpayslips.CreateRunRequest{
		Year: fx.year, Month: fx.month,
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	preview, err := env.hrmPayslipsSvc.PreviewRun(ctx, fx.orgID, run.ID)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.NegativeNetCount != 1 {
		t.Errorf("negative count = %d, want 1 — the dry run must surface what will block approval",
			preview.NegativeNetCount)
	}
	if !preview.TotalNetPay.Equal(dec(t, "-2000")) {
		t.Errorf("preview net = %s, want -2000", preview.TotalNetPay)
	}
}

// TestIntegration_Payroll_LineTypeIsRecorded pins that computed lines carry the
// new line_type, and that employer contributions are flagged separately rather
// than being inferred from component_type alone.
func TestIntegration_Payroll_LineTypeIsRecorded(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedPayrollFixture(t, env, "10000")
	addComponent(t, env, fx, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)
	addComponent(t, env, fx, "Tax", "deduction", "pct_of_gross", "10", nil, 2)
	addComponent(t, env, fx, "Employer PF", "employer_contribution", "pct_of_basic", "8", nil, 3)

	slip := computeFresh(t, env, fx)

	rows, err := env.db.Query(ctx,
		`SELECT component_name, line_type, is_employer_contribution
		   FROM hrm_payslip_lines WHERE payslip_id = $1 ORDER BY display_order`, slip.ID)
	if err != nil {
		t.Fatalf("read lines: %v", err)
	}
	defer rows.Close()

	got := map[string]struct {
		lineType string
		employer bool
	}{}
	for rows.Next() {
		var name, lineType string
		var employer bool
		if err := rows.Scan(&name, &lineType, &employer); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got[name] = struct {
			lineType string
			employer bool
		}{lineType, employer}
	}

	if got["Basic Pay"].lineType != "earning" {
		t.Errorf("Basic Pay line_type = %q, want earning", got["Basic Pay"].lineType)
	}
	if got["Tax"].lineType != "deduction" {
		t.Errorf("Tax line_type = %q, want deduction", got["Tax"].lineType)
	}
	if !got["Employer PF"].employer {
		t.Error("employer contribution was not flagged is_employer_contribution")
	}
	// An employer contribution is the employer's cost — it must not reduce the
	// employee's net or inflate their gross.
	if !slip.GrossPay.Equal(dec(t, "10000")) {
		t.Errorf("gross = %s, want 10000 — an employer contribution must not inflate gross", slip.GrossPay)
	}
	if !slip.NetPay.Equal(dec(t, "9000")) {
		t.Errorf("net = %s, want 9000 — an employer contribution must not reduce net", slip.NetPay)
	}
}

// TestIntegration_Payroll_PartialWriteAbortsAndIsRetryable covers the failure
// the compute loop used to absorb in silence.
//
// Before: a failed CreatePayslip was answered with `continue` and a failed
// CreatePayslipLines with `_ =`, so an employee could end up unpaid — or paid
// with a gross and a net but no lines explaining either — while the run was
// still marked 'computed'. TotalEmployees counted every employee but the money
// totals counted only the ones that saved, so the run looked complete and
// merely disagreed with itself.
//
// The trigger here is the real one: uq_hrm_ps_run_employee already holds a
// payslip for this run and employee, exactly as a previous aborted attempt
// would have left behind.
func TestIntegration_Payroll_PartialWriteAbortsAndIsRetryable(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedPayrollFixture(t, env, "10000")
	addComponent(t, env, fx, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)

	run, err := env.hrmPayslipsSvc.CreateRun(ctx, fx.orgID, fx.ownerID, hrmpayslips.CreateRunRequest{
		Year: fx.year, Month: fx.month,
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	// Leave a conflicting payslip behind, as a half-finished run would.
	if _, err := env.db.Exec(ctx,
		`INSERT INTO hrm_payslips
		    (org_id, employee_id, payslip_run_id, period_year, period_month,
		     gross_pay, total_deductions, net_pay, basic_pay, currency, status)
		 VALUES ($1,$2,$3,$4,$5,0,0,0,0,'USD','draft')`,
		fx.orgID, fx.employeeID, run.ID, fx.year, fx.month); err != nil {
		t.Fatalf("seed conflicting payslip: %v", err)
	}

	if _, err := env.hrmPayslipsSvc.ComputeRun(ctx, fx.orgID, run.ID, fx.ownerID); err == nil {
		t.Fatal("ComputeRun succeeded despite a payslip failing to save — " +
			"a partial payroll run must never report success")
	}

	// The run must not be stranded in 'computing': every entry point refuses
	// that status, so a run left there can never be paid.
	after, err := env.hrmPayslipsSvc.GetRun(ctx, fx.orgID, run.ID)
	if err != nil {
		t.Fatalf("re-read run: %v", err)
	}
	if after.Status != hrmpayslips.RunDraft {
		t.Errorf("run status = %s after an aborted compute, want draft", after.Status)
	}

	// The abort must also clear what it wrote, or the retry below inserts a
	// second set of payslips alongside the first.
	var stale int
	if err := env.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrm_payslips WHERE payslip_run_id = $1`, run.ID).Scan(&stale); err != nil {
		t.Fatalf("count payslips: %v", err)
	}
	if stale != 0 {
		t.Errorf("abort left %d payslips behind — a retry would duplicate them", stale)
	}

	// And the whole point of unwinding: the run is computable again.
	if _, err := env.hrmPayslipsSvc.ComputeRun(ctx, fx.orgID, run.ID, fx.ownerID); err != nil {
		t.Fatalf("retry after abort: %v", err)
	}
	final, err := env.hrmPayslipsSvc.GetRun(ctx, fx.orgID, run.ID)
	if err != nil {
		t.Fatalf("re-read run: %v", err)
	}
	if final.Status != hrmpayslips.RunComputed {
		t.Errorf("status after retry = %s, want computed", final.Status)
	}
	if !final.TotalNetPay.Equal(dec(t, "10000")) {
		t.Errorf("net after retry = %s, want 10000", final.TotalNetPay)
	}

	var n int
	if err := env.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrm_payslips WHERE payslip_run_id = $1`, run.ID).Scan(&n); err != nil {
		t.Fatalf("count payslips: %v", err)
	}
	if n != 1 {
		t.Errorf("run holds %d payslips after abort+retry, want 1", n)
	}
}

// TestIntegration_Payroll_RunTotalsMatchItsPayslips pins the invariant the old
// `continue` broke: the run header is the sum of the payslips actually written,
// and TotalEmployees is how many there are.
func TestIntegration_Payroll_RunTotalsMatchItsPayslips(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedPayrollFixture(t, env, "10000")
	addComponent(t, env, fx, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)
	addComponent(t, env, fx, "Tax", "deduction", "pct_of_gross", "10", nil, 2)

	// A second employee, so the totals are a real sum rather than a copy.
	second := seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, "", "Payee Two", nil)
	if _, err := env.db.Exec(ctx,
		`INSERT INTO hrm_employee_salary_records
		    (org_id, employee_id, structure_id, basic_pay, effective_date, change_reason, created_by)
		 VALUES ($1,$2,$3,'20000','2020-01-01','joining',$4)`,
		fx.orgID, second, fx.structureID, fx.ownerID); err != nil {
		t.Fatalf("second salary record: %v", err)
	}

	run, err := env.hrmPayslipsSvc.CreateRun(ctx, fx.orgID, fx.ownerID, hrmpayslips.CreateRunRequest{
		Year: fx.year, Month: fx.month,
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if _, err := env.hrmPayslipsSvc.ComputeRun(ctx, fx.orgID, run.ID, fx.ownerID); err != nil {
		t.Fatalf("compute: %v", err)
	}

	computed, err := env.hrmPayslipsSvc.GetRun(ctx, fx.orgID, run.ID)
	if err != nil {
		t.Fatalf("re-read run: %v", err)
	}

	var count int
	var sumGross, sumDed, sumNet decimal.Decimal
	if err := env.db.QueryRow(ctx,
		`SELECT COUNT(*), COALESCE(SUM(gross_pay),0), COALESCE(SUM(total_deductions),0),
		        COALESCE(SUM(net_pay),0)
		   FROM hrm_payslips WHERE payslip_run_id = $1`, run.ID,
	).Scan(&count, &sumGross, &sumDed, &sumNet); err != nil {
		t.Fatalf("aggregate payslips: %v", err)
	}

	if computed.TotalEmployees != count {
		t.Errorf("run reports %d employees but holds %d payslips",
			computed.TotalEmployees, count)
	}
	if !computed.TotalGrossPay.Equal(sumGross) {
		t.Errorf("run gross %s != sum of payslips %s", computed.TotalGrossPay, sumGross)
	}
	if !computed.TotalDeductions.Equal(sumDed) {
		t.Errorf("run deductions %s != sum of payslips %s", computed.TotalDeductions, sumDed)
	}
	if !computed.TotalNetPay.Equal(sumNet) {
		t.Errorf("run net %s != sum of payslips %s", computed.TotalNetPay, sumNet)
	}
	if !sumNet.Equal(dec(t, "27000")) {
		t.Errorf("total net = %s, want 27000 (9000 + 18000)", sumNet)
	}
}
