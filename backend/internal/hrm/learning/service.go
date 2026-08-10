// backend/internal/hrm/learning/service.go
package learning

import (
	"context"
	"strings"
	"time"
)

// Service is the Phase 6A surface: course authoring with versioning,
// enrollment, lesson progress, and quiz attempts with grading.
type Service interface {
	// ── Course authoring ────────────────────────────────────────────────
	ListCourses(ctx context.Context, orgID string, f CourseListFilter) (*CourseListResponse, error)
	GetCourse(ctx context.Context, orgID, ref string) (*Course, error)
	CreateCourse(ctx context.Context, orgID, createdBy string, req CreateCourseRequest) (*Course, error)
	UpdateCourse(ctx context.Context, orgID, ref string, req UpdateCourseRequest) (*Course, error)
	DeleteCourse(ctx context.Context, orgID, ref string) error

	// ── Versioning ──────────────────────────────────────────────────────
	ListVersions(ctx context.Context, orgID, courseRef string) ([]*CourseVersion, error)
	GetVersion(ctx context.Context, orgID, ref string) (*CourseVersion, error)
	CreateVersion(ctx context.Context, orgID, courseRef, createdBy string, req CreateVersionRequest) (*CourseVersion, error)
	UpdateVersion(ctx context.Context, orgID, ref string, req UpdateVersionRequest) (*CourseVersion, error)
	PublishVersion(ctx context.Context, orgID, ref, actorID string) (*CourseVersion, error)
	ArchiveVersion(ctx context.Context, orgID, ref string) (*CourseVersion, error)
	DeleteVersion(ctx context.Context, orgID, ref string) error

	// ── Content ─────────────────────────────────────────────────────────
	CreateModule(ctx context.Context, orgID, versionRef string, req CreateModuleRequest) (*Module, error)
	UpdateModule(ctx context.Context, orgID, ref string, req UpdateModuleRequest) (*Module, error)
	DeleteModule(ctx context.Context, orgID, ref string) error
	CreateLesson(ctx context.Context, orgID, moduleRef string, req CreateLessonRequest) (*Lesson, error)
	UpdateLesson(ctx context.Context, orgID, ref string, req UpdateLessonRequest) (*Lesson, error)
	DeleteLesson(ctx context.Context, orgID, ref string) error

	// ── Answer keys (authoring side) ────────────────────────────────────
	SetAnswerKey(ctx context.Context, orgID, lessonRef string, req SetAnswerKeyRequest) error
	// GetAnswerKeys is the ONE read path that returns correct answers, and it
	// requires hrm.enrollments.grade. Never reachable by a learner.
	GetAnswerKeys(ctx context.Context, orgID, lessonRef string, caller Caller) (map[string]*AnswerKey, error)

	// ── Enrollment ──────────────────────────────────────────────────────
	ListEnrollments(ctx context.Context, orgID string, caller Caller, f EnrollmentListFilter) (*EnrollmentListResponse, error)
	GetEnrollment(ctx context.Context, orgID, ref string, caller Caller) (*EnrollmentDetail, error)
	Enroll(ctx context.Context, orgID string, caller Caller, req EnrollRequest) (*EnrollmentDetail, error)
	SelfEnroll(ctx context.Context, orgID string, caller Caller, req SelfEnrollRequest) (*EnrollmentDetail, error)
	UpdateEnrollment(ctx context.Context, orgID, ref string, caller Caller, req UpdateEnrollmentRequest) (*EnrollmentDetail, error)
	CancelEnrollment(ctx context.Context, orgID, ref string, caller Caller) (*EnrollmentDetail, error)

	// ── Learning ────────────────────────────────────────────────────────
	MarkLesson(ctx context.Context, orgID, enrollmentRef, lessonRef string, caller Caller, req MarkLessonRequest) (*EnrollmentDetail, error)
	StartAttempt(ctx context.Context, orgID, enrollmentRef, lessonRef string, caller Caller) (*AttemptDetail, error)
	SubmitAttempt(ctx context.Context, orgID, attemptRef string, caller Caller, req SubmitAttemptRequest) (*AttemptDetail, error)
}

type serviceImpl struct {
	repo    Repository
	records RecordAuthorizer
	forms   FormEngine
}

func NewService(repo Repository, records RecordAuthorizer, formEngine FormEngine) Service {
	return &serviceImpl{repo: repo, records: records, forms: formEngine}
}

// ── Shared helpers ───────────────────────────────────────────────────────────

func nilIfBlank(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}

func parseDate(s *string) (*time.Time, error) {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil, nil
	}
	t, err := time.Parse(dateLayout, strings.TrimSpace(*s))
	if err != nil {
		return nil, ErrInvalidDate
	}
	return &t, nil
}

func intOr(p *int, fallback int) int {
	if p == nil {
		return fallback
	}
	return *p
}

func boolOr(p *bool, fallback bool) bool {
	if p == nil {
		return fallback
	}
	return *p
}
