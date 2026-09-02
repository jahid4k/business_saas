// backend/internal/tests/integration/hrm_dropped_status_column_test.go
// Regression tests for the third occurrence of a single bug class: raw SQL
// still querying hrm_employees.status, a column migration 00053 REPLACED with
// status_id.
//
// Five query sites across four modules were broken — payroll compute, calendar
// and announcement audience resolution, and the HRM headcount summary. Every
// one failed outright with SQLSTATE 42703 ("column does not exist"), so the
// features were not degraded, they were dead. All four modules are marked
// ✅ DONE in the docs.
//
// Why nothing caught it: each site is raw SQL reached through *pgxpool.Pool,
// not through the Repository interface the unit tests stub. That is exactly
// the gap that hid terminations.Apply and resignations.Accept for four phases
// (see hrm_employee_exit_test.go). These tests close it by exercising the real
// queries against real Postgres.
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"testing"
	"time"

	hrmannouncements "github.com/mridha/businesssaas/internal/hrm/announcements"
	hrmcalendar "github.com/mridha/businesssaas/internal/hrm/calendar"
	hrmpayslips "github.com/mridha/businesssaas/internal/hrm/payslips"
	hrmreports "github.com/mridha/businesssaas/internal/hrm/reports"
)

// setEmployeeStatus repoints an employee at the org's status row with the
// given category, so a test can place someone in a category without depending
// on the seeded status NAMES (which orgs may rename freely).
func setEmployeeStatus(t *testing.T, env *testEnv, orgID, employeeID, category string) {
	t.Helper()
	ctx := context.Background()

	var statusID string
	err := env.db.QueryRow(ctx,
		`SELECT id FROM hrm_employee_statuses WHERE org_id = $1 AND category = $2
		 ORDER BY created_at LIMIT 1`, orgID, category).Scan(&statusID)
	if err != nil {
		// seedScopeTestOrg only creates an 'active' row; create the rest on demand.
		if err := env.db.QueryRow(ctx,
			`INSERT INTO hrm_employee_statuses (org_id, name, category)
			 VALUES ($1, $2, $3) RETURNING id`,
			orgID, category+"-test", category).Scan(&statusID); err != nil {
			t.Fatalf("create %s status: %v", category, err)
		}
	}
	if _, err := env.db.Exec(ctx,
		`UPDATE hrm_employees SET status_id = $1 WHERE id = $2`, statusID, employeeID); err != nil {
		t.Fatalf("set employee status to %s: %v", category, err)
	}
}

// ============================================================
// Payroll — the worst of the five
// ============================================================

// TestIntegration_DroppedStatusColumn_PayrollComputeRuns is the headline test.
//
// ComputeRun's employee query read `e.status IN ('active','resigned')`. That
// failed 42703, so ComputeRun returned an error before computing a single
// payslip — payroll was entirely non-functional. This test would not even
// reach its assertions against the old code; it fails at ComputeRun.
//
// It also pins WHO GETS PAID, which the fix had to decide because the old
// filter was broken in a second way: 'resigned' was never a valid value even
// on the original 00021 CHECK, so it matched nothing. The rule is now active +
// on_leave + leavers whose termination_date falls inside the period.
func TestIntegration_DroppedStatusColumn_PayrollComputeRuns(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)

	activeEmp := seedEmployee(t, env, orgID, statusID, ownerID, "", "Active Person", nil)
	onLeaveEmp := seedEmployee(t, env, orgID, statusID, ownerID, "", "On Leave Person", nil)
	midLeaver := seedEmployee(t, env, orgID, statusID, ownerID, "", "Mid Period Leaver", nil)
	oldLeaver := seedEmployee(t, env, orgID, statusID, ownerID, "", "Long Gone", nil)

	setEmployeeStatus(t, env, orgID, onLeaveEmp, "on_leave")
	setEmployeeStatus(t, env, orgID, midLeaver, "terminated")
	setEmployeeStatus(t, env, orgID, oldLeaver, "terminated")

	// The run is for the current month, so "mid period" and "long gone" are
	// unambiguous relative to it.
	now := time.Now().UTC()
	year, month := now.Year(), int(now.Month())
	periodStart := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)

	if _, err := env.db.Exec(ctx,
		`UPDATE hrm_employees SET termination_date = $1 WHERE id = $2`,
		periodStart.AddDate(0, 0, 10), midLeaver); err != nil {
		t.Fatalf("set mid-period termination: %v", err)
	}
	if _, err := env.db.Exec(ctx,
		`UPDATE hrm_employees SET termination_date = $1 WHERE id = $2`,
		periodStart.AddDate(0, -3, 0), oldLeaver); err != nil {
		t.Fatalf("set old termination: %v", err)
	}

	run, err := env.hrmPayslipsSvc.CreateRun(ctx, orgID, ownerID, hrmpayslips.CreateRunRequest{
		Year: year, Month: month,
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	// Against the pre-fix code this line fails with 42703 and the test ends here.
	computed, err := env.hrmPayslipsSvc.ComputeRun(ctx, orgID, run.ID, ownerID)
	if err != nil {
		t.Fatalf("ComputeRun failed — payroll cannot compute: %v", err)
	}
	if computed.Status != hrmpayslips.RunComputed {
		t.Errorf("run status = %s, want computed", computed.Status)
	}

	paid := map[string]bool{}
	rows, err := env.db.Query(ctx,
		`SELECT employee_id::text FROM hrm_payslips WHERE payslip_run_id = $1`, run.ID)
	if err != nil {
		t.Fatalf("read payslips: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		paid[id] = true
	}

	if !paid[activeEmp] {
		t.Error("an active employee was not paid")
	}
	if !paid[onLeaveEmp] {
		t.Error("an on-leave employee was not paid — being on leave does not stop pay")
	}
	if !paid[midLeaver] {
		t.Error("an employee who left mid-period was not paid for the days they worked")
	}
	if paid[oldLeaver] {
		t.Error("an employee who left three months ago was paid")
	}
}

// ============================================================
// Audience resolution — calendar and announcements
// ============================================================

// TestIntegration_DroppedStatusColumn_AudienceResolution covers both
// GetTargetEmployeeIDs implementations, which are the same query in two
// packages. Org-wide and department scopes both filtered on the dropped
// column; the individual scope never did, so it kept working and masked the
// breakage in casual testing.
func TestIntegration_DroppedStatusColumn_AudienceResolution(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)

	activeEmp := seedEmployee(t, env, orgID, statusID, ownerID, "", "Active", nil)
	goneEmp := seedEmployee(t, env, orgID, statusID, ownerID, "", "Gone", nil)
	setEmployeeStatus(t, env, orgID, goneEmp, "terminated")

	t.Run("calendar", func(t *testing.T) {
		repo := hrmcalendar.NewRepository(env.db)
		ids, err := repo.GetTargetEmployeeIDs(ctx, orgID, hrmcalendar.ScopeOrganization, nil)
		if err != nil {
			t.Fatalf("calendar audience resolution failed: %v", err)
		}
		assertAudience(t, ids, activeEmp, goneEmp)
	})

	t.Run("announcements", func(t *testing.T) {
		repo := hrmannouncements.NewRepository(env.db)
		ids, err := repo.GetTargetEmployeeIDs(ctx, orgID, hrmannouncements.ScopeOrganization, nil)
		if err != nil {
			t.Fatalf("announcement audience resolution failed: %v", err)
		}
		assertAudience(t, ids, activeEmp, goneEmp)
	})
}

func assertAudience(t *testing.T, ids []string, wantIncluded, wantExcluded string) {
	t.Helper()
	got := map[string]bool{}
	for _, id := range ids {
		got[id] = true
	}
	if !got[wantIncluded] {
		t.Errorf("active employee %s missing from the audience", wantIncluded)
	}
	if got[wantExcluded] {
		t.Errorf("terminated employee %s was included in the audience", wantExcluded)
	}
}

// ============================================================
// Headcount summary
// ============================================================

// TestIntegration_DroppedStatusColumn_ReportsSummary. All four headcounts sat
// in ONE statement, so the broken sub-selects took the entire summary down
// with them — including the department, position and leave counts that were
// themselves fine.
func TestIntegration_DroppedStatusColumn_ReportsSummary(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)

	_ = seedEmployee(t, env, orgID, statusID, ownerID, "", "Active One", nil)
	_ = seedEmployee(t, env, orgID, statusID, ownerID, "", "Active Two", nil)
	onLeave := seedEmployee(t, env, orgID, statusID, ownerID, "", "On Leave", nil)
	gone := seedEmployee(t, env, orgID, statusID, ownerID, "", "Gone", nil)
	setEmployeeStatus(t, env, orgID, onLeave, "on_leave")
	setEmployeeStatus(t, env, orgID, gone, "terminated")

	repo := hrmreports.NewRepository(env.db)
	summary, err := repo.GetSummary(ctx, orgID)
	if err != nil {
		t.Fatalf("GetSummary failed — the whole HRM dashboard is dead: %v", err)
	}

	// seedScopeTestOrg's owner has no employee row, so the four seeded above are
	// the entire population.
	if summary.TotalEmployees != 4 {
		t.Errorf("total = %d, want 4", summary.TotalEmployees)
	}
	if summary.ActiveEmployees != 2 {
		t.Errorf("active = %d, want 2", summary.ActiveEmployees)
	}
	if summary.OnLeaveEmployees != 1 {
		t.Errorf("on_leave = %d, want 1", summary.OnLeaveEmployees)
	}
	if summary.TerminatedEmployees != 1 {
		t.Errorf("terminated = %d, want 1", summary.TerminatedEmployees)
	}
}
