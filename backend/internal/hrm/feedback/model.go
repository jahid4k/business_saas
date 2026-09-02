// backend/internal/hrm/feedback/model.go
package feedback

import (
	"context"
	"errors"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/platform/forms"
)

// Package feedback implements HRM 360-degree feedback (Phase 5C part 1).
//
// It is a separate package from internal/hrm/performance rather than a sixth
// sub-feature quartet inside it, because performance's composite Repository
// reached 58 methods in Phase 5B — the ~60 threshold its own package doc
// records as the point to split. 360 feedback shares no query surface with
// goals or appraisals; it resolves the employee facts it needs with its own
// small query, the internal/hrm/onboarding precedent, keeping the dependency
// graph free of a performance ↔ feedback edge.
//
// ── The anonymity contract ───────────────────────────────────────────────
//
// This package's whole reason for existing carefully is that respondents are
// promised anonymity, and the codebase already contains one cautionary
// example of that promise being documented and never implemented:
// hrm_complaints.is_anonymous is a stored boolean with a COMMENT describing
// the protection it provides, which no handler, service or repository ever
// branches on.
//
// Three structural decisions keep this module from becoming the second:
//
//  1. Anonymity is DERIVED, never stored. Relationship.IsAnonymous() is the
//     single source of truth. There is no is_anonymous column to set to a
//     value that lies about what the system does.
//
//  2. Identity and content are returned by DIFFERENT methods with DIFFERENT
//     types that share no field. RequestSummary carries who was asked and
//     never an answer; AnonymousResponse carries answers and has no field
//     capable of naming anyone. Neither is the other with fields blanked —
//     a separate type means no column added later can leak through it. The
//     performance.GoalRef precedent, applied to a stronger requirement.
//
//  3. A form instance id is NEVER handed to a subject for someone else's
//     response. This is the one leak that lives outside this module:
//     platform_form_instances stores respondent_user_id, so an id plus
//     GET /forms/instances/:id defeats everything above. The service reads
//     instances server-side through the FormReader interface and returns
//     answer content, never a reference.
//
// Suppression is evaluated PER RELATIONSHIP GROUP, not across the cycle.
// Five responses of which exactly one is a direct report still identify that
// direct report the moment a "direct reports said" breakdown renders.

const dateLayout = "2006-01-02"

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

// ── Relationship ─────────────────────────────────────────────────────────────

// Relationship is the respondent's relation to the subject. It is the input
// to the anonymity policy, which is why it is a CHECK-constrained column and
// not free text.
type Relationship string

const (
	RelationshipSelf         Relationship = "self"
	RelationshipManager      Relationship = "manager"
	RelationshipPeer         Relationship = "peer"
	RelationshipDirectReport Relationship = "direct_report"
	RelationshipExternal     Relationship = "external"
)

func (r Relationship) IsValid() bool {
	switch r {
	case RelationshipSelf, RelationshipManager, RelationshipPeer,
		RelationshipDirectReport, RelationshipExternal:
		return true
	}
	return false
}

// IsAnonymous reports whether responses in this relationship group are shown
// to the subject without attribution.
//
// self and manager are ATTRIBUTED BY NATURE, not by oversight. A subject
// knows what they themselves wrote, and knows who their own manager is. There
// is exactly one manager, so "anonymous manager feedback" identifies the
// manager with certainty while pretending otherwise — and suppressing it
// below a threshold it can never reach would only make the most actionable
// feedback in the cycle permanently unreadable.
//
// The tempting change here is to anonymise every group uniformly. It looks
// like a hardening and is a regression: it adds no privacy and removes the
// feature.
func (r Relationship) IsAnonymous() bool {
	switch r {
	case RelationshipSelf, RelationshipManager:
		return false
	}
	return true
}

// ── Statuses ─────────────────────────────────────────────────────────────────

type CycleStatus string

const (
	CycleDraft  CycleStatus = "draft"
	CycleActive CycleStatus = "active"
	CycleClosed CycleStatus = "closed"
)

func (s CycleStatus) IsValid() bool {
	switch s {
	case CycleDraft, CycleActive, CycleClosed:
		return true
	}
	return false
}

type RequestStatus string

const (
	RequestPending   RequestStatus = "pending"
	RequestSubmitted RequestStatus = "submitted"
	RequestDeclined  RequestStatus = "declined"
	RequestCancelled RequestStatus = "cancelled"
)

// ── Records ──────────────────────────────────────────────────────────────────

type Cycle struct {
	ID          string    `db:"id"          json:"id"`
	PublicID    string    `db:"public_id"   json:"public_id"`
	OrgID       string    `db:"org_id"      json:"org_id"`
	Name        string    `db:"name"        json:"name"`
	Description *string   `db:"description" json:"description,omitempty"`
	PeriodStart time.Time `db:"period_start" json:"period_start"`
	PeriodEnd   time.Time `db:"period_end"   json:"period_end"`

	FormTemplateID string `db:"form_template_id" json:"form_template_id"`
	// MinResponses is the per-relationship-group suppression threshold, not a
	// cycle-wide total. Per-cycle so a policy number an org may legitimately
	// differ on stays data — the hrm_goal_cycles.weight_target precedent.
	MinResponses int `db:"min_responses" json:"min_responses"`

	Status    CycleStatus `db:"status"     json:"status"`
	ClosedAt  *time.Time  `db:"closed_at"  json:"closed_at,omitempty"`
	CreatedBy string      `db:"created_by" json:"created_by"`
	CreatedAt time.Time   `db:"created_at" json:"created_at"`
	UpdatedAt time.Time   `db:"updated_at" json:"updated_at"`
}

// Request is the FULL row, used by the service internally and by the
// respondent reading their own request. It is deliberately NOT what a subject
// receives — see RequestSummary and AnonymousResponse.
type Request struct {
	ID                string `db:"id"         json:"id"`
	PublicID          string `db:"public_id"  json:"public_id"`
	OrgID             string `db:"org_id"     json:"org_id"`
	CycleID           string `db:"cycle_id"   json:"cycle_id"`
	SubjectEmployeeID string `db:"subject_employee_id" json:"subject_employee_id"`

	RespondentEmployeeID *string `db:"respondent_employee_id" json:"respondent_employee_id,omitempty"`
	RespondentUserID     *string `db:"respondent_user_id"     json:"respondent_user_id,omitempty"`
	RespondentName       string  `db:"respondent_name"        json:"respondent_name"`
	RespondentEmail      *string `db:"respondent_email"       json:"respondent_email,omitempty"`

	Relationship Relationship `db:"relationship" json:"relationship"`

	// FormInstanceID is read SERVER-SIDE ONLY. Serialising it to a subject
	// defeats the anonymity contract, because the form engine stores
	// respondent_user_id on the instance. json:"-" is the enforcement.
	FormInstanceID *string `db:"form_instance_id" json:"-"`

	Status        RequestStatus `db:"status"         json:"status"`
	SubmittedAt   *time.Time    `db:"submitted_at"   json:"submitted_at,omitempty"`
	DeclinedAt    *time.Time    `db:"declined_at"    json:"declined_at,omitempty"`
	DeclineReason *string       `db:"decline_reason" json:"decline_reason,omitempty"`

	RequestedBy string    `db:"requested_by" json:"requested_by"`
	CreatedAt   time.Time `db:"created_at"   json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"   json:"updated_at"`
}

// ── The two read paths ───────────────────────────────────────────────────────

// RequestSummary is the COORDINATION view: who was asked, and whether they
// have responded. It carries identity and deliberately carries NO answer
// content — chasing a non-responder never requires reading what anyone said.
//
// Gated on hrm.feedback.coordinate.
type RequestSummary struct {
	ID             string        `json:"id"`
	PublicID       string        `json:"public_id"`
	RespondentName string        `json:"respondent_name"`
	Relationship   Relationship  `json:"relationship"`
	Status         RequestStatus `json:"status"`
	SubmittedAt    *time.Time    `json:"submitted_at,omitempty"`
}

// AnonymousResponse is the SUBJECT view: what was said. It has no field
// capable of naming anyone — no respondent id, no name, no form instance
// reference, no timestamp precise enough to correlate against a coordination
// listing.
//
// It is NOT a Request with fields blanked. A separate type means a column
// added to hrm_feedback_requests in a later phase cannot leak through this
// path by being picked up in a shared struct.
type AnonymousResponse struct {
	Relationship Relationship      `json:"relationship"`
	Answers      []AnonymousAnswer `json:"answers"`
}

// AnonymousAnswer is one question and its answer, carrying the question
// snapshot the form engine froze and nothing about who wrote it.
type AnonymousAnswer struct {
	QuestionText  string           `json:"question_text"`
	QuestionType  string           `json:"question_type"`
	AnswerText    *string          `json:"answer_text,omitempty"`
	AnswerNumber  *decimal.Decimal `json:"answer_number,omitempty"`
	AnswerBoolean *bool            `json:"answer_boolean,omitempty"`
	AnswerDate    *string          `json:"answer_date,omitempty"`
	AnswerOptions []string         `json:"answer_options,omitempty"`
}

// RelationshipGroup is one relationship's results for a subject.
//
// When Suppressed is true, EVERY other content field is empty — including
// ResponseCount. The count is itself a signal: telling a subject "one peer
// responded but it is hidden" combined with any knowledge of who was asked
// narrows it to a person. A suppressed group reports only that it is
// suppressed, and the threshold that was not met.
type RelationshipGroup struct {
	Relationship Relationship `json:"relationship"`
	Suppressed   bool         `json:"suppressed"`
	// MinResponses is echoed so a client can explain WHY a group is hidden
	// without needing the cycle record.
	MinResponses int `json:"min_responses"`

	ResponseCount int                 `json:"response_count,omitempty"`
	AverageScore  *decimal.Decimal    `json:"average_score,omitempty"`
	Responses     []AnonymousResponse `json:"responses,omitempty"`
}

// Aggregate is everything a subject may see about themselves in one cycle.
type Aggregate struct {
	CycleID           string              `json:"cycle_id"`
	SubjectEmployeeID string              `json:"subject_employee_id"`
	Groups            []RelationshipGroup `json:"groups"`
	// TotalResponses counts only responses in groups that actually rendered,
	// so it cannot be differenced against a suppressed group to infer its size.
	TotalResponses int `json:"total_responses"`
}

// ── Caller / narrow interfaces ───────────────────────────────────────────────

// Caller carries the authorization facts the handler has already resolved, so
// the service needs no authz dependency of its own and stays testable against
// stubs alone. The performance.Caller precedent.
type Caller struct {
	UserID string
	// Tier comes from authzSvc.ResolveScope(ctx, userID, orgID, "hrm.feedback").
	Tier authz.Scope
	// CanCoordinate reports whether the caller holds hrm.feedback.coordinate,
	// which permits the identity-bearing read path. It never unlocks answer
	// content: the two paths are different methods returning different types.
	CanCoordinate bool
	// CanManage reports whether the caller holds hrm.feedback.manage.
	CanManage bool
}

// RecordAuthorizer is the one-method slice of *scope.Resolver this package
// needs. An interface purely so tests can stub it; *scope.Resolver satisfies
// it structurally, so main.go passes the resolver directly.
type RecordAuthorizer interface {
	AuthorizeRecordAccess(ctx context.Context, tier authz.Scope, orgID, callerUserID, recordEmployeeRef string) (bool, error)
}

// FormReader is the slice of the form engine this package consumes. Narrow
// and consumer-owned: platform/forms never learns that 360 feedback exists.
//
// Instantiate is called when a request is created; GetInstance and
// ScoreInstance are called SERVER-SIDE while assembling an aggregate, and
// their results are stripped of identity before anything is returned.
type FormReader interface {
	Instantiate(ctx context.Context, orgID, templateRef string, subj forms.SubjectContext) (*forms.InstanceWithResponses, error)
	GetInstance(ctx context.Context, orgID, ref string) (*forms.InstanceWithResponses, error)
	ScoreInstance(ctx context.Context, orgID, ref string) (forms.Score, error)
}

// EmployeeSubject is the slice of an employee row this package needs to
// address a feedback request. Owned here rather than imported from
// internal/hrm/employees — the onboarding precedent.
type EmployeeSubject struct {
	EmployeeID  string
	DisplayName string
	Email       *string
	UserID      *string
}

// ── Requests ─────────────────────────────────────────────────────────────────

type CreateCycleRequest struct {
	Name           string  `json:"name"`
	Description    *string `json:"description"`
	PeriodStart    string  `json:"period_start"`
	PeriodEnd      string  `json:"period_end"`
	FormTemplateID string  `json:"form_template_id"`
	MinResponses   *int    `json:"min_responses"`
}

type UpdateCycleRequest struct {
	Name         *string `json:"name"`
	Description  *string `json:"description"`
	PeriodStart  *string `json:"period_start"`
	PeriodEnd    *string `json:"period_end"`
	MinResponses *int    `json:"min_responses"`
}

// CreateRequestsRequest asks a batch of people about one subject. Batched
// rather than one-at-a-time because a 360 is meaningless as a single ask, and
// a partial batch is a coordination problem the caller must see as a whole.
type CreateRequestsRequest struct {
	SubjectEmployeeID string           `json:"subject_employee_id"`
	Respondents       []RespondentSpec `json:"respondents"`
}

// RespondentSpec identifies one person to ask. EmployeeID is used for
// internal respondents; Email + Name for external ones. Exactly one form is
// required, which the service checks.
type RespondentSpec struct {
	EmployeeID   *string      `json:"employee_id"`
	Email        *string      `json:"email"`
	Name         *string      `json:"name"`
	Relationship Relationship `json:"relationship"`
}

type DeclineRequest struct {
	Reason *string `json:"reason"`
}

// ── Filters and responses ────────────────────────────────────────────────────

type CycleListFilter struct {
	Status string
	Limit  int
	Offset int
}

func (f *CycleListFilter) Normalise() {
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

type CycleListResponse struct {
	Cycles []*Cycle `json:"cycles"`
	Total  int      `json:"total"`
	Limit  int      `json:"limit"`
	Offset int      `json:"offset"`
}

// RequestListFilter drives the COORDINATION path only. It has no field that
// could ask for answer content, which is what keeps the two paths from
// converging on one query with a flag.
type RequestListFilter struct {
	CycleID           string
	SubjectEmployeeID string
	Status            string
	Limit             int
	Offset            int

	Scope        authz.Scope
	CallerUserID string
}

func (f *RequestListFilter) Normalise() {
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

type RequestListResponse struct {
	Requests []*RequestSummary `json:"requests"`
	Total    int               `json:"total"`
	Limit    int               `json:"limit"`
	Offset   int               `json:"offset"`
}

// MyRequestsResponse is the respondent's own inbox. A respondent sees their
// OWN request in full, including the form instance id they must answer —
// that is not a leak, it is their own row.
type MyRequestsResponse struct {
	Requests []*MyRequest `json:"requests"`
	Total    int          `json:"total"`
}

// MyRequest is one ask addressed to the caller. It exposes the form instance
// id, uniquely among the response types, because the caller is the person
// meant to fill it in.
type MyRequest struct {
	ID                string        `json:"id"`
	PublicID          string        `json:"public_id"`
	CycleID           string        `json:"cycle_id"`
	CycleName         string        `json:"cycle_name"`
	SubjectName       string        `json:"subject_name"`
	Relationship      Relationship  `json:"relationship"`
	Status            RequestStatus `json:"status"`
	FormInstanceID    *string       `json:"form_instance_id,omitempty"`
	PeriodEnd         time.Time     `json:"period_end"`
	SubmittedAt       *time.Time    `json:"submitted_at,omitempty"`
	SubjectEmployeeID string        `json:"subject_employee_id"`
}

// ── Errors ───────────────────────────────────────────────────────────────────

var (
	ErrCycleNotFound    = errors.New("feedback cycle not found")
	ErrRequestNotFound  = errors.New("feedback request not found")
	ErrEmployeeNotFound = errors.New("employee not found")

	ErrCycleNameRequired = errors.New("name is required")
	ErrCycleNameTaken    = errors.New("a feedback cycle with this name already exists in this organization")
	ErrTemplateRequired  = errors.New("form_template_id is required")
	ErrInvalidPeriod     = errors.New("period_end must be on or after period_start")
	ErrInvalidDate       = errors.New("dates must be in YYYY-MM-DD format")
	ErrMinResponses      = errors.New("min_responses must be at least 1")

	ErrCycleNotActive = errors.New("the feedback cycle is not active")
	ErrCycleClosed    = errors.New("the feedback cycle is closed")
	ErrCycleStatus    = errors.New("action not allowed in the cycle's current status")

	ErrNoRespondents       = errors.New("at least one respondent is required")
	ErrInvalidRelationship = errors.New("relationship must be one of self, manager, peer, direct_report, external")
	ErrRespondentRequired  = errors.New("each respondent needs either an employee_id or an email and name")
	ErrSelfMismatch        = errors.New("a self relationship requires the respondent to be the subject")
	ErrDuplicateRequest    = errors.New("this respondent has already been asked about this subject in this cycle")

	ErrAccessDenied     = errors.New("you do not have access to this feedback")
	ErrNotRespondent    = errors.New("this feedback request is not addressed to you")
	ErrAlreadySubmitted = errors.New("this feedback request has already been submitted")
	ErrRequestClosed    = errors.New("this feedback request is no longer open")
)
