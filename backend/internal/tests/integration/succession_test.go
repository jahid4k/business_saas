// backend/internal/tests/integration/succession_test.go
// Phase 10B: succession planning.
//
// Two claims carry this slice:
//
//  1. Potential is assessed separately and is NEVER derived from
//     performance. Two people with the same appraisal rating and different
//     assessed potential must land in different boxes.
//  2. The subject's read path cannot reach the confidential material. That
//     is a property of the QUERY, not of a filter applied afterwards, so it
//     is asserted at the repository and structurally over the type.
//
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mridha/businesssaas/internal/auth"
	"github.com/mridha/businesssaas/internal/hrm/succession"
	"github.com/shopspring/decimal"
)

type succFixture struct {
	orgID    string
	statusID string
	ownerID  string
}

func reviewer(userID string) succession.Caller {
	return succession.Caller{
		UserID: userID, CanManage: true, CanViewConfidential: true, CanManagePlans: true,
	}
}

// subjectCaller holds only what a member holds: development_plan.view.
func subjectCaller(userID string) succession.Caller {
	return succession.Caller{UserID: userID}
}

// managerCaller mirrors the 00124 grant exactly: succession.view and
// development_plan.{view,manage}, but NOT view_confidential.
func managerCaller(userID string) succession.Caller {
	return succession.Caller{UserID: userID, CanManagePlans: true}
}

func seedSuccFixture(t *testing.T, env *testEnv) *succFixture {
	t.Helper()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	return &succFixture{orgID: orgID, statusID: statusID, ownerID: ownerID}
}

// succSignup creates a platform user to link an employee row to. Membership
// is not seeded: this package resolves the caller through
// hrm_employees.user_id, and the tests pass authority on the Caller value
// rather than through the route gate.
func succSignup(t *testing.T, env *testEnv, label string) string {
	t.Helper()
	u, err := env.authSvc.Signup(context.Background(), auth.SignupRequest{
		Email: uniqueEmail(label), Password: "SubjectPass123!",
	})
	if err != nil {
		t.Fatalf("signup %s: %v", label, err)
	}
	t.Cleanup(func() { cleanupUser(t, env, u.ID) })
	return u.ID
}

func seedPosition(t *testing.T, env *testEnv, orgID, ownerID, title string) string {
	t.Helper()
	var id string
	if err := env.db.QueryRow(context.Background(),
		`INSERT INTO hrm_positions (org_id, title, created_by) VALUES ($1,$2,$3) RETURNING id`,
		orgID, title, ownerID).Scan(&id); err != nil {
		t.Fatalf("seed position %q: %v", title, err)
	}
	return id
}

// seedRatingScale creates a 5-point active default scale, so a rating of 4.5
// bands high and 2.0 bands low against a real maximum.
func seedSuccRatingScale(t *testing.T, env *testEnv, orgID, ownerID string, max int) string {
	t.Helper()
	ctx := context.Background()
	var scaleID string
	if err := env.db.QueryRow(ctx,
		`INSERT INTO hrm_rating_scales (org_id, name, is_default, is_active, created_by)
		 VALUES ($1,'Standard',TRUE,TRUE,$2) RETURNING id`, orgID, ownerID).Scan(&scaleID); err != nil {
		t.Fatalf("seed rating scale: %v", err)
	}
	for i := 1; i <= max; i++ {
		if _, err := env.db.Exec(ctx,
			`INSERT INTO hrm_rating_scale_levels (scale_id, label, value, display_order)
			 VALUES ($1,$2,$3,$4)`, scaleID, fmt.Sprintf("Level %d", i), i, i); err != nil {
			t.Fatalf("seed rating level %d: %v", i, err)
		}
	}
	return scaleID
}

// seedPublishedAppraisal records a published final rating.
func seedPublishedAppraisal(t *testing.T, env *testEnv, orgID, ownerID, scaleID, employeeID string, rating float64, publishedAt time.Time) {
	t.Helper()
	ctx := context.Background()
	var cycleID string
	if err := env.db.QueryRow(ctx,
		`INSERT INTO hrm_appraisal_cycles (org_id, name, period_start, period_end, rating_scale_id, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		orgID, fmt.Sprintf("Cycle %s-%d", publishedAt.Format("2006-01-02"), time.Now().UnixNano()),
		publishedAt.AddDate(-1, 0, 0), publishedAt, scaleID, ownerID).Scan(&cycleID); err != nil {
		t.Fatalf("seed appraisal cycle: %v", err)
	}
	if _, err := env.db.Exec(ctx,
		`INSERT INTO hrm_appraisals (org_id, cycle_id, employee_id, phase, final_rating_value,
		    published_at, created_by)
		 VALUES ($1,$2,$3,'published',$4,$5,$6)`,
		orgID, cycleID, employeeID, decimal.NewFromFloat(rating), publishedAt, ownerID); err != nil {
		t.Fatalf("seed appraisal: %v", err)
	}
}

// ============================================================
// The claim: potential is not derived from performance
// ============================================================

// TestIntegration_Succession_SameRatingDifferentPotentialDifferentBox is the
// plan's stated verification. Two employees with the SAME published appraisal
// rating — so the same derived performance band — and different assessed
// potential must land in different boxes. If they did not, the 9-box would be
// the appraisal rating drawn twice at considerable expense.
func TestIntegration_Succession_SameRatingDifferentPotentialDifferentBox(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedSuccFixture(t, env)
	caller := reviewer(fx.ownerID)
	scale := seedSuccRatingScale(t, env, fx.orgID, fx.ownerID, 5)

	steady := seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, "", "Steady", nil)
	rising := seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, "", "Rising", nil)

	// Identical performance evidence.
	published := time.Now().AddDate(0, -1, 0)
	seedPublishedAppraisal(t, env, fx.orgID, fx.ownerID, scale, steady, 4.5, published)
	seedPublishedAppraisal(t, env, fx.orgID, fx.ownerID, scale, rising, 4.5, published)

	// Performance band deliberately left empty so it is DERIVED from the
	// appraisal — the only derivation this package permits.
	asOf := time.Now().Format("2006-01-02")
	aSteady, err := env.hrmSuccessionSvc.RecordAssessment(ctx, fx.orgID, caller,
		succession.RecordAssessmentRequest{
			EmployeeID: steady, AsOfDate: asOf,
			PotentialBand: "low", PotentialRationale: "Deep expert, content in role, no appetite for scope change",
		})
	if err != nil {
		t.Fatalf("assess steady: %v", err)
	}
	aRising, err := env.hrmSuccessionSvc.RecordAssessment(ctx, fx.orgID, caller,
		succession.RecordAssessmentRequest{
			EmployeeID: rising, AsOfDate: asOf,
			PotentialBand: "high", PotentialRationale: "Led two cross-team programmes outside their remit this year",
		})
	if err != nil {
		t.Fatalf("assess rising: %v", err)
	}

	if aSteady.PerformanceBand != aRising.PerformanceBand {
		t.Fatalf("the same rating derived different performance bands (%s vs %s) — "+
			"the test cannot make its point", aSteady.PerformanceBand, aRising.PerformanceBand)
	}
	if aSteady.PerformanceBand != succession.BandHigh {
		t.Errorf("4.5 of 5 derived performance band %q, want high", aSteady.PerformanceBand)
	}

	boxSteady, _ := succession.Box(aSteady.PerformanceBand, aSteady.PotentialBand)
	boxRising, _ := succession.Box(aRising.PerformanceBand, aRising.PotentialBand)
	if boxSteady.Box == boxRising.Box {
		t.Fatalf("both employees landed in box %d despite different assessed potential — "+
			"potential has collapsed into performance", boxSteady.Box)
	}
	if boxSteady.Box != 3 || boxRising.Box != 9 {
		t.Errorf("boxes = %d (%s) and %d (%s), want 3 (Solid Performer) and 9 (Star)",
			boxSteady.Box, boxSteady.Label, boxRising.Box, boxRising.Label)
	}

	// And the grid places them in different cells.
	grid, err := env.hrmSuccessionSvc.NineBoxGrid(ctx, fx.orgID, caller, nil)
	if err != nil {
		t.Fatalf("grid: %v", err)
	}
	if len(grid) != 9 {
		t.Errorf("grid has %d cells, want all 9 — an empty box is a finding, not an omission", len(grid))
	}
	cellOf := map[string]int{}
	for _, cell := range grid {
		for _, id := range cell.EmployeeIDs {
			cellOf[id] = cell.Box
		}
	}
	if cellOf[steady] == cellOf[rising] {
		t.Errorf("the grid put both in cell %d", cellOf[steady])
	}
}

// TestIntegration_Succession_PotentialMustBeStated — a potential band with no
// rationale is the unexplainable score this phase forbids, wearing a label.
func TestIntegration_Succession_PotentialMustBeStated(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedSuccFixture(t, env)
	caller := reviewer(fx.ownerID)
	emp := seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, "", "Unstated", nil)

	_, err := env.hrmSuccessionSvc.RecordAssessment(ctx, fx.orgID, caller,
		succession.RecordAssessmentRequest{
			EmployeeID: emp, PerformanceBand: "high", PotentialBand: "high", PotentialRationale: "   ",
		})
	if !errors.Is(err, succession.ErrRationaleRequired) {
		t.Errorf("a blank rationale returned %v, want ErrRationaleRequired", err)
	}

	// An omitted potential band is a refusal, NOT a copy of the performance
	// band — the moment potential mirrors performance the grid collapses.
	_, err = env.hrmSuccessionSvc.RecordAssessment(ctx, fx.orgID, caller,
		succession.RecordAssessmentRequest{
			EmployeeID: emp, PerformanceBand: "high", PotentialBand: "", PotentialRationale: "anything",
		})
	if !errors.Is(err, succession.ErrInvalidBand) {
		t.Errorf("an omitted potential band returned %v, want ErrInvalidBand", err)
	}
}

// TestIntegration_Succession_UnratedEmployeeIsUnplaced — deriving a
// performance band with no published appraisal would band somebody low on no
// evidence, which is the corner of the grid that ends careers.
func TestIntegration_Succession_UnratedEmployeeIsUnplaced(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedSuccFixture(t, env)
	caller := reviewer(fx.ownerID)
	emp := seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, "", "Unrated", nil)

	_, err := env.hrmSuccessionSvc.RecordAssessment(ctx, fx.orgID, caller,
		succession.RecordAssessmentRequest{
			EmployeeID: emp, PotentialBand: "high", PotentialRationale: "New joiner, strong signals",
		})
	if !errors.Is(err, succession.ErrInvalidBand) {
		t.Errorf("assessing an unrated employee with no explicit performance band returned %v, "+
			"want a refusal — an unrated employee is unplaced, not a low performer", err)
	}

	// Stating the band explicitly is allowed: a human has taken responsibility.
	if _, err := env.hrmSuccessionSvc.RecordAssessment(ctx, fx.orgID, caller,
		succession.RecordAssessmentRequest{
			EmployeeID: emp, PerformanceBand: "medium", PotentialBand: "high",
			PotentialRationale: "New joiner, strong signals",
		}); err != nil {
		t.Errorf("an explicitly stated band was refused: %v", err)
	}
}

// ============================================================
// The claim: the subject's read path cannot reach the confidential material
// ============================================================

// TestIntegration_Succession_SubjectViewCannotCarryConfidentialData walks the
// SubjectView TYPE and fails if anything on it could hold an assessment, a
// nomination or a flight-risk signal.
//
// ⚠ This is a structural assertion on purpose. A behavioural test only shows
// that today's code does not populate those fields; this shows there is
// nowhere to populate. The plan requires the guarantee at the type and
// repository level rather than through a handler.
func TestIntegration_Succession_SubjectViewCannotCarryConfidentialData(t *testing.T) {
	forbiddenTypes := map[string]bool{
		"TalentAssessment": true, "Candidate": true, "Signal": true,
		"NineBox": true, "ReviewerView": true, "GridCell": true,
	}
	forbiddenWords := []string{
		"potential", "performance", "flightrisk", "flight_risk", "ninebox", "nine_box",
		"nomination", "readiness", "assessment", "successor", "candidate",
	}

	seen := map[reflect.Type]bool{}
	var walk func(tp reflect.Type, path string)
	walk = func(tp reflect.Type, path string) {
		for tp.Kind() == reflect.Ptr || tp.Kind() == reflect.Slice || tp.Kind() == reflect.Array {
			tp = tp.Elem()
		}
		if tp.Kind() != reflect.Struct || seen[tp] {
			return
		}
		seen[tp] = true
		if forbiddenTypes[tp.Name()] {
			t.Errorf("SubjectView reaches confidential type %s at %s", tp.Name(), path)
			return
		}
		for i := 0; i < tp.NumField(); i++ {
			f := tp.Field(i)
			lower := strings.ToLower(f.Name + " " + f.Tag.Get("json"))
			for _, w := range forbiddenWords {
				if strings.Contains(lower, w) {
					t.Errorf("SubjectView reaches field %s.%s (json %q) at %s — the word %q "+
						"belongs to the reviewer's path only",
						tp.Name(), f.Name, f.Tag.Get("json"), path, w)
				}
			}
			walk(f.Type, path+"."+f.Name)
		}
	}
	walk(reflect.TypeOf(succession.SubjectView{}), "SubjectView")

	// The mirror assertion: ReviewerView MUST reach them, or the split is
	// hiding the material from everybody rather than from the subject.
	rv := reflect.TypeOf(succession.ReviewerView{})
	needed := map[string]bool{"Assessment": false, "NineBox": false, "FlightRisk": false, "Nominations": false}
	for i := 0; i < rv.NumField(); i++ {
		if _, ok := needed[rv.Field(i).Name]; ok {
			needed[rv.Field(i).Name] = true
		}
	}
	for name, found := range needed {
		if !found {
			t.Errorf("ReviewerView is missing %s — the confidential half must exist somewhere", name)
		}
	}
}

// TestIntegration_Succession_SubjectPathReturnsOnlyPlans is the behavioural
// half, asserted at the REPOSITORY. The employee has an assessment, a
// nomination and enough history to trip a flight-risk signal; the subject
// read must come back holding nothing but their own development plans.
func TestIntegration_Succession_SubjectPathReturnsOnlyPlans(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedSuccFixture(t, env)
	caller := reviewer(fx.ownerID)

	userID := succSignup(t, env, "succ-subject")
	emp := seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, userID, "Subject", nil)

	// Everything confidential that could possibly leak.
	if _, err := env.hrmSuccessionSvc.RecordAssessment(ctx, fx.orgID, caller,
		succession.RecordAssessmentRequest{
			EmployeeID: emp, PerformanceBand: "low", PotentialBand: "low",
			PotentialRationale: "Struggling; not a successor",
		}); err != nil {
		t.Fatalf("assess: %v", err)
	}
	pos := seedPosition(t, env, fx.orgID, fx.ownerID, "Head of Engineering")
	cp, err := env.hrmSuccessionSvc.CreateCriticalPosition(ctx, fx.orgID, caller,
		succession.CreateCriticalPositionRequest{PositionID: pos, CriticalityLevel: "mission_critical"})
	if err != nil {
		t.Fatalf("designate: %v", err)
	}
	if _, err := env.hrmSuccessionSvc.Nominate(ctx, fx.orgID, caller, cp.ID,
		succession.NominateRequest{EmployeeID: emp, Readiness: "emergency_cover"}); err != nil {
		t.Fatalf("nominate: %v", err)
	}

	// And one visible plan, shared (not draft).
	plan, err := env.hrmSuccessionSvc.CreatePlan(ctx, fx.orgID, caller, succession.CreatePlanRequest{
		EmployeeID: emp, Title: "Broaden delivery experience", Status: strPtr("active"),
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if _, err := env.hrmSuccessionSvc.AddPlanItem(ctx, fx.orgID, caller, plan.ID,
		succession.CreateItemRequest{Description: "Run the Q4 migration"}); err != nil {
		t.Fatalf("add item: %v", err)
	}
	// A draft plan the subject must NOT see: an author needs to work on a
	// plan before it is shown to the person it is about.
	if _, err := env.hrmSuccessionSvc.CreatePlan(ctx, fx.orgID, caller, succession.CreatePlanRequest{
		EmployeeID: emp, Title: "Unshared draft",
	}); err != nil {
		t.Fatalf("create draft: %v", err)
	}

	// The repository-level read the plan names.
	plans, err := env.hrmSuccessionRepo.SubjectPlans(ctx, fx.orgID, emp)
	if err != nil {
		t.Fatalf("SubjectPlans: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("SubjectPlans returned %d plans, want 1 — the draft must not be included", len(plans))
	}
	if plans[0].Title != "Broaden delivery experience" {
		t.Errorf("wrong plan returned: %q", plans[0].Title)
	}
	if len(plans[0].Items) != 1 {
		t.Errorf("plan came back with %d items, want 1", len(plans[0].Items))
	}

	// And through the service as the subject themselves, holding nothing but
	// development_plan.view.
	view, err := env.hrmSuccessionSvc.MyDevelopment(ctx, fx.orgID, subjectCaller(userID))
	if err != nil {
		t.Fatalf("MyDevelopment: %v", err)
	}
	if view.EmployeeID != emp {
		t.Errorf("MyDevelopment resolved employee %s, want %s", view.EmployeeID, emp)
	}
	if len(view.Plans) != 1 {
		t.Errorf("MyDevelopment returned %d plans, want 1", len(view.Plans))
	}
}

// TestIntegration_Succession_ManagerCannotReadConfidential pins the 00124
// grant. A manager holds succession.view and development_plan.manage but not
// view_confidential, because the subject's own manager is the reader a
// flight-risk assessment most needs protecting from.
func TestIntegration_Succession_ManagerCannotReadConfidential(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedSuccFixture(t, env)
	owner := reviewer(fx.ownerID)
	emp := seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, "", "Report", nil)
	pos := seedPosition(t, env, fx.orgID, fx.ownerID, "Director")
	cp, err := env.hrmSuccessionSvc.CreateCriticalPosition(ctx, fx.orgID, owner,
		succession.CreateCriticalPositionRequest{PositionID: pos})
	if err != nil {
		t.Fatalf("designate: %v", err)
	}
	if _, err := env.hrmSuccessionSvc.RecordAssessment(ctx, fx.orgID, owner,
		succession.RecordAssessmentRequest{
			EmployeeID: emp, PerformanceBand: "high", PotentialBand: "high",
			PotentialRationale: "Ready for a step up",
		}); err != nil {
		t.Fatalf("assess: %v", err)
	}

	mgr := managerCaller(fx.ownerID)
	for _, c := range []struct {
		name string
		call func() error
	}{
		{"nine-box grid", func() error {
			_, err := env.hrmSuccessionSvc.NineBoxGrid(ctx, fx.orgID, mgr, nil)
			return err
		}},
		{"candidate bench", func() error {
			_, err := env.hrmSuccessionSvc.ListCandidates(ctx, fx.orgID, mgr, cp.ID, true)
			return err
		}},
		{"employee review", func() error {
			_, err := env.hrmSuccessionSvc.ReviewEmployee(ctx, fx.orgID, mgr, emp)
			return err
		}},
	} {
		if err := c.call(); !errors.Is(err, succession.ErrAccessDenied) {
			t.Errorf("a manager reading the %s got %v, want ErrAccessDenied", c.name, err)
		}
	}

	// But the non-confidential half — which roles matter — is theirs.
	if _, err := env.hrmSuccessionSvc.ListCriticalPositions(ctx, fx.orgID, mgr, true); err != nil {
		t.Errorf("a manager listing critical positions was refused: %v", err)
	}
}

// TestIntegration_Succession_SubjectCannotReadAnotherPersonsPlan — the
// refusal is NOT-FOUND rather than forbidden, because confirming that a plan
// exists for a named person is itself something the caller was not shown.
func TestIntegration_Succession_SubjectCannotReadAnotherPersonsPlan(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedSuccFixture(t, env)
	owner := reviewer(fx.ownerID)

	userID := succSignup(t, env, "succ-nosy")
	mine := seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, userID, "Nosy", nil)
	theirs := seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, "", "Colleague", nil)

	ownPlan, err := env.hrmSuccessionSvc.CreatePlan(ctx, fx.orgID, owner, succession.CreatePlanRequest{
		EmployeeID: mine, Title: "My plan", Status: strPtr("active"),
	})
	if err != nil {
		t.Fatalf("create own plan: %v", err)
	}
	otherPlan, err := env.hrmSuccessionSvc.CreatePlan(ctx, fx.orgID, owner, succession.CreatePlanRequest{
		EmployeeID: theirs, Title: "Their plan", Status: strPtr("active"),
	})
	if err != nil {
		t.Fatalf("create other plan: %v", err)
	}

	subject := subjectCaller(userID)
	if _, err := env.hrmSuccessionSvc.GetPlan(ctx, fx.orgID, subject, ownPlan.ID); err != nil {
		t.Errorf("reading own plan was refused: %v", err)
	}
	if _, err := env.hrmSuccessionSvc.GetPlan(ctx, fx.orgID, subject, otherPlan.ID); !errors.Is(err, succession.ErrPlanNotFound) {
		t.Errorf("reading a colleague's plan returned %v, want ErrPlanNotFound "+
			"(not forbidden — that would confirm the plan exists)", err)
	}
	// Listing everybody's plans needs the manage authority.
	if _, err := env.hrmSuccessionSvc.ListPlans(ctx, fx.orgID, subject, ""); !errors.Is(err, succession.ErrAccessDenied) {
		t.Errorf("a member listing all plans returned %v, want ErrAccessDenied", err)
	}
}

// TestIntegration_Succession_SubjectMarksTheirOwnProgress — an employee who
// cannot record progress has a plan written ABOUT them rather than one they
// are working through. They may move status and nothing else.
func TestIntegration_Succession_SubjectMarksTheirOwnProgress(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedSuccFixture(t, env)
	owner := reviewer(fx.ownerID)

	userID := succSignup(t, env, "succ-doer")
	emp := seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, userID, "Doer", nil)

	plan, err := env.hrmSuccessionSvc.CreatePlan(ctx, fx.orgID, owner, succession.CreatePlanRequest{
		EmployeeID: emp, Title: "Grow", Status: strPtr("active"),
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	item, err := env.hrmSuccessionSvc.AddPlanItem(ctx, fx.orgID, owner, plan.ID,
		succession.CreateItemRequest{Description: "Shadow the on-call rotation"})
	if err != nil {
		t.Fatalf("add item: %v", err)
	}

	subject := subjectCaller(userID)
	done, err := env.hrmSuccessionSvc.UpdatePlanItem(ctx, fx.orgID, subject, item.ID,
		succession.UpdateItemRequest{
			Status: strPtr("completed"),
			// A rewritten description would let somebody change what they
			// were asked to do and then mark it done.
			Description: strPtr("Something much easier"),
		})
	if err != nil {
		t.Fatalf("subject completing their own item: %v", err)
	}
	if done.Status != "completed" || done.CompletedAt == nil {
		t.Errorf("status=%q completed_at=%v, want completed with a stamp", done.Status, done.CompletedAt)
	}
	if done.Description != "Shadow the on-call rotation" {
		t.Errorf("the subject rewrote the action to %q — status is the only field theirs to move",
			done.Description)
	}
}

// ============================================================
// Flight risk
// ============================================================

// TestIntegration_Succession_FlightRiskIsExplainedFactsWithNoScore — every
// signal that fires must name the evidence, and the review must carry no
// number anybody could act on without understanding it.
func TestIntegration_Succession_FlightRiskIsExplainedFactsWithNoScore(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedSuccFixture(t, env)
	caller := reviewer(fx.ownerID)
	scale := seedSuccRatingScale(t, env, fx.orgID, fx.ownerID, 5)

	// Hired long ago, never promoted.
	var emp string
	if err := env.db.QueryRow(ctx,
		`INSERT INTO hrm_employees (org_id, status_id, first_name, hire_date, created_by)
		 VALUES ($1,$2,'Stalled',CURRENT_DATE - INTERVAL '6 years',$3) RETURNING id`,
		fx.orgID, fx.statusID, fx.ownerID).Scan(&emp); err != nil {
		t.Fatalf("seed employee: %v", err)
	}

	// Paid below the minimum of their own grade band.
	var structureID string
	if err := env.db.QueryRow(ctx,
		`INSERT INTO hrm_salary_structures (org_id, name, grade_label, created_by)
		 VALUES ($1,'Engineering L3','L3',$2) RETURNING id`, fx.orgID, fx.ownerID).Scan(&structureID); err != nil {
		t.Fatalf("seed structure: %v", err)
	}
	if _, err := env.db.Exec(ctx,
		`INSERT INTO hrm_compensation_bands (org_id, grade_label, currency, min_amount, mid_amount, max_amount, effective_date, created_by)
		 VALUES ($1,'L3','USD',50000,75000,100000,CURRENT_DATE - INTERVAL '1 year',$2)`,
		fx.orgID, fx.ownerID); err != nil {
		t.Fatalf("seed band: %v", err)
	}
	if _, err := env.db.Exec(ctx,
		`INSERT INTO hrm_employee_salary_records (org_id, employee_id, structure_id, basic_pay, effective_date, change_reason, currency, created_by)
		 VALUES ($1,$2,$3,42000,CURRENT_DATE - INTERVAL '6 months','joining','USD',$4)`,
		fx.orgID, emp, structureID, fx.ownerID); err != nil {
		t.Fatalf("seed salary: %v", err)
	}

	// A declining appraisal trend.
	seedPublishedAppraisal(t, env, fx.orgID, fx.ownerID, scale, emp, 4.2, time.Now().AddDate(-1, 0, 0))
	seedPublishedAppraisal(t, env, fx.orgID, fx.ownerID, scale, emp, 3.1, time.Now().AddDate(0, -1, 0))

	view, err := env.hrmSuccessionSvc.ReviewEmployee(ctx, fx.orgID, caller, emp)
	if err != nil {
		t.Fatalf("review: %v", err)
	}

	got := map[succession.SignalType]string{}
	for _, s := range view.FlightRisk {
		if strings.TrimSpace(s.Detail) == "" {
			t.Errorf("signal %q fired with no explanation — that is the unexplainable score "+
				"this phase excludes, wearing a label", s.Type)
		}
		got[s.Type] = s.Detail
	}
	for _, want := range []succession.SignalType{
		succession.SignalNoPromotion, succession.SignalBelowBand, succession.SignalAppraisalDecline,
	} {
		if _, ok := got[want]; !ok {
			t.Errorf("signal %q did not fire; fired: %v", want, got)
		}
	}
	// The detail must carry the actual figures, or a reader cannot disagree
	// with the signal on the evidence.
	if d := got[succession.SignalBelowBand]; !strings.Contains(d, "42000.00") || !strings.Contains(d, "L3") {
		t.Errorf("below_band detail %q does not state the pay and the grade", d)
	}
	if d := got[succession.SignalAppraisalDecline]; !strings.Contains(d, "4.20") || !strings.Contains(d, "3.10") {
		t.Errorf("appraisal_decline detail %q does not state both ratings", d)
	}

	// ⚠ No score anywhere on the wire.
	rv := reflect.TypeOf(succession.ReviewerView{})
	for i := 0; i < rv.NumField(); i++ {
		n := strings.ToLower(rv.Field(i).Name)
		if strings.Contains(n, "score") || strings.Contains(n, "risklevel") || strings.Contains(n, "probability") {
			t.Errorf("ReviewerView carries %s — predictive scoring is deliberately excluded", rv.Field(i).Name)
		}
	}
}

// TestIntegration_Succession_ManagerChurnCountsSolidLinesOnly — a project
// lead rotating is not the relationship churn this signal is about, and
// counting matrix lines would make it fire on ordinary project staffing.
func TestIntegration_Succession_ManagerChurnCountsSolidLinesOnly(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedSuccFixture(t, env)
	caller := reviewer(fx.ownerID)

	emp := seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, "", "Churned", nil)
	for i := 0; i < 4; i++ {
		mgr := seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, "",
			fmt.Sprintf("Lead %d", i), nil)
		if _, err := env.db.Exec(ctx,
			`INSERT INTO hrm_reporting_relationships
			   (org_id, employee_id, manager_id, relationship_type, effective_from, created_by)
			 VALUES ($1,$2,$3,'dotted',CURRENT_DATE - INTERVAL '3 months',$4)`,
			fx.orgID, emp, mgr, fx.ownerID); err != nil {
			t.Fatalf("seed dotted line %d: %v", i, err)
		}
	}

	view, err := env.hrmSuccessionSvc.ReviewEmployee(ctx, fx.orgID, caller, emp)
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	for _, s := range view.FlightRisk {
		if s.Type == succession.SignalManagerChurn {
			t.Fatalf("four dotted-line changes fired manager churn: %q — matrix rotation is "+
				"not the relationship churn this signal is about", s.Detail)
		}
	}

	// Three SOLID changes do fire.
	for i := 0; i < 3; i++ {
		mgr := seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, "",
			fmt.Sprintf("Boss %d", i), nil)
		if _, err := env.db.Exec(ctx,
			`INSERT INTO hrm_reporting_relationships
			   (org_id, employee_id, manager_id, relationship_type, effective_from, effective_to, created_by)
			 VALUES ($1,$2,$3,'solid',CURRENT_DATE - INTERVAL '3 months',CURRENT_DATE,$4)`,
			fx.orgID, emp, mgr, fx.ownerID); err != nil {
			t.Fatalf("seed solid line %d: %v", i, err)
		}
	}
	view, err = env.hrmSuccessionSvc.ReviewEmployee(ctx, fx.orgID, caller, emp)
	if err != nil {
		t.Fatalf("review after solid lines: %v", err)
	}
	found := false
	for _, s := range view.FlightRisk {
		if s.Type == succession.SignalManagerChurn {
			found = true
			if !strings.Contains(s.Detail, "3 manager changes") {
				t.Errorf("churn detail %q does not state the count", s.Detail)
			}
		}
	}
	if !found {
		t.Error("three solid-line manager changes did not fire manager churn")
	}
}

// ============================================================
// Critical positions and the bench
// ============================================================

// TestIntegration_Succession_BenchDepthAndRetirement — an empty bench on a
// mission-critical role is the number this table exists to surface, and a
// retired designation keeps its history rather than being deleted.
func TestIntegration_Succession_BenchDepthAndRetirement(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedSuccFixture(t, env)
	caller := reviewer(fx.ownerID)

	pos := seedPosition(t, env, fx.orgID, fx.ownerID, "Chief Architect")
	cp, err := env.hrmSuccessionSvc.CreateCriticalPosition(ctx, fx.orgID, caller,
		succession.CreateCriticalPositionRequest{
			PositionID: pos, CriticalityLevel: "mission_critical", VacancyRisk: "high",
		})
	if err != nil {
		t.Fatalf("designate: %v", err)
	}
	if cp.ActiveCandidates != 0 {
		t.Errorf("a fresh designation reports %d candidates, want 0", cp.ActiveCandidates)
	}

	// The same position cannot be designated twice — two rows would split
	// the bench and make "who succeeds this role" unanswerable.
	if _, err := env.hrmSuccessionSvc.CreateCriticalPosition(ctx, fx.orgID, caller,
		succession.CreateCriticalPositionRequest{PositionID: pos}); !errors.Is(err, succession.ErrAlreadyDesignated) {
		t.Errorf("a duplicate designation returned %v, want ErrAlreadyDesignated", err)
	}

	emp := seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, "", "Successor", nil)
	cand, err := env.hrmSuccessionSvc.Nominate(ctx, fx.orgID, caller, cp.ID,
		succession.NominateRequest{EmployeeID: emp, Readiness: "ready_now"})
	if err != nil {
		t.Fatalf("nominate: %v", err)
	}
	if _, err := env.hrmSuccessionSvc.Nominate(ctx, fx.orgID, caller, cp.ID,
		succession.NominateRequest{EmployeeID: emp}); !errors.Is(err, succession.ErrAlreadyNominated) {
		t.Errorf("a duplicate nomination returned %v, want ErrAlreadyNominated", err)
	}

	list, err := env.hrmSuccessionSvc.ListCriticalPositions(ctx, fx.orgID, caller, true)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].ActiveCandidates != 1 {
		t.Fatalf("bench depth = %v, want one position with 1 candidate", list)
	}
	if list[0].PositionTitle != "Chief Architect" {
		t.Errorf("position title %q not resolved", list[0].PositionTitle)
	}

	// Withdrawing empties the bench but keeps the row.
	if _, err := env.hrmSuccessionSvc.WithdrawNomination(ctx, fx.orgID, caller, cand.ID,
		succession.WithdrawRequest{Reason: strPtr("Moved to another business unit")}); err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	all, err := env.hrmSuccessionSvc.ListCandidates(ctx, fx.orgID, caller, cp.ID, false)
	if err != nil {
		t.Fatalf("list candidates: %v", err)
	}
	if len(all) != 1 || all[0].Status != "withdrawn" || all[0].WithdrawnAt == nil {
		t.Errorf("withdrawal did not keep a stamped history row: %+v", all)
	}
	if _, err := env.hrmSuccessionSvc.WithdrawNomination(ctx, fx.orgID, caller, cand.ID,
		succession.WithdrawRequest{}); !errors.Is(err, succession.ErrAlreadyWithdrawn) {
		t.Errorf("withdrawing twice returned %v, want ErrAlreadyWithdrawn", err)
	}

	// Retiring the designation keeps it readable, and frees the position to
	// be designated again.
	inactive := false
	if _, err := env.hrmSuccessionSvc.UpdateCriticalPosition(ctx, fx.orgID, caller, cp.ID,
		succession.UpdateCriticalPositionRequest{IsActive: &inactive}); err != nil {
		t.Fatalf("retire: %v", err)
	}
	if active, _ := env.hrmSuccessionSvc.ListCriticalPositions(ctx, fx.orgID, caller, true); len(active) != 0 {
		t.Errorf("%d active designations after retiring, want 0", len(active))
	}
	if every, _ := env.hrmSuccessionSvc.ListCriticalPositions(ctx, fx.orgID, caller, false); len(every) != 1 {
		t.Errorf("%d designations in history, want 1 — retiring must not delete the record", len(every))
	}
	if _, err := env.hrmSuccessionSvc.CreateCriticalPosition(ctx, fx.orgID, caller,
		succession.CreateCriticalPositionRequest{PositionID: pos}); err != nil {
		t.Errorf("re-designating a retired position was refused: %v", err)
	}
}

// TestIntegration_Succession_NominationCannotBorrowAnotherPersonsPlan —
// pointing a nomination at somebody else's development plan would put one
// person's development behind another person's succession record, and would
// let the plan's owner infer a nomination they were never shown.
func TestIntegration_Succession_NominationCannotBorrowAnotherPersonsPlan(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedSuccFixture(t, env)
	caller := reviewer(fx.ownerID)

	nominee := seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, "", "Nominee", nil)
	other := seedEmployee(t, env, fx.orgID, fx.statusID, fx.ownerID, "", "Other", nil)
	pos := seedPosition(t, env, fx.orgID, fx.ownerID, "VP Ops")
	cp, err := env.hrmSuccessionSvc.CreateCriticalPosition(ctx, fx.orgID, caller,
		succession.CreateCriticalPositionRequest{PositionID: pos})
	if err != nil {
		t.Fatalf("designate: %v", err)
	}
	otherPlan, err := env.hrmSuccessionSvc.CreatePlan(ctx, fx.orgID, caller,
		succession.CreatePlanRequest{EmployeeID: other, Title: "Other's plan"})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}

	if _, err := env.hrmSuccessionSvc.Nominate(ctx, fx.orgID, caller, cp.ID,
		succession.NominateRequest{
			EmployeeID: nominee, DevelopmentPlanID: &otherPlan.ID,
		}); !errors.Is(err, succession.ErrPlanNotFound) {
		t.Errorf("nominating with somebody else's plan returned %v, want ErrPlanNotFound", err)
	}

	ownPlan, err := env.hrmSuccessionSvc.CreatePlan(ctx, fx.orgID, caller,
		succession.CreatePlanRequest{EmployeeID: nominee, Title: "Nominee's plan"})
	if err != nil {
		t.Fatalf("create own plan: %v", err)
	}
	cand, err := env.hrmSuccessionSvc.Nominate(ctx, fx.orgID, caller, cp.ID,
		succession.NominateRequest{EmployeeID: nominee, DevelopmentPlanID: &ownPlan.ID})
	if err != nil {
		t.Fatalf("nominating with the nominee's own plan: %v", err)
	}
	if cand.DevelopmentPlanID == nil || *cand.DevelopmentPlanID != ownPlan.ID {
		t.Errorf("the plan link was not stored: %v", cand.DevelopmentPlanID)
	}
}
