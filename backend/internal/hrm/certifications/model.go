// backend/internal/hrm/certifications/model.go
package certifications

import (
	"context"
	"errors"
	"time"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/hrm/skills"
)

// Package certifications implements HRM certification tracking and the
// expiry sweep (Phase 6B).
//
// Its own package rather than part of internal/hrm/learning: that package's
// composite Repository reached 53 methods in 6A, against the ~60 threshold
// Phase 5A recorded as the point to split. It also has a genuinely different
// lifecycle — a professional licence obtained externally has no course behind
// it at all, which is why enrollment_id is nullable and why this is not
// "learning, part two".
//
// The build plan calls the EXPIRY SWEEP "the highest-value feature here". It
// is a scheduler job (internal/platform/scheduler, Phase 0.2) rather than a
// query somebody remembers to run — see SweepExpiries.

const dateLayout = "2006-01-02"

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

// ExpiryWindowDays is how far ahead the sweep looks when marking credentials
// as expiring. Thirty days is enough notice to re-certify without the warning
// becoming background noise.
const ExpiryWindowDays = 30

// Status tracks a credential through its life. 'expiring' is set by the sweep
// rather than computed on read, deliberately: it is what makes "tell me who
// needs chasing" a plain indexed query instead of a scan with date arithmetic.
type Status string

const (
	StatusActive   Status = "active"
	StatusExpiring Status = "expiring"
	StatusExpired  Status = "expired"
	StatusRevoked  Status = "revoked"
)

func (s Status) IsValid() bool {
	switch s {
	case StatusActive, StatusExpiring, StatusExpired, StatusRevoked:
		return true
	}
	return false
}

// IsLive mirrors the partial unique index uq_hrm_ecrt_employee_cert_live,
// which is what actually enforces one live credential per employee per
// certification. The two must list the same statuses, so this is the shared
// definition.
func (s Status) IsLive() bool { return s == StatusActive || s == StatusExpiring }

// ── Records ──────────────────────────────────────────────────────────────────

type Certification struct {
	ID          string  `db:"id"           json:"id"`
	PublicID    string  `db:"public_id"    json:"public_id"`
	OrgID       string  `db:"org_id"       json:"org_id"`
	Name        string  `db:"name"         json:"name"`
	Description *string `db:"description"  json:"description,omitempty"`
	IssuingBody *string `db:"issuing_body" json:"issuing_body,omitempty"`

	// NULL means the credential does not expire — distinct from expiring
	// today, and the sweep must keep them distinguishable.
	ValidityMonths *int `db:"validity_months" json:"validity_months,omitempty"`
	// Optional: the course that grants it.
	CourseID *string `db:"course_id" json:"course_id,omitempty"`
	// Optional: the skill this credential demonstrates. Issuing the credential
	// records that skill against the employee — the in-phase consumer that
	// justifies building the taxonomy in Phase 6 rather than Phase 10.
	SkillID *string `db:"skill_id" json:"skill_id,omitempty"`

	IsActive  bool      `db:"is_active"  json:"is_active"`
	CreatedBy string    `db:"created_by" json:"created_by"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type EmployeeCertification struct {
	ID              string  `db:"id"               json:"id"`
	PublicID        string  `db:"public_id"        json:"public_id"`
	OrgID           string  `db:"org_id"           json:"org_id"`
	EmployeeID      string  `db:"employee_id"      json:"employee_id"`
	CertificationID string  `db:"certification_id" json:"certification_id"`
	EnrollmentID    *string `db:"enrollment_id"    json:"enrollment_id,omitempty"`

	CredentialID *string    `db:"credential_id" json:"credential_id,omitempty"`
	IssuedOn     time.Time  `db:"issued_on"     json:"issued_on"`
	ExpiresAt    *time.Time `db:"expires_at"    json:"expires_at,omitempty"`

	Status           Status     `db:"status"             json:"status"`
	ExpiryNotifiedAt *time.Time `db:"expiry_notified_at" json:"expiry_notified_at,omitempty"`

	Notes     *string   `db:"notes"      json:"notes,omitempty"`
	IssuedBy  *string   `db:"issued_by"  json:"issued_by,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`

	// Joined for display, never authoritative.
	CertificationName *string `db:"certification_name" json:"certification_name,omitempty"`
}

// DaysUntilExpiry is negative once the credential has lapsed, which is the
// state somebody most needs to see. Nil when it never expires.
func (e *EmployeeCertification) DaysUntilExpiry(now time.Time) *int {
	if e.ExpiresAt == nil {
		return nil
	}
	d := int(dateOnly(*e.ExpiresAt).Sub(dateOnly(now)).Hours() / 24)
	return &d
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// ── Caller / narrow interfaces ───────────────────────────────────────────────

type Caller struct {
	UserID string
	// Tier comes from authzSvc.ResolveScope(ctx, userID, orgID, "hrm.certifications").
	Tier authz.Scope
	// CanManage reports whether the caller holds hrm.certifications.manage —
	// issuing and revoking. Issuing a credential is an assertion the
	// organization stands behind, which is why 'manager' does not hold it.
	CanManage bool
}

type RecordAuthorizer interface {
	AuthorizeRecordAccess(ctx context.Context, tier authz.Scope, orgID, callerUserID, recordEmployeeRef string) (bool, error)
}

// SkillGranter is the one-method slice of the skills service this package
// uses when a certification carries a skill.
//
// It names skills' own types rather than mirroring them behind aliases, so
// skills.Service satisfies it structurally with no adapter in main.go. The
// import direction is deliberate and safe: internal/hrm/skills is a SHARED
// taxonomy meant to be consumed — by Phase 10 succession as well as here —
// and it imports nothing from this package, so there is no cycle.
type SkillGranter interface {
	GrantFromSource(ctx context.Context, orgID, employeeID, skillID string, source skills.Source, sourceID string) (*skills.EmployeeSkill, error)
}

// ── Requests ─────────────────────────────────────────────────────────────────

type CreateCertificationRequest struct {
	Name           string  `json:"name"`
	Description    *string `json:"description"`
	IssuingBody    *string `json:"issuing_body"`
	ValidityMonths *int    `json:"validity_months"`
	CourseID       *string `json:"course_id"`
	SkillID        *string `json:"skill_id"`
}

type UpdateCertificationRequest struct {
	Name           *string `json:"name"`
	Description    *string `json:"description"`
	IssuingBody    *string `json:"issuing_body"`
	ValidityMonths *int    `json:"validity_months"`
	CourseID       *string `json:"course_id"`
	SkillID        *string `json:"skill_id"`
	IsActive       *bool   `json:"is_active"`
}

// IssueRequest records a credential against an employee. ExpiresAt is
// optional: when omitted the service derives it from the certification's
// validity_months, which is the normal path.
type IssueRequest struct {
	EmployeeID      string  `json:"employee_id"`
	CertificationID string  `json:"certification_id"`
	CredentialID    *string `json:"credential_id"`
	IssuedOn        string  `json:"issued_on"`
	ExpiresAt       *string `json:"expires_at"`
	EnrollmentID    *string `json:"enrollment_id"`
	Notes           *string `json:"notes"`
}

type UpdateEmployeeCertificationRequest struct {
	CredentialID *string `json:"credential_id"`
	ExpiresAt    *string `json:"expires_at"`
	Notes        *string `json:"notes"`
}

type RevokeRequest struct {
	Reason *string `json:"reason"`
}

// ── Filters and responses ────────────────────────────────────────────────────

type CertificationListFilter struct {
	IsActive *bool
	Search   string
	Limit    int
	Offset   int
}

func (f *CertificationListFilter) Normalise() {
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

type CertificationListResponse struct {
	Certifications []*Certification `json:"certifications"`
	Total          int              `json:"total"`
	Limit          int              `json:"limit"`
	Offset         int              `json:"offset"`
}

type EmployeeCertificationListFilter struct {
	EmployeeID      string
	CertificationID string
	Status          string
	// ExpiringWithinDays narrows to credentials lapsing soon — the query a
	// compliance dashboard actually asks.
	ExpiringWithinDays int
	Limit              int
	Offset             int

	Scope        authz.Scope
	CallerUserID string
}

func (f *EmployeeCertificationListFilter) Normalise() {
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

type EmployeeCertificationListResponse struct {
	Certifications []*EmployeeCertification `json:"employee_certifications"`
	Total          int                      `json:"total"`
	Limit          int                      `json:"limit"`
	Offset         int                      `json:"offset"`
}

// SweepResult is what the scheduler job reports back. Counts rather than rows,
// because the job's contract with the scheduler is an items-processed integer.
type SweepResult struct {
	MarkedExpiring int
	MarkedExpired  int
}

// Total is what scheduler.JobFunc returns as itemsProcessed.
func (r SweepResult) Total() int { return r.MarkedExpiring + r.MarkedExpired }

// ── Errors ───────────────────────────────────────────────────────────────────

var (
	ErrNotFound             = errors.New("certification not found")
	ErrEmployeeCertNotFound = errors.New("employee certification not found")
	ErrEmployeeNotFound     = errors.New("employee not found")

	ErrNameRequired      = errors.New("name is required")
	ErrNameTaken         = errors.New("a certification with this name already exists in this organization")
	ErrInvalidDate       = errors.New("dates must be in YYYY-MM-DD format")
	ErrExpiryBeforeIssue = errors.New("expires_at must be on or after issued_on")
	ErrInvalidValidity   = errors.New("validity_months must be greater than zero")

	ErrCertInactive   = errors.New("this certification is not active")
	ErrAlreadyHeld    = errors.New("this employee already holds a live credential for this certification")
	ErrAlreadyRevoked = errors.New("this credential has already been revoked")
	ErrCertInUse      = errors.New("this certification has been issued to employees and cannot be deleted")

	ErrAccessDenied = errors.New("you do not have access to this employee's certifications")
)
