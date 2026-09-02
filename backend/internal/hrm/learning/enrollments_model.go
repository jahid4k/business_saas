// backend/internal/hrm/learning/enrollments_model.go
package learning

import (
	"time"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/authz"
)

// Enrollment is one learner on one PINNED course version.
//
// VersionID is the pin, and it is what makes a completed enrollment mean
// something: publishing a new version of the course leaves this row pointing
// at the content the learner actually took. The FK is RESTRICT for the same
// reason — a version with enrollments is history.
type Enrollment struct {
	ID         string `db:"id"          json:"id"`
	PublicID   string `db:"public_id"   json:"public_id"`
	OrgID      string `db:"org_id"      json:"org_id"`
	EmployeeID string `db:"employee_id" json:"employee_id"`
	CourseID   string `db:"course_id"   json:"course_id"`
	VersionID  string `db:"version_id"  json:"version_id"`

	Status      EnrollmentStatus `db:"status"       json:"status"`
	AssignedVia AssignedVia      `db:"assigned_via" json:"assigned_via"`

	DueDate     *time.Time `db:"due_date"     json:"due_date,omitempty"`
	StartedAt   *time.Time `db:"started_at"   json:"started_at,omitempty"`
	CompletedAt *time.Time `db:"completed_at" json:"completed_at,omitempty"`

	AssignedBy *string   `db:"assigned_by" json:"assigned_by,omitempty"`
	CreatedAt  time.Time `db:"created_at"  json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"  json:"updated_at"`

	// Joined for display, never authoritative.
	CourseTitle *string `db:"course_title" json:"course_title,omitempty"`
}

// LessonProgress is per-lesson state within an enrollment. Created lazily on
// first interaction rather than pre-seeded, so adding a lesson to a draft
// version cannot leave stale rows behind on live enrollments.
type LessonProgress struct {
	ID           string         `db:"id"            json:"id"`
	PublicID     string         `db:"public_id"     json:"public_id"`
	EnrollmentID string         `db:"enrollment_id" json:"enrollment_id"`
	LessonID     string         `db:"lesson_id"     json:"lesson_id"`
	Status       ProgressStatus `db:"status"        json:"status"`
	CompletedAt  *time.Time     `db:"completed_at"  json:"completed_at,omitempty"`
	CreatedAt    time.Time      `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time      `db:"updated_at"    json:"updated_at"`
}

// EnrollmentDetail carries the computed figures alongside the record.
//
// CompletionPercent is COMPUTED from lesson progress on every read, never
// stored — migration 00076's standing rule, and the platform_checklists
// precedent. There is no denormalised counter to drift.
type EnrollmentDetail struct {
	*Enrollment
	Version *CourseVersion `json:"version,omitempty"`

	RequiredLessons   int             `json:"required_lessons"`
	CompletedLessons  int             `json:"completed_lessons"`
	CompletionPercent decimal.Decimal `json:"completion_percent"`

	Progress []*LessonProgress `json:"progress"`
	Attempts []*QuizAttempt    `json:"attempts,omitempty"`
}

// CompletionPercent computes progress from counts. Exposed as a function so
// the rule has one home and a test can hit it directly.
//
// Returns zero when there are no required lessons — an empty course is not
// 100% complete, it is unstarted, and reporting 100 would auto-complete every
// enrollment on a version whose lessons were all optional.
func CompletionPercent(completed, required int) decimal.Decimal {
	if required <= 0 {
		return decimal.Zero
	}
	if completed > required {
		completed = required
	}
	return decimal.NewFromInt(int64(completed)).
		Div(decimal.NewFromInt(int64(required))).
		Mul(decimal.NewFromInt(100)).Round(2)
}

// ── Requests ─────────────────────────────────────────────────────────────────

// EnrollRequest assigns one employee to a course. VersionID is optional: when
// omitted the service resolves the course's current published version, which
// is the normal path. Naming it explicitly supports re-assigning somebody onto
// an older version deliberately.
type EnrollRequest struct {
	EmployeeID string  `json:"employee_id"`
	CourseID   string  `json:"course_id"`
	VersionID  *string `json:"version_id"`
	DueDate    *string `json:"due_date"`
}

// SelfEnrollRequest is the learner-facing form. It carries no employee_id —
// the service uses the caller's own, which is what stops .enroll_self being
// an assignment permission in disguise.
type SelfEnrollRequest struct {
	CourseID string  `json:"course_id"`
	DueDate  *string `json:"due_date"`
}

type UpdateEnrollmentRequest struct {
	DueDate *string `json:"due_date"`
}

// MarkLessonRequest records progress on a non-quiz lesson. Quiz lessons are
// completed by passing an attempt, never by asserting completion.
type MarkLessonRequest struct {
	Status string `json:"status"`
}

// ── Filters and responses ────────────────────────────────────────────────────

type EnrollmentListFilter struct {
	EmployeeID string
	CourseID   string
	Status     string
	Overdue    bool
	Limit      int
	Offset     int

	Scope        authz.Scope
	CallerUserID string
}

func (f *EnrollmentListFilter) Normalise() {
	if f.Limit <= 0 {
		f.Limit = DefaultLimit
	}
	if f.Limit > MaxLimit {
		f.Limit = MaxLimit
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
}

type EnrollmentListResponse struct {
	Enrollments []*Enrollment `json:"enrollments"`
	Total       int           `json:"total"`
	Limit       int           `json:"limit"`
	Offset      int           `json:"offset"`
}
