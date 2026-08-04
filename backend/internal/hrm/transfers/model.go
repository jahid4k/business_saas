// backend/internal/hrm/transfers/model.go
package transfers

import (
	"errors"
	"time"

	"github.com/mridha/businesssaas/internal/authz"
)

type TransferType string
const (
	TransferTypeDepartment TransferType = "department"
	TransferTypeLocation   TransferType = "location"
	TransferTypeReporting  TransferType = "reporting"
	TransferTypeFull       TransferType = "full"
)
func (t TransferType) IsValid() bool {
	switch t { case TransferTypeDepartment, TransferTypeLocation, TransferTypeReporting, TransferTypeFull: return true }
	return false
}

type TransferStatus string
const (
	StatusDraft           TransferStatus = "draft"
	StatusPendingApproval TransferStatus = "pending_approval"
	StatusApproved        TransferStatus = "approved"
	StatusRejected        TransferStatus = "rejected"
	StatusCancelled       TransferStatus = "cancelled"
	StatusApplied         TransferStatus = "applied"
)

type Transfer struct {
	ID                    string         `db:"id"                       json:"id"`
	PublicID              string         `db:"public_id"                json:"public_id"`
	OrgID                 string         `db:"org_id"                   json:"org_id"`
	EmployeeID            string         `db:"employee_id"              json:"employee_id"`
	TransferType          TransferType   `db:"transfer_type"            json:"transfer_type"`
	FromDepartmentID      *string        `db:"from_department_id"       json:"from_department_id,omitempty"`
	FromManagerEmployeeID *string        `db:"from_manager_employee_id" json:"from_manager_employee_id,omitempty"`
	FromLocation          *string        `db:"from_location"            json:"from_location,omitempty"`
	ToDepartmentID        *string        `db:"to_department_id"         json:"to_department_id,omitempty"`
	ToManagerEmployeeID   *string        `db:"to_manager_employee_id"   json:"to_manager_employee_id,omitempty"`
	ToLocation            *string        `db:"to_location"              json:"to_location,omitempty"`
	EffectiveDate         string         `db:"effective_date"           json:"effective_date"`
	Reason                *string        `db:"reason"                   json:"reason,omitempty"`
	Notes                 *string        `db:"notes"                    json:"notes,omitempty"`
	ApprovalInstanceID    *string        `db:"approval_instance_id"     json:"approval_instance_id,omitempty"`
	DocumentID            *string        `db:"document_id"              json:"document_id,omitempty"`
	Status                TransferStatus `db:"status"                   json:"status"`
	AppliedAt             *time.Time     `db:"applied_at"               json:"applied_at,omitempty"`
	AppliedBy             *string        `db:"applied_by"               json:"applied_by,omitempty"`
	CreatedBy             string         `db:"created_by"               json:"created_by"`
	CreatedAt             time.Time      `db:"created_at"               json:"created_at"`
	UpdatedAt             time.Time      `db:"updated_at"               json:"updated_at"`
}

type CreateTransferRequest struct {
	TransferType        TransferType `json:"transfer_type"`
	ToDepartmentID      *string      `json:"to_department_id"`
	ToManagerEmployeeID *string      `json:"to_manager_employee_id"`
	ToLocation          *string      `json:"to_location"`
	EffectiveDate       string       `json:"effective_date"`
	Reason              *string      `json:"reason"`
	Notes               *string      `json:"notes"`
}

type UpdateTransferRequest struct {
	ToDepartmentID      *string `json:"to_department_id"`
	ToManagerEmployeeID *string `json:"to_manager_employee_id"`
	ToLocation          *string `json:"to_location"`
	EffectiveDate       *string `json:"effective_date"`
	Reason              *string `json:"reason"`
	Notes               *string `json:"notes"`
	DocumentID          *string `json:"document_id"`
}

type TransferListResponse struct {
	Transfers []*Transfer `json:"transfers"`
	Total     int         `json:"total"`
	Limit     int         `json:"limit"`
	Offset    int         `json:"offset"`
}

// TransferListFilter narrows the transfer list query. Scope is enforced
// against the employee's current hrm_employees.manager_id/department_id
// (via scope.Predicate's live subquery) — never this record's own from_*/to_*
// snapshot columns, which reflect state at the time of the transfer, not
// today's reporting line.
type TransferListFilter struct {
	EmployeeID string
	Status     string
	Limit      int
	Offset     int

	// Scope and CallerUserID are set by the handler (from authzSvc.ResolveScope)
	// before calling Service.List. Scope zero value (authz.ScopeNone) means "no
	// rows" — callers that intend no scoping must explicitly pass authz.ScopeAll.
	Scope        authz.Scope
	CallerUserID string
}

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

func (f *TransferListFilter) Normalise() {
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

var (
	ErrNotFound            = errors.New("transfer not found")
	ErrInvalidTransferType = errors.New("transfer_type must be: department, location, reporting, or full")
	ErrEffectiveDateReq    = errors.New("effective_date is required (YYYY-MM-DD)")
	ErrInvalidDate         = errors.New("effective_date must be a valid YYYY-MM-DD")
	ErrWrongStatus         = errors.New("action not allowed in current transfer status")
	ErrAlreadyApplied      = errors.New("transfer has already been applied")
	ErrNotApproved         = errors.New("transfer must be approved before applying")
)
