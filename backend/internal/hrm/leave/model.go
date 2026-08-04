// backend/internal/hrm/leave/model.go
package leave

import (
	"errors"
	"time"

	"github.com/mridha/businesssaas/internal/authz"
)

// LeaveRequestStatus defines the allowed status values for a leave request.
type LeaveRequestStatus string

const (
	LeaveRequestStatusPending   LeaveRequestStatus = "pending"
	LeaveRequestStatusApproved  LeaveRequestStatus = "approved"
	LeaveRequestStatusRejected  LeaveRequestStatus = "rejected"
	LeaveRequestStatusCancelled LeaveRequestStatus = "cancelled"
)

func (s LeaveRequestStatus) IsValid() bool {
	switch s {
	case LeaveRequestStatusPending, LeaveRequestStatusApproved,
		LeaveRequestStatusRejected, LeaveRequestStatusCancelled:
		return true
	}
	return false
}

// ─────────────────────────────────────────────────────────
// Leave Types
// ─────────────────────────────────────────────────────────

// LeaveType is the core domain type for a configurable leave category.
// Mirrors hrm_leave_types columns exactly.
type LeaveType struct {
	ID               string    `db:"id"                 json:"id"`
	PublicID         string    `db:"public_id"          json:"public_id"`
	OrgID            string    `db:"org_id"             json:"org_id"`
	Name             string    `db:"name"               json:"name"`
	Description      *string   `db:"description"        json:"description,omitempty"`
	MaxDaysPerYear   int       `db:"max_days_per_year"  json:"max_days_per_year"` // 0 = unlimited
	IsPaid           bool      `db:"is_paid"            json:"is_paid"`
	RequiresApproval bool      `db:"requires_approval"  json:"requires_approval"`
	IsActive         bool      `db:"is_active"          json:"is_active"`
	CreatedBy        string    `db:"created_by"         json:"created_by"`
	CreatedAt        time.Time `db:"created_at"         json:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"         json:"updated_at"`
}

// CreateLeaveTypeRequest is the body for POST /hrm/leave/types.
type CreateLeaveTypeRequest struct {
	Name             string  `json:"name"`
	Description      *string `json:"description"`
	MaxDaysPerYear   int     `json:"max_days_per_year"` // 0 = unlimited
	IsPaid           *bool   `json:"is_paid"`           // default: true
	RequiresApproval *bool   `json:"requires_approval"` // default: true
}

// UpdateLeaveTypeRequest is the body for PATCH /hrm/leave/types/:typeId.
type UpdateLeaveTypeRequest struct {
	Name             *string `json:"name"`
	Description      *string `json:"description"`
	MaxDaysPerYear   *int    `json:"max_days_per_year"`
	IsPaid           *bool   `json:"is_paid"`
	RequiresApproval *bool   `json:"requires_approval"`
	IsActive         *bool   `json:"is_active"`
}

// LeaveTypeListResponse wraps the leave type list.
type LeaveTypeListResponse struct {
	LeaveTypes []*LeaveType `json:"leave_types"`
	Total      int          `json:"total"`
}

// ─────────────────────────────────────────────────────────
// Leave Requests
// ─────────────────────────────────────────────────────────

// LeaveRequest is the core domain type for an employee leave request.
// Mirrors hrm_leave_requests columns exactly.
type LeaveRequest struct {
	ID           string             `db:"id"              json:"id"`
	PublicID     string             `db:"public_id"       json:"public_id"`
	OrgID        string             `db:"org_id"          json:"org_id"`
	EmployeeID   string             `db:"employee_id"     json:"employee_id"`
	LeaveTypeID  string             `db:"leave_type_id"   json:"leave_type_id"`
	StartDate    time.Time          `db:"start_date"      json:"start_date"`
	EndDate      time.Time          `db:"end_date"        json:"end_date"`
	TotalDays    float64            `db:"total_days"      json:"total_days"`
	Reason       *string            `db:"reason"          json:"reason,omitempty"`
	Status       LeaveRequestStatus `db:"status"          json:"status"`
	ReviewedBy   *string            `db:"reviewed_by"     json:"reviewed_by,omitempty"`
	ReviewedAt   *time.Time         `db:"reviewed_at"     json:"reviewed_at,omitempty"`
	ReviewNote   *string            `db:"review_note"     json:"review_note,omitempty"`
	CreatedBy    string             `db:"created_by"      json:"created_by"`
	CreatedAt    time.Time          `db:"created_at"      json:"created_at"`
	UpdatedAt    time.Time          `db:"updated_at"      json:"updated_at"`
}

// CreateLeaveRequestRequest is the body for POST /hrm/leave/requests.
type CreateLeaveRequestRequest struct {
	EmployeeID  string  `json:"employee_id"`  // required
	LeaveTypeID string  `json:"leave_type_id"` // required
	StartDate   string  `json:"start_date"`    // YYYY-MM-DD
	EndDate     string  `json:"end_date"`      // YYYY-MM-DD
	TotalDays   float64 `json:"total_days"`    // supports 0.5 for half-days
	Reason      *string `json:"reason"`
}

// ReviewLeaveRequestRequest is the body for POST /hrm/leave/requests/:reqId/approve
// and POST /hrm/leave/requests/:reqId/reject.
type ReviewLeaveRequestRequest struct {
	Note *string `json:"note"`
}

// LeaveRequestFilter narrows the leave request list.
type LeaveRequestFilter struct {
	EmployeeID  string
	LeaveTypeID string
	Status      LeaveRequestStatus
	Limit       int
	Offset      int

	// Scope and CallerUserID are set by the handler (from authzSvc.ResolveScope)
	// before calling Service.ListRequests. Scope zero value (authz.ScopeNone)
	// means "no rows" — callers that intend no scoping must explicitly pass
	// authz.ScopeAll.
	Scope        authz.Scope
	CallerUserID string
}

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

func (f *LeaveRequestFilter) Normalise() {
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

// LeaveRequestListResponse wraps paginated list results.
type LeaveRequestListResponse struct {
	Requests []*LeaveRequest `json:"requests"`
	Total    int             `json:"total"`
	Limit    int             `json:"limit"`
	Offset   int             `json:"offset"`
}

// ─────────────────────────────────────────────────────────
// Sentinel errors
// ─────────────────────────────────────────────────────────

const dateLayout = "2006-01-02"

var (
	// Leave type errors
	ErrLeaveTypeNotFound = errors.New("leave type not found")
	ErrLeaveTypeNameReq  = errors.New("leave type name is required")
	ErrLeaveTypeNameLong = errors.New("leave type name must not exceed 100 characters")
	ErrLeaveTypeConflict = errors.New("a leave type with this name already exists")

	// Leave request errors
	ErrLeaveRequestNotFound    = errors.New("leave request not found")
	ErrEmployeeIDRequired      = errors.New("employee_id is required")
	ErrLeaveTypeIDRequired     = errors.New("leave_type_id is required")
	ErrStartDateRequired       = errors.New("start_date is required")
	ErrEndDateRequired         = errors.New("end_date is required")
	ErrInvalidStartDate        = errors.New("start_date must be a valid date in YYYY-MM-DD format")
	ErrInvalidEndDate          = errors.New("end_date must be a valid date in YYYY-MM-DD format")
	ErrEndBeforeStart          = errors.New("end_date cannot be before start_date")
	ErrInvalidTotalDays        = errors.New("total_days must be greater than 0")
	ErrNotPending              = errors.New("only pending leave requests can be approved or rejected")
	ErrCannotCancelNotOwn      = errors.New("only the request owner or a manager can cancel a leave request")
	ErrAlreadyCancelled        = errors.New("leave request is already cancelled")
	ErrLeaveTypeInactive       = errors.New("the selected leave type is inactive")
)
