// backend/internal/tests/integration/expenses_test.go
// hrm/expenses against real Postgres. The load-bearing claims are that
// approval is per-LINE (the claim carries no total of its own), that a
// snapshotted FX rate and an effective-dated policy cap cannot be rewritten
// later, that all three advance-settlement outcomes work, and that an
// approved claim reaches payroll through 7C's existing reimbursement seam.
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"testing"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/approvals"
	"github.com/mridha/businesssaas/internal/hrm/expenses"
	"github.com/mridha/businesssaas/internal/hrm/reimbursements"
)

// expenseFixture is an org with an employee linked to a real user, so the
// self-service create paths (which resolve the caller's own employeeID) work.
type expenseFixture struct {
	orgID      string
	statusID   string
	ownerID    string
	employeeID string
}

func seedExpenseFixture(t *testing.T, env *testEnv) *expenseFixture {
	t.Helper()
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	empID := seedEmployee(t, env, orgID, statusID, ownerID, ownerID, "Traveller", nil)
	_ = ctx
	return &expenseFixture{orgID: orgID, statusID: statusID, ownerID: ownerID, employeeID: empID}
}

func strPtrExp(s string) *string { return &s }

// newClaimWithLines creates a draft claim and adds the given lines.
// amounts are "amount|currency|rate" triples in the "other" category.
func newClaimWithLines(t *testing.T, env *testEnv, fx *expenseFixture, advanceID *string, lines ...[3]string) *expenses.Claim {
	t.Helper()
	ctx := context.Background()
	c, err := env.hrmExpensesSvc.CreateClaim(ctx, fx.orgID, fx.ownerID, expenses.CreateClaimRequest{
		Title: "Claim " + uniqueSlug("c"), AdvanceID: advanceID,
	})
	if err != nil {
		t.Fatalf("create claim: %v", err)
	}
	for i, l := range lines {
		amount, currency, rate := l[0], l[1], l[2]
		req := expenses.CreateLineRequest{
			Category: "other", ExpenseDate: "2026-03-15",
			Amount: &amount, DisplayOrder: &[]int{i}[0],
		}
		if currency != "" {
			req.Currency = &currency
		}
		if rate != "" {
			req.ExchangeRate = &rate
		}
		if _, err := env.hrmExpensesSvc.AddLine(ctx, fx.orgID, c.ID, req); err != nil {
			t.Fatalf("add line %d: %v", i, err)
		}
	}
	got, err := env.hrmExpensesSvc.GetClaim(ctx, fx.orgID, c.ID)
	if err != nil {
		t.Fatalf("re-read claim: %v", err)
	}
	return got
}

// ============================================================
// Line-level approval — the claim has no total of its own
// ============================================================

// TestIntegration_Expenses_NoStoredTotalColumns proves the structural claim:
// hrm_expense_claims carries neither total, because approval is per-line and
// a stored total would drift the moment one line was adjusted. Introspecting
// information_schema is the only way to prove a column is absent.
func TestIntegration_Expenses_NoStoredTotalColumns(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	for _, col := range []string{"total_amount", "total_approved_amount", "total_claimed", "total_approved"} {
		var n int
		if err := env.db.QueryRow(ctx,
			`SELECT COUNT(*) FROM information_schema.columns
			  WHERE table_name = 'hrm_expense_claims' AND column_name = $1`, col).Scan(&n); err != nil {
			t.Fatalf("introspect %s: %v", col, err)
		}
		if n != 0 {
			t.Errorf("hrm_expense_claims.%s EXISTS — claim totals must be SUM over lines, never stored", col)
		}
	}
}

// TestIntegration_Expenses_ReducingOneLineLeavesOthersUntouched is the
// build plan's line-level requirement made concrete.
func TestIntegration_Expenses_ReducingOneLineLeavesOthersUntouched(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedExpenseFixture(t, env)

	c := newClaimWithLines(t, env, fx, nil,
		[3]string{"400", "", ""}, // flight
		[3]string{"120", "", ""}, // hotel extras
		[3]string{"60", "", ""},  // taxi
	)
	if !c.TotalClaimed.Equal(dec(t, "580")) {
		t.Fatalf("claimed = %s, want 580", c.TotalClaimed)
	}
	if c.UndecidedLines == nil || *c.UndecidedLines != 3 {
		t.Fatalf("undecided = %v, want 3", c.UndecidedLines)
	}

	if _, err := env.hrmExpensesSvc.SubmitClaim(ctx, fx.orgID, c.ID, fx.ownerID); err != nil {
		t.Fatalf("submit: %v", err)
	}

	full, err := env.hrmExpensesSvc.GetClaim(ctx, fx.orgID, c.ID)
	if err != nil {
		t.Fatalf("get claim: %v", err)
	}
	// Approve the flight in full, TRIM the hotel extras, approve the taxi.
	decisions := map[int]string{0: "400", 1: "40", 2: "60"}
	for idx, amt := range decisions {
		if _, err := env.hrmExpensesSvc.ApproveLine(ctx, fx.orgID, c.ID, full.Lines[idx].ID,
			expenses.ApproveLineRequest{ApprovedAmount: amt}); err != nil {
			t.Fatalf("approve line %d: %v", idx, err)
		}
	}

	after, err := env.hrmExpensesSvc.GetClaim(ctx, fx.orgID, c.ID)
	if err != nil {
		t.Fatalf("get claim after approvals: %v", err)
	}
	if !after.TotalClaimed.Equal(dec(t, "580")) {
		t.Errorf("claimed changed to %s — trimming a line must not alter what was CLAIMED", after.TotalClaimed)
	}
	if !after.TotalApproved.Equal(dec(t, "500")) {
		t.Errorf("approved = %s, want 500 (400 + trimmed 40 + 60)", after.TotalApproved)
	}
	// The untouched lines kept their own amounts.
	if !after.Lines[0].ApprovedAmount.Equal(dec(t, "400")) {
		t.Errorf("line 0 approved = %s, want 400 — trimming line 1 must not touch it", after.Lines[0].ApprovedAmount)
	}
	if !after.Lines[2].ApprovedAmount.Equal(dec(t, "60")) {
		t.Errorf("line 2 approved = %s, want 60", after.Lines[2].ApprovedAmount)
	}
	if after.Status != expenses.ClaimApproved {
		t.Errorf("status = %s, want approved once every line is decided", after.Status)
	}
}

// TestIntegration_Expenses_ZeroApprovalIsDecidedNotUndecided pins why
// approved_amount is nullable rather than DEFAULT 0.
func TestIntegration_Expenses_ZeroApprovalIsDecidedNotUndecided(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedExpenseFixture(t, env)

	c := newClaimWithLines(t, env, fx, nil, [3]string{"100", "", ""}, [3]string{"80", "", ""})
	if _, err := env.hrmExpensesSvc.SubmitClaim(ctx, fx.orgID, c.ID, fx.ownerID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	full, _ := env.hrmExpensesSvc.GetClaim(ctx, fx.orgID, c.ID)

	// Decide the first at full, the second at ZERO — a real rejection.
	if _, err := env.hrmExpensesSvc.ApproveLine(ctx, fx.orgID, c.ID, full.Lines[0].ID,
		expenses.ApproveLineRequest{ApprovedAmount: "100"}); err != nil {
		t.Fatalf("approve line 0: %v", err)
	}
	after, err := env.hrmExpensesSvc.ApproveLine(ctx, fx.orgID, c.ID, full.Lines[1].ID,
		expenses.ApproveLineRequest{ApprovedAmount: "0"})
	if err != nil {
		t.Fatalf("approve line 1 at zero: %v", err)
	}

	if after.UndecidedLines == nil || *after.UndecidedLines != 0 {
		t.Errorf("undecided = %v, want 0 — a zero decision IS a decision", after.UndecidedLines)
	}
	if after.Status != expenses.ClaimApproved {
		t.Errorf("status = %s, want approved", after.Status)
	}
	if !after.TotalApproved.Equal(dec(t, "100")) {
		t.Errorf("approved = %s, want 100", after.TotalApproved)
	}
}

func TestIntegration_Expenses_CannotApproveMoreThanClaimed(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedExpenseFixture(t, env)

	c := newClaimWithLines(t, env, fx, nil, [3]string{"100", "", ""})
	if _, err := env.hrmExpensesSvc.SubmitClaim(ctx, fx.orgID, c.ID, fx.ownerID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	full, _ := env.hrmExpensesSvc.GetClaim(ctx, fx.orgID, c.ID)

	_, err := env.hrmExpensesSvc.ApproveLine(ctx, fx.orgID, c.ID, full.Lines[0].ID,
		expenses.ApproveLineRequest{ApprovedAmount: "250"})
	if err == nil {
		t.Fatal("approving MORE than was claimed succeeded — an approver may only reduce")
	}
}

// TestIntegration_Expenses_LineIsRevisableUntilPaid — deciding the LAST line
// flips a claim to approved, and locking there would leave an approver who
// mistyped it with no remedy. Only settlement is final.
func TestIntegration_Expenses_LineIsRevisableUntilPaid(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedExpenseFixture(t, env)

	c := newClaimWithLines(t, env, fx, nil, [3]string{"400", "", ""}, [3]string{"200", "", ""})
	if _, err := env.hrmExpensesSvc.SubmitClaim(ctx, fx.orgID, c.ID, fx.ownerID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	full, _ := env.hrmExpensesSvc.GetClaim(ctx, fx.orgID, c.ID)

	// Decide both — the second decision flips the claim to approved.
	for i, amt := range []string{"400", "200"} {
		if _, err := env.hrmExpensesSvc.ApproveLine(ctx, fx.orgID, c.ID, full.Lines[i].ID,
			expenses.ApproveLineRequest{ApprovedAmount: amt}); err != nil {
			t.Fatalf("approve line %d: %v", i, err)
		}
	}
	approved, _ := env.hrmExpensesSvc.GetClaim(ctx, fx.orgID, c.ID)
	if approved.Status != expenses.ClaimApproved {
		t.Fatalf("status = %s, want approved", approved.Status)
	}

	// The approver realises line 0 was wrong and corrects it.
	revised, err := env.hrmExpensesSvc.ApproveLine(ctx, fx.orgID, c.ID, full.Lines[0].ID,
		expenses.ApproveLineRequest{ApprovedAmount: "350"})
	if err != nil {
		t.Fatalf("revising a line on an approved-but-unpaid claim failed: %v", err)
	}
	if !revised.TotalApproved.Equal(dec(t, "550")) {
		t.Errorf("approved = %s, want 550 after the correction", revised.TotalApproved)
	}

	// Once SETTLED, it is final — the money has reached payroll.
	if _, err := env.hrmExpensesSvc.SettleClaim(ctx, fx.orgID, c.ID, fx.ownerID); err != nil {
		t.Fatalf("settle: %v", err)
	}
	if _, err := env.hrmExpensesSvc.ApproveLine(ctx, fx.orgID, c.ID, full.Lines[0].ID,
		expenses.ApproveLineRequest{ApprovedAmount: "100"}); err == nil {
		t.Error("a line was revised AFTER settlement — the reimbursement is already with payroll")
	}
}

// ============================================================
// Multi-currency — the rate is snapshotted, never re-derived
// ============================================================

func TestIntegration_Expenses_ExchangeRateIsSnapshottedOnTheLine(t *testing.T) {
	env := newTestEnv(t)
	fx := seedExpenseFixture(t, env)

	// 100 EUR at 1.08 -> 108 base.
	c := newClaimWithLines(t, env, fx, nil, [3]string{"100", "EUR", "1.08"})
	if !c.Lines[0].BaseAmount.Equal(dec(t, "108")) {
		t.Errorf("base_amount = %s, want 108", c.Lines[0].BaseAmount)
	}
	if !c.Lines[0].Amount.Equal(dec(t, "100")) {
		t.Errorf("amount = %s, want the original 100 EUR preserved", c.Lines[0].Amount)
	}
	if c.Lines[0].Currency != "EUR" {
		t.Errorf("currency = %s, want EUR", c.Lines[0].Currency)
	}
	if !c.Lines[0].ExchangeRate.Equal(dec(t, "1.08")) {
		t.Errorf("exchange_rate = %s, want the snapshotted 1.08", c.Lines[0].ExchangeRate)
	}
	// The claim totals in base currency.
	if !c.TotalClaimed.Equal(dec(t, "108")) {
		t.Errorf("claimed = %s, want 108 (base)", c.TotalClaimed)
	}

	// Mixed-currency claim sums correctly in base.
	c2 := newClaimWithLines(t, env, fx, nil,
		[3]string{"100", "EUR", "1.08"},
		[3]string{"50", "USD", "1"},
	)
	if !c2.TotalClaimed.Equal(dec(t, "158")) {
		t.Errorf("mixed-currency claimed = %s, want 158", c2.TotalClaimed)
	}
}

// ============================================================
// Policy violations — warnings, never blocks
// ============================================================

func TestIntegration_Expenses_OverPolicyLineIsWarnedNotBlocked(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedExpenseFixture(t, env)

	if _, err := env.hrmExpensesSvc.CreatePolicy(ctx, fx.orgID, fx.ownerID, expenses.CreatePolicyRequest{
		Category: "other", MaxAmount: "100", EffectiveDate: "2020-01-01",
	}); err != nil {
		t.Fatalf("create policy: %v", err)
	}

	// A 250 line against a 100 cap must still be ACCEPTED.
	c := newClaimWithLines(t, env, fx, nil, [3]string{"250", "", ""})
	if len(c.Lines) != 1 {
		t.Fatalf("the over-policy line was refused — violations must be warnings, not blocks")
	}
	if len(c.Lines[0].Violations) != 1 {
		t.Fatalf("expected exactly 1 recorded violation, got %d", len(c.Lines[0].Violations))
	}
	v := c.Lines[0].Violations[0]
	if !v.MaxAmount.Equal(dec(t, "100")) {
		t.Errorf("violation max = %s, want the snapshotted 100", v.MaxAmount)
	}
	if !v.ActualAmount.Equal(dec(t, "250")) {
		t.Errorf("violation actual = %s, want 250", v.ActualAmount)
	}
	// And it can still be submitted and approved — a breach is reviewable.
	if _, err := env.hrmExpensesSvc.SubmitClaim(ctx, fx.orgID, c.ID, fx.ownerID); err != nil {
		t.Fatalf("submitting an over-policy claim failed: %v", err)
	}
}

func TestIntegration_Expenses_WithinPolicyRaisesNoViolation(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedExpenseFixture(t, env)

	if _, err := env.hrmExpensesSvc.CreatePolicy(ctx, fx.orgID, fx.ownerID, expenses.CreatePolicyRequest{
		Category: "other", MaxAmount: "100", EffectiveDate: "2020-01-01",
	}); err != nil {
		t.Fatalf("create policy: %v", err)
	}
	c := newClaimWithLines(t, env, fx, nil, [3]string{"80", "", ""})
	if len(c.Lines[0].Violations) != 0 {
		t.Errorf("a within-policy line raised %d violations", len(c.Lines[0].Violations))
	}
}

// TestIntegration_Expenses_PolicyCapIsEffectiveDated — the build plan's
// mandatory rule, same as r28's statutory slabs: a cap dated later must not
// retroactively excuse an earlier breach.
func TestIntegration_Expenses_PolicyCapIsEffectiveDated(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedExpenseFixture(t, env)

	// A 100 cap from 2020, raised to 1000 from 2027.
	if _, err := env.hrmExpensesSvc.CreatePolicy(ctx, fx.orgID, fx.ownerID, expenses.CreatePolicyRequest{
		Category: "other", MaxAmount: "100", EffectiveDate: "2020-01-01",
	}); err != nil {
		t.Fatalf("create 2020 policy: %v", err)
	}
	if _, err := env.hrmExpensesSvc.CreatePolicy(ctx, fx.orgID, fx.ownerID, expenses.CreatePolicyRequest{
		Category: "other", MaxAmount: "1000", EffectiveDate: "2027-01-01",
	}); err != nil {
		t.Fatalf("create 2027 policy: %v", err)
	}

	c, err := env.hrmExpensesSvc.CreateClaim(ctx, fx.orgID, fx.ownerID, expenses.CreateClaimRequest{
		Title: "Effective dating " + uniqueSlug("c"),
	})
	if err != nil {
		t.Fatalf("create claim: %v", err)
	}
	// An expense dated 2026 is judged by the 2020 cap of 100 — the future
	// raise must not apply.
	amt := "250"
	if _, err := env.hrmExpensesSvc.AddLine(ctx, fx.orgID, c.ID, expenses.CreateLineRequest{
		Category: "other", ExpenseDate: "2026-06-01", Amount: &amt,
	}); err != nil {
		t.Fatalf("add 2026 line: %v", err)
	}
	// An expense dated 2027 is judged by the 1000 cap — no violation.
	if _, err := env.hrmExpensesSvc.AddLine(ctx, fx.orgID, c.ID, expenses.CreateLineRequest{
		Category: "other", ExpenseDate: "2027-06-01", Amount: &amt,
	}); err != nil {
		t.Fatalf("add 2027 line: %v", err)
	}

	full, err := env.hrmExpensesSvc.GetClaim(ctx, fx.orgID, c.ID)
	if err != nil {
		t.Fatalf("get claim: %v", err)
	}
	var v2026, v2027 int
	for _, l := range full.Lines {
		if l.ExpenseDate.Year() == 2026 {
			v2026 = len(l.Violations)
		} else {
			v2027 = len(l.Violations)
		}
	}
	if v2026 != 1 {
		t.Errorf("the 2026 line raised %d violations, want 1 — the 2020 cap of 100 applies", v2026)
	}
	if v2027 != 0 {
		t.Errorf("the 2027 line raised %d violations, want 0 — the 2027 cap of 1000 applies", v2027)
	}
}

// ============================================================
// Advance settlement — all THREE outcomes, and the 7C payout seam
// ============================================================

func seedDisbursedAdvance(t *testing.T, env *testEnv, fx *expenseFixture, amount string) *expenses.Advance {
	t.Helper()
	ctx := context.Background()
	a, err := env.hrmExpensesSvc.CreateAdvance(ctx, fx.orgID, fx.ownerID, expenses.CreateAdvanceRequest{
		EmployeeID: fx.employeeID, Amount: amount,
	})
	if err != nil {
		t.Fatalf("create advance: %v", err)
	}
	d, err := env.hrmExpensesSvc.DisburseAdvance(ctx, fx.orgID, a.ID, fx.ownerID)
	if err != nil {
		t.Fatalf("disburse advance: %v", err)
	}
	return d
}

// approveAllLines decides every line at its full claimed amount.
func approveAllLines(t *testing.T, env *testEnv, fx *expenseFixture, claimID string) {
	t.Helper()
	ctx := context.Background()
	if _, err := env.hrmExpensesSvc.SubmitClaim(ctx, fx.orgID, claimID, fx.ownerID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	full, err := env.hrmExpensesSvc.GetClaim(ctx, fx.orgID, claimID)
	if err != nil {
		t.Fatalf("get claim: %v", err)
	}
	for _, l := range full.Lines {
		if _, err := env.hrmExpensesSvc.ApproveLine(ctx, fx.orgID, claimID, l.ID,
			expenses.ApproveLineRequest{ApprovedAmount: l.BaseAmount.String()}); err != nil {
			t.Fatalf("approve line: %v", err)
		}
	}
}

func TestIntegration_Expenses_SettlementExact(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedExpenseFixture(t, env)
	adv := seedDisbursedAdvance(t, env, fx, "500")

	c := newClaimWithLines(t, env, fx, &adv.ID, [3]string{"500", "", ""})
	approveAllLines(t, env, fx, c.ID)

	res, err := env.hrmExpensesSvc.SettleClaim(ctx, fx.orgID, c.ID, fx.ownerID)
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	if res.Outcome != expenses.SettlementExact {
		t.Errorf("outcome = %s, want exact", res.Outcome)
	}
	if !res.Payable.IsZero() || !res.Recoverable.IsZero() {
		t.Errorf("payable=%s recoverable=%s, want both zero", res.Payable, res.Recoverable)
	}
	// Nothing payable means NO reimbursement — an empty one would show up in
	// payroll as a zero line nobody can explain.
	if res.ReimbursementID != nil {
		t.Error("a fully-offset claim created a reimbursement; there is nothing for payroll to pay")
	}
	if res.Claim.Status != expenses.ClaimPaid {
		t.Errorf("status = %s, want paid", res.Claim.Status)
	}
}

func TestIntegration_Expenses_SettlementEmployeeOwes(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedExpenseFixture(t, env)
	adv := seedDisbursedAdvance(t, env, fx, "500")

	// Spent only 300 of a 500 advance.
	c := newClaimWithLines(t, env, fx, &adv.ID, [3]string{"300", "", ""})
	approveAllLines(t, env, fx, c.ID)

	res, err := env.hrmExpensesSvc.SettleClaim(ctx, fx.orgID, c.ID, fx.ownerID)
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	if res.Outcome != expenses.SettlementEmployeeOwes {
		t.Errorf("outcome = %s, want employee_owes", res.Outcome)
	}
	if !res.Recoverable.Equal(dec(t, "200")) {
		t.Errorf("recoverable = %s, want 200", res.Recoverable)
	}
	if res.ReimbursementID != nil {
		t.Error("the org owes nothing here; no reimbursement should exist")
	}

	// The advance absorbed only what the claim used.
	var settled string
	if err := env.db.QueryRow(ctx,
		`SELECT to_char(settled_amount,'FM999999990.00') FROM hrm_travel_advances WHERE id=$1`, adv.ID).Scan(&settled); err != nil {
		t.Fatalf("read advance: %v", err)
	}
	if settled != "300.00" {
		t.Errorf("advance settled_amount = %s, want 300.00", settled)
	}
}

// TestIntegration_Expenses_SettlementOrgOwes_CreatesReimbursement is the 7C
// boundary end to end: the shortfall becomes a hrm_reimbursements row that
// 7C's payslips.ReimbursementSource will pay out — without 8B touching
// hrm/payslips at all.
func TestIntegration_Expenses_SettlementOrgOwes_CreatesReimbursement(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedExpenseFixture(t, env)
	adv := seedDisbursedAdvance(t, env, fx, "500")

	// Spent 800 against a 500 advance — the org owes 300.
	c := newClaimWithLines(t, env, fx, &adv.ID, [3]string{"800", "", ""})
	approveAllLines(t, env, fx, c.ID)

	res, err := env.hrmExpensesSvc.SettleClaim(ctx, fx.orgID, c.ID, fx.ownerID)
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	if res.Outcome != expenses.SettlementOrgOwes {
		t.Errorf("outcome = %s, want org_owes", res.Outcome)
	}
	if !res.Payable.Equal(dec(t, "300")) {
		t.Errorf("payable = %s, want 300", res.Payable)
	}
	if res.ReimbursementID == nil {
		t.Fatal("no reimbursement was created — the shortfall would never reach payroll")
	}

	// The reimbursement is real, belongs to the employee, and is for the
	// shortfall — NOT the whole claim.
	r, err := env.hrmReimbursementsSvc.Get(ctx, fx.orgID, *res.ReimbursementID)
	if err != nil {
		t.Fatalf("read reimbursement: %v", err)
	}
	if r.EmployeeID != fx.employeeID {
		t.Errorf("reimbursement employee = %s, want %s", r.EmployeeID, fx.employeeID)
	}
	if !r.Amount.Equal(dec(t, "300")) {
		t.Errorf("reimbursement amount = %s, want 300 (the shortfall, not the 800 claim)", r.Amount)
	}
	if r.Category != reimbursements.CategoryTravel {
		t.Errorf("reimbursement category = %s, want travel", r.Category)
	}

	// The whole advance was consumed.
	var settled, status string
	if err := env.db.QueryRow(ctx,
		`SELECT to_char(settled_amount,'FM999999990.00'), status FROM hrm_travel_advances WHERE id=$1`,
		adv.ID).Scan(&settled, &status); err != nil {
		t.Fatalf("read advance: %v", err)
	}
	if settled != "500.00" || status != "settled" {
		t.Errorf("advance settled=%s status=%s, want 500.00 / settled", settled, status)
	}
}

func TestIntegration_Expenses_SettlementWithNoAdvancePaysTheWholeClaim(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedExpenseFixture(t, env)

	c := newClaimWithLines(t, env, fx, nil, [3]string{"420", "", ""})
	approveAllLines(t, env, fx, c.ID)

	res, err := env.hrmExpensesSvc.SettleClaim(ctx, fx.orgID, c.ID, fx.ownerID)
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	if !res.Payable.Equal(dec(t, "420")) {
		t.Errorf("payable = %s, want the full 420", res.Payable)
	}
	if res.ReimbursementID == nil {
		t.Fatal("expected a reimbursement for the full claim")
	}
}

// TestIntegration_Expenses_SettleRefusesWhileLinesAreUndecided — silently
// treating an unreviewed line as zero would underpay the employee.
func TestIntegration_Expenses_SettleRefusesWhileLinesAreUndecided(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedExpenseFixture(t, env)

	c := newClaimWithLines(t, env, fx, nil, [3]string{"100", "", ""}, [3]string{"200", "", ""})
	if _, err := env.hrmExpensesSvc.SubmitClaim(ctx, fx.orgID, c.ID, fx.ownerID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	full, _ := env.hrmExpensesSvc.GetClaim(ctx, fx.orgID, c.ID)

	// Decide only ONE of the two lines.
	if _, err := env.hrmExpensesSvc.ApproveLine(ctx, fx.orgID, c.ID, full.Lines[0].ID,
		expenses.ApproveLineRequest{ApprovedAmount: "100"}); err != nil {
		t.Fatalf("approve line 0: %v", err)
	}

	partial, err := env.hrmExpensesSvc.GetClaim(ctx, fx.orgID, c.ID)
	if err != nil {
		t.Fatalf("get claim: %v", err)
	}
	if partial.Status != expenses.ClaimPartiallyApproved {
		t.Errorf("status = %s, want partially_approved", partial.Status)
	}
	if _, err := env.hrmExpensesSvc.SettleClaim(ctx, fx.orgID, c.ID, fx.ownerID); err == nil {
		t.Fatal("settling with an undecided line succeeded — it would have been paid as zero")
	}
}

// ============================================================
// Travel approval + scope
// ============================================================

func TestIntegration_Expenses_TravelApprovalEndToEnd(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedExpenseFixture(t, env)

	if _, err := env.hrmApprovalsSvc.CreateTemplate(ctx, fx.orgID, fx.ownerID, approvals.CreateTemplateRequest{
		Name: "Travel Approval", ActionType: approvals.ActionTypeTravelRequest, IsDefault: true,
		Levels: []approvals.CreateTemplateLevelRequest{
			{Level: 1, ApproverType: approvals.ApproverTypeSpecificUser, ApproverUserID: &fx.ownerID, SLAHours: 24, OnSLABreach: approvals.SLABreachEscalateNext},
		},
	}); err != nil {
		t.Fatalf("create template: %v", err)
	}

	tr, err := env.hrmExpensesSvc.CreateTravel(ctx, fx.orgID, fx.ownerID, expenses.CreateTravelRequest{
		Purpose: "Client visit", Destination: "Berlin", DestinationCountry: strPtrExp("DE"),
		StartDate: "2026-09-01", EndDate: "2026-09-05",
	})
	if err != nil {
		t.Fatalf("create travel: %v", err)
	}
	if tr.EmployeeID != fx.employeeID {
		t.Errorf("travel raised for %s, want the caller's own employee", tr.EmployeeID)
	}
	if got := tr.DurationDays(); got != 5 {
		t.Errorf("duration = %d days, want 5 (inclusive of both endpoints)", got)
	}

	if _, err := env.hrmExpensesSvc.AddItineraryItem(ctx, fx.orgID, tr.ID, expenses.CreateItineraryItemRequest{
		ItemType: "flight", FromLocation: strPtrExp("LHR"), ToLocation: strPtrExp("BER"),
		EstimatedCost: strPtrExp("320"),
	}); err != nil {
		t.Fatalf("add itinerary: %v", err)
	}
	itin, err := env.hrmExpensesSvc.ListItinerary(ctx, fx.orgID, tr.ID)
	if err != nil || len(itin) != 1 {
		t.Fatalf("itinerary: %v (%d items)", err, len(itin))
	}

	submitted, err := env.hrmExpensesSvc.SubmitTravel(ctx, fx.orgID, tr.ID, fx.ownerID)
	if err != nil {
		t.Fatalf("submit travel: %v", err)
	}
	if submitted.Status != expenses.TravelPendingApproval {
		t.Fatalf("status = %s, want pending_approval", submitted.Status)
	}
	if _, err := env.hrmApprovalsSvc.Decide(ctx, fx.orgID, *submitted.ApprovalInstanceID, fx.ownerID,
		approvals.DecisionRequest{Action: "approved"}); err != nil {
		t.Fatalf("decide: %v", err)
	}
	final, err := env.hrmExpensesSvc.GetTravel(ctx, fx.orgID, tr.ID)
	if err != nil {
		t.Fatalf("get travel: %v", err)
	}
	if final.Status != expenses.TravelApproved {
		t.Errorf("status = %s — the approval callback did not reach the travel request", final.Status)
	}
}

func TestIntegration_Expenses_ScopeOwnSeesOnlyOwnClaims(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedExpenseFixture(t, env)

	aliceEmail := uniqueEmail("expense-alice")
	alice, err := env.authSvc.Signup(ctx, authSignup(aliceEmail))
	if err != nil {
		t.Fatalf("signup alice: %v", err)
	}
	t.Cleanup(func() { cleanupUser(t, env, alice.ID) })
	seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, alice.ID, "Alice", nil)

	// One claim for the owner's employee, one for Alice.
	newClaimWithLines(t, env, fx, nil, [3]string{"100", "", ""})
	if _, err := env.hrmExpensesSvc.CreateClaim(ctx, fx.orgID, alice.ID, expenses.CreateClaimRequest{
		Title: "Alice claim " + uniqueSlug("c"),
	}); err != nil {
		t.Fatalf("create alice claim: %v", err)
	}

	own, err := env.hrmExpensesSvc.ListClaims(ctx, fx.orgID, expenses.ListFilter{
		Scope: authz.ScopeOwn, CallerUserID: alice.ID,
	})
	if err != nil {
		t.Fatalf("list with ScopeOwn: %v", err)
	}
	if len(own.Claims) != 1 {
		t.Fatalf("ScopeOwn returned %d claims, want exactly Alice's 1", len(own.Claims))
	}

	all, err := env.hrmExpensesSvc.ListClaims(ctx, fx.orgID, expenses.ListFilter{Scope: authz.ScopeAll})
	if err != nil {
		t.Fatalf("list with ScopeAll: %v", err)
	}
	if all.Total != 2 {
		t.Errorf("ScopeAll total = %d, want 2", all.Total)
	}
}

// TestIntegration_Expenses_MileageUsesTheRateInForceOnTheExpenseDate
func TestIntegration_Expenses_MileageUsesEffectiveDatedRate(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedExpenseFixture(t, env)

	if _, err := env.hrmExpensesSvc.CreateMileageRate(ctx, fx.orgID, fx.ownerID, expenses.CreateMileageRateRequest{
		RatePerUnit: "0.45", Unit: strPtrExp("km"), EffectiveDate: "2020-01-01",
	}); err != nil {
		t.Fatalf("create 2020 rate: %v", err)
	}
	if _, err := env.hrmExpensesSvc.CreateMileageRate(ctx, fx.orgID, fx.ownerID, expenses.CreateMileageRateRequest{
		RatePerUnit: "0.90", Unit: strPtrExp("km"), EffectiveDate: "2027-01-01",
	}); err != nil {
		t.Fatalf("create 2027 rate: %v", err)
	}

	c, err := env.hrmExpensesSvc.CreateClaim(ctx, fx.orgID, fx.ownerID, expenses.CreateClaimRequest{
		Title: "Mileage " + uniqueSlug("c"),
	})
	if err != nil {
		t.Fatalf("create claim: %v", err)
	}
	dist := "100"
	if _, err := env.hrmExpensesSvc.AddLine(ctx, fx.orgID, c.ID, expenses.CreateLineRequest{
		Category: "mileage", ExpenseDate: "2026-06-01", MileageDistance: &dist,
	}); err != nil {
		t.Fatalf("add mileage line: %v", err)
	}

	full, err := env.hrmExpensesSvc.GetClaim(ctx, fx.orgID, c.ID)
	if err != nil {
		t.Fatalf("get claim: %v", err)
	}
	// 100 km at the 2020 rate of 0.45 = 45, NOT the 2027 rate of 0.90.
	if !full.Lines[0].Amount.Equal(dec(t, "45")) {
		t.Errorf("mileage amount = %s, want 45 (100km at the 2020 rate in force on the expense date)", full.Lines[0].Amount)
	}
	if full.Lines[0].MileageRateID == nil {
		t.Error("the mileage line did not record which rate it used")
	}
}
