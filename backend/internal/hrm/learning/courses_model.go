// backend/internal/hrm/learning/courses_model.go
package learning

import (
	"time"

	"github.com/shopspring/decimal"
)

// Course is the shell. It owns no content — modules and lessons hang off a
// version, which is what makes an enrollment's pin meaningful.
type Course struct {
	ID          string    `db:"id"          json:"id"`
	PublicID    string    `db:"public_id"   json:"public_id"`
	OrgID       string    `db:"org_id"      json:"org_id"`
	Title       string    `db:"title"       json:"title"`
	Description *string   `db:"description" json:"description,omitempty"`
	Category    *string   `db:"category"    json:"category,omitempty"`
	IsActive    bool      `db:"is_active"   json:"is_active"`
	CreatedBy   string    `db:"created_by"  json:"created_by"`
	CreatedAt   time.Time `db:"created_at"  json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"  json:"updated_at"`

	Versions []*CourseVersion `db:"-" json:"versions,omitempty"`
}

// CourseVersion is the content root. draft → published → archived, and only a
// draft is editable — that pair is what stops an edit rewriting what an
// already-enrolled learner is being assessed on.
type CourseVersion struct {
	ID            string          `db:"id"             json:"id"`
	PublicID      string          `db:"public_id"      json:"public_id"`
	OrgID         string          `db:"org_id"         json:"org_id"`
	CourseID      string          `db:"course_id"      json:"course_id"`
	VersionNumber int             `db:"version_number" json:"version_number"`
	TitleSnapshot string          `db:"title_snapshot" json:"title_snapshot"`
	ChangeNote    *string         `db:"change_note"    json:"change_note,omitempty"`
	Status        VersionStatus   `db:"status"         json:"status"`
	PassThreshold decimal.Decimal `db:"pass_threshold" json:"pass_threshold"`

	PublishedAt *time.Time `db:"published_at" json:"published_at,omitempty"`
	PublishedBy *string    `db:"published_by" json:"published_by,omitempty"`
	ArchivedAt  *time.Time `db:"archived_at"  json:"archived_at,omitempty"`
	CreatedBy   string     `db:"created_by"   json:"created_by"`
	CreatedAt   time.Time  `db:"created_at"   json:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"   json:"updated_at"`

	Modules []*Module `db:"-" json:"modules,omitempty"`
}

type Module struct {
	ID           string    `db:"id"            json:"id"`
	PublicID     string    `db:"public_id"     json:"public_id"`
	VersionID    string    `db:"version_id"    json:"version_id"`
	Title        string    `db:"title"         json:"title"`
	Description  *string   `db:"description"   json:"description,omitempty"`
	DisplayOrder int       `db:"display_order" json:"display_order"`
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"    json:"updated_at"`

	Lessons []*Lesson `db:"-" json:"lessons,omitempty"`
}

// Lesson carries its content directly. There is no org-level document table
// to reference — hrm_employee_documents is employee-scoped, the wrong owner
// for course material.
//
// FormTemplateID is set only on quiz lessons. It is safe to expose: a
// template id yields the QUESTIONS, and questions never carry the correct
// answer — that lives in hrm_quiz_answer_keys, which no learner-facing query
// joins.
type Lesson struct {
	ID          string     `db:"id"           json:"id"`
	PublicID    string     `db:"public_id"    json:"public_id"`
	ModuleID    string     `db:"module_id"    json:"module_id"`
	Title       string     `db:"title"        json:"title"`
	LessonType  LessonType `db:"lesson_type"  json:"lesson_type"`
	ContentURL  *string    `db:"content_url"  json:"content_url,omitempty"`
	ContentText *string    `db:"content_text" json:"content_text,omitempty"`

	FormTemplateID *string          `db:"form_template_id" json:"form_template_id,omitempty"`
	PassMark       *decimal.Decimal `db:"pass_mark"        json:"pass_mark,omitempty"`
	MaxAttempts    *int             `db:"max_attempts"     json:"max_attempts,omitempty"`

	IsRequired   bool      `db:"is_required"   json:"is_required"`
	DisplayOrder int       `db:"display_order" json:"display_order"`
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"    json:"updated_at"`
}

// ── Requests ─────────────────────────────────────────────────────────────────

type CreateCourseRequest struct {
	Title       string  `json:"title"`
	Description *string `json:"description"`
	Category    *string `json:"category"`
}

type UpdateCourseRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Category    *string `json:"category"`
	IsActive    *bool   `json:"is_active"`
}

// CreateVersionRequest opens a new draft. Content is copied from the version
// named by CopyFromVersionID when given — the normal path, since a new
// version is usually an edit of the last one rather than a blank page.
type CreateVersionRequest struct {
	ChangeNote        *string          `json:"change_note"`
	PassThreshold     *decimal.Decimal `json:"pass_threshold"`
	CopyFromVersionID *string          `json:"copy_from_version_id"`
}

type UpdateVersionRequest struct {
	ChangeNote    *string          `json:"change_note"`
	PassThreshold *decimal.Decimal `json:"pass_threshold"`
}

type CreateModuleRequest struct {
	Title        string  `json:"title"`
	Description  *string `json:"description"`
	DisplayOrder *int    `json:"display_order"`
}

type UpdateModuleRequest struct {
	Title        *string `json:"title"`
	Description  *string `json:"description"`
	DisplayOrder *int    `json:"display_order"`
}

type CreateLessonRequest struct {
	Title          string           `json:"title"`
	LessonType     string           `json:"lesson_type"`
	ContentURL     *string          `json:"content_url"`
	ContentText    *string          `json:"content_text"`
	FormTemplateID *string          `json:"form_template_id"`
	PassMark       *decimal.Decimal `json:"pass_mark"`
	MaxAttempts    *int             `json:"max_attempts"`
	IsRequired     *bool            `json:"is_required"`
	DisplayOrder   *int             `json:"display_order"`
}

type UpdateLessonRequest struct {
	Title          *string          `json:"title"`
	ContentURL     *string          `json:"content_url"`
	ContentText    *string          `json:"content_text"`
	FormTemplateID *string          `json:"form_template_id"`
	PassMark       *decimal.Decimal `json:"pass_mark"`
	MaxAttempts    *int             `json:"max_attempts"`
	IsRequired     *bool            `json:"is_required"`
	DisplayOrder   *int             `json:"display_order"`
}

// ── Filters and responses ────────────────────────────────────────────────────

type CourseListFilter struct {
	Category string
	IsActive *bool
	Search   string
	Limit    int
	Offset   int
}

func (f *CourseListFilter) Normalise() {
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

type CourseListResponse struct {
	Courses []*Course `json:"courses"`
	Total   int       `json:"total"`
	Limit   int       `json:"limit"`
	Offset  int       `json:"offset"`
}
