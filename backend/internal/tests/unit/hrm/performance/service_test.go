// backend/internal/tests/unit/hrm/performance/service_test.go
// Service-layer rules for Phase 5A Goals/OKR, against a hand-written stub
// repository. Anything that depends on a real transaction — above all the
// weight guard's employee-row lock — is deliberately NOT tested here and is
// proved in internal/tests/integration/performance_goals_test.go instead: a
// stub cannot demonstrate that concurrent writers serialize.
package performance_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/performance"
)

// reflectFieldNames lists a struct's exported field names, so a test can
// assert a type's SHAPE rather than only its current values.
func reflectFieldNames(v any) []string {
	t := reflect.TypeOf(v)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	names := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		names = append(names, t.Field(i).Name)
	}
	return names
}

// ── Stub repository ──────────────────────────────────────────────────────────

type stubRepo struct {
	seq int

	cycles   map[string]*performance.GoalCycle
	goals    map[string]*performance.Goal
	checkins map[string][]*performance.GoalCheckin

	// employeeUsers maps platform user_id → hrm_employees.id.
	employeeUsers map[string]string
	// employees is the set of valid employee ids in the org.
	employees map[string]bool
	// cyclicPairs[goalID+":"+parentID] forces WouldCreateAlignmentCycle true,
	// so the guard can be exercised without modelling a real tree walk.
	cyclicPairs map[string]bool

	// Phase 5B state — see appraisals_stub_test.go for the methods.
	scales           map[string]*performance.RatingScale
	levels           map[string]*performance.RatingLevel
	appraisalCycles  map[string]*performance.AppraisalCycle
	appraisals       map[string]*performance.Appraisal
	phaseHistory     map[string][]*performance.PhaseHistory
	employeeSubjects map[string]*performance.EmployeeSubject
}

func newStubRepo() *stubRepo {
	return &stubRepo{
		cycles:        map[string]*performance.GoalCycle{},
		goals:         map[string]*performance.Goal{},
		checkins:      map[string][]*performance.GoalCheckin{},
		employeeUsers: map[string]string{},
		employees:     map[string]bool{},
		cyclicPairs:   map[string]bool{},

		scales:           map[string]*performance.RatingScale{},
		levels:           map[string]*performance.RatingLevel{},
		appraisalCycles:  map[string]*performance.AppraisalCycle{},
		appraisals:       map[string]*performance.Appraisal{},
		phaseHistory:     map[string][]*performance.PhaseHistory{},
		employeeSubjects: map[string]*performance.EmployeeSubject{},
	}
}

func (r *stubRepo) nextID(prefix string) string {
	r.seq++
	return fmt.Sprintf("%s_%d", prefix, r.seq)
}

func matchRef(id, publicID, ref string) bool { return id == ref || publicID == ref }

// ── Cycles ───────────────────────────────────────────────────────────────────

func (r *stubRepo) FindCycles(_ context.Context, orgID string, _ performance.CycleListFilter) ([]*performance.GoalCycle, error) {
	out := make([]*performance.GoalCycle, 0)
	for _, c := range r.cycles {
		if c.OrgID == orgID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (r *stubRepo) CountCycles(ctx context.Context, orgID string, f performance.CycleListFilter) (int, error) {
	out, _ := r.FindCycles(ctx, orgID, f)
	return len(out), nil
}

func (r *stubRepo) FindCycleByRef(_ context.Context, orgID, ref string) (*performance.GoalCycle, error) {
	for _, c := range r.cycles {
		if c.OrgID == orgID && matchRef(c.ID, c.PublicID, ref) {
			return c, nil
		}
	}
	return nil, nil
}

func (r *stubRepo) CreateCycle(_ context.Context, c *performance.GoalCycle) error {
	c.ID = r.nextID("gcyc")
	c.PublicID = "pub_" + c.ID
	c.Status = performance.CycleStatusDraft
	c.CreatedAt, c.UpdatedAt = time.Now(), time.Now()
	r.cycles[c.ID] = c
	return nil
}

func (r *stubRepo) UpdateCycle(_ context.Context, c *performance.GoalCycle) error {
	if _, ok := r.cycles[c.ID]; !ok {
		return performance.ErrCycleNotFound
	}
	r.cycles[c.ID] = c
	return nil
}

func (r *stubRepo) SetCycleStatus(_ context.Context, _, id string, status performance.CycleStatus, actorID *string) error {
	c, ok := r.cycles[id]
	if !ok {
		return performance.ErrCycleNotFound
	}
	c.Status = status
	if status == performance.CycleStatusLocked {
		now := time.Now()
		c.LockedAt, c.LockedBy = &now, actorID
	}
	return nil
}

func (r *stubRepo) CycleNameExists(_ context.Context, orgID, name, excludeID string) (bool, error) {
	for _, c := range r.cycles {
		if c.OrgID == orgID && c.Name == name && c.ID != excludeID {
			return true, nil
		}
	}
	return false, nil
}

func (r *stubRepo) FindCycleWeightTotals(_ context.Context, orgID, cycleID string) ([]*performance.EmployeeWeightTotal, error) {
	totals := map[string]*performance.EmployeeWeightTotal{}
	for _, g := range r.goals {
		if g.OrgID != orgID || g.CycleID != cycleID || g.Weight == nil || g.Status == performance.GoalStatusCancelled {
			continue
		}
		t, ok := totals[g.EmployeeID]
		if !ok {
			t = &performance.EmployeeWeightTotal{EmployeeID: g.EmployeeID, EmployeeName: g.EmployeeID, TotalWeight: decimal.Zero}
			totals[g.EmployeeID] = t
		}
		t.TotalWeight = t.TotalWeight.Add(*g.Weight)
		t.GoalCount++
	}
	out := make([]*performance.EmployeeWeightTotal, 0, len(totals))
	for _, t := range totals {
		out = append(out, t)
	}
	return out, nil
}

// ── Goals ────────────────────────────────────────────────────────────────────

func (r *stubRepo) FindGoals(_ context.Context, orgID string, filter performance.GoalListFilter) ([]*performance.Goal, error) {
	out := make([]*performance.Goal, 0)
	for _, g := range r.goals {
		if g.OrgID != orgID {
			continue
		}
		if filter.CycleID != "" && g.CycleID != filter.CycleID {
			continue
		}
		if filter.EmployeeID != "" && g.EmployeeID != filter.EmployeeID {
			continue
		}
		out = append(out, g)
	}
	return out, nil
}

func (r *stubRepo) CountGoals(ctx context.Context, orgID string, f performance.GoalListFilter) (int, error) {
	out, _ := r.FindGoals(ctx, orgID, f)
	return len(out), nil
}

func (r *stubRepo) FindGoalByRef(_ context.Context, orgID, ref string) (*performance.Goal, error) {
	for _, g := range r.goals {
		if g.OrgID == orgID && matchRef(g.ID, g.PublicID, ref) {
			return g, nil
		}
	}
	return nil, nil
}

func (r *stubRepo) FindChildGoals(_ context.Context, orgID, parentGoalID string) ([]*performance.Goal, error) {
	out := make([]*performance.Goal, 0)
	for _, g := range r.goals {
		if g.OrgID == orgID && g.ParentGoalID != nil && *g.ParentGoalID == parentGoalID {
			out = append(out, g)
		}
	}
	return out, nil
}

func (r *stubRepo) FindGoalRef(_ context.Context, orgID, goalID string) (*performance.GoalRef, error) {
	g, ok := r.goals[goalID]
	if !ok || g.OrgID != orgID {
		return nil, nil
	}
	return &performance.GoalRef{PublicID: g.PublicID, Title: g.Title, GoalLevel: g.GoalLevel}, nil
}

func (r *stubRepo) SumGoalWeights(_ context.Context, employeeID, cycleID, excludeGoalID string) (decimal.Decimal, error) {
	total := decimal.Zero
	for _, g := range r.goals {
		if g.EmployeeID == employeeID && g.CycleID == cycleID &&
			g.Weight != nil && g.Status != performance.GoalStatusCancelled && g.ID != excludeGoalID {
			total = total.Add(*g.Weight)
		}
	}
	return total, nil
}

// CreateGoalGuarded mirrors the real repository's check so the service's
// contract is exercised. The REAL guard is a row lock plus a re-read inside a
// transaction; that behaviour is only provable against Postgres.
func (r *stubRepo) CreateGoalGuarded(ctx context.Context, g *performance.Goal, weightTarget decimal.Decimal) error {
	if !r.employees[g.EmployeeID] {
		return performance.ErrEmployeeNotFound
	}
	existing, _ := r.SumGoalWeights(ctx, g.EmployeeID, g.CycleID, "")
	if g.Weight != nil && existing.Add(*g.Weight).GreaterThan(weightTarget) {
		return performance.ErrWeightExceedsCycleTarget
	}
	g.ID = r.nextID("goal")
	g.PublicID = "pub_" + g.ID
	g.Status = performance.GoalStatusDraft
	g.CreatedAt, g.UpdatedAt = time.Now(), time.Now()
	r.goals[g.ID] = g
	return nil
}

func (r *stubRepo) UpdateGoalGuarded(ctx context.Context, g *performance.Goal, weightTarget decimal.Decimal) error {
	if _, ok := r.goals[g.ID]; !ok {
		return performance.ErrGoalNotFound
	}
	existing, _ := r.SumGoalWeights(ctx, g.EmployeeID, g.CycleID, g.ID)
	if g.Weight != nil && existing.Add(*g.Weight).GreaterThan(weightTarget) {
		return performance.ErrWeightExceedsCycleTarget
	}
	g.UpdatedAt = time.Now()
	r.goals[g.ID] = g
	return nil
}

func (r *stubRepo) SetGoalStatus(_ context.Context, orgID, goalID string, status performance.GoalStatus, outcome *performance.GoalOutcome, reason *string) error {
	g, ok := r.goals[goalID]
	if !ok || g.OrgID != orgID {
		return performance.ErrGoalNotFound
	}
	g.Status = status
	if status == performance.GoalStatusCompleted {
		g.Outcome = outcome
	}
	if status == performance.GoalStatusCancelled {
		g.CancelReason = reason
	}
	return nil
}

func (r *stubRepo) DeleteGoal(_ context.Context, orgID, goalID string) error {
	g, ok := r.goals[goalID]
	if !ok || g.OrgID != orgID {
		return performance.ErrGoalNotFound
	}
	delete(r.goals, goalID)
	return nil
}

func (r *stubRepo) CountCheckinsForGoal(_ context.Context, goalID string) (int, error) {
	return len(r.checkins[goalID]), nil
}

func (r *stubRepo) CountChildGoals(ctx context.Context, orgID, goalID string) (int, error) {
	out, _ := r.FindChildGoals(ctx, orgID, goalID)
	return len(out), nil
}

func (r *stubRepo) WouldCreateAlignmentCycle(_ context.Context, _, goalID, newParentID string) (bool, error) {
	return r.cyclicPairs[goalID+":"+newParentID], nil
}

// ── Check-ins ────────────────────────────────────────────────────────────────

func (r *stubRepo) FindCheckins(_ context.Context, goalID string, _, _ int) ([]*performance.GoalCheckin, error) {
	return r.checkins[goalID], nil
}

func (r *stubRepo) CountCheckins(_ context.Context, goalID string) (int, error) {
	return len(r.checkins[goalID]), nil
}

func (r *stubRepo) CreateCheckin(_ context.Context, orgID string, ck *performance.GoalCheckin, newValue decimal.Decimal) (*performance.Goal, error) {
	g, ok := r.goals[ck.GoalID]
	if !ok || g.OrgID != orgID {
		return nil, performance.ErrGoalNotFound
	}
	if g.Status == performance.GoalStatusCompleted || g.Status == performance.GoalStatusCancelled {
		return nil, performance.ErrCheckinGoalNotOpen
	}
	ck.ID = r.nextID("gchk")
	ck.PublicID = "pub_" + ck.ID
	ck.PreviousValue = g.CurrentValue
	ck.CurrentValue = newValue
	ck.StatusSnapshot = string(g.Status)
	ck.CheckedInAt = time.Now()

	advanced := *g
	advanced.CurrentValue = newValue
	ck.ProgressPercent = advanced.RawProgressPercent()

	g.CurrentValue = newValue
	r.checkins[g.ID] = append(r.checkins[g.ID], ck)
	return g, nil
}

// ── Repository-level ─────────────────────────────────────────────────────────

func (r *stubRepo) FindEmployeeIDByUserID(_ context.Context, _, userID string) (string, error) {
	return r.employeeUsers[userID], nil
}

func (r *stubRepo) EmployeeExists(_ context.Context, _, employeeID string) (bool, error) {
	return r.employees[employeeID], nil
}

var _ performance.Repository = (*stubRepo)(nil)

// ── Stub RecordAuthorizer ────────────────────────────────────────────────────

type stubAuthorizer struct {
	// allow reports the answer for every record check. ScopeAll always
	// short-circuits to true regardless, mirroring the real resolver.
	allow bool
	calls int
}

func (a *stubAuthorizer) AuthorizeRecordAccess(_ context.Context, tier authz.Scope, _, _, _ string) (bool, error) {
	a.calls++
	if tier == authz.ScopeAll {
		return true, nil
	}
	return a.allow, nil
}

var _ performance.RecordAuthorizer = (*stubAuthorizer)(nil)

// ── Helpers ──────────────────────────────────────────────────────────────────

const (
	testOrg     = "org_1"
	ownerUserID = "user_owner"
	ownerEmpID  = "emp_owner"
	otherUserID = "user_other"
	otherEmpID  = "emp_other"
)

func newTestSvc(allow bool) (performance.Service, *stubRepo, *stubAuthorizer) {
	repo := newStubRepo()
	repo.employeeUsers[ownerUserID] = ownerEmpID
	repo.employeeUsers[otherUserID] = otherEmpID
	repo.employees[ownerEmpID] = true
	repo.employees[otherEmpID] = true
	auth := &stubAuthorizer{allow: allow}
	return performance.NewService(repo, auth, newStubFormEngine()), repo, auth
}

// adminCaller holds manage plus the widest tier.
func adminCaller() performance.Caller {
	return performance.Caller{UserID: ownerUserID, Tier: authz.ScopeAll, CanManage: true}
}

// memberCaller holds only set_own at the narrowest tier — the individual
// contributor setting their own goals.
func memberCaller() performance.Caller {
	return performance.Caller{UserID: ownerUserID, Tier: authz.ScopeOwn, CanManage: false}
}

func seedActiveCycle(t *testing.T, svc performance.Service, repo *stubRepo, target string) *performance.GoalCycle {
	t.Helper()
	ctx := context.Background()
	wt := dec(target)
	c, err := svc.CreateCycle(ctx, testOrg, ownerUserID, performance.CreateCycleRequest{
		Name: "Q1 " + repo.nextID("n"), PeriodStart: "2030-01-01", PeriodEnd: "2030-03-31", WeightTarget: &wt,
	})
	if err != nil {
		t.Fatalf("seed cycle: %v", err)
	}
	if _, err := svc.ActivateCycle(ctx, testOrg, c.ID); err != nil {
		t.Fatalf("activate cycle: %v", err)
	}
	return c
}

func seedGoal(t *testing.T, svc performance.Service, cycleID, employeeID string, weight *decimal.Decimal) *performance.Goal {
	t.Helper()
	g, err := svc.CreateGoal(context.Background(), testOrg, adminCaller(), performance.CreateGoalRequest{
		CycleID: cycleID, EmployeeID: employeeID, Title: "Goal", Weight: weight,
	})
	if err != nil {
		t.Fatalf("seed goal: %v", err)
	}
	return g
}

// ============================================================
// Weight rules
// ============================================================

func TestCreateGoal_RejectsWeightPushingTotalOverCycleTarget(t *testing.T) {
	svc, repo, _ := newTestSvc(true)
	cycle := seedActiveCycle(t, svc, repo, "100")

	w90 := dec("90")
	seedGoal(t, svc, cycle.ID, ownerEmpID, &w90)

	w20 := dec("20")
	_, err := svc.CreateGoal(context.Background(), testOrg, adminCaller(), performance.CreateGoalRequest{
		CycleID: cycle.ID, EmployeeID: ownerEmpID, Title: "Over", Weight: &w20,
	})
	if !errors.Is(err, performance.ErrWeightExceedsCycleTarget) {
		t.Fatalf("expected ErrWeightExceedsCycleTarget, got %v", err)
	}

	// Landing exactly on the target is legal — only exceeding it is not.
	w10 := dec("10")
	if _, err := svc.CreateGoal(context.Background(), testOrg, adminCaller(), performance.CreateGoalRequest{
		CycleID: cycle.ID, EmployeeID: ownerEmpID, Title: "Exact", Weight: &w10,
	}); err != nil {
		t.Fatalf("expected a goal landing exactly on the target to be accepted, got %v", err)
	}
}

// TestCreateGoal_AllowsPartialTotal pins the decision that write time enforces
// "must not exceed", never "must equal". Requiring equality would make
// creating the very first goal impossible.
func TestCreateGoal_AllowsPartialTotal(t *testing.T) {
	svc, repo, _ := newTestSvc(true)
	cycle := seedActiveCycle(t, svc, repo, "100")

	w30 := dec("30")
	if _, err := svc.CreateGoal(context.Background(), testOrg, adminCaller(), performance.CreateGoalRequest{
		CycleID: cycle.ID, EmployeeID: ownerEmpID, Title: "Partial", Weight: &w30,
	}); err != nil {
		t.Fatalf("expected a partial weight total to be accepted at write time, got %v", err)
	}
}

func TestCreateGoal_NilWeightNeverCountsTowardTotal(t *testing.T) {
	svc, repo, _ := newTestSvc(true)
	cycle := seedActiveCycle(t, svc, repo, "100")

	w100 := dec("100")
	seedGoal(t, svc, cycle.ID, ownerEmpID, &w100)

	// A tracking-only goal must still be creatable against a fully-allocated
	// employee — this is what lets an objective and its key results coexist.
	if _, err := svc.CreateGoal(context.Background(), testOrg, adminCaller(), performance.CreateGoalRequest{
		CycleID: cycle.ID, EmployeeID: ownerEmpID, Title: "Tracking only", Weight: nil,
	}); err != nil {
		t.Fatalf("expected a nil-weight goal to bypass the weight total, got %v", err)
	}
}

func TestLockCycle_RejectsWhenAnyEmployeeTotalBelowTarget(t *testing.T) {
	svc, repo, _ := newTestSvc(true)
	cycle := seedActiveCycle(t, svc, repo, "100")

	w60 := dec("60")
	seedGoal(t, svc, cycle.ID, ownerEmpID, &w60)

	_, err := svc.LockCycle(context.Background(), testOrg, cycle.ID, ownerUserID)
	if !errors.Is(err, performance.ErrCycleWeightsIncomplete) {
		t.Fatalf("expected ErrCycleWeightsIncomplete, got %v", err)
	}

	// The audit endpoint must name who is short, not merely report a boolean.
	audit, err := svc.GetCycleWeightAudit(context.Background(), testOrg, cycle.ID)
	if err != nil {
		t.Fatalf("weight audit: %v", err)
	}
	if len(audit.Incomplete) != 1 || audit.Incomplete[0].EmployeeID != ownerEmpID {
		t.Fatalf("expected the audit to name the short employee, got %+v", audit.Incomplete)
	}

	// Topping up to the target unblocks the lock.
	w40 := dec("40")
	seedGoal(t, svc, cycle.ID, ownerEmpID, &w40)
	if _, err := svc.LockCycle(context.Background(), testOrg, cycle.ID, ownerUserID); err != nil {
		t.Fatalf("expected lock to succeed once weights total the target, got %v", err)
	}
}

// ============================================================
// Cycle status gating
// ============================================================

func TestCreateGoal_RejectsWhenCycleNotActive(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		move func(svc performance.Service, cycleID string)
	}{
		{"draft", func(performance.Service, string) {}},
		{"locked", func(svc performance.Service, id string) {
			_, _ = svc.ActivateCycle(ctx, testOrg, id)
			_, _ = svc.LockCycle(ctx, testOrg, id, ownerUserID)
		}},
		{"closed", func(svc performance.Service, id string) {
			_, _ = svc.ActivateCycle(ctx, testOrg, id)
			_, _ = svc.CloseCycle(ctx, testOrg, id)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _ := newTestSvc(true)
			wt := dec("100")
			c, err := svc.CreateCycle(ctx, testOrg, ownerUserID, performance.CreateCycleRequest{
				Name: "C " + tc.name, PeriodStart: "2030-01-01", PeriodEnd: "2030-03-31", WeightTarget: &wt,
			})
			if err != nil {
				t.Fatalf("create cycle: %v", err)
			}
			tc.move(svc, c.ID)

			_, err = svc.CreateGoal(ctx, testOrg, adminCaller(), performance.CreateGoalRequest{
				CycleID: c.ID, EmployeeID: ownerEmpID, Title: "Nope",
			})
			if !errors.Is(err, performance.ErrCycleNotActive) {
				t.Fatalf("expected ErrCycleNotActive for a %s cycle, got %v", tc.name, err)
			}
		})
	}
}

// TestLockedCycle_BlocksDefinitionEditsButAllowsCheckins pins the two-axis
// meaning of 'locked': definitions freeze, progress keeps landing.
func TestLockedCycle_BlocksDefinitionEditsButAllowsCheckins(t *testing.T) {
	svc, repo, _ := newTestSvc(true)
	ctx := context.Background()
	cycle := seedActiveCycle(t, svc, repo, "100")

	w100 := dec("100")
	g := seedGoal(t, svc, cycle.ID, ownerEmpID, &w100)
	if _, err := svc.SubmitGoal(ctx, testOrg, g.ID, adminCaller()); err != nil {
		t.Fatalf("submit goal: %v", err)
	}
	if _, err := svc.LockCycle(ctx, testOrg, cycle.ID, ownerUserID); err != nil {
		t.Fatalf("lock cycle: %v", err)
	}

	newTitle := "Renamed"
	_, err := svc.UpdateGoal(ctx, testOrg, g.ID, adminCaller(), performance.UpdateGoalRequest{Title: &newTitle})
	if !errors.Is(err, performance.ErrCycleNotActive) {
		t.Fatalf("expected a locked cycle to block definition edits, got %v", err)
	}

	if _, err := svc.CreateCheckin(ctx, testOrg, g.ID, adminCaller(), performance.CreateCheckinRequest{
		CurrentValue: dec("40"),
	}); err != nil {
		t.Fatalf("expected check-ins to still land on a locked cycle, got %v", err)
	}
}

// ============================================================
// current_value is check-in-only
// ============================================================

// TestUpdateGoal_CannotMoveCurrentValue is the property that guarantees
// hrm_goal_checkins has no holes: if UpdateGoal could move progress, history
// would silently skip.
func TestUpdateGoal_CannotMoveCurrentValue(t *testing.T) {
	svc, repo, _ := newTestSvc(true)
	ctx := context.Background()
	cycle := seedActiveCycle(t, svc, repo, "100")
	g := seedGoal(t, svc, cycle.ID, ownerEmpID, nil)

	if _, err := svc.CreateCheckin(ctx, testOrg, g.ID, adminCaller(), performance.CreateCheckinRequest{
		CurrentValue: dec("42"),
	}); err != nil {
		t.Fatalf("checkin: %v", err)
	}

	newTitle := "Retitled"
	if _, err := svc.UpdateGoal(ctx, testOrg, g.ID, adminCaller(), performance.UpdateGoalRequest{Title: &newTitle}); err != nil {
		t.Fatalf("update: %v", err)
	}

	after, err := svc.GetGoal(ctx, testOrg, g.ID, adminCaller())
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !after.CurrentValue.Equal(dec("42")) {
		t.Errorf("expected current_value to survive an update untouched, got %s", after.CurrentValue)
	}
	// UpdateGoalRequest has no CurrentValue field at all, so the only way this
	// could regress is someone adding one.
	if n, _ := repo.CountCheckins(ctx, g.ID); n != 1 {
		t.Errorf("expected exactly one check-in to account for the value, got %d", n)
	}
}

// ============================================================
// Alignment
// ============================================================

func TestUpdateGoal_RejectsSelfAndDescendantParent(t *testing.T) {
	svc, repo, _ := newTestSvc(true)
	ctx := context.Background()
	cycle := seedActiveCycle(t, svc, repo, "100")
	parent := seedGoal(t, svc, cycle.ID, ownerEmpID, nil)
	child := seedGoal(t, svc, cycle.ID, ownerEmpID, nil)

	// Self-alignment.
	_, err := svc.UpdateGoal(ctx, testOrg, child.ID, adminCaller(), performance.UpdateGoalRequest{ParentGoalID: &child.ID})
	if !errors.Is(err, performance.ErrGoalAlignmentCycle) {
		t.Fatalf("expected ErrGoalAlignmentCycle for self-parent, got %v", err)
	}

	// Descendant alignment: the repository reports the edge would close a loop.
	repo.cyclicPairs[child.ID+":"+parent.ID] = true
	_, err = svc.UpdateGoal(ctx, testOrg, child.ID, adminCaller(), performance.UpdateGoalRequest{ParentGoalID: &parent.ID})
	if !errors.Is(err, performance.ErrGoalAlignmentCycle) {
		t.Fatalf("expected ErrGoalAlignmentCycle for a descendant parent, got %v", err)
	}
}

// ============================================================
// Scope + authorization
// ============================================================

func TestGetGoal_DeniesOutOfScopeRecord(t *testing.T) {
	svc, repo, _ := newTestSvc(false) // authorizer denies
	cycle := seedActiveCycle(t, svc, repo, "100")
	g := seedGoal(t, svc, cycle.ID, otherEmpID, nil)

	caller := performance.Caller{UserID: ownerUserID, Tier: authz.ScopeTeam, CanManage: false}
	_, err := svc.GetGoal(context.Background(), testOrg, g.ID, caller)
	if !errors.Is(err, performance.ErrGoalAccessDenied) {
		t.Fatalf("expected ErrGoalAccessDenied, got %v", err)
	}
}

// TestUpdateGoal_OthersGoalRequiresManageAndScope is the three-case test the
// middle of which is what people forget: holding hrm.goals.manage is NOT
// sufficient, because that permission is unscoped at the route. Only the
// record check stops a view_team manager editing outside their reporting line.
func TestUpdateGoal_OthersGoalRequiresManageAndScope(t *testing.T) {
	newTitle := "Edited"

	t.Run("no manage permission is denied", func(t *testing.T) {
		svc, repo, _ := newTestSvc(true)
		cycle := seedActiveCycle(t, svc, repo, "100")
		g := seedGoal(t, svc, cycle.ID, otherEmpID, nil)

		caller := performance.Caller{UserID: ownerUserID, Tier: authz.ScopeTeam, CanManage: false}
		_, err := svc.UpdateGoal(context.Background(), testOrg, g.ID, caller, performance.UpdateGoalRequest{Title: &newTitle})
		if !errors.Is(err, performance.ErrGoalAccessDenied) {
			t.Fatalf("expected denial without manage, got %v", err)
		}
	})

	t.Run("manage but out of scope is denied", func(t *testing.T) {
		svc, repo, _ := newTestSvc(false) // authorizer denies
		cycle := seedActiveCycle(t, svc, repo, "100")
		g := seedGoal(t, svc, cycle.ID, otherEmpID, nil)

		caller := performance.Caller{UserID: ownerUserID, Tier: authz.ScopeTeam, CanManage: true}
		_, err := svc.UpdateGoal(context.Background(), testOrg, g.ID, caller, performance.UpdateGoalRequest{Title: &newTitle})
		if !errors.Is(err, performance.ErrGoalAccessDenied) {
			t.Fatalf("expected manage alone to be insufficient without record scope, got %v", err)
		}
	})

	t.Run("manage and in scope succeeds", func(t *testing.T) {
		svc, repo, _ := newTestSvc(true)
		cycle := seedActiveCycle(t, svc, repo, "100")
		g := seedGoal(t, svc, cycle.ID, otherEmpID, nil)

		caller := performance.Caller{UserID: ownerUserID, Tier: authz.ScopeTeam, CanManage: true}
		if _, err := svc.UpdateGoal(context.Background(), testOrg, g.ID, caller, performance.UpdateGoalRequest{Title: &newTitle}); err != nil {
			t.Fatalf("expected manage + in-scope to succeed, got %v", err)
		}
	})
}

// TestCreateGoal_MemberSetsOwnGoalWithoutManage proves the self-service path:
// an omitted employee_id targets the caller, and set_own alone suffices.
func TestCreateGoal_MemberSetsOwnGoalWithoutManage(t *testing.T) {
	svc, repo, auth := newTestSvc(false) // authorizer would deny any record check
	cycle := seedActiveCycle(t, svc, repo, "100")

	g, err := svc.CreateGoal(context.Background(), testOrg, memberCaller(), performance.CreateGoalRequest{
		CycleID: cycle.ID, Title: "My own goal",
	})
	if err != nil {
		t.Fatalf("expected a member to set their own goal with set_own alone, got %v", err)
	}
	if g.EmployeeID != ownerEmpID {
		t.Errorf("expected the goal to target the caller's own employee record, got %s", g.EmployeeID)
	}
	// The own-goal path must short-circuit before any record check runs.
	if auth.calls != 0 {
		t.Errorf("expected no record-access check on the own-goal path, got %d", auth.calls)
	}
}

// ============================================================
// GoalRef shape
// ============================================================

// TestGoalRef_CarriesNoPerformanceFields pins decision 5 structurally rather
// than by convention. If someone later "simplifies" GoalRef into a *Goal, a
// parent reference would start leaking the parent owner's progress, weight and
// employee id to callers not scoped to see them — this fails first.
func TestGoalRef_CarriesNoPerformanceFields(t *testing.T) {
	svc, repo, _ := newTestSvc(true)
	ctx := context.Background()
	cycle := seedActiveCycle(t, svc, repo, "100")
	parent := seedGoal(t, svc, cycle.ID, otherEmpID, nil)
	child := seedGoal(t, svc, cycle.ID, ownerEmpID, nil)

	if _, err := svc.UpdateGoal(ctx, testOrg, child.ID, adminCaller(), performance.UpdateGoalRequest{
		ParentGoalID: &parent.ID,
	}); err != nil {
		t.Fatalf("align child: %v", err)
	}

	detail, err := svc.GetGoal(ctx, testOrg, child.ID, adminCaller())
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if detail.Parent == nil {
		t.Fatal("expected the parent reference to be hydrated on the detail response")
	}
	if detail.Parent.PublicID != parent.PublicID || detail.Parent.Title != parent.Title {
		t.Errorf("expected the parent ref to carry public_id and title, got %+v", detail.Parent)
	}

	// The type must expose exactly three fields — no owner, no values, no
	// weight, no status.
	if got := reflectFieldNames(detail.Parent); len(got) != 3 {
		t.Errorf("GoalRef must carry exactly public_id, title and goal_level; got %v", got)
	}
}

// TestListGoals_DoesNotHydrateParent keeps the alignment disclosure surface to
// exactly one endpoint.
func TestListGoals_DoesNotHydrateParent(t *testing.T) {
	svc, repo, _ := newTestSvc(true)
	ctx := context.Background()
	cycle := seedActiveCycle(t, svc, repo, "100")
	seedGoal(t, svc, cycle.ID, ownerEmpID, nil)

	res, err := svc.ListGoals(ctx, testOrg, performance.GoalListFilter{
		CycleID: cycle.ID, Scope: authz.ScopeAll, CallerUserID: ownerUserID,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(res.Goals) != 1 {
		t.Fatalf("expected 1 goal, got %d", len(res.Goals))
	}
	// GoalListItem has no Parent field by construction; this asserts progress
	// is still attached so the list is useful without it.
	if res.Goals[0].ProgressPercent.IsNegative() {
		t.Error("expected computed progress on list items")
	}
}

// ============================================================
// Delete guard
// ============================================================

func TestDeleteGoal_BlockedWhenCheckinsOrChildrenExist(t *testing.T) {
	ctx := context.Background()

	t.Run("blocked by check-ins", func(t *testing.T) {
		svc, repo, _ := newTestSvc(true)
		cycle := seedActiveCycle(t, svc, repo, "100")
		g := seedGoal(t, svc, cycle.ID, ownerEmpID, nil)
		if _, err := svc.CreateCheckin(ctx, testOrg, g.ID, adminCaller(), performance.CreateCheckinRequest{
			CurrentValue: dec("10"),
		}); err != nil {
			t.Fatalf("checkin: %v", err)
		}
		if err := svc.DeleteGoal(ctx, testOrg, g.ID, adminCaller()); !errors.Is(err, performance.ErrGoalHasHistory) {
			t.Fatalf("expected ErrGoalHasHistory, got %v", err)
		}
	})

	t.Run("blocked by aligned children", func(t *testing.T) {
		svc, repo, _ := newTestSvc(true)
		cycle := seedActiveCycle(t, svc, repo, "100")
		parent := seedGoal(t, svc, cycle.ID, ownerEmpID, nil)
		child := seedGoal(t, svc, cycle.ID, ownerEmpID, nil)
		if _, err := svc.UpdateGoal(ctx, testOrg, child.ID, adminCaller(), performance.UpdateGoalRequest{
			ParentGoalID: &parent.ID,
		}); err != nil {
			t.Fatalf("align: %v", err)
		}
		if err := svc.DeleteGoal(ctx, testOrg, parent.ID, adminCaller()); !errors.Is(err, performance.ErrGoalHasHistory) {
			t.Fatalf("expected ErrGoalHasHistory for a goal with children, got %v", err)
		}
	})

	t.Run("permitted when clean", func(t *testing.T) {
		svc, repo, _ := newTestSvc(true)
		cycle := seedActiveCycle(t, svc, repo, "100")
		g := seedGoal(t, svc, cycle.ID, ownerEmpID, nil)
		if err := svc.DeleteGoal(ctx, testOrg, g.ID, adminCaller()); err != nil {
			t.Fatalf("expected a history-free goal to be deletable, got %v", err)
		}
	})
}

// ============================================================
// Goal validation
// ============================================================

func TestCreateGoal_RejectsDegenerateAndMismatchedTargets(t *testing.T) {
	svc, repo, _ := newTestSvc(true)
	ctx := context.Background()
	cycle := seedActiveCycle(t, svc, repo, "100")

	same := dec("50")
	_, err := svc.CreateGoal(ctx, testOrg, adminCaller(), performance.CreateGoalRequest{
		CycleID: cycle.ID, EmployeeID: ownerEmpID, Title: "Degenerate",
		StartValue: &same, TargetValue: &same,
	})
	if !errors.Is(err, performance.ErrGoalTargetEqualsStart) {
		t.Fatalf("expected ErrGoalTargetEqualsStart, got %v", err)
	}

	// An 'increase' goal whose target is below its start is incoherent.
	start, target := dec("100"), dec("20")
	dir := string(performance.DirectionIncrease)
	_, err = svc.CreateGoal(ctx, testOrg, adminCaller(), performance.CreateGoalRequest{
		CycleID: cycle.ID, EmployeeID: ownerEmpID, Title: "Backwards",
		Direction: &dir, StartValue: &start, TargetValue: &target,
	})
	if !errors.Is(err, performance.ErrGoalDirectionMismatch) {
		t.Fatalf("expected ErrGoalDirectionMismatch, got %v", err)
	}
}
