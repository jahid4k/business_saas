// backend/internal/tests/integration/compensation_test.go
// hrm/compensation against real Postgres — the merit-matrix cycle engine and
// the bonus engine's payout through payslips.BonusSource, neither reachable
// from a unit test: ComputeCycle's buildContext reaches *pgxpool.Pool
// directly (the payslips.computePayslips precedent, r25), and the bonus
// payout path crosses two packages through a live approvals callback.
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/auth"
	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/approvals"
	"github.com/mridha/businesssaas/internal/hrm/compensation"
	hrmpayslips "github.com/mridha/businesssaas/internal/hrm/payslips"
)

// compFixture is one org with an employee on a graded salary structure.
type compFixture struct {
	orgID       string
	statusID    string
	ownerID     string
	employeeID  string
	gradeLabel  string
	structureID string
}

func seedCompFixture(t *testing.T, env *testEnv, basicPay, gradeLabel string) *compFixture {
	t.Helper()
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	empID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Compensation Test Employee", nil)

	var structureID string
	if err := env.db.QueryRow(ctx,
		`INSERT INTO hrm_salary_structures (org_id, name, grade_label, created_by)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		orgID, "Structure "+uniqueSlug("cs"), gradeLabel, ownerID).Scan(&structureID); err != nil {
		t.Fatalf("create salary structure: %v", err)
	}
	if _, err := env.db.Exec(ctx,
		`INSERT INTO hrm_employee_salary_records
		    (org_id, employee_id, structure_id, basic_pay, effective_date, change_reason, created_by)
		 VALUES ($1,$2,$3,$4,'2020-01-01','joining',$5)`,
		orgID, empID, structureID, basicPay, ownerID); err != nil {
		t.Fatalf("create salary record: %v", err)
	}

	return &compFixture{orgID: orgID, statusID: statusID, ownerID: ownerID, employeeID: empID, gradeLabel: gradeLabel, structureID: structureID}
}

// seedBand creates a compensation band for the fixture's grade.
func seedBand(t *testing.T, env *testEnv, fx *compFixture, min, mid, max string) *compensation.Band {
	t.Helper()
	ctx := context.Background()
	b, err := env.hrmCompensationSvc.CreateBand(ctx, fx.orgID, fx.ownerID, compensation.CreateBandRequest{
		GradeLabel: fx.gradeLabel, MinAmount: min, MidAmount: mid, MaxAmount: max,
		EffectiveDate: "2020-01-01",
	})
	if err != nil {
		t.Fatalf("create band: %v", err)
	}
	return b
}

// seedPublishedRating fabricates a rating scale, one level, an appraisal
// cycle and a PUBLISHED appraisal rating fx.employeeID at the given level —
// the minimum real path attachLatestRating reads.
func seedPublishedRating(t *testing.T, env *testEnv, fx *compFixture, label string, value string) string {
	t.Helper()
	ctx := context.Background()

	var scaleID string
	if err := env.db.QueryRow(ctx,
		`INSERT INTO hrm_rating_scales (org_id, name, created_by) VALUES ($1,$2,$3) RETURNING id`,
		fx.orgID, "Scale "+uniqueSlug("rs"), fx.ownerID).Scan(&scaleID); err != nil {
		t.Fatalf("create rating scale: %v", err)
	}
	var levelID string
	if err := env.db.QueryRow(ctx,
		`INSERT INTO hrm_rating_scale_levels (scale_id, label, value) VALUES ($1,$2,$3) RETURNING id`,
		scaleID, label, value).Scan(&levelID); err != nil {
		t.Fatalf("create rating level: %v", err)
	}
	var cycleID string
	if err := env.db.QueryRow(ctx,
		`INSERT INTO hrm_appraisal_cycles (org_id, name, period_start, period_end, rating_scale_id, created_by)
		 VALUES ($1,$2,'2020-01-01','2020-12-31',$3,$4) RETURNING id`,
		fx.orgID, "Cycle "+uniqueSlug("acyc"), scaleID, fx.ownerID).Scan(&cycleID); err != nil {
		t.Fatalf("create appraisal cycle: %v", err)
	}
	// published_at is deliberately BEFORE every revision cycle's
	// effective_date used in these tests (all "2020-06-01") — attachLatestRating
	// filters on published_at <= the cycle's reference date, so a rating
	// published "now" (2026+) would never be found for a 2020 cycle.
	if _, err := env.db.Exec(ctx,
		`INSERT INTO hrm_appraisals (org_id, cycle_id, employee_id, phase, final_rating_level_id, final_rating_label, final_rating_value, published_at, created_by)
		 VALUES ($1,$2,$3,'published',$4,$5,$6,'2020-01-15',$7)`,
		fx.orgID, cycleID, fx.employeeID, levelID, label, value, fx.ownerID); err != nil {
		t.Fatalf("create published appraisal: %v", err)
	}
	return levelID
}

func seedMatrixCell(t *testing.T, env *testEnv, fx *compFixture, ratingLevelID, min string, max *string, increasePct string) {
	t.Helper()
	ctx := context.Background()
	if _, err := env.hrmCompensationSvc.CreateMatrixCell(ctx, fx.orgID, fx.ownerID, compensation.CreateMatrixCellRequest{
		RatingLevelID: ratingLevelID, CompaRatioMin: min, CompaRatioMax: max,
		IncreasePct: increasePct, EffectiveDate: "2020-01-01",
	}); err != nil {
		t.Fatalf("create matrix cell: %v", err)
	}
}

// ============================================================
// Salary revision cycles
// ============================================================

func TestIntegration_Compensation_MeritCycle_EndToEnd(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedCompFixture(t, env, "50000", "Grade-"+uniqueSlug("g"))
	seedBand(t, env, fx, "40000", "50000", "60000") // compa-ratio = 1.0
	levelID := seedPublishedRating(t, env, fx, "Exceeds", "4")
	seedMatrixCell(t, env, fx, levelID, "0.9", strPtr("1.1"), "5") // covers ratio 1.0

	cycle, err := env.hrmCompensationSvc.CreateCycle(ctx, fx.orgID, fx.ownerID, compensation.CreateCycleRequest{
		Name: "Annual Review " + uniqueSlug("cyc"), EffectiveDate: "2020-06-01",
	})
	if err != nil {
		t.Fatalf("create cycle: %v", err)
	}
	if cycle.Status != compensation.CycleDraft {
		t.Fatalf("expected draft, got %s", cycle.Status)
	}

	computed, err := env.hrmCompensationSvc.ComputeCycle(ctx, fx.orgID, cycle.ID)
	if err != nil {
		t.Fatalf("compute cycle: %v", err)
	}
	if computed.Status != compensation.CycleComputed {
		t.Fatalf("expected computed, got %s", computed.Status)
	}

	res, err := env.hrmCompensationSvc.ListRevisions(ctx, fx.orgID, cycle.ID, compensation.ListFilter{Scope: authz.ScopeAll})
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	if len(res.Revisions) != 1 {
		t.Fatalf("expected 1 revision, got %d", len(res.Revisions))
	}
	rv := res.Revisions[0]
	if rv.ComputationWarning != nil {
		t.Errorf("unexpected warning: %s", *rv.ComputationWarning)
	}
	want := compensation.ApplyIncrease(dec(t, "50000"), dec(t, "5"))
	if !rv.ProposedBasicPay.Equal(want) {
		t.Errorf("proposed basic pay = %s, want %s", rv.ProposedBasicPay, want)
	}
	if len(rv.CalculationSnapshot) == 0 {
		t.Error("calculation_snapshot must never be empty — it is mandatory")
	}

	// No approval template configured for this org -> auto-approved fallback.
	submitted, err := env.hrmCompensationSvc.SubmitCycle(ctx, fx.orgID, cycle.ID, fx.ownerID)
	if err != nil {
		t.Fatalf("submit cycle: %v", err)
	}
	if submitted.Status != compensation.CycleApproved {
		t.Fatalf("expected approved (no template configured), got %s", submitted.Status)
	}

	applied, err := env.hrmCompensationSvc.ApplyCycle(ctx, fx.orgID, cycle.ID, fx.ownerID)
	if err != nil {
		t.Fatalf("apply cycle: %v", err)
	}
	if applied.Status != compensation.CycleApplied {
		t.Fatalf("expected applied, got %s", applied.Status)
	}

	var newBasicPay decimal.Decimal
	var changeReason string
	if err := env.db.QueryRow(ctx,
		`SELECT basic_pay, change_reason FROM hrm_employee_salary_records
		  WHERE employee_id=$1 ORDER BY effective_date DESC, created_at DESC LIMIT 1`,
		fx.employeeID).Scan(&newBasicPay, &changeReason); err != nil {
		t.Fatalf("read latest salary record: %v", err)
	}
	if !newBasicPay.Equal(want) {
		t.Errorf("applied salary record basic_pay = %s, want %s", newBasicPay, want)
	}
	if changeReason != "annual_revision" {
		t.Errorf("change_reason = %q, want annual_revision", changeReason)
	}

	final, err := env.hrmCompensationSvc.GetRevision(ctx, fx.orgID, rv.ID)
	if err != nil {
		t.Fatalf("get revision: %v", err)
	}
	if final.SalaryRecordID == nil {
		t.Error("expected salary_record_id to be set after apply")
	}
}

func TestIntegration_Compensation_MeritCycle_NoBandOrRating_SurfacesWarning(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedCompFixture(t, env, "50000", "Ungraded-"+uniqueSlug("g"))
	// Deliberately no band and no rating seeded.

	cycle, err := env.hrmCompensationSvc.CreateCycle(ctx, fx.orgID, fx.ownerID, compensation.CreateCycleRequest{
		Name: "Cycle " + uniqueSlug("cyc"), EffectiveDate: "2020-06-01",
	})
	if err != nil {
		t.Fatalf("create cycle: %v", err)
	}
	if _, err := env.hrmCompensationSvc.ComputeCycle(ctx, fx.orgID, cycle.ID); err != nil {
		t.Fatalf("compute cycle: %v", err)
	}

	res, err := env.hrmCompensationSvc.ListRevisions(ctx, fx.orgID, cycle.ID, compensation.ListFilter{Scope: authz.ScopeAll})
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	if len(res.Revisions) != 1 {
		t.Fatalf("expected 1 revision, got %d", len(res.Revisions))
	}
	rv := res.Revisions[0]
	if rv.ComputationWarning == nil {
		t.Fatal("expected a computation_warning when no band exists for the grade")
	}
	if !rv.ProposedBasicPay.Equal(rv.CurrentBasicPay) {
		t.Errorf("expected proposed == current when the engine cannot compute an increase, got proposed=%s current=%s",
			rv.ProposedBasicPay, rv.CurrentBasicPay)
	}
}

func TestIntegration_Compensation_MeritCycle_ExcludedRevisionSkippedOnApply(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedCompFixture(t, env, "50000", "Grade-"+uniqueSlug("g"))
	seedBand(t, env, fx, "40000", "50000", "60000")
	levelID := seedPublishedRating(t, env, fx, "Meets", "3")
	seedMatrixCell(t, env, fx, levelID, "0", nil, "5")

	cycle, err := env.hrmCompensationSvc.CreateCycle(ctx, fx.orgID, fx.ownerID, compensation.CreateCycleRequest{
		Name: "Cycle " + uniqueSlug("cyc"), EffectiveDate: "2020-06-01",
	})
	if err != nil {
		t.Fatalf("create cycle: %v", err)
	}
	if _, err := env.hrmCompensationSvc.ComputeCycle(ctx, fx.orgID, cycle.ID); err != nil {
		t.Fatalf("compute cycle: %v", err)
	}
	res, err := env.hrmCompensationSvc.ListRevisions(ctx, fx.orgID, cycle.ID, compensation.ListFilter{Scope: authz.ScopeAll})
	if err != nil || len(res.Revisions) != 1 {
		t.Fatalf("list revisions: %v (%d)", err, len(res.Revisions))
	}
	rv := res.Revisions[0]

	if _, err := env.hrmCompensationSvc.OverrideRevision(ctx, fx.orgID, rv.ID, compensation.OverrideRevisionRequest{
		ProposedBasicPay: rv.CurrentBasicPay.String(), Reason: "opting out this cycle",
	}); err != nil {
		t.Fatalf("override (exclude is set via a second field, see below): %v", err)
	}
	// Exclude directly at the DB layer — OverrideRevisionRequest does not
	// expose is_excluded as a public toggle yet; this proves the APPLY path
	// honours it regardless of how it got set.
	if _, err := env.db.Exec(ctx, `UPDATE hrm_salary_revisions SET is_excluded=TRUE WHERE id=$1`, rv.ID); err != nil {
		t.Fatalf("mark excluded: %v", err)
	}

	if _, err := env.hrmCompensationSvc.SubmitCycle(ctx, fx.orgID, cycle.ID, fx.ownerID); err != nil {
		t.Fatalf("submit cycle: %v", err)
	}
	if _, err := env.hrmCompensationSvc.ApplyCycle(ctx, fx.orgID, cycle.ID, fx.ownerID); err != nil {
		t.Fatalf("apply cycle: %v", err)
	}

	var count int
	if err := env.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrm_employee_salary_records WHERE employee_id=$1 AND change_reason='annual_revision'`,
		fx.employeeID).Scan(&count); err != nil {
		t.Fatalf("count salary records: %v", err)
	}
	if count != 0 {
		t.Errorf("an excluded revision must not produce a salary record, found %d", count)
	}

	final, err := env.hrmCompensationSvc.GetRevision(ctx, fx.orgID, rv.ID)
	if err != nil {
		t.Fatalf("get revision: %v", err)
	}
	if final.SalaryRecordID != nil {
		t.Error("excluded revision must not carry a salary_record_id")
	}
}

func TestIntegration_Compensation_MeritCycle_RecomputeReplacesRevisions(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedCompFixture(t, env, "50000", "Grade-"+uniqueSlug("g"))
	seedBand(t, env, fx, "40000", "50000", "60000")
	levelID := seedPublishedRating(t, env, fx, "Meets", "3")
	seedMatrixCell(t, env, fx, levelID, "0", nil, "5")

	cycle, err := env.hrmCompensationSvc.CreateCycle(ctx, fx.orgID, fx.ownerID, compensation.CreateCycleRequest{
		Name: "Cycle " + uniqueSlug("cyc"), EffectiveDate: "2020-06-01",
	})
	if err != nil {
		t.Fatalf("create cycle: %v", err)
	}
	if _, err := env.hrmCompensationSvc.ComputeCycle(ctx, fx.orgID, cycle.ID); err != nil {
		t.Fatalf("compute cycle: %v", err)
	}
	first, err := env.hrmCompensationSvc.ListRevisions(ctx, fx.orgID, cycle.ID, compensation.ListFilter{Scope: authz.ScopeAll})
	if err != nil || len(first.Revisions) != 1 {
		t.Fatalf("list revisions: %v (%d)", err, len(first.Revisions))
	}
	firstID := first.Revisions[0].ID

	if _, err := env.hrmCompensationSvc.ComputeCycle(ctx, fx.orgID, cycle.ID); err != nil {
		t.Fatalf("recompute cycle: %v", err)
	}
	second, err := env.hrmCompensationSvc.ListRevisions(ctx, fx.orgID, cycle.ID, compensation.ListFilter{Scope: authz.ScopeAll})
	if err != nil {
		t.Fatalf("list revisions after recompute: %v", err)
	}
	if len(second.Revisions) != 1 {
		t.Fatalf("recompute must replace, not accumulate — expected 1 revision, got %d", len(second.Revisions))
	}
	if second.Revisions[0].ID == firstID {
		t.Error("recompute produced the same revision id — ReplaceRevisions should delete and reinsert")
	}
}

func TestIntegration_Compensation_MeritCycle_WithApprovalTemplate_GoesThroughApprovalsAndApply(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedCompFixture(t, env, "50000", "Grade-"+uniqueSlug("g"))
	seedBand(t, env, fx, "40000", "50000", "60000")
	levelID := seedPublishedRating(t, env, fx, "Meets", "3")
	seedMatrixCell(t, env, fx, levelID, "0", nil, "5")

	if _, err := env.hrmApprovalsSvc.CreateTemplate(ctx, fx.orgID, fx.ownerID, approvals.CreateTemplateRequest{
		Name: "Salary Revision Approval", ActionType: approvals.ActionTypeSalaryRevision, IsDefault: true,
		Levels: []approvals.CreateTemplateLevelRequest{
			{Level: 1, ApproverType: approvals.ApproverTypeSpecificUser, ApproverUserID: &fx.ownerID, SLAHours: 48, OnSLABreach: approvals.SLABreachEscalateNext},
		},
	}); err != nil {
		t.Fatalf("create approval template: %v", err)
	}

	cycle, err := env.hrmCompensationSvc.CreateCycle(ctx, fx.orgID, fx.ownerID, compensation.CreateCycleRequest{
		Name: "Cycle " + uniqueSlug("cyc"), EffectiveDate: "2020-06-01",
	})
	if err != nil {
		t.Fatalf("create cycle: %v", err)
	}
	if _, err := env.hrmCompensationSvc.ComputeCycle(ctx, fx.orgID, cycle.ID); err != nil {
		t.Fatalf("compute cycle: %v", err)
	}

	submitted, err := env.hrmCompensationSvc.SubmitCycle(ctx, fx.orgID, cycle.ID, fx.ownerID)
	if err != nil {
		t.Fatalf("submit cycle: %v", err)
	}
	if submitted.Status != compensation.CyclePendingApproval {
		t.Fatalf("expected pending_approval, got %s", submitted.Status)
	}
	if submitted.ApprovalInstanceID == nil {
		t.Fatal("expected an approval_instance_id")
	}

	// Decide through the REAL approvals service — exercises the
	// RegisterCallback("salary_revision", ...) wiring set up in newTestEnv.
	if _, err := env.hrmApprovalsSvc.Decide(ctx, fx.orgID, *submitted.ApprovalInstanceID, fx.ownerID,
		approvals.DecisionRequest{Action: "approved"}); err != nil {
		t.Fatalf("decide: %v", err)
	}

	afterDecision, err := env.hrmCompensationSvc.GetCycle(ctx, fx.orgID, cycle.ID)
	if err != nil {
		t.Fatalf("get cycle: %v", err)
	}
	if afterDecision.Status != compensation.CycleApproved {
		t.Fatalf("expected the approval decision to flip the cycle to approved via the callback, got %s", afterDecision.Status)
	}

	// Apply is a distinct step from approval — the promotions.Apply precedent.
	if _, err := env.hrmCompensationSvc.ApplyCycle(ctx, fx.orgID, cycle.ID, fx.ownerID); err != nil {
		t.Fatalf("apply cycle: %v", err)
	}
	var count int
	if err := env.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrm_employee_salary_records WHERE employee_id=$1 AND change_reason='annual_revision'`,
		fx.employeeID).Scan(&count); err != nil {
		t.Fatalf("count salary records: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 applied salary record, got %d", count)
	}
}

// ============================================================
// Bonuses, and their payout through payslips.BonusSource
// ============================================================

func TestIntegration_Compensation_Bonus_ApprovalAndPayoutThroughBonusRun(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedCompFixture(t, env, "50000", "Grade-"+uniqueSlug("g"))
	now := time.Now()
	year, month := now.Year(), int(now.Month())

	b, err := env.hrmCompensationSvc.CreateBonus(ctx, fx.orgID, fx.ownerID, compensation.CreateBonusRequest{
		EmployeeID: fx.employeeID, BonusType: "performance", PeriodYear: year, PeriodMonth: &month,
		Amount: "2500",
	})
	if err != nil {
		t.Fatalf("create bonus: %v", err)
	}
	if len(b.CalculationSnapshot) == 0 {
		t.Error("bonus calculation_snapshot must never be empty — it is mandatory")
	}

	// No approval template configured -> auto-approved fallback.
	submitted, err := env.hrmCompensationSvc.SubmitBonus(ctx, fx.orgID, b.ID, fx.ownerID)
	if err != nil {
		t.Fatalf("submit bonus: %v", err)
	}
	if submitted.Status != compensation.BonusApproved {
		t.Fatalf("expected approved, got %s", submitted.Status)
	}

	run, err := env.hrmPayslipsSvc.CreateRun(ctx, fx.orgID, fx.ownerID, hrmpayslips.CreateRunRequest{
		Year: year, Month: month, RunType: strPtr("bonus"),
	})
	if err != nil {
		t.Fatalf("create bonus run: %v", err)
	}
	computed, err := env.hrmPayslipsSvc.ComputeRun(ctx, fx.orgID, run.ID, fx.ownerID)
	if err != nil {
		t.Fatalf("compute bonus run: %v", err)
	}
	if computed.TotalEmployees != 1 {
		t.Fatalf("expected 1 payslip in the bonus run, got %d", computed.TotalEmployees)
	}
	if !computed.TotalNetPay.Equal(dec(t, "2500")) {
		t.Errorf("bonus run net = %s, want 2500", computed.TotalNetPay)
	}

	// The bonus must now be marked paid, pointing at the real run/line.
	after, err := env.hrmCompensationSvc.GetBonus(ctx, fx.orgID, b.ID)
	if err != nil {
		t.Fatalf("get bonus: %v", err)
	}
	if after.Status != compensation.BonusPaid {
		t.Fatalf("expected paid, got %s", after.Status)
	}
	if after.PayslipRunID == nil || *after.PayslipRunID != run.ID {
		t.Error("expected payslip_run_id to point at the bonus run")
	}
	if after.PayslipLineID == nil {
		t.Error("expected payslip_line_id to be set")
	}

	// A second bonus run for the same period must find nothing left to pay.
	run2, err := env.hrmPayslipsSvc.CreateRun(ctx, fx.orgID, fx.ownerID, hrmpayslips.CreateRunRequest{
		Year: year, Month: month, RunType: strPtr("bonus"),
	})
	if err != nil {
		t.Fatalf("create second bonus run: %v", err)
	}
	computed2, err := env.hrmPayslipsSvc.ComputeRun(ctx, fx.orgID, run2.ID, fx.ownerID)
	if err != nil {
		t.Fatalf("compute second bonus run: %v", err)
	}
	if computed2.TotalEmployees != 0 {
		t.Errorf("second bonus run should find zero pending bonuses (already paid), got %d", computed2.TotalEmployees)
	}
}

// TestIntegration_Compensation_Bonus_RunDoesNotDuplicateRegularSalary proves
// computeBonusPayslips takes the dedicated branch rather than falling through
// to the normal salary-structure computation, which would double-pay basic
// pay alongside the bonus.
func TestIntegration_Compensation_Bonus_RunDoesNotDuplicateRegularSalary(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedCompFixture(t, env, "50000", "Grade-"+uniqueSlug("g"))
	now := time.Now()
	year, month := now.Year(), int(now.Month())

	b, err := env.hrmCompensationSvc.CreateBonus(ctx, fx.orgID, fx.ownerID, compensation.CreateBonusRequest{
		EmployeeID: fx.employeeID, BonusType: "discretionary", PeriodYear: year, PeriodMonth: &month,
		Amount: "1000",
	})
	if err != nil {
		t.Fatalf("create bonus: %v", err)
	}
	if _, err := env.hrmCompensationSvc.SubmitBonus(ctx, fx.orgID, b.ID, fx.ownerID); err != nil {
		t.Fatalf("submit bonus: %v", err)
	}

	run, err := env.hrmPayslipsSvc.CreateRun(ctx, fx.orgID, fx.ownerID, hrmpayslips.CreateRunRequest{
		Year: year, Month: month, RunType: strPtr("bonus"),
	})
	if err != nil {
		t.Fatalf("create bonus run: %v", err)
	}
	computed, err := env.hrmPayslipsSvc.ComputeRun(ctx, fx.orgID, run.ID, fx.ownerID)
	if err != nil {
		t.Fatalf("compute bonus run: %v", err)
	}
	// 50000 basic pay must NOT appear here — only the 1000 bonus.
	if !computed.TotalGrossPay.Equal(dec(t, "1000")) {
		t.Errorf("bonus run gross = %s, want exactly 1000 (basic pay must not be duplicated)", computed.TotalGrossPay)
	}
}

func TestIntegration_Compensation_Bonus_ZeroPendingBonuses_ComputesEmptyRun(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedCompFixture(t, env, "50000", "Grade-"+uniqueSlug("g"))
	now := time.Now()

	run, err := env.hrmPayslipsSvc.CreateRun(ctx, fx.orgID, fx.ownerID, hrmpayslips.CreateRunRequest{
		Year: now.Year(), Month: int(now.Month()), RunType: strPtr("bonus"),
	})
	if err != nil {
		t.Fatalf("create bonus run: %v", err)
	}
	computed, err := env.hrmPayslipsSvc.ComputeRun(ctx, fx.orgID, run.ID, fx.ownerID)
	if err != nil {
		t.Fatalf("compute bonus run with no pending bonuses: %v", err)
	}
	if computed.TotalEmployees != 0 {
		t.Errorf("expected zero payslips with no approved bonuses, got %d", computed.TotalEmployees)
	}
	if computed.Status != hrmpayslips.RunComputed {
		t.Errorf("expected the run to still reach computed status, got %s", computed.Status)
	}
}

func TestIntegration_Compensation_Bonus_MonthlessAnnualBonusMatchesAnyMonth(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedCompFixture(t, env, "50000", "Grade-"+uniqueSlug("g"))
	now := time.Now()

	b, err := env.hrmCompensationSvc.CreateBonus(ctx, fx.orgID, fx.ownerID, compensation.CreateBonusRequest{
		EmployeeID: fx.employeeID, BonusType: "retention", PeriodYear: now.Year(), PeriodMonth: nil,
		Amount: "5000",
	})
	if err != nil {
		t.Fatalf("create annual bonus: %v", err)
	}
	if _, err := env.hrmCompensationSvc.SubmitBonus(ctx, fx.orgID, b.ID, fx.ownerID); err != nil {
		t.Fatalf("submit bonus: %v", err)
	}

	run, err := env.hrmPayslipsSvc.CreateRun(ctx, fx.orgID, fx.ownerID, hrmpayslips.CreateRunRequest{
		Year: now.Year(), Month: int(now.Month()), RunType: strPtr("bonus"),
	})
	if err != nil {
		t.Fatalf("create bonus run: %v", err)
	}
	computed, err := env.hrmPayslipsSvc.ComputeRun(ctx, fx.orgID, run.ID, fx.ownerID)
	if err != nil {
		t.Fatalf("compute bonus run: %v", err)
	}
	if computed.TotalEmployees != 1 {
		t.Errorf("expected the month-less annual bonus to be picked up by this month's run, got %d employees", computed.TotalEmployees)
	}
}

// ============================================================
// Scope tiers
// ============================================================

func TestIntegration_Compensation_ScopeTiers_ViewOwnSeesOnlyOwnBonuses(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)

	aliceEmail := uniqueEmail("comp-alice")
	alice, err := env.authSvc.Signup(ctx, auth.SignupRequest{Email: aliceEmail, Password: "AlicePass123!"})
	if err != nil {
		t.Fatalf("signup alice: %v", err)
	}
	t.Cleanup(func() { cleanupUser(t, env, alice.ID) })
	aliceEmpID := seedEmployee(t, env, orgID, statusID, ownerID, alice.ID, "Alice", nil)
	bobEmpID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Bob", nil)

	now := time.Now()
	for _, empID := range []string{aliceEmpID, bobEmpID} {
		if _, err := env.hrmCompensationSvc.CreateBonus(ctx, orgID, ownerID, compensation.CreateBonusRequest{
			EmployeeID: empID, BonusType: "other", PeriodYear: now.Year(), PeriodMonth: strPtrInt(int(now.Month())),
			Amount: "100",
		}); err != nil {
			t.Fatalf("create bonus for %s: %v", empID, err)
		}
	}

	res, err := env.hrmCompensationSvc.ListBonuses(ctx, orgID, compensation.ListFilter{
		Scope: authz.ScopeOwn, CallerUserID: alice.ID,
	})
	if err != nil {
		t.Fatalf("list bonuses with ScopeOwn: %v", err)
	}
	if len(res.Bonuses) != 1 {
		t.Fatalf("ScopeOwn should return exactly Alice's bonus, got %d", len(res.Bonuses))
	}
	if res.Bonuses[0].EmployeeID != aliceEmpID {
		t.Errorf("ScopeOwn returned the wrong employee's bonus")
	}

	all, err := env.hrmCompensationSvc.ListBonuses(ctx, orgID, compensation.ListFilter{Scope: authz.ScopeAll})
	if err != nil {
		t.Fatalf("list bonuses with ScopeAll: %v", err)
	}
	if len(all.Bonuses) != 2 {
		t.Errorf("ScopeAll should return both bonuses, got %d", len(all.Bonuses))
	}
}

func strPtr(s string) *string { return &s }
func strPtrInt(i int) *int    { return &i }
