// backend/internal/tests/unit/hrm/learning/service_test.go
// Phase 6A service rules: version pinning, the answer-key boundary, quiz
// completion, and the two-part narrowing on assignment.
package learning_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/learning"
	"github.com/mridha/businesssaas/internal/platform/forms"
)

func newSvc(allow bool) (learning.Service, *stubRepo, *stubForms) {
	repo := newStubRepo()
	repo.employees[learnerEmp] = &learning.EmployeeRef{
		EmployeeID: learnerEmp, DisplayName: "The Learner", UserID: strPtr(learnerUser),
	}
	repo.employees[otherEmp] = &learning.EmployeeRef{EmployeeID: otherEmp, DisplayName: "Somebody Else"}
	repo.empUsers[learnerUser] = learnerEmp
	fs := newStubForms()
	return learning.NewService(repo, &stubAuthorizer{allow: allow}, fs), repo, fs
}

func adminCaller() learning.Caller {
	return learning.Caller{
		UserID: ownerUserID, Tier: authz.ScopeAll, CanManage: true, CanGrade: true,
	}
}

// managerCaller holds manage but NOT grade — the grant migration 00093 gives
// the 'manager' role.
func managerCaller() learning.Caller {
	return learning.Caller{
		UserID: ownerUserID, Tier: authz.ScopeAll, CanManage: true, CanGrade: false,
	}
}

func learnerCaller() learning.Caller {
	return learning.Caller{UserID: learnerUser, Tier: authz.ScopeOwn, EmployeeID: learnerEmp}
}

// seedPublishedCourse builds a course with one published version containing
// one required text lesson, and returns both.
func seedPublishedCourse(t *testing.T, svc learning.Service) (*learning.Course, *learning.CourseVersion) {
	t.Helper()
	ctx := context.Background()

	c, err := svc.CreateCourse(ctx, testOrg, ownerUserID, learning.CreateCourseRequest{Title: "Security Basics"})
	if err != nil {
		t.Fatalf("create course: %v", err)
	}
	v, err := svc.CreateVersion(ctx, testOrg, c.ID, ownerUserID, learning.CreateVersionRequest{})
	if err != nil {
		t.Fatalf("create version: %v", err)
	}
	m, err := svc.CreateModule(ctx, testOrg, v.ID, learning.CreateModuleRequest{Title: "Intro"})
	if err != nil {
		t.Fatalf("create module: %v", err)
	}
	if _, err := svc.CreateLesson(ctx, testOrg, m.ID, learning.CreateLessonRequest{
		Title: "Read the policy", LessonType: string(learning.LessonText),
	}); err != nil {
		t.Fatalf("create lesson: %v", err)
	}
	published, err := svc.PublishVersion(ctx, testOrg, v.ID, ownerUserID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	return c, published
}

// ============================================================
// Version pinning — an edit must not rewrite history
// ============================================================

// TestPublishedVersionIsImmutable is the headline authoring rule. Editing a
// published version would change what an already-enrolled learner is being
// assessed on.
func TestPublishedVersionIsImmutable(t *testing.T) {
	ctx := context.Background()
	svc, repo, _ := newSvc(true)
	_, v := seedPublishedCourse(t, svc)

	note := "sneaky edit"
	if _, err := svc.UpdateVersion(ctx, testOrg, v.ID, learning.UpdateVersionRequest{
		ChangeNote: &note,
	}); !errors.Is(err, learning.ErrVersionNotEditable) {
		t.Fatalf("expected ErrVersionNotEditable on the version itself, got %v", err)
	}

	// And every content write below it is refused too — the guard is on the
	// version, so adding a module to a published one must fail.
	if _, err := svc.CreateModule(ctx, testOrg, v.ID, learning.CreateModuleRequest{
		Title: "Bonus module",
	}); !errors.Is(err, learning.ErrVersionNotEditable) {
		t.Fatalf("expected ErrVersionNotEditable adding a module, got %v", err)
	}

	// Existing lessons cannot be edited either.
	var lessonID string
	for id := range repo.lessons {
		lessonID = id
	}
	newTitle := "Renamed"
	if _, err := svc.UpdateLesson(ctx, testOrg, lessonID, learning.UpdateLessonRequest{
		Title: &newTitle,
	}); !errors.Is(err, learning.ErrVersionNotEditable) {
		t.Errorf("expected ErrVersionNotEditable editing a published lesson, got %v", err)
	}
}

// TestEnrollmentPinsVersion_AndSurvivesANewPublish is the point of the pin: a
// learner already enrolled keeps the content they started.
func TestEnrollmentPinsVersion_AndSurvivesANewPublish(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newSvc(true)
	c, v1 := seedPublishedCourse(t, svc)

	e, err := svc.Enroll(ctx, testOrg, adminCaller(), learning.EnrollRequest{
		EmployeeID: learnerEmp, CourseID: c.ID,
	})
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}
	if e.VersionID != v1.ID {
		t.Fatalf("enrollment pinned %s, want the published version %s", e.VersionID, v1.ID)
	}

	// Publish v2 with different content.
	v2, err := svc.CreateVersion(ctx, testOrg, c.ID, ownerUserID, learning.CreateVersionRequest{
		CopyFromVersionID: &v1.ID,
	})
	if err != nil {
		t.Fatalf("create v2: %v", err)
	}
	m2, err := svc.CreateModule(ctx, testOrg, v2.ID, learning.CreateModuleRequest{Title: "New material"})
	if err != nil {
		t.Fatalf("create v2 module: %v", err)
	}
	if _, err := svc.CreateLesson(ctx, testOrg, m2.ID, learning.CreateLessonRequest{
		Title: "Extra lesson", LessonType: string(learning.LessonText),
	}); err != nil {
		t.Fatalf("create v2 lesson: %v", err)
	}
	if _, err := svc.PublishVersion(ctx, testOrg, v2.ID, ownerUserID); err != nil {
		t.Fatalf("publish v2: %v", err)
	}

	// The existing enrollment is untouched.
	reread, err := svc.GetEnrollment(ctx, testOrg, e.ID, adminCaller())
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if reread.VersionID != v1.ID {
		t.Errorf("enrollment moved to %s after a new publish — the pin is not holding", reread.VersionID)
	}
	// v1 had one required lesson; v2 has two. The pinned learner still sees 1.
	if reread.RequiredLessons != 1 {
		t.Errorf("pinned enrollment sees %d required lessons, want 1 — it is reading the wrong version",
			reread.RequiredLessons)
	}
}

func TestCreateVersion_RejectsSecondDraft(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newSvc(true)
	c, err := svc.CreateCourse(ctx, testOrg, ownerUserID, learning.CreateCourseRequest{Title: "Course"})
	if err != nil {
		t.Fatalf("create course: %v", err)
	}
	if _, err := svc.CreateVersion(ctx, testOrg, c.ID, ownerUserID, learning.CreateVersionRequest{}); err != nil {
		t.Fatalf("first draft: %v", err)
	}
	if _, err := svc.CreateVersion(ctx, testOrg, c.ID, ownerUserID, learning.CreateVersionRequest{}); !errors.Is(err, learning.ErrDraftExists) {
		t.Fatalf("expected ErrDraftExists, got %v", err)
	}
}

// TestPublishVersion_RejectsEmpty stops a version that would auto-complete for
// every learner assigned it, since completion is computed over its lessons.
func TestPublishVersion_RejectsEmpty(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newSvc(true)
	c, err := svc.CreateCourse(ctx, testOrg, ownerUserID, learning.CreateCourseRequest{Title: "Empty"})
	if err != nil {
		t.Fatalf("create course: %v", err)
	}
	v, err := svc.CreateVersion(ctx, testOrg, c.ID, ownerUserID, learning.CreateVersionRequest{})
	if err != nil {
		t.Fatalf("create version: %v", err)
	}
	if _, err := svc.PublishVersion(ctx, testOrg, v.ID, ownerUserID); !errors.Is(err, learning.ErrVersionHasNoLessons) {
		t.Fatalf("expected ErrVersionHasNoLessons, got %v", err)
	}
}

// TestEnroll_RefusesDraftVersion — a draft's content is still changing, which
// is exactly what the pin exists to prevent.
func TestEnroll_RefusesDraftVersion(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newSvc(true)
	c, err := svc.CreateCourse(ctx, testOrg, ownerUserID, learning.CreateCourseRequest{Title: "Draft only"})
	if err != nil {
		t.Fatalf("create course: %v", err)
	}
	v, err := svc.CreateVersion(ctx, testOrg, c.ID, ownerUserID, learning.CreateVersionRequest{})
	if err != nil {
		t.Fatalf("create version: %v", err)
	}

	// Neither implicitly...
	if _, err := svc.Enroll(ctx, testOrg, adminCaller(), learning.EnrollRequest{
		EmployeeID: learnerEmp, CourseID: c.ID,
	}); !errors.Is(err, learning.ErrVersionNotPublished) {
		t.Errorf("expected ErrVersionNotPublished with no published version, got %v", err)
	}
	// ...nor by naming the draft explicitly.
	if _, err := svc.Enroll(ctx, testOrg, adminCaller(), learning.EnrollRequest{
		EmployeeID: learnerEmp, CourseID: c.ID, VersionID: &v.ID,
	}); !errors.Is(err, learning.ErrVersionNotPublished) {
		t.Errorf("expected ErrVersionNotPublished naming the draft, got %v", err)
	}
}

// ============================================================
// The answer-key boundary
// ============================================================

// seedQuizCourse builds a published course whose single required lesson is a
// two-question quiz with a 50% pass mark, and registers the answer keys.
func seedQuizCourse(t *testing.T, svc learning.Service, repo *stubRepo, fs *stubForms, maxAttempts *int) (*learning.Course, *learning.Lesson) {
	t.Helper()
	ctx := context.Background()

	fs.questions[templateID] = []stubQuestion{
		{id: "q1", qType: forms.QuestionBoolean, text: "Is phishing bad?"},
		{id: "q2", qType: forms.QuestionBoolean, text: "Should you share passwords?"},
	}
	repo.templateQuestions[templateID] = []string{"q1", "q2"}

	c, err := svc.CreateCourse(ctx, testOrg, ownerUserID, learning.CreateCourseRequest{Title: "Quiz Course"})
	if err != nil {
		t.Fatalf("create course: %v", err)
	}
	v, err := svc.CreateVersion(ctx, testOrg, c.ID, ownerUserID, learning.CreateVersionRequest{})
	if err != nil {
		t.Fatalf("create version: %v", err)
	}
	m, err := svc.CreateModule(ctx, testOrg, v.ID, learning.CreateModuleRequest{Title: "Assessment"})
	if err != nil {
		t.Fatalf("create module: %v", err)
	}
	tmpl := templateID
	pass := dec("50")
	l, err := svc.CreateLesson(ctx, testOrg, m.ID, learning.CreateLessonRequest{
		Title: "Final quiz", LessonType: string(learning.LessonQuiz),
		FormTemplateID: &tmpl, PassMark: &pass, MaxAttempts: maxAttempts,
	})
	if err != nil {
		t.Fatalf("create quiz lesson: %v", err)
	}

	for _, qid := range []string{"q1", "q2"} {
		correct := qid == "q1" // q1 answer is true, q2 answer is false
		if err := svc.SetAnswerKey(ctx, testOrg, l.ID, learning.SetAnswerKeyRequest{
			QuestionID: qid, CorrectBoolean: boolPtr(correct),
		}); err != nil {
			t.Fatalf("set answer key %s: %v", qid, err)
		}
	}

	if _, err := svc.PublishVersion(ctx, testOrg, v.ID, ownerUserID); err != nil {
		t.Fatalf("publish: %v", err)
	}
	return c, l
}

// TestStartAttempt_ServesNoCorrectAnswers is the headline security test. The
// learner receives questions and their own answers, and nothing else.
func TestStartAttempt_ServesNoCorrectAnswers(t *testing.T) {
	ctx := context.Background()
	svc, repo, fs := newSvc(true)
	c, l := seedQuizCourse(t, svc, repo, fs, nil)

	e, err := svc.Enroll(ctx, testOrg, adminCaller(), learning.EnrollRequest{
		EmployeeID: learnerEmp, CourseID: c.ID,
	})
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}

	att, err := svc.StartAttempt(ctx, testOrg, e.ID, l.ID, learnerCaller())
	if err != nil {
		t.Fatalf("start attempt: %v", err)
	}
	if len(att.Questions) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(att.Questions))
	}

	// The keys exist and are non-trivial, so this is not passing vacuously.
	if len(repo.keys) != 2 {
		t.Fatalf("expected 2 answer keys seeded, got %d", len(repo.keys))
	}

	// Serialise the whole payload and prove no correct answer appears in it.
	blob := mustJSON(t, att)
	for _, banned := range []string{"correct", "answer_key", "explanation", "points"} {
		if containsFold(blob, banned) {
			t.Errorf("attempt payload contains %q — a learner must never receive the key: %s", banned, blob)
		}
	}
}

// TestGetAnswerKeys_RequiresGradePermission pins the one read path that does
// return keys. 'manager' holds manage but not grade, deliberately: a manager
// who could read the key for their report's quiz has defeated the assessment.
func TestGetAnswerKeys_RequiresGradePermission(t *testing.T) {
	ctx := context.Background()
	svc, repo, fs := newSvc(true)
	_, l := seedQuizCourse(t, svc, repo, fs, nil)

	if _, err := svc.GetAnswerKeys(ctx, testOrg, l.ID, managerCaller()); !errors.Is(err, learning.ErrGradeDenied) {
		t.Fatalf("expected ErrGradeDenied for a manager, got %v", err)
	}
	if _, err := svc.GetAnswerKeys(ctx, testOrg, l.ID, learnerCaller()); !errors.Is(err, learning.ErrGradeDenied) {
		t.Fatalf("expected ErrGradeDenied for a learner, got %v", err)
	}

	keys, err := svc.GetAnswerKeys(ctx, testOrg, l.ID, adminCaller())
	if err != nil {
		t.Fatalf("admin with grade should read keys: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

// ============================================================
// Grading and completion
// ============================================================

func TestSubmitAttempt_GradesFreezesAndCompletes(t *testing.T) {
	ctx := context.Background()
	svc, repo, fs := newSvc(true)
	c, l := seedQuizCourse(t, svc, repo, fs, nil)

	e, err := svc.Enroll(ctx, testOrg, adminCaller(), learning.EnrollRequest{
		EmployeeID: learnerEmp, CourseID: c.ID,
	})
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}
	att, err := svc.StartAttempt(ctx, testOrg, e.ID, l.ID, learnerCaller())
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	// Both correct: q1 true, q2 false.
	answers := []forms.AnswerRequest{
		{ResponseID: att.Questions[0].ResponseID, AnswerBoolean: boolPtr(true)},
		{ResponseID: att.Questions[1].ResponseID, AnswerBoolean: boolPtr(false)},
	}
	out, err := svc.SubmitAttempt(ctx, testOrg, att.ID, learnerCaller(), learning.SubmitAttemptRequest{
		Answers: answers,
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	if out.Score == nil || !out.Score.Equal(dec("100")) {
		t.Errorf("score = %v, want 100", out.Score)
	}
	if out.Passed == nil || !*out.Passed {
		t.Errorf("expected passed, got %v", out.Passed)
	}
	if out.GradedAt == nil {
		t.Error("graded_at was not stamped — the result is not frozen")
	}
	if out.PassMarkSnapshot == nil || !out.PassMarkSnapshot.Equal(dec("50")) {
		t.Errorf("pass mark snapshot = %v, want 50", out.PassMarkSnapshot)
	}

	// Passing the only required lesson completes the enrollment.
	detail, err := svc.GetEnrollment(ctx, testOrg, e.ID, adminCaller())
	if err != nil {
		t.Fatalf("get enrollment: %v", err)
	}
	if detail.Status != learning.EnrollmentCompleted {
		t.Errorf("enrollment status = %s, want completed", detail.Status)
	}
	if !detail.CompletionPercent.Equal(dec("100")) {
		t.Errorf("completion = %s, want 100", detail.CompletionPercent)
	}

	// A graded attempt cannot be resubmitted.
	if _, err := svc.SubmitAttempt(ctx, testOrg, att.ID, learnerCaller(), learning.SubmitAttemptRequest{
		Answers: answers,
	}); !errors.Is(err, learning.ErrAttemptSubmitted) {
		t.Errorf("expected ErrAttemptSubmitted on resubmit, got %v", err)
	}
}

func TestSubmitAttempt_FailingLeavesLessonIncomplete(t *testing.T) {
	ctx := context.Background()
	svc, repo, fs := newSvc(true)
	c, l := seedQuizCourse(t, svc, repo, fs, nil)

	e, _ := svc.Enroll(ctx, testOrg, adminCaller(), learning.EnrollRequest{
		EmployeeID: learnerEmp, CourseID: c.ID,
	})
	att, err := svc.StartAttempt(ctx, testOrg, e.ID, l.ID, learnerCaller())
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	// Both wrong → 0%, below the 50% pass mark.
	out, err := svc.SubmitAttempt(ctx, testOrg, att.ID, learnerCaller(), learning.SubmitAttemptRequest{
		Answers: []forms.AnswerRequest{
			{ResponseID: att.Questions[0].ResponseID, AnswerBoolean: boolPtr(false)},
			{ResponseID: att.Questions[1].ResponseID, AnswerBoolean: boolPtr(true)},
		},
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if out.Passed == nil || *out.Passed {
		t.Fatalf("expected failed, got %v", out.Passed)
	}

	detail, err := svc.GetEnrollment(ctx, testOrg, e.ID, adminCaller())
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if detail.Status == learning.EnrollmentCompleted {
		t.Error("a failed quiz must not complete the enrollment")
	}
	if !detail.CompletionPercent.Equal(dec("0")) {
		t.Errorf("completion = %s, want 0", detail.CompletionPercent)
	}
}

// TestStartAttempt_RespectsMaxAttempts, and the retry path after a failure.
func TestStartAttempt_RespectsMaxAttempts(t *testing.T) {
	ctx := context.Background()
	svc, repo, fs := newSvc(true)
	max := 1
	c, l := seedQuizCourse(t, svc, repo, fs, &max)

	e, _ := svc.Enroll(ctx, testOrg, adminCaller(), learning.EnrollRequest{
		EmployeeID: learnerEmp, CourseID: c.ID,
	})
	att, err := svc.StartAttempt(ctx, testOrg, e.ID, l.ID, learnerCaller())
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if att.AttemptsRemaining == nil || *att.AttemptsRemaining != 0 {
		t.Errorf("attempts remaining = %v, want 0 after using the only one", att.AttemptsRemaining)
	}

	// Fail it.
	if _, err := svc.SubmitAttempt(ctx, testOrg, att.ID, learnerCaller(), learning.SubmitAttemptRequest{
		Answers: []forms.AnswerRequest{
			{ResponseID: att.Questions[0].ResponseID, AnswerBoolean: boolPtr(false)},
			{ResponseID: att.Questions[1].ResponseID, AnswerBoolean: boolPtr(true)},
		},
	}); err != nil {
		t.Fatalf("submit: %v", err)
	}

	if _, err := svc.StartAttempt(ctx, testOrg, e.ID, l.ID, learnerCaller()); !errors.Is(err, learning.ErrAttemptsExhausted) {
		t.Fatalf("expected ErrAttemptsExhausted, got %v", err)
	}
}

// TestStartAttempt_ResumesRatherThanBurningARetry — a page refresh must not
// cost the learner an attempt.
func TestStartAttempt_ResumesOpenAttempt(t *testing.T) {
	ctx := context.Background()
	svc, repo, fs := newSvc(true)
	max := 2
	c, l := seedQuizCourse(t, svc, repo, fs, &max)

	e, _ := svc.Enroll(ctx, testOrg, adminCaller(), learning.EnrollRequest{
		EmployeeID: learnerEmp, CourseID: c.ID,
	})
	first, err := svc.StartAttempt(ctx, testOrg, e.ID, l.ID, learnerCaller())
	if err != nil {
		t.Fatalf("first start: %v", err)
	}
	second, err := svc.StartAttempt(ctx, testOrg, e.ID, l.ID, learnerCaller())
	if err != nil {
		t.Fatalf("second start: %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("a second start opened a NEW attempt (%s vs %s) — a refresh burns a retry",
			first.ID, second.ID)
	}
}

// TestMarkLesson_RefusesQuizLesson pins that a quiz is completed by passing,
// never by asserting completion — otherwise the assessment is optional.
func TestMarkLesson_RefusesQuizLesson(t *testing.T) {
	ctx := context.Background()
	svc, repo, fs := newSvc(true)
	c, l := seedQuizCourse(t, svc, repo, fs, nil)

	e, _ := svc.Enroll(ctx, testOrg, adminCaller(), learning.EnrollRequest{
		EmployeeID: learnerEmp, CourseID: c.ID,
	})
	if _, err := svc.MarkLesson(ctx, testOrg, e.ID, l.ID, learnerCaller(), learning.MarkLessonRequest{
		Status: string(learning.ProgressCompleted),
	}); !errors.Is(err, learning.ErrNotAQuiz) {
		t.Fatalf("expected the quiz lesson to refuse a manual completion, got %v", err)
	}
}

// TestMarkLesson_RejectsLessonFromAnotherCourse. Without this check a learner
// could complete a lesson from a course they are not on and move their own
// progress bar.
func TestMarkLesson_RejectsForeignLesson(t *testing.T) {
	ctx := context.Background()
	svc, repo, _ := newSvc(true)
	c1, _ := seedPublishedCourse(t, svc)

	// A second, unrelated course.
	c2, err := svc.CreateCourse(ctx, testOrg, ownerUserID, learning.CreateCourseRequest{Title: "Other course"})
	if err != nil {
		t.Fatalf("create other course: %v", err)
	}
	v2, _ := svc.CreateVersion(ctx, testOrg, c2.ID, ownerUserID, learning.CreateVersionRequest{})
	m2, _ := svc.CreateModule(ctx, testOrg, v2.ID, learning.CreateModuleRequest{Title: "M"})
	foreign, err := svc.CreateLesson(ctx, testOrg, m2.ID, learning.CreateLessonRequest{
		Title: "Foreign lesson", LessonType: string(learning.LessonText),
	})
	if err != nil {
		t.Fatalf("create foreign lesson: %v", err)
	}
	_ = repo

	e, _ := svc.Enroll(ctx, testOrg, adminCaller(), learning.EnrollRequest{
		EmployeeID: learnerEmp, CourseID: c1.ID,
	})
	if _, err := svc.MarkLesson(ctx, testOrg, e.ID, foreign.ID, learnerCaller(), learning.MarkLessonRequest{
		Status: string(learning.ProgressCompleted),
	}); !errors.Is(err, learning.ErrLessonNotInCourse) {
		t.Fatalf("expected ErrLessonNotInCourse, got %v", err)
	}
}

// ============================================================
// Enrollment rules and authorization
// ============================================================

func TestEnroll_RejectsDuplicateLiveEnrollment(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newSvc(true)
	c, _ := seedPublishedCourse(t, svc)

	if _, err := svc.Enroll(ctx, testOrg, adminCaller(), learning.EnrollRequest{
		EmployeeID: learnerEmp, CourseID: c.ID,
	}); err != nil {
		t.Fatalf("first enroll: %v", err)
	}
	if _, err := svc.Enroll(ctx, testOrg, adminCaller(), learning.EnrollRequest{
		EmployeeID: learnerEmp, CourseID: c.ID,
	}); !errors.Is(err, learning.ErrAlreadyEnrolled) {
		t.Fatalf("expected ErrAlreadyEnrolled, got %v", err)
	}
}

// TestCancelledEnrollmentFreesTheEmployee — which is what makes the unique
// index PARTIAL rather than absolute, and is the recertification path.
func TestCancelledEnrollmentFreesTheEmployee(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newSvc(true)
	c, _ := seedPublishedCourse(t, svc)

	e, _ := svc.Enroll(ctx, testOrg, adminCaller(), learning.EnrollRequest{
		EmployeeID: learnerEmp, CourseID: c.ID,
	})
	if _, err := svc.CancelEnrollment(ctx, testOrg, e.ID, adminCaller()); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if _, err := svc.Enroll(ctx, testOrg, adminCaller(), learning.EnrollRequest{
		EmployeeID: learnerEmp, CourseID: c.ID,
	}); err != nil {
		t.Errorf("a cancelled enrollment should free the employee: %v", err)
	}
}

// TestSelfEnroll_UsesCallersOwnEmployee is what stops .enroll_self — granted
// to every member — being an assignment permission in disguise.
func TestSelfEnroll_UsesCallersOwnEmployee(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newSvc(true)
	c, _ := seedPublishedCourse(t, svc)

	// The caller holds NO manage permission.
	caller := learning.Caller{UserID: learnerUser, Tier: authz.ScopeOwn, EmployeeID: learnerEmp}
	e, err := svc.SelfEnroll(ctx, testOrg, caller, learning.SelfEnrollRequest{CourseID: c.ID})
	if err != nil {
		t.Fatalf("self enroll: %v", err)
	}
	if e.EmployeeID != learnerEmp {
		t.Errorf("self enrollment landed on %s, want the caller's own %s", e.EmployeeID, learnerEmp)
	}
	if e.AssignedVia != learning.AssignedSelf {
		t.Errorf("assigned_via = %s, want self", e.AssignedVia)
	}
}

// TestAssignment_RequiresManageAndScope pins the two-part narrowing.
// hrm.enrollments.manage is unscoped at the route, so the record check is the
// only thing stopping a view_team manager assigning outside their line — and
// that middle case is the one people forget.
func TestAssignment_RequiresManageAndScope(t *testing.T) {
	ctx := context.Background()

	t.Run("no manage", func(t *testing.T) {
		svc, _, _ := newSvc(true)
		c, _ := seedPublishedCourse(t, svc)
		caller := learning.Caller{UserID: ownerUserID, Tier: authz.ScopeAll, CanManage: false}
		if _, err := svc.Enroll(ctx, testOrg, caller, learning.EnrollRequest{
			EmployeeID: learnerEmp, CourseID: c.ID,
		}); !errors.Is(err, learning.ErrAccessDenied) {
			t.Errorf("expected ErrAccessDenied without manage, got %v", err)
		}
	})

	t.Run("manage but out of scope", func(t *testing.T) {
		svc, _, _ := newSvc(false) // authorizer refuses anything below ScopeAll
		c, _ := seedPublishedCourse(t, svc)
		caller := learning.Caller{UserID: ownerUserID, Tier: authz.ScopeTeam, CanManage: true}
		if _, err := svc.Enroll(ctx, testOrg, caller, learning.EnrollRequest{
			EmployeeID: learnerEmp, CourseID: c.ID,
		}); !errors.Is(err, learning.ErrAccessDenied) {
			t.Errorf("expected ErrAccessDenied outside the caller's tier, got %v", err)
		}
	})

	t.Run("manage and in scope", func(t *testing.T) {
		svc, _, _ := newSvc(true)
		c, _ := seedPublishedCourse(t, svc)
		caller := learning.Caller{UserID: ownerUserID, Tier: authz.ScopeTeam, CanManage: true}
		if _, err := svc.Enroll(ctx, testOrg, caller, learning.EnrollRequest{
			EmployeeID: learnerEmp, CourseID: c.ID,
		}); err != nil {
			t.Errorf("a manager acting inside their tier should succeed: %v", err)
		}
	})
}

// TestMarkLesson_OnlyTheLearner pins the narrowing the route gate cannot
// express: hrm.enrollments.attempt reaches every member.
func TestMarkLesson_OnlyTheLearnerOrManager(t *testing.T) {
	ctx := context.Background()
	svc, repo, _ := newSvc(true)
	c, _ := seedPublishedCourse(t, svc)

	e, _ := svc.Enroll(ctx, testOrg, adminCaller(), learning.EnrollRequest{
		EmployeeID: learnerEmp, CourseID: c.ID,
	})
	var lessonID string
	for id := range repo.lessons {
		lessonID = id
	}

	// A different member cannot progress somebody else's enrollment.
	stranger := learning.Caller{UserID: "usr_stranger", Tier: authz.ScopeAll, EmployeeID: otherEmp}
	if _, err := svc.MarkLesson(ctx, testOrg, e.ID, lessonID, stranger, learning.MarkLessonRequest{
		Status: string(learning.ProgressCompleted),
	}); !errors.Is(err, learning.ErrNotLearner) {
		t.Fatalf("expected ErrNotLearner, got %v", err)
	}

	// The learner can.
	if _, err := svc.MarkLesson(ctx, testOrg, e.ID, lessonID, learnerCaller(), learning.MarkLessonRequest{
		Status: string(learning.ProgressCompleted),
	}); err != nil {
		t.Fatalf("the learner should be able to progress their own enrollment: %v", err)
	}
}

func TestGetEnrollment_DeniesOutOfScope(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newSvc(false)
	c, _ := seedPublishedCourse(t, svc)
	e, _ := svc.Enroll(ctx, testOrg, adminCaller(), learning.EnrollRequest{
		EmployeeID: learnerEmp, CourseID: c.ID,
	})

	outsider := learning.Caller{UserID: "usr_stranger", Tier: authz.ScopeOwn}
	if _, err := svc.GetEnrollment(ctx, testOrg, e.ID, outsider); !errors.Is(err, learning.ErrAccessDenied) {
		t.Fatalf("expected ErrAccessDenied, got %v", err)
	}
}

// TestCreateLesson_QuizNeedsTemplate pins the rule the schema deliberately
// carries no CHECK for — a CHECK pairing lesson_type with form_template_id
// would break DELETE on platform_form_templates (the 00076 trap).
func TestCreateLesson_QuizNeedsTemplate(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newSvc(true)
	c, err := svc.CreateCourse(ctx, testOrg, ownerUserID, learning.CreateCourseRequest{Title: "C"})
	if err != nil {
		t.Fatalf("create course: %v", err)
	}
	v, _ := svc.CreateVersion(ctx, testOrg, c.ID, ownerUserID, learning.CreateVersionRequest{})
	m, _ := svc.CreateModule(ctx, testOrg, v.ID, learning.CreateModuleRequest{Title: "M"})

	if _, err := svc.CreateLesson(ctx, testOrg, m.ID, learning.CreateLessonRequest{
		Title: "Quiz with no template", LessonType: string(learning.LessonQuiz),
	}); !errors.Is(err, learning.ErrQuizNotConfigured) {
		t.Fatalf("expected ErrQuizNotConfigured, got %v", err)
	}
}
