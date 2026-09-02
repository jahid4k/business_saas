// backend/internal/hrm/skills/model.go
package skills

import (
	"context"
	"errors"
	"time"

	"github.com/mridha/businesssaas/internal/authz"
)

// Package skills implements the HRM skills taxonomy (Phase 6B).
//
// It is its own package, NOT part of internal/hrm/learning, and that is the
// build plan's instruction realized structurally: "shared taxonomy, consumed
// by Phases 4, 5 and 10 — not an LMS-internal table". Phase 10's succession
// and gap analysis import this package directly rather than reaching through
// a learning-module dependency it has no other use for.
//
// The size rule agreed with it: learning's composite Repository reached 53
// methods in 6A, and folding certifications and skills in would have carried
// it past the ~60 threshold Phase 5A recorded as the point to split.
//
// ⚠ hrm_position_skills — the skills a POSITION requires — is deliberately
// NOT built here. It has no reader until Phase 10, recruitment and
// performance were both grepped and carry zero skills fields, so there is
// nothing to retrofit into. Building it now would be the speculative
// primitive the build plan's rule 1 exists to prevent. When Phase 10 needs
// it, it is one migration and one repository file — this package is shaped to
// receive it.

const dateLayout = "2006-01-02"

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

// Proficiency is optional on an employee skill: a taxonomy entry is useful
// even when nobody has rated the level yet.
type Proficiency string

const (
	Beginner     Proficiency = "beginner"
	Intermediate Proficiency = "intermediate"
	Advanced     Proficiency = "advanced"
	Expert       Proficiency = "expert"
)

func (p Proficiency) IsValid() bool {
	switch p {
	case Beginner, Intermediate, Advanced, Expert:
		return true
	}
	return false
}

// Source records HOW a skill was acquired. The distinction matters for trust:
// a skill granted by passing a course carries different weight from one an
// employee typed in about themselves.
type Source string

const (
	SourceManual        Source = "manual"
	SourceCourse        Source = "course"
	SourceCertification Source = "certification"
)

func (s Source) IsValid() bool {
	switch s {
	case SourceManual, SourceCourse, SourceCertification:
		return true
	}
	return false
}

// ── Records ──────────────────────────────────────────────────────────────────

// Skill is one entry in the org taxonomy. It names nothing about courses on
// purpose — that is what lets Phase 10 adopt the table unchanged.
type Skill struct {
	ID          string    `db:"id"          json:"id"`
	PublicID    string    `db:"public_id"   json:"public_id"`
	OrgID       string    `db:"org_id"      json:"org_id"`
	Name        string    `db:"name"        json:"name"`
	Description *string   `db:"description" json:"description,omitempty"`
	Category    *string   `db:"category"    json:"category,omitempty"`
	IsActive    bool      `db:"is_active"   json:"is_active"`
	CreatedBy   string    `db:"created_by"  json:"created_by"`
	CreatedAt   time.Time `db:"created_at"  json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"  json:"updated_at"`
}

type EmployeeSkill struct {
	ID          string       `db:"id"          json:"id"`
	PublicID    string       `db:"public_id"   json:"public_id"`
	OrgID       string       `db:"org_id"      json:"org_id"`
	EmployeeID  string       `db:"employee_id" json:"employee_id"`
	SkillID     string       `db:"skill_id"    json:"skill_id"`
	Proficiency *Proficiency `db:"proficiency" json:"proficiency,omitempty"`

	Source                Source     `db:"source"                  json:"source"`
	SourceEnrollmentID    *string    `db:"source_enrollment_id"    json:"source_enrollment_id,omitempty"`
	SourceCertificationID *string    `db:"source_certification_id" json:"source_certification_id,omitempty"`
	AcquiredOn            *time.Time `db:"acquired_on"             json:"acquired_on,omitempty"`

	Notes     *string   `db:"notes"      json:"notes,omitempty"`
	CreatedBy *string   `db:"created_by" json:"created_by,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`

	// Joined for display, never authoritative.
	SkillName *string `db:"skill_name" json:"skill_name,omitempty"`
}

// ── Caller / narrow interfaces ───────────────────────────────────────────────

type Caller struct {
	UserID string
	// Tier comes from authzSvc.ResolveScope(ctx, userID, orgID, "hrm.skills").
	Tier authz.Scope
	// CanManage reports whether the caller holds hrm.skills.manage — editing
	// the taxonomy and recording other people's skills. Still subject to Tier.
	CanManage bool
}

// RecordAuthorizer is the one-method slice of *scope.Resolver this package
// needs. *scope.Resolver satisfies it structurally, so main.go passes the
// resolver directly with no adapter.
type RecordAuthorizer interface {
	AuthorizeRecordAccess(ctx context.Context, tier authz.Scope, orgID, callerUserID, recordEmployeeRef string) (bool, error)
}

// ── Requests ─────────────────────────────────────────────────────────────────

type CreateSkillRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Category    *string `json:"category"`
}

type UpdateSkillRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Category    *string `json:"category"`
	IsActive    *bool   `json:"is_active"`
}

type GrantSkillRequest struct {
	EmployeeID  string  `json:"employee_id"`
	SkillID     string  `json:"skill_id"`
	Proficiency *string `json:"proficiency"`
	AcquiredOn  *string `json:"acquired_on"`
	Notes       *string `json:"notes"`
}

type UpdateEmployeeSkillRequest struct {
	Proficiency *string `json:"proficiency"`
	AcquiredOn  *string `json:"acquired_on"`
	Notes       *string `json:"notes"`
}

// ── Filters and responses ────────────────────────────────────────────────────

type SkillListFilter struct {
	Category string
	IsActive *bool
	Search   string
	Limit    int
	Offset   int
}

func (f *SkillListFilter) Normalise() {
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

type SkillListResponse struct {
	Skills []*Skill `json:"skills"`
	Total  int      `json:"total"`
	Limit  int      `json:"limit"`
	Offset int      `json:"offset"`
}

type EmployeeSkillListFilter struct {
	EmployeeID string
	SkillID    string
	Source     string
	Limit      int
	Offset     int

	Scope        authz.Scope
	CallerUserID string
}

func (f *EmployeeSkillListFilter) Normalise() {
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

type EmployeeSkillListResponse struct {
	Skills []*EmployeeSkill `json:"employee_skills"`
	Total  int              `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

// ── Errors ───────────────────────────────────────────────────────────────────

var (
	ErrSkillNotFound         = errors.New("skill not found")
	ErrEmployeeSkillNotFound = errors.New("employee skill not found")
	ErrEmployeeNotFound      = errors.New("employee not found")

	ErrNameRequired   = errors.New("name is required")
	ErrNameTaken      = errors.New("a skill with this name already exists in this organization")
	ErrInvalidProfic  = errors.New("proficiency must be one of beginner, intermediate, advanced, expert")
	ErrInvalidDate    = errors.New("dates must be in YYYY-MM-DD format")
	ErrSkillInactive  = errors.New("this skill is not active")
	ErrAlreadyGranted = errors.New("this employee already has this skill recorded")
	ErrAccessDenied   = errors.New("you do not have access to this employee's skills")
	ErrSkillInUse     = errors.New("this skill is recorded against employees and cannot be deleted")
)
