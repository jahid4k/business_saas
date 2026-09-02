// backend/internal/tests/integration/performance_goals_test.go
// Phase 5A Goals/OKR against real Postgres — the properties a stub repository
// cannot demonstrate: that the weight guard actually serializes concurrent
// writers, that the ON DELETE rules behave as designed, that the recursive
// scope CTE resolves a real reporting tree, and that a CHECK constraint the
// migration deliberately omits stays omitted.
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/auth"
	"github.com/mridha/businesssaas/internal/authz"
	hrmperformance "github.com/mridha/businesssaas/internal/hrm/performance"
)

func perfDec(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic("bad decimal literal in test: " + s)
	}
	return d
}

// perfAdmin is a caller holding manage at the widest tier.
func perfAdmin(userID string) hrmperformance.Caller {
	return hrmperformance.Caller{UserID: userID, Tier: authz.ScopeAll, CanManage: true}
}

// seedGoalCycle creates an ACTIVE cycle with the given weight target.
func seedGoalCycle(t *testing.T, env *testEnv, orgID, ownerID, target string) *hrmperformance.GoalCycle {
	t.Helper()
	ctx := context.Background()
	wt := perfDec(target)
	c, err := env.hrmPerformanceSvc.CreateCycle(ctx, orgID, ownerID, hrmperformance.CreateCycleRequest{
		Name: "Cycle " + uniqueSlug("c"), PeriodStart: "2030-01-01", PeriodEnd: "2030-03-31", WeightTarget: &wt,
	})
	if err != nil {
		t.Fatalf("create cycle: %v", err)
	}
	if _, err := env.hrmPerformanceSvc.ActivateCycle(ctx, orgID, c.ID); err != nil {
		t.Fatalf("activate cycle: %v", err)
	}
	return c
}

// ============================================================
// The weight guard under real concurrency
// ============================================================

// TestIntegration_Goals_WeightGuard_ConcurrentCreates is the headline test.
//
// The service-level pre-check cannot prevent this on its own: every goroutine
// reads the same total before any of them writes. Only the employee-row lock
// inside CreateGoalGuarded serializes them. Locking sibling GOALS would not
// work either — each competing INSERT adds a row that was in nobody's locked
// set. Without this test the guard is decoration.
func TestIntegration_Goals_WeightGuard_ConcurrentCreates(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)

	empID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Racer", nil)
	cycle := seedGoalCycle(t, env, orgID, ownerID, "100")

	// Pre-allocate 60, leaving room for exactly one more weight-30 goal.
	w60 := perfDec("60")
	if _, err := env.hrmPerformanceSvc.CreateGoal(ctx, orgID, perfAdmin(ownerID), hrmperformance.CreateGoalRequest{
		CycleID: cycle.ID, EmployeeID: empID, Title: "Existing", Weight: &w60,
	}); err != nil {
		t.Fatalf("seed existing goal: %v", err)
	}

	const attempts = 8
	var wg sync.WaitGroup
	errs := make([]error, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			w30 := perfDec("30")
			_, errs[i] = env.hrmPerformanceSvc.CreateGoal(ctx, orgID, perfAdmin(ownerID), hrmperformance.CreateGoalRequest{
				CycleID: cycle.ID, EmployeeID: empID, Title: "Racer", Weight: &w30,
			})
		}(i)
	}
	wg.Wait()

	successes := 0
	for _, err := range errs {
		if err == nil {
			successes++
		} else if !errors.Is(err, hrmperformance.ErrWeightExceedsCycleTarget) {
			t.Errorf("expected either success or ErrWeightExceedsCycleTarget, got %v", err)
		}
	}
	if successes != 1 {
		t.Errorf("expected exactly one of %d concurrent creates to succeed, got %d", attempts, successes)
	}

	var total decimal.Decimal
	if err := env.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(weight), 0) FROM hrm_goals WHERE employee_id = $1 AND cycle_id = $2 AND weight IS NOT NULL`,
		empID, cycle.ID,
	).Scan(&total); err != nil {
		t.Fatalf("sum weights: %v", err)
	}
	if total.GreaterThan(perfDec("100")) {
		t.Errorf("weight total escaped the cycle target under concurrency: %s — the employee-row lock did not serialize", total)
	}
}

// ============================================================
// ON DELETE behaviours
// ============================================================

// TestIntegration_Goals_ParentDeletePreservesChildAndCheckins pins the
// ON DELETE SET NULL decision against the real FK. If someone "corrects" the
// schema to CASCADE to match the build plan's wording, this fails loudly
// instead of silently destroying aligned goals in production.
func TestIntegration_Goals_ParentDeletePreservesChildAndCheckins(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	empID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Owner", nil)
	cycle := seedGoalCycle(t, env, orgID, ownerID, "100")

	parent, err := env.hrmPerformanceSvc.CreateGoal(ctx, orgID, perfAdmin(ownerID), hrmperformance.CreateGoalRequest{
		CycleID: cycle.ID, EmployeeID: empID, Title: "Company objective",
	})
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}
	child, err := env.hrmPerformanceSvc.CreateGoal(ctx, orgID, perfAdmin(ownerID), hrmperformance.CreateGoalRequest{
		CycleID: cycle.ID, EmployeeID: empID, Title: "Aligned goal", ParentGoalID: &parent.ID,
	})
	if err != nil {
		t.Fatalf("create child: %v", err)
	}
	for _, v := range []string{"10", "20"} {
		if _, err := env.hrmPerformanceSvc.CreateCheckin(ctx, orgID, child.ID, perfAdmin(ownerID), hrmperformance.CreateCheckinRequest{
			CurrentValue: perfDec(v),
		}); err != nil {
			t.Fatalf("checkin %s: %v", v, err)
		}
	}

	// Delete the parent directly, bypassing the service's history guard —
	// this tests the FK itself, not the service rule layered above it.
	if _, err := env.db.Exec(ctx, `DELETE FROM hrm_goals WHERE id = $1`, parent.ID); err != nil {
		t.Fatalf("SCHEMA: deleting a parent goal must not error, got: %v", err)
	}

	var parentRef *string
	if err := env.db.QueryRow(ctx, `SELECT parent_goal_id FROM hrm_goals WHERE id = $1`, child.ID).Scan(&parentRef); err != nil {
		t.Fatalf("SCHEMA: the child goal must survive its parent's deletion, got: %v", err)
	}
	if parentRef != nil {
		t.Errorf("expected parent_goal_id nulled by ON DELETE SET NULL, got %v", *parentRef)
	}

	var checkins int
	if err := env.db.QueryRow(ctx, `SELECT COUNT(*) FROM hrm_goal_checkins WHERE goal_id = $1`, child.ID).Scan(&checkins); err != nil {
		t.Fatalf("count checkins: %v", err)
	}
	if checkins != 2 {
		t.Errorf("expected both check-ins to survive the parent's deletion, got %d", checkins)
	}
}

// TestIntegration_Goals_CycleDeleteRestricted proves nobody can route around
// the goal-level history guard by deleting the cycle instead.
func TestIntegration_Goals_CycleDeleteRestricted(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	empID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Owner", nil)
	cycle := seedGoalCycle(t, env, orgID, ownerID, "100")

	if _, err := env.hrmPerformanceSvc.CreateGoal(ctx, orgID, perfAdmin(ownerID), hrmperformance.CreateGoalRequest{
		CycleID: cycle.ID, EmployeeID: empID, Title: "Attached",
	}); err != nil {
		t.Fatalf("create goal: %v", err)
	}

	if _, err := env.db.Exec(ctx, `DELETE FROM hrm_goal_cycles WHERE id = $1`, cycle.ID); err == nil {
		t.Error("expected deleting a cycle with attached goals to be RESTRICTed")
	}
}

// TestIntegration_Goals_UserDeleteDoesNotBreakLockedCycle makes the migration
// 00076 CHECK-versus-SET-NULL trap executable. Postgres re-evaluates CHECK
// constraints on UPDATE, and ON DELETE SET NULL is an UPDATE — so adding
// `CHECK (status <> 'locked' OR locked_by IS NOT NULL)` would make deleting a
// user fail 23514 for any org with a locked cycle. This test fails first if
// anyone adds it.
func TestIntegration_Goals_UserDeleteDoesNotBreakLockedCycle(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	empID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Owner", nil)
	cycle := seedGoalCycle(t, env, orgID, ownerID, "100")

	w100 := perfDec("100")
	if _, err := env.hrmPerformanceSvc.CreateGoal(ctx, orgID, perfAdmin(ownerID), hrmperformance.CreateGoalRequest{
		CycleID: cycle.ID, EmployeeID: empID, Title: "Full weight", Weight: &w100,
	}); err != nil {
		t.Fatalf("create goal: %v", err)
	}

	// The locker is a throwaway user who has created nothing else. The org
	// owner cannot be used: hrm_employees.created_by references users(id)
	// with no ON DELETE clause, so deleting them fails on an unrelated FK and
	// would mask what this test is actually asserting.
	locker, err := env.authSvc.Signup(ctx, auth.SignupRequest{
		Email: uniqueEmail("cycle-locker"), Password: "LockerPass123!",
	})
	if err != nil {
		t.Fatalf("signup locker: %v", err)
	}
	if _, err := env.hrmPerformanceSvc.LockCycle(ctx, orgID, cycle.ID, locker.ID); err != nil {
		t.Fatalf("lock cycle: %v", err)
	}

	// Deleting the locking user must succeed and degrade locked_by to NULL,
	// leaving the lock itself intact.
	if _, err := env.db.Exec(ctx, `DELETE FROM users WHERE id = $1`, locker.ID); err != nil {
		t.Fatalf("SCHEMA: deleting a user who locked a cycle must not error, got: %v", err)
	}

	var status string
	var lockedBy *string
	if err := env.db.QueryRow(ctx, `SELECT status, locked_by FROM hrm_goal_cycles WHERE id = $1`, cycle.ID).Scan(&status, &lockedBy); err != nil {
		t.Fatalf("read cycle: %v", err)
	}
	if status != string(hrmperformance.CycleStatusLocked) {
		t.Errorf("expected the cycle to remain locked, got %q", status)
	}
	if lockedBy != nil {
		t.Errorf("expected locked_by nulled by ON DELETE SET NULL, got %v", *lockedBy)
	}
}

// ============================================================
// Scope tiers over a real reporting tree
// ============================================================

// TestIntegration_Goals_ScopeTiers exercises scope.Predicate's recursive CTE,
// which only runs against real Postgres.
func TestIntegration_Goals_ScopeTiers(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)

	// managerUser is both a platform user and an employee, so ScopeOwn and
	// ScopeTeam have something to resolve against.
	managerUserID := ownerID
	managerEmpID := seedEmployee(t, env, orgID, statusID, ownerID, managerUserID, "Manager", nil)
	reportEmpID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Report", &managerEmpID)
	strangerEmpID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Stranger", nil)

	cycle := seedGoalCycle(t, env, orgID, ownerID, "100")
	for _, e := range []string{managerEmpID, reportEmpID, strangerEmpID} {
		if _, err := env.hrmPerformanceSvc.CreateGoal(ctx, orgID, perfAdmin(ownerID), hrmperformance.CreateGoalRequest{
			CycleID: cycle.ID, EmployeeID: e, Title: "Goal",
		}); err != nil {
			t.Fatalf("create goal for %s: %v", e, err)
		}
	}

	cases := []struct {
		tier authz.Scope
		want int
	}{
		{authz.ScopeOwn, 1},  // just the manager's own
		{authz.ScopeTeam, 2}, // own + direct report (ScopeTeam is inclusive of own)
		{authz.ScopeAll, 3},  // everyone
	}
	for _, tc := range cases {
		res, err := env.hrmPerformanceSvc.ListGoals(ctx, orgID, hrmperformance.GoalListFilter{
			CycleID: cycle.ID, Scope: tc.tier, CallerUserID: managerUserID,
		})
		if err != nil {
			t.Fatalf("list at tier %v: %v", tc.tier, err)
		}
		if res.Total != tc.want {
			t.Errorf("tier %v: expected %d goals, got %d", tc.tier, tc.want, res.Total)
		}
	}
}

// ============================================================
// Check-in atomicity + alignment cycle guard
// ============================================================

// TestIntegration_Goals_CheckinAdvancesGoalAndAppendsHistoryAtomically proves
// the two writes land together, and that progress_percent is derived from the
// post-advance value inside the same transaction.
func TestIntegration_Goals_CheckinAdvancesGoalAndAppendsHistory(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	empID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Owner", nil)
	cycle := seedGoalCycle(t, env, orgID, ownerID, "100")

	g, err := env.hrmPerformanceSvc.CreateGoal(ctx, orgID, perfAdmin(ownerID), hrmperformance.CreateGoalRequest{
		CycleID: cycle.ID, EmployeeID: empID, Title: "Tracked",
	})
	if err != nil {
		t.Fatalf("create goal: %v", err)
	}

	res, err := env.hrmPerformanceSvc.CreateCheckin(ctx, orgID, g.ID, perfAdmin(ownerID), hrmperformance.CreateCheckinRequest{
		CurrentValue: perfDec("40"),
	})
	if err != nil {
		t.Fatalf("checkin: %v", err)
	}
	if !res.Checkin.PreviousValue.Equal(decimal.Zero) {
		t.Errorf("expected previous_value 0, got %s", res.Checkin.PreviousValue)
	}
	if !res.Checkin.ProgressPercent.Equal(perfDec("40")) {
		t.Errorf("expected progress derived from the advanced value (40), got %s", res.Checkin.ProgressPercent)
	}

	var current decimal.Decimal
	if err := env.db.QueryRow(ctx, `SELECT current_value FROM hrm_goals WHERE id = $1`, g.ID).Scan(&current); err != nil {
		t.Fatalf("read goal: %v", err)
	}
	if !current.Equal(perfDec("40")) {
		t.Errorf("expected the goal advanced to 40 in the same transaction, got %s", current)
	}
}

// TestIntegration_Goals_AlignmentCycleGuardTerminates hand-inserts a cyclic
// A→B→A pair directly via SQL, bypassing the service, then asks the guard
// about it. Without the `<> ALL(path)` guard and depth cap copied from
// scope/predicate.go, this query would not terminate.
func TestIntegration_Goals_AlignmentCycleGuardTerminates(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	empID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Owner", nil)
	cycle := seedGoalCycle(t, env, orgID, ownerID, "100")

	a, err := env.hrmPerformanceSvc.CreateGoal(ctx, orgID, perfAdmin(ownerID), hrmperformance.CreateGoalRequest{
		CycleID: cycle.ID, EmployeeID: empID, Title: "A",
	})
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	b, err := env.hrmPerformanceSvc.CreateGoal(ctx, orgID, perfAdmin(ownerID), hrmperformance.CreateGoalRequest{
		CycleID: cycle.ID, EmployeeID: empID, Title: "B", ParentGoalID: &a.ID,
	})
	if err != nil {
		t.Fatalf("create B: %v", err)
	}

	// Force the corrupt state the service would never create: A→B while B→A.
	if _, err := env.db.Exec(ctx, `UPDATE hrm_goals SET parent_goal_id = $1 WHERE id = $2`, b.ID, a.ID); err != nil {
		t.Fatalf("seed cyclic pair: %v", err)
	}

	// The guard must return rather than spin. Attempting any re-alignment on
	// the corrupt pair exercises the recursive walk.
	_, err = env.hrmPerformanceSvc.UpdateGoal(ctx, orgID, b.ID, perfAdmin(ownerID), hrmperformance.UpdateGoalRequest{
		ParentGoalID: &a.ID,
	})
	if !errors.Is(err, hrmperformance.ErrGoalAlignmentCycle) {
		t.Fatalf("expected ErrGoalAlignmentCycle against an already-cyclic pair, got %v", err)
	}
}

// ============================================================
// Tenant isolation + weight audit
// ============================================================

func TestIntegration_Goals_TenantIsolation(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgAID, statusAID, ownerAID := seedScopeTestOrg(t, env)
	orgBID, _, _ := seedScopeTestOrg(t, env)

	empID := seedEmployee(t, env, orgAID, statusAID, ownerAID, "", "OrgA", nil)
	cycle := seedGoalCycle(t, env, orgAID, ownerAID, "100")
	g, err := env.hrmPerformanceSvc.CreateGoal(ctx, orgAID, perfAdmin(ownerAID), hrmperformance.CreateGoalRequest{
		CycleID: cycle.ID, EmployeeID: empID, Title: "Org A goal",
	})
	if err != nil {
		t.Fatalf("create goal: %v", err)
	}

	// Even at the widest tier, org B must not reach org A's goal.
	if _, err := env.hrmPerformanceSvc.GetGoal(ctx, orgBID, g.ID, perfAdmin(ownerAID)); !errors.Is(err, hrmperformance.ErrGoalNotFound) {
		t.Errorf("SECURITY: org B must not read org A's goal, got %v", err)
	}
	res, err := env.hrmPerformanceSvc.ListGoals(ctx, orgBID, hrmperformance.GoalListFilter{
		Scope: authz.ScopeAll, CallerUserID: ownerAID,
	})
	if err != nil {
		t.Fatalf("list in org B: %v", err)
	}
	if res.Total != 0 {
		t.Errorf("SECURITY: expected org B to see no goals, got %d", res.Total)
	}
}

// TestIntegration_Goals_WeightAuditAcrossEmployees exercises the real
// SUM/GROUP BY that both the audit endpoint and the lock gate depend on.
func TestIntegration_Goals_WeightAuditAcrossEmployees(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	cycle := seedGoalCycle(t, env, orgID, ownerID, "100")

	complete := seedEmployee(t, env, orgID, statusID, ownerID, "", "Complete", nil)
	short := seedEmployee(t, env, orgID, statusID, ownerID, "", "Short", nil)

	w100, w80 := perfDec("100"), perfDec("80")
	if _, err := env.hrmPerformanceSvc.CreateGoal(ctx, orgID, perfAdmin(ownerID), hrmperformance.CreateGoalRequest{
		CycleID: cycle.ID, EmployeeID: complete, Title: "Full", Weight: &w100,
	}); err != nil {
		t.Fatalf("create complete goal: %v", err)
	}
	if _, err := env.hrmPerformanceSvc.CreateGoal(ctx, orgID, perfAdmin(ownerID), hrmperformance.CreateGoalRequest{
		CycleID: cycle.ID, EmployeeID: short, Title: "Partial", Weight: &w80,
	}); err != nil {
		t.Fatalf("create short goal: %v", err)
	}

	audit, err := env.hrmPerformanceSvc.GetCycleWeightAudit(ctx, orgID, cycle.ID)
	if err != nil {
		t.Fatalf("weight audit: %v", err)
	}
	if len(audit.Employees) != 2 {
		t.Fatalf("expected 2 employees with weighted goals, got %d", len(audit.Employees))
	}
	if len(audit.Incomplete) != 1 || audit.Incomplete[0].EmployeeID != short {
		t.Fatalf("expected only the 80-weight employee to be flagged incomplete, got %+v", audit.Incomplete)
	}

	// And the lock gate must agree with the audit it shares a query with.
	if _, err := env.hrmPerformanceSvc.LockCycle(ctx, orgID, cycle.ID, ownerID); !errors.Is(err, hrmperformance.ErrCycleWeightsIncomplete) {
		t.Errorf("expected the lock gate to reject while an employee is short, got %v", err)
	}
}
