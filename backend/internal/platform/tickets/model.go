// backend/internal/platform/tickets/model.go
package tickets

import (
	"context"
	"errors"
	"time"
)

// AccessDirectory is the minimal slice of authz.Service this package needs.
// Declared locally so this package gains no platform → authz import edge;
// authz.Service satisfies it structurally.
//
// ⚠ Parameter ORDER is load-bearing: satisfaction is structural, not
// declared, so these signatures must match authz.Service's exactly. The
// checklists.AccessDirectory / forms.AccessDirectory precedent.
//
// UserRoleName backs the sensitive-category assignee restriction — a
// harassment queue may only be assigned to holders of one named role.
type AccessDirectory interface {
	Can(ctx context.Context, userID, orgID, resource, action string) (bool, error)
	UserRoleName(ctx context.Context, orgID, userID string) (string, error)
}

// ── Categories ───────────────────────────────────────────────────────────────

type Category struct {
	ID             string    `db:"id"               json:"id"`
	PublicID       string    `db:"public_id"         json:"public_id"`
	OrgID          string    `db:"org_id"            json:"org_id"`
	Name           string    `db:"name"              json:"name"`
	Description    *string   `db:"description"       json:"description,omitempty"`
	IsSensitive    bool      `db:"is_sensitive"      json:"is_sensitive"`
	RestrictedRole *string   `db:"restricted_role"   json:"restricted_role,omitempty"`
	IsActive       bool      `db:"is_active"         json:"is_active"`
	CreatedBy      string    `db:"created_by"        json:"created_by"`
	CreatedAt      time.Time `db:"created_at"        json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"        json:"updated_at"`
}

type CreateCategoryRequest struct {
	Name           string  `json:"name"`
	Description    *string `json:"description"`
	IsSensitive    *bool   `json:"is_sensitive"`
	RestrictedRole *string `json:"restricted_role"`
}

// ── SLA policies ─────────────────────────────────────────────────────────────

type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityNormal Priority = "normal"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

func (p Priority) IsValid() bool {
	switch p {
	case PriorityLow, PriorityNormal, PriorityHigh, PriorityUrgent:
		return true
	}
	return false
}

type SLAPolicy struct {
	ID                   string    `db:"id"                       json:"id"`
	PublicID             string    `db:"public_id"                 json:"public_id"`
	OrgID                string    `db:"org_id"                    json:"org_id"`
	CategoryID           *string   `db:"category_id"               json:"category_id,omitempty"`
	Priority             Priority  `db:"priority"                  json:"priority"`
	FirstResponseMinutes int       `db:"first_response_minutes"    json:"first_response_minutes"`
	ResolutionMinutes    int       `db:"resolution_minutes"        json:"resolution_minutes"`
	CreatedBy            string    `db:"created_by"                json:"created_by"`
	CreatedAt            time.Time `db:"created_at"                json:"created_at"`
	UpdatedAt            time.Time `db:"updated_at"                json:"updated_at"`
}

type CreateSLAPolicyRequest struct {
	CategoryID           *string `json:"category_id"`
	Priority             string  `json:"priority"`
	FirstResponseMinutes int     `json:"first_response_minutes"`
	ResolutionMinutes    int     `json:"resolution_minutes"`
}

// ── Tickets ──────────────────────────────────────────────────────────────────

type Status string

const (
	StatusOpen      Status = "open"
	StatusAssigned  Status = "assigned"
	StatusPaused    Status = "paused"
	StatusResolved  Status = "resolved"
	StatusClosed    Status = "closed"
	StatusConverted Status = "converted"
	StatusCancelled Status = "cancelled"
)

// RequesterType is the polymorphic discriminator. Deliberately narrow —
// widening to 'contact' for customer-facing ticketing is a CHECK change, not
// a rewrite. See migration 00110's header.
type RequesterType string

const RequesterEmployee RequesterType = "employee"

func (r RequesterType) IsValid() bool { return r == RequesterEmployee }

// Ticket carries no elapsed/paused/breached columns — all three are computed
// from the SLA event ledger on every read. See migration 00110's header.
type Ticket struct {
	ID              string        `db:"id"                  json:"id"`
	PublicID        string        `db:"public_id"            json:"public_id"`
	OrgID           string        `db:"org_id"               json:"org_id"`
	RequesterType   RequesterType `db:"requester_type"       json:"requester_type"`
	RequesterID     string        `db:"requester_id"         json:"requester_id"`
	RequesterUserID string        `db:"requester_user_id"    json:"requester_user_id"`
	CategoryID      *string       `db:"category_id"          json:"category_id,omitempty"`
	Subject         string        `db:"subject"              json:"subject"`
	Description     *string       `db:"description"          json:"description,omitempty"`
	Priority        Priority      `db:"priority"             json:"priority"`
	Status          Status        `db:"status"               json:"status"`
	AssigneeUserID  *string       `db:"assignee_user_id"     json:"assignee_user_id,omitempty"`
	SLAPolicyID     *string       `db:"sla_policy_id"        json:"sla_policy_id,omitempty"`
	FirstResponseAt *time.Time    `db:"first_response_at"    json:"first_response_at,omitempty"`
	ResolvedAt      *time.Time    `db:"resolved_at"          json:"resolved_at,omitempty"`
	ClosedAt        *time.Time    `db:"closed_at"            json:"closed_at,omitempty"`
	ConvertedToType *string       `db:"converted_to_type"    json:"converted_to_type,omitempty"`
	ConvertedToID   *string       `db:"converted_to_id"      json:"converted_to_id,omitempty"`
	ConvertedAt     *time.Time    `db:"converted_at"         json:"converted_at,omitempty"`
	CreatedAt       time.Time     `db:"created_at"           json:"created_at"`
	UpdatedAt       time.Time     `db:"updated_at"           json:"updated_at"`

	// DERIVED on read — no backing columns.
	FirstResponseSLA *SLAStatus `db:"-" json:"first_response_sla,omitempty"`
	ResolutionSLA    *SLAStatus `db:"-" json:"resolution_sla,omitempty"`
	Comments         []*Comment `db:"-" json:"comments,omitempty"`
}

type CreateTicketRequest struct {
	// RequesterID is the caller's subject id in the requester_type domain —
	// their hrm_employees.id today. Resolved by the CALLER (the HRM-facing
	// layer), because this package must not query hrm_*.
	RequesterID string  `json:"requester_id"`
	CategoryID  *string `json:"category_id"`
	Subject     string  `json:"subject"`
	Description *string `json:"description"`
	Priority    *string `json:"priority"`
}

type AssignTicketRequest struct {
	AssigneeUserID string `json:"assignee_user_id"`
}

type PauseTicketRequest struct {
	Reason *string `json:"reason"`
}

// ── Comments ─────────────────────────────────────────────────────────────────

// Comment is public unless IsInternal. Internal comments are filtered at the
// REPOSITORY layer via two separate read methods — the requester's path never
// selects them, so there is nothing in scope to forget to strip. The 5C
// 360-anonymity / 6A answer-key precedent.
type Comment struct {
	ID           string    `db:"id"               json:"id"`
	PublicID     string    `db:"public_id"         json:"public_id"`
	TicketID     string    `db:"ticket_id"         json:"ticket_id"`
	AuthorUserID string    `db:"author_user_id"    json:"author_user_id"`
	Body         string    `db:"body"              json:"body"`
	IsInternal   bool      `db:"is_internal"       json:"is_internal"`
	CreatedAt    time.Time `db:"created_at"        json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"        json:"updated_at"`
}

type CreateCommentRequest struct {
	Body       string `json:"body"`
	IsInternal *bool  `json:"is_internal"`
}

// ── Listing ──────────────────────────────────────────────────────────────────

// ListFilter has NO authz.Scope field, unlike every hrm/* filter in this
// codebase. That is the platform decision made concrete: internal/hrm/scope
// hard-codes FROM hrm_employees, so a platform package cannot use it.
// Visibility is narrowed by ViewerUserID + CanViewAll instead, resolved by
// the service from the AccessDirectory.
type ListFilter struct {
	Status     string
	Priority   string
	CategoryID string
	// AssigneeUserID filters to one agent's queue.
	AssigneeUserID string

	// ViewerUserID is whose tickets are visible when CanViewAll is false:
	// those they raised, plus those assigned to them.
	ViewerUserID string
	CanViewAll   bool

	Limit  int
	Offset int
}

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

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

type TicketListResponse struct {
	Tickets []*Ticket `json:"tickets"`
	Total   int       `json:"total"`
	Limit   int       `json:"limit"`
	Offset  int       `json:"offset"`
}

// ── Errors ───────────────────────────────────────────────────────────────────

var (
	ErrTicketNotFound   = errors.New("ticket not found")
	ErrCategoryNotFound = errors.New("ticket category not found")
	ErrPolicyNotFound   = errors.New("SLA policy not found")
	ErrCommentNotFound  = errors.New("ticket comment not found")

	ErrInvalidPriority      = errors.New("priority is not a recognised value")
	ErrInvalidRequesterType = errors.New("requester_type is not a recognised value")
	ErrInvalidSLAMinutes    = errors.New("SLA minutes must be positive and resolution must not precede first response")
	// ErrSensitiveCategoryRole blocks assigning a sensitive ticket to
	// somebody outside its restricted role — the whole point of marking a
	// category sensitive.
	ErrSensitiveCategoryRole = errors.New("this category is restricted to a specific role; the assignee does not hold it")
	ErrRestrictedRoleMissing = errors.New("a sensitive category requires a restricted_role")
	ErrAlreadyPaused         = errors.New("ticket SLA clock is already paused")
	ErrNotPaused             = errors.New("ticket SLA clock is not paused")
	ErrAlreadyConverted      = errors.New("ticket has already been converted and cannot be converted again")
	ErrWrongStatus           = errors.New("action not allowed in the ticket's current status")
	ErrAccessDenied          = errors.New("access denied")
)
