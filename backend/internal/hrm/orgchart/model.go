// backend/internal/hrm/orgchart/model.go
package orgchart

import (
	"errors"
	"time"
)

// RelationshipType separates authorization from matrix reporting.
//
// ⚠ Only Solid feeds hrm_employees.manager_id, and therefore scope.Predicate's
// view_team tier. The other three are real reporting lines an org chart must
// draw, but they must NEVER widen data access — a project lead who could read
// their contributors' payroll because of a project line would be a quiet
// privilege escalation.
type RelationshipType string

const (
	Solid      RelationshipType = "solid"
	Dotted     RelationshipType = "dotted"
	Functional RelationshipType = "functional"
	Project    RelationshipType = "project"
)

func (r RelationshipType) IsValid() bool {
	switch r {
	case Solid, Dotted, Functional, Project:
		return true
	}
	return false
}

// GrantsDataAccess reports whether this line feeds manager_id.
// Stated as a method rather than an `== Solid` comparison scattered around,
// so the rule has one home.
func (r RelationshipType) GrantsDataAccess() bool { return r == Solid }

// Relationship is one effective-dated reporting line.
type Relationship struct {
	ID               string           `db:"id"                 json:"id"`
	PublicID         string           `db:"public_id"           json:"public_id"`
	OrgID            string           `db:"org_id"              json:"org_id"`
	EmployeeID       string           `db:"employee_id"         json:"employee_id"`
	ManagerID        string           `db:"manager_id"          json:"manager_id"`
	RelationshipType RelationshipType `db:"relationship_type"   json:"relationship_type"`
	EffectiveFrom    time.Time        `db:"effective_from"      json:"effective_from"`
	// Nil = still in force. A relationship is ENDED by stamping this, never
	// by deleting the row — the history is what makes the table worth having.
	EffectiveTo *time.Time `db:"effective_to" json:"effective_to,omitempty"`
	Note        *string    `db:"note"         json:"note,omitempty"`
	CreatedBy   string     `db:"created_by"   json:"created_by"`
	CreatedAt   time.Time  `db:"created_at"   json:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"   json:"updated_at"`
}

func (r *Relationship) IsActive() bool { return r.EffectiveTo == nil }

type CreateRelationshipRequest struct {
	EmployeeID       string  `json:"employee_id"`
	ManagerID        string  `json:"manager_id"`
	RelationshipType string  `json:"relationship_type"`
	EffectiveFrom    *string `json:"effective_from"`
	Note             *string `json:"note"`
}

type EndRelationshipRequest struct {
	// EffectiveTo defaults to today. A line ends on a date, because "who did
	// this person report to in March" has to stay answerable.
	EffectiveTo *string `json:"effective_to"`
}

// ── Position seats ───────────────────────────────────────────────────────────

// Seat is one seat on a position. There is no IsOccupied field: occupancy is
// EmployeeID != nil, derived, per the 00076 rule.
type Seat struct {
	ID         string    `db:"id"           json:"id"`
	PublicID   string    `db:"public_id"     json:"public_id"`
	OrgID      string    `db:"org_id"        json:"org_id"`
	PositionID string    `db:"position_id"   json:"position_id"`
	EmployeeID *string   `db:"employee_id"   json:"employee_id,omitempty"`
	SeatLabel  *string   `db:"seat_label"    json:"seat_label,omitempty"`
	IsActive   bool      `db:"is_active"     json:"is_active"`
	CreatedBy  string    `db:"created_by"    json:"created_by"`
	CreatedAt  time.Time `db:"created_at"    json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"    json:"updated_at"`
}

// IsVacant is what a future requisition would be raised against.
func (s *Seat) IsVacant() bool { return s.EmployeeID == nil }

type CreateSeatRequest struct {
	PositionID string  `json:"position_id"`
	EmployeeID *string `json:"employee_id"`
	SeatLabel  *string `json:"seat_label"`
}

type AssignSeatRequest struct {
	// Nil vacates the seat. Distinct from deleting it: the seat still exists
	// as headcount, which is exactly what makes a vacancy visible.
	EmployeeID *string `json:"employee_id"`
}

// ── Chart ────────────────────────────────────────────────────────────────────

// ChartNode is one node of the rendered chart. Children are ids rather than
// nested nodes so a caller can lazily expand a subtree without the server
// materialising the whole graph.
type ChartNode struct {
	EmployeeID   string   `json:"employee_id"`
	DisplayName  string   `json:"display_name"`
	PositionName *string  `json:"position_name,omitempty"`
	ManagerID    *string  `json:"manager_id,omitempty"`
	ChildIDs     []string `json:"child_ids"`
	// MatrixLines are the dotted/functional/project relationships. Reported
	// separately from ChildIDs so a consumer cannot mistake a matrix line for
	// a reporting line that confers access.
	MatrixLines []*Relationship `json:"matrix_lines,omitempty"`
}

// ── Errors ───────────────────────────────────────────────────────────────────

var (
	ErrRelationshipNotFound = errors.New("reporting relationship not found")
	ErrSeatNotFound         = errors.New("position seat not found")
	ErrEmployeeNotFound     = errors.New("employee not found in this organization")
	ErrInvalidType          = errors.New("relationship_type must be one of solid, dotted, functional, project")
	ErrSelfManagement       = errors.New("an employee cannot report to themselves")
	// ErrWouldCreateCycle is refused BEFORE the insert. A cycle makes
	// scope.Predicate's view_team CTE non-terminating, so this is an
	// authorization safety check, not a data-tidiness one.
	ErrWouldCreateCycle = errors.New("this reporting line would create a cycle in the management chain")
	ErrDuplicateSolid   = errors.New("this employee already has an active solid-line manager; end it first")
	ErrAlreadyEnded     = errors.New("this reporting relationship has already ended")
	ErrSeatOccupied     = errors.New("this employee already occupies another active seat")
	ErrAccessDenied     = errors.New("access denied")
)
