// backend/internal/tests/integration/statutory_benefits_test.go
// hrm/statutory and hrm/benefits against real Postgres, including their
// payout through payslips.StatutorySource / BenefitsSource — neither
// reachable from a unit test, since computePayslips reaches *pgxpool.Pool
// directly (the r25/r26/r27 precedent). Gate: INTEGRATION=1
package integration

import (
	"context"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/auth"
	"github.com/mridha/businesssaas/internal/hrm/benefits"
	hrmpayslips "github.com/mridha/businesssaas/internal/hrm/payslips"
	"github.com/mridha/businesssaas/internal/hrm/statutory"
)

func markTaxable(t *testing.T, env *testEnv, compID string) {
	t.Helper()
	ctx := context.Background()
	if _, err := env.db.Exec(ctx, `UPDATE hrm_salary_components SET is_taxable=TRUE WHERE id=$1`, compID); err != nil {
		t.Fatalf("mark component taxable: %v", err)
	}
}

func seedStatutoryRule(t *testing.T, env *testEnv, orgID, ownerID string, base statutory.BaseVariable, isEmployerContribution bool) *statutory.Rule {
	t.Helper()
	ctx := context.Background()
	r, err := env.hrmStatutorySvc.CreateRule(ctx, orgID, ownerID, statutory.CreateRuleRequest{
		Name: "Income Tax " + uniqueSlug("sxr"), CountryCode: "US", RuleType: "income_tax",
		BaseVariable: string(base), IsEmployerContribution: isEmployerContribution,
	})
	if err != nil {
		t.Fatalf("create statutory rule: %v", err)
	}
	return r
}

func seedStatutorySlab(t *testing.T, env *testEnv, orgID, ruleID, ownerID string, upTo *string, ratePct, effectiveDate string) {
	t.Helper()
	ctx := context.Background()
	if _, err := env.hrmStatutorySvc.CreateSlab(ctx, orgID, ruleID, ownerID, statutory.CreateSlabRequest{
		UpTo: upTo, RatePct: ratePct, EffectiveDate: effectiveDate,
	}); err != nil {
		t.Fatalf("create statutory slab: %v", err)
	}
}

// ============================================================
// Statutory
// ============================================================

func TestIntegration_Statutory_ComputesProgressiveTax(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedPayrollFixture(t, env, "15000")
	basicID := addComponent(t, env, fx, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)
	markTaxable(t, env, basicID)

	rule := seedStatutoryRule(t, env, fx.orgID, fx.ownerID, statutory.BaseTaxableGross, false)
	// 0-5000 @ 0%, above @ 20% — taxableGross=15000: 5000@0 + 10000@20% = 2000
	seedStatutorySlab(t, env, fx.orgID, rule.ID, fx.ownerID, strp("5000"), "0", "2020-01-01")
	seedStatutorySlab(t, env, fx.orgID, rule.ID, fx.ownerID, nil, "20", "2020-01-01")

	slip := computeFresh(t, env, fx)
	if !slip.GrossPay.Equal(dec(t, "15000")) {
		t.Errorf("gross = %s, want 15000", slip.GrossPay)
	}
	if !slip.TotalDeductions.Equal(dec(t, "2000")) {
		t.Errorf("deductions = %s, want 2000", slip.TotalDeductions)
	}
	if !slip.NetPay.Equal(dec(t, "13000")) {
		t.Errorf("net = %s, want 13000", slip.NetPay)
	}

	var lineType, componentName string
	if err := env.db.QueryRow(ctx,
		`SELECT line_type, component_name FROM hrm_payslip_lines WHERE payslip_id=$1 AND line_type='statutory'`,
		slip.ID).Scan(&lineType, &componentName); err != nil {
		t.Fatalf("expected a statutory line: %v", err)
	}
	if lineType != "statutory" {
		t.Errorf("line_type = %q, want statutory", lineType)
	}
}

// TestIntegration_Statutory_EffectiveDatedSlabsDoNotRetroactivelyChange is
// explicitly named as non-optional by the build plan: "a rule change dated
// next month must not alter this month's computed run."
func TestIntegration_Statutory_EffectiveDatedSlabsDoNotRetroactivelyChange(t *testing.T) {
	env := newTestEnv(t)
	fx := seedPayrollFixture(t, env, "10000")
	basicID := addComponent(t, env, fx, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)
	markTaxable(t, env, basicID)

	rule := seedStatutoryRule(t, env, fx.orgID, fx.ownerID, statutory.BaseTaxableGross, false)
	seedStatutorySlab(t, env, fx.orgID, rule.ID, fx.ownerID, nil, "10", "2020-01-01")

	firstRun := computeFresh(t, env, fx)
	if !firstRun.TotalDeductions.Equal(dec(t, "1000")) {
		t.Fatalf("first run deductions = %s, want 1000 (10%% of 10000)", firstRun.TotalDeductions)
	}

	// A rate change dated NEXT MONTH must not touch this month's already-
	// computed figures, and must not touch a FRESH run for this same month
	// either — an off_cycle run, since 'regular' is capped at one per
	// org/period (r25) and computeFresh already created that one above.
	now := time.Now()
	nextY, nextM := nextMonth(now.Year(), int(now.Month()))
	futureEffective := time.Date(nextY, time.Month(nextM), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	seedStatutorySlab(t, env, fx.orgID, rule.ID, fx.ownerID, nil, "50", futureEffective)

	secondRun, err := env.hrmPayslipsSvc.CreateRun(context.Background(), fx.orgID, fx.ownerID, hrmpayslips.CreateRunRequest{
		Year: fx.year, Month: fx.month, RunType: strp("off_cycle"),
	})
	if err != nil {
		t.Fatalf("create off-cycle run: %v", err)
	}
	computedSecond, err := env.hrmPayslipsSvc.ComputeRun(context.Background(), fx.orgID, secondRun.ID, fx.ownerID)
	if err != nil {
		t.Fatalf("compute off-cycle run: %v", err)
	}
	if !computedSecond.TotalDeductions.Equal(dec(t, "1000")) {
		t.Errorf("a same-month recompute after a future-dated rate change = %s, want still 1000 (unchanged)", computedSecond.TotalDeductions)
	}

	// The future period, once it arrives, must use the NEW rate.
	futureRun := computeRunFor(t, env, fx.orgID, fx.ownerID, nextY, nextM)
	found, _, ded, _ := readPayslip(t, env, futureRun.ID, fx.employeeID)
	if !found {
		t.Fatal("expected a payslip for the future period")
	}
	if ded != "5000.00" {
		t.Errorf("future-period deductions = %s, want 5000.00 (50%% of 10000, the new rate)", ded)
	}
}

func TestIntegration_Statutory_TaxableGrossExcludesNonTaxableEarnings(t *testing.T) {
	env := newTestEnv(t)
	fx := seedPayrollFixture(t, env, "8000")
	basicID := addComponent(t, env, fx, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)
	markTaxable(t, env, basicID)
	// A non-taxable allowance — must NOT be included in the statutory base.
	addComponent(t, env, fx, "Travel Allowance", "earning", "fixed", "2000", nil, 2)

	rule := seedStatutoryRule(t, env, fx.orgID, fx.ownerID, statutory.BaseTaxableGross, false)
	seedStatutorySlab(t, env, fx.orgID, rule.ID, fx.ownerID, nil, "10", "2020-01-01")

	slip := computeFresh(t, env, fx)
	if !slip.GrossPay.Equal(dec(t, "10000")) {
		t.Errorf("gross = %s, want 10000 (8000 basic + 2000 allowance)", slip.GrossPay)
	}
	// TAXABLE_GROSS = 8000 (basic only) -> 10% = 800, NOT 1000.
	if !slip.TotalDeductions.Equal(dec(t, "800")) {
		t.Errorf("deductions = %s, want 800 (10%% of TAXABLE 8000, not full gross 10000)", slip.TotalDeductions)
	}
}

func TestIntegration_Statutory_EmployerContributionDoesNotReduceNet(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedPayrollFixture(t, env, "10000")
	basicID := addComponent(t, env, fx, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)
	markTaxable(t, env, basicID)

	rule := seedStatutoryRule(t, env, fx.orgID, fx.ownerID, statutory.BaseTaxableGross, true) // employer-paid
	seedStatutorySlab(t, env, fx.orgID, rule.ID, fx.ownerID, nil, "8", "2020-01-01")

	slip := computeFresh(t, env, fx)
	if !slip.TotalDeductions.IsZero() {
		t.Errorf("deductions = %s, want 0 — an employer contribution must not reduce the employee's deductions", slip.TotalDeductions)
	}
	if !slip.NetPay.Equal(dec(t, "10000")) {
		t.Errorf("net = %s, want 10000 — an employer contribution must not reduce net", slip.NetPay)
	}

	var isEmployerContribution bool
	if err := env.db.QueryRow(ctx,
		`SELECT is_employer_contribution FROM hrm_payslip_lines WHERE payslip_id=$1 AND line_type='statutory'`,
		slip.ID).Scan(&isEmployerContribution); err != nil {
		t.Fatalf("expected a statutory line: %v", err)
	}
	if !isEmployerContribution {
		t.Error("expected the line to be flagged is_employer_contribution")
	}
}

// ============================================================
// Benefits
// ============================================================

func seedBenefitEnrollment(t *testing.T, env *testEnv, fx *payrollFixture, empCost, erCost, effectiveDate string) *benefits.Enrollment {
	t.Helper()
	ctx := context.Background()

	plan, err := env.hrmBenefitsSvc.CreatePlan(ctx, fx.orgID, fx.ownerID, benefits.CreatePlanRequest{
		Name: "Health Plan " + uniqueSlug("bfp"), PlanType: "health",
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	tier, err := env.hrmBenefitsSvc.CreateTier(ctx, fx.orgID, plan.ID, fx.ownerID, benefits.CreateTierRequest{
		TierName: "Employee Only", EmployeeCost: empCost, EmployerCost: erCost,
	})
	if err != nil {
		t.Fatalf("create tier: %v", err)
	}

	// Link the employee to a platform user so EnrollSelf (which resolves
	// user_id -> employee_id) can find them.
	var userID string
	if err := env.db.QueryRow(ctx, `SELECT user_id FROM hrm_employees WHERE id=$1`, fx.employeeID).Scan(&userID); err != nil {
		t.Fatalf("read employee user_id: %v", err)
	}
	if userID == "" {
		t.Fatal("fixture employee has no linked user_id — seedPayrollFixture must link one for EnrollSelf tests")
	}

	e, err := env.hrmBenefitsSvc.EnrollSelf(ctx, fx.orgID, userID, benefits.CreateEnrollmentRequest{
		PlanID: plan.ID, TierID: tier.ID, EnrollmentWindowType: "open", EffectiveDate: effectiveDate,
	})
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}
	return e
}

func TestIntegration_Benefits_EnrollmentDeductsFromPayroll(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedPayrollFixtureWithUser(t, env, "10000")
	addComponent(t, env, fx, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)

	e := seedBenefitEnrollment(t, env, fx, "150", "300", "2020-01-01") // already effective
	// Force to active directly — the sweep is tested separately.
	if _, err := env.db.Exec(ctx, `UPDATE hrm_benefit_enrollments SET status='active' WHERE id=$1`, e.ID); err != nil {
		t.Fatalf("activate enrollment: %v", err)
	}

	slip := computeFresh(t, env, fx)
	if !slip.GrossPay.Equal(dec(t, "10000")) {
		t.Errorf("gross = %s, want 10000", slip.GrossPay)
	}
	if !slip.TotalDeductions.Equal(dec(t, "150")) {
		t.Errorf("deductions = %s, want 150 (the employee's own cost only)", slip.TotalDeductions)
	}
	if !slip.NetPay.Equal(dec(t, "9850")) {
		t.Errorf("net = %s, want 9850", slip.NetPay)
	}
}

// TestIntegration_Benefits_TierRepricingDoesNotChangeExistingEnrollmentCost
// proves the cost snapshot is real: a tier's price changing after enrollment
// must not change what an already-enrolled employee pays.
func TestIntegration_Benefits_TierRepricingDoesNotChangeExistingEnrollmentCost(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedPayrollFixtureWithUser(t, env, "10000")
	addComponent(t, env, fx, "Basic Pay", "earning", "pct_of_basic", "100", nil, 1)

	e := seedBenefitEnrollment(t, env, fx, "150", "300", "2020-01-01")
	if _, err := env.db.Exec(ctx, `UPDATE hrm_benefit_enrollments SET status='active' WHERE id=$1`, e.ID); err != nil {
		t.Fatalf("activate enrollment: %v", err)
	}

	// Reprice the underlying tier — direct SQL, since there is no
	// service-level "update tier cost" endpoint (tiers are catalog data
	// administered by re-creating, not editing in place — see migration
	// 00104's header).
	if _, err := env.db.Exec(ctx, `UPDATE hrm_benefit_tiers SET employee_cost=999 WHERE id=$1`, e.TierID); err != nil {
		t.Fatalf("reprice tier: %v", err)
	}

	slip := computeFresh(t, env, fx)
	if !slip.TotalDeductions.Equal(dec(t, "150")) {
		t.Errorf("deductions = %s, want 150 (the FROZEN snapshot, not the tier's new 999 price)", slip.TotalDeductions)
	}
}

func TestIntegration_Benefits_SchedulerActivatesPendingEnrollments(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedPayrollFixtureWithUser(t, env, "5000")

	due := seedBenefitEnrollment(t, env, fx, "50", "100", "2020-01-01")    // long past
	notDue := seedBenefitEnrollment(t, env, fx, "60", "120", "2099-01-01") // far future

	n, err := env.hrmBenefitsSvc.ActivatePendingEnrollments(ctx)
	if err != nil {
		t.Fatalf("activate pending enrollments: %v", err)
	}
	if n < 1 {
		t.Errorf("expected at least 1 enrollment activated, got %d", n)
	}

	var dueStatus, notDueStatus string
	if err := env.db.QueryRow(ctx, `SELECT status FROM hrm_benefit_enrollments WHERE id=$1`, due.ID).Scan(&dueStatus); err != nil {
		t.Fatalf("read due enrollment: %v", err)
	}
	if dueStatus != "active" {
		t.Errorf("due enrollment status = %q, want active", dueStatus)
	}
	if err := env.db.QueryRow(ctx, `SELECT status FROM hrm_benefit_enrollments WHERE id=$1`, notDue.ID).Scan(&notDueStatus); err != nil {
		t.Fatalf("read not-due enrollment: %v", err)
	}
	if notDueStatus != "pending" {
		t.Errorf("not-due enrollment status = %q, want still pending", notDueStatus)
	}
}

func TestIntegration_Benefits_DependentVerification(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	empID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Dependent Test Employee", nil)

	d, err := env.hrmBenefitsSvc.CreateDependent(ctx, orgID, ownerID, benefits.CreateDependentRequest{
		EmployeeID: empID, FullName: "Jamie Doe", Relationship: "spouse",
	})
	if err != nil {
		t.Fatalf("create dependent: %v", err)
	}
	if d.IsVerified {
		t.Error("a newly created dependent must not start verified")
	}

	verified, err := env.hrmBenefitsSvc.VerifyDependent(ctx, orgID, d.ID, ownerID)
	if err != nil {
		t.Fatalf("verify dependent: %v", err)
	}
	if !verified.IsVerified {
		t.Error("expected is_verified=true after VerifyDependent")
	}

	var verifiedBy string
	if err := env.db.QueryRow(ctx, `SELECT verified_by FROM hrm_dependents WHERE id=$1`, d.ID).Scan(&verifiedBy); err != nil {
		t.Fatalf("read verified_by: %v", err)
	}
	if verifiedBy != ownerID {
		t.Errorf("verified_by = %q, want %q", verifiedBy, ownerID)
	}
}

// seedPayrollFixtureWithUser is seedPayrollFixture, but the employee is
// linked to a real platform user so EnrollSelf (user_id -> employee_id) can
// resolve them.
func seedPayrollFixtureWithUser(t *testing.T, env *testEnv, basicPay string) *payrollFixture {
	t.Helper()
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)

	email := uniqueEmail("benefits-emp")
	u, err := env.authSvc.Signup(ctx, auth.SignupRequest{Email: email, Password: "BenefitsEmp123!"})
	if err != nil {
		t.Fatalf("signup benefits employee user: %v", err)
	}
	t.Cleanup(func() { cleanupUser(t, env, u.ID) })

	empID := seedEmployee(t, env, orgID, statusID, ownerID, u.ID, "Benefits Test Employee", nil)

	var structureID string
	if err := env.db.QueryRow(ctx,
		`INSERT INTO hrm_salary_structures (org_id, name, created_by) VALUES ($1,$2,$3) RETURNING id`,
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
