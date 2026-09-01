// backend/internal/hrm/promotions/model.go
package promotions

import (
	"errors"
	"github.com/shopspring/decimal"
	"time"

	"github.com/mridha/businesssaas/internal/authz"
)

type PromotionStatus string

const (
	StatusDraft           PromotionStatus = "draft"
	StatusPendingApproval PromotionStatus = "pending_approval"
	StatusApproved        PromotionStatus = "approved"
	StatusRejected        PromotionStatus = "rejected"
	StatusCancelled       PromotionStatus = "cancelled"
	StatusApplied         PromotionStatus = "applied"
)

// Promotion records a position/department/pay change for an employee.
// from_* fields are snapshots of the employee's state at record creation.
// to_* fields represent what changes after Apply().
type Promotion struct {
	ID                    string           `db:"id"                       json:"id"`
	PublicID              string           `db:"public_id"                json:"public_id"`
	OrgID                 string           `db:"org_id"                   json:"org_id"`
	EmployeeID            string           `db:"employee_id"              json:"employee_id"`
	FromPositionID        *string          `db:"from_position_id"         json:"from_position_id,omitempty"`
	FromDepartmentID      *string          `db:"from_department_id"       json:"from_department_id,omitempty"`
	FromSalaryStructureID *string          `db:"from_salary_structure_id" json:"from_salary_structure_id,omitempty"`
	FromBasicPay          *decimal.Decimal `db:"from_basic_pay"           json:"from_basic_pay,omitempty"`
	ToPositionID          string           `db:"to_position_id"           json:"to_position_id"`
	ToDepartmentID        *string          `db:"to_department_id"         json:"to_department_id,omitempty"`
	ToSalaryStructureID   *string          `db:"to_salary_structure_id"   json:"to_salary_structure_id,omitempty"`
	NewBasicPay           *decimal.Decimal `db:"new_basic_pay"            json:"new_basic_pay,omitempty"`
	EffectiveDate         string           `db:"effective_date"           json:"effective_date"`
	Reason                *string          `db:"reason"                   json:"reason,omitempty"`
	Notes                 *string          `db:"notes"                    json:"notes,omitempty"`
	ApprovalInstanceID    *string          `db:"approval_instance_id"     json:"approval_instance_id,omitempty"`
	DocumentID            *string          `db:"document_id"              json:"document_id,omitempty"`
	Status                PromotionStatus  `db:"status"                   json:"status"`
	AppliedAt             *time.Time       `db:"applied_at"               json:"applied_at,omitempty"`
	AppliedBy             *string          `db:"applied_by"               json:"applied_by,omitempty"`
	CreatedBy             string           `db:"created_by"               json:"created_by"`
	CreatedAt             time.Time        `db:"created_at"               json:"created_at"`
	UpdatedAt             time.Time        `db:"updated_at"               json:"updated_at"`
}

type CreatePromotionRequest struct {
	ToPositionID        string           `json:"to_position_id"`
	ToDepartmentID      *string          `json:"to_department_id"`
	ToSalaryStructureID *string          `json:"to_salary_structure_id"`
	NewBasicPay         *decimal.Decimal `json:"new_basic_pay"`
	EffectiveDate       string           `json:"effective_date"`
	Reason              *string          `json:"reason"`
	Notes               *string          `json:"notes"`
}

type UpdatePromotionRequest struct {
	ToPositionID        *string          `json:"to_position_id"`
	ToDepartmentID      *string          `json:"to_department_id"`
	ToSalaryStructureID *string          `json:"to_salary_structure_id"`
	NewBasicPay         *decimal.Decimal `json:"new_basic_pay"`
	EffectiveDate       *string          `json:"effective_date"`
	Reason              *string          `json:"reason"`
	Notes               *string          `json:"notes"`
	DocumentID          *string          `json:"document_id"`
}

type PromotionListResponse struct {
	Promotions []*Promotion `json:"promotions"`
	Total      int          `json:"total"`
	Limit      int          `json:"limit"`
	Offset     int          `json:"offset"`
}

// PromotionListFilter narrows the promotion list query. Scope is enforced
// against the employee's current hrm_employees.manager_id/department_id
// (via scope.Predicate's live subquery) — never this record's own from_*/to_*
// snapshot columns, which reflect state at the time of the promotion, not
// today's reporting line.
type PromotionListFilter struct {
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

func (f *PromotionListFilter) Normalise() {
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
	ErrNotFound           = errors.New("promotion not found")
	ErrToPositionRequired = errors.New("to_position_id is required")
	ErrEffectiveDateReq   = errors.New("effective_date is required (YYYY-MM-DD)")
	ErrInvalidDate        = errors.New("effective_date must be a valid YYYY-MM-DD")
	ErrWrongStatus        = errors.New("action not allowed in current promotion status")
	ErrAlreadyApplied     = errors.New("promotion has already been applied")
	ErrNotApproved        = errors.New("promotion must be approved before applying")
)
