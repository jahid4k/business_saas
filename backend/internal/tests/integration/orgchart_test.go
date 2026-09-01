// backend/internal/tests/integration/orgchart_test.go
// Phase 10A: the org chart.
//
// The load-bearing claim is that hrm_employees.manager_id and
// hrm_reporting_relationships NEVER drift — scope.Predicate's view_team tier
// is a recursive CTE on that column, so a drift silently changes who can read
// whose payroll. Cycle detection matters for the same reason: a loop makes
// that authorization query non-terminating.
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/mridha/businesssaas/internal/hrm/orgchart"
)

type chartFixture struct {
	orgID    string
	statusID string
	ownerID  string
	e        map[string]string // label -> employee id
}

func chartCaller(userID string) orgchart.Caller {
	return orgchart.Caller{UserID: userID, CanManage: true}
}

func seedChartFixture(t *testing.T, env *testEnv, labels ...string) *chartFixture {
	t.Helper()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	fx := &chartFixture{orgID: orgID, statusID: statusID, ownerID: ownerID, e: map[string]string{}}
	for _, l := range labels {
		fx.e[l] = seedEmployee(t, env, orgID, statusID, ownerID, "", "Emp "+l, nil)
	}
	return fx
}

// managerIDOf reads the DENORMALIZED column — the one authorization follows.
func managerIDOf(t *testing.T, env *testEnv, employeeID string) *string {
	t.Helper()
	var m *string
	if err := env.db.QueryRow(context.Background(),
		`SELECT manager_id::text FROM hrm_employees WHERE id=$1`, employeeID).Scan(&m); err != nil {
		t.Fatalf("read manager_id: %v", err)
	}
	return m
}

// ============================================================
// The drift claim
// ============================================================

// TestIntegration_OrgChart_SolidLineSyncsManagerID is the claim the whole
// slice rests on. hrm_employees.manager_id is what scope.Predicate's view_team
// CTE joins on, so a relationship committed without syncing it would leave
// every scope-tiered permission in the product following a stale line.
func TestIntegration_OrgChart_SolidLineSyncsManagerID(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedChartFixture(t, env, "worker", "boss", "newboss")
	caller := chartCaller(fx.ownerID)

	if before := managerIDOf(t, env, fx.e["worker"]); before != nil {
		t.Fatalf("manager_id starts at %v, want NULL", *before)
	}

	rel, err := env.hrmOrgChartSvc.CreateRelationship(ctx, fx.orgID, caller,
		orgchart.CreateRelationshipRequest{
			EmployeeID: fx.e["worker"], ManagerID: fx.e["boss"], RelationshipType: "solid",
		})
	if err != nil {
		t.Fatalf("create solid line: %v", err)
	}

	got := managerIDOf(t, env, fx.e["worker"])
	if got == nil || *got != fx.e["boss"] {
		t.Fatalf("manager_id = %v after a solid line to %s — the column authorization "+
			"follows was not synced", got, fx.e["boss"])
	}

	// Ending it must CLEAR the column. Leaving it pointing at a manager the
	// table says is no longer theirs would keep granting view_team access.
	if _, err := env.hrmOrgChartSvc.EndRelationship(ctx, fx.orgID, caller, rel.ID,
		orgchart.EndRelationshipRequest{}); err != nil {
		t.Fatalf("end relationship: %v", err)
	}
	if after := managerIDOf(t, env, fx.e["worker"]); after != nil {
		t.Errorf("manager_id = %v after ending the solid line, want NULL — a stale pointer "+
			"keeps granting view_team access to a former manager", *after)
	}
}

// TestIntegration_OrgChart_MatrixLinesDoNotGrantAccess — dotted, functional
// and project lines are real reporting that the chart must draw, but they must
// NEVER widen data access. A project lead reading their contributors' payroll
// because of a project line would be a quiet privilege escalation.
func TestIntegration_OrgChart_MatrixLinesDoNotGrantAccess(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedChartFixture(t, env, "worker", "projectlead")
	caller := chartCaller(fx.ownerID)

	for _, relType := range []string{"dotted", "functional", "project"} {
		if _, err := env.hrmOrgChartSvc.CreateRelationship(ctx, fx.orgID, caller,
			orgchart.CreateRelationshipRequest{
				EmployeeID: fx.e["worker"], ManagerID: fx.e["projectlead"], RelationshipType: relType,
			}); err != nil {
			t.Fatalf("create %s line: %v", relType, err)
		}
		if m := managerIDOf(t, env, fx.e["worker"]); m != nil {
			t.Fatalf("a %s line set manager_id to %v — matrix reporting must not confer "+
				"data access", relType, *m)
		}
	}
}

// ============================================================
// Cycle detection
// ============================================================

// TestIntegration_OrgChart_RefusesIndirectCycle is the case a parent-only
// check misses. A cycle makes scope.Predicate's recursive CTE
// non-terminating, so this is an authorization safety check.
func TestIntegration_OrgChart_RefusesIndirectCycle(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedChartFixture(t, env, "a", "b", "c")
	caller := chartCaller(fx.ownerID)

	// Build a -> b -> c.
	for _, pair := range [][2]string{{"a", "b"}, {"b", "c"}} {
		if _, err := env.hrmOrgChartSvc.CreateRelationship(ctx, fx.orgID, caller,
			orgchart.CreateRelationshipRequest{
				EmployeeID: fx.e[pair[0]], ManagerID: fx.e[pair[1]], RelationshipType: "solid",
			}); err != nil {
			t.Fatalf("create %s->%s: %v", pair[0], pair[1], err)
		}
	}

	// c -> a closes a->b->c->a. A parent-only check would allow this.
	_, err := env.hrmOrgChartSvc.CreateRelationship(ctx, fx.orgID, caller,
		orgchart.CreateRelationshipRequest{
			EmployeeID: fx.e["c"], ManagerID: fx.e["a"], RelationshipType: "solid",
		})
	if !errors.Is(err, orgchart.ErrWouldCreateCycle) {
		t.Fatalf("closing a three-hop cycle returned %v, want ErrWouldCreateCycle", err)
	}

	// And nothing was written — neither the row nor the column.
	if m := managerIDOf(t, env, fx.e["c"]); m != nil {
		t.Errorf("manager_id was set to %v despite the cycle being refused", *m)
	}
}

// TestIntegration_OrgChart_RefusesSelfAndDuplicateSolid
func TestIntegration_OrgChart_RefusesSelfAndDuplicateSolid(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedChartFixture(t, env, "a", "b", "c")
	caller := chartCaller(fx.ownerID)

	_, err := env.hrmOrgChartSvc.CreateRelationship(ctx, fx.orgID, caller,
		orgchart.CreateRelationshipRequest{
			EmployeeID: fx.e["a"], ManagerID: fx.e["a"], RelationshipType: "solid",
		})
	if !errors.Is(err, orgchart.ErrSelfManagement) {
		t.Errorf("self-management returned %v, want ErrSelfManagement", err)
	}

	if _, err := env.hrmOrgChartSvc.CreateRelationship(ctx, fx.orgID, caller,
		orgchart.CreateRelationshipRequest{
			EmployeeID: fx.e["a"], ManagerID: fx.e["b"], RelationshipType: "solid",
		}); err != nil {
		t.Fatalf("first solid line: %v", err)
	}
	// A second active solid manager would make manager_id ambiguous and the
	// view_team CTE non-deterministic.
	_, err = env.hrmOrgChartSvc.CreateRelationship(ctx, fx.orgID, caller,
		orgchart.CreateRelationshipRequest{
			EmployeeID: fx.e["a"], ManagerID: fx.e["c"], RelationshipType: "solid",
		})
	if !errors.Is(err, orgchart.ErrDuplicateSolid) {
		t.Errorf("second solid manager returned %v, want ErrDuplicateSolid", err)
	}

	// But a matrix line alongside the solid one is fine — that is the point
	// of having types at all.
	if _, err := env.hrmOrgChartSvc.CreateRelationship(ctx, fx.orgID, caller,
		orgchart.CreateRelationshipRequest{
			EmployeeID: fx.e["a"], ManagerID: fx.e["c"], RelationshipType: "dotted",
		}); err != nil {
		t.Errorf("a dotted line alongside a solid one was refused: %v", err)
	}
}

// ============================================================
// History and chart shape
// ============================================================

// TestIntegration_OrgChart_EndingKeepsHistory — a relationship is ended by
// stamping effective_to, never deleted. "Who did this person report to in
// March" is what makes the table worth having over the bare column.
func TestIntegration_OrgChart_EndingKeepsHistory(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedChartFixture(t, env, "worker", "boss")
	caller := chartCaller(fx.ownerID)

	rel, err := env.hrmOrgChartSvc.CreateRelationship(ctx, fx.orgID, caller,
		orgchart.CreateRelationshipRequest{
			EmployeeID: fx.e["worker"], ManagerID: fx.e["boss"], RelationshipType: "solid",
		})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := env.hrmOrgChartSvc.EndRelationship(ctx, fx.orgID, caller, rel.ID,
		orgchart.EndRelationshipRequest{}); err != nil {
		t.Fatalf("end: %v", err)
	}

	all, err := env.hrmOrgChartSvc.ListRelationships(ctx, fx.orgID, caller, fx.e["worker"], false)
	if err != nil {
		t.Fatalf("list history: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("%d historical relationships, want 1 — ending must not delete the row", len(all))
	}
	if all[0].EffectiveTo == nil {
		t.Error("the ended relationship has no effective_to stamp")
	}

	active, err := env.hrmOrgChartSvc.ListRelationships(ctx, fx.orgID, caller, fx.e["worker"], true)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(active) != 0 {
		t.Errorf("%d active relationships after ending, want 0", len(active))
	}

	// Ending twice is refused rather than silently re-stamping.
	if _, err := env.hrmOrgChartSvc.EndRelationship(ctx, fx.orgID, caller, rel.ID,
		orgchart.EndRelationshipRequest{}); !errors.Is(err, orgchart.ErrAlreadyEnded) {
		t.Errorf("ending twice returned %v, want ErrAlreadyEnded", err)
	}
}

// TestIntegration_OrgChart_ChartReflectsTheColumnAuthorizationUses — the chart
// is rendered from manager_id on purpose, so a drift between table and column
// would be VISIBLE rather than hidden behind a prettier query.
func TestIntegration_OrgChart_ChartReflectsTheColumnAuthorizationUses(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedChartFixture(t, env, "worker", "boss")
	caller := chartCaller(fx.ownerID)

	if _, err := env.hrmOrgChartSvc.CreateRelationship(ctx, fx.orgID, caller,
		orgchart.CreateRelationshipRequest{
			EmployeeID: fx.e["worker"], ManagerID: fx.e["boss"], RelationshipType: "solid",
		}); err != nil {
		t.Fatalf("create: %v", err)
	}
	// A matrix line that must appear separately, never as a child edge.
	if _, err := env.hrmOrgChartSvc.CreateRelationship(ctx, fx.orgID, caller,
		orgchart.CreateRelationshipRequest{
			EmployeeID: fx.e["boss"], ManagerID: fx.e["worker"], RelationshipType: "dotted",
		}); err != nil {
		t.Fatalf("create dotted: %v", err)
	}

	nodes, err := env.hrmOrgChartSvc.GetChart(ctx, fx.orgID, caller)
	if err != nil {
		t.Fatalf("get chart: %v", err)
	}
	byID := map[string]*orgchart.ChartNode{}
	for _, n := range nodes {
		byID[n.EmployeeID] = n
	}
	boss := byID[fx.e["boss"]]
	if boss == nil {
		t.Fatal("boss missing from the chart")
	}
	found := false
	for _, cid := range boss.ChildIDs {
		if cid == fx.e["worker"] {
			found = true
		}
	}
	if !found {
		t.Error("the solid report is not a child edge on the chart")
	}
	// The dotted line runs boss -> worker; it must NOT appear as worker's
	// child edge, or the chart would imply access that does not exist.
	worker := byID[fx.e["worker"]]
	for _, cid := range worker.ChildIDs {
		if cid == fx.e["boss"] {
			t.Error("a dotted line was drawn as a reporting child edge")
		}
	}
	if len(boss.MatrixLines) == 0 {
		t.Error("the dotted line is missing from MatrixLines — the chart must still draw it")
	}
}

// TestIntegration_OrgChart_VacantSeatIsRepresentable — a vacant seat is what a
// future requisition is raised against, so vacating must not delete the seat.
func TestIntegration_OrgChart_VacantSeatIsRepresentable(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedChartFixture(t, env, "worker")
	caller := chartCaller(fx.ownerID)

	var positionID string
	if err := env.db.QueryRow(ctx,
		`INSERT INTO hrm_positions (org_id, title, created_by) VALUES ($1,'Senior Engineer',$2) RETURNING id`,
		fx.orgID, fx.ownerID).Scan(&positionID); err != nil {
		t.Fatalf("seed position: %v", err)
	}

	occupant := fx.e["worker"]
	seat, err := env.hrmOrgChartSvc.CreateSeat(ctx, fx.orgID, caller, orgchart.CreateSeatRequest{
		PositionID: positionID, EmployeeID: &occupant,
	})
	if err != nil {
		t.Fatalf("create seat: %v", err)
	}
	if seat.IsVacant() {
		t.Fatal("a seat created with an occupant reports vacant")
	}

	vacated, err := env.hrmOrgChartSvc.AssignSeat(ctx, fx.orgID, caller, seat.ID,
		orgchart.AssignSeatRequest{EmployeeID: nil})
	if err != nil {
		t.Fatalf("vacate: %v", err)
	}
	if !vacated.IsVacant() {
		t.Error("the seat is not vacant after clearing its occupant")
	}

	vacancies, err := env.hrmOrgChartSvc.ListSeats(ctx, fx.orgID, caller, positionID, true)
	if err != nil {
		t.Fatalf("list vacancies: %v", err)
	}
	if len(vacancies) != 1 {
		t.Errorf("%d vacant seats, want 1 — vacating must keep the seat as headcount, "+
			"because that is what a requisition is raised against", len(vacancies))
	}
}
