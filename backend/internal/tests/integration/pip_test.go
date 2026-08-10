// backend/internal/tests/integration/pip_test.go
// Phase 5C PIP against real Postgres — what a stub cannot prove: that the
// failed-plan handoff lands a real row in hrm_terminations at status 'draft'
// and no further, that the extension and its written reason share a
// transaction, that the partial unique index actually enforces one open plan
// per employee, and that deleting the draft termination does not take the
// PIP's record of what happened with it.
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/mridha/businesssaas/internal/authz"
	hrmpip "github.com/mridha/businesssaas/internal/hrm/pip"
)

func pipAdmin(userID string) hrmpip.Caller {
	return hrmpip.Caller{UserID: userID, Tier: authz.ScopeAll, CanManage: true, CanClose: true}
}

// pipManager holds manage but not close — the grant migration 00091 gives the
// 'manager' role.
func pipManager(userID string) hrmpip.Caller {
	return hrmpip.Caller{UserID: userID, Tier: authz.ScopeAll, CanManage: true, CanClose: false}
}

type pipFixture struct {
	orgID    string
	statusID string
	ownerID  string
	empID    string
}

func seedPipFixture(t *testing.T, env *testEnv) *pipFixture {
	t.Helper()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	empID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Under Review", nil)
	return &pipFixture{orgID: orgID, statusID: statusID, ownerID: ownerID, empID: empID}
}

func seedActivePip(t *testing.T, env *testEnv, fx *pipFixture) *hrmpip.Detail {
	t.Helper()
	ctx := context.Background()
	p, err := env.hrmPipSvc.Create(ctx, fx.orgID, fx.ownerID, pipAdmin(fx.ownerID), hrmpip.CreateRequest{
		EmployeeID: fx.empID, Title: "Improve delivery predictability",
		Concerns:        "Three consecutive missed commitments",
		SuccessCriteria: "Two consecutive on-time sprints",
		StartDate:       "2030-01-01", EndDate: "2030-03-31",
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	out, err := env.hrmPipSvc.Activate(ctx, fx.orgID, p.ID, pipAdmin(fx.ownerID))
	if err != nil {
		t.Fatalf("activate plan: %v", err)
	}
	return out
}

// ============================================================
// The failed-PIP handoff, against the real terminations service
// ============================================================

// TestIntegration_PIP_FailedCreatesDraftTerminationAndStops is the headline
// test. A stub can be told it created a termination; only this proves a row
// exists in hrm_terminations, that it is at status 'draft', and — the part
// that matters — that nothing advanced it past draft, because the approval
// chain is what gates dismissals.
func TestIntegration_PIP_FailedCreatesDraftTerminationAndStops(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedPipFixture(t, env)
	p := seedActivePip(t, env, fx)

	out, err := env.hrmPipSvc.Close(ctx, fx.orgID, p.ID, pipAdmin(fx.ownerID), hrmpip.CloseRequest{
		Outcome: hrmpip.OutcomeFailed, Note: "Criteria not met after the full period",
	})
	if err != nil {
		t.Fatalf("close as failed: %v", err)
	}
	if out.TerminationID == nil {
		t.Fatal("no draft termination was created")
	}

	var status, termType, employeeID string
	var reason *string
	if err := env.db.QueryRow(ctx,
		`SELECT status, termination_type, employee_id::text, reason
		   FROM hrm_terminations WHERE id = $1`, *out.TerminationID,
	).Scan(&status, &termType, &employeeID, &reason); err != nil {
		t.Fatalf("read the draft termination: %v", err)
	}

	if status != "draft" {
		t.Errorf("termination status = %s, want draft — a PIP must never advance past draft, "+
			"because the approval chain exists specifically to gate dismissals", status)
	}
	// 'involuntary', not 'probation_fail': a PIP is a performance process for
	// a confirmed employee, and probation failure carries different notice
	// and severance rules in most jurisdictions.
	if termType != "involuntary" {
		t.Errorf("termination_type = %s, want involuntary", termType)
	}
	if employeeID != fx.empID {
		t.Errorf("termination raised against %s, want %s", employeeID, fx.empID)
	}
	if reason == nil || !containsStr(*reason, p.PublicID) {
		t.Errorf("termination reason %v should name the PIP %s so it is traceable", reason, p.PublicID)
	}

	// No approval instance was created either — Submit is what does that, and
	// the handoff deliberately does not call it.
	var approvals int
	if err := env.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrm_approval_instances
		  WHERE entity_type = 'termination' AND entity_id = $1`, *out.TerminationID).Scan(&approvals); err != nil {
		t.Fatalf("count approval instances: %v", err)
	}
	if approvals != 0 {
		t.Errorf("the handoff submitted the termination for approval; it must stop at draft (%d instances)", approvals)
	}
}

// TestIntegration_PIP_SuccessfulCloseCreatesNoTermination is the negative
// half, and the one worth having: a bug that fired the handoff on every
// close would be invisible without it.
func TestIntegration_PIP_SuccessfulCloseCreatesNoTermination(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedPipFixture(t, env)
	p := seedActivePip(t, env, fx)

	out, err := env.hrmPipSvc.Close(ctx, fx.orgID, p.ID, pipAdmin(fx.ownerID), hrmpip.CloseRequest{
		Outcome: hrmpip.OutcomeSuccessful, Note: "Criteria met with room to spare",
	})
	if err != nil {
		t.Fatalf("close as successful: %v", err)
	}
	if out.TerminationID != nil {
		t.Errorf("a successful plan created a termination: %v", out.TerminationID)
	}

	var n int
	if err := env.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrm_terminations WHERE employee_id = $1`, fx.empID).Scan(&n); err != nil {
		t.Fatalf("count terminations: %v", err)
	}
	if n != 0 {
		t.Errorf("expected no termination rows, got %d", n)
	}
}

// TestIntegration_PIP_CloseRequiresClosePermission proves the 00091 grant
// split has teeth against the real service: 'manager' runs the plan but does
// not get to pull the trigger.
func TestIntegration_PIP_CloseRequiresClosePermission(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedPipFixture(t, env)
	p := seedActivePip(t, env, fx)

	if _, err := env.hrmPipSvc.Close(ctx, fx.orgID, p.ID, pipManager(fx.ownerID), hrmpip.CloseRequest{
		Outcome: hrmpip.OutcomeFailed, Note: "Not met",
	}); !errors.Is(err, hrmpip.ErrCloseDenied) {
		t.Fatalf("expected ErrCloseDenied, got %v", err)
	}

	var n int
	if err := env.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrm_terminations WHERE employee_id = $1`, fx.empID).Scan(&n); err != nil {
		t.Fatalf("count terminations: %v", err)
	}
	if n != 0 {
		t.Errorf("a denied close reached the handoff anyway: %d terminations", n)
	}
}

// TestIntegration_PIP_DeletingTheDraftKeepsTheOutcome pins the ON DELETE SET
// NULL choice. Deleting a mistakenly-created draft termination must not be
// blocked by the PIP that suggested it, and must not erase the PIP's record
// of having failed.
func TestIntegration_PIP_DeletingTheDraftKeepsTheOutcome(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedPipFixture(t, env)
	p := seedActivePip(t, env, fx)

	out, err := env.hrmPipSvc.Close(ctx, fx.orgID, p.ID, pipAdmin(fx.ownerID), hrmpip.CloseRequest{
		Outcome: hrmpip.OutcomeFailed, Note: "Criteria not met",
	})
	if err != nil {
		t.Fatalf("close: %v", err)
	}

	if _, err := env.db.Exec(ctx, `DELETE FROM hrm_terminations WHERE id = $1`, *out.TerminationID); err != nil {
		t.Fatalf("deleting the draft termination was blocked — RESTRICT would trap a mistake: %v", err)
	}

	var terminationID, outcome *string
	if err := env.db.QueryRow(ctx,
		`SELECT termination_id::text, outcome FROM hrm_pips WHERE id = $1`, p.ID,
	).Scan(&terminationID, &outcome); err != nil {
		t.Fatalf("re-read the plan: %v", err)
	}
	if terminationID != nil {
		t.Errorf("expected termination_id nulled by the delete, got %v", *terminationID)
	}
	if outcome == nil || *outcome != string(hrmpip.OutcomeFailed) {
		t.Errorf("the outcome did not survive the delete: %v — the record of what happened "+
			"must not depend on the draft surviving", outcome)
	}
}

// ============================================================
// The extension audit trail is transactional
// ============================================================

// TestIntegration_PIP_ExtendIsTransactionalWithItsReason proves the two
// writes land together. An extension with no written reason is the failure
// mode this instrument has, so the guarantee is a transaction rather than an
// ordering convention.
func TestIntegration_PIP_ExtendIsTransactionalWithItsReason(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedPipFixture(t, env)
	p := seedActivePip(t, env, fx)
	originalEnd := p.OriginalEndDate

	out, err := env.hrmPipSvc.Extend(ctx, fx.orgID, p.ID, pipAdmin(fx.ownerID), hrmpip.ExtendRequest{
		NewEndDate: "2030-04-30", Note: "Partial progress; one more month",
	})
	if err != nil {
		t.Fatalf("extend: %v", err)
	}

	var endDate, storedOriginal, status string
	if err := env.db.QueryRow(ctx,
		`SELECT end_date::text, original_end_date::text, status FROM hrm_pips WHERE id = $1`, p.ID,
	).Scan(&endDate, &storedOriginal, &status); err != nil {
		t.Fatalf("read plan: %v", err)
	}
	if endDate != "2030-04-30" {
		t.Errorf("end_date = %s, want 2030-04-30", endDate)
	}
	if storedOriginal != originalEnd.Format("2006-01-02") {
		t.Errorf("original_end_date moved to %s — extensions must stay legible after the fact", storedOriginal)
	}
	if status != "extended" {
		t.Errorf("status = %s, want extended", status)
	}

	var entryType, note, prev, next string
	if err := env.db.QueryRow(ctx,
		`SELECT entry_type, note, previous_end_date::text, new_end_date::text
		   FROM hrm_pip_checkins WHERE pip_id = $1 AND entry_type = 'extension'`, p.ID,
	).Scan(&entryType, &note, &prev, &next); err != nil {
		t.Fatalf("the extension check-in did not land in the same transaction: %v", err)
	}
	if note == "" {
		t.Error("the extension reason was not persisted")
	}
	if prev != originalEnd.Format("2006-01-02") || next != "2030-04-30" {
		t.Errorf("the extension entry recorded %s → %s, want %s → 2030-04-30",
			prev, next, originalEnd.Format("2006-01-02"))
	}
	if !out.WasExtended {
		t.Error("WasExtended should be derived true")
	}

	// A rejected extension writes nothing at all.
	if _, err := env.hrmPipSvc.Extend(ctx, fx.orgID, p.ID, pipAdmin(fx.ownerID), hrmpip.ExtendRequest{
		NewEndDate: "2030-04-01", Note: "backwards",
	}); !errors.Is(err, hrmpip.ErrExtensionBackwards) {
		t.Fatalf("expected ErrExtensionBackwards, got %v", err)
	}
	var checkins int
	if err := env.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrm_pip_checkins WHERE pip_id = $1`, p.ID).Scan(&checkins); err != nil {
		t.Fatalf("count check-ins: %v", err)
	}
	if checkins != 1 {
		t.Errorf("a rejected extension wrote history: %d check-ins", checkins)
	}
}

// ============================================================
// One open plan per employee, under real concurrency
// ============================================================

// TestIntegration_PIP_OneOpenPlanUnderConcurrency shows the service
// pre-check is the friendly path and uq_hrm_pip_employee_open is the actual
// guarantee. Concurrent callers all read "no open plan" before any of them
// writes, so only the partial index can decide.
func TestIntegration_PIP_OneOpenPlanUnderConcurrency(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedPipFixture(t, env)

	const attempts = 6
	var wg sync.WaitGroup
	errs := make([]error, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = env.hrmPipSvc.Create(ctx, fx.orgID, fx.ownerID, pipAdmin(fx.ownerID), hrmpip.CreateRequest{
				EmployeeID: fx.empID, Title: "Racing plan", Concerns: "C",
				SuccessCriteria: "S", StartDate: "2030-01-01", EndDate: "2030-03-31",
			})
		}(i)
	}
	wg.Wait()

	successes := 0
	for _, err := range errs {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Errorf("expected exactly one of %d concurrent creates to succeed, got %d", attempts, successes)
	}

	var n int
	if err := env.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrm_pips WHERE employee_id = $1
		  AND status IN ('draft','active','extended')`, fx.empID).Scan(&n); err != nil {
		t.Fatalf("count open plans: %v", err)
	}
	if n != 1 {
		t.Errorf("expected exactly 1 open plan, got %d — the partial unique index did not hold", n)
	}

	// A closed plan frees the employee for a new one, which is what makes the
	// index PARTIAL rather than absolute.
	var openID string
	if err := env.db.QueryRow(ctx,
		`SELECT id FROM hrm_pips WHERE employee_id = $1 AND status IN ('draft','active','extended')`,
		fx.empID).Scan(&openID); err != nil {
		t.Fatalf("locate the open plan: %v", err)
	}
	if _, err := env.hrmPipSvc.Cancel(ctx, fx.orgID, openID, pipAdmin(fx.ownerID)); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if _, err := env.hrmPipSvc.Create(ctx, fx.orgID, fx.ownerID, pipAdmin(fx.ownerID), hrmpip.CreateRequest{
		EmployeeID: fx.empID, Title: "Fresh start", Concerns: "C", SuccessCriteria: "S",
		StartDate: "2030-04-01", EndDate: "2030-06-30",
	}); err != nil {
		t.Errorf("a cancelled plan should free the employee for a new one: %v", err)
	}
}

// ============================================================
// Scope tiers and tenancy
// ============================================================

func TestIntegration_PIP_ScopeTiers(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)

	managerEmp := seedEmployee(t, env, orgID, statusID, ownerID, ownerID, "Manager", nil)
	reportEmp := seedEmployee(t, env, orgID, statusID, ownerID, "", "Report", &managerEmp)
	strangerEmp := seedEmployee(t, env, orgID, statusID, ownerID, "", "Stranger", nil)

	for _, emp := range []string{managerEmp, reportEmp, strangerEmp} {
		if _, err := env.hrmPipSvc.Create(ctx, orgID, ownerID, pipAdmin(ownerID), hrmpip.CreateRequest{
			EmployeeID: emp, Title: "Plan", Concerns: "C", SuccessCriteria: "S",
			StartDate: "2030-01-01", EndDate: "2030-03-31",
		}); err != nil {
			t.Fatalf("create plan for %s: %v", emp, err)
		}
	}

	cases := []struct {
		tier authz.Scope
		want int
	}{
		{authz.ScopeOwn, 1},
		{authz.ScopeTeam, 2},
		{authz.ScopeAll, 3},
	}
	for _, tc := range cases {
		caller := hrmpip.Caller{UserID: ownerID, Tier: tc.tier}
		res, err := env.hrmPipSvc.List(ctx, orgID, caller, hrmpip.ListFilter{})
		if err != nil {
			t.Fatalf("list at tier %v: %v", tc.tier, err)
		}
		if res.Total != tc.want {
			t.Errorf("tier %v: expected %d plans, got %d", tc.tier, tc.want, res.Total)
		}
	}

	// Fetch-by-id narrows the same way — a PIP is the document that precedes
	// a dismissal, and a peer reading one is the failure mode.
	strangers, err := env.hrmPipSvc.List(ctx, orgID, pipAdmin(ownerID), hrmpip.ListFilter{EmployeeID: strangerEmp})
	if err != nil || len(strangers.PIPs) != 1 {
		t.Fatalf("locate the stranger's plan: %v (%d found)", err, len(strangers.PIPs))
	}
	own := hrmpip.Caller{UserID: ownerID, Tier: authz.ScopeOwn}
	if _, err := env.hrmPipSvc.Get(ctx, orgID, strangers.PIPs[0].ID, own); !errors.Is(err, hrmpip.ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied reading a stranger's plan at view_own, got %v", err)
	}
}

func TestIntegration_PIP_TenantIsolation(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fxA := seedPipFixture(t, env)
	fxB := seedPipFixture(t, env)
	p := seedActivePip(t, env, fxA)

	if _, err := env.hrmPipSvc.Get(ctx, fxB.orgID, p.ID, pipAdmin(fxB.ownerID)); !errors.Is(err, hrmpip.ErrNotFound) {
		t.Errorf("org B reached org A's plan: %v", err)
	}
}
