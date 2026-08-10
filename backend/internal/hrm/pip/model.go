// backend/internal/hrm/pip/model.go
package pip

import (
	"context"
	"errors"
	"time"

	"github.com/mridha/businesssaas/internal/authz"
)

// Package pip implements HRM performance improvement plans (Phase 5C part 2).
//
// A separate package from internal/hrm/performance for the same reason
// internal/hrm/feedback is: performance's composite Repository reached 58
// methods in Phase 5B, the ~60 threshold its own package doc records as the
// point to split. It is also separate from feedback, because PIP is the only
// module here with an OUTBOUND edge — to terminations — and bundling the two
// would drag 360 feedback into that dependency for no reason.
//
// ── The failed-PIP handoff ───────────────────────────────────────────────
//
// Closing a PIP as 'failed' creates a DRAFT termination and stops. It does
// not terminate anyone, and it does not submit the termination for approval.
//
// That is not timidity, it is this codebase's "no implicit state machine"
// rule. hrm_terminations already owns a draft → pending_approval → approved →
// applied lifecycle with an approval chain behind it. A PIP that advanced
// past draft would route around the approval that exists specifically to gate
// dismissals — the single control most worth not bypassing in the entire HRM
// surface.
//
// The handoff uses the consumer-owned narrow interface pattern:
// TerminationCreator and DraftTerminationRequest are declared HERE, and
// terminations.Service satisfies TerminationCreator. The dependency edge
// therefore runs terminations → pip, never pip → terminations, matching the
// recruitment.EmployeeCreator / employees.Service seam exactly.

const dateLayout = "2006-01-02"

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

// ── Status and outcome ───────────────────────────────────────────────────────

// Status is where the plan is in its lifecycle. Deliberately separate from
// Outcome, which is how it ended — fusing them yields one CHECK with seven
// values of which several are illegal in combination with the dates.
type Status string

const (
	StatusDraft     Status = "draft"
	StatusActive    Status = "active"
	StatusExtended  Status = "extended"
	StatusClosed    Status = "closed"
	StatusCancelled Status = "cancelled"
)

func (s Status) IsValid() bool {
	switch s {
	case StatusDraft, StatusActive, StatusExtended, StatusClosed, StatusCancelled:
		return true
	}
	return false
}

// IsOpen reports whether the plan is still running. Matches the partial
// unique index uq_hrm_pip_employee_open, which is what actually enforces one
// open plan per employee.
func (s Status) IsOpen() bool {
	return s == StatusDraft || s == StatusActive || s == StatusExtended
}

// Outcome is how a plan ended. NULL until it does.
type Outcome string

const (
	OutcomeSuccessful Outcome = "successful"
	OutcomeFailed     Outcome = "failed"
	OutcomeAbandoned  Outcome = "abandoned"
)

func (o Outcome) IsValid() bool {
	switch o {
	case OutcomeSuccessful, OutcomeFailed, OutcomeAbandoned:
		return true
	}
	return false
}

type EntryType string

const (
	EntryReview    EntryType = "review"
	EntryExtension EntryType = "extension"
	EntryClosure   EntryType = "closure"
)

type Progress string

const (
	ProgressOnTrack  Progress = "on_track"
	ProgressPartial  Progress = "partial"
	ProgressOffTrack Progress = "off_track"
)

func (p Progress) IsValid() bool {
	switch p {
	case ProgressOnTrack, ProgressPartial, ProgressOffTrack:
		return true
	}
	return false
}

// ── Records ──────────────────────────────────────────────────────────────────

type PIP struct {
	ID         string `db:"id"          json:"id"`
	PublicID   string `db:"public_id"   json:"public_id"`
	OrgID      string `db:"org_id"      json:"org_id"`
	EmployeeID string `db:"employee_id" json:"employee_id"`
	// Frozen at creation. A reorg must not silently hand someone else's
	// dismissal process to a manager who never opened it — the
	// hrm_appraisals.manager_employee_id_snapshot precedent.
	ManagerEmployeeID *string `db:"manager_employee_id" json:"manager_employee_id,omitempty"`

	Title           string  `db:"title"            json:"title"`
	Concerns        string  `db:"concerns"         json:"concerns"`
	SuccessCriteria string  `db:"success_criteria" json:"success_criteria"`
	SupportProvided *string `db:"support_provided" json:"support_provided,omitempty"`

	StartDate time.Time `db:"start_date" json:"start_date"`
	EndDate   time.Time `db:"end_date"   json:"end_date"`
	// Frozen at creation so extensions stay legible after the fact. A PIP
	// whose end date silently moves is the documented failure mode of the
	// whole instrument.
	OriginalEndDate time.Time `db:"original_end_date" json:"original_end_date"`

	Status  Status   `db:"status"  json:"status"`
	Outcome *Outcome `db:"outcome" json:"outcome,omitempty"`

	// Set by the failed-PIP handoff. A DRAFT termination, never an applied one.
	TerminationID *string `db:"termination_id" json:"termination_id,omitempty"`

	ClosedAt  *time.Time `db:"closed_at"  json:"closed_at,omitempty"`
	ClosedBy  *string    `db:"closed_by"  json:"closed_by,omitempty"`
	CreatedBy string     `db:"created_by" json:"created_by"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
}

// WasExtended reports whether the end date has moved from where it started.
// Computed, never stored — the 00076 rule.
func (p *PIP) WasExtended() bool {
	return p.EndDate.After(p.OriginalEndDate)
}

// Checkin is one entry in a plan's append-only review history.
type Checkin struct {
	ID        string    `db:"id"         json:"id"`
	PublicID  string    `db:"public_id"  json:"public_id"`
	PIPID     string    `db:"pip_id"     json:"pip_id"`
	EntryType EntryType `db:"entry_type" json:"entry_type"`

	Progress *Progress `db:"progress" json:"progress,omitempty"`
	Note     string    `db:"note"     json:"note"`

	PreviousEndDate *time.Time `db:"previous_end_date" json:"previous_end_date,omitempty"`
	NewEndDate      *time.Time `db:"new_end_date"      json:"new_end_date,omitempty"`

	CheckedInBy *string   `db:"checked_in_by" json:"checked_in_by,omitempty"`
	CheckedInAt time.Time `db:"checked_in_at" json:"checked_in_at"`
	CreatedAt   time.Time `db:"created_at"    json:"created_at"`
}

// Detail carries the plan plus its history and the derived facts a client
// would otherwise recompute.
type Detail struct {
	*PIP
	Checkins    []*Checkin `json:"checkins"`
	WasExtended bool       `json:"was_extended"`
	// DaysRemaining is negative once the end date has passed, which is the
	// state a manager most needs to see.
	DaysRemaining int `json:"days_remaining"`
}

// ── The terminations handoff ─────────────────────────────────────────────────

// TerminationCreator is the one-method slice of terminations.Service that the
// failed-PIP handoff needs.
//
// Declared HERE, with its own request type, so the dependency edge runs
// terminations → pip rather than pip → terminations. That is the
// recruitment.EmployeeCreator pattern: the ORCHESTRATOR owns the interface
// and the types crossing it, and the PROVIDER imports the orchestrator.
//
// It creates a DRAFT. Submitting it for approval and applying it stay with
// HR, on the existing termination endpoints.
type TerminationCreator interface {
	CreateDraftFromPIP(ctx context.Context, orgID, employeeID, createdBy string, req DraftTerminationRequest) (terminationID string, err error)
}

// DraftTerminationRequest is what the PIP service hands TerminationCreator.
// Deliberately minimal: a failed PIP knows the employee, the date the plan
// ran out, and why. Severance, rehire eligibility and notice are decisions HR
// makes on the termination itself, and pre-filling them here would make a
// draft look more decided than it is.
type DraftTerminationRequest struct {
	// TerminationDate defaults to the plan's end date at the call site.
	TerminationDate string
	LastWorkingDate string
	// Reason carries the PIP's public id so the termination is traceable back
	// to the process that produced it.
	Reason string
}

// ── Caller / narrow interfaces ───────────────────────────────────────────────

// Caller carries the authorization facts the handler has already resolved.
// The performance.Caller precedent.
type Caller struct {
	UserID string
	// Tier comes from authzSvc.ResolveScope(ctx, userID, orgID, "hrm.pips").
	Tier authz.Scope
	// CanManage reports whether the caller holds hrm.pips.manage.
	CanManage bool
	// CanClose reports whether the caller holds hrm.pips.close — a separate
	// key because closing as 'failed' is what triggers the draft-termination
	// handoff. 'manager' holds manage but NOT close.
	CanClose bool
}

// RecordAuthorizer is the one-method slice of *scope.Resolver this package
// needs. An interface purely so tests can stub it; *scope.Resolver satisfies
// it structurally.
type RecordAuthorizer interface {
	AuthorizeRecordAccess(ctx context.Context, tier authz.Scope, orgID, callerUserID, recordEmployeeRef string) (bool, error)
}

// EmployeeRef is the slice of an employee row this package needs. Owned here
// rather than imported from internal/hrm/employees — the onboarding
// precedent, which keeps the dependency graph free of an employees ↔ pip edge.
type EmployeeRef struct {
	EmployeeID        string
	DisplayName       string
	ManagerEmployeeID *string
}

// ── Requests ─────────────────────────────────────────────────────────────────

type CreateRequest struct {
	EmployeeID      string  `json:"employee_id"`
	Title           string  `json:"title"`
	Concerns        string  `json:"concerns"`
	SuccessCriteria string  `json:"success_criteria"`
	SupportProvided *string `json:"support_provided"`
	StartDate       string  `json:"start_date"`
	EndDate         string  `json:"end_date"`
}

type UpdateRequest struct {
	Title           *string `json:"title"`
	Concerns        *string `json:"concerns"`
	SuccessCriteria *string `json:"success_criteria"`
	SupportProvided *string `json:"support_provided"`
	StartDate       *string `json:"start_date"`
}

type CheckinRequest struct {
	Progress *Progress `json:"progress"`
	Note     string    `json:"note"`
}

// ExtendRequest moves the end date out. Recorded as a check-in of type
// 'extension' in the same transaction, so an extension can never happen
// without a written reason.
type ExtendRequest struct {
	NewEndDate string `json:"new_end_date"`
	Note       string `json:"note"`
}

// CloseRequest ends the plan. Outcome is mandatory: a PIP that closes without
// one is a record nobody can act on or defend.
type CloseRequest struct {
	Outcome Outcome `json:"outcome"`
	Note    string  `json:"note"`
	// LastWorkingDate is used only when Outcome is 'failed', for the draft
	// termination. Defaults to the plan's end date.
	LastWorkingDate *string `json:"last_working_date"`
}

// ── Filters and responses ────────────────────────────────────────────────────

type ListFilter struct {
	EmployeeID string
	Status     string
	Outcome    string
	Limit      int
	Offset     int

	Scope        authz.Scope
	CallerUserID string
}

func (f *ListFilter) Normalise() {
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

type ListResponse struct {
	PIPs   []*PIP `json:"pips"`
	Total  int    `json:"total"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

// ── Errors ───────────────────────────────────────────────────────────────────

var (
	ErrNotFound         = errors.New("performance improvement plan not found")
	ErrEmployeeNotFound = errors.New("employee not found")

	ErrTitleRequired      = errors.New("title is required")
	ErrConcernsRequired   = errors.New("concerns is required")
	ErrCriteriaRequired   = errors.New("success_criteria is required — a plan without stated criteria is unmeetable by construction")
	ErrInvalidDate        = errors.New("dates must be in YYYY-MM-DD format")
	ErrInvalidPeriod      = errors.New("end_date must be on or after start_date")
	ErrNoteRequired       = errors.New("a note is required")
	ErrInvalidProgress    = errors.New("progress must be one of on_track, partial, off_track")
	ErrInvalidOutcome     = errors.New("outcome must be one of successful, failed, abandoned")
	ErrOutcomeRequired    = errors.New("an outcome is required to close a plan")
	ErrExtensionBackwards = errors.New("an extension must move the end date later, not earlier")

	ErrAlreadyOpen   = errors.New("this employee already has an open performance improvement plan")
	ErrNotOpen       = errors.New("this plan is closed and can no longer be changed")
	ErrNotActive     = errors.New("this plan must be active before it can be checked in on, extended or closed")
	ErrAlreadyClosed = errors.New("this plan has already been closed")

	ErrAccessDenied = errors.New("you do not have access to this performance improvement plan")
	ErrCloseDenied  = errors.New("closing a plan requires hrm.pips.close")

	// ErrTerminationHandoff wraps a failure in the draft-termination creation
	// so the caller can tell "the PIP closed but the draft did not appear"
	// from "the PIP did not close".
	ErrTerminationHandoff = errors.New("the plan was closed but the draft termination could not be created")
)
