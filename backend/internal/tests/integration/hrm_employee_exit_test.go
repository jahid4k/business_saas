// backend/internal/tests/integration/hrm_employee_exit_test.go
// Covers the two employee-exit paths that mutate hrm_employees from raw SQL
// inside their own transaction: terminations.Apply and resignations.Accept.
//
// These are unreachable from the stub-repo unit tests by construction — both
// methods take *pgxpool.Pool and issue SQL directly, bypassing the Repository
// interface the stubs implement. That gap let a real bug ship: both wrote
// hrm_employees.status, a column migration 00053 dropped when it replaced the
// text status with a status_id FK, so every Apply/Accept failed with SQLSTATE
// 42703 and rolled back its whole transaction. The module was marked done and
// no test caught it. These tests exist so that cannot recur.
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	hrmemployees "github.com/mridha/businesssaas/internal/hrm/employees"
	hrmresignations "github.com/mridha/businesssaas/internal/hrm/resignations"
	hrmterminations "github.com/mridha/businesssaas/internal/hrm/terminations"
)

// seedExitTestEmployee creates an org with the default status set that
// migration 00053 seeds (Active, plus 'Resigned' and 'Terminated' both in the
// 'terminated' category — the ambiguity the resolution queries must handle),
// and one employee sitting on Active.
func seedExitTestEmployee(t *testing.T, env *testEnv) (orgID, ownerID, employeeID, activeStatusID string) {
	t.Helper()
	ctx := context.Background()

	orgID, _, ownerID = seedScopeTestOrg(t, env)

	// seedScopeTestOrg inserts a single 'Active' status. Add the two
	// terminated-category rows in migration 00053's order, where 'Resigned' is
	// created BEFORE 'Terminated' — which is exactly why resolving by category
	// alone with ORDER BY created_at would pick the wrong one for a
	// termination.
	if err := env.db.QueryRow(ctx,
		`SELECT id FROM hrm_employee_statuses WHERE org_id = $1 AND category = 'active'`,
		orgID,
	).Scan(&activeStatusID); err != nil {
		t.Fatalf("read active status: %v", err)
	}
	for _, s := range []struct{ name, category string }{
		{"Resigned", "terminated"},
		{"Terminated", "terminated"},
	} {
		if _, err := env.db.Exec(ctx,
			`INSERT INTO hrm_employee_statuses (org_id, name, category, color) VALUES ($1, $2, $3, '')`,
			orgID, s.name, s.category,
		); err != nil {
			t.Fatalf("seed status %s: %v", s.name, err)
		}
	}

	emp, err := env.hrmEmpSvc.Create(ctx, orgID, ownerID, hrmemployees.CreateEmployeeRequest{
		FirstName: "Exit", LastName: strPtrRecruitment("Tester"), HireDate: "2020-01-01",
	})
	if err != nil {
		t.Fatalf("create employee: %v", err)
	}
	return orgID, ownerID, emp.ID, activeStatusID
}

// employeeStatusName returns the NAME of the status an employee currently
// holds, resolved through the status_id FK.
func employeeStatusName(t *testing.T, env *testEnv, employeeID string) string {
	t.Helper()
	var name string
	if err := env.db.QueryRow(context.Background(),
		`SELECT s.name FROM hrm_employees e
		 JOIN hrm_employee_statuses s ON s.id = e.status_id
		 WHERE e.id = $1`,
		employeeID,
	).Scan(&name); err != nil {
		t.Fatalf("read employee status name: %v", err)
	}
	return name
}

// ============================================================
// terminations.Apply
// ============================================================

func TestIntegration_Terminations_Apply_SetsStatusIdAndTerminationDate(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, ownerID, employeeID, activeStatusID := seedExitTestEmployee(t, env)

	// chk_hrm_term_dates requires last_working_date <= termination_date: the
	// last day in the office falls on or before the official termination date
	// (the garden-leave model), not after it.
	lastWorkingDate := "2030-06-01"
	term, err := env.hrmTerminationSvc.Create(ctx, orgID, employeeID, ownerID, hrmterminations.CreateTerminationRequest{
		TerminationType: hrmterminations.TypeInvoluntary,
		TerminationDate: "2030-06-30",
		LastWorkingDate: lastWorkingDate,
	})
	if err != nil {
		t.Fatalf("create termination: %v", err)
	}
	// No approval template configured, so Submit auto-approves.
	term, err = env.hrmTerminationSvc.Submit(ctx, orgID, employeeID, term.ID, ownerID)
	if err != nil {
		t.Fatalf("submit termination: %v", err)
	}
	if term.Status != hrmterminations.StatusApproved {
		t.Fatalf("expected auto-approve with no template configured, got %q", term.Status)
	}

	applied, err := env.hrmTerminationSvc.Apply(ctx, orgID, employeeID, term.ID, ownerID)
	if err != nil {
		t.Fatalf("apply termination: %v", err)
	}
	if applied.Status != hrmterminations.StatusApplied {
		t.Errorf("expected termination status applied, got %q", applied.Status)
	}

	// The employee must have MOVED off Active — this is the assertion that
	// fails outright against the pre-fix code, where the whole transaction
	// rolled back on SQLSTATE 42703 and the employee kept its old status.
	var gotStatusID string
	var terminationDate *time.Time
	if err := env.db.QueryRow(ctx,
		`SELECT status_id, termination_date FROM hrm_employees WHERE id = $1`, employeeID,
	).Scan(&gotStatusID, &terminationDate); err != nil {
		t.Fatalf("read employee: %v", err)
	}
	if gotStatusID == activeStatusID {
		t.Error("expected the employee to move off the Active status after applying a termination")
	}
	if terminationDate == nil {
		t.Fatal("expected termination_date to be set")
	}
	if got := terminationDate.Format("2006-01-02"); got != lastWorkingDate {
		t.Errorf("expected termination_date = last_working_date (%s), got %s", lastWorkingDate, got)
	}

	// Category alone is ambiguous — 'Resigned' and 'Terminated' share the
	// 'terminated' category and 'Resigned' is the older row. Applying a
	// TERMINATION must land on 'Terminated', not 'Resigned'.
	if name := employeeStatusName(t, env, employeeID); name != "Terminated" {
		t.Errorf("expected the employee to land on the 'Terminated' status, got %q", name)
	}
}

func TestIntegration_Terminations_Apply_FailsClosedWhenNoTerminatedStatus(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	// An org with an Active status but NO terminated-category status at all —
	// the real state of any organization created through the API, since only
	// migration 00053's backfill and POST /hrm/employee-statuses create them.
	orgID, _, ownerID := seedScopeTestOrg(t, env)
	emp, err := env.hrmEmpSvc.Create(ctx, orgID, ownerID, hrmemployees.CreateEmployeeRequest{
		FirstName: "NoStatus", HireDate: "2020-01-01",
	})
	if err != nil {
		t.Fatalf("create employee: %v", err)
	}

	term, err := env.hrmTerminationSvc.Create(ctx, orgID, emp.ID, ownerID, hrmterminations.CreateTerminationRequest{
		TerminationType: hrmterminations.TypeLayoff,
		TerminationDate: "2030-06-30",
		LastWorkingDate: "2030-06-01",
	})
	if err != nil {
		t.Fatalf("create termination: %v", err)
	}
	if _, err = env.hrmTerminationSvc.Submit(ctx, orgID, emp.ID, term.ID, ownerID); err != nil {
		t.Fatalf("submit termination: %v", err)
	}

	// Asserting the SPECIFIC sentinel, not merely "some error": a bare
	// error check would also pass against the pre-fix code, which failed here
	// for an entirely different reason (writing a dropped column).
	_, err = env.hrmTerminationSvc.Apply(ctx, orgID, emp.ID, term.ID, ownerID)
	if !errors.Is(err, hrmterminations.ErrNoTerminatedStatus) {
		t.Fatalf("expected ErrNoTerminatedStatus when the org has no terminated-category status, got %v", err)
	}

	// Failing closed matters more than the error type: the termination must
	// NOT be left marked applied against an employee who never moved.
	var status string
	if err := env.db.QueryRow(ctx, `SELECT status FROM hrm_terminations WHERE id = $1`, term.ID).Scan(&status); err != nil {
		t.Fatalf("read termination: %v", err)
	}
	if status == string(hrmterminations.StatusApplied) {
		t.Error("expected the whole Apply transaction to roll back, leaving the termination un-applied")
	}
}

// ============================================================
// resignations.Accept
// ============================================================

func TestIntegration_Resignations_Accept_SetsStatusIdAndTerminationDate(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, ownerID, employeeID, activeStatusID := seedExitTestEmployee(t, env)

	lastWorkingDate := "2030-09-30"
	res, err := env.hrmResignationSvc.Submit(ctx, orgID, employeeID, ownerID, hrmresignations.SubmitResignationRequest{
		ResignationDate: "2030-09-01",
		LastWorkingDate: &lastWorkingDate,
	})
	if err != nil {
		t.Fatalf("submit resignation: %v", err)
	}

	accepted, err := env.hrmResignationSvc.Accept(ctx, orgID, employeeID, res.ID, ownerID)
	if err != nil {
		t.Fatalf("accept resignation: %v", err)
	}
	if accepted.Status != hrmresignations.StatusAccepted {
		t.Errorf("expected resignation status accepted, got %q", accepted.Status)
	}

	var gotStatusID string
	var terminationDate *time.Time
	if err := env.db.QueryRow(ctx,
		`SELECT status_id, termination_date FROM hrm_employees WHERE id = $1`, employeeID,
	).Scan(&gotStatusID, &terminationDate); err != nil {
		t.Fatalf("read employee: %v", err)
	}
	if gotStatusID == activeStatusID {
		t.Error("expected the employee to move off the Active status after accepting a resignation")
	}
	if terminationDate == nil {
		t.Fatal("expected termination_date to be set")
	}
	if got := terminationDate.Format("2006-01-02"); got != lastWorkingDate {
		t.Errorf("expected termination_date = last_working_date (%s), got %s", lastWorkingDate, got)
	}

	// The mirror of the termination assertion: a RESIGNATION must land on
	// 'Resigned', not on the sibling 'Terminated' row in the same category.
	if name := employeeStatusName(t, env, employeeID); name != "Resigned" {
		t.Errorf("expected the employee to land on the 'Resigned' status, got %q", name)
	}
}

func TestIntegration_Resignations_Accept_FailsClosedWhenNoStatusAvailable(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	orgID, _, ownerID := seedScopeTestOrg(t, env)
	emp, err := env.hrmEmpSvc.Create(ctx, orgID, ownerID, hrmemployees.CreateEmployeeRequest{
		FirstName: "NoStatus", HireDate: "2020-01-01",
	})
	if err != nil {
		t.Fatalf("create employee: %v", err)
	}

	lastWorkingDate := "2030-09-30"
	res, err := env.hrmResignationSvc.Submit(ctx, orgID, emp.ID, ownerID, hrmresignations.SubmitResignationRequest{
		ResignationDate: "2030-09-01",
		LastWorkingDate: &lastWorkingDate,
	})
	if err != nil {
		t.Fatalf("submit resignation: %v", err)
	}

	// The specific sentinel, for the same reason as the termination case above.
	if _, err = env.hrmResignationSvc.Accept(ctx, orgID, emp.ID, res.ID, ownerID); !errors.Is(err, hrmresignations.ErrNoResignedStatus) {
		t.Fatalf("expected ErrNoResignedStatus when the org has no terminated-category status, got %v", err)
	}

	var status string
	if err := env.db.QueryRow(ctx, `SELECT status FROM hrm_resignations WHERE id = $1`, res.ID).Scan(&status); err != nil {
		t.Fatalf("read resignation: %v", err)
	}
	if status == string(hrmresignations.StatusAccepted) {
		t.Error("expected the whole Accept transaction to roll back, leaving the resignation un-accepted")
	}
}
