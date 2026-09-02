// backend/internal/tests/integration/learning_test.go
// Phase 6A Learning & Development against real Postgres — what a stub cannot
// prove: that a frozen grade survives the question being deleted (the reason
// grading is not re-derived at all), that the version pin holds against the
// real RESTRICT FK, that the partial unique index enforces one live enrollment
// under concurrency, and that the scope CTE resolves a real reporting tree.
// Gate: INTEGRATION=1
package integration

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/authz"
	hrmlearning "github.com/mridha/businesssaas/internal/hrm/learning"
	"github.com/mridha/businesssaas/internal/platform/forms"
)

func lrnAdmin(userID string) hrmlearning.Caller {
	return hrmlearning.Caller{
		UserID: userID, Tier: authz.ScopeAll, CanManage: true, CanGrade: true,
	}
}

// lrnManager holds manage but NOT grade — the grant migration 00093 gives the
// 'manager' role.
func lrnManager(userID string) hrmlearning.Caller {
	return hrmlearning.Caller{
		UserID: userID, Tier: authz.ScopeAll, CanManage: true, CanGrade: false,
	}
}

type learningFixture struct {
	orgID    string
	statusID string
	ownerID  string
	empID    string
	course   *hrmlearning.Course
	version  *hrmlearning.CourseVersion
	quiz     *hrmlearning.Lesson
	// questionIDs are the platform_form_questions rows the answer keys hang off.
	questionIDs []string
}

// seedQuizTemplate builds a two-question boolean quiz template and returns the
// template plus its question ids.
func seedQuizTemplate(t *testing.T, env *testEnv, orgID, userID string) (*forms.Template, []string) {
	t.Helper()
	ctx := context.Background()

	tmpl, err := env.formsSvc.CreateTemplate(ctx, orgID, userID, forms.CreateTemplateRequest{
		Name: "Quiz " + uniqueSlug("q"), FormType: string(forms.FormTypeAssessment),
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	sec, err := env.formsSvc.CreateSection(ctx, orgID, tmpl.ID, forms.CreateSectionRequest{Title: "Questions"})
	if err != nil {
		t.Fatalf("create section: %v", err)
	}

	ids := make([]string, 0, 2)
	for i, text := range []string{"Is phishing a threat?", "Should passwords be shared?"} {
		q, err := env.formsSvc.CreateQuestion(ctx, orgID, sec.ID, forms.CreateQuestionRequest{
			QuestionText: text, QuestionType: string(forms.QuestionBoolean),
			DisplayOrder: intPtr(i), IsRequired: true,
		})
		if err != nil {
			t.Fatalf("create question %d: %v", i, err)
		}
		ids = append(ids, q.ID)
	}
	return tmpl, ids
}

// seedLearningFixture builds a published course whose single required lesson
// is a two-question quiz with a 50% pass mark and both answer keys set.
func seedLearningFixture(t *testing.T, env *testEnv) *learningFixture {
	t.Helper()
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)

	// The learner IS the org owner's user, so the caller can act as them.
	empID := seedEmployee(t, env, orgID, statusID, ownerID, ownerID, "Learner", nil)
	tmpl, questionIDs := seedQuizTemplate(t, env, orgID, ownerID)

	c, err := env.hrmLearningSvc.CreateCourse(ctx, orgID, ownerID, hrmlearning.CreateCourseRequest{
		Title: "Security " + uniqueSlug("c"),
	})
	if err != nil {
		t.Fatalf("create course: %v", err)
	}
	v, err := env.hrmLearningSvc.CreateVersion(ctx, orgID, c.ID, ownerID, hrmlearning.CreateVersionRequest{})
	if err != nil {
		t.Fatalf("create version: %v", err)
	}
	m, err := env.hrmLearningSvc.CreateModule(ctx, orgID, v.ID, hrmlearning.CreateModuleRequest{Title: "Assessment"})
	if err != nil {
		t.Fatalf("create module: %v", err)
	}
	pass := perfDec("50")
	quiz, err := env.hrmLearningSvc.CreateLesson(ctx, orgID, m.ID, hrmlearning.CreateLessonRequest{
		Title: "Final quiz", LessonType: string(hrmlearning.LessonQuiz),
		FormTemplateID: &tmpl.ID, PassMark: &pass,
	})
	if err != nil {
		t.Fatalf("create quiz lesson: %v", err)
	}

	// q0 answer is true, q1 answer is false.
	for i, qid := range questionIDs {
		correct := i == 0
		if err := env.hrmLearningSvc.SetAnswerKey(ctx, orgID, quiz.ID, hrmlearning.SetAnswerKeyRequest{
			QuestionID: qid, CorrectBoolean: &correct,
		}); err != nil {
			t.Fatalf("set answer key %d: %v", i, err)
		}
	}

	published, err := env.hrmLearningSvc.PublishVersion(ctx, orgID, v.ID, ownerID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	return &learningFixture{
		orgID: orgID, statusID: statusID, ownerID: ownerID, empID: empID,
		course: c, version: published, quiz: quiz, questionIDs: questionIDs,
	}
}

// ============================================================
// The answer-key boundary, against the real query layer
// ============================================================

// TestIntegration_Learning_AttemptNeverLeaksTheAnswerKey is the headline
// security test. The unit tests prove the DTO cannot carry a correct answer;
// this proves the real query path does not fetch one either.
func TestIntegration_Learning_AttemptNeverLeaksTheAnswerKey(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedLearningFixture(t, env)

	e, err := env.hrmLearningSvc.Enroll(ctx, fx.orgID, lrnAdmin(fx.ownerID), hrmlearning.EnrollRequest{
		EmployeeID: fx.empID, CourseID: fx.course.ID,
	})
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}
	att, err := env.hrmLearningSvc.StartAttempt(ctx, fx.orgID, e.ID, fx.quiz.ID, lrnAdmin(fx.ownerID))
	if err != nil {
		t.Fatalf("start attempt: %v", err)
	}
	if len(att.Questions) != 2 {
		t.Fatalf("expected 2 questions served, got %d", len(att.Questions))
	}

	// The keys really exist in the DB, so this is not passing vacuously.
	var keyCount int
	if err := env.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrm_quiz_answer_keys WHERE org_id = $1`, fx.orgID).Scan(&keyCount); err != nil {
		t.Fatalf("count keys: %v", err)
	}
	if keyCount != 2 {
		t.Fatalf("expected 2 answer keys stored, got %d", keyCount)
	}

	blob := mustJSON(t, att)
	for _, banned := range []string{"correct_boolean", "correct_text", "correct_options", "explanation", "points"} {
		if containsStr(blob, banned) {
			t.Errorf("attempt payload leaked %q: %s", banned, blob)
		}
	}
}

// TestIntegration_Learning_GetAnswerKeysRequiresGrade proves the one endpoint
// that does return keys is gated on a permission 'manager' does not hold.
func TestIntegration_Learning_GetAnswerKeysRequiresGrade(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedLearningFixture(t, env)

	if _, err := env.hrmLearningSvc.GetAnswerKeys(ctx, fx.orgID, fx.quiz.ID,
		lrnManager(fx.ownerID)); !errors.Is(err, hrmlearning.ErrGradeDenied) {
		t.Fatalf("expected ErrGradeDenied for a manager, got %v", err)
	}

	keys, err := env.hrmLearningSvc.GetAnswerKeys(ctx, fx.orgID, fx.quiz.ID, lrnAdmin(fx.ownerID))
	if err != nil {
		t.Fatalf("admin with grade should read keys: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

// ============================================================
// Grading is frozen, because it CANNOT be re-derived
// ============================================================

// TestIntegration_Learning_GradeSurvivesQuestionDeletion is the test that
// justifies storing the grade rather than recomputing it.
//
// platform_form_responses.question_id is ON DELETE SET NULL, and
// hrm_quiz_answer_keys.question_id is ON DELETE CASCADE. So deleting a
// question destroys the key AND severs the response's link to it — a re-grade
// would silently score zero. The stored score must not move.
func TestIntegration_Learning_GradeSurvivesQuestionDeletion(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedLearningFixture(t, env)
	caller := lrnAdmin(fx.ownerID)

	e, err := env.hrmLearningSvc.Enroll(ctx, fx.orgID, caller, hrmlearning.EnrollRequest{
		EmployeeID: fx.empID, CourseID: fx.course.ID,
	})
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}
	att, err := env.hrmLearningSvc.StartAttempt(ctx, fx.orgID, e.ID, fx.quiz.ID, caller)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	// Answer both correctly → 100%.
	yes, no := true, false
	graded, err := env.hrmLearningSvc.SubmitAttempt(ctx, fx.orgID, att.ID, caller,
		hrmlearning.SubmitAttemptRequest{Answers: []forms.AnswerRequest{
			{ResponseID: att.Questions[0].ResponseID, AnswerBoolean: &yes},
			{ResponseID: att.Questions[1].ResponseID, AnswerBoolean: &no},
		}})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if graded.Score == nil || !graded.Score.Equal(perfDec("100")) {
		t.Fatalf("score = %v, want 100", graded.Score)
	}
	if graded.Passed == nil || !*graded.Passed {
		t.Fatalf("expected passed, got %v", graded.Passed)
	}

	// Now destroy a question. This cascades the key away and nulls the
	// response's question_id.
	if _, err := env.db.Exec(ctx,
		`DELETE FROM platform_form_questions WHERE id = $1`, fx.questionIDs[0]); err != nil {
		t.Fatalf("delete question: %v", err)
	}

	var keysLeft int
	if err := env.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrm_quiz_answer_keys WHERE question_id = $1`,
		fx.questionIDs[0]).Scan(&keysLeft); err != nil {
		t.Fatalf("count keys: %v", err)
	}
	if keysLeft != 0 {
		t.Errorf("expected the key to cascade away, %d left", keysLeft)
	}

	// The frozen grade is untouched — which is the whole point.
	var score, passMark *decimal.Decimal
	var passed *bool
	if err := env.db.QueryRow(ctx,
		`SELECT score, passed, pass_mark_snapshot FROM hrm_quiz_attempts WHERE id = $1`, att.ID,
	).Scan(&score, &passed, &passMark); err != nil {
		t.Fatalf("re-read attempt: %v", err)
	}
	if score == nil || !score.Equal(perfDec("100")) {
		t.Errorf("stored score changed after the question was deleted: %v — the grade is being re-derived", score)
	}
	if passed == nil || !*passed {
		t.Errorf("stored pass flag changed after the question was deleted: %v", passed)
	}
	if passMark == nil || !passMark.Equal(perfDec("50")) {
		t.Errorf("pass mark snapshot lost: %v", passMark)
	}
}

// TestIntegration_Learning_PassMarkSnapshotBeatsALaterEdit — raising the pass
// mark must not retroactively fail somebody who already passed.
func TestIntegration_Learning_PassMarkSnapshotBeatsALaterEdit(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedLearningFixture(t, env)
	caller := lrnAdmin(fx.ownerID)

	e, _ := env.hrmLearningSvc.Enroll(ctx, fx.orgID, caller, hrmlearning.EnrollRequest{
		EmployeeID: fx.empID, CourseID: fx.course.ID,
	})
	att, err := env.hrmLearningSvc.StartAttempt(ctx, fx.orgID, e.ID, fx.quiz.ID, caller)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	// Answer one of two correctly → 50%, exactly the pass mark.
	yes := true
	graded, err := env.hrmLearningSvc.SubmitAttempt(ctx, fx.orgID, att.ID, caller,
		hrmlearning.SubmitAttemptRequest{Answers: []forms.AnswerRequest{
			{ResponseID: att.Questions[0].ResponseID, AnswerBoolean: &yes},
			{ResponseID: att.Questions[1].ResponseID, AnswerBoolean: &yes},
		}})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if graded.Passed == nil || !*graded.Passed {
		t.Fatalf("50%% should meet a 50%% pass mark, got %v (score %v)", graded.Passed, graded.Score)
	}

	// Raise the bar directly on the lesson.
	if _, err := env.db.Exec(ctx,
		`UPDATE hrm_course_lessons SET pass_mark = 90 WHERE id = $1`, fx.quiz.ID); err != nil {
		t.Fatalf("raise pass mark: %v", err)
	}

	var passed *bool
	if err := env.db.QueryRow(ctx,
		`SELECT passed FROM hrm_quiz_attempts WHERE id = $1`, att.ID).Scan(&passed); err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if passed == nil || !*passed {
		t.Error("raising the pass mark retroactively failed a completed attempt")
	}
}

// ============================================================
// Version pinning against the real RESTRICT FK
// ============================================================

// TestIntegration_Learning_VersionPinHoldsAndRestricts proves both halves:
// a new publish leaves an existing enrollment on its old version, and the
// pinned version cannot be deleted out from under it.
func TestIntegration_Learning_VersionPinHoldsAndRestricts(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedLearningFixture(t, env)
	caller := lrnAdmin(fx.ownerID)

	e, err := env.hrmLearningSvc.Enroll(ctx, fx.orgID, caller, hrmlearning.EnrollRequest{
		EmployeeID: fx.empID, CourseID: fx.course.ID,
	})
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}
	if e.VersionID != fx.version.ID {
		t.Fatalf("pinned %s, want %s", e.VersionID, fx.version.ID)
	}

	// Publish v2 with an extra required lesson.
	v2, err := env.hrmLearningSvc.CreateVersion(ctx, fx.orgID, fx.course.ID, fx.ownerID,
		hrmlearning.CreateVersionRequest{CopyFromVersionID: &fx.version.ID})
	if err != nil {
		t.Fatalf("create v2: %v", err)
	}
	m2, err := env.hrmLearningSvc.CreateModule(ctx, fx.orgID, v2.ID, hrmlearning.CreateModuleRequest{
		Title: "New material",
	})
	if err != nil {
		t.Fatalf("v2 module: %v", err)
	}
	if _, err := env.hrmLearningSvc.CreateLesson(ctx, fx.orgID, m2.ID, hrmlearning.CreateLessonRequest{
		Title: "Extra reading", LessonType: string(hrmlearning.LessonText),
	}); err != nil {
		t.Fatalf("v2 lesson: %v", err)
	}
	if _, err := env.hrmLearningSvc.PublishVersion(ctx, fx.orgID, v2.ID, fx.ownerID); err != nil {
		t.Fatalf("publish v2: %v", err)
	}

	reread, err := env.hrmLearningSvc.GetEnrollment(ctx, fx.orgID, e.ID, caller)
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if reread.VersionID != fx.version.ID {
		t.Errorf("enrollment moved to %s after a new publish", reread.VersionID)
	}
	if reread.RequiredLessons != 1 {
		t.Errorf("pinned enrollment sees %d required lessons, want 1 — it is reading v2",
			reread.RequiredLessons)
	}

	// The RESTRICT FK refuses to let the pinned version be deleted.
	if _, err := env.db.Exec(ctx,
		`DELETE FROM hrm_course_versions WHERE id = $1`, fx.version.ID); err == nil {
		t.Error("deleting a version with enrollments was allowed — RESTRICT is not in force")
	}
	// And the service says so in words.
	if err := env.hrmLearningSvc.DeleteVersion(ctx, fx.orgID, fx.version.ID); !errors.Is(err, hrmlearning.ErrVersionInUse) {
		t.Errorf("expected ErrVersionInUse, got %v", err)
	}
}

// ============================================================
// One live enrollment, under real concurrency
// ============================================================

// TestIntegration_Learning_OneLiveEnrollmentUnderConcurrency shows the service
// pre-check is the friendly path and uq_hrm_enr_employee_course_live is the
// actual guarantee: concurrent callers all read "not enrolled" before any of
// them writes.
func TestIntegration_Learning_OneLiveEnrollmentUnderConcurrency(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fx := seedLearningFixture(t, env)

	const attempts = 6
	var wg sync.WaitGroup
	errs := make([]error, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = env.hrmLearningSvc.Enroll(ctx, fx.orgID, lrnAdmin(fx.ownerID),
				hrmlearning.EnrollRequest{EmployeeID: fx.empID, CourseID: fx.course.ID})
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
		t.Errorf("expected exactly one of %d concurrent enrollments to succeed, got %d", attempts, successes)
	}

	var n int
	if err := env.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM hrm_enrollments WHERE employee_id=$1 AND course_id=$2
		   AND status IN ('assigned','in_progress')`, fx.empID, fx.course.ID).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("expected exactly 1 live enrollment, got %d — the partial index did not hold", n)
	}
}

// ============================================================
// Scope tiers over a real reporting tree
// ============================================================

func TestIntegration_Learning_ScopeTiers(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)

	managerEmp := seedEmployee(t, env, orgID, statusID, ownerID, ownerID, "Manager", nil)
	reportEmp := seedEmployee(t, env, orgID, statusID, ownerID, "", "Report", &managerEmp)
	strangerEmp := seedEmployee(t, env, orgID, statusID, ownerID, "", "Stranger", nil)

	c, err := env.hrmLearningSvc.CreateCourse(ctx, orgID, ownerID, hrmlearning.CreateCourseRequest{
		Title: "Scoped " + uniqueSlug("c"),
	})
	if err != nil {
		t.Fatalf("create course: %v", err)
	}
	v, _ := env.hrmLearningSvc.CreateVersion(ctx, orgID, c.ID, ownerID, hrmlearning.CreateVersionRequest{})
	m, _ := env.hrmLearningSvc.CreateModule(ctx, orgID, v.ID, hrmlearning.CreateModuleRequest{Title: "M"})
	if _, err := env.hrmLearningSvc.CreateLesson(ctx, orgID, m.ID, hrmlearning.CreateLessonRequest{
		Title: "Read", LessonType: string(hrmlearning.LessonText),
	}); err != nil {
		t.Fatalf("create lesson: %v", err)
	}
	if _, err := env.hrmLearningSvc.PublishVersion(ctx, orgID, v.ID, ownerID); err != nil {
		t.Fatalf("publish: %v", err)
	}

	for _, emp := range []string{managerEmp, reportEmp, strangerEmp} {
		if _, err := env.hrmLearningSvc.Enroll(ctx, orgID, lrnAdmin(ownerID), hrmlearning.EnrollRequest{
			EmployeeID: emp, CourseID: c.ID,
		}); err != nil {
			t.Fatalf("enroll %s: %v", emp, err)
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
		caller := hrmlearning.Caller{UserID: ownerID, Tier: tc.tier}
		res, err := env.hrmLearningSvc.ListEnrollments(ctx, orgID, caller, hrmlearning.EnrollmentListFilter{
			CourseID: c.ID,
		})
		if err != nil {
			t.Fatalf("list at tier %v: %v", tc.tier, err)
		}
		if res.Total != tc.want {
			t.Errorf("tier %v: expected %d enrollments, got %d", tc.tier, tc.want, res.Total)
		}
	}

	// Fetch-by-id narrows the same way.
	strangers, err := env.hrmLearningSvc.ListEnrollments(ctx, orgID, lrnAdmin(ownerID),
		hrmlearning.EnrollmentListFilter{EmployeeID: strangerEmp})
	if err != nil || len(strangers.Enrollments) != 1 {
		t.Fatalf("locate stranger enrollment: %v (%d)", err, len(strangers.Enrollments))
	}
	own := hrmlearning.Caller{UserID: ownerID, Tier: authz.ScopeOwn}
	if _, err := env.hrmLearningSvc.GetEnrollment(ctx, orgID, strangers.Enrollments[0].ID, own); !errors.Is(err, hrmlearning.ErrAccessDenied) {
		t.Errorf("expected ErrAccessDenied at view_own, got %v", err)
	}
}

// ============================================================
// Completion and tenancy
// ============================================================

// TestIntegration_Learning_CompletionIsComputedNotStored walks a text lesson
// to completion and checks the enrollment flips, with the percentage derived
// from lesson progress rather than any stored counter.
func TestIntegration_Learning_CompletionIsComputed(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	orgID, statusID, ownerID := seedScopeTestOrg(t, env)
	empID := seedEmployee(t, env, orgID, statusID, ownerID, ownerID, "Learner", nil)

	c, _ := env.hrmLearningSvc.CreateCourse(ctx, orgID, ownerID, hrmlearning.CreateCourseRequest{
		Title: "Two lessons " + uniqueSlug("c"),
	})
	v, _ := env.hrmLearningSvc.CreateVersion(ctx, orgID, c.ID, ownerID, hrmlearning.CreateVersionRequest{})
	m, _ := env.hrmLearningSvc.CreateModule(ctx, orgID, v.ID, hrmlearning.CreateModuleRequest{Title: "M"})
	l1, _ := env.hrmLearningSvc.CreateLesson(ctx, orgID, m.ID, hrmlearning.CreateLessonRequest{
		Title: "One", LessonType: string(hrmlearning.LessonText),
	})
	l2, _ := env.hrmLearningSvc.CreateLesson(ctx, orgID, m.ID, hrmlearning.CreateLessonRequest{
		Title: "Two", LessonType: string(hrmlearning.LessonText),
	})
	if _, err := env.hrmLearningSvc.PublishVersion(ctx, orgID, v.ID, ownerID); err != nil {
		t.Fatalf("publish: %v", err)
	}

	caller := lrnAdmin(ownerID)
	e, err := env.hrmLearningSvc.Enroll(ctx, orgID, caller, hrmlearning.EnrollRequest{
		EmployeeID: empID, CourseID: c.ID,
	})
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}

	after1, err := env.hrmLearningSvc.MarkLesson(ctx, orgID, e.ID, l1.ID, caller,
		hrmlearning.MarkLessonRequest{Status: string(hrmlearning.ProgressCompleted)})
	if err != nil {
		t.Fatalf("mark lesson 1: %v", err)
	}
	if !after1.CompletionPercent.Equal(perfDec("50")) {
		t.Errorf("completion = %s, want 50", after1.CompletionPercent)
	}
	if after1.Status != hrmlearning.EnrollmentInProgress {
		t.Errorf("status = %s, want in_progress", after1.Status)
	}

	after2, err := env.hrmLearningSvc.MarkLesson(ctx, orgID, e.ID, l2.ID, caller,
		hrmlearning.MarkLessonRequest{Status: string(hrmlearning.ProgressCompleted)})
	if err != nil {
		t.Fatalf("mark lesson 2: %v", err)
	}
	if !after2.CompletionPercent.Equal(perfDec("100")) {
		t.Errorf("completion = %s, want 100", after2.CompletionPercent)
	}
	if after2.Status != hrmlearning.EnrollmentCompleted {
		t.Errorf("status = %s, want completed", after2.Status)
	}

	// Nothing is stored: there is no completion column to drift.
	var cols int
	if err := env.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM information_schema.columns
		  WHERE table_name='hrm_enrollments' AND column_name LIKE '%percent%'`).Scan(&cols); err != nil {
		t.Fatalf("introspect: %v", err)
	}
	if cols != 0 {
		t.Errorf("hrm_enrollments has %d percentage columns — completion must stay computed", cols)
	}
}

func TestIntegration_Learning_TenantIsolation(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fxA := seedLearningFixture(t, env)
	fxB := seedLearningFixture(t, env)

	if _, err := env.hrmLearningSvc.GetCourse(ctx, fxB.orgID, fxA.course.ID); !errors.Is(err, hrmlearning.ErrCourseNotFound) {
		t.Errorf("org B reached org A's course: %v", err)
	}
	if _, err := env.hrmLearningSvc.GetVersion(ctx, fxB.orgID, fxA.version.ID); !errors.Is(err, hrmlearning.ErrVersionNotFound) {
		t.Errorf("org B reached org A's version: %v", err)
	}
	if _, err := env.hrmLearningSvc.GetAnswerKeys(ctx, fxB.orgID, fxA.quiz.ID,
		lrnAdmin(fxB.ownerID)); !errors.Is(err, hrmlearning.ErrLessonNotFound) {
		t.Errorf("org B reached org A's answer keys: %v", err)
	}
}
