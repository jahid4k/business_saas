// backend/internal/hrm/learning/model.go
package learning

import (
	"context"
	"errors"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/platform/forms"
)

// Package learning implements HRM Learning & Development (Phase 6).
//
// Sub-feature quartets in one package (courses, enrollments, quizzes), the
// internal/hrm/recruitment and internal/hrm/performance shape. Phase 5 set a
// split threshold of roughly 60 methods on a composite Repository; if this one
// approaches it in 6B, split courses/ from enrollments/ then — a file move,
// where merging later is the harder direction.
//
// ── Two rules this package exists to enforce ─────────────────────────────
//
//  1. A LEARNER NEVER RECEIVES A CORRECT ANSWER. The build plan asks for
//     "separate DTOs: QuestionForAttempt never carries the correct answer",
//     but the platform form engine has no correct-answer column at all —
//     platform_form_questions stores question text, type, options and weight,
//     and computeScore normalises answers 0-1 against their own scale, which
//     is a RATING score, not an assessment.
//
//     So the key lives in hrm_quiz_answer_keys, owned here. The protection is
//     structural rather than disciplinary: the attempt read path fetches the
//     form instance from the engine and never touches the key table, so there
//     is no field to forget to strip. Same shape internal/hrm/feedback uses
//     for 360 anonymity.
//
//  2. AN EDIT CANNOT REWRITE HISTORY. Enrollments pin a course VERSION, and
//     quiz attempts store their grade rather than re-deriving it. Both follow
//     hrm_appraisals: an immutable record whose numbers are recomputed from
//     mutable sources is not actually immutable.

const dateLayout = "2006-01-02"

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

// ── Course authoring ─────────────────────────────────────────────────────────

type VersionStatus string

const (
	VersionDraft     VersionStatus = "draft"
	VersionPublished VersionStatus = "published"
	VersionArchived  VersionStatus = "archived"
)

func (s VersionStatus) IsValid() bool {
	switch s {
	case VersionDraft, VersionPublished, VersionArchived:
		return true
	}
	return false
}

// IsEditable reports whether course content may still be changed. Only a
// draft: once published, a version is what somebody may already have been
// enrolled against, and editing it in place is exactly what version pinning
// exists to prevent.
func (s VersionStatus) IsEditable() bool { return s == VersionDraft }

type LessonType string

const (
	LessonLink LessonType = "link"
	LessonPDF  LessonType = "pdf"
	LessonText LessonType = "text"
	LessonQuiz LessonType = "quiz"
)

func (t LessonType) IsValid() bool {
	switch t {
	case LessonLink, LessonPDF, LessonText, LessonQuiz:
		return true
	}
	return false
}

// ── Enrollment ───────────────────────────────────────────────────────────────

type EnrollmentStatus string

const (
	EnrollmentAssigned   EnrollmentStatus = "assigned"
	EnrollmentInProgress EnrollmentStatus = "in_progress"
	EnrollmentCompleted  EnrollmentStatus = "completed"
	EnrollmentFailed     EnrollmentStatus = "failed"
	EnrollmentCancelled  EnrollmentStatus = "cancelled"
)

func (s EnrollmentStatus) IsValid() bool {
	switch s {
	case EnrollmentAssigned, EnrollmentInProgress, EnrollmentCompleted,
		EnrollmentFailed, EnrollmentCancelled:
		return true
	}
	return false
}

// IsLive mirrors the partial unique index uq_hrm_enr_employee_course_live,
// which is what actually enforces one live enrollment per employee per course.
// The two must list the same statuses, so this is the shared definition.
func (s EnrollmentStatus) IsLive() bool {
	return s == EnrollmentAssigned || s == EnrollmentInProgress
}

// IsTerminal reports whether the enrollment has finished, in any sense.
func (s EnrollmentStatus) IsTerminal() bool {
	return s == EnrollmentCompleted || s == EnrollmentFailed || s == EnrollmentCancelled
}

type AssignedVia string

const (
	AssignedManual AssignedVia = "manual"
	AssignedSelf   AssignedVia = "self"
	AssignedRule   AssignedVia = "rule" // set by Phase 6B auto-assignment
)

type ProgressStatus string

const (
	ProgressNotStarted ProgressStatus = "not_started"
	ProgressInProgress ProgressStatus = "in_progress"
	ProgressCompleted  ProgressStatus = "completed"
)

func (s ProgressStatus) IsValid() bool {
	switch s {
	case ProgressNotStarted, ProgressInProgress, ProgressCompleted:
		return true
	}
	return false
}

// ── Caller / narrow interfaces ───────────────────────────────────────────────

// Caller carries the authorization facts the handler has already resolved, so
// the service needs no authz dependency of its own and stays testable against
// stubs alone. The performance.Caller / pip.Caller precedent.
type Caller struct {
	UserID string
	// Tier comes from authzSvc.ResolveScope(ctx, userID, orgID, "hrm.enrollments").
	Tier authz.Scope
	// CanManage reports whether the caller holds hrm.enrollments.manage —
	// assigning and cancelling other people's enrollments. Still subject to
	// Tier, which is what stops a view_team manager assigning outside their
	// reporting line.
	CanManage bool
	// CanGrade reports whether the caller holds hrm.enrollments.grade. It is
	// the ONLY permission that may read an answer key through an endpoint, and
	// 'manager' deliberately does not hold it — a manager who could read the
	// key for their report's quiz has defeated the assessment.
	CanGrade bool
	// EmployeeID is the caller's own hrm_employees.id, resolved once by the
	// handler. Empty for a non-employee admin acting on the org, which is a
	// valid state and not a failure.
	EmployeeID string
}

// RecordAuthorizer is the one-method slice of *scope.Resolver this package
// needs. An interface purely so tests can stub it; *scope.Resolver satisfies
// it structurally, so main.go passes the resolver directly with no adapter.
type RecordAuthorizer interface {
	AuthorizeRecordAccess(ctx context.Context, tier authz.Scope, orgID, callerUserID, recordEmployeeRef string) (bool, error)
}

// FormEngine is the slice of the platform form engine this package consumes.
// Narrow and consumer-owned: platform/forms never learns that quizzes exist,
// and in particular never learns what a correct answer is.
//
// ScoreInstance is deliberately ABSENT. The engine's score is a weighted
// rating normalised 0-1 per question scale — meaningful for an appraisal,
// meaningless for a quiz, where the only question is whether each answer
// matches the key. Grading is this package's own arithmetic; see Grade().
type FormEngine interface {
	Instantiate(ctx context.Context, orgID, templateRef string, subj forms.SubjectContext) (*forms.InstanceWithResponses, error)
	GetInstance(ctx context.Context, orgID, ref string) (*forms.InstanceWithResponses, error)
	GetTemplate(ctx context.Context, orgID, ref string) (*forms.TemplateWithSections, error)
	SaveAnswers(ctx context.Context, orgID, ref, callerUserID string, req forms.SaveAnswersRequest) (*forms.InstanceWithResponses, error)
	SubmitInstance(ctx context.Context, orgID, ref, callerUserID string) (*forms.InstanceWithResponses, error)
}

// EmployeeRef is the slice of an employee row this package needs. Owned here
// rather than imported from internal/hrm/employees — the onboarding /
// feedback / pip precedent, which keeps the dependency graph free of an
// employees ↔ learning edge.
type EmployeeRef struct {
	EmployeeID  string
	DisplayName string
	UserID      *string
}

// ── Errors ───────────────────────────────────────────────────────────────────

var (
	ErrCourseNotFound     = errors.New("course not found")
	ErrVersionNotFound    = errors.New("course version not found")
	ErrModuleNotFound     = errors.New("course module not found")
	ErrLessonNotFound     = errors.New("lesson not found")
	ErrEnrollmentNotFound = errors.New("enrollment not found")
	ErrAttemptNotFound    = errors.New("quiz attempt not found")
	ErrEmployeeNotFound   = errors.New("employee not found")

	ErrTitleRequired     = errors.New("title is required")
	ErrTitleTaken        = errors.New("a course with this title already exists in this organization")
	ErrInvalidDate       = errors.New("dates must be in YYYY-MM-DD format")
	ErrInvalidLessonType = errors.New("lesson_type must be one of link, pdf, text, quiz")
	ErrInvalidStatus     = errors.New("invalid status")
	ErrPassMarkRange     = errors.New("pass_mark must be between 0 and 100")
	ErrThresholdRange    = errors.New("pass_threshold must be greater than 0 and at most 100")

	// ErrVersionNotEditable is the version-pinning guard. Editing a published
	// version would rewrite what an already-enrolled learner is being
	// assessed on.
	ErrVersionNotEditable  = errors.New("only a draft version can be edited — publish a new version instead")
	ErrVersionNotPublished = errors.New("only a published version can be enrolled against")
	ErrDraftExists         = errors.New("this course already has a draft version")
	ErrVersionHasNoLessons = errors.New("a version needs at least one lesson before it can be published")
	ErrVersionInUse        = errors.New("this version has enrollments and cannot be deleted")
	ErrCourseInactive      = errors.New("this course is not active")

	ErrAlreadyEnrolled  = errors.New("this employee already has a live enrollment on this course")
	ErrEnrollmentClosed = errors.New("this enrollment is closed and can no longer be changed")
	ErrNotLearner       = errors.New("this enrollment does not belong to you")
	ErrAccessDenied     = errors.New("you do not have access to this enrollment")

	ErrNotAQuiz          = errors.New("this lesson is not a quiz")
	ErrQuizNotConfigured = errors.New("this quiz has no form template configured")
	ErrNoAnswerKey       = errors.New("this quiz has no answer key and cannot be graded")
	ErrAttemptsExhausted = errors.New("no attempts remain on this quiz")
	ErrAttemptSubmitted  = errors.New("this attempt has already been submitted")
	ErrLessonNotInCourse = errors.New("that lesson does not belong to this enrollment's course version")
	ErrGradeDenied       = errors.New("reading an answer key requires hrm.enrollments.grade")
)
