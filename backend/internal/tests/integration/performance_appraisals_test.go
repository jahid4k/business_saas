// backend/internal/tests/integration/performance_appraisals_test.go
// Phase 5B appraisal cycles against real Postgres — the properties a stub
// repository cannot demonstrate: that the phase-history append shares a
// transaction with the phase write, that publish genuinely freezes figures
// derived from still-mutable sources, that the ON DELETE rules preserve the
// rating snapshot, that the unique index holds under concurrency, and that
// the real form engine drives the phase preconditions end to end.
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/authz"
	hrmperformance "github.com/mridha/businesssaas/internal/hrm/performance"
	"github.com/mridha/businesssaas/internal/platform/forms"
)

// ============================================================
// Seeding
// ============================================================

// seedRatingScale builds a three-level scale, returned lowest-first so a test
// can move a rating up or down without re-reading the order.
func seedRatingScale(t *testing.T, env *testEnv, orgID, ownerID string) (*hrmperformance.RatingScale, []*hrmperformance.RatingLevel) {
	t.Helper()
	ctx := context.Background()

	scale, err := env.hrmPerformanceSvc.CreateScale(ctx, orgID, ownerID, hrmperformance.CreateScaleRequest{
		Name: "Scale " + uniqueSlug("s"), IsDefault: true,
	})
	if err != nil {
		t.Fatalf("create scale: %v", err)
	}

	specs := []struct {
		label string
		value string
	}{{"Below", "1"}, {"Meets", "3"}, {"Exceeds", "5"}}
	levels := make([]*hrmperformance.RatingLevel, 0, len(specs))
	for i, spec := range specs {
		order := i
		l, err := env.hrmPerformanceSvc.CreateLevel(ctx, orgID, scale.ID, hrmperformance.CreateLevelRequest{
			Label: spec.label, Value: perfDec(spec.value), DisplayOrder: &order,
		})
		if err != nil {
			t.Fatalf("create level %s: %v", spec.label, err)
		}
		levels = append(levels, l)
	}
	return scale, levels
}

// seedAppraisalForm builds a single-question weighted form. One scale
// question is enough: the arithmetic is proved in the forms tests, and what
// matters here is that a score exists and moves.
func seedAppraisalForm(t *testing.T, env *testEnv, orgID, ownerID, title string) *forms.Template {
	t.Helper()
	ctx := context.Background()

	tmpl, err := env.formsSvc.CreateTemplate(ctx, orgID, ownerID, forms.CreateTemplateRequest{
		Name: title + " " + uniqueSlug("t"), FormType: string(forms.FormTypeAppraisal),
	})
	if err != nil {
		t.Fatalf("create %s template: %v", title, err)
	}
	sec, err := env.formsSvc.CreateSection(ctx, orgID, tmpl.ID, forms.CreateSectionRequest{Title: title})
	if err != nil {
		t.Fatalf("create %s section: %v", title, err)
	}
	weight := formDec("100")
	if _, err := env.formsSvc.CreateQuestion(ctx, orgID, sec.ID, forms.CreateQuestionRequest{
		QuestionText: "Overall", QuestionType: string(forms.QuestionScale),
		ScaleMin: intPtr(1), ScaleMax: intPtr(5), Weight: &weight, IsRequired: true,
	}); err != nil {
		t.Fatalf("create %s question: %v", title, err)
	}
	return tmpl
}

// answerAndSubmit fills the single scale question and submits, which is what
// flips the phase precondition from blocked to satisfied.
func answerAndSubmit(t *testing.T, env *testEnv, orgID, instanceID, userID, value string) {
	t.Helper()
	ctx := context.Background()

	inst, err := env.formsSvc.GetInstance(ctx, orgID, instanceID)
	if err != nil {
		t.Fatalf("get form instance: %v", err)
	}
	if len(inst.Responses) == 0 {
		t.Fatal("form instance has no responses — instantiation did not materialise rows")
	}
	answer := formDec(value)
	if _, err := env.formsSvc.SaveAnswers(ctx, orgID, instanceID, userID, forms.SaveAnswersRequest{
		Answers: []forms.AnswerRequest{{ResponseID: inst.Responses[0].ID, AnswerNumber: &answer}},
	}); err != nil {
		t.Fatalf("save answers: %v", err)
	}
	if _, err := env.formsSvc.SubmitInstance(ctx, orgID, instanceID, userID); err != nil {
		t.Fatalf("submit form: %v", err)
	}
}

// appraisalFixture is one fully wired org: a rating scale, a goal cycle with
// one weighted goal already part-progressed, both forms, and an ACTIVE
// appraisal cycle joining them.
type appraisalFixture struct {
	orgID     string
	statusID  string
	ownerID   string
	empID     string
	scale     *hrmperformance.RatingScale
	levels    []*hrmperformance.RatingLevel
	goalCycle *hrmperformance.GoalCycle
	goalID    string
	cycle     *hrmperformance.AppraisalCycle
}

func seedAppraisalFixture(t *testing.T, env *testEnv) *appraisalFixture {
	t.Helper()
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)

	// The appraisee IS the org owner's platform user, so the self form gets a
	// real respondent and SaveAnswers has someone to attribute answers to.
	empID := seedEmployee(t, env, orgID, statusID, ownerID, ownerID, "Appraisee", nil)

	scale, levels := seedRatingScale(t, env, orgID, ownerID)
	goalCycle := seedGoalCycle(t, env, orgID, ownerID, "100")

	weight := perfDec("100")
	goal, err := env.hrmPerformanceSvc.CreateGoal(ctx, orgID, perfAdmin(ownerID), hrmperformance.CreateGoalRequest{
		CycleID: goalCycle.ID, EmployeeID: empID, Title: "Ship it",
		TargetValue: &[]decimal.Decimal{perfDec("100")}[0], Weight: &weight,
	})
	if err != nil {
		t.Fatalf("create goal: %v", err)
	}
	if _, err := env.hrmPerformanceSvc.CreateCheckin(ctx, orgID, goal.ID, perfAdmin(ownerID),
		hrmperformance.CreateCheckinRequest{CurrentValue: perfDec("40")}); err != nil {
		t.Fatalf("seed checkin: %v", err)
	}

	selfTmpl := seedAppraisalForm(t, env, orgID, ownerID, "Self")
	mgrTmpl := seedAppraisalForm(t, env, orgID, ownerID, "Manager")

	cycle, err := env.hrmPerformanceSvc.CreateAppraisalCycle(ctx, orgID, ownerID, hrmperformance.CreateAppraisalCycleRequest{
		Name: "Review " + uniqueSlug("a"), PeriodStart: "2030-01-01", PeriodEnd: "2030-12-31",
		GoalCycleID: &goalCycle.ID, RatingScaleID: scale.ID,
		SelfFormTemplateID: &selfTmpl.ID, ManagerFormTemplateID: &mgrTmpl.ID,
	})
	if err != nil {
		t.Fatalf("create appraisal cycle: %v", err)
	}
	if _, err := env.hrmPerformanceSvc.ActivateAppraisalCycle(ctx, orgID, cycle.ID); err != nil {
		t.Fatalf("activate appraisal cycle: %v", err)
	}

	return &appraisalFixture{
		orgID: orgID, statusID: statusID, ownerID: ownerID, empID: empID,
		scale: scale, levels: levels, goalCycle: goalCycle, goalID: goal.ID, cycle: cycle,
	}
}

// ============================================================
// The full lifecycle against the real form engine
// ============================================================

// TestIntegration_Appraisals_FullLifecycle walks draft → acknowledged with a
// real form engine behind the preconditions. A stub engine can be told what
// to report; this proves the appraisal actually reads submitted_at off a row
// the forms service wrote, and that the self/manager respondent split lands
// on the right instances.
func TestIntegration_Appraisals_FullLifecycle(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedAppraisalFixture(t, env)
	caller := perfAdmin(fx.ownerID)

	a, err := env.hrmPerformanceSvc.InstantiateAppraisal(ctx, fx.orgID, fx.cycle.ID, fx.ownerID,
		hrmperformance.InstantiateAppraisalRequest{EmployeeID: fx.empID})
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	if a.SelfFormInstanceID == nil || a.ManagerFormInstanceID == nil {
		t.Fatal("expected both form instances to be created")
	}
	if *a.SelfFormInstanceID == *a.ManagerFormInstanceID {
		t.Fatal("self and manager forms must be distinct instances")
	}
	if a.Phase != hrmperformance.PhaseDraft {
		t.Errorf("expected draft, got %s", a.Phase)
	}

	// The subject is the employee on both forms; the respondent differs.
	selfInst, err := env.formsSvc.GetInstance(ctx, fx.orgID, *a.SelfFormInstanceID)
	if err != nil {
		t.Fatalf("get self instance: %v", err)
	}
	if selfInst.SubjectID != fx.empID {
		t.Errorf("self form subject = %s, want employee %s", selfInst.SubjectID, fx.empID)
	}
	if selfInst.RespondentUserID == nil || *selfInst.RespondentUserID != fx.ownerID {
		t.Errorf("self form respondent = %v, want the appraisee's user", selfInst.RespondentUserID)
	}
	mgrInst, err := env.formsSvc.GetInstance(ctx, fx.orgID, *a.ManagerFormInstanceID)
	if err != nil {
		t.Fatalf("get manager instance: %v", err)
	}
	// This employee has no manager, so the manager form exists with no
	// respondent rather than not existing — "nobody assigned yet" is not the
	// same as "no manager review in this cycle".
	if mgrInst.RespondentUserID != nil {
		t.Errorf("manager form respondent = %v, want nil for an employee with no manager", mgrInst.RespondentUserID)
	}

	// Goal attainment is live before publish: one weight-100 goal at 40/100.
	if a.GoalAttainment == nil || !a.GoalAttainment.Equal(perfDec("40")) {
		t.Errorf("expected live goal attainment 40, got %v", a.GoalAttainment)
	}

	a = advancePhase(t, env, fx.orgID, a.ID, caller, hrmperformance.PhaseSelfReview)

	// Blocked until the real form is submitted.
	if _, err := env.hrmPerformanceSvc.AdvancePhase(ctx, fx.orgID, a.ID, caller,
		hrmperformance.AdvancePhaseRequest{ToPhase: string(hrmperformance.PhaseManagerReview)}); !errors.Is(err, hrmperformance.ErrSelfReviewIncomplete) {
		t.Fatalf("expected ErrSelfReviewIncomplete, got %v", err)
	}
	answerAndSubmit(t, env, fx.orgID, *a.SelfFormInstanceID, fx.ownerID, "4")
	a = advancePhase(t, env, fx.orgID, a.ID, caller, hrmperformance.PhaseManagerReview)

	// The self score is now live off the submitted form: 4 on a 1-5 scale.
	if a.SelfScore == nil || !a.SelfScore.Equal(perfDec("75")) {
		t.Errorf("expected self score 75 (4 of 1..5), got %v", a.SelfScore)
	}

	if _, err := env.hrmPerformanceSvc.SetRating(ctx, fx.orgID, a.ID, caller,
		hrmperformance.SetRatingRequest{RatingLevelID: fx.levels[1].ID}); err != nil {
		t.Fatalf("set rating: %v", err)
	}

	answerAndSubmit(t, env, fx.orgID, *a.ManagerFormInstanceID, fx.ownerID, "5")
	a = advancePhase(t, env, fx.orgID, a.ID, caller, hrmperformance.PhaseCalibration)

	a, err = env.hrmPerformanceSvc.Calibrate(ctx, fx.orgID, a.ID, caller, hrmperformance.CalibrateRequest{
		RatingLevelID: fx.levels[2].ID, Note: "Exceeded on delivery across both halves",
	})
	if err != nil {
		t.Fatalf("calibrate: %v", err)
	}
	if a.FinalRatingLabel == nil || *a.FinalRatingLabel != "Exceeds" {
		t.Errorf("expected calibrated label Exceeds, got %v", a.FinalRatingLabel)
	}

	a, err = env.hrmPerformanceSvc.PublishAppraisal(ctx, fx.orgID, a.ID, caller)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if a.Phase != hrmperformance.PhasePublished {
		t.Fatalf("expected published, got %s", a.Phase)
	}

	// Every figure is now a stored column, not a live read.
	var selfScore, mgrScore, attainment *decimal.Decimal
	if err := env.db.QueryRow(ctx,
		`SELECT self_score, manager_score, goal_attainment FROM hrm_appraisals WHERE id = $1`, a.ID,
	).Scan(&selfScore, &mgrScore, &attainment); err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if selfScore == nil || !selfScore.Equal(perfDec("75")) {
		t.Errorf("self_score not snapshotted: %v", selfScore)
	}
	if mgrScore == nil || !mgrScore.Equal(perfDec("100")) {
		t.Errorf("manager_score not snapshotted: %v", mgrScore)
	}
	if attainment == nil || !attainment.Equal(perfDec("40")) {
		t.Errorf("goal_attainment not snapshotted: %v", attainment)
	}

	// Acknowledged is the one move left, and it is terminal.
	a = advancePhase(t, env, fx.orgID, a.ID, caller, hrmperformance.PhaseAcknowledged)
	if len(a.AllowedTransitions) != 0 {
		t.Errorf("acknowledged should be terminal, got transitions %v", a.AllowedTransitions)
	}
	var ackedAt *string
	if err := env.db.QueryRow(ctx,
		`SELECT acknowledged_at::text FROM hrm_appraisals WHERE id = $1`, a.ID).Scan(&ackedAt); err != nil {
		t.Fatalf("read acknowledged_at: %v", err)
	}
	if ackedAt == nil {
		t.Error("acknowledged_at was not stamped by the phase write")
	}
}

func advancePhase(t *testing.T, env *testEnv, orgID, ref string, caller hrmperformance.Caller, to hrmperformance.Phase) *hrmperformance.AppraisalDetail {
	t.Helper()
	a, err := env.hrmPerformanceSvc.AdvancePhase(context.Background(), orgID, ref, caller,
		hrmperformance.AdvancePhaseRequest{ToPhase: string(to)})
	if err != nil {
		t.Fatalf("advance to %s: %v", to, err)
	}
	return a
}

// ============================================================
// Publish immutability against a still-mutable source
// ============================================================

// TestIntegration_Appraisals_PublishFreezesAgainstLaterGoalProgress is the
// point of snapshotting rather than recomputing. Phase 5A goals stay editable
// after an appraisal publishes; if the published record recomputed its
// figures, a later check-in would silently rewrite history.
func TestIntegration_Appraisals_PublishFreezesAgainstLaterGoalProgress(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedAppraisalFixture(t, env)
	caller := perfAdmin(fx.ownerID)

	a := publishedAppraisal(t, env, fx)
	frozen := *a.GoalAttainment

	// The goal moves from 40 to 90 AFTER publication.
	if _, err := env.hrmPerformanceSvc.CreateCheckin(ctx, fx.orgID, fx.goalID, caller,
		hrmperformance.CreateCheckinRequest{CurrentValue: perfDec("90")}); err != nil {
		t.Fatalf("post-publish checkin: %v", err)
	}

	reread, err := env.hrmPerformanceSvc.GetAppraisal(ctx, fx.orgID, a.ID, caller)
	if err != nil {
		t.Fatalf("re-read appraisal: %v", err)
	}
	if reread.GoalAttainment == nil || !reread.GoalAttainment.Equal(frozen) {
		t.Errorf("published appraisal recomputed its goal attainment: was %s, now %v — the snapshot is not load-bearing",
			frozen, reread.GoalAttainment)
	}

	// And nothing further can change it.
	if _, err := env.hrmPerformanceSvc.Calibrate(ctx, fx.orgID, a.ID, caller, hrmperformance.CalibrateRequest{
		RatingLevelID: fx.levels[0].ID, Note: "second thoughts",
	}); !errors.Is(err, hrmperformance.ErrAppraisalPublished) {
		t.Errorf("expected ErrAppraisalPublished on post-publish calibrate, got %v", err)
	}
}

// publishedAppraisal drives one appraisal to published, forms and all.
func publishedAppraisal(t *testing.T, env *testEnv, fx *appraisalFixture) *hrmperformance.AppraisalDetail {
	t.Helper()
	ctx := context.Background()
	caller := perfAdmin(fx.ownerID)

	a, err := env.hrmPerformanceSvc.InstantiateAppraisal(ctx, fx.orgID, fx.cycle.ID, fx.ownerID,
		hrmperformance.InstantiateAppraisalRequest{EmployeeID: fx.empID})
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	a = advancePhase(t, env, fx.orgID, a.ID, caller, hrmperformance.PhaseSelfReview)
	answerAndSubmit(t, env, fx.orgID, *a.SelfFormInstanceID, fx.ownerID, "4")
	a = advancePhase(t, env, fx.orgID, a.ID, caller, hrmperformance.PhaseManagerReview)
	if _, err := env.hrmPerformanceSvc.SetRating(ctx, fx.orgID, a.ID, caller,
		hrmperformance.SetRatingRequest{RatingLevelID: fx.levels[1].ID}); err != nil {
		t.Fatalf("set rating: %v", err)
	}
	answerAndSubmit(t, env, fx.orgID, *a.ManagerFormInstanceID, fx.ownerID, "5")
	a = advancePhase(t, env, fx.orgID, a.ID, caller, hrmperformance.PhaseCalibration)
	a, err = env.hrmPerformanceSvc.PublishAppraisal(ctx, fx.orgID, a.ID, caller)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	return a
}

// ============================================================
// The audit trail is transactional, not best-effort
// ============================================================

// TestIntegration_Appraisals_PhaseHistoryIsTransactional proves the history
// row and the phase write land together. A rejected transition must leave no
// trace at all — a history row for a move that did not happen is worse than
// no history, because it reads as fact.
func TestIntegration_Appraisals_PhaseHistoryIsTransactional(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedAppraisalFixture(t, env)
	caller := perfAdmin(fx.ownerID)

	a, err := env.hrmPerformanceSvc.InstantiateAppraisal(ctx, fx.orgID, fx.cycle.ID, fx.ownerID,
		hrmperformance.InstantiateAppraisalRequest{EmployeeID: fx.empID})
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}

	countHistory := func() int {
		var n int
		if err := env.db.QueryRow(ctx,
			`SELECT COUNT(*) FROM hrm_appraisal_phase_history WHERE appraisal_id = $1`, a.ID).Scan(&n); err != nil {
			t.Fatalf("count history: %v", err)
		}
		return n
	}

	if got := countHistory(); got != 0 {
		t.Errorf("instantiation should append no history, got %d rows", got)
	}

	advancePhase(t, env, fx.orgID, a.ID, caller, hrmperformance.PhaseSelfReview)
	if got := countHistory(); got != 1 {
		t.Fatalf("expected 1 history row after one transition, got %d", got)
	}

	// A rejected precondition rolls the whole thing back.
	if _, err := env.hrmPerformanceSvc.AdvancePhase(ctx, fx.orgID, a.ID, caller,
		hrmperformance.AdvancePhaseRequest{ToPhase: string(hrmperformance.PhaseManagerReview)}); err == nil {
		t.Fatal("expected the unsubmitted self form to block the transition")
	}
	if got := countHistory(); got != 1 {
		t.Errorf("a rejected transition wrote history: %d rows", got)
	}

	var phase string
	if err := env.db.QueryRow(ctx, `SELECT phase FROM hrm_appraisals WHERE id = $1`, a.ID).Scan(&phase); err != nil {
		t.Fatalf("read phase: %v", err)
	}
	if phase != string(hrmperformance.PhaseSelfReview) {
		t.Errorf("phase advanced despite rejection: %s", phase)
	}

	// Calibration records BOTH sides of a rating override plus the actor —
	// this is the record the build plan requires.
	answerAndSubmit(t, env, fx.orgID, *a.SelfFormInstanceID, fx.ownerID, "4")
	advancePhase(t, env, fx.orgID, a.ID, caller, hrmperformance.PhaseManagerReview)
	if _, err := env.hrmPerformanceSvc.SetRating(ctx, fx.orgID, a.ID, caller,
		hrmperformance.SetRatingRequest{RatingLevelID: fx.levels[1].ID}); err != nil {
		t.Fatalf("set rating: %v", err)
	}
	answerAndSubmit(t, env, fx.orgID, *a.ManagerFormInstanceID, fx.ownerID, "5")
	advancePhase(t, env, fx.orgID, a.ID, caller, hrmperformance.PhaseCalibration)
	if _, err := env.hrmPerformanceSvc.Calibrate(ctx, fx.orgID, a.ID, caller, hrmperformance.CalibrateRequest{
		RatingLevelID: fx.levels[2].ID, Note: "Moderation raised this one",
	}); err != nil {
		t.Fatalf("calibrate: %v", err)
	}

	var fromLabel, toLabel, note, changedBy *string
	if err := env.db.QueryRow(ctx,
		`SELECT from_rating_label, to_rating_label, note, changed_by::text
		   FROM hrm_appraisal_phase_history
		  WHERE appraisal_id = $1 AND note IS NOT NULL
		  ORDER BY changed_at DESC LIMIT 1`, a.ID,
	).Scan(&fromLabel, &toLabel, &note, &changedBy); err != nil {
		t.Fatalf("read calibration history: %v", err)
	}
	if fromLabel == nil || *fromLabel != "Meets" {
		t.Errorf("calibration lost the prior rating: %v", fromLabel)
	}
	if toLabel == nil || *toLabel != "Exceeds" {
		t.Errorf("calibration did not record the new rating: %v", toLabel)
	}
	if note == nil || *note == "" {
		t.Error("calibration note was not persisted")
	}
	if changedBy == nil || *changedBy != fx.ownerID {
		t.Errorf("changed_by = %v, want %s", changedBy, fx.ownerID)
	}
}

// ============================================================
// ON DELETE behaviour on the rating FK
// ============================================================

// TestIntegration_Appraisals_LevelDeleteKeepsRatingSnapshot pins why the
// appraisal stores the label and value alongside the FK. Deleting the level
// nulls the FK by design; if that were the only record of the rating, every
// published appraisal referencing it would read as unrated.
func TestIntegration_Appraisals_LevelDeleteKeepsRatingSnapshot(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedAppraisalFixture(t, env)

	a := publishedAppraisal(t, env, fx)
	if a.FinalRatingLevelID == nil {
		t.Fatal("expected a rating on the published appraisal")
	}

	// Deleted directly: the service refuses to delete a level in use, and the
	// question here is what the SCHEMA does if one ever goes away.
	if _, err := env.db.Exec(ctx, `DELETE FROM hrm_rating_scale_levels WHERE id = $1`, *a.FinalRatingLevelID); err != nil {
		t.Fatalf("delete level: %v", err)
	}

	var levelID, label *string
	var value *decimal.Decimal
	if err := env.db.QueryRow(ctx,
		`SELECT final_rating_level_id::text, final_rating_label, final_rating_value
		   FROM hrm_appraisals WHERE id = $1`, a.ID,
	).Scan(&levelID, &label, &value); err != nil {
		t.Fatalf("re-read appraisal: %v", err)
	}
	if levelID != nil {
		t.Errorf("expected final_rating_level_id to be nulled, got %v", *levelID)
	}
	// publishedAppraisal rates at the middle level, "Meets" / 3.
	if label == nil || *label != "Meets" {
		t.Errorf("the rating label did not survive the level delete: %s", derefStr(label))
	}
	if value == nil || !value.Equal(perfDec("3")) {
		t.Errorf("the rating value did not survive the level delete: %v", value)
	}
}

func derefStr(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

// TestIntegration_Appraisals_ScaleInUseCannotBeDeleted proves the RESTRICT FK
// surfaces as a usable error rather than a raw 23503.
func TestIntegration_Appraisals_ScaleInUseCannotBeDeleted(t *testing.T) {
	env := newTestEnv(t)
	fx := seedAppraisalFixture(t, env)

	if err := env.hrmPerformanceSvc.DeleteScale(context.Background(), fx.orgID, fx.scale.ID); !errors.Is(err, hrmperformance.ErrScaleInUse) {
		t.Fatalf("expected ErrScaleInUse, got %v", err)
	}
}

// ============================================================
// The unique index, under real concurrency
// ============================================================

// TestIntegration_Appraisals_DuplicateInstantiationBlocked shows the service
// pre-check is the friendly path and uq_hrm_appr_cycle_employee is the actual
// guarantee. Concurrent callers all read "no existing appraisal" before any
// of them writes, so only the index can decide.
func TestIntegration_Appraisals_DuplicateInstantiationBlocked(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedAppraisalFixture(t, env)

	const attempts = 6
	var wg sync.WaitGroup
	errs := make([]error, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = env.hrmPerformanceSvc.InstantiateAppraisal(ctx, fx.orgID, fx.cycle.ID, fx.ownerID,
				hrmperformance.InstantiateAppraisalRequest{EmployeeID: fx.empID})
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
		t.Errorf("expected exactly one of %d concurrent instantiations to succeed, got %d", attempts, successes)
	}

	var n int
	if err := env.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrm_appraisals WHERE cycle_id = $1 AND employee_id = $2`,
		fx.cycle.ID, fx.empID).Scan(&n); err != nil {
		t.Fatalf("count appraisals: %v", err)
	}
	if n != 1 {
		t.Errorf("expected exactly 1 appraisal row, got %d — the unique index did not hold", n)
	}

	// The sequential path still returns the friendly sentinel.
	if _, err := env.hrmPerformanceSvc.InstantiateAppraisal(ctx, fx.orgID, fx.cycle.ID, fx.ownerID,
		hrmperformance.InstantiateAppraisalRequest{EmployeeID: fx.empID}); !errors.Is(err, hrmperformance.ErrAppraisalExists) {
		t.Errorf("expected ErrAppraisalExists on the sequential retry, got %v", err)
	}
}

// ============================================================
// The widened acknowledgement CHECK
// ============================================================

// TestIntegration_Appraisals_AcknowledgeableTypeAccepted proves migration
// 00086's CHECK widening actually took. Without it the Phase 6 pattern of
// routing a published appraisal through hrm_acknowledgements fails at insert
// time, which is the sort of thing that only shows up in production.
func TestIntegration_Appraisals_AcknowledgeableTypeAccepted(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedAppraisalFixture(t, env)
	a := publishedAppraisal(t, env, fx)

	var ackID string
	err := env.db.QueryRow(ctx,
		`INSERT INTO hrm_acknowledgements
		    (org_id, employee_id, acknowledgeable_type, acknowledgeable_id, entity_title, requested_by)
		 VALUES ($1, $2, 'appraisal', $3, 'FY30 Review', $4)
		 RETURNING id`,
		fx.orgID, fx.empID, a.ID, fx.ownerID,
	).Scan(&ackID)
	if err != nil {
		t.Fatalf("the acknowledgeable_type CHECK rejected 'appraisal': %v", err)
	}

	// The negative half: an unrelated value must still be refused, so the
	// widening did not turn the CHECK into a no-op.
	if _, err := env.db.Exec(ctx,
		`INSERT INTO hrm_acknowledgements
		    (org_id, employee_id, acknowledgeable_type, acknowledgeable_id, entity_title, requested_by)
		 VALUES ($1, $2, 'not_a_real_type', $3, 'Nope', $4)`,
		fx.orgID, fx.empID, a.ID, fx.ownerID,
	); err == nil {
		t.Error("the widened CHECK now accepts anything")
	}
}

// ============================================================
// Scope tiers over a real reporting tree
// ============================================================

// TestIntegration_Appraisals_ScopeTiers exercises scope.Predicate's recursive
// CTE on appraisals. Draft leakage is this module's named failure mode: an
// unpublished appraisal visible to the wrong reader is the whole risk.
func TestIntegration_Appraisals_ScopeTiers(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)

	managerEmpID := seedEmployee(t, env, orgID, statusID, ownerID, ownerID, "Manager", nil)
	reportEmpID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Report", &managerEmpID)
	strangerEmpID := seedEmployee(t, env, orgID, statusID, ownerID, "", "Stranger", nil)

	scale, _ := seedRatingScale(t, env, orgID, ownerID)
	selfTmpl := seedAppraisalForm(t, env, orgID, ownerID, "Self")
	cycle, err := env.hrmPerformanceSvc.CreateAppraisalCycle(ctx, orgID, ownerID, hrmperformance.CreateAppraisalCycleRequest{
		Name: "Scoped " + uniqueSlug("a"), PeriodStart: "2030-01-01", PeriodEnd: "2030-12-31",
		RatingScaleID: scale.ID, SelfFormTemplateID: &selfTmpl.ID,
	})
	if err != nil {
		t.Fatalf("create cycle: %v", err)
	}
	if _, err := env.hrmPerformanceSvc.ActivateAppraisalCycle(ctx, orgID, cycle.ID); err != nil {
		t.Fatalf("activate cycle: %v", err)
	}

	for _, e := range []string{managerEmpID, reportEmpID, strangerEmpID} {
		if _, err := env.hrmPerformanceSvc.InstantiateAppraisal(ctx, orgID, cycle.ID, ownerID,
			hrmperformance.InstantiateAppraisalRequest{EmployeeID: e}); err != nil {
			t.Fatalf("instantiate for %s: %v", e, err)
		}
	}

	cases := []struct {
		tier authz.Scope
		want int
	}{
		{authz.ScopeOwn, 1},  // the manager's own review
		{authz.ScopeTeam, 2}, // own + direct report
		{authz.ScopeAll, 3},  // the whole org
	}
	for _, tc := range cases {
		res, err := env.hrmPerformanceSvc.ListAppraisals(ctx, orgID, hrmperformance.AppraisalListFilter{
			CycleID: cycle.ID, Scope: tc.tier, CallerUserID: ownerID,
		})
		if err != nil {
			t.Fatalf("list at tier %v: %v", tc.tier, err)
		}
		if res.Total != tc.want {
			t.Errorf("tier %v: expected %d appraisals, got %d", tc.tier, tc.want, res.Total)
		}
	}

	// Fetch-by-id narrows the same way: a view_own caller cannot reach a
	// stranger's draft by guessing its id.
	strangers, err := env.hrmPerformanceSvc.ListAppraisals(ctx, orgID, hrmperformance.AppraisalListFilter{
		CycleID: cycle.ID, EmployeeID: strangerEmpID, Scope: authz.ScopeAll, CallerUserID: ownerID,
	})
	if err != nil || len(strangers.Appraisals) != 1 {
		t.Fatalf("locate stranger appraisal: %v (%d found)", err, len(strangers.Appraisals))
	}
	_, err = env.hrmPerformanceSvc.GetAppraisal(ctx, orgID, strangers.Appraisals[0].ID,
		hrmperformance.Caller{UserID: ownerID, Tier: authz.ScopeOwn})
	if !errors.Is(err, hrmperformance.ErrAppraisalAccessDenied) {
		t.Errorf("expected ErrAppraisalAccessDenied fetching a stranger's draft at view_own, got %v", err)
	}
}

// ============================================================
// Tenant isolation
// ============================================================

func TestIntegration_Appraisals_TenantIsolation(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fxA := seedAppraisalFixture(t, env)
	fxB := seedAppraisalFixture(t, env)

	a, err := env.hrmPerformanceSvc.InstantiateAppraisal(ctx, fxA.orgID, fxA.cycle.ID, fxA.ownerID,
		hrmperformance.InstantiateAppraisalRequest{EmployeeID: fxA.empID})
	if err != nil {
		t.Fatalf("instantiate in org A: %v", err)
	}

	if _, err := env.hrmPerformanceSvc.GetAppraisal(ctx, fxB.orgID, a.ID,
		perfAdmin(fxB.ownerID)); !errors.Is(err, hrmperformance.ErrAppraisalNotFound) {
		t.Errorf("org B reached org A's appraisal: %v", err)
	}
	if _, err := env.hrmPerformanceSvc.GetScale(ctx, fxB.orgID, fxA.scale.ID); !errors.Is(err, hrmperformance.ErrScaleNotFound) {
		t.Errorf("org B reached org A's rating scale: %v", err)
	}
	// A level from another org's scale cannot be used to rate — the cross-org
	// case of the wrong-scale check.
	if _, err := env.hrmPerformanceSvc.SetRating(ctx, fxA.orgID, a.ID, perfAdmin(fxA.ownerID),
		hrmperformance.SetRatingRequest{RatingLevelID: fxB.levels[0].ID}); err == nil {
		t.Error("a rating level from another org was accepted")
	}
}
