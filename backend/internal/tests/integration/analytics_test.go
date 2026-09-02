// backend/internal/tests/integration/analytics_test.go
// Phase 10C: people analytics.
//
// Three claims carry this slice, and each is one the plan names explicitly:
//
//  1. Analytics reads the SNAPSHOT, never OLTP. Proved by mutating OLTP and
//     showing the metric does not move until the nightly job has run.
//  2. Attrition splits voluntary/involuntary AND regretted/non-regretted,
//     the latter from Phase 9's rehire flag — with UNKNOWN kept separate.
//  3. A DEI group below the suppression threshold reports suppressed and
//     cannot be differenced out of the total.
//
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/hrm/analytics"
	"github.com/shopspring/decimal"
)

type anFixture struct {
	orgID    string
	statusID string
	ownerID  string
}

func analyst(userID string) analytics.Caller {
	return analytics.Caller{
		UserID: userID, CanView: true, CanViewCompensation: true,
		CanViewDEI: true, CanExport: true, CanManage: true,
	}
}

// anManager mirrors the 00126 grant exactly: view and nothing else.
func anManager(userID string) analytics.Caller {
	return analytics.Caller{UserID: userID, CanView: true}
}

func seedAnFixture(t *testing.T, env *testEnv) *anFixture {
	t.Helper()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	return &anFixture{orgID: orgID, statusID: statusID, ownerID: ownerID}
}

// seedHire inserts an employee with an explicit hire date and optional gender.
func seedHire(t *testing.T, env *testEnv, fx *anFixture, name string, hire time.Time, gender *string) string {
	t.Helper()
	var id string
	if err := env.db.QueryRow(context.Background(),
		`INSERT INTO hrm_employees (org_id, status_id, first_name, hire_date, gender, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		fx.orgID, fx.statusID, name, hire, gender, fx.ownerID).Scan(&id); err != nil {
		t.Fatalf("seed hire %s: %v", name, err)
	}
	return id
}

// seedExit writes the 9A umbrella row plus, for a termination, its source and
// rehire decision — the real shape the fact builder reads.
//
// rehire: "" means no decision was ever recorded, which must stay UNKNOWN.
func seedExit(t *testing.T, env *testEnv, fx *anFixture, employeeID, sourceType, terminationType string, lastWorkingDate time.Time, rehire string) string {
	t.Helper()
	ctx := context.Background()
	var sourceID *string
	if sourceType == "termination" {
		var tid string
		if err := env.db.QueryRow(ctx,
			`INSERT INTO hrm_terminations (org_id, employee_id, termination_type, termination_date,
			     last_working_date, status, created_by)
			 VALUES ($1,$2,$3,$4,$4,'applied',$5) RETURNING id`,
			fx.orgID, employeeID, terminationType, lastWorkingDate, fx.ownerID).Scan(&tid); err != nil {
			t.Fatalf("seed termination: %v", err)
		}
		sourceID = &tid
	} else {
		var rid string
		if err := env.db.QueryRow(ctx,
			`INSERT INTO hrm_resignations (org_id, employee_id, resignation_date, last_working_date,
			     status, created_by)
			 VALUES ($1,$2,$3,$4,'accepted',$5) RETURNING id`,
			fx.orgID, employeeID, lastWorkingDate.AddDate(0, -1, 0), lastWorkingDate, fx.ownerID).Scan(&rid); err != nil {
			t.Fatalf("seed resignation: %v", err)
		}
		sourceID = &rid
	}

	var exitID string
	if err := env.db.QueryRow(ctx,
		`INSERT INTO hrm_exits (org_id, employee_id, source_type, source_id, last_working_date,
		     status, created_by)
		 VALUES ($1,$2,$3,$4,$5,'completed',$6) RETURNING id`,
		fx.orgID, employeeID, sourceType, sourceID, lastWorkingDate, fx.ownerID).Scan(&exitID); err != nil {
		t.Fatalf("seed exit: %v", err)
	}
	if rehire != "" {
		if _, err := env.db.Exec(ctx,
			`INSERT INTO hrm_rehire_eligibility (org_id, employee_id, exit_id, status, decided_by, decided_at)
			 VALUES ($1,$2,$3,$4,$5,NOW())`,
			fx.orgID, employeeID, exitID, rehire, fx.ownerID); err != nil {
			t.Fatalf("seed rehire eligibility: %v", err)
		}
	}
	// The employee row carries the termination date too, which is what the
	// headcount population filter reads.
	if _, err := env.db.Exec(ctx,
		`UPDATE hrm_employees SET termination_date=$2 WHERE id=$1`, employeeID, lastWorkingDate); err != nil {
		t.Fatalf("stamp termination_date: %v", err)
	}
	return exitID
}

// ============================================================
// Claim 1 — the metric reads the snapshot, never OLTP
// ============================================================

// TestIntegration_Analytics_MetricDoesNotMoveUntilTheJobRuns is the plan's
// stated verification, and the rule the whole slice is built around.
//
// If the read path aggregated live tables the number would change under a
// reader between two refreshes of the same page, and — worse — correcting an
// old record would silently rewrite last March.
func TestIntegration_Analytics_MetricDoesNotMoveUntilTheJobRuns(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedAnFixture(t, env)
	caller := analyst(fx.ownerID)
	today := time.Now().Truncate(24 * time.Hour)
	from, to := today.AddDate(0, -6, 0), today

	for i := 0; i < 6; i++ {
		seedHire(t, env, fx, fmt.Sprintf("Staff %d", i), today.AddDate(-3, 0, 0), nil)
	}
	if _, err := env.hrmAnalyticsSvc.RunSnapshotForOrg(ctx, fx.orgID, caller, today); err != nil {
		t.Fatalf("initial snapshot: %v", err)
	}

	before, err := env.hrmAnalyticsSvc.Attrition(ctx, fx.orgID, caller, from, to)
	if err != nil {
		t.Fatalf("attrition before: %v", err)
	}
	if before.Leavers != 0 || before.ClosingHeadcount != 6 {
		t.Fatalf("baseline is leavers=%d headcount=%d, want 0 and 6",
			before.Leavers, before.ClosingHeadcount)
	}

	// Mutate OLTP: a real, completed exit.
	leaver := seedHire(t, env, fx, "Departing", today.AddDate(-2, 0, 0), nil)
	seedExit(t, env, fx, leaver, "resignation", "", today.AddDate(0, 0, -1), "eligible")

	// ⚠ The metric must NOT have moved. This is the assertion; everything
	// else in the test is setup for it.
	during, err := env.hrmAnalyticsSvc.Attrition(ctx, fx.orgID, caller, from, to)
	if err != nil {
		t.Fatalf("attrition during: %v", err)
	}
	if during.Leavers != 0 {
		t.Errorf("attrition reported %d leavers straight from an OLTP write — the read path "+
			"is aggregating live tables instead of the fact tables", during.Leavers)
	}
	if during.ClosingHeadcount != before.ClosingHeadcount {
		t.Errorf("headcount moved from %d to %d without the job running",
			before.ClosingHeadcount, during.ClosingHeadcount)
	}

	// Now run the job, and it must appear.
	res, err := env.hrmAnalyticsSvc.RunSnapshotForOrg(ctx, fx.orgID, caller, today)
	if err != nil {
		t.Fatalf("snapshot run: %v", err)
	}
	if res.FactsWritten == 0 {
		t.Fatal("the job wrote no attrition facts")
	}
	after, err := env.hrmAnalyticsSvc.Attrition(ctx, fx.orgID, caller, from, to)
	if err != nil {
		t.Fatalf("attrition after: %v", err)
	}
	if after.Leavers != 1 {
		t.Errorf("attrition reported %d leavers after the job ran, want 1", after.Leavers)
	}
	if after.AttritionRate == nil {
		t.Error("no attrition rate after the job ran")
	}
}

// TestIntegration_Analytics_EmptyPeriodHasNoRateRatherThanZero — a confident
// 0% for an organization with no snapshot is worse than a gap in the chart.
func TestIntegration_Analytics_EmptyPeriodHasNoRateRatherThanZero(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedAnFixture(t, env)
	caller := analyst(fx.ownerID)
	today := time.Now().Truncate(24 * time.Hour)

	sum, err := env.hrmAnalyticsSvc.Attrition(ctx, fx.orgID, caller, today.AddDate(0, -1, 0), today)
	if err != nil {
		t.Fatalf("attrition: %v", err)
	}
	if sum.AttritionRate != nil {
		t.Errorf("attrition rate = %s with no snapshot to divide by, want no rate at all",
			sum.AttritionRate)
	}
	if sum.Leavers != 0 {
		t.Errorf("leavers = %d on an empty org", sum.Leavers)
	}
	// And an inverted period is refused rather than silently returning nothing.
	if _, err := env.hrmAnalyticsSvc.Attrition(ctx, fx.orgID, caller, today, today.AddDate(0, -1, 0)); !errors.Is(err, analytics.ErrInvalidPeriod) {
		t.Errorf("an inverted period returned %v, want ErrInvalidPeriod", err)
	}
}

// ============================================================
// Claim 2 — the four-way attrition split
// ============================================================

// TestIntegration_Analytics_AttritionSplitsFourWays covers both axes at once,
// including the case that makes the regretted split honest: an exit nobody
// has reviewed must stay UNKNOWN rather than counting as non-regretted.
func TestIntegration_Analytics_AttritionSplitsFourWays(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedAnFixture(t, env)
	caller := analyst(fx.ownerID)
	today := time.Now().Truncate(24 * time.Hour)
	lwd := today.AddDate(0, 0, -10)
	hire := today.AddDate(-4, 0, 0)

	// A population so there is a denominator.
	for i := 0; i < 10; i++ {
		seedHire(t, env, fx, fmt.Sprintf("Staff %d", i), hire, nil)
	}

	cases := []struct {
		name, source, termType, rehire string
		wantVoluntary                  bool
		wantRegretted                  string // "yes" / "no" / "unknown"
	}{
		{"resigned, would rehire", "resignation", "", "eligible", true, "yes"},
		{"resigned, would not rehire", "resignation", "", "not_eligible", true, "no"},
		{"resigned, never reviewed", "resignation", "", "", true, "unknown"},
		{"resigned, conditional", "resignation", "", "conditional", true, "unknown"},
		{"retired", "termination", "retirement", "eligible", true, "yes"},
		{"dismissed", "termination", "involuntary", "not_eligible", false, "no"},
		{"laid off", "termination", "layoff", "eligible", false, "yes"},
		{"contract ended", "termination", "contract_end", "", false, "unknown"},
	}
	for i, c := range cases {
		emp := seedHire(t, env, fx, fmt.Sprintf("Leaver %d", i), hire, nil)
		seedExit(t, env, fx, emp, c.source, c.termType, lwd, c.rehire)
	}

	if _, err := env.hrmAnalyticsSvc.RunSnapshotForOrg(ctx, fx.orgID, caller, today); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	sum, err := env.hrmAnalyticsSvc.Attrition(ctx, fx.orgID, caller, today.AddDate(0, -1, 0), today)
	if err != nil {
		t.Fatalf("attrition: %v", err)
	}

	if sum.Leavers != 8 {
		t.Fatalf("leavers = %d, want 8", sum.Leavers)
	}
	if sum.Voluntary != 5 || sum.Involuntary != 3 {
		t.Errorf("voluntary/involuntary = %d/%d, want 5/3 — retirement is voluntary and "+
			"contract_end is not", sum.Voluntary, sum.Involuntary)
	}
	if sum.Regretted != 3 || sum.NonRegretted != 2 || sum.RegrettedUnknown != 3 {
		t.Errorf("regretted/non/unknown = %d/%d/%d, want 3/2/3",
			sum.Regretted, sum.NonRegretted, sum.RegrettedUnknown)
	}
	// ⚠ The load-bearing half: unknown must not have been swept into
	// non-regretted, which would flatter the number this metric exposes.
	if sum.NonRegretted > 2 {
		t.Errorf("non-regretted = %d — un-reviewed exits are being counted as good riddance",
			sum.NonRegretted)
	}
	if sum.Regretted+sum.NonRegretted+sum.RegrettedUnknown != sum.Leavers {
		t.Errorf("the regretted split does not account for every leaver")
	}
	if sum.Voluntary+sum.Involuntary != sum.Leavers {
		t.Errorf("the voluntary split does not account for every leaver")
	}

	byType := map[string]int{}
	for _, g := range sum.ByTerminationType {
		byType[g.Key] = g.Count
	}
	if byType["resignation"] != 4 || byType["retirement"] != 1 || byType["layoff"] != 1 {
		t.Errorf("termination-type breakdown = %v", byType)
	}
}

// TestIntegration_Analytics_FirstYearAttritionAndCohorts — first-year loss is
// a different question from overall attrition and needs its own figure.
func TestIntegration_Analytics_FirstYearAttritionAndCohorts(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedAnFixture(t, env)
	caller := analyst(fx.ownerID)
	today := time.Now().Truncate(24 * time.Hour)

	for i := 0; i < 8; i++ {
		seedHire(t, env, fx, fmt.Sprintf("Veteran %d", i), today.AddDate(-5, 0, 0), nil)
	}
	// Left after 90 days.
	newJoiner := seedHire(t, env, fx, "Quick exit", today.AddDate(0, 0, -100), nil)
	seedExit(t, env, fx, newJoiner, "resignation", "", today.AddDate(0, 0, -10), "eligible")
	// Left after four years.
	veteran := seedHire(t, env, fx, "Long server", today.AddDate(-4, 0, 0), nil)
	seedExit(t, env, fx, veteran, "resignation", "", today.AddDate(0, 0, -10), "eligible")

	if _, err := env.hrmAnalyticsSvc.RunSnapshotForOrg(ctx, fx.orgID, caller, today); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	sum, err := env.hrmAnalyticsSvc.Attrition(ctx, fx.orgID, caller, today.AddDate(0, -1, 0), today)
	if err != nil {
		t.Fatalf("attrition: %v", err)
	}
	if sum.Leavers != 2 {
		t.Fatalf("leavers = %d, want 2", sum.Leavers)
	}
	if sum.FirstYearExits != 1 {
		t.Errorf("first-year exits = %d, want 1 — a four-year tenure is not a first-year loss",
			sum.FirstYearExits)
	}
	if sum.FirstYearRate == nil || sum.AttritionRate == nil {
		t.Fatal("rates missing")
	}
	if sum.FirstYearRate.GreaterThanOrEqual(*sum.AttritionRate) {
		t.Errorf("first-year rate %s is not below the overall rate %s, though only one of "+
			"two leavers went inside a year", sum.FirstYearRate, sum.AttritionRate)
	}

	cohorts, err := env.hrmAnalyticsSvc.Cohorts(ctx, fx.orgID, caller, today.AddDate(-6, 0, 0), today)
	if err != nil {
		t.Fatalf("cohorts: %v", err)
	}
	if len(cohorts) == 0 {
		t.Fatal("no cohorts returned")
	}
	for _, c := range cohorts {
		if c.CohortSize > 0 && c.Retention == nil {
			t.Errorf("cohort %s has %d hires but no retention figure",
				c.CohortMonth.Format("2006-01"), c.CohortSize)
		}
		if c.CohortMonth.Day() != 1 {
			t.Errorf("cohort month %s is not truncated to the first",
				c.CohortMonth.Format("2006-01-02"))
		}
	}
}

// ============================================================
// Claim 3 — DEI suppression, end to end
// ============================================================

// TestIntegration_Analytics_SuppressedGroupCannotBeDifferencedOut runs the
// disclosure rule against a real population.
func TestIntegration_Analytics_SuppressedGroupCannotBeDifferencedOut(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedAnFixture(t, env)
	caller := analyst(fx.ownerID)
	today := time.Now().Truncate(24 * time.Hour)
	hire := today.AddDate(-2, 0, 0)

	male, female, other := "male", "female", "other"
	for i := 0; i < 20; i++ {
		seedHire(t, env, fx, fmt.Sprintf("M%d", i), hire, &male)
	}
	for i := 0; i < 14; i++ {
		seedHire(t, env, fx, fmt.Sprintf("F%d", i), hire, &female)
	}
	// One person. At the default threshold of 5 this must never be reported.
	seedHire(t, env, fx, "Solo", hire, &other)

	dists, err := env.hrmAnalyticsSvc.Diversity(ctx, fx.orgID, caller, today.AddDate(0, -1, 0), today)
	if err != nil {
		t.Fatalf("diversity: %v", err)
	}
	d, ok := dists["headcount_by_gender"]
	if !ok {
		t.Fatal("headcount_by_gender missing")
	}

	if d.Total != nil {
		t.Errorf("total = %d was published alongside suppression — 35 minus the disclosed "+
			"groups recovers the hidden one", *d.Total)
	}
	if d.SuppressedGroups < 2 {
		t.Errorf("%d group(s) suppressed; with one hole and any other figure the count of 1 "+
			"is arithmetic", d.SuppressedGroups)
	}
	for _, g := range d.Groups {
		if g.Key == "other" && (!g.Suppressed || g.Count != nil) {
			t.Errorf("the group of one was disclosed: suppressed=%v count=%v", g.Suppressed, g.Count)
		}
	}

	// ⚠ The rule is NOT a permission and an owner does not escape it. This
	// caller holds every analytics key there is.
	sumDisclosed := 0
	for _, g := range d.Groups {
		if g.Count != nil {
			sumDisclosed += *g.Count
		}
	}
	if sumDisclosed == 35 {
		t.Error("the disclosed groups sum to the full population, so the suppressed group " +
			"is zero by subtraction — which discloses it exactly")
	}
}

// TestIntegration_Analytics_ManagerCannotSeeDEIOrCompensation pins the 00126
// grant: a manager holds view alone.
func TestIntegration_Analytics_ManagerCannotSeeDEIOrCompensation(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedAnFixture(t, env)
	today := time.Now().Truncate(24 * time.Hour)
	mgr := anManager(fx.ownerID)

	for _, c := range []struct {
		name string
		call func() error
	}{
		{"diversity", func() error {
			_, err := env.hrmAnalyticsSvc.Diversity(ctx, fx.orgID, mgr, today.AddDate(0, -1, 0), today)
			return err
		}},
		{"compensation", func() error {
			_, err := env.hrmAnalyticsSvc.Compensation(ctx, fx.orgID, mgr, today, analytics.GrainOrg)
			return err
		}},
		{"export", func() error {
			_, err := env.hrmAnalyticsSvc.ExportAttrition(ctx, fx.orgID, mgr, today.AddDate(0, -1, 0), today)
			return err
		}},
		{"run the snapshot job", func() error {
			_, err := env.hrmAnalyticsSvc.RunSnapshotForOrg(ctx, fx.orgID, mgr, today)
			return err
		}},
	} {
		if err := c.call(); !errors.Is(err, analytics.ErrAccessDenied) {
			t.Errorf("a manager reaching %s got %v, want ErrAccessDenied", c.name, err)
		}
	}

	// Headcount and attrition are theirs.
	if _, err := env.hrmAnalyticsSvc.Attrition(ctx, fx.orgID, mgr, today.AddDate(0, -1, 0), today); err != nil {
		t.Errorf("a manager reading attrition was refused: %v", err)
	}
}

// ============================================================
// Compensation
// ============================================================

// TestIntegration_Analytics_SmallGroupPayIsNeverWrittenDown — the percentiles
// are withheld at WRITE time, so there is no stored value for any later
// query, export or backup to expose.
func TestIntegration_Analytics_SmallGroupPayIsNeverWrittenDown(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedAnFixture(t, env)
	caller := analyst(fx.ownerID)
	today := time.Now().Truncate(24 * time.Hour)
	hire := today.AddDate(-2, 0, 0)

	var structureID string
	if err := env.db.QueryRow(ctx,
		`INSERT INTO hrm_salary_structures (org_id, name, grade_label, created_by)
		 VALUES ($1,'Standard','L3',$2) RETURNING id`, fx.orgID, fx.ownerID).Scan(&structureID); err != nil {
		t.Fatalf("seed structure: %v", err)
	}
	pay := func(emp string, amount int) {
		if _, err := env.db.Exec(ctx,
			`INSERT INTO hrm_employee_salary_records (org_id, employee_id, structure_id, basic_pay,
			     effective_date, change_reason, currency, created_by)
			 VALUES ($1,$2,$3,$4,$5,'joining','USD',$6)`,
			fx.orgID, emp, structureID, amount, hire, fx.ownerID); err != nil {
			t.Fatalf("seed salary: %v", err)
		}
	}

	// Three people — below the default threshold of 5.
	for i, amount := range []int{50000, 60000, 70000} {
		pay(seedHire(t, env, fx, fmt.Sprintf("Small %d", i), hire, nil), amount)
	}
	if _, err := env.hrmAnalyticsSvc.RunSnapshotForOrg(ctx, fx.orgID, caller, today); err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	// ⚠ Assert at the DATABASE. The claim is that nothing was written, not
	// that a read path filtered it.
	var stored *decimal.Decimal
	if err := env.db.QueryRow(ctx,
		`SELECT comp_median FROM hrm_headcount_snapshots
		  WHERE org_id=$1 AND snapshot_date=$2 AND dimension='org'`,
		fx.orgID, today).Scan(&stored); err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if stored != nil {
		t.Errorf("comp_median %s was written for a group of 3 — a small team's pay "+
			"distribution must never be recorded, because a stored value outlives every "+
			"read-path filter", stored)
	}

	bands, err := env.hrmAnalyticsSvc.Compensation(ctx, fx.orgID, caller, today, analytics.GrainOrg)
	if err != nil {
		t.Fatalf("compensation: %v", err)
	}
	if len(bands) != 1 || !bands[0].Suppressed || bands[0].Median != nil {
		t.Errorf("compensation band = %+v, want one suppressed band", bands)
	}

	// Grow the group past the threshold and the figures appear.
	for i, amount := range []int{55000, 65000, 75000} {
		pay(seedHire(t, env, fx, fmt.Sprintf("More %d", i), hire, nil), amount)
	}
	if _, err := env.hrmAnalyticsSvc.RunSnapshotForOrg(ctx, fx.orgID, caller, today); err != nil {
		t.Fatalf("snapshot 2: %v", err)
	}
	bands, err = env.hrmAnalyticsSvc.Compensation(ctx, fx.orgID, caller, today, analytics.GrainOrg)
	if err != nil {
		t.Fatalf("compensation 2: %v", err)
	}
	if len(bands) != 1 || bands[0].Suppressed || bands[0].Median == nil {
		t.Fatalf("a group of 6 is still suppressed: %+v", bands)
	}
	// Median of 50k,55k,60k,65k,70k,75k is 62,500 exactly.
	if !bands[0].Median.Equal(decimal.RequireFromString("62500.00")) {
		t.Errorf("median = %s, want 62500.00", bands[0].Median)
	}
}

// TestIntegration_Analytics_HeadcountStripsPayWithoutThePermission — the
// permission strip and the disclosure threshold are different mechanisms and
// neither substitutes for the other.
func TestIntegration_Analytics_HeadcountStripsPayWithoutThePermission(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedAnFixture(t, env)
	caller := analyst(fx.ownerID)
	today := time.Now().Truncate(24 * time.Hour)
	hire := today.AddDate(-2, 0, 0)

	var structureID string
	if err := env.db.QueryRow(ctx,
		`INSERT INTO hrm_salary_structures (org_id, name, grade_label, created_by)
		 VALUES ($1,'Standard','L3',$2) RETURNING id`, fx.orgID, fx.ownerID).Scan(&structureID); err != nil {
		t.Fatalf("seed structure: %v", err)
	}
	for i := 0; i < 8; i++ {
		emp := seedHire(t, env, fx, fmt.Sprintf("Staff %d", i), hire, nil)
		if _, err := env.db.Exec(ctx,
			`INSERT INTO hrm_employee_salary_records (org_id, employee_id, structure_id, basic_pay,
			     effective_date, change_reason, currency, created_by)
			 VALUES ($1,$2,$3,$4,$5,'joining','USD',$6)`,
			fx.orgID, emp, structureID, 50000+i*1000, hire, fx.ownerID); err != nil {
			t.Fatalf("seed salary: %v", err)
		}
	}
	if _, err := env.hrmAnalyticsSvc.RunSnapshotForOrg(ctx, fx.orgID, caller, today); err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	withPay, err := env.hrmAnalyticsSvc.Headcount(ctx, fx.orgID, caller, today, today, analytics.GrainOrg)
	if err != nil {
		t.Fatalf("headcount: %v", err)
	}
	if len(withPay) != 1 || withPay[0].CompMedian == nil {
		t.Fatalf("a caller with view_compensation got no median: %+v", withPay)
	}

	mgr := anManager(fx.ownerID)
	withoutPay, err := env.hrmAnalyticsSvc.Headcount(ctx, fx.orgID, mgr, today, today, analytics.GrainOrg)
	if err != nil {
		t.Fatalf("headcount as manager: %v", err)
	}
	if len(withoutPay) != 1 {
		t.Fatalf("expected one snapshot, got %d", len(withoutPay))
	}
	if withoutPay[0].CompMedian != nil || withoutPay[0].CompP25 != nil || withoutPay[0].CompCurrency != nil {
		t.Errorf("a caller without view_compensation received pay figures: median=%v p25=%v",
			withoutPay[0].CompMedian, withoutPay[0].CompP25)
	}
	if withoutPay[0].Headcount != 8 {
		t.Errorf("headcount = %d, want 8 — stripping pay must not remove the metric",
			withoutPay[0].Headcount)
	}
}

// ============================================================
// Export and definitions
// ============================================================

// TestIntegration_Analytics_ExportCarriesNoDemographicColumn — a row-level
// extract with gender on it is exactly what "DEI is aggregate-only" forbids:
// a spreadsheet the suppression rule cannot reach.
func TestIntegration_Analytics_ExportCarriesNoDemographicColumn(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedAnFixture(t, env)
	caller := analyst(fx.ownerID)
	today := time.Now().Truncate(24 * time.Hour)

	other := "other"
	emp := seedHire(t, env, fx, "Solo", today.AddDate(-2, 0, 0), &other)
	seedExit(t, env, fx, emp, "resignation", "", today.AddDate(0, 0, -5), "")
	if _, err := env.hrmAnalyticsSvc.RunSnapshotForOrg(ctx, fx.orgID, caller, today); err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	csv, err := env.hrmAnalyticsSvc.ExportAttrition(ctx, fx.orgID, caller, today.AddDate(0, -1, 0), today)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	lower := strings.ToLower(csv)
	for _, forbidden := range []string{"gender", "male", "female", "other"} {
		if strings.Contains(lower, forbidden) {
			t.Errorf("the export contains %q — a row-level extract with a demographic "+
				"attribute defeats aggregate-only suppression entirely\n%s", forbidden, csv)
		}
	}
	// The unknown rehire decision must say so rather than leaving a blank a
	// spreadsheet would read as false.
	if !strings.Contains(csv, "unknown") {
		t.Errorf("an un-reviewed exit exported without saying its regretted status is "+
			"unknown:\n%s", csv)
	}
	if lines := strings.Split(strings.TrimSpace(csv), "\n"); len(lines) != 2 {
		t.Errorf("export has %d lines, want a header plus one fact row", len(lines))
	}
}

// TestIntegration_Analytics_MetricDefinitionsAreDataNotFormulas — the
// definition names a computation from a closed vocabulary and states itself
// in words. Nothing is ever parsed.
func TestIntegration_Analytics_MetricDefinitionsAreDataNotFormulas(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedAnFixture(t, env)
	caller := analyst(fx.ownerID)

	m, err := env.hrmAnalyticsSvc.CreateMetric(ctx, fx.orgID, caller, analytics.CreateMetricRequest{
		MetricKey: "attrition_rate", Name: "Annualised attrition",
		Computation:      "attrition_rate",
		FormulaStatement: "leavers in period ÷ average headcount × 100",
	})
	if err != nil {
		t.Fatalf("create metric: %v", err)
	}
	if m.SuppressionThreshold != analytics.DefaultSuppressionThreshold {
		t.Errorf("default threshold = %d, want %d", m.SuppressionThreshold,
			analytics.DefaultSuppressionThreshold)
	}

	if _, err := env.hrmAnalyticsSvc.CreateMetric(ctx, fx.orgID, caller, analytics.CreateMetricRequest{
		MetricKey: "attrition_rate", Name: "Duplicate", Computation: "attrition_rate",
		FormulaStatement: "x",
	}); !errors.Is(err, analytics.ErrDuplicateMetric) {
		t.Errorf("a duplicate key returned %v, want ErrDuplicateMetric", err)
	}

	// ⚠ Predictive scoring is not in the vocabulary and cannot be added by
	// naming it.
	if _, err := env.hrmAnalyticsSvc.CreateMetric(ctx, fx.orgID, caller, analytics.CreateMetricRequest{
		MetricKey: "flight_risk", Name: "Flight risk score",
		Computation: "predictive_attrition", FormulaStatement: "a model",
	}); !errors.Is(err, analytics.ErrInvalidComputation) {
		t.Errorf("a predictive computation returned %v, want ErrInvalidComputation", err)
	}

	// A definition with no statement is refused: the statement is the only
	// thing a reader can check the named computation against.
	if _, err := env.hrmAnalyticsSvc.CreateMetric(ctx, fx.orgID, caller, analytics.CreateMetricRequest{
		MetricKey: "headcount", Name: "Headcount", Computation: "headcount",
		FormulaStatement: "   ",
	}); !errors.Is(err, analytics.ErrStatementRequired) {
		t.Errorf("a definition with no statement returned %v, want ErrStatementRequired", err)
	}
}

// TestIntegration_Analytics_SuppressionThresholdCannotBeLowered — somebody
// lowering a disclosure threshold must be TOLD they cannot, not quietly
// overruled and left believing the setting took.
func TestIntegration_Analytics_SuppressionThresholdCannotBeLowered(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedAnFixture(t, env)
	caller := analyst(fx.ownerID)

	one := 1
	if _, err := env.hrmAnalyticsSvc.CreateMetric(ctx, fx.orgID, caller, analytics.CreateMetricRequest{
		MetricKey: "dei_distribution", Name: "Diversity", Computation: "dei_distribution",
		FormulaStatement: "headcount by gender", SuppressionThreshold: &one,
	}); !errors.Is(err, analytics.ErrThresholdTooLow) {
		t.Fatalf("a threshold of 1 returned %v, want ErrThresholdTooLow", err)
	}

	three := 3
	m, err := env.hrmAnalyticsSvc.CreateMetric(ctx, fx.orgID, caller, analytics.CreateMetricRequest{
		MetricKey: "dei_distribution", Name: "Diversity", Computation: "dei_distribution",
		FormulaStatement: "headcount by gender", SuppressionThreshold: &three,
	})
	if err != nil {
		t.Fatalf("create metric: %v", err)
	}
	zero := 0
	if _, err := env.hrmAnalyticsSvc.UpdateMetric(ctx, fx.orgID, caller, m.ID,
		analytics.UpdateMetricRequest{SuppressionThreshold: &zero}); !errors.Is(err, analytics.ErrThresholdTooLow) {
		t.Errorf("lowering to 0 returned %v, want ErrThresholdTooLow", err)
	}

	// The org's own threshold is what the DEI view uses.
	today := time.Now().Truncate(24 * time.Hour)
	male, female := "male", "female"
	for i := 0; i < 10; i++ {
		seedHire(t, env, fx, fmt.Sprintf("M%d", i), today.AddDate(-1, 0, 0), &male)
	}
	for i := 0; i < 4; i++ {
		seedHire(t, env, fx, fmt.Sprintf("F%d", i), today.AddDate(-1, 0, 0), &female)
	}
	dists, err := env.hrmAnalyticsSvc.Diversity(ctx, fx.orgID, caller, today.AddDate(0, -1, 0), today)
	if err != nil {
		t.Fatalf("diversity: %v", err)
	}
	d := dists["headcount_by_gender"]
	if d.Threshold != 3 {
		t.Errorf("threshold = %d, want the org's configured 3", d.Threshold)
	}
	if d.SuppressedGroups != 0 {
		t.Errorf("a group of 4 was suppressed at the org's threshold of 3")
	}
}
